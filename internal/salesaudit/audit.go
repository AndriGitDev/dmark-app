package salesaudit

import (
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Signal struct {
	Name    string
	Status  scanner.Status
	Message string
	Record  string
}

type Options struct {
	RequestedEmail string
}

type Result struct {
	Domain        string
	Grade         string
	Risk          RiskLevel
	Safe          bool
	ProofEligible bool
	Message       string
	CTA           string
	Signals       []Signal
	BlockedReason string
}

func Assess(scan scanner.Result, options Options) Result {
	policy := dmarcPolicy(scan.DMARC.Record)
	spfOK := scan.SPF.Status == scanner.StatusPass || scan.SPF.Status == scanner.StatusWarn
	dkimOK := scan.DKIM.Status == scanner.StatusPass || scan.DKIM.Status == scanner.StatusWarn
	dmarcOK := scan.DMARC.Status == scanner.StatusPass || scan.DMARC.Status == scanner.StatusWarn
	safe := policy == "quarantine" || policy == "reject"
	grade := gradeFor(policy, spfOK, dkimOK, dmarcOK, scan)
	risk := riskFor(grade, safe)
	blockedReason := proofBlocker(scan.Domain, options.RequestedEmail)
	proofEligible := !safe && blockedReason == ""
	message, cta := copyFor(safe, proofEligible, policy)
	return Result{
		Domain:        scan.Domain,
		Grade:         grade,
		Risk:          risk,
		Safe:          safe,
		ProofEligible: proofEligible,
		Message:       message,
		CTA:           cta,
		Signals: []Signal{
			newSignal(scan.SPF),
			newSignal(scan.DKIM),
			newSignal(scan.DMARC),
			newSignal(scan.BIMI),
		},
		BlockedReason: blockedReason,
	}
}

func gradeFor(policy string, spfOK bool, dkimOK bool, dmarcOK bool, scan scanner.Result) string {
	if policy == "reject" && spfOK && dkimOK {
		return "A+"
	}
	if policy == "reject" || (policy == "quarantine" && spfOK && dkimOK) {
		return "A"
	}
	if policy == "quarantine" || (policy == "none" && spfOK && dkimOK) {
		return "B"
	}
	if dmarcOK && spfOK {
		return "C"
	}
	if dmarcOK || spfOK || scan.DKIM.Status == scanner.StatusPass {
		return "D"
	}
	return "F"
}

func riskFor(grade string, safe bool) RiskLevel {
	if safe && (grade == "A+" || grade == "A") {
		return RiskLow
	}
	switch grade {
	case "A+", "A":
		return RiskLow
	case "B":
		return RiskMedium
	case "C", "D":
		return RiskHigh
	default:
		return RiskCritical
	}
}

func copyFor(safe bool, proofEligible bool, policy string) (string, string) {
	if safe {
		return "This domain publishes an enforcing DMARC policy. Keep monitoring for sender drift and DNS changes.", "Protect this domain with continuous monitoring"
	}
	if proofEligible {
		return "This domain does not publish an enforcing DMARC policy, so forged mail may be accepted by receivers.", "Verify your work email and claim the full trust audit"
	}
	if policy == "" {
		return "This domain does not have a usable DMARC policy yet.", "Start concierge DMARC setup"
	}
	return "This domain is not ready for DMARC enforcement yet.", "Review sender evidence and enforcement steps"
}

func newSignal(check scanner.CheckResult) Signal {
	return Signal{
		Name:    check.Name,
		Status:  check.Status,
		Message: check.Message,
		Record:  check.Record,
	}
}

func dmarcPolicy(record string) string {
	for _, part := range strings.Split(record, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "p") {
			return strings.ToLower(strings.TrimSpace(value))
		}
	}
	return ""
}

func proofBlocker(domain string, requestedEmail string) string {
	normalizedDomain, err := scanner.NormalizeDomain(domain)
	if err != nil {
		return "domain is invalid"
	}
	if blockedDomain(normalizedDomain) {
		return "domain is blocked from proof dispatch"
	}
	if strings.TrimSpace(requestedEmail) == "" {
		return "work email verification is required"
	}
	parts := strings.Split(strings.TrimSpace(strings.ToLower(requestedEmail)), "@")
	if len(parts) != 2 {
		return "work email is invalid"
	}
	emailDomain, err := scanner.NormalizeDomain(parts[1])
	if err != nil {
		return "work email domain is invalid"
	}
	if emailDomain != normalizedDomain {
		return "work email domain must match the audited domain"
	}
	return ""
}

func blockedDomain(domain string) bool {
	blockedExact := map[string]struct{}{
		"gmail.com":       {},
		"google.com":      {},
		"outlook.com":     {},
		"hotmail.com":     {},
		"icloud.com":      {},
		"proton.me":       {},
		"protonmail.com":  {},
		"microsoft.com":   {},
		"apple.com":       {},
		"cloudflare.com":  {},
		"amazon.com":      {},
		"paypal.com":      {},
		"resend.com":      {},
		"sendgrid.com":    {},
		"mailgun.com":     {},
		"postmarkapp.com": {},
	}
	if _, ok := blockedExact[domain]; ok {
		return true
	}
	for _, suffix := range []string{".gov", ".mil", ".edu", ".int"} {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}
