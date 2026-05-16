package domains

import (
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestFilterHistoryByStatus(t *testing.T) {
	t.Parallel()

	entries := []HistoryEntry{
		{Domain: "a.example", Check: Check{SPF: scanner.CheckResult{Status: scanner.StatusPass}, DMARC: scanner.CheckResult{Status: scanner.StatusWarn}}},
		{Domain: "b.example", Check: Check{SPF: scanner.CheckResult{Status: scanner.StatusFail}}},
	}
	filtered := FilterHistory(entries, HistoryFilter{Status: scanner.StatusWarn})

	if len(filtered) != 1 || filtered[0].Domain != "a.example" {
		t.Fatalf("filtered = %#v, want only a.example", filtered)
	}
}

func TestFilterHistoryByTypeAndStatus(t *testing.T) {
	t.Parallel()

	entries := []HistoryEntry{
		{Domain: "a.example", Check: Check{DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn}}},
		{Domain: "b.example", Check: Check{DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass}}},
		{Domain: "c.example", Check: Check{SPF: scanner.CheckResult{Name: "SPF", Status: scanner.StatusWarn}}},
	}
	filtered := FilterHistory(entries, HistoryFilter{Type: CheckTypeDMARC, Status: scanner.StatusWarn})

	if len(filtered) != 1 || filtered[0].Domain != "a.example" {
		t.Fatalf("filtered = %#v, want only warning DMARC entry", filtered)
	}
}

func TestFilterHistoryCopiesUnfilteredEntries(t *testing.T) {
	t.Parallel()

	entries := []HistoryEntry{{Domain: "example.com"}}
	filtered := FilterHistory(entries, HistoryFilter{})
	filtered[0].Domain = "changed.example"

	if entries[0].Domain != "example.com" {
		t.Fatalf("FilterHistory returned aliased slice")
	}
}
