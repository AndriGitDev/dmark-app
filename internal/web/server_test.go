package web

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/admin"
	"github.com/AndriGitDev/meil/internal/alerts"
	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/billing"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/entitlements"
	"github.com/AndriGitDev/meil/internal/privacy"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/statuspage"
	"github.com/AndriGitDev/meil/internal/teams"
	"github.com/AndriGitDev/meil/internal/webhooks"
)

type fakeScanner struct {
	result scanner.Result
	err    error
}

func (s fakeScanner) Scan(ctx context.Context, domain string) (scanner.Result, error) {
	return s.result, s.err
}

type webDomainStore struct {
	domains           []domains.Domain
	history           []domains.HistoryEntry
	driftEvents       []drift.Event
	reportSources     []reporter.DashboardRow
	reportSenders     []reporter.InventoryEntry
	reportHealth      reporter.HealthResult
	dashboardAlerts   []alerts.DashboardAlert
	dashboardSettings domains.DashboardSettings
	queuedRecheck     domains.RecheckJob
	webhookEndpoints  []webhooks.Endpoint
	apiKeys           []apikeys.Key
	invitations       []teams.Invitation
	retention         privacy.RetentionRequest
	auditClaims       []salesaudit.Claim
	adminCases        []admin.Case
	abuseDecisions    []admin.AbuseDecision
	statusComponents  []statuspage.Component
	statusIncidents   []statuspage.Incident
	deletedUserID     string
	checkedDomain     string
}

func (s *webDomainStore) CreateDomain(ctx context.Context, userID string, domain string) (domains.Domain, error) {
	saved := domains.Domain{ID: "domain-" + domain, Domain: domain}
	s.domains = append(s.domains, saved)
	return saved, nil
}

func (s *webDomainStore) ListDomains(ctx context.Context, userID string) ([]domains.Domain, error) {
	return append([]domains.Domain(nil), s.domains...), nil
}

func (s *webDomainStore) ListOrganizationDomains(ctx context.Context, organizationID string) ([]domains.Domain, error) {
	return append([]domains.Domain(nil), s.domains...), nil
}

func (s *webDomainStore) CreateOrganizationDomain(ctx context.Context, organizationID string, domain string) (domains.Domain, error) {
	saved := domains.Domain{ID: "org-domain-" + domain, Domain: domain}
	s.domains = append(s.domains, saved)
	return saved, nil
}

func (s *webDomainStore) GetOrganizationDomain(ctx context.Context, organizationID string, domainID string) (domains.Domain, error) {
	for _, domain := range s.domains {
		if domain.ID == domainID {
			return domain, nil
		}
	}
	return domains.Domain{}, errors.New("domain not found")
}

func (s *webDomainStore) ListOrganizationDomainHistory(ctx context.Context, organizationID string, domainID string, limit int) ([]domains.HistoryEntry, error) {
	filtered := []domains.HistoryEntry{}
	for _, entry := range s.history {
		if entry.DomainID == domainID {
			filtered = append(filtered, entry)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (s *webDomainStore) ListOrganizationDomainDrift(ctx context.Context, organizationID string, domainID string, limit int) ([]drift.Event, error) {
	events := append([]drift.Event(nil), s.driftEvents...)
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func (s *webDomainStore) ListOrganizationReporterSources(ctx context.Context, organizationID string, domainID string, limit int) ([]reporter.DashboardRow, error) {
	rows := append([]reporter.DashboardRow(nil), s.reportSources...)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (s *webDomainStore) ListOrganizationReporterSenders(ctx context.Context, organizationID string, domainID string, limit int) ([]reporter.InventoryEntry, error) {
	entries := append([]reporter.InventoryEntry(nil), s.reportSenders...)
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (s *webDomainStore) GetOrganizationReporterHealth(ctx context.Context, organizationID string, domainID string) (reporter.HealthResult, error) {
	return s.reportHealth, nil
}

func (s *webDomainStore) QueueOrganizationDomainRecheck(ctx context.Context, organizationID string, domainID string, at time.Time) (domains.RecheckJob, error) {
	if s.queuedRecheck.ID == "" {
		s.queuedRecheck = domains.RecheckJob{
			ID:          "job-1",
			DomainID:    domainID,
			Status:      "queued",
			QueuedAt:    at.UTC(),
			NextAllowed: at.UTC().Add(5 * time.Minute),
		}
	}
	return s.queuedRecheck, nil
}

func (s *webDomainStore) ListOrganizationDashboardAlerts(ctx context.Context, organizationID string, activeOnly bool, limit int) ([]alerts.DashboardAlert, error) {
	result := []alerts.DashboardAlert{}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	for _, alert := range s.dashboardAlerts {
		if activeOnly && (!alert.AcknowledgedAt.IsZero() || (!alert.MutedUntil.IsZero() && alert.MutedUntil.After(now))) {
			continue
		}
		result = append(result, alert)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *webDomainStore) AcknowledgeOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, reason string) error {
	for i := range s.dashboardAlerts {
		if s.dashboardAlerts[i].ID == alertID {
			s.dashboardAlerts[i].AcknowledgedAt = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
			return nil
		}
	}
	return errors.New("alert not found")
}

func (s *webDomainStore) MuteOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, until time.Time, reason string) error {
	for i := range s.dashboardAlerts {
		if s.dashboardAlerts[i].ID == alertID {
			s.dashboardAlerts[i].MutedUntil = until
			return nil
		}
	}
	return errors.New("alert not found")
}

func (s *webDomainStore) GetOrganizationDashboardSettings(ctx context.Context, organizationID string) (domains.DashboardSettings, error) {
	if s.dashboardSettings.RequireAlignmentBeforeReject == 0 {
		return domains.DefaultDashboardSettings(), nil
	}
	return s.dashboardSettings, nil
}

func (s *webDomainStore) SaveOrganizationDashboardSettings(ctx context.Context, organizationID string, settings domains.DashboardSettings) (domains.DashboardSettings, error) {
	settings.UpdatedAt = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	s.dashboardSettings = settings
	return settings, nil
}

func (s *webDomainStore) ListUserReporterSenders(ctx context.Context, userID string, domainID string, limit int) ([]reporter.InventoryEntry, error) {
	entries := append([]reporter.InventoryEntry(nil), s.reportSenders...)
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (s *webDomainStore) GetUserReporterHealth(ctx context.Context, userID string, domainID string) (reporter.HealthResult, error) {
	return s.reportHealth, nil
}

func (s *webDomainStore) CreateAuditClaim(ctx context.Context, claim salesaudit.Claim) (salesaudit.Claim, error) {
	claim.ID = "claim-1"
	s.auditClaims = append(s.auditClaims, claim)
	return claim, nil
}

func (s *webDomainStore) ListOrganizationWebhookEndpoints(ctx context.Context, organizationID string) ([]webhooks.Endpoint, error) {
	return append([]webhooks.Endpoint(nil), s.webhookEndpoints...), nil
}

func (s *webDomainStore) CreateOrganizationWebhookEndpoint(ctx context.Context, organizationID string, rawURL string, eventTypes []string) (webhooks.Endpoint, string, error) {
	endpoint, secret, err := webhooks.NewEndpoint(organizationID, rawURL, eventTypes, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	endpoint.ID = "webhook-1"
	s.webhookEndpoints = append(s.webhookEndpoints, endpoint)
	return endpoint, secret, nil
}

func (s *webDomainStore) RotateOrganizationWebhookSecret(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, string, error) {
	for _, endpoint := range s.webhookEndpoints {
		if endpoint.ID == endpointID {
			secret, err := webhooks.RotateSecret()
			return endpoint, secret, err
		}
	}
	return webhooks.Endpoint{}, "", errors.New("webhook not found")
}

func (s *webDomainStore) DisableOrganizationWebhookEndpoint(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, error) {
	for i, endpoint := range s.webhookEndpoints {
		if endpoint.ID == endpointID {
			s.webhookEndpoints[i].Enabled = false
			return s.webhookEndpoints[i], nil
		}
	}
	return webhooks.Endpoint{}, errors.New("webhook not found")
}

func (s *webDomainStore) ListOrganizationAPIKeys(ctx context.Context, organizationID string) ([]apikeys.Key, error) {
	return append([]apikeys.Key(nil), s.apiKeys...), nil
}

func (s *webDomainStore) CreateOrganizationAPIKey(ctx context.Context, organizationID string, name string, scopes []apikeys.Scope) (apikeys.Key, error) {
	key := apikeys.Key{
		ID:             "api-key-1",
		OrganizationID: organizationID,
		Name:           name,
		Scopes:         scopes,
		Token:          "meil_created",
		CreatedAt:      time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}
	s.apiKeys = append(s.apiKeys, key)
	return key, nil
}

func (s *webDomainStore) RevokeOrganizationAPIKey(ctx context.Context, organizationID string, keyID string) (apikeys.Key, error) {
	for i, key := range s.apiKeys {
		if key.ID == keyID {
			s.apiKeys[i].RevokedAt = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
			return s.apiKeys[i], nil
		}
	}
	return apikeys.Key{}, errors.New("API key not found")
}

func (s *webDomainStore) ListOrganizationInvitations(ctx context.Context, organizationID string) ([]teams.Invitation, error) {
	return append([]teams.Invitation(nil), s.invitations...), nil
}

func (s *webDomainStore) CreateOrganizationInvitation(ctx context.Context, organizationID string, email string, role entitlements.Role) (teams.Invitation, error) {
	invitation, err := teams.NewInvitation(email, role)
	if err != nil {
		return teams.Invitation{}, err
	}
	invitation.ID = "invite-1"
	invitation.OrganizationID = organizationID
	invitation.ExpiresAt = time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	invitation.CreatedAt = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	s.invitations = append(s.invitations, invitation)
	return invitation, nil
}

func (s *webDomainStore) RequestOrganizationDomainTransfer(ctx context.Context, organizationID string, domainID string, toOrganizationID string, requestedByUserID string) (teams.TransferRequest, error) {
	request, err := teams.NewTransferRequest(teams.DomainTransfer{
		DomainID:          domainID,
		FromOrganization:  organizationID,
		ToOrganization:    toOrganizationID,
		RequestedByUserID: requestedByUserID,
	}, entitlements.RoleAdmin, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		return teams.TransferRequest{}, err
	}
	request.ID = "transfer-1"
	return request, nil
}

func (s *webDomainStore) GetOrganizationRetention(ctx context.Context, organizationID string) (privacy.RetentionRequest, error) {
	if s.retention == (privacy.RetentionRequest{}) {
		return privacy.RetentionRequest{RawReportDays: 90, AggregateRowDays: 365, NotificationLogDays: 180}, nil
	}
	return s.retention, nil
}

func (s *webDomainStore) SaveOrganizationRetention(ctx context.Context, organizationID string, request privacy.RetentionRequest) (privacy.RetentionRequest, error) {
	s.retention = request
	return request, nil
}

func (s *webDomainStore) ListAdminCases(ctx context.Context, status string, limit int) ([]admin.Case, error) {
	cases := []admin.Case{}
	for _, c := range s.adminCases {
		if status == "" || string(c.Status) == status {
			cases = append(cases, c)
		}
	}
	if limit > 0 && len(cases) > limit {
		cases = cases[:limit]
	}
	return cases, nil
}

func (s *webDomainStore) CreateAdminCase(ctx context.Context, kind admin.CaseKind, subject string, userID string, domainID string) (admin.Case, error) {
	c, err := admin.NewCase(kind, subject, userID, domainID, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		return admin.Case{}, err
	}
	c.ID = "case-1"
	s.adminCases = append(s.adminCases, c)
	return c, nil
}

func (s *webDomainStore) ResolveAdminCase(ctx context.Context, caseID string, actorUserID string) (admin.Case, error) {
	for i, c := range s.adminCases {
		if c.ID == caseID {
			s.adminCases[i] = c.Resolve(time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC))
			s.adminCases[i].AssignedTo = actorUserID
			return s.adminCases[i], nil
		}
	}
	return admin.Case{}, errors.New("admin case not found")
}

func (s *webDomainStore) ListAbuseDecisions(ctx context.Context, identifier string, limit int) ([]admin.AbuseDecision, error) {
	decisions := []admin.AbuseDecision{}
	for _, decision := range s.abuseDecisions {
		if identifier == "" || decision.Identifier == identifier {
			decisions = append(decisions, decision)
		}
	}
	if limit > 0 && len(decisions) > limit {
		decisions = decisions[:limit]
	}
	return decisions, nil
}

func (s *webDomainStore) CreateAbuseDecision(ctx context.Context, identifier string, action admin.AbuseAction, reason string, duration time.Duration) (admin.AbuseDecision, error) {
	decision, err := admin.NewAbuseDecision(identifier, action, reason, duration, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		return admin.AbuseDecision{}, err
	}
	decision.ID = "abuse-1"
	decision.CreatedAt = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	s.abuseDecisions = append(s.abuseDecisions, decision)
	return decision, nil
}

func (s *webDomainStore) ListStatusComponents(ctx context.Context) ([]statuspage.Component, error) {
	return append([]statuspage.Component(nil), s.statusComponents...), nil
}

func (s *webDomainStore) UpsertStatusComponent(ctx context.Context, name string, status statuspage.ComponentStatus) (statuspage.Component, error) {
	component := statuspage.Component{
		ID:        "component-1",
		Name:      name,
		Status:    status,
		UpdatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}
	for i, existing := range s.statusComponents {
		if existing.Name == name {
			component.ID = existing.ID
			s.statusComponents[i] = component
			return component, nil
		}
	}
	s.statusComponents = append(s.statusComponents, component)
	return component, nil
}

func (s *webDomainStore) ListStatusIncidents(ctx context.Context, limit int) ([]statuspage.Incident, error) {
	incidents := append([]statuspage.Incident(nil), s.statusIncidents...)
	if limit > 0 && len(incidents) > limit {
		incidents = incidents[:limit]
	}
	return incidents, nil
}

func (s *webDomainStore) CreateStatusIncident(ctx context.Context, title string, status string, message string) (statuspage.Incident, error) {
	incident := statuspage.Incident{
		ID:        "incident-1",
		Title:     title,
		Status:    status,
		StartedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}
	if strings.TrimSpace(message) != "" {
		incident.Updates = append(incident.Updates, statuspage.IncidentUpdate{At: incident.CreatedAt, Message: strings.TrimSpace(message)})
	}
	s.statusIncidents = append(s.statusIncidents, incident)
	return incident, nil
}

func (s *webDomainStore) ResolveStatusIncident(ctx context.Context, incidentID string, message string) (statuspage.Incident, error) {
	for i, incident := range s.statusIncidents {
		if incident.ID == incidentID {
			s.statusIncidents[i].Status = "resolved"
			s.statusIncidents[i].ResolvedAt = time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC)
			if strings.TrimSpace(message) != "" {
				s.statusIncidents[i].Updates = append(s.statusIncidents[i].Updates, statuspage.IncidentUpdate{At: s.statusIncidents[i].ResolvedAt, Message: strings.TrimSpace(message)})
			}
			return s.statusIncidents[i], nil
		}
	}
	return statuspage.Incident{}, errors.New("status incident not found")
}

func (s *webDomainStore) DeleteAccountData(ctx context.Context, userID string, email string) (privacy.DeletionPlan, error) {
	s.deletedUserID = userID
	return privacy.NewDeletionPlan(userID), nil
}

func (s *webDomainStore) GetDomain(ctx context.Context, userID string, domainID string) (domains.Domain, error) {
	for _, domain := range s.domains {
		if domain.ID == domainID {
			return domain, nil
		}
	}
	return domains.Domain{}, errors.New("domain not found")
}

func (s *webDomainStore) DeleteDomain(ctx context.Context, userID string, domainID string) error {
	return nil
}

func (s *webDomainStore) CreateCheck(ctx context.Context, domainID string, result scanner.Result) error {
	s.checkedDomain = domainID
	return nil
}

type webBillingStore struct {
	event        billing.WebhookEvent
	applied      bool
	cancellation billing.Cancellation
}

func (s *webBillingStore) ApplyBillingWebhook(ctx context.Context, event billing.WebhookEvent) (bool, error) {
	s.event = event
	if !s.applied {
		s.applied = true
		return true, nil
	}
	return false, nil
}

func (s *webBillingStore) RequestBillingCancellation(ctx context.Context, organizationID string, reason string, periodEndsAt time.Time) (billing.Cancellation, error) {
	cancellation, err := billing.NewCancellation(organizationID, reason, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC), periodEndsAt)
	if err != nil {
		return billing.Cancellation{}, err
	}
	s.cancellation = cancellation
	return cancellation, nil
}

type webAPIKeyAuth struct {
	principal apikeys.Principal
	err       error
	token     string
}

func (a *webAPIKeyAuth) AuthenticateAPIKey(ctx context.Context, key string) (apikeys.Principal, error) {
	a.token = key
	if a.err != nil {
		return apikeys.Principal{}, a.err
	}
	return a.principal, nil
}

type webLookalikeResolver struct {
	hosts map[string][]string
	mx    map[string][]*net.MX
}

func (r webLookalikeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if values := r.hosts[host]; len(values) > 0 {
		return values, nil
	}
	return nil, errors.New("not found")
}

func (r webLookalikeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	if values := r.mx[name]; len(values) > 0 {
		return values, nil
	}
	return nil, errors.New("not found")
}

func TestBrowserRoutesAreHandedOff(t *testing.T) {
	t.Parallel()

	router := NewRouter()
	for _, tt := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/login"},
		{http.MethodPost, "/login"},
		{http.MethodGet, "/logout"},
		{http.MethodPost, "/logout"},
		{http.MethodGet, "/auth/callback"},
		{http.MethodPost, "/check"},
		{http.MethodPost, "/claim-audit"},
		{http.MethodGet, "/app/domains"},
		{http.MethodGet, "/assets/logo.svg"},
	} {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
			if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", contentType)
			}
			if !strings.Contains(rec.Body.String(), "dmark-web") {
				t.Fatalf("body missing dmark-web handoff message: %s", rec.Body.String())
			}
		})
	}
}

func TestRequestIDHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req-test")
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "req-test" {
		t.Fatalf("X-Request-ID = %q, want req-test", got)
	}
}

func TestMuteUntil(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	if got := muteUntil("1h", now); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("muteUntil 1h = %s, want %s", got, now.Add(time.Hour))
	}
	if got := muteUntil("7d", now); !got.Equal(now.Add(7 * 24 * time.Hour)) {
		t.Fatalf("muteUntil 7d = %s, want one week", got)
	}
	if got := muteUntil("", now); !got.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("muteUntil default = %s, want one day", got)
	}
}

func TestBillingWebhookAppliesVerifiedEvent(t *testing.T) {
	t.Parallel()

	store := &webBillingStore{}
	body := `{"id":"evt_1","customer_id":"cus_1","subscription_id":"sub_1","plan":"reporter","status":"active","occurred_at":"2026-05-16T12:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader(body))
	req.Header.Set("X-Billing-Signature", billing.SignPayload("secret", []byte(body)))
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Billing:              store,
		BillingWebhookSecret: "secret",
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.event.ID != "evt_1" || store.event.Plan != "reporter" {
		t.Fatalf("stored billing event = %+v, want evt_1 reporter", store.event)
	}
	if !strings.Contains(rec.Body.String(), `"applied":true`) {
		t.Fatalf("body missing applied response: %s", rec.Body.String())
	}
}

func TestBillingWebhookRejectsInvalidSignature(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader(`{"id":"evt_1"}`))
	signature := "sha256=badfixturevalue"
	req.Header.Set("X-Billing-Signature", signature)
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Billing:              &webBillingStore{},
		BillingWebhookSecret: "billing-fixture-value",
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "invalid billing webhook signature") {
		t.Fatalf("body missing signature error: %s", body)
	}
	if strings.Contains(body, signature) || strings.Contains(body, "billing-fixture-value") {
		t.Fatalf("body leaked webhook secret material: %s", body)
	}
}

func TestAPIScanRequiresBearerToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scan?domain=example.com", nil)
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{Domain: "example.com"}},
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeScanRead}},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if !strings.Contains(rec.Body.String(), "valid bearer API key is required") {
		t.Fatalf("body missing API auth error: %s", rec.Body.String())
	}
}

func TestAPIScanRejectsInvalidKeyWithoutLeakingToken(t *testing.T) {
	t.Parallel()

	token := "meil_rejected_value"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scan?domain=example.com", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{Domain: "example.com"}},
		APIKeys: &webAPIKeyAuth{
			err: apikeys.ErrInvalidKey,
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "invalid API key") {
		t.Fatalf("body missing API key error: %s", body)
	}
	if strings.Contains(body, token) {
		t.Fatalf("body leaked API key: %s", body)
	}
}

func TestAPIScanReturnsJSON(t *testing.T) {
	t.Parallel()

	authenticator := &webAPIKeyAuth{principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeScanRead}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scan?domain=example.com", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{
			Domain: "example.com",
			SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
			BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
			DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		}},
		APIKeys: authenticator,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if authenticator.token != "meil_test" {
		t.Fatalf("authenticated token = %q, want meil_test", authenticator.token)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{`"domain":"example.com"`, `"summary"`, `"audit"`, `"grade"`, `"cta"`, `"checks"`, `"SPF"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestPublicScanReturnsJSONWithoutAPIKey(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/public/scan?domain=example.com", nil)
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{
			Domain: "example.com",
			SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
			BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
			DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		}},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"domain":"example.com"`) || !strings.Contains(body, `"audit"`) {
		t.Fatalf("body missing public scan response: %s", body)
	}
}

func TestPublicScanAcceptsPOSTBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/public/scan", strings.NewReader(`{"domain":"Example.COM"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{Domain: "example.com"}},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestPublicScanRateLimitsByClient(t *testing.T) {
	t.Parallel()

	router := NewRouterWithDeps(Deps{
		Scanner:               fakeScanner{result: scanner.Result{Domain: "example.com"}},
		PublicChecksPerMinute: 1,
	})
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodGet, "/api/public/scan?domain=example.com", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != want {
			t.Fatalf("request %d status = %d, want %d; body=%s", i+1, rec.Code, want, rec.Body.String())
		}
	}
}

func TestPublicAuditClaimPersistsVerifiedWorkEmail(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	req := httptest.NewRequest(http.MethodPost, "/api/public/audit-claims", strings.NewReader(`{"domain":"example.com","email":"security@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{
			Domain: "example.com",
			SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
			BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
			DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		}},
		AuditClaims: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(store.auditClaims) != 1 {
		t.Fatalf("audit claims stored = %d, want 1", len(store.auditClaims))
	}
	claim := store.auditClaims[0]
	if claim.Domain != "example.com" || claim.Email != "security@example.com" || claim.Status != salesaudit.ClaimMagicLinkSent {
		t.Fatalf("claim = %+v, want normalized persisted claim", claim)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"claim_id":"claim-1"`) || !strings.Contains(body, `"proof_eligible":true`) {
		t.Fatalf("body missing claim response: %s", body)
	}
}

func TestPublicAuditClaimRejectsMismatchedEmailDomain(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	req := httptest.NewRequest(http.MethodPost, "/api/public/audit-claims", strings.NewReader(`{"domain":"example.com","email":"security@other.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{
			Domain: "example.com",
			SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
		}},
		AuditClaims: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if len(store.auditClaims) != 0 {
		t.Fatalf("audit claims stored = %d, want 0", len(store.auditClaims))
	}
	if !strings.Contains(rec.Body.String(), "work email domain must match") {
		t.Fatalf("body missing email-domain error: %s", rec.Body.String())
	}
}

func TestCORSAllowsConfiguredFrontendOrigin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodOptions, "/api/public/scan", nil)
	req.Header.Set("Origin", "https://dmark.is")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		FrontendOrigins: []string{"https://dmark.is"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dmark.is" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
}

func TestAPIScanRejectsMissingScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scan?domain=example.com", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{Domain: "example.com"}},
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIDomainsReturnsOrganizationDomains(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomains: &webDomainStore{domains: []domains.Domain{{ID: "domain-1", Domain: "example.com"}}},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"domains"`) || !strings.Contains(body, `"example.com"`) {
		t.Fatalf("body missing domains response: %s", body)
	}
}

func TestDashboardDomainsListsFrontendContract(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	store := &webDomainStore{
		domains: []domains.Domain{{
			ID:        "domain-1",
			Domain:    "example.com",
			CreatedAt: checkedAt.Add(-24 * time.Hour),
			LatestCheck: &domains.Check{
				CheckedAt: checkedAt,
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Record: "v=spf1 include:_spf.example.net -all"},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=reject; rua=mailto:dmarc@example.com"},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing},
			},
		}},
		reportSenders: []reporter.InventoryEntry{{
			SourceIP:    "203.0.113.10",
			Volume:      20,
			FailureRate: 0.1,
		}},
	}
	router := NewRouterWithDeps(Deps{
		APIKeys:     &webAPIKeyAuth{principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead, apikeys.ScopeReporterRead}}},
		APIDomains:  store,
		APIReporter: store,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/domains", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Domains []struct {
			ID                 string  `json:"id"`
			Status             string  `json:"status"`
			DMARCPolicy        string  `json:"dmarc_policy"`
			SPFLookupCount     int     `json:"spf_lookup_count"`
			AlignmentRate      float64 `json:"alignment_rate"`
			UnknownSenderCount int     `json:"unknown_sender_count"`
		} `json:"domains"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if len(body.Domains) != 1 || body.Domains[0].ID != "domain-1" {
		t.Fatalf("unexpected domains: %+v", body.Domains)
	}
	if body.Domains[0].Status != "monitoring" || body.Domains[0].DMARCPolicy != "reject" || body.Domains[0].SPFLookupCount != 1 {
		t.Fatalf("dashboard mapping mismatch: %+v", body.Domains[0])
	}
	if body.Domains[0].AlignmentRate != 90 || body.Domains[0].UnknownSenderCount != 1 {
		t.Fatalf("reporter metrics mismatch: %+v", body.Domains[0])
	}
}

func TestDashboardDomainMissingReturnsJSON404(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	router := NewRouterWithDeps(Deps{
		APIKeys:          &webAPIKeyAuth{principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}}},
		APIDomainChecker: store,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/domains/missing", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON", contentType)
	}
}

func TestDashboardRecheckQueuesJob(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{domains: []domains.Domain{{ID: "domain-1", Domain: "example.com"}}}
	router := NewRouterWithDeps(Deps{
		APIKeys:           &webAPIKeyAuth{principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainWrite}}},
		APIDomainChecker:  store,
		APIDomainRechecks: store,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/domains/domain-1/recheck", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"job_id":"job-1"`) || !strings.Contains(rec.Body.String(), `"status":"queued"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestDashboardBrandRiskReturnsEmailAuthAndLookalikeEvidence(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{domains: []domains.Domain{{
		ID:     "domain-1",
		Domain: "example.com",
		LatestCheck: &domains.Check{
			CheckedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
		},
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys:          &webAPIKeyAuth{principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}}},
		APIDomainChecker: store,
		BrandRiskResolver: webLookalikeResolver{
			hosts: map[string][]string{"exmple.com": {"192.0.2.10"}},
			mx:    map[string][]*net.MX{"exampl.com": {{Host: "mail.example.", Pref: 10}}},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/domains/domain-1/brand-risk?tld=com&limit=25", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Findings []struct {
			Type     string `json:"type"`
			Severity string `json:"severity"`
		} `json:"findings"`
		Lookalikes []struct {
			Domain            string `json:"domain"`
			Severity          string `json:"severity"`
			HasAddressRecords bool   `json:"has_address_records"`
			HasMXRecords      bool   `json:"has_mx_records"`
		} `json:"lookalikes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if len(body.Findings) == 0 || body.Findings[0].Type != "dmarc_policy_relaxed" {
		t.Fatalf("email-auth findings missing: %+v", body.Findings)
	}
	if !hasLookalikeResponse(body.Lookalikes, "exmple.com", "Medium", true, false) {
		t.Fatalf("lookalikes missing address evidence: %+v", body.Lookalikes)
	}
	if !hasLookalikeResponse(body.Lookalikes, "exampl.com", "High", false, true) {
		t.Fatalf("lookalikes missing MX evidence: %+v", body.Lookalikes)
	}
}

func TestDashboardSettingsRejectsInvalidThreshold(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	router := NewRouterWithDeps(Deps{
		APIKeys:              &webAPIKeyAuth{principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainWrite}}},
		APIDashboardSettings: store,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/settings", strings.NewReader(`{"require_alignment_before_reject":101}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "require_alignment_before_reject") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func hasLookalikeResponse(rows []struct {
	Domain            string `json:"domain"`
	Severity          string `json:"severity"`
	HasAddressRecords bool   `json:"has_address_records"`
	HasMXRecords      bool   `json:"has_mx_records"`
}, domain string, severity string, hasAddress bool, hasMX bool) bool {
	for _, row := range rows {
		if row.Domain == domain && row.Severity == severity && row.HasAddressRecords == hasAddress && row.HasMXRecords == hasMX {
			return true
		}
	}
	return false
}

func TestAPIDomainsExportReturnsCSV(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/export", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomains: &webDomainStore{domains: []domains.Domain{{ID: "domain-1", Domain: "example.com"}}},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("Content-Type = %q, want text/csv", contentType)
	}
	if disposition := rec.Header().Get("Content-Disposition"); !strings.Contains(disposition, "dmark-protected-domains.csv") {
		t.Fatalf("Content-Disposition = %q, want CSV attachment", disposition)
	}
	if body := rec.Body.String(); !strings.Contains(body, "domain,created_at,last_checked_at") || !strings.Contains(body, "example.com") {
		t.Fatalf("body missing CSV export: %s", body)
	}
}

func TestAPIDomainsRejectsMissingScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeScanRead}},
		},
		APIDomains: &webDomainStore{domains: []domains.Domain{{ID: "domain-1", Domain: "example.com"}}},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIDomainHistoryReturnsFilteredChecks(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{history: []domains.HistoryEntry{
		{
			DomainID: "domain-1",
			Domain:   "example.com",
			Check: domains.Check{
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM ok"},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
				CheckedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			DomainID: "domain-1",
			Domain:   "example.com",
			Check: domains.Check{
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM ok"},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC ok"},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
				CheckedAt: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
			},
		},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/domain-1/history?type=dmarc&status=warn", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomainHistory: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"history"`) || !strings.Contains(body, `"DMARC"`) || !strings.Contains(body, `"warn"`) {
		t.Fatalf("body missing history response: %s", body)
	}
	if strings.Contains(body, `"DMARC ok"`) {
		t.Fatalf("body included unfiltered history entry: %s", body)
	}
}

func TestAPIDomainDriftReturnsEvents(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{driftEvents: []drift.Event{{
		CheckName:    "DMARC",
		Kind:         drift.KindPolicyRelaxed,
		Severity:     drift.SeverityCritical,
		Message:      "DMARC policy was relaxed.",
		BeforeStatus: scanner.StatusPass,
		AfterStatus:  scanner.StatusWarn,
		BeforeRecord: "v=DMARC1; p=reject",
		AfterRecord:  "v=DMARC1; p=none",
	}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/domain-1/drift", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomainDrift: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"events"`, `"DMARC"`, `"policy_relaxed"`, `"critical"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestAPIReporterSourcesReturnsRows(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{reportSources: []reporter.DashboardRow{{
		SourceIP:   "203.0.113.10",
		HeaderFrom: "example.com",
		Provider:   "Google",
		Confidence: "high",
		TotalCount: 42,
		PassCount:  40,
		FailCount:  2,
		Disposition: map[string]int{
			"none": 42,
		},
	}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/domain-1/reporter/sources", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeReporterRead}},
		},
		APIReporter: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"sources"`, `"203.0.113.10"`, `"Google"`, `"total_count":42`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestAPIReporterSendersReturnsInventory(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{reportSenders: []reporter.InventoryEntry{{
		SourceIP:    "203.0.113.10",
		Provider:    "Google",
		Confidence:  attribution.ConfidenceHigh,
		Volume:      100,
		FailureRate: 0.02,
		Expected:    true,
	}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/domain-1/reporter/senders", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeReporterRead}},
		},
		APIReporter: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"senders"`, `"203.0.113.10"`, `"risk":"expected"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestAPIReporterHealthReturnsStatus(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{reportHealth: reporter.HealthResult{
		Status:       "healthy",
		Message:      "DMARC reports are arriving as expected.",
		ReporterOrgs: []string{"Google"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/domain-1/reporter/health", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeReporterRead}},
		},
		APIReporter: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"status":"healthy"`, `"Google"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestAPIReporterRequiresReporterScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/domain-1/reporter/sources", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIReporter: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIWebhooksListCreateRotateAndDisable(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{webhookEndpoints: []webhooks.Endpoint{{
		ID:             "webhook-existing",
		OrganizationID: "org-1",
		URL:            "https://example.com/hook",
		EventTypes:     []string{"drift.created"},
		Enabled:        true,
		CreatedAt:      time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeWebhookManage}},
		},
		APIWebhooks: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if body := listRec.Body.String(); !strings.Contains(body, `"webhook-existing"`) || strings.Contains(body, `"secret"`) {
		t.Fatalf("list body unexpected: %s", body)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", strings.NewReader(`{"url":"https://example.org/hook","event_types":["drift.created","report.ingested"]}`))
	createReq.Header.Set("Authorization", "Bearer meil_test")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if body := createRec.Body.String(); !strings.Contains(body, `"webhook-1"`) || !strings.Contains(body, `"secret":"whsec_`) {
		t.Fatalf("create body missing webhook secret: %s", body)
	}

	rotateReq := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/webhook-existing/rotate-secret", nil)
	rotateReq.Header.Set("Authorization", "Bearer meil_test")
	rotateRec := httptest.NewRecorder()
	router.ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("rotate status = %d, want %d; body=%s", rotateRec.Code, http.StatusOK, rotateRec.Body.String())
	}
	if !strings.Contains(rotateRec.Body.String(), `"secret":"whsec_`) {
		t.Fatalf("rotate body missing new secret: %s", rotateRec.Body.String())
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/webhook-existing/disable", nil)
	disableReq.Header.Set("Authorization", "Bearer meil_test")
	disableRec := httptest.NewRecorder()
	router.ServeHTTP(disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want %d; body=%s", disableRec.Code, http.StatusOK, disableRec.Body.String())
	}
	if !strings.Contains(disableRec.Body.String(), `"enabled":false`) {
		t.Fatalf("disable body missing disabled endpoint: %s", disableRec.Body.String())
	}
}

func TestAPIWebhooksRequireManageScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIWebhooks: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIBillingCancelRecordsCancellation(t *testing.T) {
	t.Parallel()

	store := &webBillingStore{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/cancel", strings.NewReader(`{"reason":"switching providers","period_ends_at":"2026-06-16T12:00:00Z"}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeBillingManage}},
		},
		BillingCancellation: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.cancellation.OrganizationID != "org-1" || store.cancellation.Reason != "switching providers" {
		t.Fatalf("cancellation = %+v, want org-1 switching providers", store.cancellation)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"organization_id":"org-1"`) || !strings.Contains(body, `"effective_at":"2026-06-16T12:00:00Z"`) {
		t.Fatalf("body missing cancellation response: %s", body)
	}
}

func TestAPIBillingCancelRequiresManageScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/cancel", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		BillingCancellation: &webBillingStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIKeyManagementListCreateAndRevoke(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{apiKeys: []apikeys.Key{{
		ID:        "api-key-existing",
		Name:      "Existing",
		Scopes:    []apikeys.Scope{apikeys.ScopeScanRead},
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeAPIKeyManage}},
		},
		APIKeyManagement: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if body := listRec.Body.String(); !strings.Contains(body, `"api-key-existing"`) || strings.Contains(body, `"token"`) {
		t.Fatalf("list body unexpected: %s", body)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", strings.NewReader(`{"name":"Automation","scopes":["scan:read","domain:read"]}`))
	createReq.Header.Set("Authorization", "Bearer meil_test")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if body := createRec.Body.String(); !strings.Contains(body, `"api-key-1"`) || !strings.Contains(body, `"token":"meil_created"`) {
		t.Fatalf("create body missing one-time token: %s", body)
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys/api-key-existing/revoke", nil)
	revokeReq.Header.Set("Authorization", "Bearer meil_test")
	revokeRec := httptest.NewRecorder()
	router.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want %d; body=%s", revokeRec.Code, http.StatusOK, revokeRec.Body.String())
	}
	if !strings.Contains(revokeRec.Body.String(), `"revoked_at"`) {
		t.Fatalf("revoke body missing revoked timestamp: %s", revokeRec.Body.String())
	}
}

func TestAPIKeyManagementRequiresManageScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIKeyManagement: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPITeamInvitationsListAndCreate(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{invitations: []teams.Invitation{{
		ID:        "invite-existing",
		Email:     "person@example.com",
		Role:      entitlements.RoleViewer,
		ExpiresAt: time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeTeamManage}},
		},
		APITeams: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/team/invitations", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if body := listRec.Body.String(); !strings.Contains(body, `"invite-existing"`) || strings.Contains(body, `"token"`) {
		t.Fatalf("list body unexpected: %s", body)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/team/invitations", strings.NewReader(`{"email":"Admin@Example.com","role":"admin"}`))
	createReq.Header.Set("Authorization", "Bearer meil_test")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if body := createRec.Body.String(); !strings.Contains(body, `"invite-1"`) || !strings.Contains(body, `"admin@example.com"`) || !strings.Contains(body, `"token"`) {
		t.Fatalf("create body missing invitation token: %s", body)
	}
}

func TestAPITeamInvitationsRequireManageScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/team/invitations", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APITeams: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIDomainTransferCreatesRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/domain-1/transfer", strings.NewReader(`{"to_organization_id":"org-2","requested_by_user_id":"user-1"}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeTeamManage}},
		},
		APITeams: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"transfer-1"`, `"domain_id":"domain-1"`, `"to_organization_id":"org-2"`, `"status":"pending"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestAPIPrivacyRetentionGetAndUpdate(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopePrivacyManage}},
		},
		APIPrivacy: store,
	})

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/retention", nil)
	getReq.Header.Set("Authorization", "Bearer meil_test")
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), `"raw_report_days":90`) {
		t.Fatalf("get body missing default retention: %s", getRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/privacy/retention", strings.NewReader(`{"raw_report_days":30,"aggregate_row_days":365,"notification_log_days":90}`))
	updateReq.Header.Set("Authorization", "Bearer meil_test")
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}
	if store.retention.RawReportDays != 30 || store.retention.NotificationLogDays != 90 {
		t.Fatalf("retention = %+v, want saved settings", store.retention)
	}
}

func TestAPIPrivacyRetentionRejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/privacy/retention", strings.NewReader(`{"raw_report_days":400,"aggregate_row_days":365,"notification_log_days":90}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopePrivacyManage}},
		},
		APIPrivacy: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "raw report retention exceeds plan limit") {
		t.Fatalf("body missing validation error: %s", rec.Body.String())
	}
}

func TestAPIAdminCasesListCreateAndResolve(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{adminCases: []admin.Case{{
		ID:        "case-existing",
		Kind:      admin.CaseAbuse,
		Status:    admin.CaseOpen,
		Subject:   "Suspicious scans",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeAdminManage}},
		},
		APIAdmin: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases?status=open", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"case-existing"`) {
		t.Fatalf("list body missing existing case: %s", listRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(`{"kind":"support","subject":"Need help"}`))
	createReq.Header.Set("Authorization", "Bearer meil_test")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"case-1"`) {
		t.Fatalf("create body missing case id: %s", createRec.Body.String())
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/case-existing/resolve", strings.NewReader(`{"actor_user_id":"user-1"}`))
	resolveReq.Header.Set("Authorization", "Bearer meil_test")
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	router.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d; body=%s", resolveRec.Code, http.StatusOK, resolveRec.Body.String())
	}
	if body := resolveRec.Body.String(); !strings.Contains(body, `"status":"resolved"`) || !strings.Contains(body, `"assigned_to":"user-1"`) {
		t.Fatalf("resolve body missing resolved case: %s", body)
	}
}

func TestAPIAdminCasesRequireManageScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIAdmin: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIAdminAbuseDecisionsListAndCreate(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{abuseDecisions: []admin.AbuseDecision{{
		ID:         "abuse-existing",
		Identifier: "203.0.113.10",
		Action:     admin.AbuseActionBlock,
		Reason:     "scanner abuse",
		CreatedAt:  time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeAdminManage}},
		},
		APIAdmin: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/abuse-decisions?identifier=203.0.113.10", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"abuse-existing"`) {
		t.Fatalf("list body missing existing decision: %s", listRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/abuse-decisions", strings.NewReader(`{"identifier":"198.51.100.4","action":"block","reason":"bad crawler","duration":"1h"}`))
	createReq.Header.Set("Authorization", "Bearer meil_test")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if body := createRec.Body.String(); !strings.Contains(body, `"abuse-1"`) || !strings.Contains(body, `"198.51.100.4"`) {
		t.Fatalf("create body missing abuse decision: %s", body)
	}
}

func TestAPIStatusComponentsListAndUpsert(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{statusComponents: []statuspage.Component{{
		ID:        "component-existing",
		Name:      "API",
		Status:    statuspage.StatusOperational,
		UpdatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeAdminManage}},
		},
		APIStatus: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/status/components", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"component-existing"`) {
		t.Fatalf("list body missing status component: %s", listRec.Body.String())
	}

	upsertReq := httptest.NewRequest(http.MethodPost, "/api/v1/status/components", strings.NewReader(`{"name":"API","status":"degraded"}`))
	upsertReq.Header.Set("Authorization", "Bearer meil_test")
	upsertReq.Header.Set("Content-Type", "application/json")
	upsertRec := httptest.NewRecorder()
	router.ServeHTTP(upsertRec, upsertReq)
	if upsertRec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, want %d; body=%s", upsertRec.Code, http.StatusOK, upsertRec.Body.String())
	}
	if body := upsertRec.Body.String(); !strings.Contains(body, `"component-existing"`) || !strings.Contains(body, `"status":"degraded"`) {
		t.Fatalf("upsert body missing degraded component: %s", body)
	}
}

func TestAPIStatusIncidentsListCreateAndResolve(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{statusIncidents: []statuspage.Incident{{
		ID:        "incident-existing",
		Title:     "Delayed reports",
		Status:    "investigating",
		StartedAt: time.Date(2026, 5, 16, 11, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 5, 16, 11, 0, 0, 0, time.UTC),
	}}}
	router := NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeAdminManage}},
		},
		APIStatus: store,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/status/incidents", nil)
	listReq.Header.Set("Authorization", "Bearer meil_test")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"incident-existing"`) {
		t.Fatalf("list body missing incident: %s", listRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/status/incidents", strings.NewReader(`{"title":"API latency","status":"investigating","message":"Investigating elevated latency."}`))
	createReq.Header.Set("Authorization", "Bearer meil_test")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"incident-1"`) {
		t.Fatalf("create body missing incident id: %s", createRec.Body.String())
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/status/incidents/incident-existing/resolve", strings.NewReader(`{"message":"Recovered."}`))
	resolveReq.Header.Set("Authorization", "Bearer meil_test")
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	router.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d; body=%s", resolveRec.Code, http.StatusOK, resolveRec.Body.String())
	}
	if body := resolveRec.Body.String(); !strings.Contains(body, `"status":"resolved"`) || !strings.Contains(body, `"resolved_at"`) {
		t.Fatalf("resolve body missing resolved incident: %s", body)
	}
}

func TestAPIStatusRequiresManageScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status/components", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIStatus: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIDomainCreateReturnsDomain(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{"domain":"https://Example.com/path"}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainWrite}},
		},
		APIDomainCreator: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(store.domains) != 1 || store.domains[0].Domain != "example.com" {
		t.Fatalf("created domains = %+v, want normalized example.com", store.domains)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"domain":"example.com"`) {
		t.Fatalf("body missing domain response: %s", body)
	}
}

func TestAPIDomainCreateRequiresWriteScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{"domain":"example.com"}`))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomainCreator: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIDomainImportCreatesDomainsFromCSV(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/import", strings.NewReader("domain\nExample.com\nhttps://mail.example.org/path\n"))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainWrite}},
		},
		APIDomainCreator: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(store.domains) != 2 || store.domains[0].Domain != "example.com" || store.domains[1].Domain != "mail.example.org" {
		t.Fatalf("created domains = %+v, want normalized imported domains", store.domains)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"created":2`) || !strings.Contains(body, `"mail.example.org"`) {
		t.Fatalf("body missing import response: %s", body)
	}
}

func TestAPIDomainImportRequiresWriteScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/import", strings.NewReader("domain\nexample.com\n"))
	req.Header.Set("Authorization", "Bearer meil_test")
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomainCreator: &webDomainStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestAPIDomainRecheckScansAndStoresResult(t *testing.T) {
	t.Parallel()

	store := &webDomainStore{domains: []domains.Domain{{ID: "domain-1", Domain: "example.com"}}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/domain-1/check", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{
			Domain: "example.com",
			SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC ok"},
			BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
			DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		}},
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainWrite}},
		},
		APIDomainChecker: store,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.checkedDomain != "domain-1" {
		t.Fatalf("checked domain = %q, want domain-1", store.checkedDomain)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"domain":"example.com"`) || !strings.Contains(body, `"checks"`) {
		t.Fatalf("body missing scan response: %s", body)
	}
}

func TestAPIDomainRecheckRequiresWriteScope(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/domain-1/check", nil)
	req.Header.Set("Authorization", "Bearer meil_test")
	rec := httptest.NewRecorder()

	NewRouterWithDeps(Deps{
		Scanner: fakeScanner{result: scanner.Result{Domain: "example.com"}},
		APIKeys: &webAPIKeyAuth{
			principal: apikeys.Principal{OrganizationID: "org-1", Scopes: []apikeys.Scope{apikeys.ScopeDomainRead}},
		},
		APIDomainChecker: &webDomainStore{domains: []domains.Domain{{ID: "domain-1", Domain: "example.com"}}},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "required scope") {
		t.Fatalf("body missing scope error: %s", rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

func TestStatusEndpointReturnsJSON(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{"Dmark", "operational", "report ingestion"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestSecurityEndpointReturnsJSON(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/security", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{"contact", "API keys", "DMARC report ingestion rejects unsafe archives"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}
