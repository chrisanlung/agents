package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func mailpitURL() string {
	u := os.Getenv("MAILPIT_URL")
	if u == "" {
		u = "http://localhost:8025"
	}
	return u
}

// MailpitMessage is a simplified envelope from the Mailpit v1 API.
type MailpitMessage struct {
	ID      string `json:"ID"`
	Subject string `json:"Subject"`
	Snippet string `json:"Snippet"`
	To      []struct {
		Address string `json:"Address"`
		Name    string `json:"Name"`
	} `json:"To"`
}

// MailpitListResponse is the top-level shape returned by GET /api/v1/messages.
type MailpitListResponse struct {
	Messages []MailpitMessage `json:"messages"`
	Total    int              `json:"total"`
}

// Messages returns inbox messages matching the given `to` address.
// Uses the Mailpit search API: GET /api/v1/messages?query=to:<addr>
func Messages(toAddress string) ([]MailpitMessage, error) {
	query := url.QueryEscape(fmt.Sprintf("to:%s", toAddress))
	endpoint := fmt.Sprintf("%s/api/v1/messages?query=%s", mailpitURL(), query)
	resp, err := http.Get(endpoint) //nolint:noctx — test helper
	if err != nil {
		return nil, fmt.Errorf("mailpit messages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mailpit messages: HTTP %d — %s", resp.StatusCode, body)
	}
	var out MailpitListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("mailpit decode: %w", err)
	}
	return out.Messages, nil
}

// MessageBody fetches the raw text body for a Mailpit message by ID.
func MessageBody(id string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/v1/message/%s", mailpitURL(), id)
	resp, err := http.Get(endpoint) //nolint:noctx — test helper
	if err != nil {
		return "", fmt.Errorf("mailpit body: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("mailpit body read: %w", err)
	}
	return string(body), nil
}

// ClearInbox deletes all messages from Mailpit's inbox.
func ClearInbox() error {
	req, err := http.NewRequest(http.MethodDelete, mailpitURL()+"/api/v1/messages", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clear inbox: HTTP %d — %s", resp.StatusCode, body)
	}
	return nil
}
