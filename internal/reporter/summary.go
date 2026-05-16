package reporter

import (
	"net"
	"sort"

	"github.com/AndriGitDev/meil/internal/ingest"
)

type SourceSummary struct {
	SourceIP      net.IP
	HeaderFrom    string
	TotalCount    int
	PassCount     int
	FailCount     int
	Disposition   map[string]int
	Organizations map[string]int
}

func SummarizeSources(rows []ingest.AggregateRecord, orgName string) []SourceSummary {
	byKey := map[string]*SourceSummary{}
	for _, row := range rows {
		key := row.SourceIP.String() + "\x00" + row.HeaderFrom
		summary := byKey[key]
		if summary == nil {
			summary = &SourceSummary{
				SourceIP:      append(net.IP(nil), row.SourceIP...),
				HeaderFrom:    row.HeaderFrom,
				Disposition:   map[string]int{},
				Organizations: map[string]int{},
			}
			byKey[key] = summary
		}
		summary.TotalCount += row.Count
		if row.DKIMAlignedResult == "pass" || row.SPFAlignedResult == "pass" {
			summary.PassCount += row.Count
		} else {
			summary.FailCount += row.Count
		}
		summary.Disposition[row.Disposition] += row.Count
		if orgName != "" {
			summary.Organizations[orgName] += row.Count
		}
	}
	summaries := make([]SourceSummary, 0, len(byKey))
	for _, summary := range byKey {
		summaries = append(summaries, *summary)
	}
	sort.Slice(summaries, func(i int, j int) bool {
		if summaries[i].TotalCount == summaries[j].TotalCount {
			return summaries[i].SourceIP.String() < summaries[j].SourceIP.String()
		}
		return summaries[i].TotalCount > summaries[j].TotalCount
	})
	return summaries
}

type SenderInventoryItem struct {
	Provider    string
	Expected    bool
	Ignored     bool
	Suspicious  bool
	Volume      int
	FailureRate float64
}

func InventoryRisk(item SenderInventoryItem) string {
	if item.Expected && !item.Suspicious && item.FailureRate < 0.05 {
		return "expected"
	}
	if item.Suspicious || item.FailureRate >= 0.50 {
		return "high"
	}
	if item.Provider == "" || item.FailureRate >= 0.10 {
		return "review"
	}
	return "low"
}
