package ingest

import "strings"

type ReportKey struct {
	OrgName     string
	ReportID    string
	Domain      string
	ContentHash string
}

func Key(report AggregateReport) ReportKey {
	return ReportKey{
		OrgName:     strings.ToLower(strings.TrimSpace(report.OrgName)),
		ReportID:    strings.TrimSpace(report.ReportID),
		Domain:      strings.ToLower(strings.TrimSpace(report.Domain)),
		ContentHash: strings.TrimSpace(report.ContentHash),
	}
}

func (key ReportKey) String() string {
	return key.Domain + "\x00" + key.OrgName + "\x00" + key.ReportID + "\x00" + key.ContentHash
}

type ImportDecision struct {
	Report    AggregateReport
	Duplicate bool
	Reason    string
}

func Deduplicate(reports []AggregateReport, existing map[string]struct{}) []ImportDecision {
	seen := map[string]struct{}{}
	decisions := make([]ImportDecision, 0, len(reports))
	for _, report := range reports {
		key := Key(report).String()
		decision := ImportDecision{Report: report}
		if _, ok := existing[key]; ok {
			decision.Duplicate = true
			decision.Reason = "already imported"
		} else if _, ok := seen[key]; ok {
			decision.Duplicate = true
			decision.Reason = "duplicate in batch"
		} else {
			seen[key] = struct{}{}
		}
		decisions = append(decisions, decision)
	}
	return decisions
}
