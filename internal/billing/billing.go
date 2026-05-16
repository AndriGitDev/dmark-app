package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var ErrInvalidSignature = errors.New("invalid billing webhook signature")

type WebhookEvent struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	SubscriptionID string    `json:"subscription_id"`
	Plan           string    `json:"plan"`
	Status         string    `json:"status"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func VerifyWebhook(secret string, payload []byte, signature string) error {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(signature) == "" {
		return ErrInvalidSignature
	}
	expected := SignPayload(secret, payload)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return ErrInvalidSignature
	}
	return nil
}

func SignPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type SubscriptionState struct {
	LastEventID    string
	CustomerID     string
	SubscriptionID string
	Plan           string
	Status         string
	UpdatedAt      time.Time
}

func ApplySubscriptionEvent(current SubscriptionState, event WebhookEvent) (SubscriptionState, bool) {
	if event.ID == "" || event.ID == current.LastEventID {
		return current, false
	}
	current.LastEventID = event.ID
	current.CustomerID = event.CustomerID
	current.SubscriptionID = event.SubscriptionID
	current.Plan = event.Plan
	current.Status = event.Status
	current.UpdatedAt = event.OccurredAt
	return current, true
}

type Cancellation struct {
	OrganizationID string
	Reason         string
	RequestedAt    time.Time
	EffectiveAt    time.Time
}

func NewCancellation(organizationID string, reason string, requestedAt time.Time, periodEndsAt time.Time) (Cancellation, error) {
	if organizationID == "" {
		return Cancellation{}, errors.New("organization id is required")
	}
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	effectiveAt := requestedAt
	if periodEndsAt.After(requestedAt) {
		effectiveAt = periodEndsAt
	}
	return Cancellation{
		OrganizationID: organizationID,
		Reason:         strings.TrimSpace(reason),
		RequestedAt:    requestedAt.UTC(),
		EffectiveAt:    effectiveAt.UTC(),
	}, nil
}
