package admin

import (
	"testing"
	"time"
)

func TestCaseLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	c, err := NewCase(CaseSupport, "Login help", "user-1", "", now)
	if err != nil {
		t.Fatalf("NewCase returned error: %v", err)
	}
	c, err = c.AddNote("admin-1", "sent troubleshooting steps", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("AddNote returned error: %v", err)
	}
	c = c.Resolve(now.Add(time.Hour))
	if c.Status != CaseResolved || len(c.Notes) != 1 || c.ResolvedAt.IsZero() {
		t.Fatalf("unexpected resolved case: %+v", c)
	}
}

func TestNewCaseRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := NewCase("", "", "", "", time.Time{}); err == nil {
		t.Fatalf("NewCase returned nil error for invalid input")
	}
}

func TestNewAbuseDecision(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	decision, err := NewAbuseDecision("203.0.113.10", AbuseActionBlock, "scanner abuse", time.Hour, now)
	if err != nil {
		t.Fatalf("NewAbuseDecision returned error: %v", err)
	}
	if decision.Identifier != "203.0.113.10" || decision.Action != AbuseActionBlock || !decision.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected abuse decision: %+v", decision)
	}
}
