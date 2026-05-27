package trustreport

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/brandrisk"
	"github.com/AndriGitDev/meil/internal/dmarcjourney"
	"github.com/AndriGitDev/meil/internal/recommendations"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type Input struct {
	GeneratedAt time.Time
	Customer    string
	Domains     []DomainInput
}

type DomainInput struct {
	Domain          string
	Scan            scanner.Result
	DMARCEvidence   dmarcjourney.Evidence
	SenderInventory []reporter.InventoryEntry
}

type Report struct {
	GeneratedAt      time.Time
	Customer         string
	TotalDomains     int
	HealthyDomains   int
	DomainsToReview  int
	ExecutiveSummary string
	Domains          []DomainSection
}

type DomainSection struct {
	Domain             string
	PostureScore       int
	PostureStatus      scanner.Status
	TopIssue           string
	DMARCJourney       dmarcjourney.Journey
	Findings           []brandrisk.Finding
	Recommendations    []recommendations.Recommendation
	SenderRiskCounts   map[string]int
	UnknownSenderCount int
}

func Build(input Input) Report {
	report := Report{
		GeneratedAt:  input.GeneratedAt,
		Customer:     strings.TrimSpace(input.Customer),
		TotalDomains: len(input.Domains),
	}
	for _, domain := range sortedDomains(input.Domains) {
		section := buildDomainSection(domain)
		if section.PostureStatus == scanner.StatusPass && !section.DMARCJourney.ReadyForNext && section.DMARCJourney.State == dmarcjourney.StateReject {
			report.HealthyDomains++
		} else {
			report.DomainsToReview++
		}
		report.Domains = append(report.Domains, section)
	}
	report.ExecutiveSummary = executiveSummary(report)
	return report
}

func buildDomainSection(input DomainInput) DomainSection {
	scan := input.Scan
	if scan.Domain == "" {
		scan.Domain = input.Domain
	}
	posture := scanner.Summarize(scan)
	section := DomainSection{
		Domain:             scan.Domain,
		PostureScore:       posture.Score,
		PostureStatus:      posture.Status,
		TopIssue:           posture.TopIssue,
		DMARCJourney:       dmarcjourney.Evaluate(scan.DMARC, input.DMARCEvidence),
		Findings:           brandrisk.FromScan(scan),
		Recommendations:    recommendations.ForPosture(scan),
		SenderRiskCounts:   map[string]int{},
		UnknownSenderCount: 0,
	}
	for _, entry := range input.SenderInventory {
		risk := reporter.InventoryRisk(reporter.ToSenderInventoryItem(entry))
		section.SenderRiskCounts[risk]++
		if risk == "review" && entry.Provider == "" {
			section.UnknownSenderCount++
		}
		if risk == "high" || risk == "review" {
			section.Recommendations = append(section.Recommendations, recommendations.ForSender(reporter.ToSenderInventoryItem(entry)))
		}
	}
	sort.Slice(section.Recommendations, func(i int, j int) bool {
		if section.Recommendations[i].Priority == section.Recommendations[j].Priority {
			return section.Recommendations[i].Title < section.Recommendations[j].Title
		}
		return section.Recommendations[i].Priority > section.Recommendations[j].Priority
	})
	return section
}

func executiveSummary(report Report) string {
	if report.TotalDomains == 0 {
		return "No protected domains were included in this trust report."
	}
	if report.DomainsToReview == 0 {
		return fmt.Sprintf("%d protected domain(s) are at the expected trust baseline. Keep monitoring for DNS drift and new sender evidence.", report.TotalDomains)
	}
	return fmt.Sprintf("%d of %d protected domain(s) need review before the next DMARC enforcement step.", report.DomainsToReview, report.TotalDomains)
}

func sortedDomains(domains []DomainInput) []DomainInput {
	sorted := append([]DomainInput(nil), domains...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].Domain < sorted[j].Domain
	})
	return sorted
}
