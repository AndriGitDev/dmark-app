package reporter

import (
	"sort"
	"time"
)

type AggregateRow struct {
	Domain            string
	SourceIP          string
	SourceOrg         string
	SourceASN         string
	SourceCountry     string
	HeaderFrom        string
	Disposition       string
	DKIMAlignedResult string
	SPFAlignedResult  string
	Provider          string
	Confidence        string
	Count             int
	BeginAt           time.Time
	EndAt             time.Time
}

type DashboardRow struct {
	SourceIP      string
	HeaderFrom    string
	Provider      string
	Confidence    string
	SourceOrg     string
	SourceASN     string
	SourceCountry string
	FirstSeen     time.Time
	LastSeen      time.Time
	TotalCount    int
	PassCount     int
	FailCount     int
	Disposition   map[string]int
}

func BuildDashboardRows(rows []AggregateRow) []DashboardRow {
	byKey := map[string]*DashboardRow{}
	for _, row := range rows {
		key := row.SourceIP + "\x00" + row.HeaderFrom + "\x00" + row.Provider
		item := byKey[key]
		if item == nil {
			item = &DashboardRow{
				SourceIP:      row.SourceIP,
				HeaderFrom:    row.HeaderFrom,
				Provider:      row.Provider,
				Confidence:    row.Confidence,
				SourceOrg:     row.SourceOrg,
				SourceASN:     row.SourceASN,
				SourceCountry: row.SourceCountry,
				FirstSeen:     row.BeginAt,
				LastSeen:      row.EndAt,
				Disposition:   map[string]int{},
			}
			byKey[key] = item
		}
		if item.FirstSeen.IsZero() || row.BeginAt.Before(item.FirstSeen) {
			item.FirstSeen = row.BeginAt
		}
		if row.EndAt.After(item.LastSeen) {
			item.LastSeen = row.EndAt
		}
		item.TotalCount += row.Count
		if row.DKIMAlignedResult == "pass" || row.SPFAlignedResult == "pass" {
			item.PassCount += row.Count
		} else {
			item.FailCount += row.Count
		}
		item.Disposition[row.Disposition] += row.Count
	}
	result := make([]DashboardRow, 0, len(byKey))
	for _, item := range byKey {
		result = append(result, *item)
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].TotalCount == result[j].TotalCount {
			return result[i].SourceIP < result[j].SourceIP
		}
		return result[i].TotalCount > result[j].TotalCount
	})
	return result
}
