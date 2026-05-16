package reporter

import "sort"

type UnknownSenderRule struct {
	MinimumVolume      int
	MinimumFailureRate float64
}

type UnknownSenderAlert struct {
	Provider    string
	Volume      int
	FailureRate float64
	Reason      string
}

func DetectUnknownSenders(items []SenderInventoryItem, rule UnknownSenderRule) []UnknownSenderAlert {
	if rule.MinimumVolume <= 0 {
		rule.MinimumVolume = 10
	}
	alerts := []UnknownSenderAlert{}
	for _, item := range items {
		if item.Expected || item.Ignored || item.Volume < rule.MinimumVolume {
			continue
		}
		risk := InventoryRisk(item)
		if risk != "high" && risk != "review" && item.FailureRate < rule.MinimumFailureRate {
			continue
		}
		reason := "sender is not marked expected"
		if item.Provider == "" {
			reason = "sender provider is unclassified"
		}
		if item.Suspicious {
			reason = "sender is marked suspicious"
		}
		alerts = append(alerts, UnknownSenderAlert{
			Provider:    item.Provider,
			Volume:      item.Volume,
			FailureRate: item.FailureRate,
			Reason:      reason,
		})
	}
	sort.Slice(alerts, func(i int, j int) bool {
		if alerts[i].Volume == alerts[j].Volume {
			return alerts[i].FailureRate > alerts[j].FailureRate
		}
		return alerts[i].Volume > alerts[j].Volume
	})
	return alerts
}
