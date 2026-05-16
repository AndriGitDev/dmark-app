package reporter

import (
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/attribution"
)

func TestBuildInventoryMergesSourceHistory(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)

	inventory := BuildInventory([]AggregateObservation{
		{
			SourceIP:    "203.0.113.10",
			Provider:    "Unknown",
			Confidence:  attribution.ConfidenceNone,
			Count:       20,
			FailedCount: 5,
			ObservedAt:  first,
		},
		{
			SourceIP:    "203.0.113.10",
			Provider:    "Google Workspace",
			Confidence:  attribution.ConfidenceHigh,
			Count:       30,
			FailedCount: 0,
			ObservedAt:  last,
			Expected:    true,
		},
		{
			SourceIP:    "203.0.113.20",
			Provider:    "Suspicious ESP",
			Confidence:  attribution.ConfidenceMedium,
			Count:       100,
			FailedCount: 90,
			ObservedAt:  last,
			Suspicious:  true,
		},
	})

	if len(inventory) != 2 {
		t.Fatalf("len(inventory) = %d, want 2", len(inventory))
	}
	if inventory[0].SourceIP != "203.0.113.20" || inventory[0].Volume != 100 {
		t.Fatalf("inventory not sorted by volume: %+v", inventory)
	}

	entry := inventory[1]
	if entry.SourceIP != "203.0.113.10" {
		t.Fatalf("merged entry source = %q, want 203.0.113.10", entry.SourceIP)
	}
	if entry.Provider != "Google Workspace" || entry.Confidence != attribution.ConfidenceHigh {
		t.Fatalf("provider/confidence not promoted: %+v", entry)
	}
	if !entry.FirstSeen.Equal(first) || !entry.LastSeen.Equal(last) {
		t.Fatalf("first/last seen = %s/%s, want %s/%s", entry.FirstSeen, entry.LastSeen, first, last)
	}
	if entry.Volume != 50 || entry.FailureRate != 0.10 || !entry.Expected {
		t.Fatalf("unexpected merged metrics: %+v", entry)
	}
}

func TestToSenderInventoryItemPreservesRiskFields(t *testing.T) {
	t.Parallel()

	item := ToSenderInventoryItem(InventoryEntry{
		Provider:    "Known",
		Volume:      25,
		FailureRate: 0.2,
		Expected:    true,
		Ignored:     true,
		Suspicious:  true,
	})

	if item.Provider != "Known" || item.Volume != 25 || item.FailureRate != 0.2 || !item.Expected || !item.Ignored || !item.Suspicious {
		t.Fatalf("item = %+v, want all risk fields preserved", item)
	}
}
