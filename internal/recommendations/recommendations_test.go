package recommendations

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestForPosture(t *testing.T) {
	t.Parallel()

	recommendations := ForPosture(scanner.Result{
		SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusWarn},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusFail},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	})
	if len(recommendations) != 3 {
		t.Fatalf("len(recommendations) = %d, want 3", len(recommendations))
	}
	if !strings.Contains(recommendations[0].Title, "DMARC") {
		t.Fatalf("highest priority recommendation = %+v, want DMARC", recommendations[0])
	}
}

func TestForSender(t *testing.T) {
	t.Parallel()

	rec := ForSender(reporter.SenderInventoryItem{Suspicious: true})
	if rec.Priority != 100 || !strings.Contains(rec.Title, "suspicious") {
		t.Fatalf("unexpected sender recommendation: %+v", rec)
	}
}

func TestForDrift(t *testing.T) {
	t.Parallel()

	rec := ForDrift(drift.Event{CheckName: "SPF", Kind: drift.KindPolicyRelaxed, Severity: drift.SeverityCritical})
	if rec.Priority != 100 || !strings.Contains(rec.Title, "SPF") {
		t.Fatalf("unexpected drift recommendation: %+v", rec)
	}
}
