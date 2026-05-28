package domains

import (
	"sort"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type ScanState string

const (
	ScanStateEmpty      ScanState = "empty"
	ScanStateHealthy    ScanState = "healthy"
	ScanStateFailed     ScanState = "failed"
	ScanStateInProgress ScanState = "in_progress"
	ScanStatePending    ScanState = "pending"
)

type DashboardDomain struct {
	ID            string
	Domain        string
	State         ScanState
	LastCheckedAt time.Time
	NextCheckAt   time.Time
	TopIssue      string
	Score         int
}

type DashboardOptions struct {
	Now           time.Time
	Cadence       time.Duration
	InProgressIDs map[string]struct{}
	FailedScanIDs map[string]struct{}
}

func BuildDashboard(saved []Domain, options DashboardOptions) []DashboardDomain {
	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cadence := options.Cadence
	if cadence <= 0 {
		cadence = 24 * time.Hour
	}
	items := make([]DashboardDomain, 0, len(saved))
	for _, domain := range saved {
		item := DashboardDomain{
			ID:     domain.ID,
			Domain: domain.Domain,
			State:  ScanStatePending,
		}
		if _, ok := options.InProgressIDs[domain.ID]; ok {
			item.State = ScanStateInProgress
		}
		if _, ok := options.FailedScanIDs[domain.ID]; ok {
			item.State = ScanStateFailed
		}
		if domain.LatestCheck == nil {
			if item.State == ScanStatePending {
				item.State = ScanStateEmpty
			}
			items = append(items, item)
			continue
		}
		item.LastCheckedAt = domain.LatestCheck.CheckedAt.UTC()
		item.NextCheckAt = item.LastCheckedAt.Add(cadence)
		summary := scanner.Summarize(scanner.Result{
			Domain: domain.Domain,
			SPF:    domain.LatestCheck.SPF,
			DKIM:   domain.LatestCheck.DKIM,
			DMARC:  domain.LatestCheck.DMARC,
			BIMI:   domain.LatestCheck.BIMI,
		})
		item.Score = summary.Score
		item.TopIssue = summary.TopIssue
		if item.State == ScanStatePending {
			item.State = ScanStateHealthy
			if summary.Status != scanner.StatusPass {
				item.State = ScanStateFailed
			}
			if !item.NextCheckAt.IsZero() && !item.NextCheckAt.After(now) {
				item.State = ScanStatePending
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].State == items[j].State {
			return items[i].Domain < items[j].Domain
		}
		return dashboardStateRank(items[i].State) > dashboardStateRank(items[j].State)
	})
	return items
}

func dashboardStateRank(state ScanState) int {
	switch state {
	case ScanStateFailed:
		return 5
	case ScanStateInProgress:
		return 4
	case ScanStatePending:
		return 3
	case ScanStateEmpty:
		return 2
	case ScanStateHealthy:
		return 1
	default:
		return 0
	}
}

type DashboardSettings struct {
	RequireAlignmentBeforeReject        int
	PauseAutomationOnUnknownSenderSpike bool
	AutoGenerateMonthlyExecutiveReports bool
	NotificationChannels                []string
	UpdatedAt                           time.Time
}

type RecheckJob struct {
	ID          string
	DomainID    string
	Status      string
	QueuedAt    time.Time
	NextAllowed time.Time
}

func DefaultDashboardSettings() DashboardSettings {
	return DashboardSettings{
		RequireAlignmentBeforeReject:        95,
		PauseAutomationOnUnknownSenderSpike: true,
		NotificationChannels:                []string{"email"},
	}
}

func NormalizeDashboardSettings(settings DashboardSettings) (DashboardSettings, error) {
	defaults := DefaultDashboardSettings()
	if settings.RequireAlignmentBeforeReject == 0 {
		settings.RequireAlignmentBeforeReject = defaults.RequireAlignmentBeforeReject
	}
	if settings.RequireAlignmentBeforeReject < 1 || settings.RequireAlignmentBeforeReject > 100 {
		return DashboardSettings{}, ErrInvalidDashboardSettings("require_alignment_before_reject must be between 1 and 100")
	}
	settings.NotificationChannels = normalizeDashboardChannels(settings.NotificationChannels)
	if len(settings.NotificationChannels) == 0 {
		settings.NotificationChannels = defaults.NotificationChannels
	}
	return settings, nil
}

type ErrInvalidDashboardSettings string

func (err ErrInvalidDashboardSettings) Error() string {
	return string(err)
}

func normalizeDashboardChannels(values []string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		switch value {
		case "email", "slack", "webhook":
		default:
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
