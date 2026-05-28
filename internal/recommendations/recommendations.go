package recommendations

import (
	"sort"
	"strconv"
	"strings"

	"github.com/AndriGitDev/meil/internal/dmarcjourney"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type Recommendation struct {
	Title    string
	Priority int
	Action   string
}

type ChangeStatus string

const (
	ChangeReady   ChangeStatus = "ready"
	ChangeReview  ChangeStatus = "review"
	ChangeWaiting ChangeStatus = "waiting"
	ChangeBlocked ChangeStatus = "blocked"
)

type DNSChange struct {
	ID           string
	Title        string
	Status       ChangeStatus
	RecordName   string
	RecordType   string
	RecordText   string
	Evidence     string
	Risk         string
	RollbackNote string
}

type ChangePlan struct {
	Domain       string
	GeneratedAt  string
	Changes      []DNSChange
	NextAction   string
	JourneyState dmarcjourney.State
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

func BuildChangePlan(result scanner.Result, evidence dmarcjourney.Evidence) []DNSChange {
	changes := []DNSChange{}
	if change, ok := spfChange(result.Domain, result.SPF); ok {
		changes = append(changes, change)
	}
	journey := dmarcjourney.Evaluate(result.DMARC, evidence)
	if change, ok := dmarcChange(result.Domain, result.DMARC, journey); ok {
		changes = append(changes, change)
	}
	if change, ok := dkimChange(result.Domain, result.DKIM); ok {
		changes = append(changes, change)
	}
	if change, ok := bimiChange(result.Domain, result.BIMI, journey); ok {
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i int, j int) bool {
		if changeStatusRank(changes[i].Status) == changeStatusRank(changes[j].Status) {
			return changes[i].ID < changes[j].ID
		}
		return changeStatusRank(changes[i].Status) > changeStatusRank(changes[j].Status)
	})
	return changes
}

func spfChange(domain string, check scanner.CheckResult) (DNSChange, bool) {
	if check.Record == "" && check.Status == scanner.StatusPass {
		return DNSChange{}, false
	}
	lookupCount := SPFLookupCount(check.Record)
	if lookupCount <= 10 && check.Status == scanner.StatusPass {
		return DNSChange{}, false
	}
	status := ChangeReview
	evidence := "Current SPF evidence requires review before changing DNS."
	if lookupCount > 10 {
		status = ChangeBlocked
		evidence = "Current SPF record uses " + strconv.Itoa(lookupCount) + " DNS lookups, above the 10 lookup receiver limit."
	}
	recordText := check.Record
	if recordText == "" {
		recordText = "v=spf1 include:_spf.example.net -all"
	}
	return DNSChange{
		ID:           "spf-flattening",
		Title:        "Review SPF lookup budget",
		Status:       status,
		RecordName:   domain,
		RecordType:   "TXT",
		RecordText:   recordText,
		Evidence:     evidence,
		Risk:         "Changing SPF can block legitimate senders if includes are removed without provider evidence.",
		RollbackNote: "Restore the previous SPF TXT record if legitimate mail starts failing SPF.",
	}, true
}

func dmarcChange(domain string, check scanner.CheckResult, journey dmarcjourney.Journey) (DNSChange, bool) {
	if check.Status == scanner.StatusPass && journey.State == dmarcjourney.StateReject {
		return DNSChange{}, false
	}
	recordName := "_dmarc." + domain
	status := ChangeWaiting
	recordText := check.Record
	if recordText == "" {
		recordText = "v=DMARC1; p=none; rua=mailto:dmarc@" + domain
	}
	evidence := journey.Reason
	switch journey.State {
	case dmarcjourney.StateNotStarted:
		status = ChangeReady
	case dmarcjourney.StateReadyForQuarantine:
		status = ChangeReview
		recordText = replaceDMARCPolicy(recordText, "quarantine")
	case dmarcjourney.StateReadyForReject:
		status = ChangeReview
		recordText = replaceDMARCPolicy(recordText, "reject")
	case dmarcjourney.StateObserving, dmarcjourney.StateQuarantine:
		status = ChangeWaiting
		if len(journey.BlockingSignals) > 0 {
			evidence = strings.Join(journey.BlockingSignals, " ")
		}
	default:
		status = ChangeReview
	}
	return DNSChange{
		ID:           "dmarc-enforcement-ladder",
		Title:        "Advance the DMARC enforcement ladder",
		Status:       status,
		RecordName:   recordName,
		RecordType:   "TXT",
		RecordText:   recordText,
		Evidence:     evidence,
		Risk:         "DMARC enforcement can reject legitimate mail if sender alignment evidence is incomplete.",
		RollbackNote: `Move the policy back to "p=none" or the previous policy while investigating failures.`,
	}, true
}

func dkimChange(domain string, check scanner.CheckResult) (DNSChange, bool) {
	if check.Status == scanner.StatusPass {
		return DNSChange{}, false
	}
	return DNSChange{
		ID:           "dkim-coverage",
		Title:        "Publish DKIM keys for active senders",
		Status:       ChangeReview,
		RecordName:   "selector._domainkey." + domain,
		RecordType:   "TXT",
		RecordText:   "v=DKIM1; k=rsa; p=PUBLICKEY",
		Evidence:     "Scanner evidence did not confirm DKIM coverage for the probed selectors.",
		Risk:         "DKIM selector names and keys must come from each sending provider.",
		RollbackNote: "Remove only the newly added selector record if the provider rejects it.",
	}, true
}

func bimiChange(domain string, check scanner.CheckResult, journey dmarcjourney.Journey) (DNSChange, bool) {
	if check.Status == scanner.StatusPass {
		return DNSChange{}, false
	}
	status := ChangeWaiting
	evidence := "BIMI should wait until DMARC reaches quarantine or reject enforcement."
	if journey.State == dmarcjourney.StateQuarantine || journey.State == dmarcjourney.StateReadyForReject || journey.State == dmarcjourney.StateReject {
		status = ChangeReview
		evidence = "Current DMARC enforcement evidence can support BIMI planning."
	}
	return DNSChange{
		ID:           "bimi-follow-up",
		Title:        "Prepare BIMI after enforcement",
		Status:       status,
		RecordName:   "default._bimi." + domain,
		RecordType:   "TXT",
		RecordText:   "v=BIMI1; l=https://" + domain + "/bimi.svg; a=",
		Evidence:     evidence,
		Risk:         "BIMI visibility depends on mailbox-provider support and DMARC enforcement.",
		RollbackNote: "Remove the BIMI TXT record if brand assets or certificates need correction.",
	}, true
}

func SPFLookupCount(record string) int {
	record = strings.ToLower(record)
	count := 0
	for _, token := range strings.Fields(record) {
		token = strings.TrimSpace(token)
		if strings.HasPrefix(token, "include:") || strings.HasPrefix(token, "exists:") || strings.HasPrefix(token, "redirect=") {
			count++
			continue
		}
		mechanism := strings.TrimLeft(token, "+-~?")
		if mechanism == "a" || strings.HasPrefix(mechanism, "a:") || mechanism == "mx" || strings.HasPrefix(mechanism, "mx:") || mechanism == "ptr" || strings.HasPrefix(mechanism, "ptr:") {
			count++
		}
	}
	return count
}

func replaceDMARCPolicy(record string, policy string) string {
	if strings.TrimSpace(record) == "" {
		return "v=DMARC1; p=" + policy
	}
	parts := strings.Split(record, ";")
	replaced := false
	for i, part := range parts {
		key, _, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "p") {
			parts[i] = " p=" + policy
			replaced = true
		}
	}
	if !replaced {
		parts = append(parts, " p="+policy)
	}
	return strings.TrimSpace(strings.Join(parts, ";"))
}

func changeStatusRank(status ChangeStatus) int {
	switch status {
	case ChangeReady:
		return 4
	case ChangeReview:
		return 3
	case ChangeWaiting:
		return 2
	case ChangeBlocked:
		return 1
	default:
		return 0
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
