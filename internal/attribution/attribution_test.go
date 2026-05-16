package attribution

import (
	"net/netip"
	"testing"
)

func TestClassifierMatchesProviderByMultipleSignals(t *testing.T) {
	t.Parallel()

	result := DefaultClassifier().Classify(Signal{
		IP:                  "209.85.220.41",
		ReverseDNS:          "mail-sor-f41.google.com.",
		ASN:                 "AS15169",
		AuthenticatedDomain: "_spf.google.com",
	})

	if result.Provider != "Google Workspace" || result.Confidence != ConfidenceHigh {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Evidence) < 2 {
		t.Fatalf("expected multiple evidence entries: %+v", result)
	}
}

func TestClassifierReturnsNoneForUnknownSender(t *testing.T) {
	t.Parallel()

	result := DefaultClassifier().Classify(Signal{
		IP:         "203.0.113.10",
		ReverseDNS: "mail.unknown.example",
		ASN:        "AS64500",
	})

	if result.Provider != "" || result.Confidence != ConfidenceNone {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClassifierMatchesCustomIPPrefix(t *testing.T) {
	t.Parallel()

	classifier := NewClassifier([]ProviderRule{
		{
			Name:       "Example ESP",
			IPPrefixes: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
		},
	})
	result := classifier.Classify(Signal{IP: "203.0.113.10"})

	if result.Provider != "Example ESP" || result.Confidence != ConfidenceMedium {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClassifierReviewDetectsAmbiguousTopCandidates(t *testing.T) {
	t.Parallel()

	classifier := NewClassifier([]ProviderRule{
		{Name: "Provider A", ASNs: []string{"AS64500"}},
		{Name: "Provider B", ASNs: []string{"AS64500"}},
	})
	review := classifier.Review(Signal{IP: "203.0.113.10", ASN: "AS64500"})
	if !review.Ambiguous {
		t.Fatalf("Review should flag equal-confidence candidates as ambiguous: %+v", review)
	}
	if len(review.Candidates) != 2 {
		t.Fatalf("len(Candidates) = %d, want 2", len(review.Candidates))
	}
	if review.Result.Provider != "Provider A" || review.Result.Confidence != ConfidenceMedium {
		t.Fatalf("unexpected selected result: %+v", review.Result)
	}
}

func TestProviderReviewFixtures(t *testing.T) {
	t.Parallel()

	classifier := DefaultClassifier()
	for _, fixture := range ProviderReviewFixtures() {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			review := classifier.Review(Signal{
				IP:                  fixture.Lookup.IP,
				ReverseDNS:          fixture.Lookup.ReverseDNS,
				ASN:                 fixture.Lookup.ASN,
				AuthenticatedDomain: fixture.AuthenticatedDomain,
			})
			if review.Result.Provider != fixture.WantProvider || review.Result.Confidence != fixture.WantConfidence || review.Ambiguous != fixture.WantAmbiguous {
				t.Fatalf("Review = %+v, want provider %q confidence %q ambiguous %v", review, fixture.WantProvider, fixture.WantConfidence, fixture.WantAmbiguous)
			}
		})
	}
}
