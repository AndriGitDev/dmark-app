package brandrisk

import (
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type Severity string

const (
	SeverityInfo     Severity = "Info"
	SeverityLow      Severity = "Low"
	SeverityMedium   Severity = "Medium"
	SeverityHigh     Severity = "High"
	SeverityCritical Severity = "Critical"
)

type Finding struct {
	Category          string
	Type              string
	Title             string
	Severity          Severity
	Evidence          string
	RecommendedAction string
}

func FromScan(result scanner.Result) []Finding {
	findings := []Finding{}
	findings = append(findings, spfFindings(result.SPF)...)
	findings = append(findings, dmarcFindings(result.DMARC)...)
	findings = append(findings, dkimFindings(result.DKIM)...)
	findings = append(findings, bimiFindings(result.BIMI)...)
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Category:          "Monitoring coverage",
			Type:              "baseline_healthy",
			Title:             "Current DNS trust baseline is healthy",
			Severity:          SeverityInfo,
			Evidence:          "SPF, DKIM, DMARC, and BIMI checks did not produce a brand-risk finding in the latest scan.",
			RecommendedAction: "Keep scheduled monitoring enabled so future DNS drift creates a visible trust timeline.",
		})
	}
	return findings
}

func spfFindings(check scanner.CheckResult) []Finding {
	switch check.Status {
	case scanner.StatusMissing:
		return []Finding{{
			Category:          "Email spoofing",
			Type:              "spf_missing",
			Title:             "SPF record is missing",
			Severity:          SeverityHigh,
			Evidence:          evidence(check),
			RecommendedAction: "Publish exactly one SPF record that lists authorized senders and ends with a clear all mechanism.",
		}}
	case scanner.StatusFail:
		return []Finding{{
			Category:          "Email spoofing",
			Type:              "spf_broken",
			Title:             "SPF record is broken",
			Severity:          SeverityHigh,
			Evidence:          evidence(check),
			RecommendedAction: "Fix the SPF syntax, duplicate record, lookup limit, or unsafe mechanism before relying on SPF authorization.",
		}}
	case scanner.StatusWarn:
		findingType := "spf_review"
		title := "SPF record needs review"
		if strings.Contains(check.Message, "soft-fail") || strings.Contains(check.Message, "?all") || strings.Contains(check.Message, "+all") {
			findingType = "spf_permissive"
			title = "SPF record is too permissive"
		}
		return []Finding{{
			Category:          "Email spoofing",
			Type:              findingType,
			Title:             title,
			Severity:          SeverityMedium,
			Evidence:          evidence(check),
			RecommendedAction: "Review authorized senders and remove broad or unsafe mechanisms where possible.",
		}}
	default:
		return nil
	}
}

func dmarcFindings(check scanner.CheckResult) []Finding {
	switch check.Status {
	case scanner.StatusMissing:
		return []Finding{{
			Category:          "Domain trust",
			Type:              "dmarc_missing",
			Title:             "DMARC record is missing",
			Severity:          SeverityCritical,
			Evidence:          evidence(check),
			RecommendedAction: "Publish a DMARC record with reporting enabled, then move toward quarantine or reject after legitimate senders are verified.",
		}}
	case scanner.StatusFail:
		return []Finding{{
			Category:          "Domain trust",
			Type:              "dmarc_broken",
			Title:             "DMARC record is invalid",
			Severity:          SeverityHigh,
			Evidence:          evidence(check),
			RecommendedAction: "Fix the DMARC TXT record so receivers can apply policy and send aggregate reports.",
		}}
	case scanner.StatusWarn:
		return []Finding{{
			Category:          "Domain trust",
			Type:              "dmarc_policy_relaxed",
			Title:             "DMARC policy is relaxed",
			Severity:          SeverityMedium,
			Evidence:          evidence(check),
			RecommendedAction: "Move toward DMARC enforcement once legitimate senders are verified.",
		}}
	default:
		return nil
	}
}

func dkimFindings(check scanner.CheckResult) []Finding {
	if check.Status != scanner.StatusMissing && check.Status != scanner.StatusFail {
		return nil
	}
	return []Finding{{
		Category:          "Sender identity",
		Type:              "dkim_selector_missing",
		Title:             "DKIM selector evidence is missing",
		Severity:          SeverityMedium,
		Evidence:          evidence(check),
		RecommendedAction: "Confirm the selectors used by active sending providers and publish DKIM keys for each expected selector.",
	}}
}

func bimiFindings(check scanner.CheckResult) []Finding {
	if check.Status != scanner.StatusMissing && check.Status != scanner.StatusFail {
		return nil
	}
	return []Finding{{
		Category:          "Brand indicators",
		Type:              "bimi_missing",
		Title:             "BIMI brand indicator is not active",
		Severity:          SeverityLow,
		Evidence:          evidence(check),
		RecommendedAction: "Treat BIMI as a follow-up after DMARC enforcement, then publish a valid logo and certificate URI where supported.",
	}}
}

func evidence(check scanner.CheckResult) string {
	if strings.TrimSpace(check.Record) != "" {
		return check.Name + ": " + check.Message + " Record: " + check.Record
	}
	return check.Name + ": " + check.Message
}
