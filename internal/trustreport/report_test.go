package trustreport

import (
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/dmarcjourney"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestBuildSummarizesDomainTrust(t *testing.T) {
	t.Parallel()

	report := Build(Input{
		GeneratedAt: time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
		Customer:    "Acme Iceland",
		Domains: []DomainInput{{
			Domain: "example.is",
			Scan: scanner.Result{
				Domain: "example.is",
				SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
				DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
				DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Record: "v=DMARC1; p=none; rua=mailto:dmarc@example.is"},
				BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "No BIMI record found."},
			},
			DMARCEvidence: dmarcjourney.Evidence{
				ReporterConfigured:    true,
				DaysObserved:          14,
				TotalMessages:         1000,
				AlignedMessages:       990,
				FailedMessages:        10,
				UnknownSenderMessages: 0,
			},
			SenderInventory: []reporter.InventoryEntry{{
				SourceIP:    "203.0.113.10",
				Volume:      100,
				FailureRate: 0.75,
				Suspicious:  true,
			}},
		}},
	})

	if report.TotalDomains != 1 || report.HealthyDomains != 0 || report.DomainsToReview != 1 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
	section := report.Domains[0]
	if section.DMARCJourney.State != dmarcjourney.StateReadyForQuarantine {
		t.Fatalf("DMARCJourney = %+v, want ready_for_quarantine", section.DMARCJourney)
	}
	if section.SenderRiskCounts["high"] != 1 {
		t.Fatalf("SenderRiskCounts = %+v, want one high-risk sender", section.SenderRiskCounts)
	}
	if !strings.Contains(report.ExecutiveSummary, "need review") {
		t.Fatalf("ExecutiveSummary = %q, want review summary", report.ExecutiveSummary)
	}
}

func TestBuildRecognizesHealthyRejectDomain(t *testing.T) {
	t.Parallel()

	report := Build(Input{Domains: []DomainInput{{
		Domain: "healthy.is",
		Scan: scanner.Result{
			Domain: "healthy.is",
			SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
			DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
			DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=reject; rua=mailto:dmarc@healthy.is"},
			BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
		},
	}}})

	if report.HealthyDomains != 1 || report.DomainsToReview != 0 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
	if !strings.Contains(report.ExecutiveSummary, "expected trust baseline") {
		t.Fatalf("ExecutiveSummary = %q, want healthy summary", report.ExecutiveSummary)
	}
}
