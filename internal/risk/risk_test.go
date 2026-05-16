package risk

import (
	"testing"

	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestCalculateLowRisk(t *testing.T) {
	t.Parallel()

	score := Calculate(Inputs{Posture: scanner.PostureSummary{Score: 100, Status: scanner.StatusPass}})
	if score.Value != 0 || score.Level != "low" {
		t.Fatalf("score = %+v, want low zero risk", score)
	}
}

func TestCalculateCombinesSignals(t *testing.T) {
	t.Parallel()

	score := Calculate(Inputs{
		Posture: scanner.PostureSummary{Score: 60, Status: scanner.StatusWarn, TopIssue: "DMARC warning"},
		DriftEvents: []drift.Event{
			{Severity: drift.SeverityCritical, Message: "DMARC relaxed"},
		},
		Senders:      []reporter.SenderInventoryItem{{Suspicious: true}},
		ReportHealth: reporter.HealthResult{Status: "missing_reports", Message: "Missing reports"},
	})
	if score.Value < 100 || score.Level != "critical" || len(score.Reasons) < 4 {
		t.Fatalf("score = %+v, want capped critical risk with reasons", score)
	}
}
