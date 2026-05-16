package drift

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type Kind string

const (
	KindStatusRegression Kind = "status_regression"
	KindStatusRecovery   Kind = "status_recovery"
	KindRecordMissing    Kind = "record_missing"
	KindPolicyRelaxed    Kind = "policy_relaxed"
	KindIncludeRemoved   Kind = "spf_include_removed"
	KindDKIMRemoved      Kind = "dkim_selector_removed"
	KindBIMIRemoved      Kind = "bimi_removed"
	KindLookupLimit      Kind = "spf_lookup_limit_exceeded"
	KindMaterialRecord   Kind = "material_record_changed"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityCritical Severity = "critical"
)

type Event struct {
	CheckName    string
	Kind         Kind
	Severity     Severity
	Message      string
	BeforeStatus scanner.Status
	AfterStatus  scanner.Status
	BeforeRecord string
	AfterRecord  string
}

func Compare(before scanner.Result, after scanner.Result) []Event {
	events := []Event{}
	for _, pair := range []struct {
		name   string
		before scanner.CheckResult
		after  scanner.CheckResult
	}{
		{name: "SPF", before: before.SPF, after: after.SPF},
		{name: "DKIM", before: before.DKIM, after: after.DKIM},
		{name: "DMARC", before: before.DMARC, after: after.DMARC},
		{name: "BIMI", before: before.BIMI, after: after.BIMI},
	} {
		events = append(events, compareCheck(pair.name, pair.before, pair.after)...)
	}
	events = append(events, compareSPF(before.SPF, after.SPF)...)
	events = append(events, compareDMARC(before.DMARC, after.DMARC)...)
	events = append(events, compareDKIM(before.DKIM, after.DKIM)...)
	events = append(events, compareBIMI(before.BIMI, after.BIMI)...)
	return dedupe(events)
}

func compareCheck(name string, before scanner.CheckResult, after scanner.CheckResult) []Event {
	events := []Event{}
	if before.Status != "" && after.Status != "" {
		if statusRank(after.Status) < statusRank(before.Status) {
			events = append(events, newEvent(name, KindStatusRegression, severityForStatus(after.Status), fmt.Sprintf("%s regressed from %s to %s.", name, before.Status, after.Status), before, after))
		}
		if statusRank(before.Status) < statusRank(after.Status) && after.Status == scanner.StatusPass {
			events = append(events, newEvent(name, KindStatusRecovery, SeverityInfo, fmt.Sprintf("%s recovered from %s to pass.", name, before.Status), before, after))
		}
	}
	if before.Record != "" && after.Status == scanner.StatusMissing {
		events = append(events, newEvent(name, KindRecordMissing, SeverityCritical, fmt.Sprintf("%s record is no longer published.", name), before, after))
	}
	if before.Record != "" && after.Record != "" && normalizeRecord(before.Record) != normalizeRecord(after.Record) {
		events = append(events, newEvent(name, KindMaterialRecord, SeverityWarn, fmt.Sprintf("%s record changed.", name), before, after))
	}
	return events
}

func compareSPF(before scanner.CheckResult, after scanner.CheckResult) []Event {
	events := []Event{}
	for _, include := range removed(spfIncludes(before.Record), spfIncludes(after.Record)) {
		events = append(events, newEvent("SPF", KindIncludeRemoved, SeverityWarn, "SPF include removed: "+include+".", before, after))
	}
	if spfLookupCount(after.Record) > 10 {
		events = append(events, newEvent("SPF", KindLookupLimit, SeverityCritical, "SPF DNS lookup count now exceeds the receiver limit of 10.", before, after))
	}
	if spfPolicyRank(after.Record) < spfPolicyRank(before.Record) {
		events = append(events, newEvent("SPF", KindPolicyRelaxed, SeverityCritical, "SPF terminal policy was relaxed.", before, after))
	}
	return events
}

func compareDMARC(before scanner.CheckResult, after scanner.CheckResult) []Event {
	if dmarcPolicyRank(after.Record) < dmarcPolicyRank(before.Record) {
		return []Event{newEvent("DMARC", KindPolicyRelaxed, SeverityCritical, "DMARC policy was relaxed.", before, after)}
	}
	return nil
}

func compareDKIM(before scanner.CheckResult, after scanner.CheckResult) []Event {
	beforeSelectors := passingSelectors(before.Selectors)
	afterSelectors := passingSelectors(after.Selectors)
	events := []Event{}
	for _, selector := range removed(beforeSelectors, afterSelectors) {
		events = append(events, newEvent("DKIM", KindDKIMRemoved, SeverityCritical, "DKIM selector removed: "+selector+".", before, after))
	}
	return events
}

func compareBIMI(before scanner.CheckResult, after scanner.CheckResult) []Event {
	if before.Record != "" && after.Status == scanner.StatusMissing {
		return []Event{newEvent("BIMI", KindBIMIRemoved, SeverityWarn, "BIMI record was removed.", before, after)}
	}
	return nil
}

func newEvent(name string, kind Kind, severity Severity, message string, before scanner.CheckResult, after scanner.CheckResult) Event {
	return Event{
		CheckName:    name,
		Kind:         kind,
		Severity:     severity,
		Message:      message,
		BeforeStatus: before.Status,
		AfterStatus:  after.Status,
		BeforeRecord: before.Record,
		AfterRecord:  after.Record,
	}
}

func statusRank(status scanner.Status) int {
	switch status {
	case scanner.StatusMissing:
		return 0
	case scanner.StatusFail:
		return 1
	case scanner.StatusUnknown:
		return 2
	case scanner.StatusWarn:
		return 3
	case scanner.StatusPass:
		return 4
	default:
		return 2
	}
}

func severityForStatus(status scanner.Status) Severity {
	switch status {
	case scanner.StatusMissing, scanner.StatusFail:
		return SeverityCritical
	case scanner.StatusWarn, scanner.StatusUnknown:
		return SeverityWarn
	default:
		return SeverityInfo
	}
}

func normalizeRecord(record string) string {
	return strings.Join(strings.Fields(strings.ToLower(record)), " ")
}

func spfIncludes(record string) []string {
	includes := []string{}
	for _, field := range strings.Fields(strings.ToLower(record)) {
		if include, ok := strings.CutPrefix(field, "include:"); ok {
			includes = append(includes, include)
		}
	}
	sort.Strings(includes)
	return includes
}

func spfLookupCount(record string) int {
	count := 0
	for _, field := range strings.Fields(strings.ToLower(record)) {
		name := strings.TrimLeft(field, "+-~?")
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = before
		}
		if before, _, ok := strings.Cut(name, "/"); ok {
			name = before
		}
		switch name {
		case "include", "a", "mx", "ptr", "exists":
			count++
		default:
			if strings.HasPrefix(name, "redirect=") || strings.HasPrefix(name, "exp=") {
				count++
			}
		}
	}
	return count
}

func spfPolicyRank(record string) int {
	for _, field := range strings.Fields(strings.ToLower(record)) {
		switch field {
		case "-all":
			return 4
		case "~all":
			return 3
		case "?all":
			return 2
		case "+all":
			return 1
		case "all":
			return 1
		}
	}
	if strings.Contains(strings.ToLower(record), "redirect=") {
		return 3
	}
	return 0
}

func dmarcPolicyRank(record string) int {
	for _, part := range strings.Split(strings.ToLower(record), ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || key != "p" {
			continue
		}
		switch strings.TrimSpace(value) {
		case "reject":
			return 3
		case "quarantine":
			return 2
		case "none":
			return 1
		}
	}
	return 0
}

func passingSelectors(selectors []scanner.SelectorResult) []string {
	values := []string{}
	for _, selector := range selectors {
		if selector.Status == scanner.StatusPass {
			values = append(values, selector.Selector)
		}
	}
	sort.Strings(values)
	return values
}

func removed(before []string, after []string) []string {
	afterSet := map[string]struct{}{}
	for _, value := range after {
		afterSet[value] = struct{}{}
	}
	missing := []string{}
	for _, value := range before {
		if _, ok := afterSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func dedupe(events []Event) []Event {
	seen := map[string]struct{}{}
	unique := []Event{}
	for _, event := range events {
		key := event.CheckName + "\x00" + string(event.Kind) + "\x00" + event.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, event)
	}
	return unique
}
