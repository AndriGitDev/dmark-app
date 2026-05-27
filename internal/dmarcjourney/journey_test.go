package dmarcjourney

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestEvaluateStartsWithMissingDMARC(t *testing.T) {
	t.Parallel()

	journey := Evaluate(scanner.CheckResult{Name: "DMARC", Status: scanner.StatusMissing}, Evidence{})
	if journey.State != StateNotStarted || journey.ReadyForNext {
		t.Fatalf("journey = %+v, want not_started and not ready", journey)
	}
	if !strings.Contains(journey.NextAction, "Publish a valid DMARC record") {
		t.Fatalf("NextAction = %q, want publish guidance", journey.NextAction)
	}
}

func TestEvaluateReadyForQuarantine(t *testing.T) {
	t.Parallel()

	journey := Evaluate(scanner.CheckResult{
		Name:   "DMARC",
		Status: scanner.StatusWarn,
		Record: "v=DMARC1; p=none; rua=mailto:dmarc@example.com",
	}, Evidence{
		ReporterConfigured:    true,
		DaysObserved:          14,
		TotalMessages:         10000,
		AlignedMessages:       9950,
		FailedMessages:        50,
		UnknownSenderMessages: 10,
	})
	if journey.State != StateReadyForQuarantine || !journey.ReadyForNext {
		t.Fatalf("journey = %+v, want ready_for_quarantine", journey)
	}
}

func TestEvaluateBlocksQuarantineWhenEvidenceIsNoisy(t *testing.T) {
	t.Parallel()

	journey := Evaluate(scanner.CheckResult{
		Name:   "DMARC",
		Status: scanner.StatusWarn,
		Record: "v=DMARC1; p=quarantine; pct=100; rua=mailto:dmarc@example.com",
	}, Evidence{
		ReporterConfigured:    true,
		DaysObserved:          14,
		TotalMessages:         1000,
		AlignedMessages:       900,
		FailedMessages:        75,
		UnknownSenderMessages: 25,
	})
	if journey.State != StateQuarantine || journey.ReadyForNext {
		t.Fatalf("journey = %+v, want quarantine and not ready", journey)
	}
	if len(journey.BlockingSignals) != 2 {
		t.Fatalf("BlockingSignals = %+v, want failure and unknown sender blockers", journey.BlockingSignals)
	}
}

func TestEvaluateReadyForReject(t *testing.T) {
	t.Parallel()

	journey := Evaluate(scanner.CheckResult{
		Name:   "DMARC",
		Status: scanner.StatusPass,
		Record: "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com",
	}, Evidence{
		ReporterConfigured:    true,
		DaysObserved:          30,
		TotalMessages:         5000,
		AlignedMessages:       4990,
		FailedMessages:        10,
		UnknownSenderMessages: 0,
	})
	if journey.State != StateReadyForReject || !journey.ReadyForNext {
		t.Fatalf("journey = %+v, want ready_for_reject", journey)
	}
}

func TestEvaluateRejectIsTerminal(t *testing.T) {
	t.Parallel()

	journey := Evaluate(scanner.CheckResult{
		Name:   "DMARC",
		Status: scanner.StatusPass,
		Record: "v=DMARC1; p=reject; pct=100; rua=mailto:dmarc@example.com",
	}, Evidence{})
	if journey.State != StateReject || journey.Percent != 100 || journey.ReadyForNext {
		t.Fatalf("journey = %+v, want reject terminal", journey)
	}
}
