package spfflatten

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeTXTResolver struct {
	records map[string][]string
	calls   []string
}

func (f *fakeTXTResolver) LookupTXT(_ context.Context, domain string) ([]string, error) {
	f.calls = append(f.calls, domain)
	if txt, ok := f.records[domain]; ok {
		return txt, nil
	}
	return nil, errors.New("not found")
}

func TestLiteralIPPassthroughDoesNotUseDNS(t *testing.T) {
	dns := &fakeTXTResolver{}
	result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{
		{Kind: SourceIP4, Value: "192.0.2.1", Label: "literal-v4"},
		{Kind: SourceIP6, Value: "2001:db8::/32", Label: "literal-v6"},
	})

	if result.Blocked() {
		t.Fatalf("expected no blocking errors, got %#v", result.Errors)
	}
	if len(dns.calls) != 0 {
		t.Fatalf("expected no DNS lookups, got %v", dns.calls)
	}
	want := []Mechanism{
		{Kind: SourceIP4, Value: "192.0.2.1", Source: "literal-v4"},
		{Kind: SourceIP6, Value: "2001:db8::/32", Source: "literal-v6"},
	}
	assertMechanisms(t, result.Mechanisms, want)
}

func TestIncludeLookupExtractsIPMechanismsAndEvidence(t *testing.T) {
	tests := []struct {
		name    string
		records []string
	}{
		{name: "single TXT string", records: []string{"v=spf1 ip4:192.0.2.10 ip6:2001:db8::/32 ~all"}},
		{name: "joined TXT fragments", records: []string{"v=spf1 ip4:192.0.2.10 ", "ip6:2001:db8::/32 ~all"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dns := &fakeTXTResolver{records: map[string][]string{
				"_spf.example.test": tt.records,
			}}
			result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{
				{Kind: SourceInclude, Value: " _SPF.Example.Test. ", Label: "example include"},
			})

			if result.Blocked() {
				t.Fatalf("expected no blocking errors, got %#v", result.Errors)
			}
			assertMechanisms(t, result.Mechanisms, []Mechanism{
				{Kind: SourceIP4, Value: "192.0.2.10", Source: "_spf.example.test"},
				{Kind: SourceIP6, Value: "2001:db8::/32", Source: "_spf.example.test"},
			})
			if got, want := dns.calls, []string{"_spf.example.test"}; len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("DNS calls = %v, want %v", got, want)
			}
			if len(result.Evidence) < 2 {
				t.Fatalf("expected useful evidence, got %#v", result.Evidence)
			}
			for _, ev := range result.Evidence {
				if ev.SourceValue != "_spf.example.test" || ev.ResolvedDomain != "_spf.example.test" {
					t.Fatalf("unexpected evidence domain/source: %#v", ev)
				}
				if ev.Kind == "" || ev.Mechanism == "" || ev.Message == "" {
					t.Fatalf("expected populated evidence fields: %#v", ev)
				}
			}
		})
	}
}

func TestSPFRecordSelectionBlocksOnZeroOrMultipleRecords(t *testing.T) {
	tests := []struct {
		name    string
		records []string
		wantMsg string
	}{
		{name: "zero", records: []string{"google-site-verification=abc"}, wantMsg: "no SPF"},
		{name: "multiple", records: []string{"v=spf1 ip4:192.0.2.1 ~all", "v=spf1 ip4:192.0.2.2 ~all"}, wantMsg: "multiple SPF"},
		{name: "fragmented multiple", records: []string{"v=spf1 ip4:192.0.2.1 ~all", "v=spf1 ", "ip4:192.0.2.2 ~all"}, wantMsg: "multiple SPF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dns := &fakeTXTResolver{records: map[string][]string{"example.test": tt.records}}
			result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{{Kind: SourceInclude, Value: "example.test"}})
			if !result.Blocked() {
				t.Fatalf("expected blocked result, got %#v", result)
			}
			if len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, tt.wantMsg) {
				t.Fatalf("expected error containing %q, got %#v", tt.wantMsg, result.Errors)
			}
		})
	}
}

func TestNestedIncludeReturnsMechanism(t *testing.T) {
	dns := &fakeTXTResolver{records: map[string][]string{
		"a.test": {"v=spf1 include:b.test ~all"},
		"b.test": {"v=spf1 ip4:192.0.2.55 -all"},
	}}
	result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{{Kind: SourceInclude, Value: "a.test"}})

	if result.Blocked() {
		t.Fatalf("expected no blocking errors, got %#v", result.Errors)
	}
	assertMechanisms(t, result.Mechanisms, []Mechanism{{Kind: SourceIP4, Value: "192.0.2.55", Source: "b.test"}})
	if got, want := strings.Join(dns.calls, ","), "a.test,b.test"; got != want {
		t.Fatalf("DNS calls = %q, want %q", got, want)
	}
}

func TestLoopDetectionBlocks(t *testing.T) {
	dns := &fakeTXTResolver{records: map[string][]string{
		"a.test": {"v=spf1 include:b.test ~all"},
		"b.test": {"v=spf1 include:a.test ~all"},
	}}
	result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{{Kind: SourceInclude, Value: "a.test"}})

	if !result.Blocked() {
		t.Fatalf("expected blocked result for loop, got %#v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(strings.ToLower(result.Errors[0].Message), "loop") {
		t.Fatalf("expected loop error, got %#v", result.Errors)
	}
}

func TestUnsupportedSPFMechanismsBlock(t *testing.T) {
	tests := []struct {
		name   string
		record string
		want   string
	}{
		{name: "mx mechanism", record: "v=spf1 mx include:_spf.example.test ~all", want: "unsupported SPF mechanism"},
		{name: "a mechanism", record: "v=spf1 a ip4:192.0.2.1 ~all", want: "unsupported SPF mechanism"},
		{name: "redirect modifier", record: "v=spf1 redirect=_spf.example.test", want: "unsupported SPF modifier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dns := &fakeTXTResolver{records: map[string][]string{
				"example.test": {tt.record},
			}}
			result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{{Kind: SourceInclude, Value: "example.test"}})
			if !result.Blocked() {
				t.Fatalf("expected unsupported mechanism to block, got %#v", result)
			}
			if len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, tt.want) {
				t.Fatalf("expected error containing %q, got %#v", tt.want, result.Errors)
			}
		})
	}
}

func TestDefaultMaxDepthBlocksLongChain(t *testing.T) {
	records := map[string][]string{}
	for i := 0; i < 11; i++ {
		records[string(rune('a'+i))+".test"] = []string{"v=spf1 include:" + string(rune('a'+i+1)) + ".test ~all"}
	}
	records["l.test"] = []string{"v=spf1 ip4:192.0.2.99 ~all"}
	dns := &fakeTXTResolver{records: records}

	result := (Resolver{DNS: dns}).Resolve(context.Background(), []Source{{Kind: SourceInclude, Value: "a.test"}})

	if !result.Blocked() {
		t.Fatalf("expected blocked result for max depth, got %#v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(strings.ToLower(result.Errors[0].Message), "depth") {
		t.Fatalf("expected depth error, got %#v", result.Errors)
	}
}

func assertMechanisms(t *testing.T, got, want []Mechanism) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("mechanism count = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mechanism[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
