package brandrisk

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestGenerateLookalikeCandidatesIncludesIcelandicHomoglyphs(t *testing.T) {
	t.Parallel()

	candidates := GenerateLookalikeCandidates("postur.is", CandidateOptions{TLD: "is"})
	if !containsCandidate(candidates, "póstur.is", CandidateHomoglyph) {
		t.Fatalf("candidates = %+v, want póstur.is homoglyph", candidates)
	}
	if !containsCandidate(candidates, "postr.is", CandidateTypo) {
		t.Fatalf("candidates = %+v, want postr.is typo", candidates)
	}
}

func TestGenerateLookalikeCandidatesRespectsLimitAndTLD(t *testing.T) {
	t.Parallel()

	candidates := GenerateLookalikeCandidates("example.com", CandidateOptions{TLD: ".is", MaxCount: 3})
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d, want 3", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.Domain[len(candidate.Domain)-3:] != ".is" {
			t.Fatalf("candidate = %+v, want .is TLD", candidate)
		}
	}
}

func TestGenerateLookalikeCandidatesRejectsInvalidDomain(t *testing.T) {
	t.Parallel()

	if candidates := GenerateLookalikeCandidates("localhost", CandidateOptions{}); len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", candidates)
	}
}

func TestEvaluateLookalikeCandidatesReturnsOnlyCorroboratedEvidence(t *testing.T) {
	t.Parallel()

	resolver := fakeLookalikeResolver{
		hosts: map[string][]string{
			"postr.is": {"192.0.2.10"},
		},
		mx: map[string][]*net.MX{
			"póstur.is": {{Host: "mail.example.", Pref: 10}},
		},
	}
	findings := EvaluateLookalikeCandidates(context.Background(), resolver, "postur.is", CandidateOptions{TLD: "is", MaxCount: 20})

	if !containsLookalikeFinding(findings, "postr.is", SeverityMedium, true, false) {
		t.Fatalf("findings = %+v, want active address finding for postr.is", findings)
	}
	if !containsLookalikeFinding(findings, "póstur.is", SeverityHigh, false, true) {
		t.Fatalf("findings = %+v, want MX finding for póstur.is", findings)
	}
	if containsLookalikeDomain(findings, "postur.is") {
		t.Fatalf("findings should not include original domain: %+v", findings)
	}
	for _, finding := range findings {
		if finding.Evidence == "" || finding.RecommendedAction == "" {
			t.Fatalf("finding missing evidence/action: %+v", finding)
		}
	}
}

func TestEvaluateLookalikeCandidatesCanIncludeInactiveWatchlist(t *testing.T) {
	t.Parallel()

	findings := EvaluateLookalikeCandidates(context.Background(), fakeLookalikeResolver{}, "postur.is", CandidateOptions{TLD: "is", MaxCount: 1, IncludeInactive: true})
	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1 inactive watch candidate", len(findings))
	}
	if findings[0].Severity != SeverityInfo || findings[0].HasAddressRecords || findings[0].HasMXRecords {
		t.Fatalf("unexpected inactive finding: %+v", findings[0])
	}
}

func containsCandidate(candidates []LookalikeCandidate, domain string, kind CandidateKind) bool {
	for _, candidate := range candidates {
		if candidate.Domain == domain && candidate.Kind == kind {
			return true
		}
	}
	return false
}

func containsLookalikeFinding(findings []LookalikeFinding, domain string, severity Severity, hasAddress bool, hasMX bool) bool {
	for _, finding := range findings {
		if finding.Domain == domain && finding.Severity == severity && finding.HasAddressRecords == hasAddress && finding.HasMXRecords == hasMX {
			return true
		}
	}
	return false
}

func containsLookalikeDomain(findings []LookalikeFinding, domain string) bool {
	for _, finding := range findings {
		if finding.Domain == domain {
			return true
		}
	}
	return false
}

type fakeLookalikeResolver struct {
	hosts map[string][]string
	mx    map[string][]*net.MX
}

func (r fakeLookalikeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if values := r.hosts[host]; len(values) > 0 {
		return values, nil
	}
	for domain, values := range r.hosts {
		normalized, _ := scanner.NormalizeDomain(domain)
		if normalized == host && len(values) > 0 {
			return values, nil
		}
	}
	return nil, errors.New("not found")
}

func (r fakeLookalikeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	if values := r.mx[name]; len(values) > 0 {
		return values, nil
	}
	for domain, values := range r.mx {
		normalized, _ := scanner.NormalizeDomain(domain)
		if normalized == name && len(values) > 0 {
			return values, nil
		}
	}
	return nil, errors.New("not found")
}
