-- +goose Up
CREATE TABLE domain_settings (
    domain_id uuid PRIMARY KEY REFERENCES domains(id) ON DELETE CASCADE,
    notification_email text,
    dkim_selectors text[] NOT NULL DEFAULT '{}',
    expected_senders text[] NOT NULL DEFAULT '{}',
    ignored_findings text[] NOT NULL DEFAULT '{}',
    scan_cadence interval NOT NULL DEFAULT interval '24 hours',
    ownership_challenge text NOT NULL DEFAULT '',
    ownership_verified_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE scan_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    idempotency_key text NOT NULL UNIQUE,
    scheduled_for timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'queued',
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX scan_jobs_status_scheduled_for_idx ON scan_jobs(status, scheduled_for);
CREATE INDEX scan_jobs_domain_id_idx ON scan_jobs(domain_id);

CREATE TABLE job_dead_letters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    idempotency_key text NOT NULL,
    args jsonb NOT NULL,
    last_error text NOT NULL,
    attempts integer NOT NULL,
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, idempotency_key)
);

CREATE INDEX job_dead_letters_created_at_idx ON job_dead_letters(created_at DESC);

CREATE TABLE drift_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    before_check_id uuid REFERENCES domain_checks(id) ON DELETE SET NULL,
    after_check_id uuid REFERENCES domain_checks(id) ON DELETE SET NULL,
    check_name text NOT NULL,
    kind text NOT NULL,
    severity text NOT NULL,
    message text NOT NULL,
    before_status text NOT NULL DEFAULT '',
    after_status text NOT NULL DEFAULT '',
    before_record text NOT NULL DEFAULT '',
    after_record text NOT NULL DEFAULT '',
    acknowledged_at timestamptz,
    muted_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX drift_events_domain_id_created_at_idx ON drift_events(domain_id, created_at DESC);
CREATE INDEX drift_events_unacknowledged_idx ON drift_events(domain_id, severity, created_at DESC) WHERE acknowledged_at IS NULL;

CREATE TABLE notification_dispatches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    drift_event_id uuid REFERENCES drift_events(id) ON DELETE SET NULL,
    channel text NOT NULL,
    recipient text NOT NULL,
    subject text NOT NULL,
    status text NOT NULL,
    provider_message_id text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notification_dispatches_domain_id_created_at_idx ON notification_dispatches(domain_id, created_at DESC);
CREATE UNIQUE INDEX notification_dispatches_event_channel_recipient_idx ON notification_dispatches(drift_event_id, channel, recipient) WHERE drift_event_id IS NOT NULL;

CREATE TABLE dmarc_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    org_name text NOT NULL,
    report_id text NOT NULL,
    email text NOT NULL DEFAULT '',
    begin_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    policy_domain text NOT NULL,
    policy_adkim text NOT NULL DEFAULT '',
    policy_aspf text NOT NULL DEFAULT '',
    policy_p text NOT NULL DEFAULT '',
    policy_sp text NOT NULL DEFAULT '',
    policy_pct integer NOT NULL DEFAULT 100,
    content_hash text NOT NULL,
    source_file text NOT NULL DEFAULT '',
    raw_xml bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (domain_id, org_name, report_id, content_hash)
);

CREATE INDEX dmarc_reports_domain_id_begin_at_idx ON dmarc_reports(domain_id, begin_at DESC);

CREATE TABLE dmarc_ingestion_failures (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid REFERENCES domains(id) ON DELETE SET NULL,
    source_file text NOT NULL DEFAULT '',
    content_hash text NOT NULL,
    reason text NOT NULL,
    detail text NOT NULL DEFAULT '',
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (content_hash, reason)
);

CREATE INDEX dmarc_ingestion_failures_created_at_idx ON dmarc_ingestion_failures(created_at DESC);

CREATE TABLE dmarc_aggregate_rows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES dmarc_reports(id) ON DELETE CASCADE,
    source_ip inet NOT NULL,
    source_org text NOT NULL DEFAULT '',
    source_asn text NOT NULL DEFAULT '',
    source_country text NOT NULL DEFAULT '',
    count integer NOT NULL,
    disposition text NOT NULL,
    dkim_aligned_result text NOT NULL,
    spf_aligned_result text NOT NULL,
    header_from text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dmarc_aggregate_rows_report_id_idx ON dmarc_aggregate_rows(report_id);
CREATE INDEX dmarc_aggregate_rows_source_ip_idx ON dmarc_aggregate_rows(source_ip);

CREATE TABLE sender_attributions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    source_ip inet NOT NULL,
    reverse_dns text NOT NULL DEFAULT '',
    asn text NOT NULL DEFAULT '',
    provider text NOT NULL DEFAULT '',
    confidence text NOT NULL,
    evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
    first_seen_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expected boolean NOT NULL DEFAULT false,
    ignored boolean NOT NULL DEFAULT false,
    suspicious boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (domain_id, source_ip)
);

CREATE INDEX sender_attributions_domain_id_last_seen_at_idx ON sender_attributions(domain_id, last_seen_at DESC);

CREATE TABLE enrichment_cache (
    source_ip inet PRIMARY KEY,
    reverse_dns text NOT NULL DEFAULT '',
    asn text NOT NULL DEFAULT '',
    country_code text NOT NULL DEFAULT '',
    provider_hint text NOT NULL DEFAULT '',
    confidence text NOT NULL DEFAULT 'none',
    evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
    refreshed_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX enrichment_cache_expires_at_idx ON enrichment_cache(expires_at);

CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    plan text NOT NULL DEFAULT 'free',
    billing_customer_id text NOT NULL DEFAULT '',
    billing_subscription_id text NOT NULL DEFAULT '',
    billing_last_event_id text NOT NULL DEFAULT '',
    billing_status text NOT NULL DEFAULT '',
    billing_cancel_requested_at timestamptz,
    billing_cancel_effective_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE domains
ADD COLUMN organization_id uuid REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE domains
ALTER COLUMN user_id DROP NOT NULL;

CREATE INDEX domains_organization_id_idx ON domains(organization_id);
CREATE UNIQUE INDEX domains_organization_domain_idx ON domains(organization_id, domain) WHERE organization_id IS NOT NULL;

CREATE TABLE organization_members (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE organization_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email text NOT NULL,
    role text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE domain_transfers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    from_organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    to_organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    requested_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending',
    decided_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    decided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX domain_transfers_to_organization_status_idx ON domain_transfers(to_organization_id, status, created_at DESC);

CREATE TABLE api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    scopes text[] NOT NULL DEFAULT '{}',
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE webhook_endpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url text NOT NULL,
    secret_hash bytea NOT NULL,
    event_types text[] NOT NULL DEFAULT '{}',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX webhook_endpoints_organization_id_idx ON webhook_endpoints(organization_id);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_id text NOT NULL,
    event_type text NOT NULL,
    status text NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (endpoint_id, event_id)
);

CREATE INDEX webhook_deliveries_endpoint_id_created_at_idx ON webhook_deliveries(endpoint_id, created_at DESC);

CREATE TABLE audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    domain_id uuid REFERENCES domains(id) ON DELETE SET NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_organization_id_created_at_idx ON audit_log(organization_id, created_at DESC);
CREATE INDEX audit_log_user_id_created_at_idx ON audit_log(user_id, created_at DESC);

CREATE TABLE admin_cases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    status text NOT NULL DEFAULT 'open',
    subject text NOT NULL,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    domain_id uuid REFERENCES domains(id) ON DELETE SET NULL,
    assigned_to uuid REFERENCES users(id) ON DELETE SET NULL,
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX admin_cases_status_created_at_idx ON admin_cases(status, created_at DESC);
CREATE INDEX admin_cases_user_id_created_at_idx ON admin_cases(user_id, created_at DESC);

CREATE TABLE admin_case_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id uuid NOT NULL REFERENCES admin_cases(id) ON DELETE CASCADE,
    author_id uuid REFERENCES users(id) ON DELETE SET NULL,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX admin_case_notes_case_id_created_at_idx ON admin_case_notes(case_id, created_at);

CREATE TABLE abuse_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identifier text NOT NULL,
    action text NOT NULL,
    reason text NOT NULL,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX abuse_decisions_identifier_expires_at_idx ON abuse_decisions(identifier, expires_at);

CREATE TABLE status_components (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    status text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE status_incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    status text NOT NULL,
    started_at timestamptz NOT NULL,
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE status_incident_updates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id uuid NOT NULL REFERENCES status_incidents(id) ON DELETE CASCADE,
    message text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX status_incident_updates_incident_id_created_at_idx ON status_incident_updates(incident_id, created_at);

CREATE TABLE organization_retention_settings (
    organization_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    raw_report_days integer NOT NULL,
    aggregate_row_days integer NOT NULL,
    notification_log_days integer NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS organization_retention_settings;
DROP TABLE IF EXISTS status_incident_updates;
DROP TABLE IF EXISTS status_incidents;
DROP TABLE IF EXISTS status_components;
DROP TABLE IF EXISTS abuse_decisions;
DROP TABLE IF EXISTS admin_case_notes;
DROP TABLE IF EXISTS admin_cases;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS domain_transfers;
DROP TABLE IF EXISTS organization_invitations;
DROP TABLE IF EXISTS organization_members;
DROP INDEX IF EXISTS domains_organization_domain_idx;
DROP INDEX IF EXISTS domains_organization_id_idx;
DELETE FROM domains WHERE user_id IS NULL;
ALTER TABLE domains DROP COLUMN IF EXISTS organization_id;
ALTER TABLE domains ALTER COLUMN user_id SET NOT NULL;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS enrichment_cache;
DROP TABLE IF EXISTS sender_attributions;
DROP TABLE IF EXISTS dmarc_aggregate_rows;
DROP TABLE IF EXISTS dmarc_ingestion_failures;
DROP TABLE IF EXISTS dmarc_reports;
DROP TABLE IF EXISTS notification_dispatches;
DROP TABLE IF EXISTS drift_events;
DROP TABLE IF EXISTS job_dead_letters;
DROP TABLE IF EXISTS scan_jobs;
DROP TABLE IF EXISTS domain_settings;
