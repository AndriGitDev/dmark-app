package api

import (
	"encoding/json"
	"testing"
	"time"

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
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC ok"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	})

	if response.Domain != "example.com" || response.DurationMS != 12 || len(response.Checks) != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Summary.Score == 0 || response.Checks[0].LookupDurationMS != 1 {
		t.Fatalf("summary/check metadata missing: %+v", response)
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
