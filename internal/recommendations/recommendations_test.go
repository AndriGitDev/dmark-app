package recommendations

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/dmarcjourney"
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

func TestBuildChangePlanBlocksSPFOverTenLookups(t *testing.T) {
	t.Parallel()

	plan := BuildChangePlan(scanner.Result{
		Domain: "example.com",
		SPF: scanner.CheckResult{
			Name:   "SPF",
			Status: scanner.StatusWarn,
			Record: "v=spf1 include:a.example include:b.example include:c.example include:d.example include:e.example include:f.example include:g.example include:h.example include:i.example include:j.example include:k.example -all",
		},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=reject"},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	}, dmarcjourney.Evidence{})

	if len(plan) != 1 || plan[0].ID != "spf-flattening" || plan[0].Status != ChangeBlocked {
		t.Fatalf("unexpected SPF change plan: %+v", plan)
	}
}

func TestBuildChangePlanDMARCNoneWaitsWithoutEvidence(t *testing.T) {
	t.Parallel()

	plan := BuildChangePlan(scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=none; rua=mailto:dmarc@example.com"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	}, dmarcjourney.Evidence{})

	if len(plan) != 1 || plan[0].ID != "dmarc-enforcement-ladder" || plan[0].Status != ChangeWaiting {
		t.Fatalf("unexpected DMARC p=none plan: %+v", plan)
	}
}

func TestBuildChangePlanQuarantineProgression(t *testing.T) {
	t.Parallel()

	plan := BuildChangePlan(scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=none; rua=mailto:dmarc@example.com"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	}, dmarcjourney.Evidence{ReporterConfigured: true, DaysObserved: 14, TotalMessages: 1000, AlignedMessages: 995, FailedMessages: 5})

	if len(plan) != 1 || plan[0].Status != ChangeReview || !strings.Contains(plan[0].RecordText, "p=quarantine") {
		t.Fatalf("unexpected quarantine progression plan: %+v", plan)
	}
}

func TestBuildChangePlanRejectReadiness(t *testing.T) {
	t.Parallel()

	plan := BuildChangePlan(scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	}, dmarcjourney.Evidence{ReporterConfigured: true, DaysObserved: 14, TotalMessages: 1000, AlignedMessages: 995, FailedMessages: 5})

	if len(plan) != 1 || plan[0].Status != ChangeReview || !strings.Contains(plan[0].RecordText, "p=reject") {
		t.Fatalf("unexpected reject readiness plan: %+v", plan)
	}
}

func TestBuildChangePlanMissingEvidenceDoesNotClaimSafeChange(t *testing.T) {
	t.Parallel()

	plan := BuildChangePlan(scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	}, dmarcjourney.Evidence{})

	if len(plan) != 1 || plan[0].Status != ChangeWaiting || strings.Contains(strings.ToLower(plan[0].Evidence), "safe") {
		t.Fatalf("unexpected missing-evidence plan: %+v", plan)
	}
}
