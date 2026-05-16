package domains

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type WeeklySummary struct {
	TotalDomains int
	Healthy      int
	NeedsReview  int
	Lines        []string
}

func BuildWeeklySummary(saved []Domain) WeeklySummary {
	sorted := append([]Domain(nil), saved...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].Domain < sorted[j].Domain
	})
	summary := WeeklySummary{TotalDomains: len(sorted)}
	for _, domain := range sorted {
		if domain.LatestCheck == nil {
			summary.NeedsReview++
			summary.Lines = append(summary.Lines, domain.Domain+": no saved check yet")
			continue
		}
		posture := scanner.Summarize(scanner.Result{
			Domain: domain.Domain,
			SPF:    domain.LatestCheck.SPF,
			DKIM:   domain.LatestCheck.DKIM,
			DMARC:  domain.LatestCheck.DMARC,
			BIMI:   domain.LatestCheck.BIMI,
		})
		if posture.Status == scanner.StatusPass {
			summary.Healthy++
		} else {
			summary.NeedsReview++
		}
		summary.Lines = append(summary.Lines, fmt.Sprintf("%s: %d %s - %s", domain.Domain, posture.Score, posture.Status, posture.TopIssue))
	}
	return summary
}

func (summary WeeklySummary) Text() string {
	lines := []string{
		fmt.Sprintf("Domains monitored: %d", summary.TotalDomains),
		fmt.Sprintf("Healthy: %d", summary.Healthy),
		fmt.Sprintf("Needs review: %d", summary.NeedsReview),
		"",
	}
	lines = append(lines, summary.Lines...)
	return strings.Join(lines, "\n")
}
