package reporter

import "time"

// AlignmentDay is one day of aggregate DMARC alignment volume for a domain.
type AlignmentDay struct {
	Day     time.Time
	Total   int
	Aligned int
}

func (d AlignmentDay) AlignedPercentage() float64 {
	if d.Total <= 0 {
		return 0
	}
	return float64(d.Aligned) / float64(d.Total) * 100
}
