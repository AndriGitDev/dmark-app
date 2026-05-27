package api

import (
	"time"

	"github.com/AndriGitDev/meil/internal/admin"
	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/billing"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/statuspage"
	"github.com/AndriGitDev/meil/internal/teams"
	"github.com/AndriGitDev/meil/internal/webhooks"
)

type ScanResponse struct {
	Domain     string          `json:"domain"`
	CheckedAt  time.Time       `json:"checked_at"`
	DurationMS int64           `json:"duration_ms"`
	Summary    SummaryResponse `json:"summary"`
	Audit      AuditResponse   `json:"audit"`
	Checks     []CheckResponse `json:"checks"`
}

type SummaryResponse struct {
	Score       int            `json:"score"`
	Status      scanner.Status `json:"status"`
	TopIssue    string         `json:"top_issue"`
	Remediation string         `json:"remediation"`
}

type CheckResponse struct {
	Name             string                   `json:"name"`
	Status           scanner.Status           `json:"status"`
	Message          string                   `json:"message"`
	Record           string                   `json:"record,omitempty"`
	SourceName       string                   `json:"source_name,omitempty"`
	LookupDurationMS int64                    `json:"lookup_duration_ms"`
	ErrorClass       string                   `json:"error_class,omitempty"`
	Selectors        []scanner.SelectorResult `json:"selectors,omitempty"`
}

type AuditResponse struct {
	Grade         string               `json:"grade"`
	Risk          salesaudit.RiskLevel `json:"risk"`
	Safe          bool                 `json:"safe"`
	ProofEligible bool                 `json:"proof_eligible"`
	Message       string               `json:"message"`
	CTA           string               `json:"cta"`
	BlockedReason string               `json:"blocked_reason,omitempty"`
}

func NewScanResponse(result scanner.Result) ScanResponse {
	summary := scanner.Summarize(result)
	audit := salesaudit.Assess(result, salesaudit.Options{})
	return ScanResponse{
		Domain:     result.Domain,
		CheckedAt:  result.CheckedAt.UTC(),
		DurationMS: result.Duration.Milliseconds(),
		Summary: SummaryResponse{
			Score:       summary.Score,
			Status:      summary.Status,
			TopIssue:    summary.TopIssue,
			Remediation: summary.Remediation,
		},
		Audit: AuditResponse{
			Grade:         audit.Grade,
			Risk:          audit.Risk,
			Safe:          audit.Safe,
			ProofEligible: audit.ProofEligible,
			Message:       audit.Message,
			CTA:           audit.CTA,
			BlockedReason: audit.BlockedReason,
		},
		Checks: []CheckResponse{
			newCheckResponse(result.SPF),
			newCheckResponse(result.DKIM),
			newCheckResponse(result.DMARC),
			newCheckResponse(result.BIMI),
		},
	}
}

func newCheckResponse(check scanner.CheckResult) CheckResponse {
	return CheckResponse{
		Name:             check.Name,
		Status:           check.Status,
		Message:          check.Message,
		Record:           check.Record,
		SourceName:       check.SourceName,
		LookupDurationMS: check.LookupDuration.Milliseconds(),
		ErrorClass:       check.ErrorClass,
		Selectors:        check.Selectors,
	}
}

type DomainResponse struct {
	ID             string         `json:"id"`
	Domain         string         `json:"domain"`
	CreatedAt      time.Time      `json:"created_at"`
	LatestCheck    *CheckSnapshot `json:"latest_check,omitempty"`
	DashboardState string         `json:"dashboard_state,omitempty"`
}

type CheckSnapshot struct {
	CheckedAt time.Time     `json:"checked_at"`
	SPF       CheckResponse `json:"spf"`
	DKIM      CheckResponse `json:"dkim"`
	DMARC     CheckResponse `json:"dmarc"`
	BIMI      CheckResponse `json:"bimi"`
}

func NewDomainResponse(domain domains.Domain) DomainResponse {
	response := DomainResponse{
		ID:        domain.ID,
		Domain:    domain.Domain,
		CreatedAt: domain.CreatedAt.UTC(),
	}
	if domain.LatestCheck != nil {
		snapshot := NewCheckSnapshot(*domain.LatestCheck)
		response.LatestCheck = &snapshot
	}
	return response
}

func NewCheckSnapshot(check domains.Check) CheckSnapshot {
	return CheckSnapshot{
		CheckedAt: check.CheckedAt.UTC(),
		SPF:       newCheckResponse(check.SPF),
		DKIM:      newCheckResponse(check.DKIM),
		DMARC:     newCheckResponse(check.DMARC),
		BIMI:      newCheckResponse(check.BIMI),
	}
}

type HistoryResponse struct {
	DomainID string        `json:"domain_id"`
	Domain   string        `json:"domain"`
	Check    CheckSnapshot `json:"check"`
}

func NewHistoryResponse(entry domains.HistoryEntry) HistoryResponse {
	return HistoryResponse{
		DomainID: entry.DomainID,
		Domain:   entry.Domain,
		Check:    NewCheckSnapshot(entry.Check),
	}
}

type DriftEventResponse struct {
	CheckName    string         `json:"check_name"`
	Kind         drift.Kind     `json:"kind"`
	Severity     drift.Severity `json:"severity"`
	Message      string         `json:"message"`
	BeforeStatus scanner.Status `json:"before_status,omitempty"`
	AfterStatus  scanner.Status `json:"after_status,omitempty"`
	BeforeRecord string         `json:"before_record,omitempty"`
	AfterRecord  string         `json:"after_record,omitempty"`
}

func NewDriftEventResponse(event drift.Event) DriftEventResponse {
	return DriftEventResponse{
		CheckName:    event.CheckName,
		Kind:         event.Kind,
		Severity:     event.Severity,
		Message:      event.Message,
		BeforeStatus: event.BeforeStatus,
		AfterStatus:  event.AfterStatus,
		BeforeRecord: event.BeforeRecord,
		AfterRecord:  event.AfterRecord,
	}
}

type ReporterSourceResponse struct {
	SourceIP      string         `json:"source_ip"`
	HeaderFrom    string         `json:"header_from"`
	Provider      string         `json:"provider,omitempty"`
	Confidence    string         `json:"confidence,omitempty"`
	SourceOrg     string         `json:"source_org,omitempty"`
	SourceASN     string         `json:"source_asn,omitempty"`
	SourceCountry string         `json:"source_country,omitempty"`
	FirstSeen     time.Time      `json:"first_seen"`
	LastSeen      time.Time      `json:"last_seen"`
	TotalCount    int            `json:"total_count"`
	PassCount     int            `json:"pass_count"`
	FailCount     int            `json:"fail_count"`
	Disposition   map[string]int `json:"disposition"`
}

func NewReporterSourceResponse(row reporter.DashboardRow) ReporterSourceResponse {
	return ReporterSourceResponse{
		SourceIP:      row.SourceIP,
		HeaderFrom:    row.HeaderFrom,
		Provider:      row.Provider,
		Confidence:    row.Confidence,
		SourceOrg:     row.SourceOrg,
		SourceASN:     row.SourceASN,
		SourceCountry: row.SourceCountry,
		FirstSeen:     row.FirstSeen.UTC(),
		LastSeen:      row.LastSeen.UTC(),
		TotalCount:    row.TotalCount,
		PassCount:     row.PassCount,
		FailCount:     row.FailCount,
		Disposition:   row.Disposition,
	}
}

type ReporterSenderResponse struct {
	SourceIP    string    `json:"source_ip"`
	Provider    string    `json:"provider,omitempty"`
	Confidence  string    `json:"confidence"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Volume      int       `json:"volume"`
	FailureRate float64   `json:"failure_rate"`
	Expected    bool      `json:"expected"`
	Ignored     bool      `json:"ignored"`
	Suspicious  bool      `json:"suspicious"`
	Risk        string    `json:"risk"`
}

func NewReporterSenderResponse(entry reporter.InventoryEntry) ReporterSenderResponse {
	return ReporterSenderResponse{
		SourceIP:    entry.SourceIP,
		Provider:    entry.Provider,
		Confidence:  string(entry.Confidence),
		FirstSeen:   entry.FirstSeen.UTC(),
		LastSeen:    entry.LastSeen.UTC(),
		Volume:      entry.Volume,
		FailureRate: entry.FailureRate,
		Expected:    entry.Expected,
		Ignored:     entry.Ignored,
		Suspicious:  entry.Suspicious,
		Risk:        reporter.InventoryRisk(reporter.ToSenderInventoryItem(entry)),
	}
}

type ReporterHealthResponse struct {
	LastReportAt time.Time `json:"last_report_at,omitempty"`
	ReporterOrgs []string  `json:"reporter_orgs"`
	MissingOrgs  []string  `json:"missing_orgs"`
	FailureCount int       `json:"failure_count"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
}

func NewReporterHealthResponse(result reporter.HealthResult) ReporterHealthResponse {
	return ReporterHealthResponse{
		LastReportAt: result.LastReportAt.UTC(),
		ReporterOrgs: result.ReporterOrgs,
		MissingOrgs:  result.MissingOrgs,
		FailureCount: result.FailureCount,
		Status:       result.Status,
		Message:      result.Message,
	}
}

type WebhookEndpointResponse struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	EventTypes []string  `json:"event_types"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Secret     string    `json:"secret,omitempty"`
}

func NewWebhookEndpointResponse(endpoint webhooks.Endpoint, secret string) WebhookEndpointResponse {
	return WebhookEndpointResponse{
		ID:         endpoint.ID,
		URL:        endpoint.URL,
		EventTypes: endpoint.EventTypes,
		Enabled:    endpoint.Enabled,
		CreatedAt:  endpoint.CreatedAt.UTC(),
		UpdatedAt:  endpoint.UpdatedAt.UTC(),
		Secret:     secret,
	}
}

type BillingCancellationResponse struct {
	OrganizationID string    `json:"organization_id"`
	Reason         string    `json:"reason,omitempty"`
	RequestedAt    time.Time `json:"requested_at"`
	EffectiveAt    time.Time `json:"effective_at"`
}

func NewBillingCancellationResponse(cancellation billing.Cancellation) BillingCancellationResponse {
	return BillingCancellationResponse{
		OrganizationID: cancellation.OrganizationID,
		Reason:         cancellation.Reason,
		RequestedAt:    cancellation.RequestedAt.UTC(),
		EffectiveAt:    cancellation.EffectiveAt.UTC(),
	}
}

type APIKeyResponse struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Scopes     []apikeys.Scope `json:"scopes"`
	Token      string          `json:"token,omitempty"`
	LastUsedAt *time.Time      `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time      `json:"revoked_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

func NewAPIKeyResponse(key apikeys.Key) APIKeyResponse {
	response := APIKeyResponse{
		ID:        key.ID,
		Name:      key.Name,
		Scopes:    key.Scopes,
		Token:     key.Token,
		CreatedAt: key.CreatedAt.UTC(),
	}
	if !key.LastUsedAt.IsZero() {
		lastUsedAt := key.LastUsedAt.UTC()
		response.LastUsedAt = &lastUsedAt
	}
	if !key.RevokedAt.IsZero() {
		revokedAt := key.RevokedAt.UTC()
		response.RevokedAt = &revokedAt
	}
	return response
}

type TeamInvitationResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Token      string     `json:"token,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func NewTeamInvitationResponse(invitation teams.Invitation) TeamInvitationResponse {
	response := TeamInvitationResponse{
		ID:        invitation.ID,
		Email:     invitation.Email,
		Role:      string(invitation.Role),
		Token:     invitation.Token,
		ExpiresAt: invitation.ExpiresAt.UTC(),
		CreatedAt: invitation.CreatedAt.UTC(),
	}
	if !invitation.AcceptedAt.IsZero() {
		acceptedAt := invitation.AcceptedAt.UTC()
		response.AcceptedAt = &acceptedAt
	}
	return response
}

type DomainTransferResponse struct {
	ID                string     `json:"id"`
	DomainID          string     `json:"domain_id"`
	FromOrganization  string     `json:"from_organization_id"`
	ToOrganization    string     `json:"to_organization_id"`
	RequestedByUserID string     `json:"requested_by_user_id"`
	Status            string     `json:"status"`
	RequestedAt       time.Time  `json:"requested_at"`
	DecidedAt         *time.Time `json:"decided_at,omitempty"`
	DecidedBy         string     `json:"decided_by,omitempty"`
}

func NewDomainTransferResponse(request teams.TransferRequest) DomainTransferResponse {
	response := DomainTransferResponse{
		ID:                request.ID,
		DomainID:          request.DomainID,
		FromOrganization:  request.FromOrganization,
		ToOrganization:    request.ToOrganization,
		RequestedByUserID: request.RequestedByUserID,
		Status:            string(request.Status),
		RequestedAt:       request.RequestedAt.UTC(),
		DecidedBy:         request.DecidedBy,
	}
	if !request.DecidedAt.IsZero() {
		decidedAt := request.DecidedAt.UTC()
		response.DecidedAt = &decidedAt
	}
	return response
}

type AdminCaseResponse struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	Subject    string     `json:"subject"`
	UserID     string     `json:"user_id,omitempty"`
	DomainID   string     `json:"domain_id,omitempty"`
	AssignedTo string     `json:"assigned_to,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

func NewAdminCaseResponse(c admin.Case) AdminCaseResponse {
	response := AdminCaseResponse{
		ID:         c.ID,
		Kind:       string(c.Kind),
		Status:     string(c.Status),
		Subject:    c.Subject,
		UserID:     c.UserID,
		DomainID:   c.DomainID,
		AssignedTo: c.AssignedTo,
		CreatedAt:  c.CreatedAt.UTC(),
	}
	if !c.ResolvedAt.IsZero() {
		resolvedAt := c.ResolvedAt.UTC()
		response.ResolvedAt = &resolvedAt
	}
	return response
}

type AbuseDecisionResponse struct {
	ID         string     `json:"id"`
	Identifier string     `json:"identifier"`
	Action     string     `json:"action"`
	Reason     string     `json:"reason"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func NewAbuseDecisionResponse(decision admin.AbuseDecision) AbuseDecisionResponse {
	response := AbuseDecisionResponse{
		ID:         decision.ID,
		Identifier: decision.Identifier,
		Action:     string(decision.Action),
		Reason:     decision.Reason,
		CreatedAt:  decision.CreatedAt.UTC(),
	}
	if !decision.ExpiresAt.IsZero() {
		expiresAt := decision.ExpiresAt.UTC()
		response.ExpiresAt = &expiresAt
	}
	return response
}

type StatusComponentResponse struct {
	ID        string                     `json:"id"`
	Name      string                     `json:"name"`
	Status    statuspage.ComponentStatus `json:"status"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

func NewStatusComponentResponse(component statuspage.Component) StatusComponentResponse {
	return StatusComponentResponse{
		ID:        component.ID,
		Name:      component.Name,
		Status:    component.Status,
		UpdatedAt: component.UpdatedAt.UTC(),
	}
}

type StatusIncidentResponse struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func NewStatusIncidentResponse(incident statuspage.Incident) StatusIncidentResponse {
	response := StatusIncidentResponse{
		ID:        incident.ID,
		Title:     incident.Title,
		Status:    incident.Status,
		StartedAt: incident.StartedAt.UTC(),
		CreatedAt: incident.CreatedAt.UTC(),
	}
	if !incident.ResolvedAt.IsZero() {
		resolvedAt := incident.ResolvedAt.UTC()
		response.ResolvedAt = &resolvedAt
	}
	return response
}
