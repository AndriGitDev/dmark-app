package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestShouldNotify(t *testing.T) {
	t.Parallel()

	event := drift.Event{CheckName: "DMARC", Kind: drift.KindPolicyRelaxed, Severity: drift.SeverityCritical}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	rule := Rule{CheckName: "DMARC", Kind: drift.KindPolicyRelaxed, Minimum: drift.SeverityWarn, Enabled: true}

	if decision := ShouldNotify(event, rule, now); !decision.Send {
		t.Fatalf("ShouldNotify = %+v, want send", decision)
	}
	rule.MutedUntil = now.Add(time.Hour)
	if decision := ShouldNotify(event, rule, now); decision.Send || decision.Reason != "rule muted" {
		t.Fatalf("ShouldNotify muted = %+v, want muted no-send", decision)
	}
}

func TestShouldNotifySeverityThreshold(t *testing.T) {
	t.Parallel()

	event := drift.Event{CheckName: "SPF", Kind: drift.KindMaterialRecord, Severity: drift.SeverityInfo}
	rule := Rule{Minimum: drift.SeverityWarn, Enabled: true}
	if decision := ShouldNotify(event, rule, time.Now()); decision.Send {
		t.Fatalf("ShouldNotify = %+v, want no-send below threshold", decision)
	}
}

func TestBuildEmailPayload(t *testing.T) {
	t.Parallel()

	payload := BuildEmailPayload("example.com", drift.Event{
		CheckName:    "DMARC",
		Kind:         drift.KindPolicyRelaxed,
		Severity:     drift.SeverityCritical,
		Message:      "DMARC policy was relaxed.",
		BeforeStatus: scanner.StatusPass,
		AfterStatus:  scanner.StatusWarn,
		BeforeRecord: "v=DMARC1; p=reject",
		AfterRecord:  "v=DMARC1; p=none",
	})

	for _, want := range []string{"[CRITICAL]", "example.com", "DMARC"} {
		if !strings.Contains(payload.Subject, want) {
			t.Fatalf("subject missing %q: %s", want, payload.Subject)
		}
	}
	for _, want := range []string{"DMARC policy was relaxed.", "v=DMARC1; p=reject", "Recommended action"} {
		if !strings.Contains(payload.Text, want) {
			t.Fatalf("text missing %q: %s", want, payload.Text)
		}
	}
}
