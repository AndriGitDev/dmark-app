package reporter

import (
	"sort"
	"time"

	"github.com/AndriGitDev/meil/internal/attribution"
)

type AggregateObservation struct {
	SourceIP    string
	Provider    string
	Confidence  attribution.Confidence
	Count       int
	FailedCount int
	ObservedAt  time.Time
	Expected    bool
	Ignored     bool
	Suspicious  bool
}

type InventoryEntry struct {
	SourceIP    string
	Provider    string
	Confidence  attribution.Confidence
	FirstSeen   time.Time
	LastSeen    time.Time
	Volume      int
	FailureRate float64
	Expected    bool
	Ignored     bool
	Suspicious  bool
}

func BuildInventory(observations []AggregateObservation) []InventoryEntry {
	bySource := map[string]*InventoryEntry{}
	failures := map[string]int{}
	for _, observation := range observations {
		entry := bySource[observation.SourceIP]
		if entry == nil {
			entry = &InventoryEntry{
				SourceIP:   observation.SourceIP,
				Provider:   observation.Provider,
				Confidence: observation.Confidence,
				FirstSeen:  observation.ObservedAt,
				LastSeen:   observation.ObservedAt,
			}
			bySource[observation.SourceIP] = entry
		}
		if observation.Provider != "" {
			entry.Provider = observation.Provider
		}
		if confidenceRank(observation.Confidence) > confidenceRank(entry.Confidence) {
			entry.Confidence = observation.Confidence
		}
		if entry.FirstSeen.IsZero() || observation.ObservedAt.Before(entry.FirstSeen) {
			entry.FirstSeen = observation.ObservedAt
		}
		if observation.ObservedAt.After(entry.LastSeen) {
			entry.LastSeen = observation.ObservedAt
		}
		entry.Volume += observation.Count
		failures[observation.SourceIP] += observation.FailedCount
		entry.Expected = entry.Expected || observation.Expected
		entry.Ignored = entry.Ignored || observation.Ignored
		entry.Suspicious = entry.Suspicious || observation.Suspicious
	}
	inventory := make([]InventoryEntry, 0, len(bySource))
	for source, entry := range bySource {
		if entry.Volume > 0 {
			entry.FailureRate = float64(failures[source]) / float64(entry.Volume)
		}
		inventory = append(inventory, *entry)
	}
	sort.Slice(inventory, func(i int, j int) bool {
		if inventory[i].Volume == inventory[j].Volume {
			return inventory[i].SourceIP < inventory[j].SourceIP
		}
		return inventory[i].Volume > inventory[j].Volume
	})
	return inventory
}

func ToSenderInventoryItem(entry InventoryEntry) SenderInventoryItem {
	return SenderInventoryItem{
		Provider:    entry.Provider,
		Expected:    entry.Expected,
		Ignored:     entry.Ignored,
		Suspicious:  entry.Suspicious,
		Volume:      entry.Volume,
		FailureRate: entry.FailureRate,
	}
}

func confidenceRank(confidence attribution.Confidence) int {
	switch confidence {
	case attribution.ConfidenceHigh:
		return 3
	case attribution.ConfidenceMedium:
		return 2
	case attribution.ConfidenceLow:
		return 1
	default:
		return 0
	}
}
