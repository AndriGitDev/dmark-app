package statuspage

import "time"

type ComponentStatus string

const (
	StatusOperational ComponentStatus = "operational"
	StatusDegraded    ComponentStatus = "degraded"
	StatusOutage      ComponentStatus = "outage"
)

type Component struct {
	ID        string
	Name      string
	Status    ComponentStatus
	UpdatedAt time.Time
}

type Incident struct {
	ID         string
	Title      string
	Status     string
	StartedAt  time.Time
	ResolvedAt time.Time
	CreatedAt  time.Time
	Updates    []IncidentUpdate
}

type IncidentUpdate struct {
	At      time.Time
	Message string
}

func OverallStatus(components []Component) ComponentStatus {
	overall := StatusOperational
	for _, component := range components {
		if component.Status == StatusOutage {
			return StatusOutage
		}
		if component.Status == StatusDegraded {
			overall = StatusDegraded
		}
	}
	return overall
}

func (incident Incident) IsResolved() bool {
	return !incident.ResolvedAt.IsZero()
}
