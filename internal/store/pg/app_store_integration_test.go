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

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AndriGitDev/meil/internal/apikeys"
	"github.com/AndriGitDev/meil/internal/auth"
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
		SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusWarn, Message: "SPF warning", Record: "v=spf1 ~all"},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
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
		SPF:   scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF pass", Record: "v=spf1 -all"},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM pass"},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Message: "DMARC pass"},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
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
	domainWithCheck, err := store.GetDomain(ctx, user.ID, domain.ID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}
	if domainWithCheck.LatestCheck == nil || domainWithCheck.LatestCheck.SPF.Status != scanner.StatusPass {
		t.Fatalf("get domain latest check = %+v, want SPF pass", domainWithCheck.LatestCheck)
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
