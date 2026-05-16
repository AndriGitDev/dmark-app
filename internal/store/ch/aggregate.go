package ch

import (
	"time"

	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/ingest"
)

type AggregateRow struct {
	Domain            string
	ReportID          string
	OrgName           string
	BeginAt           time.Time
	EndAt             time.Time
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
}

type ReportContext struct {
	SourceOrg     string
	SourceASN     string
	SourceCountry string
	Attribution   map[string]attribution.Result
}

func RowsFromReport(report ingest.AggregateReport, context ReportContext) []AggregateRow {
	rows := make([]AggregateRow, 0, len(report.Records))
	beginAt := time.Unix(report.BeginUnix, 0).UTC()
	endAt := time.Unix(report.EndUnix, 0).UTC()
	for _, record := range report.Records {
		ip := record.SourceIP.String()
		match := context.Attribution[ip]
		rows = append(rows, AggregateRow{
			Domain:            report.Domain,
			ReportID:          report.ReportID,
			OrgName:           report.OrgName,
			BeginAt:           beginAt,
			EndAt:             endAt,
			SourceIP:          ip,
			SourceOrg:         context.SourceOrg,
			SourceASN:         context.SourceASN,
			SourceCountry:     context.SourceCountry,
			HeaderFrom:        record.HeaderFrom,
			Disposition:       record.Disposition,
			DKIMAlignedResult: record.DKIMAlignedResult,
			SPFAlignedResult:  record.SPFAlignedResult,
			Provider:          match.Provider,
			Confidence:        string(match.Confidence),
			Count:             record.Count,
		})
	}
	return rows
}

type DateRangeQuery struct {
	Domain string
	From   time.Time
	To     time.Time
	Limit  int
}

func (query DateRangeQuery) SQL() (string, []any) {
	limit := query.Limit
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	return `SELECT
    source_ip,
    header_from,
    provider,
    confidence,
    sum(count) AS total_count,
    sumIf(count, dkim_aligned_result = 'pass' OR spf_aligned_result = 'pass') AS pass_count,
    sumIf(count, dkim_aligned_result != 'pass' AND spf_aligned_result != 'pass') AS fail_count
FROM dmarc_aggregate_rows
WHERE domain = ? AND begin_at >= ? AND begin_at < ?
GROUP BY source_ip, header_from, provider, confidence
ORDER BY total_count DESC
LIMIT ?`, []any{query.Domain, query.From.UTC(), query.To.UTC(), limit}
}
