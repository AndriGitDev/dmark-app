package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/ops"
	"github.com/AndriGitDev/meil/internal/salesaudit"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/spfroutes"
)

func TestPostgresMigrationsAndAuthDomainStoreIntegration(t *testing.T) {
	if os.Getenv("MEIL_DB_INTEGRATION") != "1" {
		t.Skip("set MEIL_DB_INTEGRATION=1 and DATABASE_URL to run Postgres integration tests")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required for Postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set migration dialect: %v", err)
	}
	migrationDir := filepath.Join("..", "..", "..", "migrations", "postgres")
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect pgx pool: %v", err)
	}
	defer pool.Close()

	store := NewAppStore(New(pool))
	operationalEvent, err := store.CreateOperationalEvent(ctx, ops.Event{
		Kind:       "integration.provider_failed",
		Severity:   ops.SeverityError,
		Message:    "provider failed during integration test",
		Actor:      "test",
		Resource:   "bunny",
		Metadata:   map[string]any{"provider": "bunny", "api_token": "should-redact"},
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create operational event: %v", err)
	}
	if operationalEvent.ID == "" || operationalEvent.Metadata["api_token"] != "[REDACTED]" {
		t.Fatalf("operational event = %+v, want id and redacted metadata", operationalEvent)
	}
	operationalEvents, err := store.ListOperationalEvents(ctx, string(ops.SeverityError), 1)
	if err != nil {
		t.Fatalf("list operational events: %v", err)
	}
	if len(operationalEvents) != 1 || operationalEvents[0].ID != operationalEvent.ID || operationalEvents[0].Metadata["api_token"] != "[REDACTED]" {
		t.Fatalf("operational events = %+v, want created redacted event", operationalEvents)
	}
	token, _, err := apikeys.New()
	if err != nil {
		t.Fatalf("new bootstrap API key: %v", err)
	}
	bootstrapEmail := fmt.Sprintf("bootstrap-%d@example.com", time.Now().UnixNano())
	bootstrapResult, err := store.EnsureBootstrapOrganizationAPIKey(ctx, bootstrapEmail, "DMARK Bootstrap Test", "frontend-proxy", token, []apikeys.Scope{apikeys.ScopeDomainRead, apikeys.ScopeDomainWrite})
	if err != nil {
		t.Fatalf("ensure bootstrap: %v", err)
	}
	if !bootstrapResult.Applied || !bootstrapResult.APIKeyCreated || bootstrapResult.UserID == "" || bootstrapResult.OrganizationID == "" {
		t.Fatalf("bootstrap result = %+v, want applied created ids", bootstrapResult)
	}
	principal, err := store.AuthenticateAPIKey(ctx, token)
	if err != nil {
		t.Fatalf("authenticate bootstrap API key: %v", err)
	}
	if principal.OrganizationID != bootstrapResult.OrganizationID || !apikeys.Allows(principal, apikeys.ScopeDomainWrite) {
		t.Fatalf("bootstrap principal = %+v, want org %s domain write", principal, bootstrapResult.OrganizationID)
	}
	bootstrapAgain, err := store.EnsureBootstrapOrganizationAPIKey(ctx, bootstrapEmail, "DMARK Bootstrap Test", "frontend-proxy", token, []apikeys.Scope{apikeys.ScopeDomainRead})
	if err != nil {
		t.Fatalf("ensure bootstrap idempotently: %v", err)
	}
	if bootstrapAgain.UserID != bootstrapResult.UserID || bootstrapAgain.OrganizationID != bootstrapResult.OrganizationID || bootstrapAgain.APIKeyCreated {
		t.Fatalf("second bootstrap result = %+v, want same ids and existing API key", bootstrapAgain)
	}

	suffix := time.Now().UnixNano()
	userEmail := fmt.Sprintf("pg-it-%d@example.com", suffix)
	otherEmail := fmt.Sprintf("pg-it-other-%d@example.com", suffix)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM sales_audit_claims WHERE email = ANY($1)`, []string{userEmail, otherEmail})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE email = ANY($1)`, []string{userEmail, otherEmail})
	}()

	user, err := store.UpsertUserByEmail(ctx, userEmail)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	sameUser, err := store.UpsertUserByEmail(ctx, userEmail)
	if err != nil {
		t.Fatalf("upsert same user: %v", err)
	}
	if sameUser.ID != user.ID {
		t.Fatalf("upsert user id = %q, want original %q", sameUser.ID, user.ID)
	}
	otherUser, err := store.UpsertUserByEmail(ctx, otherEmail)
	if err != nil {
		t.Fatalf("upsert other user: %v", err)
	}

	validHash := []byte(fmt.Sprintf("valid-%d", suffix))
	if err := store.CreateLoginToken(ctx, user.ID, validHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create login token: %v", err)
	}
	consumed, err := store.ConsumeLoginToken(ctx, validHash)
	if err != nil {
		t.Fatalf("consume login token: %v", err)
	}
	if consumed.UserID != user.ID || consumed.Email != userEmail {
		t.Fatalf("consumed token = %+v, want user %s email %s", consumed, user.ID, userEmail)
	}
	if _, err := store.ConsumeLoginToken(ctx, validHash); !errors.Is(err, auth.ErrConsumedToken) {
		t.Fatalf("consume used token error = %v, want %v", err, auth.ErrConsumedToken)
	}

	expiredHash := []byte(fmt.Sprintf("expired-%d", suffix))
	if err := store.CreateLoginToken(ctx, user.ID, expiredHash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := store.ConsumeLoginToken(ctx, expiredHash); !errors.Is(err, auth.ErrExpiredToken) {
		t.Fatalf("consume expired token error = %v, want %v", err, auth.ErrExpiredToken)
	}

	domain, err := store.CreateDomain(ctx, user.ID, "example.com")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	duplicate, err := store.CreateDomain(ctx, user.ID, "example.com")
	if err != nil {
		t.Fatalf("create duplicate domain: %v", err)
	}
	if duplicate.ID != domain.ID {
		t.Fatalf("duplicate domain id = %q, want original %q", duplicate.ID, domain.ID)
	}
	otherDomain, err := store.CreateDomain(ctx, otherUser.ID, "example.com")
	if err != nil {
		t.Fatalf("create other user's domain: %v", err)
	}
	if otherDomain.ID == domain.ID {
		t.Fatalf("cross-user domain reused id %q", domain.ID)
	}

	if err := store.CreateCheck(ctx, domain.ID, scanner.Result{
		Domain:    "example.com",
		CheckedAt: time.Date(2026, 6, 2, 10, 30, 0, 0, time.UTC),
		Duration:  31 * time.Millisecond,
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusWarn, Message: "SPF warning", Record: "v=spf1 include:secret-mailer.example ~all", SourceName: "example.com", LookupDuration: 3 * time.Millisecond},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing", Selectors: []scanner.SelectorResult{{Selector: "default", Record: "v=DKIM1; p=SECRET", Status: scanner.StatusMissing}}},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	}); err != nil {
		t.Fatalf("create first check: %v", err)
	}

	spfRoute, err := store.EnsureSPFRoute(ctx, spfroutes.Route{
		DomainID:            domain.ID,
		RouteLabel:          "example-com",
		RouteFQDN:           "example-com.route.dmark.is",
		CustomerCNAMEName:   "_spf.example.com",
		CustomerCNAMETarget: "example-com.route.dmark.is",
		Enabled:             true,
		MechanismPolicy:     spfroutes.PolicySoftfail,
		Sources: []spfroutes.Source{
			{Kind: spfroutes.SourceKindInclude, Value: "_spf.mailer.example", Label: "Mailer", Enabled: true},
			{Kind: spfroutes.SourceKindIP4, Value: "198.51.100.5", Label: "Static gateway", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("ensure SPF route: %v", err)
	}
	if spfRoute.ID == "" || spfRoute.DomainID != domain.ID || len(spfRoute.Sources) != 2 {
		t.Fatalf("SPF route = %+v, want persisted route with sources", spfRoute)
	}
	spfRoute, err = store.SaveSPFRouteSync(ctx, spfRoute.ID, spfroutes.SyncUpdate{
		GeneratedSPF:  "v=spf1 ip4:198.51.100.5 ~all",
		PublishedSPF:  "v=spf1 ip4:198.51.100.5 ~all",
		Status:        spfroutes.StatusHealthy,
		BunnyRecordID: 42,
		Evidence: []spfroutes.Evidence{{
			SourceValue:    "_spf.mailer.example",
			ResolvedDomain: "_spf.mailer.example",
			Mechanism:      "ip4:198.51.100.5",
			Kind:           "ip4",
			Message:        "resolved ip4 mechanism from SPF include",
		}},
	})
	if err != nil {
		t.Fatalf("save SPF route sync: %v", err)
	}
	if spfRoute.LastSyncStatus != spfroutes.StatusHealthy || spfRoute.BunnyRecordID != 42 || len(spfRoute.Evidence) != 1 {
		t.Fatalf("synced SPF route = %+v, want healthy record 42 with evidence", spfRoute)
	}
	time.Sleep(10 * time.Millisecond)
	if err := store.CreateCheck(ctx, domain.ID, scanner.Result{
		Domain:    "example.com",
		CheckedAt: time.Date(2026, 6, 2, 10, 31, 0, 0, time.UTC),
		Duration:  17 * time.Millisecond,
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF pass", Record: "v=spf1 -all", SourceName: "example.com", LookupDuration: 2 * time.Millisecond},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM pass"},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC pass"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	}); err != nil {
		t.Fatalf("create latest check: %v", err)
	}

	domains, err := store.ListDomains(ctx, user.ID)
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("list domains length = %d, want 1: %+v", len(domains), domains)
	}
	if domains[0].LatestCheck == nil || domains[0].LatestCheck.SPF.Status != scanner.StatusPass {
		t.Fatalf("latest check = %+v, want SPF pass", domains[0].LatestCheck)
	}
	if domains[0].LatestCheck.EvidenceSnapshot.Domain != "example.com" || domains[0].LatestCheck.EvidenceSnapshot.DurationMS != 17 {
		t.Fatalf("list latest evidence snapshot = %+v, want persisted snapshot", domains[0].LatestCheck.EvidenceSnapshot)
	}
	if len(domains[0].LatestCheck.EvidenceSnapshot.Checks) == 0 || !domains[0].LatestCheck.EvidenceSnapshot.Checks[0].RecordPresent {
		t.Fatalf("list latest evidence checks = %+v, want record presence", domains[0].LatestCheck.EvidenceSnapshot.Checks)
	}
	domainWithCheck, err := store.GetDomain(ctx, user.ID, domain.ID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}
	if domainWithCheck.LatestCheck == nil || domainWithCheck.LatestCheck.SPF.Status != scanner.StatusPass {
		t.Fatalf("get domain latest check = %+v, want SPF pass", domainWithCheck.LatestCheck)
	}
	if domainWithCheck.LatestCheck.EvidenceSnapshot.Domain != "example.com" || domainWithCheck.LatestCheck.EvidenceSnapshot.DurationMS != 17 {
		t.Fatalf("get latest evidence snapshot = %+v, want persisted snapshot", domainWithCheck.LatestCheck.EvidenceSnapshot)
	}
	claim, err := store.CreateAuditClaim(ctx, salesaudit.Claim{
		Domain:        "example.com",
		Email:         userEmail,
		Grade:         "B",
		Risk:          salesaudit.RiskMedium,
		ProofEligible: true,
		Status:        salesaudit.ClaimMagicLinkSent,
	})
	if err != nil {
		t.Fatalf("create audit claim: %v", err)
	}
	if claim.ID == "" || claim.CreatedAt.IsZero() {
		t.Fatalf("audit claim missing persisted fields: %+v", claim)
	}

	if err := store.DeleteDomain(ctx, otherUser.ID, domain.ID); err != nil {
		t.Fatalf("cross-user delete domain: %v", err)
	}
	domains, err = store.ListDomains(ctx, user.ID)
	if err != nil {
		t.Fatalf("list after cross-user delete: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("cross-user delete removed domain: %+v", domains)
	}
	if err := store.DeleteDomain(ctx, user.ID, domain.ID); err != nil {
		t.Fatalf("delete own domain: %v", err)
	}
	domains, err = store.ListDomains(ctx, user.ID)
	if err != nil {
		t.Fatalf("list after own delete: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("own delete left domains: %+v", domains)
	}

	if err := store.CreateLoginToken(ctx, otherUser.ID, []byte(fmt.Sprintf("cascade-%d", suffix)), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create cascade token: %v", err)
	}
	otherUserID, err := uuidFromString(otherUser.ID)
	if err != nil {
		t.Fatalf("parse other user id: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUserID); err != nil {
		t.Fatalf("delete user for cascade check: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM login_tokens WHERE user_id = $1`, otherUserID, 0)
	assertCount(t, ctx, pool, `SELECT count(*) FROM domains WHERE user_id = $1`, otherUserID, 0)
}

func TestPostgresDashboardEnrichmentStoreIntegration(t *testing.T) {
	if os.Getenv("MEIL_DB_INTEGRATION") != "1" {
		t.Skip("set MEIL_DB_INTEGRATION=1 and DATABASE_URL to run Postgres integration tests")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required for Postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set migration dialect: %v", err)
	}
	migrationDir := filepath.Join("..", "..", "..", "migrations", "postgres")
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect pgx pool: %v", err)
	}
	defer pool.Close()

	store := NewAppStore(New(pool))
	suffix := time.Now().UnixNano()
	token, _, err := apikeys.New()
	if err != nil {
		t.Fatalf("new bootstrap API key: %v", err)
	}
	bootstrapEmail := fmt.Sprintf("enrichment-%d@example.com", suffix)
	bootstrap, err := store.EnsureBootstrapOrganizationAPIKey(ctx, bootstrapEmail, fmt.Sprintf("Enrichment Test %d", suffix), "enrichment-test", token, []apikeys.Scope{apikeys.ScopeDomainRead, apikeys.ScopeDomainWrite})
	if err != nil {
		t.Fatalf("ensure bootstrap: %v", err)
	}
	organizationUUID, err := uuidFromString(bootstrap.OrganizationID)
	if err != nil {
		t.Fatalf("parse organization id: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE email = $1`, bootstrapEmail)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizations WHERE id = $1`, organizationUUID)
	}()

	domainName := fmt.Sprintf("enrichment-%d.example", suffix)
	domain, err := store.CreateOrganizationDomain(ctx, bootstrap.OrganizationID, domainName)
	if err != nil {
		t.Fatalf("create organization domain: %v", err)
	}
	domainUUID, err := uuidFromString(domain.ID)
	if err != nil {
		t.Fatalf("parse domain id: %v", err)
	}

	if err := store.CreateCheck(ctx, domain.ID, scanner.Result{
		Domain:    domainName,
		CheckedAt: time.Now().UTC().Add(-24 * time.Hour),
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusWarn, Message: "SPF warning"},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	}); err != nil {
		t.Fatalf("create older check: %v", err)
	}
	// Backdate the first check one day so the timeline spans two day buckets.
	if _, err := pool.Exec(ctx, `UPDATE domain_checks SET created_at = created_at - interval '1 day' WHERE domain_id = $1`, domainUUID); err != nil {
		t.Fatalf("backdate older check: %v", err)
	}
	if err := store.CreateCheck(ctx, domain.ID, scanner.Result{
		Domain:    domainName,
		CheckedAt: time.Now().UTC(),
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF pass"},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM pass"},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC pass", Record: "v=DMARC1; p=reject"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	}); err != nil {
		t.Fatalf("create latest check: %v", err)
	}

	timeline, err := store.ListOrganizationDomainCheckTimeline(ctx, bootstrap.OrganizationID, domain.ID, 30)
	if err != nil {
		t.Fatalf("list check timeline: %v", err)
	}
	if len(timeline) != 2 {
		t.Fatalf("timeline length = %d, want 2: %+v", len(timeline), timeline)
	}
	if timeline[0].Check.SPF.Status != scanner.StatusWarn || timeline[1].Check.SPF.Status != scanner.StatusPass {
		t.Fatalf("timeline order mismatch: first=%+v second=%+v", timeline[0].Check.SPF, timeline[1].Check.SPF)
	}
	if !timeline[0].Check.CheckedAt.Before(timeline[1].Check.CheckedAt) {
		t.Fatalf("timeline not ascending: %v then %v", timeline[0].Check.CheckedAt, timeline[1].Check.CheckedAt)
	}

	var reportUUID pgtype.UUID
	if err := pool.QueryRow(ctx, `
INSERT INTO dmarc_reports (domain_id, org_name, report_id, begin_at, end_at, policy_domain, content_hash, raw_xml)
VALUES ($1, 'google.com', $2, now() - interval '1 day', now(), $3, $4, ''::bytea)
RETURNING id
`, domainUUID, fmt.Sprintf("report-%d", suffix), domainName, fmt.Sprintf("hash-%d", suffix)).Scan(&reportUUID); err != nil {
		t.Fatalf("insert dmarc report: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO dmarc_aggregate_rows (report_id, source_ip, count, disposition, dkim_aligned_result, spf_aligned_result, header_from)
VALUES
    ($1, '203.0.113.10', 150, 'none', 'pass', 'fail', $2),
    ($1, '203.0.113.11', 50, 'none', 'fail', 'fail', $2)
`, reportUUID, domainName); err != nil {
		t.Fatalf("insert aggregate rows: %v", err)
	}

	alignment, err := store.ListOrganizationDomainAlignmentDaily(ctx, bootstrap.OrganizationID, domain.ID, 30)
	if err != nil {
		t.Fatalf("list alignment daily: %v", err)
	}
	if len(alignment) != 1 {
		t.Fatalf("alignment length = %d, want 1: %+v", len(alignment), alignment)
	}
	if alignment[0].Total != 200 || alignment[0].Aligned != 150 {
		t.Fatalf("alignment day = %+v, want total 200 aligned 150", alignment[0])
	}

	resource := fmt.Sprintf("spf_route:route-%d", suffix)
	if _, err := store.CreateOperationalEvent(ctx, ops.Event{
		Kind:     "dashboard.spf_route.sync_queued",
		Severity: ops.SeverityInfo,
		Message:  "sync queued",
		Actor:    "test",
		Resource: resource,
		Metadata: map[string]any{"job_id": "job-1"},
	}); err != nil {
		t.Fatalf("create resource event: %v", err)
	}
	if _, err := store.CreateOperationalEvent(ctx, ops.Event{
		Kind:     "dashboard.spf_route.sync_queued",
		Severity: ops.SeverityInfo,
		Message:  "other resource",
		Actor:    "test",
		Resource: fmt.Sprintf("spf_route:other-%d", suffix),
	}); err != nil {
		t.Fatalf("create other resource event: %v", err)
	}
	resourceEvents, err := store.ListOperationalEventsByResource(ctx, resource, 10)
	if err != nil {
		t.Fatalf("list events by resource: %v", err)
	}
	if len(resourceEvents) != 1 || resourceEvents[0].Resource != resource || resourceEvents[0].Metadata["job_id"] != "job-1" {
		t.Fatalf("resource events = %+v, want single matching event", resourceEvents)
	}

	plan, err := store.GetOrganizationPlan(ctx, bootstrap.OrganizationID)
	if err != nil {
		t.Fatalf("get organization plan: %v", err)
	}
	// Bootstrap provisioning writes the legacy 'pro' plan, which maps to
	// business-tier entitlements.
	if plan != "pro" {
		t.Fatalf("plan = %q, want pro", plan)
	}
	count, err := store.CountOrganizationDomains(ctx, bootstrap.OrganizationID)
	if err != nil {
		t.Fatalf("count organization domains: %v", err)
	}
	if count != 1 {
		t.Fatalf("domain count = %d, want 1", count)
	}

	// Cross-organization reads must return nothing.
	otherToken, _, err := apikeys.New()
	if err != nil {
		t.Fatalf("new other bootstrap API key: %v", err)
	}
	otherEmail := fmt.Sprintf("enrichment-other-%d@example.com", suffix)
	otherBootstrap, err := store.EnsureBootstrapOrganizationAPIKey(ctx, otherEmail, fmt.Sprintf("Enrichment Other %d", suffix), "enrichment-other", otherToken, []apikeys.Scope{apikeys.ScopeDomainRead})
	if err != nil {
		t.Fatalf("ensure other bootstrap: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE email = $1`, otherEmail)
		if otherUUID, err := uuidFromString(otherBootstrap.OrganizationID); err == nil {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizations WHERE id = $1`, otherUUID)
		}
	}()
	crossTimeline, err := store.ListOrganizationDomainCheckTimeline(ctx, otherBootstrap.OrganizationID, domain.ID, 30)
	if err != nil {
		t.Fatalf("cross-org timeline: %v", err)
	}
	crossAlignment, err := store.ListOrganizationDomainAlignmentDaily(ctx, otherBootstrap.OrganizationID, domain.ID, 30)
	if err != nil {
		t.Fatalf("cross-org alignment: %v", err)
	}
	if len(crossTimeline) != 0 || len(crossAlignment) != 0 {
		t.Fatalf("cross-org leak: timeline=%+v alignment=%+v", crossTimeline, crossAlignment)
	}
}

func assertCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, arg any, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, query, arg).Scan(&got); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d for %s", got, want, query)
	}
}
