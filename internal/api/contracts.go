package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/admin"
	"github.com/AndriGitDev/meil/internal/alerts"
	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/billing"
	"github.com/AndriGitDev/meil/internal/brandrisk"
	"github.com/AndriGitDev/meil/internal/dmarcjourney"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/ops"
	"github.com/AndriGitDev/meil/internal/recommendations"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/spfroutes"
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

type DashboardResponse struct {
	GeneratedAt          time.Time                 `json:"generated_at"`
	ProtectedDomainCount int                       `json:"protected_domain_count"`
	AverageScore         int                       `json:"average_score"`
	AlertCount           int                       `json:"alert_count"`
	UnknownSenderCount   int                       `json:"unknown_sender_count"`
	Domains              []DashboardDomainResponse `json:"domains"`
}

type DashboardDomainResponse struct {
	ID                 string    `json:"id"`
	Domain             string    `json:"domain"`
	CreatedAt          time.Time `json:"created_at"`
	LastCheckedAt      time.Time `json:"last_checked_at,omitempty"`
	Status             string    `json:"status"`
	Score              int       `json:"score"`
	DMARCPolicy        string    `json:"dmarc_policy"`
	SPFLookupCount     int       `json:"spf_lookup_count"`
	DKIMCoverage       string    `json:"dkim_coverage"`
	AlignmentRate      float64   `json:"alignment_rate"`
	UnknownSenderCount int       `json:"unknown_sender_count"`
	NextAction         string    `json:"next_action"`
}

type DashboardDomainDetailResponse struct {
	Domain  DashboardDomainResponse `json:"domain"`
	History []HistoryResponse       `json:"history"`
}

func NewDashboardResponse(items []DashboardDomainResponse, alertCount int, generatedAt time.Time) DashboardResponse {
	totalScore := 0
	unknownSenderCount := 0
	for _, item := range items {
		totalScore += item.Score
		unknownSenderCount += item.UnknownSenderCount
	}
	averageScore := 0
	if len(items) > 0 {
		averageScore = totalScore / len(items)
	}
	return DashboardResponse{
		GeneratedAt:          generatedAt.UTC(),
		ProtectedDomainCount: len(items),
		AverageScore:         averageScore,
		AlertCount:           alertCount,
		UnknownSenderCount:   unknownSenderCount,
		Domains:              items,
	}
}

func NewDashboardDomainResponse(domain domains.Domain, senders []reporter.InventoryEntry) DashboardDomainResponse {
	response := DashboardDomainResponse{
		ID:           domain.ID,
		Domain:       domain.Domain,
		CreatedAt:    domain.CreatedAt.UTC(),
		Status:       "monitoring",
		DKIMCoverage: "unknown",
		NextAction:   "Run a record check to establish the protected-domain baseline.",
	}
	if domain.LatestCheck == nil {
		response.Status = "needs-work"
		return response
	}
	check := *domain.LatestCheck
	response.LastCheckedAt = check.CheckedAt.UTC()
	result := scanner.Result{
		Domain: domain.Domain,
		SPF:    check.SPF,
		DKIM:   check.DKIM,
		DMARC:  check.DMARC,
		BIMI:   check.BIMI,
	}
	summary := scanner.Summarize(result)
	response.Score = summary.Score
	response.NextAction = summary.Remediation
	response.Status = dashboardStatus(summary, check.DMARC)
	response.DMARCPolicy, _ = dmarcPolicy(check.DMARC.Record)
	if response.DMARCPolicy == "" {
		response.DMARCPolicy = "none"
	}
	response.SPFLookupCount = recommendations.SPFLookupCount(check.SPF.Record)
	response.DKIMCoverage = "missing"
	if check.DKIM.Status == scanner.StatusPass {
		response.DKIMCoverage = "covered"
	}
	response.AlignmentRate, response.UnknownSenderCount = reporterMetrics(senders)
	if response.UnknownSenderCount > 0 && response.Status == "protected" {
		response.Status = "monitoring"
		response.NextAction = "Review unknown Reporter senders before advancing enforcement."
	}
	return response
}

type DashboardRecheckResponse struct {
	JobID       string    `json:"job_id"`
	CheckID     string    `json:"check_id,omitempty"`
	DomainID    string    `json:"domain_id"`
	Status      string    `json:"status"`
	QueuedAt    time.Time `json:"queued_at"`
	NextAllowed time.Time `json:"next_allowed_at,omitempty"`
}

type DashboardSPFRouteResponse struct {
	ID                  string                      `json:"id"`
	DomainID            string                      `json:"domain_id"`
	Domain              string                      `json:"domain"`
	Status              string                      `json:"status"`
	RouteFQDN           string                      `json:"route_fqdn"`
	CustomerCNAMEName   string                      `json:"customer_cname_name"`
	CustomerCNAMETarget string                      `json:"customer_cname_target"`
	GeneratedSPFPreview string                      `json:"generated_spf_preview"`
	LastPublishedSPF    string                      `json:"last_published_spf"`
	LastSyncedAt        *time.Time                  `json:"last_synced_at,omitempty"`
	LastSyncError       string                      `json:"last_sync_error,omitempty"`
	EnabledSources      []DashboardSPFRouteSource   `json:"enabled_sources"`
	EvidenceSummary     []DashboardSPFRouteEvidence `json:"evidence_summary"`
}

type DashboardSPFRouteSource struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

type DashboardSPFRouteEvidence struct {
	SourceValue    string `json:"source_value,omitempty"`
	ResolvedDomain string `json:"resolved_domain,omitempty"`
	Mechanism      string `json:"mechanism,omitempty"`
	Kind           string `json:"kind"`
	Message        string `json:"message"`
}

type DashboardSPFRouteSyncResponse struct {
	JobID    string    `json:"job_id"`
	RouteID  string    `json:"route_id"`
	DomainID string    `json:"domain_id"`
	Status   string    `json:"status"`
	QueuedAt time.Time `json:"queued_at"`
}

func NewDashboardSPFRouteResponse(route spfroutes.Route) DashboardSPFRouteResponse {
	status := string(route.LastSyncStatus)
	if status == "" {
		status = "pending"
	}
	preview := strings.TrimSpace(route.LastGeneratedSPF)
	if preview == "" {
		preview = dashboardSPFPreview(route.Sources, route.MechanismPolicy)
	}
	response := DashboardSPFRouteResponse{
		ID:                  route.ID,
		DomainID:            route.DomainID,
		Domain:              route.Domain,
		Status:              status,
		RouteFQDN:           route.RouteFQDN,
		CustomerCNAMEName:   route.CustomerCNAMEName,
		CustomerCNAMETarget: route.CustomerCNAMETarget,
		GeneratedSPFPreview: preview,
		LastPublishedSPF:    route.LastPublishedSPF,
		LastSyncError:       dashboardSafeSPFRouteSyncError(route.LastSyncError),
		EnabledSources:      []DashboardSPFRouteSource{},
		EvidenceSummary:     []DashboardSPFRouteEvidence{},
	}
	if !route.LastSyncedAt.IsZero() {
		lastSyncedAt := route.LastSyncedAt.UTC()
		response.LastSyncedAt = &lastSyncedAt
	}
	for _, source := range route.Sources {
		if !source.Enabled {
			continue
		}
		response.EnabledSources = append(response.EnabledSources, DashboardSPFRouteSource{Kind: string(source.Kind), Value: source.Value, Label: source.Label})
	}
	for _, evidence := range route.Evidence {
		response.EvidenceSummary = append(response.EvidenceSummary, DashboardSPFRouteEvidence{SourceValue: evidence.SourceValue, ResolvedDomain: evidence.ResolvedDomain, Mechanism: evidence.Mechanism, Kind: evidence.Kind, Message: evidence.Message})
	}
	return response
}

func dashboardSafeSPFRouteSyncError(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "SPF route sync failed; see operational events for operator details"
}

func dashboardSPFPreview(sources []spfroutes.Source, policy spfroutes.Policy) string {
	parts := []string{"v=spf1"}
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		switch source.Kind {
		case spfroutes.SourceKindInclude:
			parts = append(parts, "include:"+source.Value)
		case spfroutes.SourceKindIP4:
			parts = append(parts, "ip4:"+source.Value)
		case spfroutes.SourceKindIP6:
			parts = append(parts, "ip6:"+source.Value)
		}
	}
	switch policy {
	case spfroutes.PolicyFail:
		parts = append(parts, "-all")
	case spfroutes.PolicyNeutral:
		parts = append(parts, "?all")
	default:
		parts = append(parts, "~all")
	}
	return strings.Join(parts, " ")
}

type DashboardChangePlanResponse struct {
	DomainID     string               `json:"domain_id"`
	Domain       string               `json:"domain"`
	GeneratedAt  time.Time            `json:"generated_at"`
	JourneyState dmarcjourney.State   `json:"journey_state"`
	NextAction   string               `json:"next_action"`
	Changes      []DashboardDNSChange `json:"changes"`
}

type DashboardDNSChange struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	RecordName   string `json:"record_name"`
	RecordType   string `json:"record_type"`
	RecordText   string `json:"record_text"`
	Evidence     string `json:"evidence"`
	Risk         string `json:"risk"`
	RollbackNote string `json:"rollback_note"`
}

func NewDashboardChangePlanResponse(domain domains.Domain, evidence dmarcjourney.Evidence, generatedAt time.Time) DashboardChangePlanResponse {
	result := scanner.Result{Domain: domain.Domain}
	if domain.LatestCheck != nil {
		result.SPF = domain.LatestCheck.SPF
		result.DKIM = domain.LatestCheck.DKIM
		result.DMARC = domain.LatestCheck.DMARC
		result.BIMI = domain.LatestCheck.BIMI
	}
	journey := dmarcjourney.Evaluate(result.DMARC, evidence)
	changes := recommendations.BuildChangePlan(result, evidence)
	response := DashboardChangePlanResponse{
		DomainID:     domain.ID,
		Domain:       domain.Domain,
		GeneratedAt:  generatedAt.UTC(),
		JourneyState: journey.State,
		NextAction:   journey.NextAction,
		Changes:      make([]DashboardDNSChange, 0, len(changes)),
	}
	for _, change := range changes {
		response.Changes = append(response.Changes, DashboardDNSChange{
			ID:           change.ID,
			Title:        change.Title,
			Status:       string(change.Status),
			RecordName:   change.RecordName,
			RecordType:   change.RecordType,
			RecordText:   change.RecordText,
			Evidence:     change.Evidence,
			Risk:         change.Risk,
			RollbackNote: change.RollbackNote,
		})
	}
	return response
}

type DashboardReporterResponse struct {
	DomainID          string                    `json:"domain_id"`
	Domain            string                    `json:"domain"`
	ReportVolume      int                       `json:"report_volume"`
	AlignedPercentage float64                   `json:"aligned_percentage"`
	ReporterOrgCount  int                       `json:"reporter_org_count"`
	Failures          int                       `json:"failures"`
	RejectCandidates  int                       `json:"reject_candidates"`
	SetupGuidance     string                    `json:"setup_guidance,omitempty"`
	SenderInventory   []DashboardSenderResponse `json:"sender_inventory"`
}

type DashboardSenderResponse struct {
	SourceIP          string    `json:"source_ip"`
	Provider          string    `json:"provider"`
	Volume            int       `json:"volume"`
	AlignedPercentage float64   `json:"aligned_percentage"`
	Classification    string    `json:"classification"`
	Trend             string    `json:"trend"`
	Risk              string    `json:"risk"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
}

type DashboardBrandRiskResponse struct {
	DomainID    string                         `json:"domain_id"`
	Domain      string                         `json:"domain"`
	GeneratedAt time.Time                      `json:"generated_at"`
	Findings    []DashboardBrandRiskFinding    `json:"findings"`
	Lookalikes  []DashboardLookalikeRiskSignal `json:"lookalikes"`
}

type DashboardBrandRiskFinding struct {
	Category          string `json:"category"`
	Type              string `json:"type"`
	Title             string `json:"title"`
	Severity          string `json:"severity"`
	Evidence          string `json:"evidence"`
	RecommendedAction string `json:"recommended_action"`
}

type DashboardLookalikeRiskSignal struct {
	Domain            string `json:"domain"`
	Kind              string `json:"kind"`
	Reason            string `json:"reason"`
	HasAddressRecords bool   `json:"has_address_records"`
	HasMXRecords      bool   `json:"has_mx_records"`
	Severity          string `json:"severity"`
	Evidence          string `json:"evidence"`
	RecommendedAction string `json:"recommended_action"`
}

func NewDashboardBrandRiskResponse(domain domains.Domain, lookalikes []brandrisk.LookalikeFinding, generatedAt time.Time) DashboardBrandRiskResponse {
	response := DashboardBrandRiskResponse{
		DomainID:    domain.ID,
		Domain:      domain.Domain,
		GeneratedAt: generatedAt.UTC(),
		Findings:    []DashboardBrandRiskFinding{},
		Lookalikes:  make([]DashboardLookalikeRiskSignal, 0, len(lookalikes)),
	}
	if domain.LatestCheck != nil {
		scan := scanner.Result{
			Domain:    domain.Domain,
			CheckedAt: domain.LatestCheck.CheckedAt,
			SPF:       domain.LatestCheck.SPF,
			DKIM:      domain.LatestCheck.DKIM,
			DMARC:     domain.LatestCheck.DMARC,
			BIMI:      domain.LatestCheck.BIMI,
		}
		for _, finding := range brandrisk.FromScan(scan) {
			response.Findings = append(response.Findings, DashboardBrandRiskFinding{
				Category:          finding.Category,
				Type:              finding.Type,
				Title:             finding.Title,
				Severity:          string(finding.Severity),
				Evidence:          finding.Evidence,
				RecommendedAction: finding.RecommendedAction,
			})
		}
	}
	for _, finding := range lookalikes {
		response.Lookalikes = append(response.Lookalikes, DashboardLookalikeRiskSignal{
			Domain:            finding.Domain,
			Kind:              string(finding.Kind),
			Reason:            finding.Reason,
			HasAddressRecords: finding.HasAddressRecords,
			HasMXRecords:      finding.HasMXRecords,
			Severity:          string(finding.Severity),
			Evidence:          finding.Evidence,
			RecommendedAction: finding.RecommendedAction,
		})
	}
	return response
}

func NewDashboardReporterResponse(domain domains.Domain, senders []reporter.InventoryEntry, health reporter.HealthResult, unknownOnly bool) DashboardReporterResponse {
	response := DashboardReporterResponse{
		DomainID:         domain.ID,
		Domain:           domain.Domain,
		ReporterOrgCount: len(health.ReporterOrgs),
		Failures:         health.FailureCount,
		SenderInventory:  []DashboardSenderResponse{},
	}
	aligned := 0
	for _, entry := range senders {
		risk := reporter.InventoryRisk(reporter.ToSenderInventoryItem(entry))
		classification := senderClassification(entry, risk)
		if unknownOnly && classification != "unknown" {
			continue
		}
		failed := int(float64(entry.Volume) * entry.FailureRate)
		if failed < 0 {
			failed = 0
		}
		alignedVolume := entry.Volume - failed
		if alignedVolume < 0 {
			alignedVolume = 0
		}
		response.ReportVolume += entry.Volume
		aligned += alignedVolume
		if risk == "expected" && entry.FailureRate <= 0.02 {
			response.RejectCandidates++
		}
		response.SenderInventory = append(response.SenderInventory, DashboardSenderResponse{
			SourceIP:          entry.SourceIP,
			Provider:          entry.Provider,
			Volume:            entry.Volume,
			AlignedPercentage: percent(alignedVolume, entry.Volume),
			Classification:    classification,
			Trend:             senderTrend(entry),
			Risk:              risk,
			FirstSeen:         entry.FirstSeen.UTC(),
			LastSeen:          entry.LastSeen.UTC(),
		})
	}
	response.AlignedPercentage = percent(aligned, response.ReportVolume)
	if response.ReportVolume == 0 {
		response.SetupGuidance = "Enable DMARC aggregate reporting with a rua mailbox to populate Reporter sender inventory."
	}
	return response
}

type DashboardAlertResponse struct {
	ID           string    `json:"id"`
	DomainID     string    `json:"domain_id"`
	Domain       string    `json:"domain"`
	CheckName    string    `json:"check_name"`
	Kind         string    `json:"kind"`
	Severity     string    `json:"severity"`
	Message      string    `json:"message"`
	Evidence     string    `json:"evidence"`
	CreatedAt    time.Time `json:"created_at"`
	MutedUntil   time.Time `json:"muted_until,omitempty"`
	Acknowledged bool      `json:"acknowledged"`
}

func NewDashboardAlertResponse(state alerts.DashboardAlert) DashboardAlertResponse {
	return DashboardAlertResponse{
		ID:           state.ID,
		DomainID:     state.DomainID,
		Domain:       state.Domain,
		CheckName:    state.Event.CheckName,
		Kind:         string(state.Event.Kind),
		Severity:     string(state.Event.Severity),
		Message:      state.Event.Message,
		Evidence:     state.Event.AfterRecord,
		CreatedAt:    state.CreatedAt.UTC(),
		MutedUntil:   state.MutedUntil.UTC(),
		Acknowledged: !state.AcknowledgedAt.IsZero(),
	}
}

type DashboardAlertActionResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	AppliedAt time.Time `json:"applied_at"`
}

type DashboardSettingsResponse struct {
	RequireAlignmentBeforeReject        int       `json:"require_alignment_before_reject"`
	PauseAutomationOnUnknownSenderSpike bool      `json:"pause_automation_on_unknown_senders_spike"`
	AutoGenerateMonthlyExecutiveReports bool      `json:"auto_generate_monthly_executive_reports"`
	NotificationChannels                []string  `json:"notification_channels"`
	UpdatedAt                           time.Time `json:"updated_at,omitempty"`
}

func NewDashboardSettingsResponse(settings domains.DashboardSettings) DashboardSettingsResponse {
	return DashboardSettingsResponse{
		RequireAlignmentBeforeReject:        settings.RequireAlignmentBeforeReject,
		PauseAutomationOnUnknownSenderSpike: settings.PauseAutomationOnUnknownSenderSpike,
		AutoGenerateMonthlyExecutiveReports: settings.AutoGenerateMonthlyExecutiveReports,
		NotificationChannels:                settings.NotificationChannels,
		UpdatedAt:                           settings.UpdatedAt.UTC(),
	}
}

func dashboardStatus(summary scanner.PostureSummary, dmarc scanner.CheckResult) string {
	policy, _ := dmarcPolicy(dmarc.Record)
	if summary.Score >= 90 && policy == "reject" {
		return "protected"
	}
	if summary.Score < 70 || dmarc.Status == scanner.StatusMissing || policy == "none" || policy == "" {
		return "needs-work"
	}
	return "monitoring"
}

func dmarcPolicy(record string) (string, int) {
	policy := ""
	pct := 100
	for _, part := range strings.Split(record, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "p":
			policy = strings.ToLower(strings.TrimSpace(value))
		case "pct":
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed >= 0 && parsed <= 100 {
				pct = parsed
			}
		}
	}
	return policy, pct
}

func reporterMetrics(senders []reporter.InventoryEntry) (float64, int) {
	total := 0
	aligned := 0
	unknown := 0
	for _, entry := range senders {
		total += entry.Volume
		failed := int(float64(entry.Volume) * entry.FailureRate)
		aligned += entry.Volume - failed
		risk := reporter.InventoryRisk(reporter.ToSenderInventoryItem(entry))
		if senderClassification(entry, risk) == "unknown" {
			unknown++
		}
	}
	return percent(aligned, total), unknown
}

func senderClassification(entry reporter.InventoryEntry, risk string) string {
	if entry.Expected {
		return "expected"
	}
	if entry.Ignored {
		return "ignored"
	}
	if entry.Suspicious || risk == "high" || entry.Provider == "" {
		return "unknown"
	}
	return "review"
}

func senderTrend(entry reporter.InventoryEntry) string {
	if entry.FirstSeen.IsZero() || entry.LastSeen.IsZero() || entry.FirstSeen.Equal(entry.LastSeen) {
		return "new"
	}
	if entry.LastSeen.Sub(entry.FirstSeen) > 14*24*time.Hour {
		return "stable"
	}
	return "watch"
}

func percent(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
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

type OperationalEventResponse struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity"`
	Message    string         `json:"message"`
	Actor      string         `json:"actor,omitempty"`
	Resource   string         `json:"resource,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
}

func NewOperationalEventResponse(event ops.Event) OperationalEventResponse {
	response := OperationalEventResponse{
		ID:         event.ID,
		Kind:       event.Kind,
		Severity:   string(event.Severity),
		Message:    event.Message,
		Actor:      event.Actor,
		Resource:   event.Resource,
		OccurredAt: event.OccurredAt.UTC(),
	}
	if metadata := ops.SanitizeMetadata(event.Metadata); len(metadata) > 0 {
		response.Metadata = metadata
	}
	return response
}

type ProviderHealthResponse struct {
	GeneratedAt                  time.Time `json:"generated_at"`
	PublicBaseURLConfigured      bool      `json:"public_base_url_configured"`
	BunnyDNSConfigured           bool      `json:"bunny_dns_configured"`
	BunnyDNSZoneIDConfigured     bool      `json:"bunny_dns_zone_id_configured"`
	BootstrapConfigured          bool      `json:"bootstrap_configured"`
	BootstrapPartiallyConfigured bool      `json:"bootstrap_partially_configured"`
	BootstrapSkipped             bool      `json:"bootstrap_skipped"`
	SPFRouteSchedulerEnabled     bool      `json:"spf_route_scheduler_enabled"`
	SPFRouteSchedulerInterval    string    `json:"spf_route_scheduler_interval,omitempty"`
	SPFRouteScheduledSyncCadence string    `json:"spf_route_scheduled_sync_cadence,omitempty"`
	Version                      string    `json:"version"`
	Commit                       string    `json:"commit"`
	BuildTime                    string    `json:"build_time"`
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
