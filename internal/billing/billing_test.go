package billing

import (
	"errors"
	"testing"
	"time"
)

func TestVerifyWebhook(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"id":"evt_1"}`)
	signature := SignPayload("secret", payload)
	if err := VerifyWebhook("secret", payload, signature); err != nil {
		t.Fatalf("VerifyWebhook returned error: %v", err)
	}
	if err := VerifyWebhook("wrong", payload, signature); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("VerifyWebhook error = %v, want ErrInvalidSignature", err)
	}
}

func TestApplySubscriptionEventIsIdempotent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	state, changed := ApplySubscriptionEvent(SubscriptionState{}, WebhookEvent{
		ID:             "evt_1",
		CustomerID:     "cus_1",
		SubscriptionID: "sub_1",
		Plan:           "reporter",
		Status:         "active",
		OccurredAt:     now,
	})
	if !changed || state.Plan != "reporter" || state.UpdatedAt != now {
		t.Fatalf("unexpected applied state: changed=%v state=%+v", changed, state)
	}
	state, changed = ApplySubscriptionEvent(state, WebhookEvent{ID: "evt_1", Plan: "business"})
	if changed || state.Plan != "reporter" {
		t.Fatalf("duplicate event should not change state: changed=%v state=%+v", changed, state)
	}
}

func TestNewCancellation(t *testing.T) {
	t.Parallel()

	requestedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	periodEndsAt := requestedAt.Add(30 * 24 * time.Hour)
	cancellation, err := NewCancellation("org-1", "not needed", requestedAt, periodEndsAt)
	if err != nil {
		t.Fatalf("NewCancellation returned error: %v", err)
	}
	if cancellation.OrganizationID != "org-1" || cancellation.EffectiveAt != periodEndsAt {
		t.Fatalf("unexpected cancellation: %+v", cancellation)
	}
	if _, err := NewCancellation("", "", requestedAt, time.Time{}); err == nil {
		t.Fatalf("NewCancellation returned nil error for missing organization")
	}
}
