// Package web wires HTTP routes and middleware.
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AndriGitDev/meil/internal/admin"
	"github.com/AndriGitDev/meil/internal/alerts"
	apicontracts "github.com/AndriGitDev/meil/internal/api"
	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/billing"
	"github.com/AndriGitDev/meil/internal/brand"
	"github.com/AndriGitDev/meil/internal/brandrisk"
	"github.com/AndriGitDev/meil/internal/dmarcjourney"
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
	"github.com/AndriGitDev/meil/internal/trustreport"
	"github.com/AndriGitDev/meil/internal/web/views"
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
	r.Use(deps.SessionManager.LoadAndSave)
	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}
	r.Get("/assets/css/marketing-20260512.css", func(w http.ResponseWriter, r *http.Request) {
		// #nosec G703 -- assetsDir is an operator-controlled directory, and the served file path is fixed.
		http.ServeFile(w, r, filepath.Join(assetsDir, "css", "output.css"))
	})
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(views.Login(csrfToken(deps.SessionManager, r))).ServeHTTP(w, r)
	})
	loginLimiter := newRateLimiter(deps.LoginAttemptsPerMinute, time.Minute)
	r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager), loginLimiter.middleware).Post("/login", func(w http.ResponseWriter, r *http.Request) {
		if deps.Auth == nil || deps.Sender == nil {
			renderError(w, http.StatusServiceUnavailable, "Login unavailable", "Email login is not configured for this environment.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			renderError(w, http.StatusBadRequest, "Invalid form", "Check the submitted fields and try again.")
			return
		}
		email, err := startLoginAndSendEmail(r.Context(), deps, r.PostForm.Get("email"), "Sign in to "+brand.FullProductName, "Sign in to "+brand.FullProductName)
		if err != nil {
			if strings.Contains(err.Error(), "invalid email address") {
				renderError(w, http.StatusBadRequest, "Login failed", err.Error())
				return
			}
			renderError(w, http.StatusBadGateway, "Email delivery failed", "The login email could not be sent. Try again shortly.")
			return
		}
		templ.Handler(views.LoginSent(email)).ServeHTTP(w, r)
	})
	r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager), loginLimiter.middleware).Post("/claim-audit", func(w http.ResponseWriter, r *http.Request) {
		if deps.Auth == nil || deps.Sender == nil {
			renderError(w, http.StatusServiceUnavailable, "Audit claim unavailable", "Email login is not configured for this environment.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			renderError(w, http.StatusBadRequest, "Invalid form", "Check the submitted fields and try again.")
			return
		}
		domainName, err := scanner.NormalizeDomain(r.PostForm.Get("domain"))
		if err != nil {
			renderError(w, http.StatusBadRequest, "Audit claim failed", err.Error())
			return
		}
		email, err := auth.NormalizeEmail(r.PostForm.Get("email"))
		if err != nil {
			renderError(w, http.StatusBadRequest, "Audit claim failed", err.Error())
			return
		}
		audit := salesaudit.Assess(scanner.Result{Domain: domainName}, salesaudit.Options{RequestedEmail: email})
		if audit.BlockedReason != "" {
			renderError(w, http.StatusBadRequest, "Audit claim blocked", audit.BlockedReason)
			return
		}
		sentEmail, err := startLoginAndSendEmail(r.Context(), deps, email, "Claim your "+brand.FullProductName+" audit", "Claim the "+brand.FullProductName+" audit for "+domainName)
		if err != nil {
			renderError(w, http.StatusBadGateway, "Email delivery failed", "The audit claim email could not be sent. Try again shortly.")
			return
		}
		deps.SessionManager.Put(r.Context(), claimDomainSessionKey, domainName)
		templ.Handler(views.ClaimSent(sentEmail, domainName)).ServeHTTP(w, r)
	})
	r.Get("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if deps.Auth == nil {
			renderError(w, http.StatusServiceUnavailable, "Login unavailable", "Email login is not configured for this environment.")
			return
		}
		login, err := deps.Auth.FinishLogin(r.Context(), r.URL.Query().Get("token"))
		if err != nil {
			if errors.Is(err, auth.ErrExpiredToken) {
				w.WriteHeader(http.StatusBadRequest)
				templ.Handler(views.LoginProblem("Login link expired", "Request a new magic link to continue.")).ServeHTTP(w, r)
				return
			}
			if errors.Is(err, auth.ErrConsumedToken) {
				w.WriteHeader(http.StatusBadRequest)
				templ.Handler(views.LoginProblem("Login link already used", "Magic links can only be used once. Request a new link if you need to sign in again.")).ServeHTTP(w, r)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			templ.Handler(views.LoginProblem("Invalid login link", "Request a new magic link to continue.")).ServeHTTP(w, r)
			return
		}
		if err := deps.SessionManager.RenewToken(r.Context()); err != nil {
			renderError(w, http.StatusInternalServerError, "Session error", "The session could not be renewed. Try signing in again.")
			return
		}
		deps.SessionManager.Put(r.Context(), "user_id", login.UserID)
		deps.SessionManager.Put(r.Context(), "user_email", login.Email)
		if claimDomain := popClaimDomain(deps.SessionManager, r); claimDomain != "" {
			if deps.Domains != nil {
				if err := deps.Domains.AddDomain(r.Context(), login.UserID, claimDomain); err != nil {
					pushFlash(deps.SessionManager, r, ErrorFlash("Signed in, but the claimed domain could not be protected. Add it manually from this page."))
				} else {
					pushFlash(deps.SessionManager, r, DomainAddedFlash(claimDomain))
				}
			} else {
				pushFlash(deps.SessionManager, r, NewFlash(FlashInfo, "Signed in. Add "+claimDomain+" to protect it."))
			}
		}
		http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
	})
	r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		_ = deps.SessionManager.Destroy(r.Context())
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		if currentUserID(deps.SessionManager, r) == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		writeLogoutConfirmationPage(w, csrfToken(deps.SessionManager, r))
	})
	checkLimiter := newRateLimiter(deps.PublicChecksPerMinute, time.Minute)
	r.With(sameOriginPost(deps.PublicBaseURL), checkLimiter.middleware).Post("/check", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			renderError(w, http.StatusBadRequest, "Invalid form", "Check the submitted domain and try again.")
			return
		}
		if len(r.PostForm.Get("domain")) > 253 {
			renderError(w, http.StatusBadRequest, "Domain is too long", "Domain names must be 253 characters or fewer.")
			return
		}
		result, err := deps.Scanner.Scan(r.Context(), r.PostForm.Get("domain"))
		if err != nil {
			renderError(w, http.StatusBadRequest, "Unable to complete check", err.Error())
			return
		}
		audit := salesaudit.Assess(result, salesaudit.Options{})
		templ.Handler(views.CheckResult(result, audit, csrfToken(deps.SessionManager, r))).ServeHTTP(w, r)
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
		w.Header().Set("Content-Disposition", `attachment; filename="recordmaster-protected-domains.csv"`)
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
	r.Route("/app", func(r chi.Router) {
		r.Use(requireLogin(deps.SessionManager))
		r.Get("/domains", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				renderError(w, http.StatusServiceUnavailable, "Domains unavailable", "Protected domains are not configured for this environment.")
				return
			}
			saved, err := deps.Domains.ListDomains(r.Context(), currentUserID(deps.SessionManager, r))
			if err != nil {
				renderError(w, http.StatusInternalServerError, "Unable to load domains", "Protected domains could not be loaded. Try again shortly.")
				return
			}
			templ.Handler(views.AppDomains(currentUserEmail(deps.SessionManager, r), saved, csrfToken(deps.SessionManager, r), popFlashViews(deps.SessionManager, r))).ServeHTTP(w, r)
		})
		r.Get("/defender", func(w http.ResponseWriter, r *http.Request) {
			writeStaticPage(w, "Brand Risk", "Select a protected domain to review DNS and email-auth evidence for current brand-risk findings.", []string{
				"Domain trust findings from SPF, DKIM, DMARC, and BIMI.",
				"Evidence and recommended action on every finding.",
				"Lookalike-domain monitoring remains planned until the Defender feed is implemented.",
			})
		})
		r.Get("/domains/{id}/trust-report", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				renderError(w, http.StatusServiceUnavailable, "Domains unavailable", "Protected domains are not configured for this environment.")
				return
			}
			saved, err := deps.Domains.GetDomain(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id"))
			if err != nil {
				renderError(w, http.StatusNotFound, "Domain not found", "The protected domain could not be found for this account.")
				return
			}
			report := buildTrustReport(r.Context(), currentUserEmail(deps.SessionManager, r), currentUserID(deps.SessionManager, r), saved, deps.UserReporter)
			templ.Handler(views.TrustReport(currentUserEmail(deps.SessionManager, r), saved, report, csrfToken(deps.SessionManager, r))).ServeHTTP(w, r)
		})
		r.Get("/domains/{id}/brand-risk", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				renderError(w, http.StatusServiceUnavailable, "Domains unavailable", "Protected domains are not configured for this environment.")
				return
			}
			saved, err := deps.Domains.GetDomain(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id"))
			if err != nil {
				renderError(w, http.StatusNotFound, "Domain not found", "The protected domain could not be found for this account.")
				return
			}
			findings := []brandrisk.Finding{}
			if saved.LatestCheck != nil {
				findings = brandrisk.FromScan(scanner.Result{
					Domain:    saved.Domain,
					CheckedAt: saved.LatestCheck.CheckedAt,
					SPF:       saved.LatestCheck.SPF,
					DKIM:      saved.LatestCheck.DKIM,
					DMARC:     saved.LatestCheck.DMARC,
					BIMI:      saved.LatestCheck.BIMI,
				})
			}
			templ.Handler(views.BrandRisk(currentUserEmail(deps.SessionManager, r), saved, findings, csrfToken(deps.SessionManager, r))).ServeHTTP(w, r)
		})
		r.Get("/domains/export", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				renderError(w, http.StatusServiceUnavailable, "Domains unavailable", "Protected domains are not configured for this environment.")
				return
			}
			var export strings.Builder
			if err := deps.Domains.ExportCSV(r.Context(), currentUserID(deps.SessionManager, r), &export); err != nil {
				renderError(w, http.StatusInternalServerError, "Export failed", "Protected domains could not be exported. Try again shortly.")
				return
			}
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="recordmaster-protected-domains.csv"`)
			_, _ = io.WriteString(w, export.String())
		})
		r.Get("/privacy/export", func(w http.ResponseWriter, r *http.Request) {
			includeRawXML := r.URL.Query().Get("raw_xml") == "1" || strings.EqualFold(r.URL.Query().Get("raw_xml"), "true")
			writeJSON(w, http.StatusOK, privacy.NewExportManifest(
				currentUserID(deps.SessionManager, r),
				currentUserEmail(deps.SessionManager, r),
				includeRawXML,
			))
		})
		r.Get("/privacy/deletion-plan", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, privacy.NewDeletionPlan(currentUserID(deps.SessionManager, r)))
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/privacy/delete", func(w http.ResponseWriter, r *http.Request) {
			if deps.PrivacyDeletion == nil {
				renderError(w, http.StatusServiceUnavailable, "Account deletion unavailable", "Account deletion is not configured for this environment.")
				return
			}
			if !strings.EqualFold(strings.TrimSpace(r.PostForm.Get("confirm")), "delete") {
				renderError(w, http.StatusBadRequest, "Confirmation required", "Type DELETE to confirm account deletion.")
				return
			}
			if _, err := deps.PrivacyDeletion.DeleteAccountData(r.Context(), currentUserID(deps.SessionManager, r), currentUserEmail(deps.SessionManager, r)); err != nil {
				renderError(w, http.StatusInternalServerError, "Deletion failed", "Account data could not be deleted. Contact support.")
				return
			}
			_ = deps.SessionManager.Destroy(r.Context())
			http.Redirect(w, r, "/", http.StatusSeeOther)
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/domains/import", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				renderError(w, http.StatusServiceUnavailable, "Domains unavailable", "Protected domains are not configured for this environment.")
				return
			}
			reader, closeFn, err := csvImportReader(r)
			if err != nil {
				renderError(w, http.StatusBadRequest, "Import failed", err.Error())
				return
			}
			defer closeFn()
			result, err := deps.Domains.ImportCSV(r.Context(), currentUserID(deps.SessionManager, r), reader)
			if err != nil {
				renderError(w, http.StatusBadRequest, "Import failed", err.Error())
				return
			}
			pushFlash(deps.SessionManager, r, NewFlash(FlashSuccess, fmt.Sprintf("Imported %d domains.", result.Created)))
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/domains", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				renderError(w, http.StatusServiceUnavailable, "Domains unavailable", "Protected domains are not configured for this environment.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			if err := r.ParseForm(); err != nil {
				renderError(w, http.StatusBadRequest, "Invalid form", "Check the submitted domain and try again.")
				return
			}
			domainName := r.PostForm.Get("domain")
			if err := deps.Domains.AddDomain(r.Context(), currentUserID(deps.SessionManager, r), domainName); err != nil {
				renderError(w, http.StatusBadRequest, "Unable to save domain", err.Error())
				return
			}
			pushFlash(deps.SessionManager, r, DomainAddedFlash(domainName))
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/domains/{id}/check", func(w http.ResponseWriter, r *http.Request) {
			if err := deps.Domains.ScanDomain(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id")); err != nil {
				renderError(w, http.StatusBadRequest, "Recheck failed", "The protected domain could not be checked. Confirm it still belongs to your account.")
				return
			}
			pushFlash(deps.SessionManager, r, NewFlash(FlashInfo, "Recheck queued."))
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/domains/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
			if err := deps.Domains.DeleteDomain(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id")); err != nil {
				renderError(w, http.StatusBadRequest, "Delete failed", "The protected domain could not be deleted. Confirm it still belongs to your account.")
				return
			}
			pushFlash(deps.SessionManager, r, NewFlash(FlashSuccess, "Deleted domain."))
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/alerts/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
			if deps.Alerts == nil {
				renderError(w, http.StatusServiceUnavailable, "Alerts unavailable", "Alerts are not configured for this environment.")
				return
			}
			if err := deps.Alerts.AcknowledgeEvent(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id"), time.Now()); err != nil {
				renderError(w, http.StatusBadRequest, "Acknowledgement failed", "The alert could not be acknowledged.")
				return
			}
			pushFlash(deps.SessionManager, r, NewFlash(FlashSuccess, "Alert acknowledged."))
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.With(sameOriginPost(deps.PublicBaseURL), csrfPost(deps.SessionManager)).Post("/alerts/{id}/mute", func(w http.ResponseWriter, r *http.Request) {
			if deps.Alerts == nil {
				renderError(w, http.StatusServiceUnavailable, "Alerts unavailable", "Alerts are not configured for this environment.")
				return
			}
			until := muteUntil(r.PostForm.Get("duration"), time.Now())
			if err := deps.Alerts.MuteEvent(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id"), until); err != nil {
				renderError(w, http.StatusBadRequest, "Mute failed", "The alert could not be muted.")
				return
			}
			pushFlash(deps.SessionManager, r, NewFlash(FlashInfo, "Alert muted."))
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		writeStaticPage(w, brand.ProductName+" status", "All core components are operational.", []string{
			"Public checker",
			"Protected-domain monitoring",
			"DMARC report ingestion",
			"Notifications",
		})
	})
	r.Get("/security", func(w http.ResponseWriter, r *http.Request) {
		writeStaticPage(w, "Security", "Report security issues privately to the project owner before opening public issues.", []string{
			"Passwordless login tokens are hashed, single-use, and expiring.",
			"Public checker requests are rate-limited and body-limited.",
			"DMARC report ingestion rejects unsafe archives and oversized payloads.",
			"Current subprocessors: Lettermint and operator-managed infrastructure.",
		})
	})
	r.Get("/", templ.Handler(views.Home()).ServeHTTP)
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

func writeStaticPage(w http.ResponseWriter, title string, lead string, items []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>%s - %s</title><link rel=\"stylesheet\" href=\"/assets/css/marketing-20260512.css\"></head><body class=\"min-h-screen bg-background font-sans text-foreground\"><main class=\"mx-auto max-w-3xl px-5 py-12\"><a href=\"/\" class=\"font-mono text-xs uppercase tracking-[0.2em] text-yellow\">RecordMaster</a><h1 class=\"mt-6 text-5xl font-semibold text-white\">%s</h1><p class=\"mt-4 text-muted\">%s</p><ul class=\"mt-8 grid gap-3\">", title, brand.FullProductName, title, lead)
	for _, item := range items {
		_, _ = fmt.Fprintf(w, "<li class=\"border border-white/10 bg-card p-4 text-sm text-muted\">%s</li>", item)
	}
	_, _ = fmt.Fprint(w, "</ul></main></body></html>")
}

func renderError(w http.ResponseWriter, status int, title string, detail string) {
	if title == "" {
		title = http.StatusText(status)
	}
	if detail == "" {
		detail = "Try again shortly."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>%s - %s</title><link rel=\"stylesheet\" href=\"/assets/css/marketing-20260512.css\"></head><body class=\"min-h-screen bg-background font-sans text-foreground\"><main class=\"mx-auto grid min-h-screen max-w-xl place-items-center px-5 py-12\"><section class=\"w-full border border-white/10 bg-card p-8\"><a href=\"/\" class=\"font-mono text-xs uppercase tracking-[0.2em] text-yellow\">RecordMaster</a><p class=\"mt-6 font-mono text-xs uppercase tracking-[0.24em] text-muted\">%d</p><h1 class=\"mt-3 text-4xl font-semibold text-white\">%s</h1><p class=\"mt-4 text-sm leading-6 text-muted\">%s</p><a href=\"/\" class=\"mt-8 inline-flex min-h-12 items-center bg-accent px-5 font-mono text-xs font-semibold uppercase tracking-[0.18em] text-white transition hover:bg-white hover:text-black\">Return home</a></section></main></body></html>",
		html.EscapeString(title),
		brand.FullProductName,
		status,
		html.EscapeString(title),
		html.EscapeString(detail),
	)
}

func writeLogoutConfirmationPage(w http.ResponseWriter, csrfToken string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Sign out - %s</title><link rel=\"stylesheet\" href=\"/assets/css/marketing-20260512.css\"></head><body class=\"min-h-screen bg-background font-sans text-foreground\"><main class=\"mx-auto grid min-h-screen max-w-xl place-items-center px-5 py-12\"><section class=\"w-full border border-white/10 bg-card p-8\"><a href=\"/app/domains\" class=\"font-mono text-xs uppercase tracking-[0.2em] text-yellow\">RecordMaster</a><h1 class=\"mt-6 text-4xl font-semibold text-white\">Sign out?</h1><p class=\"mt-4 text-sm leading-6 text-muted\">End this RecordMaster session on the current browser.</p><form method=\"post\" action=\"/logout\" class=\"mt-8 flex flex-wrap gap-3\"><input type=\"hidden\" name=\"csrf_token\" value=\"%s\"><button type=\"submit\" class=\"min-h-12 bg-accent px-5 font-mono text-xs font-semibold uppercase tracking-[0.18em] text-white transition hover:bg-white hover:text-black\">Sign out</button><a href=\"/app/domains\" class=\"inline-flex min-h-12 items-center border border-white/10 px-5 font-mono text-xs font-semibold uppercase tracking-[0.18em] text-muted transition hover:text-white\">Stay signed in</a></form></section></main></body></html>",
		brand.FullProductName,
		html.EscapeString(csrfToken),
	)
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

func requireLogin(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if currentUserID(sessionManager, r) == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func currentUserID(sessionManager *scs.SessionManager, r *http.Request) string {
	return sessionManager.GetString(r.Context(), "user_id")
}

func currentUserEmail(sessionManager *scs.SessionManager, r *http.Request) string {
	return sessionManager.GetString(r.Context(), "user_email")
}

const (
	csrfSessionKey        = "csrf_token"
	flashSessionKey       = "flash_message"
	claimDomainSessionKey = "claim_domain"
)

func startLoginAndSendEmail(ctx context.Context, deps Deps, emailInput string, subject string, intro string) (string, error) {
	email, token, err := deps.Auth.StartLogin(ctx, emailInput)
	if err != nil {
		return "", err
	}
	link := deps.PublicBaseURL + "/auth/callback?token=" + url.QueryEscape(token)
	if err := deps.Sender.Send(ctx, notifier.Email{
		To:      email,
		Subject: subject,
		Text:    intro + ": " + link,
		HTML:    fmt.Sprintf(`<p>%s:</p><p><a href="%s">Open your magic link</a></p>`, html.EscapeString(intro), html.EscapeString(link)),
	}); err != nil {
		return "", err
	}
	return email, nil
}

func popClaimDomain(sessionManager *scs.SessionManager, r *http.Request) string {
	domain := sessionManager.GetString(r.Context(), claimDomainSessionKey)
	if domain == "" {
		return ""
	}
	sessionManager.Remove(r.Context(), claimDomainSessionKey)
	return domain
}

func buildTrustReport(ctx context.Context, email string, userID string, domain domains.Domain, reporterStore UserReporterStore) trustreport.Report {
	input := trustreport.Input{
		GeneratedAt: time.Now().UTC(),
		Customer:    email,
		Domains:     []trustreport.DomainInput{{Domain: domain.Domain}},
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
		input.Domains[0].Scan = scan
		input.Domains[0].DMARCEvidence = dmarcjourney.Evidence{
			ReporterConfigured: strings.Contains(strings.ToLower(domain.LatestCheck.DMARC.Record), "rua="),
		}
	}
	if reporterStore != nil {
		if senders, err := reporterStore.ListUserReporterSenders(ctx, userID, domain.ID, 50); err == nil {
			input.Domains[0].SenderInventory = senders
			mergeSenderEvidence(&input.Domains[0].DMARCEvidence, senders)
		}
		if health, err := reporterStore.GetUserReporterHealth(ctx, userID, domain.ID); err == nil {
			mergeReporterHealth(&input.Domains[0].DMARCEvidence, health)
		}
	}
	return trustreport.Build(input)
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

func csrfToken(sessionManager *scs.SessionManager, r *http.Request) string {
	token := sessionManager.GetString(r.Context(), csrfSessionKey)
	if token != "" {
		return token
	}
	token = newCSRFToken()
	sessionManager.Put(r.Context(), csrfSessionKey, token)
	return token
}

func newCSRFToken() string {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("csrf-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

func pushFlash(sessionManager *scs.SessionManager, r *http.Request, message FlashMessage) {
	visible := VisibleFlashes([]FlashMessage{message})
	if len(visible) == 0 {
		return
	}
	sessionManager.Put(r.Context(), flashSessionKey, encodeFlash(visible[0]))
}

func popFlashViews(sessionManager *scs.SessionManager, r *http.Request) []views.Flash {
	raw := sessionManager.GetString(r.Context(), flashSessionKey)
	if raw == "" {
		return nil
	}
	sessionManager.Remove(r.Context(), flashSessionKey)
	flash := decodeFlash(raw)
	visible := VisibleFlashes([]FlashMessage{flash})
	if len(visible) == 0 {
		return nil
	}
	return []views.Flash{{
		Kind:    string(visible[0].Kind),
		Message: visible[0].Message,
	}}
}

func encodeFlash(message FlashMessage) string {
	kind := strings.ReplaceAll(string(message.Kind), "\t", " ")
	text := strings.ReplaceAll(message.Message, "\t", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	return kind + "\t" + text
}

func decodeFlash(raw string) FlashMessage {
	kind, message, ok := strings.Cut(raw, "\t")
	if !ok {
		return NewFlash(FlashInfo, raw)
	}
	return NewFlash(FlashKind(kind), message)
}

func csrfPost(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			var err error
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				// #nosec G120 -- r.Body is capped by MaxBytesReader immediately above.
				err = r.ParseMultipartForm(4096)
			} else {
				err = r.ParseForm()
			}
			if err != nil {
				renderError(w, http.StatusBadRequest, "Invalid form", "Check the submitted fields and try again.")
				return
			}
			expected := sessionManager.GetString(r.Context(), csrfSessionKey)
			actual := r.PostForm.Get("csrf_token")
			if actual == "" {
				actual = r.Header.Get("X-CSRF-Token")
			}
			if expected == "" || actual == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
				renderError(w, http.StatusForbidden, "Invalid security token", "Refresh the page and submit the form again.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csvImportReader(r *http.Request) (io.Reader, func(), error) {
	file, _, err := r.FormFile("csv")
	if err == nil {
		return file, func() { _ = file.Close() }, nil
	}
	value := strings.TrimSpace(r.PostForm.Get("csv"))
	if value == "" {
		return nil, func() {}, fmt.Errorf("csv import file is required")
	}
	return strings.NewReader(value), func() {}, nil
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

func sameOriginPost(publicBaseURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			if !sameOrigin(r, publicBaseURL) {
				renderError(w, http.StatusForbidden, "Request rejected", "The form submission did not come from this site.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sameOrigin(r *http.Request, publicBaseURL string) bool {
	expected := requestOrigin(r)
	if configured, err := url.Parse(publicBaseURL); err == nil && configured.Host != "" {
		expected = configured.Scheme + "://" + configured.Host
	}
	for _, header := range []string{"Origin", "Referer"} {
		value := r.Header.Get(header)
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return false
		}
		if parsed.Scheme+"://"+parsed.Host != expected {
			return false
		}
	}
	return true
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return scheme + "://" + r.Host
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]rateWindow
}

type rateWindow struct {
	reset time.Time
	count int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		clients: map[string]rateWindow{},
	}
}

func (l *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), time.Now()) {
			renderError(w, http.StatusTooManyRequests, "Too many requests", "Wait a minute before checking another domain.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	window := l.clients[key]
	if window.reset.IsZero() || now.After(window.reset) {
		l.clients[key] = rateWindow{reset: now.Add(l.window), count: 1}
		return true
	}
	if window.count >= l.limit {
		return false
	}
	window.count++
	l.clients[key] = window
	return true
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return strings.TrimSpace(host)
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}
