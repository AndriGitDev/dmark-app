package attribution

import (
	"net/netip"
	"strings"
)

type Confidence string

const (
	ConfidenceNone   Confidence = "none"
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type Signal struct {
	IP                  string
	ReverseDNS          string
	ASN                 string
	AuthenticatedDomain string
}

type ProviderRule struct {
	Name            string
	IPPrefixes      []netip.Prefix
	ReverseSuffixes []string
	ASNs            []string
	Domains         []string
}

type Result struct {
	Provider   string
	Confidence Confidence
	Evidence   []string
}

type Review struct {
	Result     Result
	Candidates []Result
	Ambiguous  bool
}

type Classifier struct {
	rules []ProviderRule
}

func NewClassifier(rules []ProviderRule) Classifier {
	copied := make([]ProviderRule, len(rules))
	copy(copied, rules)
	return Classifier{rules: copied}
}

func DefaultClassifier() Classifier {
	return NewClassifier([]ProviderRule{
		{Name: "Google Workspace", ReverseSuffixes: []string{".google.com", ".googlemail.com"}, ASNs: []string{"AS15169"}, Domains: []string{"google.com", "_spf.google.com"}},
		{Name: "Microsoft 365", ReverseSuffixes: []string{".outbound.protection.outlook.com"}, ASNs: []string{"AS8075"}, Domains: []string{"protection.outlook.com", "spf.protection.outlook.com"}},
		{Name: "Amazon SES", ReverseSuffixes: []string{".amazonses.com"}, ASNs: []string{"AS16509"}, Domains: []string{"amazonses.com"}},
		{Name: "Mailchimp", ReverseSuffixes: []string{".mcsv.net", ".mailchimp.com"}, Domains: []string{"servers.mcsv.net", "mailchimp.com"}},
		{Name: "SendGrid", ReverseSuffixes: []string{".sendgrid.net"}, Domains: []string{"sendgrid.net"}},
	})
}

func (c Classifier) Classify(signal Signal) Result {
	return c.Review(signal).Result
}

func (c Classifier) Review(signal Signal) Review {
	best := Result{Confidence: ConfidenceNone}
	candidates := []Result{}
	for _, rule := range c.rules {
		result := matchRule(rule, signal)
		rank := confidenceRank(result.Confidence)
		bestRank := confidenceRank(best.Confidence)
		if rank > bestRank {
			best = result
			if rank > 0 {
				candidates = []Result{result}
			}
			continue
		}
		if rank > 0 && rank == bestRank {
			candidates = append(candidates, result)
		}
	}
	return Review{
		Result:     best,
		Candidates: candidates,
		Ambiguous:  len(candidates) > 1,
	}
}

func matchRule(rule ProviderRule, signal Signal) Result {
	evidence := []string{}
	ip, _ := netip.ParseAddr(strings.TrimSpace(signal.IP))
	for _, prefix := range rule.IPPrefixes {
		if ip.IsValid() && prefix.Contains(ip) {
			evidence = append(evidence, "source IP matches "+prefix.String())
		}
	}
	reverse := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(signal.ReverseDNS), "."))
	for _, suffix := range rule.ReverseSuffixes {
		suffix = strings.TrimSuffix(strings.ToLower(suffix), ".")
		if reverse != "" && strings.HasSuffix(reverse, suffix) {
			evidence = append(evidence, "reverse DNS matches "+suffix)
		}
	}
	asn := strings.ToUpper(strings.TrimSpace(signal.ASN))
	for _, ruleASN := range rule.ASNs {
		if asn != "" && asn == strings.ToUpper(ruleASN) {
			evidence = append(evidence, "ASN matches "+strings.ToUpper(ruleASN))
		}
	}
	authDomain := strings.ToLower(strings.TrimSpace(signal.AuthenticatedDomain))
	for _, domain := range rule.Domains {
		domain = strings.ToLower(domain)
		if authDomain == domain || strings.HasSuffix(authDomain, "."+domain) {
			evidence = append(evidence, "authenticated domain matches "+domain)
		}
	}
	return Result{Provider: rule.Name, Confidence: confidenceForEvidence(len(evidence)), Evidence: evidence}
}

func confidenceForEvidence(count int) Confidence {
	switch {
	case count >= 2:
		return ConfidenceHigh
	case count == 1:
		return ConfidenceMedium
	default:
		return ConfidenceNone
	}
}

func confidenceRank(confidence Confidence) int {
	switch confidence {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}
