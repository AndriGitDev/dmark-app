package spfflatten

import (
	"context"
	"fmt"
	"strings"
)

const defaultMaxDepth = 10

// TXTResolver looks up TXT records for a domain.
type TXTResolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

type SourceKind string

const (
	SourceInclude SourceKind = "include"
	SourceIP4     SourceKind = "ip4"
	SourceIP6     SourceKind = "ip6"
)

type Source struct {
	Kind  SourceKind
	Value string
	Label string
}

type Mechanism struct {
	Kind   SourceKind
	Value  string
	Source string
}

type Evidence struct {
	SourceValue    string
	ResolvedDomain string
	Mechanism      string
	Kind           string
	Message        string
}

type Result struct {
	Mechanisms []Mechanism
	Evidence   []Evidence
	Errors     []ResolutionError
}

type ResolutionError struct {
	Domain  string
	Message string
}

func (r Result) Blocked() bool { return len(r.Errors) > 0 }

type Resolver struct {
	DNS      TXTResolver
	MaxDepth int
}

func (r Resolver) Resolve(ctx context.Context, sources []Source) Result {
	state := resolverState{resolver: r, maxDepth: r.MaxDepth}
	if state.maxDepth == 0 {
		state.maxDepth = defaultMaxDepth
	}

	for _, source := range sources {
		switch source.Kind {
		case SourceIP4, SourceIP6:
			value := strings.TrimSpace(source.Value)
			state.result.Mechanisms = append(state.result.Mechanisms, Mechanism{
				Kind:   source.Kind,
				Value:  value,
				Source: source.Label,
			})
			state.result.Evidence = append(state.result.Evidence, Evidence{
				SourceValue:    source.Value,
				ResolvedDomain: source.Label,
				Mechanism:      string(source.Kind) + ":" + value,
				Kind:           string(source.Kind),
				Message:        "literal SPF mechanism preserved",
			})
		case SourceInclude:
			domain := normalizeDomain(source.Value)
			state.resolveInclude(ctx, source.Value, domain, 0, nil)
		}
	}

	return state.result
}

type resolverState struct {
	resolver Resolver
	maxDepth int
	result   Result
}

func (s *resolverState) resolveInclude(ctx context.Context, sourceValue, domain string, depth int, stack []string) {
	if domain == "" {
		s.addError(domain, "empty include domain")
		return
	}
	if depth > s.maxDepth {
		s.addError(domain, fmt.Sprintf("maximum SPF include depth exceeded at %s", domain))
		return
	}
	for _, seen := range stack {
		if seen == domain {
			s.addError(domain, fmt.Sprintf("SPF include loop detected: %s -> %s", strings.Join(stack, " -> "), domain))
			return
		}
	}
	if s.resolver.DNS == nil {
		s.addError(domain, "TXT resolver is nil")
		return
	}

	txts, err := s.resolver.DNS.LookupTXT(ctx, domain)
	if err != nil {
		s.addError(domain, fmt.Sprintf("lookup TXT: %v", err))
		return
	}

	record, ok := selectSPFRecord(txts)
	if !ok {
		s.addError(domain, "no SPF record found")
		return
	}
	if record == multipleSPFRecords {
		s.addError(domain, "multiple SPF records found")
		return
	}

	currentStack := append(stack, domain)
	for _, token := range strings.Fields(record) {
		mechanism := stripQualifier(token)
		switch {
		case mechanism == "v=spf1" || isTerminalMechanism(mechanism):
			continue
		case strings.HasPrefix(mechanism, "ip4:"):
			value := strings.TrimSpace(strings.TrimPrefix(mechanism, "ip4:"))
			if value == "" {
				continue
			}
			s.result.Mechanisms = append(s.result.Mechanisms, Mechanism{Kind: SourceIP4, Value: value, Source: domain})
			s.result.Evidence = append(s.result.Evidence, Evidence{
				SourceValue:    normalizeDomain(sourceValue),
				ResolvedDomain: domain,
				Mechanism:      token,
				Kind:           string(SourceIP4),
				Message:        "resolved ip4 mechanism from SPF include",
			})
		case strings.HasPrefix(mechanism, "ip6:"):
			value := strings.TrimSpace(strings.TrimPrefix(mechanism, "ip6:"))
			if value == "" {
				continue
			}
			s.result.Mechanisms = append(s.result.Mechanisms, Mechanism{Kind: SourceIP6, Value: value, Source: domain})
			s.result.Evidence = append(s.result.Evidence, Evidence{
				SourceValue:    normalizeDomain(sourceValue),
				ResolvedDomain: domain,
				Mechanism:      token,
				Kind:           string(SourceIP6),
				Message:        "resolved ip6 mechanism from SPF include",
			})
		case strings.HasPrefix(mechanism, "include:"):
			included := normalizeDomain(strings.TrimPrefix(mechanism, "include:"))
			s.result.Evidence = append(s.result.Evidence, Evidence{
				SourceValue:    normalizeDomain(sourceValue),
				ResolvedDomain: domain,
				Mechanism:      token,
				Kind:           string(SourceInclude),
				Message:        "following SPF include " + included,
			})
			s.resolveInclude(ctx, included, included, depth+1, currentStack)
		case strings.Contains(mechanism, "="):
			s.addError(domain, "unsupported SPF modifier "+mechanism)
		default:
			s.addError(domain, "unsupported SPF mechanism "+mechanism)
		}
	}
}

func (s *resolverState) addError(domain, message string) {
	s.result.Errors = append(s.result.Errors, ResolutionError{Domain: domain, Message: message})
}

const multipleSPFRecords = "\x00multiple SPF records\x00"

func selectSPFRecord(txts []string) (string, bool) {
	var records []string
	for i := 0; i < len(txts); i++ {
		part := strings.TrimSpace(txts[i])
		if !startsSPF(part) {
			continue
		}

		record := txts[i]
		for i+1 < len(txts) && !startsSPF(strings.TrimSpace(txts[i+1])) && likelySPFContinuation(record) {
			i++
			record += txts[i]
		}
		records = append(records, strings.TrimSpace(record))
	}

	switch len(records) {
	case 0:
		return "", false
	case 1:
		return records[0], true
	default:
		return multipleSPFRecords, true
	}
}

func likelySPFContinuation(record string) bool {
	fields := strings.Fields(record)
	if len(fields) == 0 {
		return false
	}
	last := fields[len(fields)-1]
	return !strings.HasSuffix(last, "all")
}

func startsSPF(record string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(record)), "v=spf1")
}

func stripQualifier(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return token
	}
	switch token[0] {
	case '+', '-', '~', '?':
		return token[1:]
	default:
		return token
	}
}

func isTerminalMechanism(mechanism string) bool {
	return mechanism == "all"
}

func normalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}
