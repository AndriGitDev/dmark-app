package privacy

import "fmt"

type ExportManifest struct {
	UserID         string
	Email          string
	Sections       []Section
	IncludesRawXML bool
}

type Section struct {
	Name        string
	Description string
}

func NewExportManifest(userID string, email string, includeRawXML bool) ExportManifest {
	sections := []Section{
		{Name: "account", Description: "User email, account creation time, and session-independent profile data."},
		{Name: "domains", Description: "Protected domains, domain settings, latest checks, and custom selectors."},
		{Name: "checks", Description: "Saved DNS check snapshots for SPF, DKIM, DMARC, and BIMI."},
		{Name: "drift_events", Description: "Detected DNS posture changes, acknowledgements, mutes, and alert state."},
		{Name: "notifications", Description: "Notification dispatch audit rows and provider delivery status."},
		{Name: "reporter", Description: "DMARC report metadata, aggregate rows, sender attributions, and expected sender flags."},
		{Name: "audit_log", Description: "Security-sensitive actions tied to the account or organizations."},
	}
	if includeRawXML {
		sections = append(sections, Section{Name: "raw_dmarc_xml", Description: "Raw retained DMARC aggregate XML source material."})
	}
	return ExportManifest{UserID: userID, Email: email, Sections: sections, IncludesRawXML: includeRawXML}
}

type DeletionPlan struct {
	UserID string
	Steps  []DeletionStep
}

type DeletionStep struct {
	Name        string
	Description string
	Destructive bool
}

func NewDeletionPlan(userID string) DeletionPlan {
	return DeletionPlan{
		UserID: userID,
		Steps: []DeletionStep{
			{Name: "revoke_api_keys", Description: "Revoke organization API keys owned by the user.", Destructive: true},
			{Name: "remove_invitations", Description: "Remove pending organization invitations for the user email.", Destructive: true},
			{Name: "delete_domains", Description: "Delete protected domains; cascades checks, settings, drift events, notifications, reports, and attributions.", Destructive: true},
			{Name: "remove_memberships", Description: "Remove organization memberships or transfer ownership before deletion.", Destructive: true},
			{Name: "delete_user", Description: "Delete the user row; cascades login tokens.", Destructive: true},
			{Name: "retain_audit_tombstone", Description: "Retain minimal audit tombstones required for security and billing integrity.", Destructive: false},
		},
	}
}

type RetentionRequest struct {
	RawReportDays       int `json:"raw_report_days"`
	AggregateRowDays    int `json:"aggregate_row_days"`
	NotificationLogDays int `json:"notification_log_days"`
}

type RetentionLimits struct {
	MaxRawReportDays       int
	MaxAggregateRowDays    int
	MaxNotificationLogDays int
}

func ValidateRetentionRequest(request RetentionRequest, limits RetentionLimits) (RetentionRequest, error) {
	if request.RawReportDays < 0 || request.AggregateRowDays < 0 || request.NotificationLogDays < 0 {
		return RetentionRequest{}, fmt.Errorf("retention days cannot be negative")
	}
	if request.RawReportDays > limits.MaxRawReportDays {
		return RetentionRequest{}, fmt.Errorf("raw report retention exceeds plan limit")
	}
	if request.AggregateRowDays > limits.MaxAggregateRowDays {
		return RetentionRequest{}, fmt.Errorf("aggregate row retention exceeds plan limit")
	}
	if request.NotificationLogDays > limits.MaxNotificationLogDays {
		return RetentionRequest{}, fmt.Errorf("notification retention exceeds plan limit")
	}
	if request.RawReportDays > 0 && request.AggregateRowDays > 0 && request.RawReportDays > request.AggregateRowDays {
		return RetentionRequest{}, fmt.Errorf("raw report retention cannot exceed aggregate row retention")
	}
	return request, nil
}
