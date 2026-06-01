package ops

import (
	"testing"
	"time"
)

func TestNewEventDefaultsOccurredAtAndRejectsInvalidFields(t *testing.T) {
	created, err := NewEvent("bootstrap.skipped", SeverityInfo, "bootstrap skipped", time.Time{})
	if err != nil {
		t.Fatalf("NewEvent returned error: %v", err)
	}
	if created.Kind != "bootstrap.skipped" || created.Severity != SeverityInfo || created.Message != "bootstrap skipped" || created.OccurredAt.IsZero() {
		t.Fatalf("event = %+v, want normalized event with default occurred_at", created)
	}
	if _, err := NewEvent("", SeverityInfo, "message", time.Now()); err == nil {
		t.Fatalf("NewEvent accepted blank kind")
	}
	if _, err := NewEvent("kind", Severity("debug"), "message", time.Now()); err == nil {
		t.Fatalf("NewEvent accepted invalid severity")
	}
}

func TestSanitizeMetadataRedactsSecretShapedKeys(t *testing.T) {
	metadata := SanitizeMetadata(map[string]any{
		"provider":          "bunny",
		"status":            "degraded",
		"api_token":         "secret-value",
		"connection-string": "postgres://secret",
		"nested": map[string]any{
			"password": "nested-secret",
		},
		"records": []any{
			map[string]any{"secret": "inside-slice"},
		},
		"database":      "postgres://user:password@example.com/db",
		"database_host": "postgres://db.internal/app",
	})
	if metadata["provider"] != "bunny" || metadata["status"] != "degraded" {
		t.Fatalf("safe metadata = provider:%v status:%v, want preserved", metadata["provider"], metadata["status"])
	}
	if metadata["api_token"] != "[REDACTED]" || metadata["connection-string"] != "[REDACTED]" || metadata["database"] != "[REDACTED]" || metadata["database_host"] != "[REDACTED]" {
		t.Fatalf("metadata = %+v, want secret-shaped keys and DSN values redacted", metadata)
	}
	nested := metadata["nested"].(map[string]any)
	if nested["password"] != "[REDACTED]" {
		t.Fatalf("nested metadata = %+v, want password redacted", nested)
	}
	records := metadata["records"].([]any)
	record := records[0].(map[string]any)
	if record["secret"] != "[REDACTED]" {
		t.Fatalf("slice metadata = %+v, want secret redacted", records)
	}
}
