package scanner

type PostureSummary struct {
	Score       int
	Status      Status
	TopIssue    string
	Remediation string
}

func Summarize(result Result) PostureSummary {
	checks := []CheckResult{result.SPF, result.DKIM, result.DMARC, result.BIMI}
	score := 100
	var top CheckResult
	for _, check := range checks {
		score -= penalty(check.Status)
		if top.Name == "" || statusPriority(check.Status) > statusPriority(top.Status) {
			top = check
		}
	}
	if score < 0 {
		score = 0
	}
	status := StatusPass
	if score < 90 {
		status = StatusWarn
	}
	if score < 60 {
		status = StatusFail
	}
	if top.Name == "" || top.Status == StatusPass {
		return PostureSummary{Score: score, Status: status, TopIssue: "No high-priority issue detected.", Remediation: "Keep monitoring DNS authentication records for drift."}
	}
	return PostureSummary{Score: score, Status: status, TopIssue: top.Name + ": " + top.Message, Remediation: remediationFor(top)}
}

func penalty(status Status) int {
	switch status {
	case StatusMissing:
		return 25
	case StatusFail:
		return 30
	case StatusUnknown:
		return 15
	case StatusWarn:
		return 10
	default:
		return 0
	}
}

func statusPriority(status Status) int {
	switch status {
	case StatusFail:
		return 5
	case StatusMissing:
		return 4
	case StatusUnknown:
		return 3
	case StatusWarn:
		return 2
	default:
		return 1
	}
}

func remediationFor(check CheckResult) string {
	switch check.Name {
	case "SPF":
		return `Publish exactly one SPF TXT record, avoid +all/?all/ptr, keep DNS lookups at 10 or fewer, and prefer a terminal -all policy. Example: "v=spf1 include:_spf.example.net -all".`
	case "DKIM":
		return `Publish DKIM public keys for each provider selector that signs outbound mail. Example: selector1._domainkey.example.com TXT "v=DKIM1; k=rsa; p=PUBLICKEY".`
	case "DMARC":
		return `Publish one DMARC TXT record with an enforcement policy after validating reports. Example: "v=DMARC1; p=reject; rua=mailto:dmarc@example.com".`
	case "BIMI":
		return `Publish BIMI only after DMARC enforcement is in place, with an HTTPS SVG logo and certificate URI when available. Example: "v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem".`
	default:
		return "Review the DNS record and restore the expected authentication posture."
	}
}
