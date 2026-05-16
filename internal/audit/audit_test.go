package audit

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	event, err := New("domain.delete", "domain", "domain-1", "user-1")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if event.Action != "domain.delete" || event.TargetType != "domain" || event.TargetID != "domain-1" || event.UserID != "user-1" || event.CreatedAt.IsZero() {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestNewRejectsInvalidEvent(t *testing.T) {
	t.Parallel()

	if _, err := New("", "domain", "domain-1", "user-1"); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("New error = %v, want ErrInvalidEvent", err)
	}
}

func TestMetadataJSON(t *testing.T) {
	t.Parallel()

	event := Event{Metadata: map[string]string{"status": "ok"}}
	data, err := event.MetadataJSON()
	if err != nil {
		t.Fatalf("MetadataJSON returned error: %v", err)
	}
	if string(data) != `{"status":"ok"}` {
		t.Fatalf("MetadataJSON = %s", data)
	}
}
