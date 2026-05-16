package notifier

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AndriGitDev/meil/internal/drift"
)

type DigestItem struct {
	Domain string
	Event  drift.Event
}

type Digest struct {
	Recipient string
	Items     []DigestItem
}

func BuildDigestEmail(digest Digest) Email {
	items := append([]DigestItem(nil), digest.Items...)
	sort.Slice(items, func(i int, j int) bool {
		if severityRank(items[i].Event.Severity) == severityRank(items[j].Event.Severity) {
			if items[i].Domain == items[j].Domain {
				return items[i].Event.CheckName < items[j].Event.CheckName
			}
			return items[i].Domain < items[j].Domain
		}
		return severityRank(items[i].Event.Severity) > severityRank(items[j].Event.Severity)
	})
	lines := []string{"Meil digest", ""}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- [%s] %s %s: %s", strings.ToUpper(string(item.Event.Severity)), item.Domain, item.Event.CheckName, item.Event.Message))
	}
	if len(items) == 0 {
		lines = append(lines, "No drift events in this digest window.")
	}
	subject := fmt.Sprintf("Meil digest: %d event(s)", len(items))
	return Email{To: digest.Recipient, Subject: subject, Text: strings.Join(lines, "\n")}
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
