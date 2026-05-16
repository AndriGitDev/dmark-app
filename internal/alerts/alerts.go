package alerts

import (
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/drift"
)

type Rule struct {
	CheckName  string
	Kind       drift.Kind
	Minimum    drift.Severity
	MutedUntil time.Time
	Enabled    bool
}

type Decision struct {
	Send   bool
	Reason string
}

func ShouldNotify(event drift.Event, rule Rule, now time.Time) Decision {
	if !rule.Enabled {
		return Decision{Reason: "rule disabled"}
	}
	if rule.CheckName != "" && !strings.EqualFold(rule.CheckName, event.CheckName) {
		return Decision{Reason: "check does not match rule"}
	}
	if rule.Kind != "" && rule.Kind != event.Kind {
		return Decision{Reason: "kind does not match rule"}
	}
	if !rule.MutedUntil.IsZero() && now.Before(rule.MutedUntil) {
		return Decision{Reason: "rule muted"}
	}
	if severityRank(event.Severity) < severityRank(rule.Minimum) {
		return Decision{Reason: "below severity threshold"}
	}
	return Decision{Send: true, Reason: "matched"}
}

type EmailPayload struct {
	Subject string
	Text    string
}

func BuildEmailPayload(domain string, event drift.Event) EmailPayload {
	subject := "[" + strings.ToUpper(string(event.Severity)) + "] " + domain + " " + event.CheckName + " " + string(event.Kind)
	lines := []string{
		"Domain: " + domain,
		"Check: " + event.CheckName,
		"Severity: " + string(event.Severity),
		"Change: " + event.Message,
		"Before status: " + string(event.BeforeStatus),
		"After status: " + string(event.AfterStatus),
	}
	if event.BeforeRecord != "" {
		lines = append(lines, "Before record: "+event.BeforeRecord)
	}
	if event.AfterRecord != "" {
		lines = append(lines, "After record: "+event.AfterRecord)
	}
	lines = append(lines, "Recommended action: review the DNS change and restore the expected record if this was not intentional.")
	return EmailPayload{Subject: subject, Text: strings.Join(lines, "\n")}
}

func severityRank(severity drift.Severity) int {
	switch severity {
	case drift.SeverityCritical:
		return 3
	case drift.SeverityWarn:
		return 2
	case drift.SeverityInfo:
		return 1
	default:
		return 0
	}
}
