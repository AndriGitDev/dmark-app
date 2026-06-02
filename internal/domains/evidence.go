package domains

import (
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type EvidenceSnapshot struct {
	Domain     string          `json:"domain,omitempty"`
	CheckedAt  time.Time       `json:"checked_at,omitempty"`
	DurationMS int64           `json:"duration_ms,omitempty"`
	Checks     []CheckEvidence `json:"checks,omitempty"`
}

type CheckEvidence struct {
	Name             string             `json:"name"`
	Status           scanner.Status     `json:"status"`
	SourceName       string             `json:"source_name,omitempty"`
	LookupDurationMS int64              `json:"lookup_duration_ms,omitempty"`
	ErrorClass       string             `json:"error_class,omitempty"`
	RecordPresent    bool               `json:"record_present"`
	RecordLength     int                `json:"record_length,omitempty"`
	Selectors        []SelectorEvidence `json:"selectors,omitempty"`
}

type SelectorEvidence struct {
	Selector      string         `json:"selector"`
	Status        scanner.Status `json:"status"`
	RecordPresent bool           `json:"record_present"`
	RecordLength  int            `json:"record_length,omitempty"`
}

func NewScanEvidenceSnapshot(result scanner.Result) EvidenceSnapshot {
	return EvidenceSnapshot{
		Domain:     result.Domain,
		CheckedAt:  result.CheckedAt.UTC(),
		DurationMS: result.Duration.Milliseconds(),
		Checks: []CheckEvidence{
			newCheckEvidence(result.SPF),
			newCheckEvidence(result.DKIM),
			newCheckEvidence(result.DMARC),
			newCheckEvidence(result.BIMI),
		},
	}
}

func newCheckEvidence(check scanner.CheckResult) CheckEvidence {
	evidence := CheckEvidence{
		Name:             check.Name,
		Status:           check.Status,
		SourceName:       check.SourceName,
		LookupDurationMS: check.LookupDuration.Milliseconds(),
		ErrorClass:       check.ErrorClass,
		RecordPresent:    check.Record != "",
		RecordLength:     len(check.Record),
	}
	if len(check.Selectors) > 0 {
		evidence.Selectors = make([]SelectorEvidence, 0, len(check.Selectors))
		for _, selector := range check.Selectors {
			evidence.Selectors = append(evidence.Selectors, SelectorEvidence{
				Selector:      selector.Selector,
				Status:        selector.Status,
				RecordPresent: selector.Record != "",
				RecordLength:  len(selector.Record),
			})
		}
	}
	return evidence
}
