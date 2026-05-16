package domains

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func ParseCSV(reader io.Reader) ([]string, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading domain CSV: %w", err)
	}
	domains := []string{}
	seen := map[string]struct{}{}
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		value := strings.TrimSpace(row[0])
		if value == "" {
			continue
		}
		if i == 0 && strings.EqualFold(value, "domain") {
			continue
		}
		domain, err := scanner.NormalizeDomain(value)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	return domains, nil
}

func WriteCSV(writer io.Writer, saved []Domain) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"domain", "created_at", "last_checked_at", "spf_status", "dkim_status", "dmarc_status", "bimi_status"}); err != nil {
		return err
	}
	for _, domain := range saved {
		row := []string{domain.Domain, domain.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), "", "", "", "", ""}
		if domain.LatestCheck != nil {
			row[2] = domain.LatestCheck.CheckedAt.Format("2006-01-02T15:04:05Z07:00")
			row[3] = string(domain.LatestCheck.SPF.Status)
			row[4] = string(domain.LatestCheck.DKIM.Status)
			row[5] = string(domain.LatestCheck.DMARC.Status)
			row[6] = string(domain.LatestCheck.BIMI.Status)
		}
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}
