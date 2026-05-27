package salesaudit

import (
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestAssessGradesProtectedRejectDomain(t *testing.T) {
	t.Parallel()

	result := Assess(scanner.Result{
		Domain: "example.is",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=reject; rua=mailto:dmarc@example.is"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	}, Options{RequestedEmail: "admin@example.is"})

	if result.Grade != "A+" || result.Risk != RiskLow || !result.Safe || result.ProofEligible {
		t.Fatalf("result = %+v, want protected A+ without proof eligibility", result)
	}
}

func TestAssessMarksVulnerableDomainProofEligibleAfterMatchingEmail(t *testing.T) {
	t.Parallel()

	result := Assess(scanner.Result{
		Domain: "example.is",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Record: "v=DMARC1; p=none; rua=mailto:dmarc@example.is"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing},
	}, Options{RequestedEmail: "security@example.is"})

	if result.Grade != "C" || result.Risk != RiskHigh || result.Safe || !result.ProofEligible {
		t.Fatalf("result = %+v, want vulnerable proof-eligible C", result)
	}
}

func TestAssessBlocksProofWithoutMatchingEmail(t *testing.T) {
	t.Parallel()

	result := Assess(scanner.Result{
		Domain: "example.is",
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusMissing},
	}, Options{RequestedEmail: "security@other.is"})

	if result.ProofEligible || result.BlockedReason == "" {
		t.Fatalf("result = %+v, want blocked proof", result)
	}
}

func TestAssessBlocksConsumerMailboxDomains(t *testing.T) {
	t.Parallel()

	result := Assess(scanner.Result{
		Domain: "gmail.com",
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusMissing},
	}, Options{RequestedEmail: "security@gmail.com"})

	if result.ProofEligible || result.BlockedReason != "domain is blocked from proof dispatch" {
		t.Fatalf("result = %+v, want consumer mailbox block", result)
	}
}

func TestNewClaimCapturesAuditResult(t *testing.T) {
	t.Parallel()

	audit := Result{Grade: "C", Risk: RiskHigh, ProofEligible: true}
	claim, err := NewClaim("Example.IS", "SECURITY@Example.IS", audit, time.Date(2026, 5, 27, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewClaim returned error: %v", err)
	}
	if claim.Domain != "example.is" || claim.Email != "security@example.is" || claim.Grade != "C" || claim.Risk != RiskHigh || !claim.ProofEligible || claim.Status != ClaimMagicLinkSent {
		t.Fatalf("claim = %+v, want normalized audit claim", claim)
	}
}
