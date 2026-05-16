package notifier

import (
	"errors"
	"testing"
	"time"
)

func TestNewDispatchRecordNormalizesFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	record := NewDispatchRecord(" domain-1 ", " event-1 ", " Email ", " PERSON@Example.COM ", " Subject ", now)

	if record.DomainID != "domain-1" || record.DriftEventID != "event-1" || record.Channel != "email" || record.Recipient != "person@example.com" || record.Subject != "Subject" {
		t.Fatalf("record not normalized: %+v", record)
	}
	if record.Status != DispatchPending || !record.CreatedAt.Equal(now) {
		t.Fatalf("unexpected initial dispatch state: %+v", record)
	}
}

func TestDispatchRecordIdempotencyKey(t *testing.T) {
	t.Parallel()

	record := NewDispatchRecord("domain-1", "event-1", "email", "person@example.com", "Subject", time.Now())
	same := NewDispatchRecord("domain-2", "event-1", "EMAIL", "PERSON@example.com", "Other", time.Now())
	different := NewDispatchRecord("domain-1", "event-2", "email", "person@example.com", "Subject", time.Now())

	if record.IdempotencyKey() != same.IdempotencyKey() {
		t.Fatalf("same event/channel/recipient should share idempotency key")
	}
	if record.IdempotencyKey() == different.IdempotencyKey() {
		t.Fatalf("different event should produce a different idempotency key")
	}
}

func TestDispatchRecordTransitions(t *testing.T) {
	t.Parallel()

	record := NewDispatchRecord("domain-1", "event-1", "email", "person@example.com", "Subject", time.Now())
	if sent := record.Sent("provider-1"); sent.Status != DispatchSent || sent.ProviderMessageID != "provider-1" || sent.Error != "" {
		t.Fatalf("sent = %+v, want sent provider state", sent)
	}
	if failed := record.Failed(errors.New("temporary failure")); failed.Status != DispatchFailed || failed.Error != "temporary failure" {
		t.Fatalf("failed = %+v, want failure state", failed)
	}
	if skipped := record.Skipped("muted"); skipped.Status != DispatchSkipped || skipped.Error != "muted" {
		t.Fatalf("skipped = %+v, want skipped state", skipped)
	}
}
