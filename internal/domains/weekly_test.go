package domains

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestBuildWeeklySummary(t *testing.T) {
	t.Parallel()

	summary := BuildWeeklySummary([]Domain{
		{Domain: "healthy.example", LatestCheck: &Check{
			SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
			DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
			DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
			BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
		}},
		{Domain: "review.example", LatestCheck: &Check{
			SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
			DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "missing"},
			DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
			BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
		}},
		{Domain: "unchecked.example"},
	})

	if summary.TotalDomains != 3 || summary.Healthy != 1 || summary.NeedsReview != 2 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	text := summary.Text()
	for _, want := range []string{"Domains monitored: 3", "healthy.example", "review.example", "unchecked.example"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary text missing %q: %s", want, text)
		}
	}
}
