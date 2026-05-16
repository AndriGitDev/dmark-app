package scanner

import (
	"strings"
	"testing"
)

func TestSummarizeHealthyPosture(t *testing.T) {
	t.Parallel()

	summary := Summarize(Result{
		SPF:   CheckResult{Name: "SPF", Status: StatusPass},
		DKIM:  CheckResult{Name: "DKIM", Status: StatusPass},
		DMARC: CheckResult{Name: "DMARC", Status: StatusPass},
		BIMI:  CheckResult{Name: "BIMI", Status: StatusPass},
	})
	if summary.Score != 100 || summary.Status != StatusPass {
		t.Fatalf("summary = %+v, want healthy score", summary)
	}
}

func TestSummarizePrioritizesHighestIssue(t *testing.T) {
	t.Parallel()

	summary := Summarize(Result{
		SPF:   CheckResult{Name: "SPF", Status: StatusWarn, Message: "soft fail"},
		DKIM:  CheckResult{Name: "DKIM", Status: StatusMissing, Message: "missing"},
		DMARC: CheckResult{Name: "DMARC", Status: StatusFail, Message: "invalid policy"},
		BIMI:  CheckResult{Name: "BIMI", Status: StatusPass},
	})
	if summary.Status != StatusFail || !strings.Contains(summary.TopIssue, "DMARC") || !strings.Contains(summary.Remediation, "p=reject") {
		t.Fatalf("summary = %+v, want DMARC fail as top issue", summary)
	}
}
