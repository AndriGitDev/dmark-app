package dnsprovider

import (
	"strings"
	"testing"
)

func TestDetectRecognizesKnownProviders(t *testing.T) {
	tests := []struct {
		name        string
		nameservers []string
		wantID      string
	}{
		{
			name:        "isnic normalizes whitespace case and trailing dots",
			nameservers: []string{"  NS0.ISNIC.IS. ", "ns1.isnic.is"},
			wantID:      "isnic",
		},
		{
			name:        "cloudflare",
			nameservers: []string{"alice.ns.cloudflare.com", "bob.ns.cloudflare.com."},
			wantID:      "cloudflare",
		},
		{
			name:        "route53 awsdns",
			nameservers: []string{"ns-123.awsdns-45.com", "ns-678.awsdns-90.net"},
			wantID:      "route53",
		},
		{
			name:        "bunny",
			nameservers: []string{"kiki.bunnydns.com", "coco.bunny.net"},
			wantID:      "bunny",
		},
		{
			name:        "1984",
			nameservers: []string{"ns0.1984.is", "ns1.1984.is."},
			wantID:      "1984",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.nameservers)
			if got.Status != StatusDetected {
				t.Fatalf("Status = %q, want %q (detection: %+v)", got.Status, StatusDetected, got)
			}
			if got.ProviderID != tt.wantID {
				t.Fatalf("ProviderID = %q, want %q", got.ProviderID, tt.wantID)
			}
			if got.ProviderName == "" {
				t.Fatal("ProviderName should be populated for detected provider")
			}
			if got.Confidence != "high" {
				t.Fatalf("Confidence = %q, want high", got.Confidence)
			}
			if len(got.EvidenceNameservers) != len(tt.nameservers) {
				t.Fatalf("EvidenceNameservers = %#v, want %d normalized entries", got.EvidenceNameservers, len(tt.nameservers))
			}
			for _, ns := range got.EvidenceNameservers {
				if ns != strings.TrimSuffix(strings.ToLower(strings.TrimSpace(ns)), ".") {
					t.Fatalf("evidence nameserver %q was not normalized", ns)
				}
			}
			if got.DNSDashboardURL == "" {
				t.Fatal("DNSDashboardURL should be populated for detected provider")
			}
		})
	}
}

func TestDetectUnknownAndAmbiguous(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		got := Detect([]string{"ns1.example.invalid.", " ns2.example.invalid "})
		if got.Status != StatusUnknown {
			t.Fatalf("Status = %q, want %q (detection: %+v)", got.Status, StatusUnknown, got)
		}
		if got.ProviderID != "" || got.ProviderName != "" {
			t.Fatalf("provider should be empty for unknown: %+v", got)
		}
		if got.Confidence != "low" {
			t.Fatalf("Confidence = %q, want low", got.Confidence)
		}
		if want := []string{"ns1.example.invalid", "ns2.example.invalid"}; strings.Join(got.EvidenceNameservers, ",") != strings.Join(want, ",") {
			t.Fatalf("EvidenceNameservers = %#v, want %#v", got.EvidenceNameservers, want)
		}
	})

	t.Run("ambiguous mixed provider nameservers", func(t *testing.T) {
		got := Detect([]string{"alice.ns.cloudflare.com", "ns-123.awsdns-45.com"})
		if got.Status != StatusAmbiguous {
			t.Fatalf("Status = %q, want %q (detection: %+v)", got.Status, StatusAmbiguous, got)
		}
		if got.ProviderID != "" || got.ProviderName != "" || got.DNSDashboardURL != "" {
			t.Fatalf("single provider fields should be empty for ambiguous detection: %+v", got)
		}
		if got.Confidence != "ambiguous" {
			t.Fatalf("Confidence = %q, want ambiguous", got.Confidence)
		}
		if len(got.EvidenceNameservers) != 2 {
			t.Fatalf("EvidenceNameservers = %#v, want both nameservers", got.EvidenceNameservers)
		}
	})
}

func TestDetectDoesNotOverMatchRoute53Substring(t *testing.T) {
	got := Detect([]string{"awsdns.example.invalid"})
	if got.Status != StatusUnknown {
		t.Fatalf("Status = %q, want unknown for unrelated awsdns substring: %+v", got.Status, got)
	}
}

func TestProvidersReturnsDeepCopy(t *testing.T) {
	providers := Providers()
	if len(providers) == 0 || len(providers[0].NSPatterns) == 0 {
		t.Fatal("expected provider registry with nameserver patterns")
	}
	original := providers[0].NSPatterns[0]
	providers[0].NSPatterns[0] = "mutated.example"
	again := Providers()
	if again[0].NSPatterns[0] != original {
		t.Fatalf("Providers returned mutable registry backing slice: got %q, want %q", again[0].NSPatterns[0], original)
	}
}

func TestBuildInstructionsForDetectedCloudflareSPFRouteCNAME(t *testing.T) {
	detection := Detect([]string{"alice.ns.cloudflare.com", "bob.ns.cloudflare.com"})
	got := BuildInstructions(detection, RecordInstructionInput{
		Kind:  RecordSPFRouteCNAME,
		Name:  "_spf.example.com",
		Value: "route.dmark.example.net",
		TTL:   300,
	})

	if got.ProviderID != "cloudflare" || got.ProviderName == "" || got.DashboardURL == "" {
		t.Fatalf("provider metadata not copied from detection: %+v", got)
	}
	if len(got.Steps) < 3 || len(got.Steps) > 5 {
		t.Fatalf("got %d steps, want 3-5: %#v", len(got.Steps), got.Steps)
	}
	joined := strings.Join(got.Steps, "\n")
	for _, want := range []string{"Cloudflare", "_spf.example.com", "CNAME", "route.dmark.example.net", "300"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("instructions missing %q in:\n%s", want, joined)
		}
	}
}

func TestBuildInstructionsGenericAndRecordKinds(t *testing.T) {
	tests := []struct {
		name      string
		kind      RecordKind
		recordTyp string
		record    string
		value     string
	}{
		{name: "DMARC TXT", kind: RecordDMARCTXT, recordTyp: "TXT", record: "_dmarc.example.com", value: "v=DMARC1; p=none"},
		{name: "SPF TXT", kind: RecordSPFTXT, recordTyp: "TXT", record: "example.com", value: "v=spf1 include:_spf.example.com ~all"},
		{name: "SPF route CNAME", kind: RecordSPFRouteCNAME, recordTyp: "CNAME", record: "_spf.example.com", value: "route.dmark.example.net"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildInstructions(Detection{Status: StatusUnknown, Confidence: "low"}, RecordInstructionInput{
				Kind:  tt.kind,
				Name:  tt.record,
				Value: tt.value,
				TTL:   3600,
			})
			if got.ProviderID != "" || got.ProviderName != "Generic DNS" || got.DashboardURL != "" {
				t.Fatalf("generic metadata wrong: %+v", got)
			}
			joined := strings.Join(got.Steps, "\n")
			for _, want := range []string{tt.recordTyp, tt.record, tt.value, "3600"} {
				if !strings.Contains(joined, want) {
					t.Fatalf("generic instructions missing %q in:\n%s", want, joined)
				}
			}
		})
	}
}
