package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AndriGitDev/meil/internal/apikeys"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func defaultDatabaseURL() string {
	return "postgres://meil:" + "meil" + "@localhost:5432/meil?sslmode=disable"
}

func run(ctx context.Context) error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	token, hash, err := apikeys.New()
	if err != nil {
		return fmt.Errorf("creating API key: %w", err)
	}
	var organizationID string
	if err := pool.QueryRow(ctx, `
INSERT INTO organizations (name, plan)
VALUES ('Dmark Demo', 'pro')
RETURNING id::text
`).Scan(&organizationID); err != nil {
		return fmt.Errorf("creating organization: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO api_keys (organization_id, name, token_hash, scopes)
VALUES ($1, 'dmark-web local dashboard', $2, $3)
`, organizationID, hash, []string{"domain:read", "domain:write", "reporter:read"}); err != nil {
		return fmt.Errorf("creating API key row: %w", err)
	}

	var domainID string
	if err := pool.QueryRow(ctx, `
INSERT INTO domains (organization_id, domain)
VALUES ($1, 'example.com')
RETURNING id::text
`, organizationID).Scan(&domainID); err != nil {
		return fmt.Errorf("creating domain: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO domain_checks (
    domain_id,
    spf_status,
    spf_message,
    spf_record,
    dkim_status,
    dkim_message,
    dkim_record,
    dmarc_status,
    dmarc_message,
    dmarc_record,
    bimi_status,
    bimi_message,
    bimi_record
) VALUES (
    $1,
    'pass',
    'SPF record is present.',
    'v=spf1 include:_spf.google.com -all',
    'pass',
    'DKIM selector responded.',
    'v=DKIM1; k=rsa; p=PUBLICKEY',
    'pass',
    'DMARC reject policy is present.',
    'v=DMARC1; p=reject; rua=mailto:dmarc@example.com',
    'missing',
    'BIMI record was not found.',
    ''
)
`, domainID); err != nil {
		return fmt.Errorf("creating domain check: %w", err)
	}

	var reportID string
	if err := pool.QueryRow(ctx, `
INSERT INTO dmarc_reports (
    domain_id,
    org_name,
    report_id,
    begin_at,
    end_at,
    policy_domain,
    policy_p,
    content_hash,
    raw_xml
) VALUES ($1, 'google.com', 'demo-report-1', now() - interval '1 day', now(), 'example.com', 'reject', 'demo-report-1', '<feedback />'::bytea)
RETURNING id::text
`, domainID).Scan(&reportID); err != nil {
		return fmt.Errorf("creating DMARC report: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO dmarc_aggregate_rows (
    report_id,
    source_ip,
    source_org,
    source_asn,
    source_country,
    count,
    disposition,
    dkim_aligned_result,
    spf_aligned_result,
    header_from
) VALUES
    ($1, '203.0.113.10', 'Google', 'AS15169', 'US', 900, 'none', 'pass', 'pass', 'example.com'),
    ($1, '198.51.100.25', 'Unknown', 'AS64500', 'IS', 30, 'none', 'fail', 'fail', 'example.com')
`, reportID); err != nil {
		return fmt.Errorf("creating aggregate rows: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO sender_attributions (
    domain_id,
    source_ip,
    provider,
    confidence,
    first_seen_at,
    last_seen_at,
    expected,
    suspicious
) VALUES
    ($1, '203.0.113.10', 'Google Workspace', 'high', now() - interval '30 days', now(), true, false),
    ($1, '198.51.100.25', '', 'none', now() - interval '1 day', now(), false, true)
`, domainID); err != nil {
		return fmt.Errorf("creating sender attribution: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO drift_events (
    domain_id,
    check_name,
    kind,
    severity,
    message,
    before_status,
    after_status,
    before_record,
    after_record
) VALUES ($1, 'DMARC', 'record_changed', 'critical', 'DMARC policy changed during demo seed.', 'pass', 'warn', 'v=DMARC1; p=reject', 'v=DMARC1; p=none')
`, domainID); err != nil {
		return fmt.Errorf("creating alert: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO organization_dashboard_settings (
    organization_id,
    require_alignment_before_reject,
    pause_automation_on_unknown_sender_spike,
    auto_generate_monthly_executive_reports,
    notification_channels
) VALUES ($1, 95, true, true, '{email,webhook}')
`, organizationID); err != nil {
		return fmt.Errorf("creating dashboard settings: %w", err)
	}

	fmt.Printf("organization_id=%s\n", organizationID)
	fmt.Printf("domain_id=%s\n", domainID)
	fmt.Printf("api_token=%s\n", token)
	return nil
}
