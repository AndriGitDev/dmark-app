package webhooks

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	CreatedAt time.Time      `json:"created_at"`
	Data      map[string]any `json:"data"`
}

func Encode(event Event) ([]byte, error) {
	return json.Marshal(event)
}

func Sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret string, payload []byte, signature string) bool {
	expected := Sign(secret, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}

type SlackPayload struct {
	Text   string         `json:"text"`
	Blocks []SlackBlock   `json:"blocks,omitempty"`
	Meta   map[string]any `json:"metadata,omitempty"`
}

type SlackBlock struct {
	Type string `json:"type"`
	Text struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"text"`
}

func DriftSlackPayload(domain string, severity string, message string) SlackPayload {
	payload := SlackPayload{
		Text: domain + ": " + message,
		Meta: map[string]any{"domain": domain, "severity": severity},
	}
	block := SlackBlock{Type: "section"}
	block.Text.Type = "mrkdwn"
	block.Text.Text = "*" + severity + "* " + domain + "\n" + message
	payload.Blocks = []SlackBlock{block}
	return payload
}

type Endpoint struct {
	ID             string
	OrganizationID string
	URL            string
	EventTypes     []string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewEndpoint(organizationID string, rawURL string, eventTypes []string, now time.Time) (Endpoint, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return Endpoint{}, "", fmt.Errorf("webhook URL must be absolute HTTPS")
	}
	secret, err := NewSecret()
	if err != nil {
		return Endpoint{}, "", err
	}
	return Endpoint{
		OrganizationID: strings.TrimSpace(organizationID),
		URL:            parsed.String(),
		EventTypes:     normalizeEventTypes(eventTypes),
		Enabled:        true,
		CreatedAt:      now.UTC(),
		UpdatedAt:      now.UTC(),
	}, secret, nil
}

func NewSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func RotateSecret() (string, error) {
	return NewSecret()
}

func normalizeEventTypes(eventTypes []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, eventType := range eventTypes {
		eventType = strings.ToLower(strings.TrimSpace(eventType))
		if eventType == "" {
			continue
		}
		if _, ok := seen[eventType]; ok {
			continue
		}
		seen[eventType] = struct{}{}
		result = append(result, eventType)
	}
	sort.Strings(result)
	return result
}
