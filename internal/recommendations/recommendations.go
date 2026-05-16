package recommendations

import (
	"sort"

	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type Recommendation struct {
	Title    string
	Priority int
	Action   string
}

func ForPosture(result scanner.Result) []Recommendation {
	recommendations := []Recommendation{}
	for _, check := range []scanner.CheckResult{result.SPF, result.DKIM, result.DMARC, result.BIMI} {
		if check.Status == scanner.StatusPass {
			continue
		}
		recommendations = append(recommendations, Recommendation{
			Title:    check.Name + " needs attention",
			Priority: priorityForStatus(check.Status),
			Action:   scanner.Summarize(scanner.Result{SPF: check}).Remediation,
		})
	}
	sortRecommendations(recommendations)
	return recommendations
}

func ForSender(item reporter.SenderInventoryItem) Recommendation {
	switch reporter.InventoryRisk(item) {
	case "high":
		return Recommendation{Title: "Investigate suspicious sender", Priority: 100, Action: "Review source evidence, block unauthorized mail, or mark the sender expected if legitimate."}
	case "review":
		return Recommendation{Title: "Review unclassified sender", Priority: 60, Action: "Confirm whether this sender belongs to an approved email service and add it to expected senders if legitimate."}
	case "expected":
		return Recommendation{Title: "Expected sender healthy", Priority: 10, Action: "No action required; continue monitoring aggregate reports."}
	default:
		return Recommendation{Title: "Monitor sender", Priority: 25, Action: "Keep watching volume and failure rate for changes."}
	}
}

func ForDrift(event drift.Event) Recommendation {
	return Recommendation{
		Title:    event.CheckName + " drift: " + string(event.Kind),
		Priority: priorityForSeverity(event.Severity),
		Action:   "Review the before and after records, then restore the expected DNS posture or acknowledge the intentional change.",
	}
}

func priorityForStatus(status scanner.Status) int {
	switch status {
	case scanner.StatusFail:
		return 100
	case scanner.StatusMissing:
		return 90
	case scanner.StatusUnknown:
		return 50
	case scanner.StatusWarn:
		return 40
	default:
		return 0
	}
}

func priorityForSeverity(severity drift.Severity) int {
	switch severity {
	case drift.SeverityCritical:
		return 100
	case drift.SeverityWarn:
		return 60
	case drift.SeverityInfo:
		return 20
	default:
		return 0
	}
}

func sortRecommendations(recommendations []Recommendation) {
	sort.Slice(recommendations, func(i int, j int) bool {
		if recommendations[i].Priority == recommendations[j].Priority {
			return recommendations[i].Title < recommendations[j].Title
		}
		return recommendations[i].Priority > recommendations[j].Priority
	})
}
