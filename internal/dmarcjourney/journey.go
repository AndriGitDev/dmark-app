package dmarcjourney

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type State string

const (
	StateNotStarted         State = "not_started"
	StateObserving          State = "observing"
	StateReadyForQuarantine State = "ready_for_quarantine"
	StateQuarantine         State = "quarantine"
	StateReadyForReject     State = "ready_for_reject"
	StateReject             State = "reject"
)

type Evidence struct {
	ReporterConfigured    bool
	DaysObserved          int
	TotalMessages         int
	AlignedMessages       int
	FailedMessages        int
	UnknownSenderMessages int
}

type Journey struct {
	State           State
	Policy          string
	Percent         int
	ReadyForNext    bool
	NextAction      string
	Reason          string
	BlockingSignals []string
}

func Evaluate(check scanner.CheckResult, evidence Evidence) Journey {
	policy, pct := parsePolicy(check.Record)
	if pct == 0 {
		pct = 100
	}
	journey := Journey{Policy: policy, Percent: pct}
	if check.Status == scanner.StatusMissing || check.Status == scanner.StatusFail || policy == "" {
		journey.State = StateNotStarted
		journey.NextAction = `Publish a valid DMARC record with reporting enabled, starting with "v=DMARC1; p=none; rua=mailto:...".`
		journey.Reason = "DMARC is missing or invalid, so receivers cannot reliably report or enforce policy."
		return journey
	}

	blockers := readinessBlockers(evidence)
	journey.BlockingSignals = blockers
	ready := len(blockers) == 0
	switch policy {
	case "reject":
		journey.State = StateReject
		journey.ReadyForNext = false
		journey.NextAction = "Keep monitoring sender drift, report health, and unexpected failures."
		journey.Reason = "DMARC is already at reject enforcement."
	case "quarantine":
		if ready {
			journey.State = StateReadyForReject
			journey.ReadyForNext = true
			journey.NextAction = `Move the DMARC policy to "p=reject" after notifying mail owners and watching the next reporting window.`
			journey.Reason = "Reporter evidence shows enough aligned traffic and low unknown-sender risk for the next enforcement step."
		} else {
			journey.State = StateQuarantine
			journey.NextAction = "Keep quarantine in place and resolve the blocking sender evidence before moving to reject."
			journey.Reason = "DMARC is enforcing quarantine, but the evidence is not clean enough for reject."
		}
	default:
		if ready {
			journey.State = StateReadyForQuarantine
			journey.ReadyForNext = true
			journey.NextAction = `Move the DMARC policy to "p=quarantine" with pct=100 after confirming the listed senders.`
			journey.Reason = "Reporter evidence is sufficient to begin enforcement."
		} else {
			journey.State = StateObserving
			journey.NextAction = "Keep p=none reporting enabled until legitimate senders are identified and alignment failures are low."
			journey.Reason = "DMARC reporting is present, but more evidence or cleanup is needed before enforcement."
		}
	}
	return journey
}

func readinessBlockers(evidence Evidence) []string {
	blockers := []string{}
	if !evidence.ReporterConfigured {
		blockers = append(blockers, "DMARC aggregate reporting is not configured.")
	}
	if evidence.DaysObserved < 7 {
		blockers = append(blockers, "Fewer than 7 days of aggregate reports have been observed.")
	}
	if evidence.TotalMessages <= 0 {
		blockers = append(blockers, "No aggregate-report message volume has been observed.")
	}
	if evidence.TotalMessages > 0 {
		failedRate := float64(evidence.FailedMessages) / float64(evidence.TotalMessages)
		unknownRate := float64(evidence.UnknownSenderMessages) / float64(evidence.TotalMessages)
		if failedRate > 0.02 {
			blockers = append(blockers, fmt.Sprintf("Alignment failure rate is %.1f%%, above the 2.0%% readiness threshold.", failedRate*100))
		}
		if unknownRate > 0.01 {
			blockers = append(blockers, fmt.Sprintf("Unknown sender volume is %.1f%%, above the 1.0%% readiness threshold.", unknownRate*100))
		}
	}
	sort.Strings(blockers)
	return blockers
}

func parsePolicy(record string) (string, int) {
	policy := ""
	pct := 100
	for _, part := range strings.Split(record, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "p":
			policy = strings.ToLower(strings.TrimSpace(value))
		case "pct":
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed >= 0 && parsed <= 100 {
				pct = parsed
			}
		}
	}
	return policy, pct
}
