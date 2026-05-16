package alerts

import (
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/drift"
)

func TestShouldNotifyStateRespectsAcknowledgementMuteAndSuppression(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	base := EventState{
		ID:     "event-1",
		Domain: "example.com",
		Event:  drift.Event{CheckName: "DMARC", Kind: drift.KindPolicyRelaxed, Severity: drift.SeverityCritical},
	}
	rule := Rule{CheckName: "DMARC", Minimum: drift.SeverityWarn, Enabled: true}

	if decision := ShouldNotifyState(base, rule, now); !decision.Send {
		t.Fatalf("ShouldNotifyState = %+v, want send", decision)
	}
	if decision := ShouldNotifyState(Acknowledge(base, now), rule, now); decision.Send || decision.Reason != "event acknowledged" {
		t.Fatalf("acknowledged decision = %+v, want acknowledged no-send", decision)
	}
	if decision := ShouldNotifyState(Mute(base, now.Add(time.Hour)), rule, now); decision.Send || decision.Reason != "event muted" {
		t.Fatalf("muted decision = %+v, want muted no-send", decision)
	}
	if decision := ShouldNotifyState(Suppress(base, now.Add(time.Hour)), rule, now); decision.Send || decision.Reason != "event suppressed" {
		t.Fatalf("suppressed decision = %+v, want suppressed no-send", decision)
	}
}

func TestBuildTimelineSortsNewestFirstAndMarksState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	states := []EventState{
		{
			ID:         "old",
			Domain:     "example.com",
			CreatedAt:  now.Add(-time.Hour),
			MutedUntil: now.Add(time.Hour),
			Event:      drift.Event{CheckName: "SPF", Severity: drift.SeverityWarn, Message: "SPF changed."},
		},
		{
			ID:             "new",
			Domain:         "example.com",
			CreatedAt:      now,
			AcknowledgedAt: now,
			Event:          drift.Event{CheckName: "DMARC", Severity: drift.SeverityCritical, Message: "DMARC relaxed."},
		},
	}

	timeline := BuildTimeline(states, now)
	if len(timeline) != 2 {
		t.Fatalf("len(timeline) = %d, want 2", len(timeline))
	}
	if timeline[0].ID != "new" || !timeline[0].Acknowledged {
		t.Fatalf("unexpected first timeline item: %+v", timeline[0])
	}
	if timeline[1].ID != "old" || !timeline[1].Muted {
		t.Fatalf("unexpected second timeline item: %+v", timeline[1])
	}
}
