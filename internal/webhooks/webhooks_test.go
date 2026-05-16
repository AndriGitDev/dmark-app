package webhooks

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEncodeSignVerify(t *testing.T) {
	t.Parallel()

	payload, err := Encode(Event{
		ID:        "evt_1",
		Type:      "drift.created",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		Data:      map[string]any{"domain": "example.com"},
	})
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	signature := Sign("secret", payload)
	if !Verify("secret", payload, signature) {
		t.Fatalf("Verify returned false for valid signature")
	}
	if Verify("wrong", payload, signature) {
		t.Fatalf("Verify returned true for wrong secret")
	}
}

func TestDriftSlackPayload(t *testing.T) {
	t.Parallel()

	payload := DriftSlackPayload("example.com", "critical", "DMARC policy relaxed.")
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if payload.Text == "" || len(payload.Blocks) != 1 || len(data) == 0 {
		t.Fatalf("unexpected Slack payload: %+v", payload)
	}
}

func TestNewEndpoint(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	endpoint, secret, err := NewEndpoint("org-1", "https://hooks.example.com/meil", []string{"drift.created", "DRIFT.CREATED", " report.ingested "}, now)
	if err != nil {
		t.Fatalf("NewEndpoint returned error: %v", err)
	}
	if endpoint.OrganizationID != "org-1" || endpoint.URL != "https://hooks.example.com/meil" || !endpoint.Enabled {
		t.Fatalf("unexpected endpoint: %+v", endpoint)
	}
	if len(endpoint.EventTypes) != 2 || endpoint.EventTypes[0] != "drift.created" || endpoint.EventTypes[1] != "report.ingested" {
		t.Fatalf("event types not normalized: %+v", endpoint.EventTypes)
	}
	if !strings.HasPrefix(secret, "whsec_") {
		t.Fatalf("secret = %q, want whsec_ prefix", secret)
	}
}

func TestNewEndpointRejectsNonHTTPS(t *testing.T) {
	t.Parallel()

	if _, _, err := NewEndpoint("org-1", "http://hooks.example.com/meil", nil, time.Now()); err == nil {
		t.Fatalf("NewEndpoint returned nil error for non-HTTPS URL")
	}
}
