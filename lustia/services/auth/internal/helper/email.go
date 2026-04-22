// Package helper — email dispatch.
//
// This file provides a minimal SMTP sender suitable for local dev (Mailpit /
// MailHog / MailCatcher) and for transactional providers that expose an SMTP
// interface (AWS SES, SendGrid, Postmark, Mailgun).
//
// Design choices:
//   - Uses the stdlib net/smtp only. No new dependency.
//   - PLAIN auth when username+password are provided; no auth when blank
//     (Mailpit-style local dev).
//   - Fire-and-forget: the caller launches this from a goroutine so API
//     responses never wait on SMTP. Errors are returned for logging, not
//     surfaced to the user (anti-enumeration).
//   - When disabled (Enabled=false or Host empty), Send returns nil without
//     contacting any network — safe default for environments that have not
//     wired email yet.
//
// Sensitive data rules (SECURITY.md §7.2):
//   - The raw reset token is part of the email Body. It MUST NOT be logged.
//   - SMTP password MUST NOT be logged. Never include the full SMTPConfig in
//     a log line — log only the non-sensitive fields.
package helper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// EmailMessage is the minimum shape an email sender needs.
// Subject + one of TextBody / HTMLBody (both is allowed; HTML wins for
// clients that render it, text is the fallback).
type EmailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string // optional
}

// SMTPConfig carries the runtime config for the sender. Load from YAML/env
// via common-configs; never hardcode.
type SMTPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`      // "Lustia <no-reply@lustia.local>"
	StartTLS bool   `yaml:"startTls"`  // upgrade plaintext connection to TLS via STARTTLS
	Timeout  int    `yaml:"timeoutMs"` // dial timeout in ms; default 5000
}

// EmailSender is the interface services depend on for sending transactional
// emails. The concrete SMTPSender below satisfies it; tests use a fake.
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// SMTPSender implements EmailSender over plain SMTP (PLAIN auth if creds
// given, no auth otherwise).
type SMTPSender struct {
	cfg SMTPConfig
}

// NewSMTPSender constructs an SMTPSender. If cfg.Enabled is false or Host is
// empty, Send becomes a no-op — safe default for environments that have not
// wired email delivery.
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5000
	}
	return &SMTPSender{cfg: cfg}
}

// Send delivers msg over SMTP. Returns nil when sender is disabled.
//
// Context is observed for cancellation during the TCP dial phase; once the
// SMTP conversation starts, stdlib net/smtp does not accept a context, so
// the only deadline is the configured Timeout.
func (s *SMTPSender) Send(ctx context.Context, msg EmailMessage) error {
	if !s.cfg.Enabled || s.cfg.Host == "" {
		return nil
	}
	if msg.To == "" || msg.Subject == "" || (msg.TextBody == "" && msg.HTMLBody == "") {
		return errors.New("smtp: incomplete email message (To, Subject, and a body are required)")
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	dialer := &net.Dialer{Timeout: time.Duration(s.cfg.Timeout) * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer func() { _ = client.Close() }()

	if s.cfg.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(nil); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(extractAddress(s.cfg.From)); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := wc.Write([]byte(buildMIME(s.cfg.From, msg))); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
	}

	return client.Quit()
}

// Enabled reports whether this sender will actually talk to a server.
// Callers can log one line on startup to make the mode obvious.
func (s *SMTPSender) Enabled() bool { return s.cfg.Enabled && s.cfg.Host != "" }

// buildMIME composes a minimal RFC-compliant MIME message. When an HTML body
// is present the message is multipart/alternative with the text part first;
// when only text is present a single text/plain part is sent.
func buildMIME(from string, msg EmailMessage) string {
	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(msg.To)
	b.WriteString("\r\n")
	b.WriteString("Subject: ")
	b.WriteString(msg.Subject)
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Date: ")
	b.WriteString(time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("\r\n")

	if msg.HTMLBody == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.TextBody)
		return b.String()
	}

	boundary := "lustia-mime-boundary-7f3a6b"
	b.WriteString("Content-Type: multipart/alternative; boundary=\"")
	b.WriteString(boundary)
	b.WriteString("\"\r\n\r\n")

	if msg.TextBody != "" {
		b.WriteString("--")
		b.WriteString(boundary)
		b.WriteString("\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.TextBody)
		b.WriteString("\r\n")
	}

	b.WriteString("--")
	b.WriteString(boundary)
	b.WriteString("\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.HTMLBody)
	b.WriteString("\r\n--")
	b.WriteString(boundary)
	b.WriteString("--\r\n")
	return b.String()
}

// extractAddress pulls the bare address out of a "Name <addr>" From header
// for the SMTP envelope; when there's no angle-bracketed form, returns as-is.
func extractAddress(from string) string {
	open := strings.LastIndex(from, "<")
	close := strings.LastIndex(from, ">")
	if open >= 0 && close > open {
		return from[open+1 : close]
	}
	return strings.TrimSpace(from)
}
