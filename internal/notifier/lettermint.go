package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Email struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type EmailSender interface {
	Send(ctx context.Context, email Email) error
}

type LettermintSender struct {
	baseURL string
	token   string
	from    string
	client  *http.Client
}

func NewLettermintSender(baseURL string, token string, from string) *LettermintSender {
	return &LettermintSender{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		from:    from,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *LettermintSender) Send(ctx context.Context, email Email) error {
	if s.token == "" || s.from == "" {
		return fmt.Errorf("lettermint sender is not configured")
	}
	body := map[string]any{
		"from":    s.from,
		"to":      []string{email.To},
		"subject": email.Subject,
		"text":    email.Text,
		"html":    email.HTML,
		"tag":     "magic-link",
	}
	buf := bytes.Buffer{}
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/send", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-lettermint-token", s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lettermint send failed: status %d", resp.StatusCode)
	}
	return nil
}
