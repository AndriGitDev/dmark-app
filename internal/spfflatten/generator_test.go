package spfflatten

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateDeduplicatesSortsAndUsesDefaultTerminal(t *testing.T) {
	got := Generate([]Mechanism{
		{Kind: SourceIP6, Value: "2001:db8::2"},
		{Kind: SourceIP4, Value: "203.0.113.7"},
		{Kind: SourceIP4, Value: "192.0.2.1"},
		{Kind: SourceIP4, Value: "203.0.113.7"},
		{Kind: SourceIP6, Value: "2001:db8::1"},
		{Kind: SourceIP6, Value: "2001:db8::2"},
	}, GenerateOptions{})

	if got.Blocked {
		t.Fatalf("expected generated SPF not blocked, got error %q", got.Error)
	}
	want := "v=spf1 ip4:192.0.2.1 ip4:203.0.113.7 ip6:2001:db8::1 ip6:2001:db8::2 ~all"
	if got.SPF != want {
		t.Fatalf("SPF = %q, want %q", got.SPF, want)
	}
}

func TestGenerateSupportsConfiguredTerminalPolicy(t *testing.T) {
	tests := []struct {
		name     string
		terminal string
		want     string
	}{
		{name: "default explicit", terminal: "~all", want: "v=spf1 ip4:192.0.2.1 ~all"},
		{name: "fail", terminal: "-all", want: "v=spf1 ip4:192.0.2.1 -all"},
		{name: "neutral", terminal: "?all", want: "v=spf1 ip4:192.0.2.1 ?all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Generate([]Mechanism{{Kind: SourceIP4, Value: "192.0.2.1"}}, GenerateOptions{Terminal: tt.terminal})
			if got.Blocked {
				t.Fatalf("expected terminal %q to be accepted, got error %q", tt.terminal, got.Error)
			}
			if got.SPF != tt.want {
				t.Fatalf("SPF = %q, want %q", got.SPF, tt.want)
			}
		})
	}
}

func TestGenerateRejectsUnsupportedTerminalPolicy(t *testing.T) {
	got := Generate([]Mechanism{{Kind: SourceIP4, Value: "192.0.2.1"}}, GenerateOptions{Terminal: "+all"})

	if !got.Blocked {
		t.Fatalf("expected unsupported terminal to block, got %#v", got)
	}
	if got.Error == "" || !strings.Contains(got.Error, "unsupported terminal") {
		t.Fatalf("expected useful unsupported terminal error, got %q", got.Error)
	}
	if got.SPF != "" {
		t.Fatalf("expected no SPF on blocked result, got %q", got.SPF)
	}
}

func TestGenerateTrimsConfiguredTerminalPolicy(t *testing.T) {
	got := Generate([]Mechanism{{Kind: SourceIP4, Value: "192.0.2.1"}}, GenerateOptions{Terminal: " -all "})
	if got.Blocked {
		t.Fatalf("expected trimmed terminal to be accepted, got error %q", got.Error)
	}
	if got.SPF != "v=spf1 ip4:192.0.2.1 -all" {
		t.Fatalf("SPF = %q", got.SPF)
	}
}

func TestGenerateBlocksMalformedMechanismValues(t *testing.T) {
	tests := []struct {
		name      string
		mechanism Mechanism
	}{
		{name: "ip4 injection", mechanism: Mechanism{Kind: SourceIP4, Value: "192.0.2.1 -all"}},
		{name: "invalid ip4", mechanism: Mechanism{Kind: SourceIP4, Value: "not-an-ip"}},
		{name: "invalid ip6", mechanism: Mechanism{Kind: SourceIP6, Value: "192.0.2.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Generate([]Mechanism{tt.mechanism}, GenerateOptions{})
			if !got.Blocked {
				t.Fatalf("expected malformed mechanism to block, got %#v", got)
			}
			if got.Error == "" || !strings.Contains(got.Error, "invalid") {
				t.Fatalf("expected invalid mechanism error, got %q", got.Error)
			}
		})
	}
}

func TestGenerateBlocksWhenOutputExceedsConfiguredMaxLength(t *testing.T) {
	got := Generate([]Mechanism{{Kind: SourceIP4, Value: "192.0.2.1"}}, GenerateOptions{MaxLength: len("v=spf1 ip4:192.0.2.1 ~all") - 1})

	if !got.Blocked {
		t.Fatalf("expected max length violation to block, got %#v", got)
	}
	if got.Error == "" || !strings.Contains(got.Error, "exceeds max length") {
		t.Fatalf("expected useful max length error, got %q", got.Error)
	}
}

func TestGenerateUsesDefaultMaxLengthWhenUnset(t *testing.T) {
	var mechanisms []Mechanism
	for i := 0; i < 50; i++ {
		mechanisms = append(mechanisms, Mechanism{Kind: SourceIP4, Value: fmt.Sprintf("192.0.2.%d", i)})
	}

	got := Generate(mechanisms, GenerateOptions{})
	if !got.Blocked {
		t.Fatalf("expected default max length to block long generated SPF, got length %d SPF %q", len(got.SPF), got.SPF)
	}
	if got.Error == "" || !strings.Contains(got.Error, "450") {
		t.Fatalf("expected error to mention default max length, got %q", got.Error)
	}
}

func TestGenerateIgnoresUnsupportedKindsAndEmptyValues(t *testing.T) {
	got := Generate([]Mechanism{
		{Kind: SourceInclude, Value: "_spf.example.test"},
		{Kind: SourceIP4, Value: ""},
		{Kind: SourceIP6, Value: ""},
		{Kind: SourceIP4, Value: "192.0.2.1"},
	}, GenerateOptions{})

	if got.Blocked {
		t.Fatalf("expected generated SPF not blocked, got error %q", got.Error)
	}
	want := "v=spf1 ip4:192.0.2.1 ~all"
	if got.SPF != want {
		t.Fatalf("SPF = %q, want %q", got.SPF, want)
	}
}
