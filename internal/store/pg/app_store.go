package pg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/AndriGitDev/meil/internal/admin"
	"github.com/AndriGitDev/meil/internal/alerts"
	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/billing"
	"github.com/AndriGitDev/meil/internal/bootstrap"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/entitlements"
	"github.com/AndriGitDev/meil/internal/privacy"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/spfroutes"
	"github.com/AndriGitDev/meil/internal/statuspage"
	"github.com/AndriGitDev/meil/internal/teams"
	"github.com/AndriGitDev/meil/internal/webhooks"
)

type AppStore struct {
	queries *Queries
}

func NewAppStore(queries *Queries) *AppStore {
	return &AppStore{queries: queries}
}

func (s *AppStore) EnsureBootstrapOrganizationAPIKey(ctx context.Context, ownerEmail string, organizationName string, apiKeyName string, apiKeyToken string, scopes []apikeys.Scope) (bootstrap.Result, error) {
	hash, err := apikeys.Hash(apiKeyToken)
	if err != nil {
		return bootstrap.Result{}, err
	}
	scopeStrings := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = apikeys.Scope(strings.TrimSpace(string(scope)))
		if scope != "" {
			scopeStrings = append(scopeStrings, string(scope))
		}
	}

	user, err := s.queries.UpsertUserByEmail(ctx, ownerEmail)
	if err != nil {
		return bootstrap.Result{}, fmt.Errorf("upserting bootstrap owner user: %w", err)
	}

	var organizationID pgtype.UUID
	err = s.queries.db.QueryRow(ctx, `
SELECT id
FROM organizations
WHERE lower(name) = lower($1)
ORDER BY created_at ASC
LIMIT 1
`, organizationName).Scan(&organizationID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.queries.db.QueryRow(ctx, `
INSERT INTO organizations (name, plan)
VALUES ($1, 'pro')
RETURNING id
`, organizationName).Scan(&organizationID)
	}
	if err != nil {
		return bootstrap.Result{}, fmt.Errorf("ensuring bootstrap organization: %w", err)
	}

	_, err = s.queries.db.Exec(ctx, `
INSERT INTO organization_members (organization_id, user_id, role)
VALUES ($1, $2, 'owner')
ON CONFLICT (organization_id, user_id) DO UPDATE SET role = 'owner'
`, organizationID, user.ID)
	if err != nil {
		return bootstrap.Result{}, fmt.Errorf("ensuring bootstrap owner membership: %w", err)
	}

	var apiKeyCreated bool
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO api_keys (organization_id, name, token_hash, scopes)
VALUES ($1, $2, $3, $4)
ON CONFLICT (token_hash) DO UPDATE SET
    organization_id = EXCLUDED.organization_id,
    name = EXCLUDED.name,
    scopes = EXCLUDED.scopes,
    revoked_at = NULL
RETURNING xmax = 0
`, organizationID, apiKeyName, hash, scopeStrings).Scan(&apiKeyCreated)
	if err != nil {
		return bootstrap.Result{}, fmt.Errorf("ensuring bootstrap API key: %w", err)
	}
	return bootstrap.Result{
		Applied:          true,
		UserID:           uuidToString(user.ID),
		OrganizationID:   uuidToString(organizationID),
		APIKeyCreated:    apiKeyCreated,
		APIKeyName:       apiKeyName,
		ConfiguredScopes: scopes,
	}, nil
}

func (s *AppStore) StoreEnrichmentCache(ctx context.Context, entry attribution.EnrichmentEntry) error {
	evidence, err := json.Marshal(entry.Evidence)
	if err != nil {
		return fmt.Errorf("encoding enrichment evidence: %w", err)
	}
	_, err = s.queries.db.Exec(ctx, `
INSERT INTO enrichment_cache (
    source_ip,
    reverse_dns,
    asn,
    country_code,
    provider_hint,
    confidence,
    evidence,
    refreshed_at,
    expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)
ON CONFLICT (source_ip) DO UPDATE SET
    reverse_dns = EXCLUDED.reverse_dns,
    asn = EXCLUDED.asn,
    country_code = EXCLUDED.country_code,
    provider_hint = EXCLUDED.provider_hint,
    confidence = EXCLUDED.confidence,
    evidence = EXCLUDED.evidence,
    refreshed_at = EXCLUDED.refreshed_at,
    expires_at = EXCLUDED.expires_at
`, entry.IP, entry.ReverseDNS, entry.ASN, entry.CountryCode, entry.ProviderHint, string(entry.Confidence), string(evidence), entry.RefreshedAt.UTC(), entry.ExpiresAt.UTC())
	return err
}

func (s *AppStore) AuthenticateAPIKey(ctx context.Context, key string) (apikeys.Principal, error) {
	hash, err := apikeys.Hash(key)
	if err != nil {
		return apikeys.Principal{}, err
	}
	var organizationID pgtype.UUID
	var scopes []string
	var revoked bool
	err = s.queries.db.QueryRow(ctx, `
SELECT organization_id, scopes, revoked_at IS NOT NULL
FROM api_keys
WHERE token_hash = $1
`, hash).Scan(&organizationID, &scopes, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return apikeys.Principal{}, apikeys.ErrInvalidKey
	}
	if err != nil {
		return apikeys.Principal{}, err
	}
	if _, err := s.queries.db.Exec(ctx, `
UPDATE api_keys
SET last_used_at = now()
WHERE token_hash = $1
`, hash); err != nil {
		return apikeys.Principal{}, err
	}
	principal := apikeys.Principal{
		OrganizationID: uuidToString(organizationID),
		Revoked:        revoked,
		Scopes:         make([]apikeys.Scope, 0, len(scopes)),
	}
	for _, scope := range scopes {
		principal.Scopes = append(principal.Scopes, apikeys.Scope(scope))
	}
	return principal, nil
}

func (s *AppStore) ApplyBillingWebhook(ctx context.Context, event billing.WebhookEvent) (bool, error) {
	if strings.TrimSpace(event.ID) == "" {
		return false, fmt.Errorf("billing event id is required")
	}
	if strings.TrimSpace(event.CustomerID) == "" && strings.TrimSpace(event.SubscriptionID) == "" {
		return false, fmt.Errorf("billing customer or subscription id is required")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	tag, err := s.queries.db.Exec(ctx, `
UPDATE organizations
SET
    plan = $1,
    billing_customer_id = $2,
    billing_subscription_id = $3,
    billing_status = $4,
    billing_last_event_id = $5,
    updated_at = $6
WHERE (($2 <> '' AND billing_customer_id = $2) OR ($3 <> '' AND billing_subscription_id = $3))
  AND billing_last_event_id IS DISTINCT FROM $5
`, event.Plan, event.CustomerID, event.SubscriptionID, event.Status, event.ID, event.OccurredAt.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *AppStore) RequestBillingCancellation(ctx context.Context, organizationID string, reason string, periodEndsAt time.Time) (billing.Cancellation, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return billing.Cancellation{}, err
	}
	requestedAt := time.Now().UTC()
	cancellation, err := billing.NewCancellation(organizationID, reason, requestedAt, periodEndsAt)
	if err != nil {
		return billing.Cancellation{}, err
	}
	err = s.queries.db.QueryRow(ctx, `
UPDATE organizations
SET
    billing_cancel_requested_at = $2,
    billing_cancel_effective_at = $3,
    updated_at = $2
WHERE id = $1
RETURNING billing_cancel_requested_at, billing_cancel_effective_at
`, organizationUUID, cancellation.RequestedAt, cancellation.EffectiveAt).Scan(&cancellation.RequestedAt, &cancellation.EffectiveAt)
	if err != nil {
		return billing.Cancellation{}, err
	}
	return cancellation, nil
}

func (s *AppStore) ListOrganizationAPIKeys(ctx context.Context, organizationID string) ([]apikeys.Key, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT id, name, scopes, last_used_at, revoked_at, created_at
FROM api_keys
WHERE organization_id = $1
ORDER BY created_at DESC
`, organizationUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []apikeys.Key{}
	for rows.Next() {
		key, err := scanAPIKey(rows, organizationID)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *AppStore) CreateOrganizationAPIKey(ctx context.Context, organizationID string, name string, scopes []apikeys.Scope) (apikeys.Key, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return apikeys.Key{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return apikeys.Key{}, fmt.Errorf("API key name is required")
	}
	token, hash, err := apikeys.New()
	if err != nil {
		return apikeys.Key{}, err
	}
	scopeStrings := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = apikeys.Scope(strings.TrimSpace(string(scope)))
		if scope != "" {
			scopeStrings = append(scopeStrings, string(scope))
		}
	}
	var key apikeys.Key
	var id pgtype.UUID
	var createdAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO api_keys (organization_id, name, token_hash, scopes)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at
`, organizationUUID, name, hash, scopeStrings).Scan(&id, &createdAt)
	if err != nil {
		return apikeys.Key{}, err
	}
	key.ID = uuidToString(id)
	key.OrganizationID = organizationID
	key.Name = name
	key.Scopes = scopes
	key.Token = token
	key.CreatedAt = createdAt.Time
	return key, nil
}

func (s *AppStore) RevokeOrganizationAPIKey(ctx context.Context, organizationID string, keyID string) (apikeys.Key, error) {
	organizationUUID, keyUUID, err := organizationDomainUUIDs(organizationID, keyID)
	if err != nil {
		return apikeys.Key{}, err
	}
	row := s.queries.db.QueryRow(ctx, `
UPDATE api_keys
SET revoked_at = COALESCE(revoked_at, now())
WHERE organization_id = $1 AND id = $2
RETURNING id, name, scopes, last_used_at, revoked_at, created_at
`, organizationUUID, keyUUID)
	return scanAPIKey(row, organizationID)
}

type apiKeyRow interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apiKeyRow, organizationID string) (apikeys.Key, error) {
	var key apikeys.Key
	var id pgtype.UUID
	var scopes []string
	var lastUsedAt pgtype.Timestamptz
	var revokedAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &key.Name, &scopes, &lastUsedAt, &revokedAt, &createdAt); err != nil {
		return apikeys.Key{}, err
	}
	key.ID = uuidToString(id)
	key.OrganizationID = organizationID
	key.CreatedAt = createdAt.Time
	key.Scopes = make([]apikeys.Scope, 0, len(scopes))
	for _, scope := range scopes {
		key.Scopes = append(key.Scopes, apikeys.Scope(scope))
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = lastUsedAt.Time
	}
	if revokedAt.Valid {
		key.RevokedAt = revokedAt.Time
	}
	return key, nil
}

func (s *AppStore) ListOrganizationInvitations(ctx context.Context, organizationID string) ([]teams.Invitation, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT id, email, role, expires_at, accepted_at, created_at
FROM organization_invitations
WHERE organization_id = $1
ORDER BY created_at DESC
`, organizationUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invitations := []teams.Invitation{}
	for rows.Next() {
		invitation, err := scanInvitation(rows, organizationID)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (s *AppStore) CreateOrganizationInvitation(ctx context.Context, organizationID string, email string, role entitlements.Role) (teams.Invitation, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return teams.Invitation{}, err
	}
	invitation, err := teams.NewInvitation(email, role)
	if err != nil {
		return teams.Invitation{}, err
	}
	invitation.OrganizationID = organizationID
	invitation.ExpiresAt = time.Now().UTC().Add(7 * 24 * time.Hour)
	var id pgtype.UUID
	var createdAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO organization_invitations (organization_id, email, role, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at
`, organizationUUID, invitation.Email, string(invitation.Role), invitation.TokenHash, invitation.ExpiresAt).Scan(&id, &createdAt)
	if err != nil {
		return teams.Invitation{}, err
	}
	invitation.ID = uuidToString(id)
	invitation.CreatedAt = createdAt.Time
	return invitation, nil
}

func (s *AppStore) RequestOrganizationDomainTransfer(ctx context.Context, organizationID string, domainID string, toOrganizationID string, requestedByUserID string) (teams.TransferRequest, error) {
	fromOrganizationUUID, domainUUID, err := organizationDomainUUIDs(organizationID, domainID)
	if err != nil {
		return teams.TransferRequest{}, err
	}
	toOrganizationUUID, err := uuidFromString(toOrganizationID)
	if err != nil {
		return teams.TransferRequest{}, err
	}
	requestedByUUID, err := uuidFromString(requestedByUserID)
	if err != nil {
		return teams.TransferRequest{}, err
	}
	request, err := teams.NewTransferRequest(teams.DomainTransfer{
		DomainID:          domainID,
		FromOrganization:  organizationID,
		ToOrganization:    toOrganizationID,
		RequestedByUserID: requestedByUserID,
	}, entitlements.RoleAdmin, time.Now().UTC())
	if err != nil {
		return teams.TransferRequest{}, err
	}
	var transferID pgtype.UUID
	var createdAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO domain_transfers (domain_id, from_organization_id, to_organization_id, requested_by_user_id, status)
SELECT d.id, $2, $3, $4, $5
FROM domains d
WHERE d.id = $1 AND d.organization_id = $2
RETURNING id, created_at
`, domainUUID, fromOrganizationUUID, toOrganizationUUID, requestedByUUID, string(request.Status)).Scan(&transferID, &createdAt)
	if err != nil {
		return teams.TransferRequest{}, err
	}
	request.ID = uuidToString(transferID)
	request.RequestedAt = createdAt.Time
	return request, nil
}

func (s *AppStore) GetOrganizationRetention(ctx context.Context, organizationID string) (privacy.RetentionRequest, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return privacy.RetentionRequest{}, err
	}
	var request privacy.RetentionRequest
	err = s.queries.db.QueryRow(ctx, `
SELECT raw_report_days, aggregate_row_days, notification_log_days
FROM organization_retention_settings
WHERE organization_id = $1
`, organizationUUID).Scan(&request.RawReportDays, &request.AggregateRowDays, &request.NotificationLogDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return privacy.RetentionRequest{RawReportDays: 90, AggregateRowDays: 365, NotificationLogDays: 180}, nil
	}
	if err != nil {
		return privacy.RetentionRequest{}, err
	}
	return request, nil
}

func (s *AppStore) SaveOrganizationRetention(ctx context.Context, organizationID string, request privacy.RetentionRequest) (privacy.RetentionRequest, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return privacy.RetentionRequest{}, err
	}
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO organization_retention_settings (organization_id, raw_report_days, aggregate_row_days, notification_log_days)
VALUES ($1, $2, $3, $4)
ON CONFLICT (organization_id) DO UPDATE SET
    raw_report_days = EXCLUDED.raw_report_days,
    aggregate_row_days = EXCLUDED.aggregate_row_days,
    notification_log_days = EXCLUDED.notification_log_days,
    updated_at = now()
RETURNING raw_report_days, aggregate_row_days, notification_log_days
`, organizationUUID, request.RawReportDays, request.AggregateRowDays, request.NotificationLogDays).Scan(&request.RawReportDays, &request.AggregateRowDays, &request.NotificationLogDays)
	if err != nil {
		return privacy.RetentionRequest{}, err
	}
	return request, nil
}

func (s *AppStore) DeleteAccountData(ctx context.Context, userID string, email string) (privacy.DeletionPlan, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return privacy.DeletionPlan{}, err
	}
	plan := privacy.NewDeletionPlan(userID)
	if _, err := s.queries.db.Exec(ctx, `
UPDATE api_keys
SET revoked_at = COALESCE(revoked_at, now())
WHERE organization_id IN (
    SELECT organization_id
    FROM organization_members
    WHERE user_id = $1
)
`, userUUID); err != nil {
		return privacy.DeletionPlan{}, err
	}
	if strings.TrimSpace(email) != "" {
		if _, err := s.queries.db.Exec(ctx, `
DELETE FROM organization_invitations
WHERE lower(email) = lower($1)
`, email); err != nil {
			return privacy.DeletionPlan{}, err
		}
	}
	if _, err := s.queries.db.Exec(ctx, `
INSERT INTO audit_log (user_id, action, target_type, target_id, metadata)
VALUES ($1, 'account.deleted', 'user', $2, jsonb_build_object('email', $3))
`, userUUID, userID, email); err != nil {
		return privacy.DeletionPlan{}, err
	}
	for _, statement := range []string{
		`DELETE FROM domains WHERE user_id = $1`,
		`DELETE FROM organization_members WHERE user_id = $1`,
		`DELETE FROM login_tokens WHERE user_id = $1`,
		`DELETE FROM users WHERE id = $1`,
	} {
		if _, err := s.queries.db.Exec(ctx, statement, userUUID); err != nil {
			return privacy.DeletionPlan{}, err
		}
	}
	return plan, nil
}

func (s *AppStore) ListAdminCases(ctx context.Context, status string, limit int) ([]admin.Case, error) {
	args := []any{limit}
	query := `
SELECT id, kind, status, subject, user_id, domain_id, assigned_to, created_at, resolved_at
FROM admin_cases
ORDER BY created_at DESC
LIMIT $1
`
	if strings.TrimSpace(status) != "" {
		query = `
SELECT id, kind, status, subject, user_id, domain_id, assigned_to, created_at, resolved_at
FROM admin_cases
WHERE status = $2
ORDER BY created_at DESC
LIMIT $1
`
		args = append(args, status)
	}
	rows, err := s.queries.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cases := []admin.Case{}
	for rows.Next() {
		c, err := scanAdminCase(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

func (s *AppStore) CreateAdminCase(ctx context.Context, kind admin.CaseKind, subject string, userID string, domainID string) (admin.Case, error) {
	c, err := admin.NewCase(kind, subject, userID, domainID, time.Now().UTC())
	if err != nil {
		return admin.Case{}, err
	}
	userUUID, userValid, err := nullableUUID(userID)
	if err != nil {
		return admin.Case{}, err
	}
	domainUUID, domainValid, err := nullableUUID(domainID)
	if err != nil {
		return admin.Case{}, err
	}
	row := s.queries.db.QueryRow(ctx, `
INSERT INTO admin_cases (kind, status, subject, user_id, domain_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, kind, status, subject, user_id, domain_id, assigned_to, created_at, resolved_at
`, string(c.Kind), string(c.Status), c.Subject, pgtype.UUID{Bytes: userUUID.Bytes, Valid: userValid}, pgtype.UUID{Bytes: domainUUID.Bytes, Valid: domainValid})
	return scanAdminCase(row)
}

func (s *AppStore) ResolveAdminCase(ctx context.Context, caseID string, actorUserID string) (admin.Case, error) {
	caseUUID, err := uuidFromString(caseID)
	if err != nil {
		return admin.Case{}, err
	}
	actorUUID, actorValid, err := nullableUUID(actorUserID)
	if err != nil {
		return admin.Case{}, err
	}
	row := s.queries.db.QueryRow(ctx, `
UPDATE admin_cases
SET status = $3, assigned_to = COALESCE(assigned_to, $2), resolved_at = now()
WHERE id = $1
RETURNING id, kind, status, subject, user_id, domain_id, assigned_to, created_at, resolved_at
`, caseUUID, pgtype.UUID{Bytes: actorUUID.Bytes, Valid: actorValid}, string(admin.CaseResolved))
	return scanAdminCase(row)
}

func (s *AppStore) ListAbuseDecisions(ctx context.Context, identifier string, limit int) ([]admin.AbuseDecision, error) {
	args := []any{limit}
	query := `
SELECT id, identifier, action, reason, expires_at, created_at
FROM abuse_decisions
ORDER BY created_at DESC
LIMIT $1
`
	if strings.TrimSpace(identifier) != "" {
		query = `
SELECT id, identifier, action, reason, expires_at, created_at
FROM abuse_decisions
WHERE identifier = $2
ORDER BY created_at DESC
LIMIT $1
`
		args = append(args, identifier)
	}
	rows, err := s.queries.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := []admin.AbuseDecision{}
	for rows.Next() {
		decision, err := scanAbuseDecision(rows)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	return decisions, rows.Err()
}

func (s *AppStore) CreateAbuseDecision(ctx context.Context, identifier string, action admin.AbuseAction, reason string, duration time.Duration) (admin.AbuseDecision, error) {
	decision, err := admin.NewAbuseDecision(identifier, action, reason, duration, time.Now().UTC())
	if err != nil {
		return admin.AbuseDecision{}, err
	}
	expiresAt := pgtype.Timestamptz{}
	if !decision.ExpiresAt.IsZero() {
		expiresAt = pgtype.Timestamptz{Time: decision.ExpiresAt, Valid: true}
	}
	row := s.queries.db.QueryRow(ctx, `
INSERT INTO abuse_decisions (identifier, action, reason, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, identifier, action, reason, expires_at, created_at
`, decision.Identifier, string(decision.Action), decision.Reason, expiresAt)
	return scanAbuseDecision(row)
}

func (s *AppStore) ListStatusComponents(ctx context.Context) ([]statuspage.Component, error) {
	rows, err := s.queries.db.Query(ctx, `
SELECT id, name, status, updated_at
FROM status_components
ORDER BY name
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	components := []statuspage.Component{}
	for rows.Next() {
		component, err := scanStatusComponent(rows)
		if err != nil {
			return nil, err
		}
		components = append(components, component)
	}
	return components, rows.Err()
}

func (s *AppStore) UpsertStatusComponent(ctx context.Context, name string, status statuspage.ComponentStatus) (statuspage.Component, error) {
	name = strings.TrimSpace(name)
	if name == "" || status == "" {
		return statuspage.Component{}, fmt.Errorf("status component requires name and status")
	}
	row := s.queries.db.QueryRow(ctx, `
INSERT INTO status_components (name, status)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE SET status = EXCLUDED.status, updated_at = now()
RETURNING id, name, status, updated_at
`, name, string(status))
	return scanStatusComponent(row)
}

func (s *AppStore) ListStatusIncidents(ctx context.Context, limit int) ([]statuspage.Incident, error) {
	rows, err := s.queries.db.Query(ctx, `
SELECT id, title, status, started_at, resolved_at, created_at
FROM status_incidents
ORDER BY started_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	incidents := []statuspage.Incident{}
	for rows.Next() {
		incident, err := scanStatusIncident(rows)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, incident)
	}
	return incidents, rows.Err()
}

func (s *AppStore) CreateStatusIncident(ctx context.Context, title string, status string, message string) (statuspage.Incident, error) {
	title = strings.TrimSpace(title)
	status = strings.TrimSpace(status)
	if title == "" || status == "" {
		return statuspage.Incident{}, fmt.Errorf("status incident requires title and status")
	}
	row := s.queries.db.QueryRow(ctx, `
INSERT INTO status_incidents (title, status, started_at)
VALUES ($1, $2, now())
RETURNING id, title, status, started_at, resolved_at, created_at
`, title, status)
	incident, err := scanStatusIncident(row)
	if err != nil {
		return statuspage.Incident{}, err
	}
	incidentUUID, err := uuidFromString(incident.ID)
	if err != nil {
		return statuspage.Incident{}, err
	}
	if strings.TrimSpace(message) != "" {
		if _, err := s.queries.db.Exec(ctx, `
INSERT INTO status_incident_updates (incident_id, message)
VALUES ($1, $2)
`, incidentUUID, strings.TrimSpace(message)); err != nil {
			return statuspage.Incident{}, err
		}
	}
	return incident, nil
}

func (s *AppStore) ResolveStatusIncident(ctx context.Context, incidentID string, message string) (statuspage.Incident, error) {
	incidentUUID, err := uuidFromString(incidentID)
	if err != nil {
		return statuspage.Incident{}, err
	}
	row := s.queries.db.QueryRow(ctx, `
UPDATE status_incidents
SET status = 'resolved', resolved_at = COALESCE(resolved_at, now())
WHERE id = $1
RETURNING id, title, status, started_at, resolved_at, created_at
`, incidentUUID)
	incident, err := scanStatusIncident(row)
	if err != nil {
		return statuspage.Incident{}, err
	}
	if strings.TrimSpace(message) != "" {
		if _, err := s.queries.db.Exec(ctx, `
INSERT INTO status_incident_updates (incident_id, message)
VALUES ($1, $2)
`, incidentUUID, strings.TrimSpace(message)); err != nil {
			return statuspage.Incident{}, err
		}
	}
	return incident, nil
}

type statusComponentRow interface {
	Scan(dest ...any) error
}

func scanStatusComponent(row statusComponentRow) (statuspage.Component, error) {
	var component statuspage.Component
	var id pgtype.UUID
	var status string
	var updatedAt pgtype.Timestamptz
	if err := row.Scan(&id, &component.Name, &status, &updatedAt); err != nil {
		return statuspage.Component{}, err
	}
	component.ID = uuidToString(id)
	component.Status = statuspage.ComponentStatus(status)
	component.UpdatedAt = updatedAt.Time
	return component, nil
}

type statusIncidentRow interface {
	Scan(dest ...any) error
}

func scanStatusIncident(row statusIncidentRow) (statuspage.Incident, error) {
	var incident statuspage.Incident
	var id pgtype.UUID
	var startedAt pgtype.Timestamptz
	var resolvedAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &incident.Title, &incident.Status, &startedAt, &resolvedAt, &createdAt); err != nil {
		return statuspage.Incident{}, err
	}
	incident.ID = uuidToString(id)
	incident.StartedAt = startedAt.Time
	if resolvedAt.Valid {
		incident.ResolvedAt = resolvedAt.Time
	}
	incident.CreatedAt = createdAt.Time
	return incident, nil
}

type abuseDecisionRow interface {
	Scan(dest ...any) error
}

func scanAbuseDecision(row abuseDecisionRow) (admin.AbuseDecision, error) {
	var decision admin.AbuseDecision
	var id pgtype.UUID
	var action string
	var expiresAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &decision.Identifier, &action, &decision.Reason, &expiresAt, &createdAt); err != nil {
		return admin.AbuseDecision{}, err
	}
	decision.ID = uuidToString(id)
	decision.Action = admin.AbuseAction(action)
	if expiresAt.Valid {
		decision.ExpiresAt = expiresAt.Time
	}
	decision.CreatedAt = createdAt.Time
	return decision, nil
}

type adminCaseRow interface {
	Scan(dest ...any) error
}

func scanAdminCase(row adminCaseRow) (admin.Case, error) {
	var c admin.Case
	var id pgtype.UUID
	var kind string
	var status string
	var userID pgtype.UUID
	var domainID pgtype.UUID
	var assignedTo pgtype.UUID
	var createdAt pgtype.Timestamptz
	var resolvedAt pgtype.Timestamptz
	if err := row.Scan(&id, &kind, &status, &c.Subject, &userID, &domainID, &assignedTo, &createdAt, &resolvedAt); err != nil {
		return admin.Case{}, err
	}
	c.ID = uuidToString(id)
	c.Kind = admin.CaseKind(kind)
	c.Status = admin.CaseStatus(status)
	if userID.Valid {
		c.UserID = uuidToString(userID)
	}
	if domainID.Valid {
		c.DomainID = uuidToString(domainID)
	}
	if assignedTo.Valid {
		c.AssignedTo = uuidToString(assignedTo)
	}
	c.CreatedAt = createdAt.Time
	if resolvedAt.Valid {
		c.ResolvedAt = resolvedAt.Time
	}
	return c, nil
}

func nullableUUID(value string) (pgtype.UUID, bool, error) {
	if strings.TrimSpace(value) == "" {
		return pgtype.UUID{}, false, nil
	}
	id, err := uuidFromString(value)
	if err != nil {
		return pgtype.UUID{}, false, err
	}
	return id, true, nil
}

type invitationRow interface {
	Scan(dest ...any) error
}

func scanInvitation(row invitationRow, organizationID string) (teams.Invitation, error) {
	var invitation teams.Invitation
	var id pgtype.UUID
	var role string
	var expiresAt pgtype.Timestamptz
	var acceptedAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &invitation.Email, &role, &expiresAt, &acceptedAt, &createdAt); err != nil {
		return teams.Invitation{}, err
	}
	invitation.ID = uuidToString(id)
	invitation.OrganizationID = organizationID
	invitation.Role = entitlements.Role(role)
	invitation.ExpiresAt = expiresAt.Time
	if acceptedAt.Valid {
		invitation.AcceptedAt = acceptedAt.Time
	}
	invitation.CreatedAt = createdAt.Time
	return invitation, nil
}

func (s *AppStore) UpsertUserByEmail(ctx context.Context, email string) (auth.User, error) {
	user, err := s.queries.UpsertUserByEmail(ctx, email)
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{ID: uuidToString(user.ID), Email: user.Email}, nil
}

func (s *AppStore) CreateLoginToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.CreateLoginToken(ctx, CreateLoginTokenParams{
		UserID:    userUUID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return err
}

func (s *AppStore) ConsumeLoginToken(ctx context.Context, tokenHash []byte) (auth.LoginToken, error) {
	token, err := s.queries.ConsumeLoginToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.LoginToken{}, s.loginTokenStatusError(ctx, tokenHash)
		}
		return auth.LoginToken{}, err
	}
	user, err := s.queries.GetUserByID(ctx, token.UserID)
	if err != nil {
		return auth.LoginToken{}, err
	}
	return auth.LoginToken{UserID: uuidToString(token.UserID), Email: user.Email}, nil
}

func (s *AppStore) loginTokenStatusError(ctx context.Context, tokenHash []byte) error {
	var consumed bool
	var expired bool
	err := s.queries.db.QueryRow(ctx, `
SELECT consumed_at IS NOT NULL, expires_at <= now()
FROM login_tokens
WHERE token_hash = $1
`, tokenHash).Scan(&consumed, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if consumed {
		return auth.ErrConsumedToken
	}
	if expired {
		return auth.ErrExpiredToken
	}
	return auth.ErrInvalidToken
}

func (s *AppStore) CreateDomain(ctx context.Context, userID string, domain string) (domains.Domain, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return domains.Domain{}, err
	}
	saved, err := s.queries.CreateDomain(ctx, CreateDomainParams{UserID: userUUID, Domain: domain})
	if err != nil {
		return domains.Domain{}, err
	}
	return domainFromParts(saved.ID, saved.Domain, saved.CreatedAt), nil
}

func (s *AppStore) ListDomains(ctx context.Context, userID string) ([]domains.Domain, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDomainsWithLatestCheck(ctx, userUUID)
	if err != nil {
		return nil, err
	}
	result := make([]domains.Domain, 0, len(rows))
	for _, row := range rows {
		saved := domains.Domain{
			ID:        uuidToString(row.ID),
			Domain:    row.Domain,
			CreatedAt: row.CreatedAt.Time,
		}
		if row.CheckID.Valid {
			saved.LatestCheck = &domains.Check{
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.Status(row.SpfStatus), Message: row.SpfMessage, Record: row.SpfRecord},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.Status(row.DkimStatus), Message: row.DkimMessage, Record: row.DkimRecord},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.Status(row.DmarcStatus), Message: row.DmarcMessage, Record: row.DmarcRecord},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.Status(row.BimiStatus), Message: row.BimiMessage, Record: row.BimiRecord},
				CheckedAt: row.CheckedAt.Time,
			}
		}
		result = append(result, saved)
	}
	return result, nil
}

func (s *AppStore) ListOrganizationDomains(ctx context.Context, organizationID string) ([]domains.Domain, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    d.id,
    d.domain,
    d.created_at,
    c.id AS check_id,
    COALESCE(c.spf_status, '') AS spf_status,
    COALESCE(c.spf_message, '') AS spf_message,
    COALESCE(c.spf_record, '') AS spf_record,
    COALESCE(c.dkim_status, '') AS dkim_status,
    COALESCE(c.dkim_message, '') AS dkim_message,
    COALESCE(c.dkim_record, '') AS dkim_record,
    COALESCE(c.dmarc_status, '') AS dmarc_status,
    COALESCE(c.dmarc_message, '') AS dmarc_message,
    COALESCE(c.dmarc_record, '') AS dmarc_record,
    COALESCE(c.bimi_status, '') AS bimi_status,
    COALESCE(c.bimi_message, '') AS bimi_message,
    COALESCE(c.bimi_record, '') AS bimi_record,
    c.created_at AS checked_at
FROM domains d
LEFT JOIN LATERAL (
    SELECT id, spf_status, spf_message, spf_record, dkim_status, dkim_message, dkim_record, dmarc_status, dmarc_message, dmarc_record, bimi_status, bimi_message, bimi_record, created_at
    FROM domain_checks dc
    WHERE dc.domain_id = d.id
    ORDER BY dc.created_at DESC
    LIMIT 1
) c ON true
WHERE d.organization_id = $1
ORDER BY d.created_at DESC
`, organizationUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []domains.Domain{}
	for rows.Next() {
		var saved domains.Domain
		var domainID pgtype.UUID
		var checkID pgtype.UUID
		var spfStatus, spfMessage, spfRecord string
		var dkimStatus, dkimMessage, dkimRecord string
		var dmarcStatus, dmarcMessage, dmarcRecord string
		var bimiStatus, bimiMessage, bimiRecord string
		var createdAt pgtype.Timestamptz
		var checkedAt pgtype.Timestamptz
		if err := rows.Scan(
			&domainID,
			&saved.Domain,
			&createdAt,
			&checkID,
			&spfStatus,
			&spfMessage,
			&spfRecord,
			&dkimStatus,
			&dkimMessage,
			&dkimRecord,
			&dmarcStatus,
			&dmarcMessage,
			&dmarcRecord,
			&bimiStatus,
			&bimiMessage,
			&bimiRecord,
			&checkedAt,
		); err != nil {
			return nil, err
		}
		saved.ID = uuidToString(domainID)
		saved.CreatedAt = createdAt.Time
		if checkID.Valid {
			saved.LatestCheck = &domains.Check{
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.Status(spfStatus), Message: spfMessage, Record: spfRecord},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.Status(dkimStatus), Message: dkimMessage, Record: dkimRecord},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.Status(dmarcStatus), Message: dmarcMessage, Record: dmarcRecord},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.Status(bimiStatus), Message: bimiMessage, Record: bimiRecord},
				CheckedAt: checkedAt.Time,
			}
		}
		result = append(result, saved)
	}
	return result, rows.Err()
}

func (s *AppStore) CreateOrganizationDomain(ctx context.Context, organizationID string, domain string) (domains.Domain, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return domains.Domain{}, err
	}
	var saved domains.Domain
	var domainID pgtype.UUID
	var createdAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
WITH inserted AS (
    INSERT INTO domains (organization_id, domain)
    VALUES ($1, $2)
    ON CONFLICT DO NOTHING
    RETURNING id, domain, created_at
)
SELECT id, domain, created_at FROM inserted
UNION ALL
SELECT id, domain, created_at FROM domains
WHERE organization_id = $1 AND domain = $2
ORDER BY created_at DESC
LIMIT 1
`, organizationUUID, domain).Scan(&domainID, &saved.Domain, &createdAt)
	if err != nil {
		return domains.Domain{}, err
	}
	saved.ID = uuidToString(domainID)
	saved.CreatedAt = createdAt.Time
	return saved, nil
}

func (s *AppStore) GetOrganizationDomain(ctx context.Context, organizationID string, domainID string) (domains.Domain, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return domains.Domain{}, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return domains.Domain{}, err
	}
	var saved domains.Domain
	var checkID pgtype.UUID
	var spfStatus, spfMessage, spfRecord string
	var dkimStatus, dkimMessage, dkimRecord string
	var dmarcStatus, dmarcMessage, dmarcRecord string
	var bimiStatus, bimiMessage, bimiRecord string
	var createdAt pgtype.Timestamptz
	var checkedAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
SELECT
    d.domain,
    d.created_at,
    c.id AS check_id,
    COALESCE(c.spf_status, '') AS spf_status,
    COALESCE(c.spf_message, '') AS spf_message,
    COALESCE(c.spf_record, '') AS spf_record,
    COALESCE(c.dkim_status, '') AS dkim_status,
    COALESCE(c.dkim_message, '') AS dkim_message,
    COALESCE(c.dkim_record, '') AS dkim_record,
    COALESCE(c.dmarc_status, '') AS dmarc_status,
    COALESCE(c.dmarc_message, '') AS dmarc_message,
    COALESCE(c.dmarc_record, '') AS dmarc_record,
    COALESCE(c.bimi_status, '') AS bimi_status,
    COALESCE(c.bimi_message, '') AS bimi_message,
    COALESCE(c.bimi_record, '') AS bimi_record,
    c.created_at AS checked_at
FROM domains d
LEFT JOIN LATERAL (
    SELECT id, spf_status, spf_message, spf_record, dkim_status, dkim_message, dkim_record, dmarc_status, dmarc_message, dmarc_record, bimi_status, bimi_message, bimi_record, created_at
    FROM domain_checks dc
    WHERE dc.domain_id = d.id
    ORDER BY dc.created_at DESC
    LIMIT 1
) c ON true
WHERE d.organization_id = $1 AND d.id = $2
`, organizationUUID, domainUUID).Scan(
		&saved.Domain,
		&createdAt,
		&checkID,
		&spfStatus,
		&spfMessage,
		&spfRecord,
		&dkimStatus,
		&dkimMessage,
		&dkimRecord,
		&dmarcStatus,
		&dmarcMessage,
		&dmarcRecord,
		&bimiStatus,
		&bimiMessage,
		&bimiRecord,
		&checkedAt,
	)
	if err != nil {
		return domains.Domain{}, err
	}
	saved.ID = domainID
	saved.CreatedAt = createdAt.Time
	if checkID.Valid {
		saved.LatestCheck = &domains.Check{
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.Status(spfStatus), Message: spfMessage, Record: spfRecord},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.Status(dkimStatus), Message: dkimMessage, Record: dkimRecord},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.Status(dmarcStatus), Message: dmarcMessage, Record: dmarcRecord},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.Status(bimiStatus), Message: bimiMessage, Record: bimiRecord},
			CheckedAt: checkedAt.Time,
		}
	}
	return saved, nil
}

func (s *AppStore) QueueOrganizationDomainRecheck(ctx context.Context, organizationID string, domainID string, at time.Time) (domains.RecheckJob, error) {
	organizationUUID, domainUUID, err := organizationDomainUUIDs(organizationID, domainID)
	if err != nil {
		return domains.RecheckJob{}, err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	slot := at.UTC().Truncate(5 * time.Minute)
	idempotencyKey := "dashboard-recheck:" + organizationID + ":" + domainID + ":" + slot.Format(time.RFC3339)
	var job domains.RecheckJob
	var id pgtype.UUID
	var createdAt pgtype.Timestamptz
	var scheduledFor pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
WITH scoped_domain AS (
    SELECT id
    FROM domains
    WHERE organization_id = $1 AND id = $2
),
inserted AS (
    INSERT INTO scan_jobs (domain_id, idempotency_key, scheduled_for, status)
    SELECT id, $3, $4, 'queued'
    FROM scoped_domain
    ON CONFLICT (idempotency_key) DO UPDATE SET updated_at = scan_jobs.updated_at
    RETURNING id, domain_id, status, created_at, scheduled_for
)
SELECT id, status, created_at, scheduled_for
FROM inserted
`, organizationUUID, domainUUID, idempotencyKey, at.UTC()).Scan(&id, &job.Status, &createdAt, &scheduledFor)
	if err != nil {
		return domains.RecheckJob{}, err
	}
	job.ID = uuidToString(id)
	job.DomainID = domainID
	job.QueuedAt = createdAt.Time
	job.NextAllowed = slot.Add(5 * time.Minute)
	if scheduledFor.Valid && job.QueuedAt.IsZero() {
		job.QueuedAt = scheduledFor.Time
	}
	return job, nil
}

func (s *AppStore) ListOrganizationDashboardAlerts(ctx context.Context, organizationID string, activeOnly bool, limit int) ([]alerts.DashboardAlert, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	args := []any{organizationUUID, limit}
	activeClause := ""
	if activeOnly {
		activeClause = "AND e.acknowledged_at IS NULL AND (e.muted_until IS NULL OR e.muted_until <= now())"
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    e.id,
    d.id,
    d.domain,
    e.check_name,
    e.kind,
    e.severity,
    e.message,
    e.before_status,
    e.after_status,
    e.before_record,
    e.after_record,
    e.created_at,
    e.muted_until,
    e.acknowledged_at
FROM drift_events e
JOIN domains d ON d.id = e.domain_id
WHERE d.organization_id = $1 `+activeClause+`
ORDER BY e.created_at DESC
LIMIT $2
`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []alerts.DashboardAlert{}
	for rows.Next() {
		var alert alerts.DashboardAlert
		var alertID pgtype.UUID
		var domainID pgtype.UUID
		var kind string
		var severity string
		var beforeStatus string
		var afterStatus string
		var createdAt pgtype.Timestamptz
		var mutedUntil pgtype.Timestamptz
		var acknowledgedAt pgtype.Timestamptz
		if err := rows.Scan(
			&alertID,
			&domainID,
			&alert.Domain,
			&alert.Event.CheckName,
			&kind,
			&severity,
			&alert.Event.Message,
			&beforeStatus,
			&afterStatus,
			&alert.Event.BeforeRecord,
			&alert.Event.AfterRecord,
			&createdAt,
			&mutedUntil,
			&acknowledgedAt,
		); err != nil {
			return nil, err
		}
		alert.ID = uuidToString(alertID)
		alert.DomainID = uuidToString(domainID)
		alert.Event.Kind = drift.Kind(kind)
		alert.Event.Severity = drift.Severity(severity)
		alert.Event.BeforeStatus = scanner.Status(beforeStatus)
		alert.Event.AfterStatus = scanner.Status(afterStatus)
		alert.CreatedAt = createdAt.Time
		if mutedUntil.Valid {
			alert.MutedUntil = mutedUntil.Time
		}
		if acknowledgedAt.Valid {
			alert.AcknowledgedAt = acknowledgedAt.Time
		}
		result = append(result, alert)
	}
	return result, rows.Err()
}

func (s *AppStore) AcknowledgeOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, reason string) error {
	return s.updateOrganizationDashboardAlert(ctx, organizationID, alertID, "acknowledged", reason, time.Time{})
}

func (s *AppStore) MuteOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, until time.Time, reason string) error {
	return s.updateOrganizationDashboardAlert(ctx, organizationID, alertID, "muted", reason, until.UTC())
}

func (s *AppStore) updateOrganizationDashboardAlert(ctx context.Context, organizationID string, alertID string, action string, reason string, mutedUntil time.Time) error {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return err
	}
	alertUUID, err := uuidFromString(alertID)
	if err != nil {
		return err
	}
	var domainID pgtype.UUID
	var tag pgconn.CommandTag
	if action == "acknowledged" {
		tag, err = s.queries.db.Exec(ctx, `
UPDATE drift_events e
SET acknowledged_at = COALESCE(acknowledged_at, now())
FROM domains d
WHERE d.id = e.domain_id AND d.organization_id = $1 AND e.id = $2
`, organizationUUID, alertUUID)
	} else {
		tag, err = s.queries.db.Exec(ctx, `
UPDATE drift_events e
SET muted_until = $3
FROM domains d
WHERE d.id = e.domain_id AND d.organization_id = $1 AND e.id = $2
`, organizationUUID, alertUUID, mutedUntil)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_ = s.queries.db.QueryRow(ctx, `
SELECT domain_id
FROM drift_events
WHERE id = $1
`, alertUUID).Scan(&domainID)
	metadata := map[string]string{"actor": "api_key", "reason": strings.TrimSpace(reason)}
	if !mutedUntil.IsZero() {
		metadata["muted_until"] = mutedUntil.UTC().Format(time.RFC3339)
	}
	return s.insertOrganizationAudit(ctx, organizationUUID, domainID, "dashboard.alert."+action, "alert", alertID, metadata)
}

func (s *AppStore) AcknowledgeEvent(ctx context.Context, userID string, eventID string, at time.Time) error {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	eventUUID, err := uuidFromString(eventID)
	if err != nil {
		return err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tag, err := s.queries.db.Exec(ctx, `
UPDATE drift_events e
SET acknowledged_at = COALESCE(acknowledged_at, $3)
FROM domains d
WHERE d.id = e.domain_id AND d.user_id = $1 AND e.id = $2
`, userUUID, eventUUID, at.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *AppStore) MuteEvent(ctx context.Context, userID string, eventID string, until time.Time) error {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	eventUUID, err := uuidFromString(eventID)
	if err != nil {
		return err
	}
	tag, err := s.queries.db.Exec(ctx, `
UPDATE drift_events e
SET muted_until = $3
FROM domains d
WHERE d.id = e.domain_id AND d.user_id = $1 AND e.id = $2
`, userUUID, eventUUID, until.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *AppStore) GetOrganizationDashboardSettings(ctx context.Context, organizationID string) (domains.DashboardSettings, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return domains.DashboardSettings{}, err
	}
	var settings domains.DashboardSettings
	var updatedAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
SELECT
    require_alignment_before_reject,
    pause_automation_on_unknown_sender_spike,
    auto_generate_monthly_executive_reports,
    notification_channels,
    updated_at
FROM organization_dashboard_settings
WHERE organization_id = $1
`, organizationUUID).Scan(
		&settings.RequireAlignmentBeforeReject,
		&settings.PauseAutomationOnUnknownSenderSpike,
		&settings.AutoGenerateMonthlyExecutiveReports,
		&settings.NotificationChannels,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domains.DefaultDashboardSettings(), nil
	}
	if err != nil {
		return domains.DashboardSettings{}, err
	}
	settings.UpdatedAt = updatedAt.Time
	return settings, nil
}

func (s *AppStore) SaveOrganizationDashboardSettings(ctx context.Context, organizationID string, settings domains.DashboardSettings) (domains.DashboardSettings, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return domains.DashboardSettings{}, err
	}
	settings, err = domains.NormalizeDashboardSettings(settings)
	if err != nil {
		return domains.DashboardSettings{}, err
	}
	var updatedAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO organization_dashboard_settings (
    organization_id,
    require_alignment_before_reject,
    pause_automation_on_unknown_sender_spike,
    auto_generate_monthly_executive_reports,
    notification_channels
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id) DO UPDATE SET
    require_alignment_before_reject = EXCLUDED.require_alignment_before_reject,
    pause_automation_on_unknown_sender_spike = EXCLUDED.pause_automation_on_unknown_sender_spike,
    auto_generate_monthly_executive_reports = EXCLUDED.auto_generate_monthly_executive_reports,
    notification_channels = EXCLUDED.notification_channels,
    updated_at = now()
RETURNING updated_at
`, organizationUUID, settings.RequireAlignmentBeforeReject, settings.PauseAutomationOnUnknownSenderSpike, settings.AutoGenerateMonthlyExecutiveReports, settings.NotificationChannels).Scan(&updatedAt)
	if err != nil {
		return domains.DashboardSettings{}, err
	}
	settings.UpdatedAt = updatedAt.Time
	if err := s.insertOrganizationAudit(ctx, organizationUUID, pgtype.UUID{}, "dashboard.settings.updated", "dashboard_settings", organizationID, map[string]string{
		"actor":                           "api_key",
		"require_alignment_before_reject": fmt.Sprintf("%d", settings.RequireAlignmentBeforeReject),
		"notification_channels":           strings.Join(settings.NotificationChannels, ","),
	}); err != nil {
		return domains.DashboardSettings{}, err
	}
	return settings, nil
}

func (s *AppStore) insertOrganizationAudit(ctx context.Context, organizationID pgtype.UUID, domainID pgtype.UUID, action string, targetType string, targetID string, metadata map[string]string) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	domainValue := any(nil)
	if domainID.Valid {
		domainValue = domainID
	}
	_, err = s.queries.db.Exec(ctx, `
INSERT INTO audit_log (organization_id, domain_id, action, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
`, organizationID, domainValue, action, targetType, targetID, string(metadataJSON))
	return err
}

func (s *AppStore) ListOrganizationDomainHistory(ctx context.Context, organizationID string, domainID string, limit int) ([]domains.HistoryEntry, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    d.id,
    d.domain,
    c.spf_status,
    c.spf_message,
    c.spf_record,
    c.dkim_status,
    c.dkim_message,
    c.dkim_record,
    c.dmarc_status,
    c.dmarc_message,
    c.dmarc_record,
    c.bimi_status,
    c.bimi_message,
    c.bimi_record,
    c.created_at
FROM domains d
JOIN domain_checks c ON c.domain_id = d.id
WHERE d.organization_id = $1 AND d.id = $2
ORDER BY c.created_at DESC
LIMIT $3
`, organizationUUID, domainUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := []domains.HistoryEntry{}
	for rows.Next() {
		var entry domains.HistoryEntry
		var id pgtype.UUID
		var checkedAt pgtype.Timestamptz
		var spfStatus, spfMessage, spfRecord string
		var dkimStatus, dkimMessage, dkimRecord string
		var dmarcStatus, dmarcMessage, dmarcRecord string
		var bimiStatus, bimiMessage, bimiRecord string
		if err := rows.Scan(
			&id,
			&entry.Domain,
			&spfStatus,
			&spfMessage,
			&spfRecord,
			&dkimStatus,
			&dkimMessage,
			&dkimRecord,
			&dmarcStatus,
			&dmarcMessage,
			&dmarcRecord,
			&bimiStatus,
			&bimiMessage,
			&bimiRecord,
			&checkedAt,
		); err != nil {
			return nil, err
		}
		entry.DomainID = uuidToString(id)
		entry.Check = domains.Check{
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.Status(spfStatus), Message: spfMessage, Record: spfRecord},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.Status(dkimStatus), Message: dkimMessage, Record: dkimRecord},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.Status(dmarcStatus), Message: dmarcMessage, Record: dmarcRecord},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.Status(bimiStatus), Message: bimiMessage, Record: bimiRecord},
			CheckedAt: checkedAt.Time,
		}
		history = append(history, entry)
	}
	return history, rows.Err()
}

func (s *AppStore) ListOrganizationDomainDrift(ctx context.Context, organizationID string, domainID string, limit int) ([]drift.Event, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    e.check_name,
    e.kind,
    e.severity,
    e.message,
    e.before_status,
    e.after_status,
    e.before_record,
    e.after_record
FROM drift_events e
JOIN domains d ON d.id = e.domain_id
WHERE d.organization_id = $1 AND d.id = $2
ORDER BY e.created_at DESC
LIMIT $3
`, organizationUUID, domainUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []drift.Event{}
	for rows.Next() {
		var event drift.Event
		var kind string
		var severity string
		var beforeStatus string
		var afterStatus string
		if err := rows.Scan(
			&event.CheckName,
			&kind,
			&severity,
			&event.Message,
			&beforeStatus,
			&afterStatus,
			&event.BeforeRecord,
			&event.AfterRecord,
		); err != nil {
			return nil, err
		}
		event.Kind = drift.Kind(kind)
		event.Severity = drift.Severity(severity)
		event.BeforeStatus = scanner.Status(beforeStatus)
		event.AfterStatus = scanner.Status(afterStatus)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *AppStore) ListOrganizationReporterSources(ctx context.Context, organizationID string, domainID string, limit int) ([]reporter.DashboardRow, error) {
	organizationUUID, domainUUID, err := organizationDomainUUIDs(organizationID, domainID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    r.policy_domain,
    a.source_ip::text,
    a.source_org,
    a.source_asn,
    a.source_country,
    a.header_from,
    a.disposition,
    a.dkim_aligned_result,
    a.spf_aligned_result,
    COALESCE(sa.provider, '') AS provider,
    COALESCE(sa.confidence, '') AS confidence,
    a.count,
    r.begin_at,
    r.end_at
FROM dmarc_aggregate_rows a
JOIN dmarc_reports r ON r.id = a.report_id
JOIN domains d ON d.id = r.domain_id
LEFT JOIN sender_attributions sa ON sa.domain_id = d.id AND sa.source_ip = a.source_ip
WHERE d.organization_id = $1 AND d.id = $2
ORDER BY r.end_at DESC, a.count DESC
LIMIT $3
`, organizationUUID, domainUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aggregateRows := []reporter.AggregateRow{}
	for rows.Next() {
		var row reporter.AggregateRow
		if err := rows.Scan(
			&row.Domain,
			&row.SourceIP,
			&row.SourceOrg,
			&row.SourceASN,
			&row.SourceCountry,
			&row.HeaderFrom,
			&row.Disposition,
			&row.DKIMAlignedResult,
			&row.SPFAlignedResult,
			&row.Provider,
			&row.Confidence,
			&row.Count,
			&row.BeginAt,
			&row.EndAt,
		); err != nil {
			return nil, err
		}
		aggregateRows = append(aggregateRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reporter.BuildDashboardRows(aggregateRows), nil
}

func (s *AppStore) ListOrganizationReporterSenders(ctx context.Context, organizationID string, domainID string, limit int) ([]reporter.InventoryEntry, error) {
	organizationUUID, domainUUID, err := organizationDomainUUIDs(organizationID, domainID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    a.source_ip::text,
    COALESCE(sa.provider, '') AS provider,
    COALESCE(sa.confidence, 'none') AS confidence,
    sum(a.count)::integer AS volume,
    sum(CASE WHEN a.dkim_aligned_result = 'pass' OR a.spf_aligned_result = 'pass' THEN 0 ELSE a.count END)::integer AS failed_count,
    min(r.begin_at),
    max(r.end_at),
    COALESCE(sa.expected, false) AS expected,
    COALESCE(sa.ignored, false) AS ignored,
    COALESCE(sa.suspicious, false) AS suspicious
FROM dmarc_aggregate_rows a
JOIN dmarc_reports r ON r.id = a.report_id
JOIN domains d ON d.id = r.domain_id
LEFT JOIN sender_attributions sa ON sa.domain_id = d.id AND sa.source_ip = a.source_ip
WHERE d.organization_id = $1 AND d.id = $2
GROUP BY a.source_ip, sa.provider, sa.confidence, sa.expected, sa.ignored, sa.suspicious
ORDER BY volume DESC, a.source_ip
LIMIT $3
`, organizationUUID, domainUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []reporter.InventoryEntry{}
	for rows.Next() {
		var entry reporter.InventoryEntry
		var confidence string
		var failed int
		if err := rows.Scan(
			&entry.SourceIP,
			&entry.Provider,
			&confidence,
			&entry.Volume,
			&failed,
			&entry.FirstSeen,
			&entry.LastSeen,
			&entry.Expected,
			&entry.Ignored,
			&entry.Suspicious,
		); err != nil {
			return nil, err
		}
		entry.Confidence = attribution.Confidence(confidence)
		if entry.Volume > 0 {
			entry.FailureRate = float64(failed) / float64(entry.Volume)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *AppStore) GetOrganizationReporterHealth(ctx context.Context, organizationID string, domainID string) (reporter.HealthResult, error) {
	organizationUUID, domainUUID, err := organizationDomainUUIDs(organizationID, domainID)
	if err != nil {
		return reporter.HealthResult{}, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT r.org_name, r.created_at AS received_at, false AS failed, ''
FROM dmarc_reports r
JOIN domains d ON d.id = r.domain_id
WHERE d.organization_id = $1 AND d.id = $2
UNION ALL
SELECT 'ingestion', f.created_at AS received_at, true AS failed, f.reason
FROM dmarc_ingestion_failures f
JOIN domains d ON d.id = f.domain_id
WHERE d.organization_id = $1 AND d.id = $2
ORDER BY received_at DESC
LIMIT 200
`, organizationUUID, domainUUID)
	if err != nil {
		return reporter.HealthResult{}, err
	}
	defer rows.Close()

	receipts := []reporter.ReportReceipt{}
	for rows.Next() {
		var receipt reporter.ReportReceipt
		if err := rows.Scan(&receipt.OrgName, &receipt.ReceivedAt, &receipt.Failed, &receipt.Error); err != nil {
			return reporter.HealthResult{}, err
		}
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		return reporter.HealthResult{}, err
	}
	return reporter.EvaluateHealth(reporter.HealthCheck{Now: time.Now().UTC(), Receipts: receipts}), nil
}

func (s *AppStore) CreateAuditClaim(ctx context.Context, claim salesaudit.Claim) (salesaudit.Claim, error) {
	row := s.queries.db.QueryRow(ctx, `
INSERT INTO sales_audit_claims (domain, email, grade, risk, safe, proof_eligible, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at
`, claim.Domain, claim.Email, claim.Grade, string(claim.Risk), claim.Safe, claim.ProofEligible, string(claim.Status))
	var id pgtype.UUID
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &createdAt); err != nil {
		return salesaudit.Claim{}, err
	}
	claim.ID = uuidToString(id)
	claim.CreatedAt = createdAt.Time
	return claim, nil
}

func (s *AppStore) ListUserReporterSenders(ctx context.Context, userID string, domainID string, limit int) ([]reporter.InventoryEntry, error) {
	userUUID, domainUUID, err := userDomainUUIDs(userID, domainID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT
    a.source_ip::text,
    COALESCE(sa.provider, '') AS provider,
    COALESCE(sa.confidence, 'none') AS confidence,
    sum(a.count)::integer AS volume,
    sum(CASE WHEN a.dkim_aligned_result = 'pass' OR a.spf_aligned_result = 'pass' THEN 0 ELSE a.count END)::integer AS failed_count,
    min(r.begin_at),
    max(r.end_at),
    COALESCE(sa.expected, false) AS expected,
    COALESCE(sa.ignored, false) AS ignored,
    COALESCE(sa.suspicious, false) AS suspicious
FROM dmarc_aggregate_rows a
JOIN dmarc_reports r ON r.id = a.report_id
JOIN domains d ON d.id = r.domain_id
LEFT JOIN sender_attributions sa ON sa.domain_id = d.id AND sa.source_ip = a.source_ip
WHERE d.user_id = $1 AND d.id = $2
GROUP BY a.source_ip, sa.provider, sa.confidence, sa.expected, sa.ignored, sa.suspicious
ORDER BY volume DESC, a.source_ip
LIMIT $3
`, userUUID, domainUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []reporter.InventoryEntry{}
	for rows.Next() {
		var entry reporter.InventoryEntry
		var confidence string
		var failed int
		if err := rows.Scan(
			&entry.SourceIP,
			&entry.Provider,
			&confidence,
			&entry.Volume,
			&failed,
			&entry.FirstSeen,
			&entry.LastSeen,
			&entry.Expected,
			&entry.Ignored,
			&entry.Suspicious,
		); err != nil {
			return nil, err
		}
		entry.Confidence = attribution.Confidence(confidence)
		if entry.Volume > 0 {
			entry.FailureRate = float64(failed) / float64(entry.Volume)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *AppStore) GetUserReporterHealth(ctx context.Context, userID string, domainID string) (reporter.HealthResult, error) {
	userUUID, domainUUID, err := userDomainUUIDs(userID, domainID)
	if err != nil {
		return reporter.HealthResult{}, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT r.org_name, r.created_at AS received_at, false AS failed, ''
FROM dmarc_reports r
JOIN domains d ON d.id = r.domain_id
WHERE d.user_id = $1 AND d.id = $2
UNION ALL
SELECT 'ingestion', f.created_at AS received_at, true AS failed, f.reason
FROM dmarc_ingestion_failures f
JOIN domains d ON d.id = f.domain_id
WHERE d.user_id = $1 AND d.id = $2
ORDER BY received_at DESC
LIMIT 200
`, userUUID, domainUUID)
	if err != nil {
		return reporter.HealthResult{}, err
	}
	defer rows.Close()

	receipts := []reporter.ReportReceipt{}
	for rows.Next() {
		var receipt reporter.ReportReceipt
		if err := rows.Scan(&receipt.OrgName, &receipt.ReceivedAt, &receipt.Failed, &receipt.Error); err != nil {
			return reporter.HealthResult{}, err
		}
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		return reporter.HealthResult{}, err
	}
	return reporter.EvaluateHealth(reporter.HealthCheck{Now: time.Now().UTC(), Receipts: receipts}), nil
}

func (s *AppStore) ListOrganizationWebhookEndpoints(ctx context.Context, organizationID string) ([]webhooks.Endpoint, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.db.Query(ctx, `
SELECT id, url, event_types, enabled, created_at, updated_at
FROM webhook_endpoints
WHERE organization_id = $1
ORDER BY created_at DESC
`, organizationUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	endpoints := []webhooks.Endpoint{}
	for rows.Next() {
		endpoint, err := scanWebhookEndpoint(rows, organizationID)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, rows.Err()
}

func (s *AppStore) CreateOrganizationWebhookEndpoint(ctx context.Context, organizationID string, rawURL string, eventTypes []string) (webhooks.Endpoint, string, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	endpoint, secret, err := webhooks.NewEndpoint(organizationID, rawURL, eventTypes, time.Now().UTC())
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	var endpointID pgtype.UUID
	err = s.queries.db.QueryRow(ctx, `
INSERT INTO webhook_endpoints (organization_id, url, secret_hash, event_types, enabled)
VALUES ($1, $2, $3, $4, true)
RETURNING id, created_at, updated_at
`, organizationUUID, endpoint.URL, webhookSecretHash(secret), endpoint.EventTypes).Scan(&endpointID, &endpoint.CreatedAt, &endpoint.UpdatedAt)
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	endpoint.ID = uuidToString(endpointID)
	return endpoint, secret, nil
}

func (s *AppStore) RotateOrganizationWebhookSecret(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, string, error) {
	organizationUUID, endpointUUID, err := organizationDomainUUIDs(organizationID, endpointID)
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	secret, err := webhooks.RotateSecret()
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	row := s.queries.db.QueryRow(ctx, `
UPDATE webhook_endpoints
SET secret_hash = $3, updated_at = now()
WHERE organization_id = $1 AND id = $2
RETURNING id, url, event_types, enabled, created_at, updated_at
`, organizationUUID, endpointUUID, webhookSecretHash(secret))
	endpoint, err := scanWebhookEndpoint(row, organizationID)
	if err != nil {
		return webhooks.Endpoint{}, "", err
	}
	return endpoint, secret, nil
}

func (s *AppStore) DisableOrganizationWebhookEndpoint(ctx context.Context, organizationID string, endpointID string) (webhooks.Endpoint, error) {
	organizationUUID, endpointUUID, err := organizationDomainUUIDs(organizationID, endpointID)
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	row := s.queries.db.QueryRow(ctx, `
UPDATE webhook_endpoints
SET enabled = false, updated_at = now()
WHERE organization_id = $1 AND id = $2
RETURNING id, url, event_types, enabled, created_at, updated_at
`, organizationUUID, endpointUUID)
	return scanWebhookEndpoint(row, organizationID)
}

type webhookEndpointRow interface {
	Scan(dest ...any) error
}

func scanWebhookEndpoint(row webhookEndpointRow, organizationID string) (webhooks.Endpoint, error) {
	var endpoint webhooks.Endpoint
	var id pgtype.UUID
	if err := row.Scan(&id, &endpoint.URL, &endpoint.EventTypes, &endpoint.Enabled, &endpoint.CreatedAt, &endpoint.UpdatedAt); err != nil {
		return webhooks.Endpoint{}, err
	}
	endpoint.ID = uuidToString(id)
	endpoint.OrganizationID = organizationID
	return endpoint, nil
}

func webhookSecretHash(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func organizationDomainUUIDs(organizationID string, domainID string) (pgtype.UUID, pgtype.UUID, error) {
	organizationUUID, err := uuidFromString(organizationID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	return organizationUUID, domainUUID, nil
}

func userDomainUUIDs(userID string, domainID string) (pgtype.UUID, pgtype.UUID, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	return userUUID, domainUUID, nil
}

func (s *AppStore) GetDomain(ctx context.Context, userID string, domainID string) (domains.Domain, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return domains.Domain{}, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return domains.Domain{}, err
	}
	var saved domains.Domain
	var checkID pgtype.UUID
	var spfStatus, spfMessage, spfRecord string
	var dkimStatus, dkimMessage, dkimRecord string
	var dmarcStatus, dmarcMessage, dmarcRecord string
	var bimiStatus, bimiMessage, bimiRecord string
	var createdAt pgtype.Timestamptz
	var checkedAt pgtype.Timestamptz
	err = s.queries.db.QueryRow(ctx, `
SELECT
    d.domain,
    d.created_at,
    c.id AS check_id,
    COALESCE(c.spf_status, '') AS spf_status,
    COALESCE(c.spf_message, '') AS spf_message,
    COALESCE(c.spf_record, '') AS spf_record,
    COALESCE(c.dkim_status, '') AS dkim_status,
    COALESCE(c.dkim_message, '') AS dkim_message,
    COALESCE(c.dkim_record, '') AS dkim_record,
    COALESCE(c.dmarc_status, '') AS dmarc_status,
    COALESCE(c.dmarc_message, '') AS dmarc_message,
    COALESCE(c.dmarc_record, '') AS dmarc_record,
    COALESCE(c.bimi_status, '') AS bimi_status,
    COALESCE(c.bimi_message, '') AS bimi_message,
    COALESCE(c.bimi_record, '') AS bimi_record,
    c.created_at AS checked_at
FROM domains d
LEFT JOIN LATERAL (
    SELECT id, spf_status, spf_message, spf_record, dkim_status, dkim_message, dkim_record, dmarc_status, dmarc_message, dmarc_record, bimi_status, bimi_message, bimi_record, created_at
    FROM domain_checks dc
    WHERE dc.domain_id = d.id
    ORDER BY dc.created_at DESC
    LIMIT 1
) c ON true
WHERE d.id = $1 AND d.user_id = $2
`, domainUUID, userUUID).Scan(
		&saved.Domain,
		&createdAt,
		&checkID,
		&spfStatus,
		&spfMessage,
		&spfRecord,
		&dkimStatus,
		&dkimMessage,
		&dkimRecord,
		&dmarcStatus,
		&dmarcMessage,
		&dmarcRecord,
		&bimiStatus,
		&bimiMessage,
		&bimiRecord,
		&checkedAt,
	)
	if err != nil {
		return domains.Domain{}, err
	}
	saved.ID = domainID
	saved.CreatedAt = createdAt.Time
	if checkID.Valid {
		saved.LatestCheck = &domains.Check{
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.Status(spfStatus), Message: spfMessage, Record: spfRecord},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.Status(dkimStatus), Message: dkimMessage, Record: dkimRecord},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.Status(dmarcStatus), Message: dmarcMessage, Record: dmarcRecord},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.Status(bimiStatus), Message: bimiMessage, Record: bimiRecord},
			CheckedAt: checkedAt.Time,
		}
	}
	return saved, nil
}

func (s *AppStore) DeleteDomain(ctx context.Context, userID string, domainID string) error {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return err
	}
	return s.queries.DeleteDomainForUser(ctx, DeleteDomainForUserParams{ID: domainUUID, UserID: userUUID})
}

func (s *AppStore) CreateCheck(ctx context.Context, domainID string, result scanner.Result) error {
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return err
	}
	_, err = s.queries.CreateDomainCheck(ctx, CreateDomainCheckParams{
		DomainID:     domainUUID,
		SpfStatus:    string(result.SPF.Status),
		SpfMessage:   result.SPF.Message,
		SpfRecord:    result.SPF.Record,
		DkimStatus:   string(result.DKIM.Status),
		DkimMessage:  result.DKIM.Message,
		DkimRecord:   result.DKIM.Record,
		DmarcStatus:  string(result.DMARC.Status),
		DmarcMessage: result.DMARC.Message,
		DmarcRecord:  result.DMARC.Record,
		BimiStatus:   string(result.BIMI.Status),
		BimiMessage:  result.BIMI.Message,
		BimiRecord:   result.BIMI.Record,
	})
	return err
}

func domainFromParts(id pgtype.UUID, domain string, createdAt pgtype.Timestamptz) domains.Domain {
	return domains.Domain{ID: uuidToString(id), Domain: domain, CreatedAt: createdAt.Time}
}

func (s *AppStore) EnsureSPFRoute(ctx context.Context, route spfroutes.Route) (spfroutes.Route, error) {
	if strings.TrimSpace(route.DomainID) == "" {
		return spfroutes.Route{}, fmt.Errorf("domain id is required")
	}
	if strings.TrimSpace(route.RouteLabel) == "" {
		return spfroutes.Route{}, fmt.Errorf("route label is required")
	}
	if strings.TrimSpace(route.RouteFQDN) == "" {
		return spfroutes.Route{}, fmt.Errorf("route FQDN is required")
	}
	if strings.TrimSpace(route.CustomerCNAMEName) == "" {
		return spfroutes.Route{}, fmt.Errorf("customer CNAME name is required")
	}
	if strings.TrimSpace(route.CustomerCNAMETarget) == "" {
		return spfroutes.Route{}, fmt.Errorf("customer CNAME target is required")
	}
	if route.MechanismPolicy == "" {
		route.MechanismPolicy = spfroutes.PolicySoftfail
	}
	err := s.withTx(ctx, func(db DBTX) error {
		if err := db.QueryRow(ctx, `
INSERT INTO spf_routes (
    organization_id,
    domain_id,
    route_label,
    route_fqdn,
    customer_cname_name,
    customer_cname_target,
    enabled,
    mechanism_policy
) VALUES (
    COALESCE(NULLIF($1, '')::uuid, (SELECT organization_id FROM domains WHERE id = $2::uuid)),
    $2::uuid,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
ON CONFLICT (domain_id) DO UPDATE SET
    organization_id = COALESCE(EXCLUDED.organization_id, spf_routes.organization_id),
    route_label = EXCLUDED.route_label,
    route_fqdn = EXCLUDED.route_fqdn,
    customer_cname_name = EXCLUDED.customer_cname_name,
    customer_cname_target = EXCLUDED.customer_cname_target,
    enabled = EXCLUDED.enabled,
    mechanism_policy = EXCLUDED.mechanism_policy,
    updated_at = now()
RETURNING id::text
`, route.OrganizationID, route.DomainID, route.RouteLabel, route.RouteFQDN, route.CustomerCNAMEName, route.CustomerCNAMETarget, route.Enabled, string(route.MechanismPolicy)).Scan(&route.ID); err != nil {
			return err
		}
		if route.Sources != nil {
			if err := replaceSPFRouteSources(ctx, db, route.ID, route.Sources); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return spfroutes.Route{}, fmt.Errorf("ensuring SPF route: %w", err)
	}
	return s.GetSPFRoute(ctx, route.ID)
}

func (s *AppStore) GetSPFRoute(ctx context.Context, routeID string) (spfroutes.Route, error) {
	route, err := s.scanSPFRoute(ctx, `
SELECT
    r.id::text,
    COALESCE(r.organization_id::text, ''),
    r.domain_id::text,
    d.domain,
    r.route_label,
    r.route_fqdn,
    r.customer_cname_name,
    r.customer_cname_target,
    r.enabled,
    r.mechanism_policy,
    r.bunny_record_id,
    r.last_generated_spf,
    r.last_published_spf,
    r.last_sync_status,
    r.last_sync_error,
    r.last_synced_at
FROM spf_routes r
JOIN domains d ON d.id = r.domain_id
WHERE r.id = $1::uuid
`, routeID)
	if err != nil {
		return spfroutes.Route{}, err
	}
	return s.hydrateSPFRoute(ctx, route)
}

func (s *AppStore) GetSPFRouteByDomain(ctx context.Context, domainID string) (spfroutes.Route, error) {
	route, err := s.scanSPFRoute(ctx, `
SELECT
    r.id::text,
    COALESCE(r.organization_id::text, ''),
    r.domain_id::text,
    d.domain,
    r.route_label,
    r.route_fqdn,
    r.customer_cname_name,
    r.customer_cname_target,
    r.enabled,
    r.mechanism_policy,
    r.bunny_record_id,
    r.last_generated_spf,
    r.last_published_spf,
    r.last_sync_status,
    r.last_sync_error,
    r.last_synced_at
FROM spf_routes r
JOIN domains d ON d.id = r.domain_id
WHERE r.domain_id = $1::uuid
`, domainID)
	if err != nil {
		return spfroutes.Route{}, err
	}
	return s.hydrateSPFRoute(ctx, route)
}

func (s *AppStore) SaveSPFRouteSync(ctx context.Context, routeID string, update spfroutes.SyncUpdate) (spfroutes.Route, error) {
	err := s.withTx(ctx, func(db DBTX) error {
		tag, err := db.Exec(ctx, `
UPDATE spf_routes
SET last_generated_spf = $2,
    last_published_spf = $3,
    last_sync_status = $4,
    last_sync_error = $5,
    bunny_record_id = $6,
    last_synced_at = now(),
    updated_at = now()
WHERE id = $1::uuid
`, routeID, update.GeneratedSPF, update.PublishedSPF, string(update.Status), update.Error, update.BunnyRecordID)
		if err != nil {
			return fmt.Errorf("updating SPF route sync state: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("updating SPF route sync state: route not found")
		}
		if _, err := db.Exec(ctx, `DELETE FROM spf_route_evidence WHERE spf_route_id = $1::uuid`, routeID); err != nil {
			return fmt.Errorf("clearing SPF route evidence: %w", err)
		}
		for _, evidence := range update.Evidence {
			if _, err := db.Exec(ctx, `
INSERT INTO spf_route_evidence (spf_route_id, source_value, resolved_domain, mechanism, evidence_kind, message)
VALUES ($1::uuid, $2, $3, $4, $5, $6)
`, routeID, evidence.SourceValue, evidence.ResolvedDomain, evidence.Mechanism, evidence.Kind, evidence.Message); err != nil {
				return fmt.Errorf("inserting SPF route evidence: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return spfroutes.Route{}, err
	}
	return s.GetSPFRoute(ctx, routeID)
}

func (s *AppStore) ListEnabledSPFRoutes(ctx context.Context) ([]spfroutes.Route, error) {
	rows, err := s.queries.db.Query(ctx, `
SELECT id::text FROM spf_routes WHERE enabled = true ORDER BY updated_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []spfroutes.Route
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		route, err := s.GetSPFRoute(ctx, id)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (s *AppStore) replaceSPFRouteSources(ctx context.Context, routeID string, sources []spfroutes.Source) error {
	return s.withTx(ctx, func(db DBTX) error {
		return replaceSPFRouteSources(ctx, db, routeID, sources)
	})
}

func replaceSPFRouteSources(ctx context.Context, db DBTX, routeID string, sources []spfroutes.Source) error {
	if _, err := db.Exec(ctx, `DELETE FROM spf_route_sources WHERE spf_route_id = $1::uuid`, routeID); err != nil {
		return fmt.Errorf("clearing SPF route sources: %w", err)
	}
	for _, source := range sources {
		if _, err := db.Exec(ctx, `
INSERT INTO spf_route_sources (spf_route_id, kind, value, label, enabled)
VALUES ($1::uuid, $2, $3, $4, $5)
`, routeID, string(source.Kind), source.Value, source.Label, source.Enabled); err != nil {
			return fmt.Errorf("inserting SPF route source: %w", err)
		}
	}
	return nil
}

func (s *AppStore) withTx(ctx context.Context, fn func(DBTX) error) error {
	beginner, ok := s.queries.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return fn(s.queries.db)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *AppStore) scanSPFRoute(ctx context.Context, query string, args ...any) (spfroutes.Route, error) {
	var route spfroutes.Route
	var lastSyncedAt pgtype.Timestamptz
	err := s.queries.db.QueryRow(ctx, query, args...).Scan(
		&route.ID,
		&route.OrganizationID,
		&route.DomainID,
		&route.Domain,
		&route.RouteLabel,
		&route.RouteFQDN,
		&route.CustomerCNAMEName,
		&route.CustomerCNAMETarget,
		&route.Enabled,
		&route.MechanismPolicy,
		&route.BunnyRecordID,
		&route.LastGeneratedSPF,
		&route.LastPublishedSPF,
		&route.LastSyncStatus,
		&route.LastSyncError,
		&lastSyncedAt,
	)
	if err != nil {
		return spfroutes.Route{}, err
	}
	if lastSyncedAt.Valid {
		route.LastSyncedAt = lastSyncedAt.Time
	}
	return route, nil
}

func (s *AppStore) hydrateSPFRoute(ctx context.Context, route spfroutes.Route) (spfroutes.Route, error) {
	sources, err := s.queries.db.Query(ctx, `
SELECT id::text, kind, value, label, enabled
FROM spf_route_sources
WHERE spf_route_id = $1::uuid
ORDER BY created_at ASC, id ASC
`, route.ID)
	if err != nil {
		return spfroutes.Route{}, err
	}
	defer sources.Close()
	for sources.Next() {
		var source spfroutes.Source
		if err := sources.Scan(&source.ID, &source.Kind, &source.Value, &source.Label, &source.Enabled); err != nil {
			return spfroutes.Route{}, err
		}
		route.Sources = append(route.Sources, source)
	}
	if err := sources.Err(); err != nil {
		return spfroutes.Route{}, err
	}

	evidenceRows, err := s.queries.db.Query(ctx, `
SELECT source_value, resolved_domain, mechanism, evidence_kind, message
FROM spf_route_evidence
WHERE spf_route_id = $1::uuid
ORDER BY created_at ASC, id ASC
`, route.ID)
	if err != nil {
		return spfroutes.Route{}, err
	}
	defer evidenceRows.Close()
	for evidenceRows.Next() {
		var evidence spfroutes.Evidence
		if err := evidenceRows.Scan(&evidence.SourceValue, &evidence.ResolvedDomain, &evidence.Mechanism, &evidence.Kind, &evidence.Message); err != nil {
			return spfroutes.Route{}, err
		}
		route.Evidence = append(route.Evidence, evidence)
	}
	return route, evidenceRows.Err()
}

func uuidFromString(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parsing uuid %q: %w", value, err)
	}
	return id, nil
}

func uuidToString(value pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", value.Bytes[0:4], value.Bytes[4:6], value.Bytes[6:8], value.Bytes[8:10], value.Bytes[10:16])
}
