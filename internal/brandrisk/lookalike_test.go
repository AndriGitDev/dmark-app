package brandrisk

import "testing"

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

func containsCandidate(candidates []LookalikeCandidate, domain string, kind CandidateKind) bool {
	for _, candidate := range candidates {
		if candidate.Domain == domain && candidate.Kind == kind {
			return true
		}
	}
	return false
}
