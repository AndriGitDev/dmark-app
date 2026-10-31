// Package web wires HTTP routes and middleware.
package web

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AndriGitDev/meil/internal/admin"
	"github.com/AndriGitDev/meil/internal/alerts"
	apicontracts "github.com/AndriGitDev/meil/internal/api"
	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/brand"
	"github.com/AndriGitDev/meil/internal/brandrisk"
	"github.com/AndriGitDev/meil/internal/buildinfo"
	"github.com/AndriGitDev/meil/internal/dmarcjourney"
	"github.com/AndriGitDev/meil/internal/dnsprovider"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/entitlements"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/ops"
	"github.com/AndriGitDev/meil/internal/privacy"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/spfroutes"
	"github.com/AndriGitDev/meil/internal/statuspage"
	"github.com/AndriGitDev/meil/internal/teams"
	"github.com/AndriGitDev/meil/internal/webhooks"
	"github.com/AndriGitDev/meil/internal/workspace"
)

type EmailScanner interface {
	Scan(ctx context.Context, domain string) (scanner.Result, error)
}

type APIKeyAuthenticator interface {
	AuthenticateAPIKey(ctx context.Context, key string) (apikeys.Principal, error)
}

type OrganizationDomainLister interface {
	ListOrganizationDomains(ctx context.Context, organizationID string) ([]domains.Domain, error)
}

type OrganizationDomainCreator interface {
	CreateOrganizationDomain(ctx context.Context, organizationID string, domain string) (domains.Domain, error)
}

type OrganizationDomainChecker interface {
	GetOrganizationDomain(ctx context.Context, organizationID string, domainID string) (domains.Domain, error)
	CreateCheck(ctx context.Context, domainID string, result scanner.Result) error
}

type OrganizationDomainRecheckQueue interface {
	QueueOrganizationDomainRecheck(ctx context.Context, organizationID string, domainID string, at time.Time) (domains.RecheckJob, error)
}

type OrganizationSPFRouteStore interface {
	GetOrganizationSPFRoute(ctx context.Context, organizationID string, domainID string) (spfroutes.Route, error)
	UpsertOrganizationSPFRoute(ctx context.Context, organizationID string, route spfroutes.Route) (spfroutes.Route, error)
}

type OrganizationSPFRouteSyncQueue interface {
	QueueOrganizationSPFRouteSync(ctx context.Context, organizationID string, domainID string, routeID string, at time.Time) (string, error)
}

type OrganizationDomainHistoryStore interface {
	ListOrganizationDomainHistory(ctx context.Context, organizationID string, domainID string, limit int) ([]domains.HistoryEntry, error)
}

type OrganizationDomainTrendStore interface {
	ListOrganizationDomainCheckTimeline(ctx context.Context, organizationID string, domainID string, days int) ([]domains.HistoryEntry, error)
	ListOrganizationDomainAlignmentDaily(ctx context.Context, organizationID string, domainID string, days int) ([]reporter.AlignmentDay, error)
}

type OrganizationPlanStore interface {
	CountOrganizationDomains(ctx context.Context, organizationID string) (int, error)
}

type OperationalEventResourceStore interface {
	ListOperationalEventsByResource(ctx context.Context, resource string, limit int) ([]ops.Event, error)
}

type OrganizationDomainDriftStore interface {
	ListOrganizationDomainDrift(ctx context.Context, organizationID string, domainID string, limit int) ([]drift.Event, error)
}

type OrganizationReporterStore interface {
	ListOrganizationReporterSources(ctx context.Context, organizationID string, domainID string, limit int) ([]reporter.DashboardRow, error)
	ListOrganizationReporterSenders(ctx context.Context, organizationID string, domainID string, limit int) ([]reporter.InventoryEntry, error)
	GetOrganizationReporterHealth(ctx context.Context, organizationID string, domainID string) (reporter.HealthResult, error)
}

type UserReporterStore interface {
	ListUserReporterSenders(ctx context.Context, userID string, domainID string, limit int) ([]reporter.InventoryEntry, error)
	GetUserReporterHealth(ctx context.Context, userID string, domainID string) (reporter.HealthResult, error)
}

type OrganizationDashboardAlertStore interface {
	ListOrganizationDashboardAlerts(ctx context.Context, organizationID string, activeOnly bool, limit int) ([]alerts.DashboardAlert, error)
	AcknowledgeOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, reason string) error
	MuteOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, until time.Time, reason string) error
}

type OrganizationDashboardSettingsStore interface {
	GetOrganizationDashboardSettings(ctx context.Context, organizationID string) (domains.DashboardSettings, error)
	SaveOrganizationDashboardSettings(ctx context.Context, organizationID string, settings domains.DashboardSettings) (domains.DashboardSettings, error)
}

type AuditClaimStore interface {
	CreateAuditClaim(ctx context.Context, claim salesaudit.Claim) (salesaudit.Claim, error)
}

type OrganizationWebhookStore interface {
	ListOrganizationWebhookEndpoints(ctx context.Context, organizationID string) ([]webhooks.Endpoint, error)
	CreateOrganizationWebhookEndpoint(ctx context.Context, organizationID string, rawURL string, eventTypes []string) (webhooks.Endpoint, string, error)
	RotateOrganizationWebhookSecret(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, string, error)
	DisableOrganizationWebhookEndpoint(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, error)
}

type OrganizationAPIKeyStore interface {
	ListOrganizationAPIKeys(ctx context.Context, organizationID string) ([]apikeys.Key, error)
	CreateOrganizationAPIKey(ctx context.Context, organizationID string, name string, scopes []apikeys.Scope) (apikeys.Key, error)
	RevokeOrganizationAPIKey(ctx context.Context, organizationID string, keyID string) (apikeys.Key, error)
}

type OrganizationTeamStore interface {
	ListOrganizationInvitations(ctx context.Context, organizationID string) ([]teams.Invitation, error)
	CreateOrganizationInvitation(ctx context.Context, organizationID string, email string, role entitlements.Role) (teams.Invitation, error)
	RequestOrganizationDomainTransfer(ctx context.Context, organizationID string, domainID string, toOrganizationID string, requestedByUserID string) (teams.TransferRequest, error)
}

type OrganizationPrivacyStore interface {
	GetOrganizationRetention(ctx context.Context, organizationID string) (privacy.RetentionRequest, error)
	SaveOrganizationRetention(ctx context.Context, organizationID string, request privacy.RetentionRequest) (privacy.RetentionRequest, error)
}

type AdminCaseStore interface {
	ListAdminCases(ctx context.Context, status string, limit int) ([]admin.Case, error)
	CreateAdminCase(ctx context.Context, kind admin.CaseKind, subject string, userID string, domainID string) (admin.Case, error)
	ResolveAdminCase(ctx context.Context, caseID string, actorUserID string) (admin.Case, error)
	ListAbuseDecisions(ctx context.Context, identifier string, limit int) ([]admin.AbuseDecision, error)
	CreateAbuseDecision(ctx context.Context, identifier string, action admin.AbuseAction, reason string, duration time.Duration) (admin.AbuseDecision, error)
}

type OperationalEventStore interface {
	CreateOperationalEvent(ctx context.Context, event ops.Event) (ops.Event, error)
	ListOperationalEvents(ctx context.Context, severity string, limit int) ([]ops.Event, error)
}

type AdminSPFRouteAuditStore interface {
	GetSPFRoute(ctx context.Context, routeID string) (spfroutes.Route, error)
}

type DNSNameserverResolver interface {
	LookupNS(ctx context.Context, name string) ([]*net.NS, error)
}

type netDNSNameserverResolver struct{}

func (netDNSNameserverResolver) LookupNS(ctx context.Context, name string) ([]*net.NS, error) {
	return net.DefaultResolver.LookupNS(ctx, name)
}

type DNSPublicationResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupCNAME(ctx context.Context, name string) (string, error)
}

type netDNSPublicationResolver struct{}

func (netDNSPublicationResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

func (netDNSPublicationResolver) LookupCNAME(ctx context.Context, name string) (string, error) {
	return net.DefaultResolver.LookupCNAME(ctx, name)
}

type PrivacyDeletionStore interface {
	DeleteAccountData(ctx context.Context, userID string, email string) (privacy.DeletionPlan, error)
}

type StatusAdminStore interface {
	ListStatusComponents(ctx context.Context) ([]statuspage.Component, error)
	UpsertStatusComponent(ctx context.Context, name string, status statuspage.ComponentStatus) (statuspage.Component, error)
	ListStatusIncidents(ctx context.Context, limit int) ([]statuspage.Incident, error)
	CreateStatusIncident(ctx context.Context, title string, status string, message string) (statuspage.Incident, error)
	ResolveStatusIncident(ctx context.Context, incidentID string, message string) (statuspage.Incident, error)
}

type Deps struct {
	Workspace                    workspace.Store
	FrontendBaseURL              string
	Scanner                      EmailScanner
	APIKeys                      APIKeyAuthenticator
	APIDomains                   OrganizationDomainLister
	APIDomainCreator             OrganizationDomainCreator
	APIDomainChecker             OrganizationDomainChecker
	APIDomainRechecks            OrganizationDomainRecheckQueue
	APISPFRoutes                 OrganizationSPFRouteStore
	APISPFRouteSyncs             OrganizationSPFRouteSyncQueue
	APIDomainHistory             OrganizationDomainHistoryStore
	APIDomainTrends              OrganizationDomainTrendStore
	APIPlan                      OrganizationPlanStore
	APISPFRouteHistory           OperationalEventResourceStore
	APIDomainDrift               OrganizationDomainDriftStore
	APIReporter                  OrganizationReporterStore
	APIDashboardAlerts           OrganizationDashboardAlertStore
	APIDashboardSettings         OrganizationDashboardSettingsStore
	BrandRiskResolver            brandrisk.LookalikeResolver
	UserReporter                 UserReporterStore
	AuditClaims                  AuditClaimStore
	APIWebhooks                  OrganizationWebhookStore
	APIKeyManagement             OrganizationAPIKeyStore
	APITeams                     OrganizationTeamStore
	APIPrivacy                   OrganizationPrivacyStore
	APIAdmin                     AdminCaseStore
	APIOperations                OperationalEventStore
	AdminSPFRouteAudits          AdminSPFRouteAuditStore
	DNSNameserverResolver        DNSNameserverResolver
	DNSNameserverTimeout         time.Duration
	DNSPublicationResolver       DNSPublicationResolver
	DNSPublicationTimeout        time.Duration
	PrivacyDeletion              PrivacyDeletionStore
	APIStatus                    StatusAdminStore
	Auth                         *auth.Service
	Domains                      *domains.Service
	Alerts                       alerts.Store
	Sender                       notifier.EmailSender
	SessionManager               *scs.SessionManager
	PublicBaseURL                string
	PublicBaseURLConfigured      bool
	FrontendOrigins              []string
	PublicChecksPerMinute        int
	LoginAttemptsPerMinute       int
	BunnyDNSConfigured           bool
	BunnyDNSZoneIDConfigured     bool
	BootstrapConfigured          bool
	BootstrapPartiallyConfigured bool
	BootstrapSkipped             bool
	SPFRouteSchedulerEnabled     bool
	SPFRouteSchedulerInterval    time.Duration
	SPFRouteScheduledSyncEvery   time.Duration
	WorkerQueueName              string
	WorkerMaxWorkers             int
	WorkerRegisteredJobs         []string
}

var defaultWorkerRegisteredJobs = []string{"scan_domain", "refresh_enrichment", "sync_spf_route"}

func NewRouter() http.Handler {
	return NewRouterWithScanner(scanner.New(nil))
}

func NewRouterWithScanner(emailScanner EmailScanner) http.Handler {
	return NewRouterWithDeps(Deps{Scanner: emailScanner})
}

func NewRouterWithDeps(deps Deps) http.Handler {
	if deps.Scanner == nil {
		deps.Scanner = scanner.New(nil)
	}
	if deps.PublicBaseURL == "" {
		deps.PublicBaseURL = "http://localhost:8080"
	}
	if deps.SessionManager == nil {
		deps.SessionManager = scs.New()
	}
	configureSessionManager(deps.SessionManager, deps.PublicBaseURL)
	if deps.PublicChecksPerMinute <= 0 {
		deps.PublicChecksPerMinute = 12
	}
	if deps.LoginAttemptsPerMinute <= 0 {
		deps.LoginAttemptsPerMinute = 6
	}
	if deps.DNSNameserverResolver == nil {
		deps.DNSNameserverResolver = netDNSNameserverResolver{}
	}
	if deps.DNSNameserverTimeout <= 0 {
		deps.DNSNameserverTimeout = 2 * time.Second
	}
	if deps.DNSPublicationResolver == nil {
		deps.DNSPublicationResolver = netDNSPublicationResolver{}
	}
	if deps.DNSPublicationTimeout <= 0 {
		deps.DNSPublicationTimeout = 2 * time.Second
	}
	if strings.TrimSpace(deps.WorkerQueueName) == "" {
		deps.WorkerQueueName = "default"
	}
	if deps.WorkerMaxWorkers <= 0 {
		deps.WorkerMaxWorkers = 4
	}
	if len(deps.WorkerRegisteredJobs) == 0 {
		deps.WorkerRegisteredJobs = append([]string(nil), defaultWorkerRegisteredJobs...)
	}

	r := chi.NewRouter()
	r.Use(securityHeadersMiddleware)
	r.Use(requestIDMiddleware)
	r.Use(legacyHostRedirect)
	r.Use(corsMiddleware(deps.FrontendOrigins))
	r.Use(deps.SessionManager.LoadAndSave)
	r.Use(backendOnlySurface)
	registerAccountRoutes(r, deps)
	publicLimiter := newPublicRateLimiter(deps.PublicChecksPerMinute)
	r.Get("/api/public/scan", func(w http.ResponseWriter, r *http.Request) {
		if !publicLimiter.allow(publicRateKey(r), time.Now()) {
			writeAPIError(w, http.StatusTooManyRequests, "public scan rate limit exceeded")
			return
		}
		domain, err := publicScanDomain(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := deps.Scanner.Scan(r.Context(), domain)
		if err != nil {
			recordPublicScanOperationalEvent(r, deps.APIOperations, domain, http.StatusBadRequest, false)
			writeAPIError(w, http.StatusBadRequest, "scan failed")
			return
		}
		recordPublicScanOperationalEvent(r, deps.APIOperations, domain, http.StatusOK, true)
		writeJSON(w, http.StatusOK, apicontracts.NewScanResponse(result))
	})
	r.Post("/api/public/scan", func(w http.ResponseWriter, r *http.Request) {
		if !publicLimiter.allow(publicRateKey(r), time.Now()) {
			writeAPIError(w, http.StatusTooManyRequests, "public scan rate limit exceeded")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		domain, err := publicScanDomain(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := deps.Scanner.Scan(r.Context(), domain)
		if err != nil {
			recordPublicScanOperationalEvent(r, deps.APIOperations, domain, http.StatusBadRequest, false)
			writeAPIError(w, http.StatusBadRequest, "scan failed")
			return
		}
		recordPublicScanOperationalEvent(r, deps.APIOperations, domain, http.StatusOK, true)
		writeJSON(w, http.StatusOK, apicontracts.NewScanResponse(result))
	})
	r.Post("/api/public/audit-claims", func(w http.ResponseWriter, r *http.Request) {
		if !publicLimiter.allow(publicRateKey(r)+":claim", time.Now()) {
			writeAPIError(w, http.StatusTooManyRequests, "public claim rate limit exceeded")
			return
		}
		if deps.AuditClaims == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "audit claim storage is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Domain string `json:"domain"`
			Email  string `json:"email"`
		}
		if err := decodePublicJSONBody(r.Body, &payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		domain, err := scanner.NormalizeDomain(payload.Domain)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		email, err := normalizePublicEmail(payload.Email)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		emailDomain := publicEmailDomain(email)
		result, err := deps.Scanner.Scan(r.Context(), domain)
		if err != nil {
			recordPublicAuditClaimOperationalEvent(r, deps.APIOperations, domain, emailDomain, http.StatusBadRequest, "audit_claim.failed", ops.SeverityWarn, "audit claim failed", map[string]any{"error_class": "scanner_error"})
			writeAPIError(w, http.StatusBadRequest, "audit claim scan failed")
			return
		}
		audit := salesaudit.Assess(result, salesaudit.Options{RequestedEmail: email})
		if audit.BlockedReason != "" {
			recordPublicAuditClaimOperationalEvent(r, deps.APIOperations, domain, emailDomain, http.StatusBadRequest, "audit_claim.blocked", ops.SeverityWarn, "audit claim blocked", map[string]any{"blocked_reason": audit.BlockedReason})
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": audit.BlockedReason,
				"audit": apicontracts.AuditResponse{
					Grade:         audit.Grade,
					Risk:          audit.Risk,
					Safe:          audit.Safe,
					ProofEligible: audit.ProofEligible,
					Message:       audit.Message,
					CTA:           audit.CTA,
					BlockedReason: audit.BlockedReason,
				},
			})
			return
		}
		claim, err := salesaudit.NewClaim(domain, email, audit, time.Now())
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		claim, err = deps.AuditClaims.CreateAuditClaim(r.Context(), claim)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "saving audit claim failed")
			return
		}
		recordPublicAuditClaimOperationalEvent(r, deps.APIOperations, domain, emailDomain, http.StatusCreated, "audit_claim.created", ops.SeverityInfo, "audit claim created", map[string]any{
			"claim_id":       claim.ID,
			"proof_eligible": audit.ProofEligible,
		})
		writeJSON(w, http.StatusCreated, map[string]any{
			"claim_id": claim.ID,
			"status":   claim.Status,
			"audit": apicontracts.AuditResponse{
				Grade:         audit.Grade,
				Risk:          audit.Risk,
				Safe:          audit.Safe,
				ProofEligible: audit.ProofEligible,
				Message:       audit.Message,
				CTA:           audit.CTA,
				BlockedReason: audit.BlockedReason,
			},
		})
	})
	r.Get("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeScanRead); !ok {
			return
		}
		domain := strings.TrimSpace(r.URL.Query().Get("domain"))
		if domain == "" {
			writeAPIError(w, http.StatusBadRequest, "domain query parameter is required")
			return
		}
		if len(domain) > 253 {
			writeAPIError(w, http.StatusBadRequest, "domain is too long")
			return
		}
		result, err := deps.Scanner.Scan(r.Context(), domain)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewScanResponse(result))
	})
	r.Get("/api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		items, err := dashboardDomainResponses(r.Context(), deps, principal.OrganizationID, apikeys.Allows(principal, apikeys.ScopeReporterRead))
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error())
			return
		}
		alertCount := 0
		if deps.APIDashboardAlerts != nil {
			if activeAlerts, err := deps.APIDashboardAlerts.ListOrganizationDashboardAlerts(r.Context(), principal.OrganizationID, true, 200); err == nil {
				alertCount = len(activeAlerts)
			}
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardResponse(items, alertCount, time.Now().UTC()))
	})
	r.Get("/api/v1/dashboard/domains", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		items, err := dashboardDomainResponses(r.Context(), deps, principal.OrganizationID, apikeys.Allows(principal, apikeys.ScopeReporterRead))
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.DashboardDomainResponse{"domains": items})
	})
	r.Get("/api/v1/dashboard/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDomainChecker == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		domain, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain was not found")
			return
		}
		var senders []reporter.InventoryEntry
		if apikeys.Allows(principal, apikeys.ScopeReporterRead) {
			senders = dashboardReporterSenders(r.Context(), deps.APIReporter, principal.OrganizationID, domain.ID, 200)
		}
		history := []apicontracts.HistoryResponse{}
		if deps.APIDomainHistory != nil {
			entries, err := deps.APIDomainHistory.ListOrganizationDomainHistory(r.Context(), principal.OrganizationID, domain.ID, 25)
			if err == nil {
				for _, entry := range entries {
					history = append(history, apicontracts.NewHistoryResponse(entry))
				}
			}
		}
		writeJSON(w, http.StatusOK, apicontracts.DashboardDomainDetailResponse{
			Domain:  apicontracts.NewDashboardDomainResponse(domain, senders),
			History: history,
		})
	})
	r.Get("/api/v1/dashboard/domains/{id}/dns-guidance", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		domain, ok := dashboardDomainForAPI(w, r, deps, principal.OrganizationID)
		if !ok {
			return
		}
		response := dashboardDNSGuidanceResponse(r.Context(), deps, principal.OrganizationID, domain, time.Now().UTC())
		writeJSON(w, http.StatusOK, response)
	})
	r.Post("/api/v1/dashboard/domains/{id}/recheck", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDomainChecker == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		domain, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain was not found")
			return
		}
		now := time.Now().UTC()
		if deps.APIDomainRechecks != nil {
			job, err := deps.APIDomainRechecks.QueueOrganizationDomainRecheck(r.Context(), principal.OrganizationID, domain.ID, now)
			if err != nil {
				recordDashboardRecheckOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, http.StatusTooManyRequests, "dashboard.recheck.failed", ops.SeverityWarn, "dashboard recheck queue failed", map[string]any{"error_class": "queue_error"})
				writeAPIError(w, http.StatusTooManyRequests, err.Error())
				return
			}
			recordDashboardRecheckOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, http.StatusAccepted, "dashboard.recheck.queued", ops.SeverityInfo, "dashboard recheck queued", map[string]any{"job_id": job.ID, "status": job.Status})
			writeJSON(w, http.StatusAccepted, apicontracts.DashboardRecheckResponse{
				JobID:       job.ID,
				DomainID:    job.DomainID,
				Status:      job.Status,
				QueuedAt:    job.QueuedAt.UTC(),
				NextAllowed: job.NextAllowed.UTC(),
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		result, err := deps.Scanner.Scan(ctx, domain.Domain)
		if err != nil {
			recordDashboardRecheckOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, http.StatusBadRequest, "dashboard.recheck.failed", ops.SeverityWarn, "dashboard recheck failed", map[string]any{"error_class": "scanner_error"})
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := deps.APIDomainChecker.CreateCheck(r.Context(), domain.ID, result); err != nil {
			recordDashboardRecheckOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, http.StatusInternalServerError, "dashboard.recheck.failed", ops.SeverityError, "dashboard recheck save failed", map[string]any{"error_class": "store_error"})
			writeAPIError(w, http.StatusInternalServerError, "saving check failed")
			return
		}
		recordDashboardRecheckOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, http.StatusAccepted, "dashboard.recheck.completed", ops.SeverityInfo, "dashboard recheck completed", map[string]any{"status": "completed"})
		writeJSON(w, http.StatusAccepted, apicontracts.DashboardRecheckResponse{
			DomainID: domain.ID,
			Status:   "completed",
			QueuedAt: now,
		})
	})
	r.Get("/api/v1/dashboard/domains/{id}/spf-route", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		domain, ok := dashboardDomainForAPI(w, r, deps, principal.OrganizationID)
		if !ok {
			return
		}
		if deps.APISPFRoutes == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "SPF route API is not configured")
			return
		}
		route, err := deps.APISPFRoutes.GetOrganizationSPFRoute(r.Context(), principal.OrganizationID, domain.ID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "SPF route was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardSPFRouteResponse(route))
	})
	r.Put("/api/v1/dashboard/domains/{id}/spf-route", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		domain, ok := dashboardDomainForAPI(w, r, deps, principal.OrganizationID)
		if !ok {
			return
		}
		if deps.APISPFRoutes == nil || deps.APISPFRouteSyncs == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "SPF route sync queue is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		route, err := dashboardSPFRouteFromRequest(r, principal.OrganizationID, domain)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		saved, err := deps.APISPFRoutes.UpsertOrganizationSPFRoute(r.Context(), principal.OrganizationID, route)
		if err != nil {
			recordDashboardSPFRouteOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, route, http.StatusInternalServerError, "dashboard.spf_route.failed", ops.SeverityError, "dashboard SPF route save failed", map[string]any{"error_class": "store_error"})
			writeAPIError(w, http.StatusInternalServerError, "saving SPF route failed")
			return
		}
		now := time.Now().UTC()
		recordDashboardSPFRouteOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, saved, http.StatusOK, "dashboard.spf_route.upserted", ops.SeverityInfo, "dashboard SPF route upserted", nil)
		jobID, err := deps.APISPFRouteSyncs.QueueOrganizationSPFRouteSync(r.Context(), principal.OrganizationID, domain.ID, saved.ID, now)
		if err != nil {
			recordDashboardSPFRouteOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, saved, http.StatusInternalServerError, "dashboard.spf_route.sync_failed", ops.SeverityError, "dashboard SPF route sync queue failed", map[string]any{"error_class": "queue_error"})
			writeAPIError(w, http.StatusInternalServerError, "queueing SPF route sync failed")
			return
		}
		recordDashboardSPFRouteOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, saved, http.StatusOK, "dashboard.spf_route.sync_queued", ops.SeverityInfo, "dashboard SPF route sync queued", map[string]any{"job_id": jobID})
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardSPFRouteResponse(saved))
	})
	r.Post("/api/v1/dashboard/domains/{id}/spf-route/sync", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		domain, ok := dashboardDomainForAPI(w, r, deps, principal.OrganizationID)
		if !ok {
			return
		}
		if deps.APISPFRoutes == nil || deps.APISPFRouteSyncs == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "SPF route sync queue is not configured")
			return
		}
		route, err := deps.APISPFRoutes.GetOrganizationSPFRoute(r.Context(), principal.OrganizationID, domain.ID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "SPF route was not found")
			return
		}
		now := time.Now().UTC()
		jobID, err := deps.APISPFRouteSyncs.QueueOrganizationSPFRouteSync(r.Context(), principal.OrganizationID, domain.ID, route.ID, now)
		if err != nil {
			recordDashboardSPFRouteOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, route, http.StatusInternalServerError, "dashboard.spf_route.sync_failed", ops.SeverityError, "dashboard SPF route sync queue failed", map[string]any{"error_class": "queue_error"})
			writeAPIError(w, http.StatusInternalServerError, "queueing SPF route sync failed")
			return
		}
		recordDashboardSPFRouteOperationalEvent(r, deps.APIOperations, principal.OrganizationID, domain, route, http.StatusAccepted, "dashboard.spf_route.sync_queued", ops.SeverityInfo, "dashboard SPF route sync queued", map[string]any{"job_id": jobID})
		writeJSON(w, http.StatusAccepted, apicontracts.DashboardSPFRouteSyncResponse{JobID: jobID, RouteID: route.ID, DomainID: domain.ID, Status: "queued", QueuedAt: now})
	})
	r.Get("/api/v1/dashboard/domains/{id}/change-plan", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDomainChecker == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		domain, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain was not found")
			return
		}
		evidence := dmarcjourney.Evidence{}
		if apikeys.Allows(principal, apikeys.ScopeReporterRead) {
			evidence = dashboardDMARCEvidence(r.Context(), deps.APIReporter, principal.OrganizationID, domain.ID)
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardChangePlanResponse(domain, evidence, time.Now().UTC()))
	})
	r.Get("/api/v1/dashboard/domains/{id}/trends", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDomainTrends == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain trends API is not configured")
			return
		}
		domain, ok := dashboardDomainForAPI(w, r, deps, principal.OrganizationID)
		if !ok {
			return
		}
		days, err := dashboardTrendDays(r.URL.Query().Get("days"))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		timeline, err := deps.APIDomainTrends.ListOrganizationDomainCheckTimeline(r.Context(), principal.OrganizationID, domain.ID, days)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading domain trends failed")
			return
		}
		alignment := []reporter.AlignmentDay{}
		if apikeys.Allows(principal, apikeys.ScopeReporterRead) {
			alignment, err = deps.APIDomainTrends.ListOrganizationDomainAlignmentDaily(r.Context(), principal.OrganizationID, domain.ID, days)
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "loading alignment trends failed")
				return
			}
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardTrendsResponse(domain, timeline, alignment, days, time.Now().UTC()))
	})
	r.Get("/api/v1/dashboard/domains/{id}/spf-route/history", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		domain, ok := dashboardDomainForAPI(w, r, deps, principal.OrganizationID)
		if !ok {
			return
		}
		if deps.APISPFRoutes == nil || deps.APISPFRouteHistory == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "SPF route history API is not configured")
			return
		}
		route, err := deps.APISPFRoutes.GetOrganizationSPFRoute(r.Context(), principal.OrganizationID, domain.ID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "SPF route was not found")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		events, err := deps.APISPFRouteHistory.ListOperationalEventsByResource(r.Context(), "spf_route:"+route.ID, limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading SPF route history failed")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardSPFRouteHistoryResponse(route, events, time.Now().UTC()))
	})
	r.Get("/api/v1/dashboard/plan", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIPlan == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "resource API is not configured")
			return
		}
		count, err := deps.APIPlan.CountOrganizationDomains(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "counting organization domains failed")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardPlanResponse(count))
	})
	r.Get("/api/v1/dashboard/domains/{id}/brand-risk", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDomainChecker == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		domain, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain was not found")
			return
		}
		options, err := dashboardLookalikeOptions(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		lookalikes := brandrisk.EvaluateLookalikeCandidates(ctx, deps.BrandRiskResolver, domain.Domain, options)
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardBrandRiskResponse(domain, lookalikes, time.Now().UTC()))
	})
	r.Get("/api/v1/dashboard/domains/{id}/reporter", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeReporterRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDomainChecker == nil || deps.APIReporter == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "Reporter API is not configured")
			return
		}
		domain, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain was not found")
			return
		}
		senders := dashboardReporterSenders(r.Context(), deps.APIReporter, principal.OrganizationID, domain.ID, 200)
		health, _ := deps.APIReporter.GetOrganizationReporterHealth(r.Context(), principal.OrganizationID, domain.ID)
		unknownOnly := strings.EqualFold(r.URL.Query().Get("filter"), "unknown")
		response := apicontracts.NewDashboardReporterResponse(domain, senders, health, unknownOnly)
		if strings.EqualFold(r.URL.Query().Get("format"), "csv") {
			writeDashboardReporterCSV(w, response)
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	r.Get("/api/v1/dashboard/alerts", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDashboardAlerts == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "dashboard alerts are not configured")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 100, 500)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		activeOnly := !strings.EqualFold(r.URL.Query().Get("active"), "false")
		alerts, err := deps.APIDashboardAlerts.ListOrganizationDashboardAlerts(r.Context(), principal.OrganizationID, activeOnly, limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading dashboard alerts failed")
			return
		}
		response := make([]apicontracts.DashboardAlertResponse, 0, len(alerts))
		for _, alert := range alerts {
			response = append(response, apicontracts.NewDashboardAlertResponse(alert))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.DashboardAlertResponse{"alerts": response})
	})
	r.Post("/api/v1/dashboard/alerts/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDashboardAlerts == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "dashboard alerts are not configured")
			return
		}
		reason, err := dashboardReason(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := deps.APIDashboardAlerts.AcknowledgeOrganizationDashboardAlert(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), reason); err != nil {
			writeAPIError(w, http.StatusNotFound, "alert was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.DashboardAlertActionResponse{ID: chi.URLParam(r, "id"), Status: "acknowledged", AppliedAt: time.Now().UTC()})
	})
	r.Post("/api/v1/dashboard/alerts/{id}/mute", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDashboardAlerts == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "dashboard alerts are not configured")
			return
		}
		reason, until, err := dashboardMuteRequest(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := deps.APIDashboardAlerts.MuteOrganizationDashboardAlert(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), until, reason); err != nil {
			writeAPIError(w, http.StatusNotFound, "alert was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.DashboardAlertActionResponse{ID: chi.URLParam(r, "id"), Status: "muted", AppliedAt: time.Now().UTC()})
	})
	r.Get("/api/v1/dashboard/settings", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDashboardSettings == nil {
			writeJSON(w, http.StatusOK, apicontracts.NewDashboardSettingsResponse(domains.DefaultDashboardSettings()))
			return
		}
		settings, err := deps.APIDashboardSettings.GetOrganizationDashboardSettings(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading dashboard settings failed")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardSettingsResponse(settings))
	})
	r.Post("/api/v1/dashboard/settings", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if !requireOrganizationAPI(w, principal) {
			return
		}
		if deps.APIDashboardSettings == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "dashboard settings are not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			RequireAlignmentBeforeReject        int      `json:"require_alignment_before_reject"`
			PauseAutomationOnUnknownSenderSpike bool     `json:"pause_automation_on_unknown_senders_spike"`
			AutoGenerateMonthlyExecutiveReports bool     `json:"auto_generate_monthly_executive_reports"`
			NotificationChannels                []string `json:"notification_channels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		settings, err := domains.NormalizeDashboardSettings(domains.DashboardSettings{
			RequireAlignmentBeforeReject:        payload.RequireAlignmentBeforeReject,
			PauseAutomationOnUnknownSenderSpike: payload.PauseAutomationOnUnknownSenderSpike,
			AutoGenerateMonthlyExecutiveReports: payload.AutoGenerateMonthlyExecutiveReports,
			NotificationChannels:                payload.NotificationChannels,
		})
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		settings, err = deps.APIDashboardSettings.SaveOrganizationDashboardSettings(r.Context(), principal.OrganizationID, settings)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "saving dashboard settings failed")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewDashboardSettingsResponse(settings))
	})
	r.Get("/api/v1/domains", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if deps.APIDomains == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		saved, err := deps.APIDomains.ListOrganizationDomains(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading domains failed")
			return
		}
		response := make([]apicontracts.DomainResponse, 0, len(saved))
		for _, domain := range saved {
			response = append(response, apicontracts.NewDomainResponse(domain))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.DomainResponse{"domains": response})
	})
	r.Get("/api/v1/domains/export", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if deps.APIDomains == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		saved, err := deps.APIDomains.ListOrganizationDomains(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "exporting domains failed")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="dmark-protected-domains.csv"`)
		if err := domains.WriteCSV(w, saved); err != nil {
			// Headers are already committed, so fall back to the connection error path.
			return
		}
	})
	r.Get("/api/v1/domains/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if deps.APIDomainHistory == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain history API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		history, err := deps.APIDomainHistory.ListOrganizationDomainHistory(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), limit)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain history was not found")
			return
		}
		filtered := domains.FilterHistory(history, domains.HistoryFilter{
			Status: scanner.Status(r.URL.Query().Get("status")),
			Type:   domains.CheckType(r.URL.Query().Get("type")),
		})
		response := make([]apicontracts.HistoryResponse, 0, len(filtered))
		for _, entry := range filtered {
			response = append(response, apicontracts.NewHistoryResponse(entry))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.HistoryResponse{"history": response})
	})
	r.Get("/api/v1/domains/{id}/drift", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainRead)
		if !ok {
			return
		}
		if deps.APIDomainDrift == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain drift API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		events, err := deps.APIDomainDrift.ListOrganizationDomainDrift(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), limit)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain drift was not found")
			return
		}
		response := make([]apicontracts.DriftEventResponse, 0, len(events))
		for _, event := range events {
			response = append(response, apicontracts.NewDriftEventResponse(event))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.DriftEventResponse{"events": response})
	})
	r.Get("/api/v1/domains/{id}/reporter/sources", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeReporterRead)
		if !ok {
			return
		}
		if deps.APIReporter == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "Reporter API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		rows, err := deps.APIReporter.ListOrganizationReporterSources(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), limit)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "Reporter sources were not found")
			return
		}
		response := make([]apicontracts.ReporterSourceResponse, 0, len(rows))
		for _, row := range rows {
			response = append(response, apicontracts.NewReporterSourceResponse(row))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.ReporterSourceResponse{"sources": response})
	})
	r.Get("/api/v1/domains/{id}/reporter/senders", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeReporterRead)
		if !ok {
			return
		}
		if deps.APIReporter == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "Reporter API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		entries, err := deps.APIReporter.ListOrganizationReporterSenders(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), limit)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "Reporter senders were not found")
			return
		}
		response := make([]apicontracts.ReporterSenderResponse, 0, len(entries))
		for _, entry := range entries {
			response = append(response, apicontracts.NewReporterSenderResponse(entry))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.ReporterSenderResponse{"senders": response})
	})
	r.Get("/api/v1/domains/{id}/reporter/health", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeReporterRead)
		if !ok {
			return
		}
		if deps.APIReporter == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "Reporter API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		health, err := deps.APIReporter.GetOrganizationReporterHealth(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "Reporter health was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewReporterHealthResponse(health))
	})
	r.Get("/api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeWebhookManage)
		if !ok {
			return
		}
		if deps.APIWebhooks == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "webhook API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		endpoints, err := deps.APIWebhooks.ListOrganizationWebhookEndpoints(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading webhooks failed")
			return
		}
		response := make([]apicontracts.WebhookEndpointResponse, 0, len(endpoints))
		for _, endpoint := range endpoints {
			response = append(response, apicontracts.NewWebhookEndpointResponse(endpoint, ""))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.WebhookEndpointResponse{"webhooks": response})
	})
	r.Post("/api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeWebhookManage)
		if !ok {
			return
		}
		if deps.APIWebhooks == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "webhook API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		var payload struct {
			URL        string   `json:"url"`
			EventTypes []string `json:"event_types"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		endpoint, secret, err := deps.APIWebhooks.CreateOrganizationWebhookEndpoint(r.Context(), principal.OrganizationID, payload.URL, payload.EventTypes)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewWebhookEndpointResponse(endpoint, secret))
	})
	r.Post("/api/v1/webhooks/{id}/rotate-secret", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeWebhookManage)
		if !ok {
			return
		}
		if deps.APIWebhooks == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "webhook API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		endpoint, secret, err := deps.APIWebhooks.RotateOrganizationWebhookSecret(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "webhook endpoint was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewWebhookEndpointResponse(endpoint, secret))
	})
	r.Post("/api/v1/webhooks/{id}/disable", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeWebhookManage)
		if !ok {
			return
		}
		if deps.APIWebhooks == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "webhook API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		endpoint, err := deps.APIWebhooks.DisableOrganizationWebhookEndpoint(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "webhook endpoint was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewWebhookEndpointResponse(endpoint, ""))
	})
	r.Get("/api/v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAPIKeyManage)
		if !ok {
			return
		}
		if deps.APIKeyManagement == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "API key management is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		keys, err := deps.APIKeyManagement.ListOrganizationAPIKeys(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading API keys failed")
			return
		}
		response := make([]apicontracts.APIKeyResponse, 0, len(keys))
		for _, key := range keys {
			response = append(response, apicontracts.NewAPIKeyResponse(key))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.APIKeyResponse{"api_keys": response})
	})
	r.Post("/api/v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAPIKeyManage)
		if !ok {
			return
		}
		if deps.APIKeyManagement == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "API key management is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		scopes := make([]apikeys.Scope, 0, len(payload.Scopes))
		for _, scope := range payload.Scopes {
			scopes = append(scopes, apikeys.Scope(scope))
		}
		key, err := deps.APIKeyManagement.CreateOrganizationAPIKey(r.Context(), principal.OrganizationID, payload.Name, scopes)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewAPIKeyResponse(key))
	})
	r.Post("/api/v1/api-keys/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAPIKeyManage)
		if !ok {
			return
		}
		if deps.APIKeyManagement == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "API key management is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		key, err := deps.APIKeyManagement.RevokeOrganizationAPIKey(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "API key was not found")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewAPIKeyResponse(key))
	})
	r.Get("/api/v1/team/invitations", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeTeamManage)
		if !ok {
			return
		}
		if deps.APITeams == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "team API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		invitations, err := deps.APITeams.ListOrganizationInvitations(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading team invitations failed")
			return
		}
		response := make([]apicontracts.TeamInvitationResponse, 0, len(invitations))
		for _, invitation := range invitations {
			response = append(response, apicontracts.NewTeamInvitationResponse(invitation))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.TeamInvitationResponse{"invitations": response})
	})
	r.Post("/api/v1/team/invitations", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeTeamManage)
		if !ok {
			return
		}
		if deps.APITeams == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "team API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		invitation, err := deps.APITeams.CreateOrganizationInvitation(r.Context(), principal.OrganizationID, payload.Email, entitlements.Role(payload.Role))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewTeamInvitationResponse(invitation))
	})
	r.Post("/api/v1/domains/{id}/transfer", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeTeamManage)
		if !ok {
			return
		}
		if deps.APITeams == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "team API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			ToOrganizationID  string `json:"to_organization_id"`
			RequestedByUserID string `json:"requested_by_user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		request, err := deps.APITeams.RequestOrganizationDomainTransfer(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"), payload.ToOrganizationID, payload.RequestedByUserID)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewDomainTransferResponse(request))
	})
	r.Get("/api/v1/privacy/retention", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopePrivacyManage)
		if !ok {
			return
		}
		if deps.APIPrivacy == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "privacy API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		settings, err := deps.APIPrivacy.GetOrganizationRetention(r.Context(), principal.OrganizationID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading retention settings failed")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	})
	r.Post("/api/v1/privacy/retention", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopePrivacyManage)
		if !ok {
			return
		}
		if deps.APIPrivacy == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "privacy API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var request privacy.RetentionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		validated, err := privacy.ValidateRetentionRequest(request, privacy.RetentionLimits{
			MaxRawReportDays:       365,
			MaxAggregateRowDays:    730,
			MaxNotificationLogDays: 365,
		})
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		settings, err := deps.APIPrivacy.SaveOrganizationRetention(r.Context(), principal.OrganizationID, validated)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "saving retention settings failed")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	})
	r.Get("/api/v1/admin/cases", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIAdmin == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "admin API is not configured")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		cases, err := deps.APIAdmin.ListAdminCases(r.Context(), r.URL.Query().Get("status"), limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading admin cases failed")
			return
		}
		response := make([]apicontracts.AdminCaseResponse, 0, len(cases))
		for _, c := range cases {
			response = append(response, apicontracts.NewAdminCaseResponse(c))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.AdminCaseResponse{"cases": response})
	})
	r.Post("/api/v1/admin/cases", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIAdmin == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "admin API is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Kind     string `json:"kind"`
			Subject  string `json:"subject"`
			UserID   string `json:"user_id"`
			DomainID string `json:"domain_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		c, err := deps.APIAdmin.CreateAdminCase(r.Context(), admin.CaseKind(payload.Kind), payload.Subject, payload.UserID, payload.DomainID)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewAdminCaseResponse(c))
	})
	r.Post("/api/v1/admin/cases/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIAdmin == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "admin API is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			ActorUserID string `json:"actor_user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		c, err := deps.APIAdmin.ResolveAdminCase(r.Context(), chi.URLParam(r, "id"), payload.ActorUserID)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewAdminCaseResponse(c))
	})
	r.Get("/api/v1/admin/abuse-decisions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIAdmin == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "admin API is not configured")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		decisions, err := deps.APIAdmin.ListAbuseDecisions(r.Context(), r.URL.Query().Get("identifier"), limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading abuse decisions failed")
			return
		}
		response := make([]apicontracts.AbuseDecisionResponse, 0, len(decisions))
		for _, decision := range decisions {
			response = append(response, apicontracts.NewAbuseDecisionResponse(decision))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.AbuseDecisionResponse{"decisions": response})
	})
	r.Post("/api/v1/admin/abuse-decisions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIAdmin == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "admin API is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Identifier string `json:"identifier"`
			Action     string `json:"action"`
			Reason     string `json:"reason"`
			Duration   string `json:"duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		duration := time.Duration(0)
		if strings.TrimSpace(payload.Duration) != "" {
			parsed, err := time.ParseDuration(payload.Duration)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "duration is invalid")
				return
			}
			duration = parsed
		}
		decision, err := deps.APIAdmin.CreateAbuseDecision(r.Context(), payload.Identifier, admin.AbuseAction(payload.Action), payload.Reason, duration)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewAbuseDecisionResponse(decision))
	})
	r.Get("/api/v1/admin/operational-events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIOperations == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "operational event API is not configured")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		severity := strings.TrimSpace(r.URL.Query().Get("severity"))
		events, err := deps.APIOperations.ListOperationalEvents(r.Context(), severity, limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading operational events failed")
			return
		}
		response := make([]apicontracts.OperationalEventResponse, 0, len(events))
		for _, event := range events {
			response = append(response, apicontracts.NewOperationalEventResponse(event))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.OperationalEventResponse{"events": response})
	})
	r.Get("/api/v1/admin/provider-health", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		build := buildinfo.Current()
		response := apicontracts.ProviderHealthResponse{
			GeneratedAt:                  time.Now().UTC(),
			PublicBaseURLConfigured:      deps.PublicBaseURLConfigured,
			BunnyDNSConfigured:           deps.BunnyDNSConfigured,
			BunnyDNSZoneIDConfigured:     deps.BunnyDNSZoneIDConfigured,
			BootstrapConfigured:          deps.BootstrapConfigured,
			BootstrapPartiallyConfigured: deps.BootstrapPartiallyConfigured,
			BootstrapSkipped:             deps.BootstrapSkipped,
			SPFRouteSchedulerEnabled:     deps.SPFRouteSchedulerEnabled,
			Version:                      build.Version,
			Commit:                       build.Commit,
			BuildTime:                    build.BuildTime,
			Worker: apicontracts.ProviderWorkerHealthResponse{
				QueueName:      deps.WorkerQueueName,
				MaxWorkers:     deps.WorkerMaxWorkers,
				RegisteredJobs: append([]string(nil), deps.WorkerRegisteredJobs...),
			},
		}
		if deps.SPFRouteSchedulerInterval > 0 {
			response.SPFRouteSchedulerInterval = deps.SPFRouteSchedulerInterval.String()
		}
		if deps.SPFRouteScheduledSyncEvery > 0 {
			response.SPFRouteScheduledSyncCadence = deps.SPFRouteScheduledSyncEvery.String()
		}
		writeJSON(w, http.StatusOK, response)
	})
	r.Get("/api/v1/admin/spf-routes/{routeID}/publication-audit", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.AdminSPFRouteAudits == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "SPF route audit API is not configured")
			return
		}
		routeID := strings.TrimSpace(chi.URLParam(r, "routeID"))
		if routeID == "" {
			writeAPIError(w, http.StatusBadRequest, "route ID is required")
			return
		}
		route, err := deps.AdminSPFRouteAudits.GetSPFRoute(r.Context(), routeID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "SPF route not found")
			return
		}
		writeJSON(w, http.StatusOK, adminSPFRoutePublicationAuditResponse(r.Context(), route, deps.DNSPublicationResolver, deps.DNSPublicationTimeout))
	})
	r.Get("/api/v1/status/components", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIStatus == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "status API is not configured")
			return
		}
		components, err := deps.APIStatus.ListStatusComponents(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading status components failed")
			return
		}
		response := make([]apicontracts.StatusComponentResponse, 0, len(components))
		for _, component := range components {
			response = append(response, apicontracts.NewStatusComponentResponse(component))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.StatusComponentResponse{"components": response})
	})
	r.Post("/api/v1/status/components", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIStatus == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "status API is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		component, err := deps.APIStatus.UpsertStatusComponent(r.Context(), payload.Name, statuspage.ComponentStatus(payload.Status))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewStatusComponentResponse(component))
	})
	r.Get("/api/v1/status/incidents", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIStatus == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "status API is not configured")
			return
		}
		limit, err := apiLimit(r.URL.Query().Get("limit"), 50, 200)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		incidents, err := deps.APIStatus.ListStatusIncidents(r.Context(), limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "loading status incidents failed")
			return
		}
		response := make([]apicontracts.StatusIncidentResponse, 0, len(incidents))
		for _, incident := range incidents {
			response = append(response, apicontracts.NewStatusIncidentResponse(incident))
		}
		writeJSON(w, http.StatusOK, map[string][]apicontracts.StatusIncidentResponse{"incidents": response})
	})
	r.Post("/api/v1/status/incidents", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIStatus == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "status API is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Title   string `json:"title"`
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		incident, err := deps.APIStatus.CreateStatusIncident(r.Context(), payload.Title, payload.Status, payload.Message)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewStatusIncidentResponse(incident))
	})
	r.Post("/api/v1/status/incidents/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeAdminManage); !ok {
			return
		}
		if deps.APIStatus == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "status API is not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		incident, err := deps.APIStatus.ResolveStatusIncident(r.Context(), chi.URLParam(r, "id"), payload.Message)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewStatusIncidentResponse(incident))
	})
	r.Post("/api/v1/domains", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if deps.APIDomainCreator == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		domainName, err := scanner.NormalizeDomain(payload.Domain)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !allowOrganizationSavedDomains(w, r, deps, principal.OrganizationID, 1) {
			return
		}
		saved, err := deps.APIDomainCreator.CreateOrganizationDomain(r.Context(), principal.OrganizationID, domainName)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "saving domain failed")
			return
		}
		if deps.Scanner != nil && deps.APIDomainChecker != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			result, err := deps.Scanner.Scan(ctx, saved.Domain)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := deps.APIDomainChecker.CreateCheck(r.Context(), saved.ID, result); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "saving check failed")
				return
			}
			checkedAt := result.CheckedAt
			if checkedAt.IsZero() {
				checkedAt = time.Now().UTC()
			}
			saved.LatestCheck = &domains.Check{
				SPF:       result.SPF,
				DKIM:      result.DKIM,
				DMARC:     result.DMARC,
				BIMI:      result.BIMI,
				CheckedAt: checkedAt,
			}
			writeJSON(w, http.StatusCreated, apicontracts.NewDashboardDomainResponse(saved, nil))
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewDomainResponse(saved))
	})
	r.Post("/api/v1/domains/import", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if deps.APIDomainCreator == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		domainNames, err := domains.ParseCSV(r.Body)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !allowOrganizationSavedDomains(w, r, deps, principal.OrganizationID, len(domainNames)) {
			return
		}
		imported := make([]apicontracts.DomainResponse, 0, len(domainNames))
		for _, domainName := range domainNames {
			saved, err := deps.APIDomainCreator.CreateOrganizationDomain(r.Context(), principal.OrganizationID, domainName)
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "importing domains failed")
				return
			}
			imported = append(imported, apicontracts.NewDomainResponse(saved))
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"created": len(imported),
			"domains": imported,
		})
	})
	r.Post("/api/v1/domains/{id}/check", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, deps.APIOperations, apikeys.ScopeDomainWrite)
		if !ok {
			return
		}
		if deps.APIDomainChecker == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
			return
		}
		if deps.Scanner == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "scanner is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		saved, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), principal.OrganizationID, chi.URLParam(r, "id"))
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "domain was not found")
			return
		}
		result, err := deps.Scanner.Scan(r.Context(), saved.Domain)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := deps.APIDomainChecker.CreateCheck(r.Context(), saved.ID, result); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "saving check failed")
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewScanResponse(result))
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": brand.ProductName,
			"status":  "operational",
			"version": buildinfo.Current(),
			"components": []string{
				"api",
				"scanner",
				"report ingestion",
				"notifications",
			},
		})
	})
	r.Get("/security", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"contact": "Report security issues privately to the project owner before opening public issues.",
			"controls": []string{
				"API keys are stored as hashes only.",
				"DMARC report ingestion rejects unsafe archives and oversized payloads.",
				"Current subprocessors: Lettermint and operator-managed infrastructure.",
			},
		})
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, "browser routes are served by dmark-web")
	})
	return r
}

func configureSessionManager(sessionManager *scs.SessionManager, publicBaseURL string) {
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	parsed, err := url.Parse(publicBaseURL)
	sessionManager.Cookie.Secure = err == nil && strings.EqualFold(parsed.Scheme, "https")
}

func legacyHostRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if strings.Contains(host, ":") {
			if parsedHost, _, err := net.SplitHostPort(host); err == nil {
				host = parsedHost
			}
		}
		if strings.EqualFold(host, brand.LegacyDomain) {
			target := (&url.URL{
				Scheme:   "https",
				Host:     brand.PrimaryDomain,
				Path:     r.URL.EscapedPath(),
				RawQuery: r.URL.RawQuery,
			}).String()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func backendOnlySurface(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isFrontendRoute(r.URL.Path) {
			writeAPIError(w, http.StatusNotFound, "browser routes are served by dmark-web")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isFrontendRoute(path string) bool {
	switch path {
	case "/", "/login", "/logout", "/auth/callback", "/check", "/claim-audit":
		return true
	}
	return strings.HasPrefix(path, "/app/") || path == "/app" || strings.HasPrefix(path, "/assets/")
}

func authorizeAPI(w http.ResponseWriter, r *http.Request, authenticator APIKeyAuthenticator, operations OperationalEventStore, required apikeys.Scope) (apikeys.Principal, bool) {
	token, err := apikeys.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "valid bearer API key is required")
		return apikeys.Principal{}, false
	}
	if authenticator == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "API keys are not configured")
		return apikeys.Principal{}, false
	}
	principal, err := authenticator.AuthenticateAPIKey(r.Context(), token)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "invalid API key")
		return apikeys.Principal{}, false
	}
	if !apikeys.Allows(principal, required) {
		writeAPIError(w, http.StatusForbidden, "API key does not have the required scope")
		return apikeys.Principal{}, false
	}
	if required == apikeys.ScopeAdminManage && !principal.GlobalAdmin {
		writeAPIError(w, http.StatusForbidden, "global admin principal is required")
		return apikeys.Principal{}, false
	}
	recordAPIKeyUsedOperationalEvent(r, operations, principal, required)
	return principal, true
}

func recordAPIKeyUsedOperationalEvent(r *http.Request, store OperationalEventStore, principal apikeys.Principal, required apikeys.Scope) {
	if store == nil {
		return
	}
	actor := "api-key"
	if strings.TrimSpace(principal.OrganizationID) != "" {
		actor = "organization:" + strings.TrimSpace(principal.OrganizationID)
	} else if principal.GlobalAdmin {
		actor = "global-admin"
	}
	event, err := ops.NewEvent("api_key.used", ops.SeverityInfo, "API key used", time.Now().UTC())
	if err != nil {
		return
	}
	event.Actor = actor
	path := apiOperationalEventPath(r)
	event.Resource = "api:" + path
	event.Metadata = ops.SanitizeMetadata(map[string]any{
		"organization_id": strings.TrimSpace(principal.OrganizationID),
		"scope":           string(required),
		"method":          r.Method,
		"path":            path,
		"request_id":      RequestID(r.Context()),
		"global_admin":    principal.GlobalAdmin,
	})
	_, _ = store.CreateOperationalEvent(r.Context(), event)
}

func apiOperationalEventPath(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := strings.TrimSpace(routeContext.RoutePattern()); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// allowOrganizationSavedDomains enforces the shared saved-domain limit before
// creating additional domains. It fails open when the domain count store is not
// configured so older deployments keep working.
func allowOrganizationSavedDomains(w http.ResponseWriter, r *http.Request, deps Deps, organizationID string, additional int) bool {
	if deps.APIPlan == nil || additional <= 0 {
		return true
	}
	count, err := deps.APIPlan.CountOrganizationDomains(r.Context(), organizationID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "counting organization domains failed")
		return false
	}
	limits := entitlements.DefaultLimits()
	if !limits.AllowsSavedDomain(count + additional - 1) {
		writeAPIError(w, http.StatusForbidden, fmt.Sprintf("resource limit reached: %d of %d saved domains are in use", count, limits.SavedDomains))
		return false
	}
	return true
}

func dashboardTrendDays(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 30, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("days must be a positive integer")
	}
	if value < 7 {
		return 7, nil
	}
	if value > 90 {
		return 90, nil
	}
	return value, nil
}

func apiLimit(raw string, fallback int, max int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if value > max {
		return max, nil
	}
	return value, nil
}

func requireOrganizationAPI(w http.ResponseWriter, principal apikeys.Principal) bool {
	if strings.TrimSpace(principal.OrganizationID) == "" {
		writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
		return false
	}
	return true
}

func adminSPFRoutePublicationAuditResponse(ctx context.Context, route spfroutes.Route, resolver DNSPublicationResolver, lookupTimeout time.Duration) apicontracts.AdminSPFRoutePublicationAuditResponse {
	expectedTXTValue := strings.TrimSpace(route.LastPublishedSPF)
	if expectedTXTValue == "" {
		expectedTXTValue = strings.TrimSpace(route.LastGeneratedSPF)
	}
	response := apicontracts.AdminSPFRoutePublicationAuditResponse{
		RouteID:          route.ID,
		DomainID:         route.DomainID,
		Domain:           route.Domain,
		BunnyRecordID:    route.BunnyRecordID,
		LastGeneratedSPF: route.LastGeneratedSPF,
		LastPublishedSPF: route.LastPublishedSPF,
		LastKnownGoodSPF: route.LastPublishedSPF,
		LastSyncStatus:   string(route.LastSyncStatus),
		LastSyncError:    adminSafeSPFRouteSyncError(route.LastSyncError),
		Expected: apicontracts.DNSPublicationExpected{
			TXT:   apicontracts.DNSExpectedTXT{Name: route.RouteFQDN, Value: expectedTXTValue},
			CNAME: apicontracts.DNSExpectedCNAME{Name: route.CustomerCNAMEName, Target: route.CustomerCNAMETarget},
		},
		Recursive: apicontracts.DNSPublicationLive{
			TXT:   apicontracts.DNSPublicationRecord{Values: []string{}, Status: "not_configured"},
			CNAME: apicontracts.DNSPublicationRecord{Values: []string{}, Status: "not_configured"},
		},
		Authoritative:     apicontracts.DNSPublicationStatus{Status: "not_configured"},
		PropagationStatus: "unknown",
	}
	if !route.LastSyncedAt.IsZero() {
		lastSyncedAt := route.LastSyncedAt.UTC()
		response.LastSyncedAt = &lastSyncedAt
	}
	if resolver != nil {
		lookupCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
		defer cancel()
		response.Recursive.TXT = lookupPublicationTXT(lookupCtx, resolver, route.RouteFQDN, expectedTXTValue)
		response.Recursive.CNAME = lookupPublicationCNAME(lookupCtx, resolver, route.CustomerCNAMEName, route.CustomerCNAMETarget)
	}
	response.PropagationStatus = propagationStatus(response.Recursive.TXT.Status, response.Recursive.CNAME.Status)
	return response
}

func lookupPublicationTXT(ctx context.Context, resolver DNSPublicationResolver, name, expected string) apicontracts.DNSPublicationRecord {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(expected) == "" {
		return apicontracts.DNSPublicationRecord{Values: []string{}, Status: "not_configured"}
	}
	values, err := resolver.LookupTXT(ctx, name)
	if err != nil {
		return apicontracts.DNSPublicationRecord{Values: []string{}, Status: "missing", Error: "dns_lookup_failed"}
	}
	status := "drift"
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(expected) {
			status = "match"
			break
		}
	}
	return apicontracts.DNSPublicationRecord{Values: values, Status: status}
}

func lookupPublicationCNAME(ctx context.Context, resolver DNSPublicationResolver, name, expected string) apicontracts.DNSPublicationRecord {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(expected) == "" {
		return apicontracts.DNSPublicationRecord{Values: []string{}, Status: "not_configured"}
	}
	value, err := resolver.LookupCNAME(ctx, name)
	if err != nil {
		return apicontracts.DNSPublicationRecord{Values: []string{}, Status: "missing", Error: "dns_lookup_failed"}
	}
	status := "drift"
	if normalizeDNSName(value) == normalizeDNSName(expected) {
		status = "match"
	}
	return apicontracts.DNSPublicationRecord{Values: []string{value}, Status: status}
}

func propagationStatus(statuses ...string) string {
	matched := false
	for _, status := range statuses {
		switch status {
		case "drift":
			return "drift"
		case "missing":
			return "propagating"
		case "match":
			matched = true
		}
	}
	if matched {
		return "match"
	}
	return "unknown"
}

func normalizeDNSName(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}

func adminSafeSPFRouteSyncError(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "SPF route sync failed; see operational events for operator details"
}

func dashboardDomainResponses(ctx context.Context, deps Deps, organizationID string, includeReporter bool) ([]apicontracts.DashboardDomainResponse, error) {
	if deps.APIDomains == nil {
		return nil, fmt.Errorf("domain API is not configured")
	}
	saved, err := deps.APIDomains.ListOrganizationDomains(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("loading dashboard domains failed")
	}
	response := make([]apicontracts.DashboardDomainResponse, 0, len(saved))
	for _, domain := range saved {
		var senders []reporter.InventoryEntry
		if includeReporter {
			senders = dashboardReporterSenders(ctx, deps.APIReporter, organizationID, domain.ID, 200)
		}
		response = append(response, apicontracts.NewDashboardDomainResponse(domain, senders))
	}
	return response, nil
}

func dashboardDNSGuidanceResponse(ctx context.Context, deps Deps, organizationID string, domain domains.Domain, generatedAt time.Time) apicontracts.DashboardDNSGuidanceResponse {
	detection, resolverError := dashboardDNSProviderDetection(ctx, deps, domain.Domain)
	records := dashboardDNSGuidanceRecords(domain)
	if deps.APISPFRoutes != nil {
		if route, err := deps.APISPFRoutes.GetOrganizationSPFRoute(ctx, organizationID, domain.ID); err == nil {
			if name := strings.TrimSpace(route.CustomerCNAMEName); name != "" {
				if target := strings.Trim(strings.TrimSpace(route.CustomerCNAMETarget), "."); target != "" {
					records = append(records, dnsprovider.RecordInstructionInput{Kind: dnsprovider.RecordSPFRouteCNAME, Name: name, Value: target, TTL: 300})
				}
			}
		}
	}
	return apicontracts.NewDashboardDNSGuidanceResponse(domain, detection, resolverError, records, generatedAt)
}

func dashboardDNSProviderDetection(ctx context.Context, deps Deps, domainName string) (dnsprovider.Detection, string) {
	lookupCtx, cancel := context.WithTimeout(ctx, deps.DNSNameserverTimeout)
	defer cancel()
	nsRecords, err := deps.DNSNameserverResolver.LookupNS(lookupCtx, domainName)
	if err != nil {
		return dnsprovider.Detection{Status: dnsprovider.StatusUnknown, Confidence: "low", EvidenceNameservers: []string{}}, "authoritative nameserver lookup failed"
	}
	nameservers := make([]string, 0, len(nsRecords))
	for _, ns := range nsRecords {
		if ns == nil {
			continue
		}
		nameservers = append(nameservers, ns.Host)
	}
	return dnsprovider.Detect(nameservers), ""
}

func dashboardDNSGuidanceRecords(domain domains.Domain) []dnsprovider.RecordInstructionInput {
	dmarcValue := "v=DMARC1; p=none; rua=mailto:dmarc@" + domain.Domain
	spfValue := "v=spf1 ~all"
	if domain.LatestCheck != nil {
		if record := strings.TrimSpace(domain.LatestCheck.DMARC.Record); record != "" {
			dmarcValue = record
		}
		if record := strings.TrimSpace(domain.LatestCheck.SPF.Record); record != "" {
			spfValue = record
		}
	}
	return []dnsprovider.RecordInstructionInput{
		{Kind: dnsprovider.RecordDMARCTXT, Name: "_dmarc." + domain.Domain, Value: dmarcValue, TTL: 300},
		{Kind: dnsprovider.RecordSPFTXT, Name: domain.Domain, Value: spfValue, TTL: 300},
	}
}

func dashboardDomainForAPI(w http.ResponseWriter, r *http.Request, deps Deps, organizationID string) (domains.Domain, bool) {
	if deps.APIDomainChecker == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "domain API is not configured")
		return domains.Domain{}, false
	}
	domain, err := deps.APIDomainChecker.GetOrganizationDomain(r.Context(), organizationID, chi.URLParam(r, "id"))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "domain was not found")
		return domains.Domain{}, false
	}
	return domain, true
}

func dashboardSPFRouteFromRequest(r *http.Request, organizationID string, domain domains.Domain) (spfroutes.Route, error) {
	var payload struct {
		RouteFQDN           string `json:"route_fqdn"`
		CustomerCNAMEName   string `json:"customer_cname_name"`
		CustomerCNAMETarget string `json:"customer_cname_target"`
		Enabled             bool   `json:"enabled"`
		MechanismPolicy     string `json:"mechanism_policy"`
		Sources             []struct {
			Kind    string `json:"kind"`
			Value   string `json:"value"`
			Label   string `json:"label"`
			Enabled bool   `json:"enabled"`
		} `json:"sources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return spfroutes.Route{}, fmt.Errorf("invalid JSON body")
	}
	route := spfroutes.Route{
		OrganizationID:      organizationID,
		DomainID:            domain.ID,
		Domain:              domain.Domain,
		RouteFQDN:           strings.TrimSpace(payload.RouteFQDN),
		CustomerCNAMEName:   strings.TrimSpace(payload.CustomerCNAMEName),
		CustomerCNAMETarget: strings.TrimSpace(payload.CustomerCNAMETarget),
		Enabled:             payload.Enabled,
		MechanismPolicy:     spfroutes.Policy(strings.TrimSpace(payload.MechanismPolicy)),
		Sources:             make([]spfroutes.Source, 0, len(payload.Sources)),
	}
	if route.RouteFQDN == "" {
		return spfroutes.Route{}, fmt.Errorf("route_fqdn is required")
	}
	route.RouteLabel = dashboardSPFRouteLabel(route.RouteFQDN)
	if route.RouteLabel == "" {
		return spfroutes.Route{}, fmt.Errorf("route_fqdn must contain a valid route label")
	}
	if route.CustomerCNAMEName == "" {
		return spfroutes.Route{}, fmt.Errorf("customer_cname_name is required")
	}
	if route.CustomerCNAMETarget == "" {
		return spfroutes.Route{}, fmt.Errorf("customer_cname_target is required")
	}
	if route.MechanismPolicy == "" {
		route.MechanismPolicy = spfroutes.PolicySoftfail
	}
	if route.MechanismPolicy != spfroutes.PolicySoftfail && route.MechanismPolicy != spfroutes.PolicyFail && route.MechanismPolicy != spfroutes.PolicyNeutral {
		return spfroutes.Route{}, fmt.Errorf("mechanism_policy must be softfail, fail, or neutral")
	}
	for _, source := range payload.Sources {
		kind := spfroutes.SourceKind(strings.TrimSpace(source.Kind))
		if kind != spfroutes.SourceKindInclude && kind != spfroutes.SourceKindIP4 && kind != spfroutes.SourceKindIP6 {
			return spfroutes.Route{}, fmt.Errorf("source kind must be include, ip4, or ip6")
		}
		value := strings.TrimSpace(source.Value)
		if value == "" {
			return spfroutes.Route{}, fmt.Errorf("source value is required")
		}
		route.Sources = append(route.Sources, spfroutes.Source{Kind: kind, Value: value, Label: strings.TrimSpace(source.Label), Enabled: source.Enabled})
	}
	return route, nil
}

func dashboardSPFRouteLabel(routeFQDN string) string {
	routeFQDN = strings.ToLower(strings.Trim(strings.TrimSpace(routeFQDN), "."))
	label, _, _ := strings.Cut(routeFQDN, ".")
	if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return ""
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return ""
	}
	return label
}

func dashboardReporterSenders(ctx context.Context, store OrganizationReporterStore, organizationID string, domainID string, limit int) []reporter.InventoryEntry {
	if store == nil {
		return nil
	}
	senders, err := store.ListOrganizationReporterSenders(ctx, organizationID, domainID, limit)
	if err != nil {
		return nil
	}
	return senders
}

func dashboardDMARCEvidence(ctx context.Context, store OrganizationReporterStore, organizationID string, domainID string) dmarcjourney.Evidence {
	evidence := dmarcjourney.Evidence{}
	senders := dashboardReporterSenders(ctx, store, organizationID, domainID, 500)
	mergeSenderEvidence(&evidence, senders)
	if store != nil {
		if health, err := store.GetOrganizationReporterHealth(ctx, organizationID, domainID); err == nil {
			mergeReporterHealth(&evidence, health)
		}
	}
	return evidence
}

func dashboardLookalikeOptions(r *http.Request) (brandrisk.CandidateOptions, error) {
	limit, err := apiLimit(r.URL.Query().Get("limit"), 25, 100)
	if err != nil {
		return brandrisk.CandidateOptions{}, err
	}
	tld := strings.TrimSpace(r.URL.Query().Get("tld"))
	if tld == "" {
		tld = "is"
	}
	return brandrisk.CandidateOptions{
		TLD:             tld,
		MaxCount:        limit,
		IncludeInactive: strings.EqualFold(r.URL.Query().Get("include_inactive"), "true"),
	}, nil
}

func dashboardReason(r *http.Request) (string, error) {
	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("invalid JSON body")
	}
	if len(payload.Reason) > 500 {
		return "", fmt.Errorf("reason is too long")
	}
	return strings.TrimSpace(payload.Reason), nil
}

func dashboardMuteRequest(r *http.Request) (string, time.Time, error) {
	var payload struct {
		Reason   string    `json:"reason"`
		Until    time.Time `json:"until"`
		Duration string    `json:"duration"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return "", time.Time{}, fmt.Errorf("invalid JSON body")
	}
	if len(payload.Reason) > 500 {
		return "", time.Time{}, fmt.Errorf("reason is too long")
	}
	until := payload.Until
	if until.IsZero() {
		until = muteUntil(payload.Duration, time.Now().UTC())
	}
	if !until.After(time.Now().UTC()) {
		return "", time.Time{}, fmt.Errorf("mute expiration must be in the future")
	}
	return strings.TrimSpace(payload.Reason), until.UTC(), nil
}

func writeDashboardReporterCSV(w http.ResponseWriter, response apicontracts.DashboardReporterResponse) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="dmark-reporter-senders.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"source_ip", "provider", "volume", "aligned_percentage", "classification", "trend", "risk", "first_seen", "last_seen"})
	for _, sender := range response.SenderInventory {
		_ = writer.Write([]string{
			sender.SourceIP,
			sender.Provider,
			strconv.Itoa(sender.Volume),
			fmt.Sprintf("%.2f", sender.AlignedPercentage),
			sender.Classification,
			sender.Trend,
			sender.Risk,
			sender.FirstSeen.UTC().Format(time.RFC3339),
			sender.LastSeen.UTC().Format(time.RFC3339),
		})
	}
	writer.Flush()
}

func mergeReporterHealth(evidence *dmarcjourney.Evidence, health reporter.HealthResult) {
	if !health.LastReportAt.IsZero() || len(health.ReporterOrgs) > 0 {
		evidence.ReporterConfigured = true
	}
}

func mergeSenderEvidence(evidence *dmarcjourney.Evidence, senders []reporter.InventoryEntry) {
	var firstSeen time.Time
	var lastSeen time.Time
	for _, sender := range senders {
		evidence.TotalMessages += sender.Volume
		evidence.FailedMessages += int(float64(sender.Volume) * sender.FailureRate)
		if reporter.InventoryRisk(reporter.ToSenderInventoryItem(sender)) == "review" && sender.Provider == "" {
			evidence.UnknownSenderMessages += sender.Volume
		}
		if !sender.FirstSeen.IsZero() && (firstSeen.IsZero() || sender.FirstSeen.Before(firstSeen)) {
			firstSeen = sender.FirstSeen
		}
		if sender.LastSeen.After(lastSeen) {
			lastSeen = sender.LastSeen
		}
	}
	evidence.AlignedMessages = evidence.TotalMessages - evidence.FailedMessages
	if evidence.AlignedMessages < 0 {
		evidence.AlignedMessages = 0
	}
	if !firstSeen.IsZero() && !lastSeen.IsZero() {
		evidence.DaysObserved = int(lastSeen.Sub(firstSeen).Hours()/24) + 1
		if evidence.DaysObserved < 1 {
			evidence.DaysObserved = 1
		}
	}
}

func muteUntil(duration string, now time.Time) time.Time {
	switch strings.TrimSpace(strings.ToLower(duration)) {
	case "1h", "hour":
		return now.UTC().Add(time.Hour)
	case "7d", "week":
		return now.UTC().Add(7 * 24 * time.Hour)
	case "30d", "month":
		return now.UTC().Add(30 * 24 * time.Hour)
	default:
		return now.UTC().Add(24 * time.Hour)
	}
}

type requestIDContextKey struct{}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !isSafeRequestID(requestID) {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isSafeRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			continue
		}
		switch ch {
		case '-', '_', '.', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func newRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return "req-" + hex.EncodeToString(raw[:])
}
