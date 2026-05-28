package brandrisk

import (
	"context"
	"net"
	"sort"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type CandidateKind string

const (
	CandidateTypo      CandidateKind = "typo"
	CandidateHomoglyph CandidateKind = "homoglyph"
)

type LookalikeCandidate struct {
	Domain string
	Kind   CandidateKind
	Reason string
}

type LookalikeResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
}

type LookalikeFinding struct {
	Domain            string
	Kind              CandidateKind
	Reason            string
	HasAddressRecords bool
	HasMXRecords      bool
	Severity          Severity
	Evidence          string
	RecommendedAction string
}

type CandidateOptions struct {
	TLD             string
	MaxCount        int
	IncludeInactive bool
}

func GenerateLookalikeCandidates(domain string, options CandidateOptions) []LookalikeCandidate {
	normalized, err := scanner.NormalizeDomain(domain)
	if err != nil {
		return nil
	}
	label, _, ok := strings.Cut(normalized, ".")
	if !ok || label == "" {
		return nil
	}
	tld := strings.TrimPrefix(strings.TrimSpace(options.TLD), ".")
	if tld == "" {
		tld = "is"
	}
	limit := options.MaxCount
	if limit <= 0 {
		limit = 100
	}
	seen := map[string]LookalikeCandidate{}
	add := func(candidate string, kind CandidateKind, reason string) {
		if candidate == label || candidate == "" {
			return
		}
		name := candidate + "." + tld
		if _, err := scanner.NormalizeDomain(name); err != nil {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = LookalikeCandidate{Domain: name, Kind: kind, Reason: reason}
	}

	for i := range label {
		add(label[:i]+label[i+1:], CandidateTypo, "single-character deletion")
	}
	for i := 0; i < len(label)-1; i++ {
		swapped := []byte(label)
		swapped[i], swapped[i+1] = swapped[i+1], swapped[i]
		add(string(swapped), CandidateTypo, "adjacent-character swap")
	}
	for from, replacements := range icelandicHomoglyphs() {
		for _, to := range replacements {
			if strings.Contains(label, from) {
				add(strings.ReplaceAll(label, from, to), CandidateHomoglyph, "Icelandic-character homoglyph")
			}
		}
	}

	candidates := make([]LookalikeCandidate, 0, len(seen))
	for _, candidate := range seen {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].Kind == candidates[j].Kind {
			return candidates[i].Domain < candidates[j].Domain
		}
		return candidates[i].Kind < candidates[j].Kind
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func EvaluateLookalikeCandidates(ctx context.Context, resolver LookalikeResolver, domain string, options CandidateOptions) []LookalikeFinding {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	candidates := GenerateLookalikeCandidates(domain, options)
	findings := make([]LookalikeFinding, 0, len(candidates))
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		queryDomain, err := scanner.NormalizeDomain(candidate.Domain)
		if err != nil {
			continue
		}
		addresses, _ := resolver.LookupHost(ctx, queryDomain)
		mx, _ := resolver.LookupMX(ctx, queryDomain)
		hasAddress := len(addresses) > 0
		hasMX := len(mx) > 0
		if !hasAddress && !hasMX && !options.IncludeInactive {
			continue
		}
		findings = append(findings, LookalikeFinding{
			Domain:            candidate.Domain,
			Kind:              candidate.Kind,
			Reason:            candidate.Reason,
			HasAddressRecords: hasAddress,
			HasMXRecords:      hasMX,
			Severity:          lookalikeSeverity(hasAddress, hasMX),
			Evidence:          lookalikeEvidence(hasAddress, hasMX),
			RecommendedAction: lookalikeAction(hasAddress, hasMX),
		})
	}
	return findings
}

func lookalikeSeverity(hasAddress bool, hasMX bool) Severity {
	switch {
	case hasMX:
		return SeverityHigh
	case hasAddress:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}

func lookalikeEvidence(hasAddress bool, hasMX bool) string {
	signals := []string{}
	if hasAddress {
		signals = append(signals, "A/AAAA records are published")
	}
	if hasMX {
		signals = append(signals, "MX records are published")
	}
	if len(signals) == 0 {
		return "No active DNS address or MX records were observed during the bounded Defender lookup."
	}
	return "DNS evidence: " + strings.Join(signals, "; ") + "."
}

func lookalikeAction(hasAddress bool, hasMX bool) string {
	if hasMX {
		return "Review ownership and mail-routing intent immediately; MX records make this lookalike suitable for email impersonation."
	}
	if hasAddress {
		return "Review ownership and web content for impersonation risk, then acknowledge or monitor the candidate."
	}
	return "Keep the candidate in watch mode and only alert when DNS evidence appears."
}

func icelandicHomoglyphs() map[string][]string {
	return map[string][]string{
		"a":  {"á"},
		"d":  {"ð"},
		"e":  {"é"},
		"i":  {"í"},
		"o":  {"ó", "ö"},
		"p":  {"þ"},
		"u":  {"ú"},
		"y":  {"ý"},
		"á":  {"a"},
		"ð":  {"d"},
		"é":  {"e"},
		"í":  {"i"},
		"ó":  {"o", "ö"},
		"ö":  {"o", "ó"},
		"þ":  {"p"},
		"ú":  {"u"},
		"ý":  {"y"},
		"ae": {"æ"},
		"æ":  {"ae"},
	}
}
