package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

const (
	passwordResetTokenTTL   = 1 * time.Hour
	passwordResetRateWindow = 1 * time.Hour
	passwordResetMaxPerHour = 3
)

// PasswordService handles forgot-password and reset-password flows.
type PasswordService struct {
	users        UserRepository
	resets       PasswordResetRepository
	tokens       RefreshTokenRepository
	hasher       PasswordHasher
	clock        Clock
	audit        AuditRepository
	mailer       EmailSender
	tx           TxManager
	resetURLBase string // e.g. "http://localhost:3000/auth/reset" — appended with ?token=<raw>
	senderName   string // e.g. "Lustia"
}

// NewPasswordService constructs a PasswordService.
// mailer may be a no-op sender when email dispatch is not wired yet.
// resetURLBase is appended with "?token=<raw>" to form the user-facing link.
func NewPasswordService(
	users UserRepository,
	resets PasswordResetRepository,
	tokens RefreshTokenRepository,
	hasher PasswordHasher,
	clock Clock,
	audit AuditRepository,
	mailer EmailSender,
	tx TxManager,
	resetURLBase string,
	senderName string,
) *PasswordService {
	return &PasswordService{
		users:        users,
		resets:       resets,
		tokens:       tokens,
		hasher:       hasher,
		clock:        clock,
		audit:        audit,
		mailer:       mailer,
		tx:           tx,
		resetURLBase: resetURLBase,
		senderName:   senderName,
	}
}

// ForgotPassword initiates a password reset. It always returns nil (success)
// regardless of whether the email exists — anti-enumeration.
//
// ADR 0007: tenant_slug is removed from the input. The user lookup is now
// global by email. The tenant context is switched to __platform__ before the
// lookup so the RLS-protected "user" table is always readable.
func (s *PasswordService) ForgotPassword(ctx context.Context, in ForgotPasswordInput) error {
	// Switch to platform context so the RLS-protected "user" table is readable
	// without knowing which tenant the user belongs to.
	if err := s.tx.SetTenantContext(ctx, constants.PlatformTenantSentinel, ""); err != nil {
		return fmt.Errorf("switch tenant context: %w", err)
	}

	user, err := s.users.FindByEmail(ctx, in.Email)
	if err != nil {
		return nil // anti-enumeration: silently succeed
	}
	if !user.IsActive {
		return nil
	}

	now := s.clock.Now()

	count, err := s.resets.CountRecentByUser(ctx, user.ID, now.Add(-passwordResetRateWindow))
	if err != nil {
		return fmt.Errorf("count recent resets: %w", err)
	}
	if count >= passwordResetMaxPerHour {
		return nil
	}

	rawToken := helper.GenerateOpaqueToken()
	hash := helper.HashToken(rawToken)

	prt := &model.PasswordReset{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(passwordResetTokenTTL),
		CreatedAt: now,
	}
	if err := s.resets.Save(ctx, prt); err != nil {
		return fmt.Errorf("save password reset token: %w", err)
	}

	// Fire-and-forget email dispatch. Errors are swallowed to preserve the
	// anti-enumeration contract (the API response is already committed to 204
	// regardless of outcome). The raw token is embedded in the link and MUST
	// NOT be logged by any helper downstream.
	link := s.resetURLBase + "?token=" + rawToken
	emailTo := user.Email
	senderName := s.senderName
	mailer := s.mailer
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = mailer.Send(bgCtx, EmailMessage{
			To:       emailTo,
			Subject:  senderName + " — password reset",
			TextBody: buildResetEmailText(senderName, link),
			HTMLBody: buildResetEmailHTML(senderName, link),
		})
	}()

	return nil
}

// buildResetEmailText composes the plaintext body for a password-reset email.
func buildResetEmailText(senderName, link string) string {
	return "Hi,\n\n" +
		"You (or someone claiming to be you) requested a password reset on " + senderName + ".\n" +
		"Click the link below within 60 minutes to set a new password:\n\n" +
		link + "\n\n" +
		"If you did not request this, you can ignore this email. The link will expire on its own.\n\n" +
		"— " + senderName
}

// buildResetEmailHTML composes the HTML body.
func buildResetEmailHTML(senderName, link string) string {
	return `<!doctype html><html><body style="font-family:system-ui,sans-serif;line-height:1.5;color:#222">` +
		`<p>Hi,</p>` +
		`<p>You (or someone claiming to be you) requested a password reset on <strong>` + senderName + `</strong>.</p>` +
		`<p>Click the button below within 60 minutes to set a new password:</p>` +
		`<p><a href="` + link + `" style="display:inline-block;padding:10px 16px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px">Reset password</a></p>` +
		`<p>Or open this link:<br><code>` + link + `</code></p>` +
		`<p>If you did not request this, you can ignore this email. The link will expire on its own.</p>` +
		`<p>— ` + senderName + `</p>` +
		`</body></html>`
}

// ResetPassword consumes a password reset token and sets a new password.
func (s *PasswordService) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	hash := helper.HashToken(in.Token)
	prt, err := s.resets.FindByHash(ctx, hash)
	if err != nil {
		return constants.ErrPasswordResetTokenNotFound
	}

	now := s.clock.Now()
	if prt.IsExpired(now) {
		return constants.ErrPasswordResetTokenExpired
	}
	if prt.IsUsed() {
		return constants.ErrPasswordResetTokenUsed
	}

	newHash, err := s.hasher.Hash(ctx, in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := s.users.UpdatePassword(ctx, prt.UserID, newHash); err != nil {
		return fmt.Errorf("persist new password: %w", err)
	}

	if err := s.resets.MarkUsed(ctx, prt.ID); err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}

	if err := s.tokens.RevokeAllForUser(ctx, prt.UserID); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}

	go func() {
		uid := prt.UserID
		_ = s.audit.Append(context.Background(), AuditEntry{
			ActorUserID:  &uid,
			Action:       "auth.password_reset",
			ResourceType: "user",
			ResourceID:   prt.UserID,
			Meta:         map[string]interface{}{},
		})
	}()

	return nil
}

// ChangePassword verifies the current password and replaces it, then revokes
// all other refresh tokens (forces re-login on all other sessions).
func (s *PasswordService) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	user, err := s.users.FindByID(ctx, in.CallerUserID)
	if err != nil {
		return fmt.Errorf("find user for password change: %w", err)
	}

	match, err := s.hasher.Verify(ctx, in.OldPassword, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify old password: %w", err)
	}
	if !match {
		return constants.ErrInvalidCredentials
	}

	newHash, err := s.hasher.Hash(ctx, in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := s.users.UpdatePassword(ctx, user.ID, newHash); err != nil {
		return fmt.Errorf("persist new password: %w", err)
	}

	if err := s.tokens.RevokeAllForUser(ctx, user.ID); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}

	go func() {
		uid := user.ID
		_ = s.audit.Append(context.Background(), AuditEntry{
			ActorUserID:  &uid,
			Action:       "auth.password_changed",
			ResourceType: "user",
			ResourceID:   user.ID,
			Meta:         map[string]interface{}{},
		})
	}()

	return nil
}
