-- +goose Up
CREATE TABLE organization_dashboard_settings (
    organization_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    require_alignment_before_reject integer NOT NULL DEFAULT 95,
    pause_automation_on_unknown_sender_spike boolean NOT NULL DEFAULT true,
    auto_generate_monthly_executive_reports boolean NOT NULL DEFAULT false,
    notification_channels text[] NOT NULL DEFAULT '{email}',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (require_alignment_before_reject BETWEEN 1 AND 100)
);

CREATE INDEX scan_jobs_domain_status_created_at_idx ON scan_jobs(domain_id, status, created_at DESC);
CREATE INDEX drift_events_active_dashboard_idx ON drift_events(domain_id, created_at DESC)
WHERE acknowledged_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS drift_events_active_dashboard_idx;
DROP INDEX IF EXISTS scan_jobs_domain_status_created_at_idx;
DROP TABLE IF EXISTS organization_dashboard_settings;
