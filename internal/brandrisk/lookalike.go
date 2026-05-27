package brandrisk

import (
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

type CandidateOptions struct {
	TLD      string
	MaxCount int
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
