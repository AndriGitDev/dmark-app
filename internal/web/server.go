// Package web wires HTTP routes and middleware.
package web

import (
	"context"
	"crypto/rand"
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
	"github.com/AndriGitDev/meil/internal/billing"
	"github.com/AndriGitDev/meil/internal/brand"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/entitlements"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/privacy"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/statuspage"
	"github.com/AndriGitDev/meil/internal/teams"
	"github.com/AndriGitDev/meil/internal/webhooks"
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

type OrganizationDomainHistoryStore interface {
	ListOrganizationDomainHistory(ctx context.Context, organizationID string, domainID string, limit int) ([]domains.HistoryEntry, error)
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

type AuditClaimStore interface {
	CreateAuditClaim(ctx context.Context, claim salesaudit.Claim) (salesaudit.Claim, error)
}

type OrganizationWebhookStore interface {
	ListOrganizationWebhookEndpoints(ctx context.Context, organizationID string) ([]webhooks.Endpoint, error)
	CreateOrganizationWebhookEndpoint(ctx context.Context, organizationID string, rawURL string, eventTypes []string) (webhooks.Endpoint, string, error)
	RotateOrganizationWebhookSecret(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, string, error)
	DisableOrganizationWebhookEndpoint(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, error)
}

type BillingWebhookStore interface {
	ApplyBillingWebhook(ctx context.Context, event billing.WebhookEvent) (bool, error)
}

type BillingCancellationStore interface {
	RequestBillingCancellation(ctx context.Context, organizationID string, reason string, periodEndsAt time.Time) (billing.Cancellation, error)
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
	Scanner                EmailScanner
	APIKeys                APIKeyAuthenticator
	APIDomains             OrganizationDomainLister
	APIDomainCreator       OrganizationDomainCreator
	APIDomainChecker       OrganizationDomainChecker
	APIDomainHistory       OrganizationDomainHistoryStore
	APIDomainDrift         OrganizationDomainDriftStore
	APIReporter            OrganizationReporterStore
	UserReporter           UserReporterStore
	AuditClaims            AuditClaimStore
	APIWebhooks            OrganizationWebhookStore
	Billing                BillingWebhookStore
	BillingCancellation    BillingCancellationStore
	BillingWebhookSecret   string
	APIKeyManagement       OrganizationAPIKeyStore
	APITeams               OrganizationTeamStore
	APIPrivacy             OrganizationPrivacyStore
	APIAdmin               AdminCaseStore
	PrivacyDeletion        PrivacyDeletionStore
	APIStatus              StatusAdminStore
	Auth                   *auth.Service
	Domains                *domains.Service
	Alerts                 alerts.Store
	Sender                 notifier.EmailSender
	SessionManager         *scs.SessionManager
	PublicBaseURL          string
	FrontendOrigins        []string
	PublicChecksPerMinute  int
	LoginAttemptsPerMinute int
}

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

	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(legacyHostRedirect)
	r.Use(corsMiddleware(deps.FrontendOrigins))
	r.Use(deps.SessionManager.LoadAndSave)
	r.Use(backendOnlySurface)
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
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
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
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
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
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
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
		result, err := deps.Scanner.Scan(r.Context(), domain)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		audit := salesaudit.Assess(result, salesaudit.Options{RequestedEmail: email})
		if audit.BlockedReason != "" {
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
	r.Post("/billing/webhook", func(w http.ResponseWriter, r *http.Request) {
		if deps.Billing == nil || strings.TrimSpace(deps.BillingWebhookSecret) == "" {
			writeAPIError(w, http.StatusServiceUnavailable, "billing webhooks are not configured")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid webhook body")
			return
		}
		signature := r.Header.Get("X-Billing-Signature")
		if signature == "" {
			signature = r.Header.Get("X-Meil-Billing-Signature")
		}
		if err := billing.VerifyWebhook(deps.BillingWebhookSecret, body, signature); err != nil {
			writeAPIError(w, http.StatusUnauthorized, "invalid billing webhook signature")
			return
		}
		var event billing.WebhookEvent
		if err := json.Unmarshal(body, &event); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid billing webhook JSON")
			return
		}
		if event.OccurredAt.IsZero() {
			event.OccurredAt = time.Now().UTC()
		}
		applied, err := deps.Billing.ApplyBillingWebhook(r.Context(), event)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "applying billing webhook failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"applied": applied})
	})
	r.Get("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeScanRead); !ok {
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
	r.Get("/api/v1/domains", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeReporterRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeReporterRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeReporterRead)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeWebhookManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeWebhookManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeWebhookManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeWebhookManage)
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
	r.Post("/api/v1/billing/cancel", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeBillingManage)
		if !ok {
			return
		}
		if deps.BillingCancellation == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "billing API is not configured")
			return
		}
		if strings.TrimSpace(principal.OrganizationID) == "" {
			writeAPIError(w, http.StatusForbidden, "API key is not attached to an organization")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var payload struct {
			Reason       string    `json:"reason"`
			PeriodEndsAt time.Time `json:"period_ends_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		cancellation, err := deps.BillingCancellation.RequestBillingCancellation(r.Context(), principal.OrganizationID, payload.Reason, payload.PeriodEndsAt)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apicontracts.NewBillingCancellationResponse(cancellation))
	})
	r.Get("/api/v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAPIKeyManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAPIKeyManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAPIKeyManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeTeamManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeTeamManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeTeamManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopePrivacyManage)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopePrivacyManage)
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
	r.Get("/api/v1/status/components", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		if _, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeAdminManage); !ok {
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainWrite)
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
		saved, err := deps.APIDomainCreator.CreateOrganizationDomain(r.Context(), principal.OrganizationID, domainName)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "saving domain failed")
			return
		}
		writeJSON(w, http.StatusCreated, apicontracts.NewDomainResponse(saved))
	})
	r.Post("/api/v1/domains/import", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainWrite)
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
		principal, ok := authorizeAPI(w, r, deps.APIKeys, apikeys.ScopeDomainWrite)
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

func authorizeAPI(w http.ResponseWriter, r *http.Request, authenticator APIKeyAuthenticator, required apikeys.Scope) (apikeys.Principal, bool) {
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
	return principal, true
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
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

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
