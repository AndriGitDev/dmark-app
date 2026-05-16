package brandrisk

import (
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestFromScanBuildsEvidenceBackedFindings(t *testing.T) {
	t.Parallel()

	findings := FromScan(scanner.Result{
		SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusWarn, Message: "SPF ends with neutral (?all).", Record: "v=spf1 ?all"},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusMissing, Message: "No DMARC record found."},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "No DKIM selectors found."},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "No BIMI record found."},
	})
	if len(findings) != 4 {
		t.Fatalf("len(findings) = %d, want 4", len(findings))
	}
	for _, finding := range findings {
		if finding.Evidence == "" || finding.RecommendedAction == "" {
			t.Fatalf("finding missing evidence/action: %+v", finding)
		}
	}
	if findings[0].Type != "spf_permissive" || findings[1].Type != "dmarc_missing" {
		t.Fatalf("unexpected finding order/types: %+v", findings)
	}
}

func TestFromScanAddsHealthyBaseline(t *testing.T) {
	t.Parallel()

	findings := FromScan(scanner.Result{
		SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF passes."},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC passes."},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM passes."},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass, Message: "BIMI passes."},
	})
	if len(findings) != 1 || findings[0].Type != "baseline_healthy" || findings[0].Severity != SeverityInfo {
		t.Fatalf("unexpected healthy findings: %+v", findings)
	}
}
