package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/brandrisk"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestNewScanResponse(t *testing.T) {
	t.Parallel()

	response := NewScanResponse(scanner.Result{
		Domain:    "example.com",
		CheckedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		Duration:  12 * time.Millisecond,
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok", SourceName: "example.com", LookupDuration: time.Millisecond},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	})

	if response.Domain != "example.com" || response.DurationMS != 12 || len(response.Checks) != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Summary.Score == 0 || response.Checks[0].LookupDurationMS != 1 {
		t.Fatalf("summary/check metadata missing: %+v", response)
	}
	if response.Audit.Grade != "C" || response.Audit.Risk == "" || response.Audit.CTA == "" {
		t.Fatalf("audit metadata missing: %+v", response.Audit)
	}
	if _, err := json.Marshal(response); err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
}

func TestNewDomainResponse(t *testing.T) {
	t.Parallel()

	response := NewDomainResponse(domains.Domain{
		ID:        "domain-1",
		Domain:    "example.com",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		LatestCheck: &domains.Check{
			CheckedAt: time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC),
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing},
		},
	})

	if response.ID != "domain-1" || response.Domain != "example.com" || response.LatestCheck == nil {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.LatestCheck.SPF.Status != scanner.StatusPass {
		t.Fatalf("latest check not mapped: %+v", response.LatestCheck)
	}
}

func TestNewDashboardBrandRiskResponse(t *testing.T) {
	t.Parallel()

	response := NewDashboardBrandRiskResponse(domains.Domain{
		ID:     "domain-1",
		Domain: "example.com",
		LatestCheck: &domains.Check{
			CheckedAt: time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC),
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
		},
	}, []brandrisk.LookalikeFinding{{
		Domain:            "exmple.com",
		Kind:              brandrisk.CandidateTypo,
		Reason:            "single-character deletion",
		HasAddressRecords: true,
		Severity:          brandrisk.SeverityMedium,
		Evidence:          "DNS evidence: A/AAAA records are published.",
		RecommendedAction: "Review ownership.",
	}}, time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC))

	if response.DomainID != "domain-1" || response.Domain != "example.com" {
		t.Fatalf("domain mapping mismatch: %+v", response)
	}
	if len(response.Findings) == 0 || response.Findings[0].Type != "dmarc_policy_relaxed" {
		t.Fatalf("scan findings missing: %+v", response.Findings)
	}
	if len(response.Lookalikes) != 1 || response.Lookalikes[0].Domain != "exmple.com" || response.Lookalikes[0].Severity != "Medium" {
		t.Fatalf("lookalike mapping mismatch: %+v", response.Lookalikes)
	}
	if _, err := json.Marshal(response); err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
}
