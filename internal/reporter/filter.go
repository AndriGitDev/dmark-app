package reporter

import (
	"net"
	"sort"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/ingest"
)

type RowObservation struct {
	Domain     string
	Provider   string
	ObservedAt time.Time
	Record     ingest.AggregateRecord
}

type Filter struct {
	Domain   string
	Provider string
	SourceIP net.IP
	Result   string
	From     time.Time
	To       time.Time
}

func ApplyFilter(rows []RowObservation, filter Filter) []RowObservation {
	filter.Domain = strings.ToLower(strings.TrimSpace(filter.Domain))
	filter.Provider = strings.ToLower(strings.TrimSpace(filter.Provider))
	filter.Result = strings.ToLower(strings.TrimSpace(filter.Result))

	matched := []RowObservation{}
	for _, row := range rows {
		if filter.Domain != "" && strings.ToLower(row.Domain) != filter.Domain {
			continue
		}
		if filter.Provider != "" && strings.ToLower(row.Provider) != filter.Provider {
			continue
		}
		if filter.SourceIP != nil && !row.Record.SourceIP.Equal(filter.SourceIP) {
			continue
		}
		if filter.Result != "" && !recordMatchesResult(row.Record, filter.Result) {
			continue
		}
		if !filter.From.IsZero() && row.ObservedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && row.ObservedAt.After(filter.To) {
			continue
		}
		matched = append(matched, row)
	}
	sort.Slice(matched, func(i int, j int) bool {
		if matched[i].ObservedAt.Equal(matched[j].ObservedAt) {
			return matched[i].Record.SourceIP.String() < matched[j].Record.SourceIP.String()
		}
		return matched[i].ObservedAt.Before(matched[j].ObservedAt)
	})
	return matched
}

func recordMatchesResult(record ingest.AggregateRecord, result string) bool {
	dkim := strings.ToLower(record.DKIMAlignedResult)
	spf := strings.ToLower(record.SPFAlignedResult)
	switch result {
	case "pass", "aligned":
		return dkim == "pass" || spf == "pass"
	case "fail", "unaligned":
		return dkim != "pass" && spf != "pass"
	default:
		return strings.ToLower(record.Disposition) == result
	}
}
