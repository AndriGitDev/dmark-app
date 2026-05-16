-- +goose Up
CREATE TABLE protected_brands (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    display_name text NOT NULL,
    primary_domain text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (organization_id IS NOT NULL OR user_id IS NOT NULL)
);

CREATE INDEX protected_brands_organization_id_idx ON protected_brands(organization_id);
CREATE INDEX protected_brands_user_id_idx ON protected_brands(user_id);

CREATE TABLE brand_watch_terms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    protected_brand_id uuid NOT NULL REFERENCES protected_brands(id) ON DELETE CASCADE,
    term text NOT NULL,
    kind text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX brand_watch_terms_protected_brand_id_idx ON brand_watch_terms(protected_brand_id);
CREATE UNIQUE INDEX brand_watch_terms_unique_idx ON brand_watch_terms(protected_brand_id, lower(term), kind);

CREATE TABLE brand_domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    protected_brand_id uuid NOT NULL REFERENCES protected_brands(id) ON DELETE CASCADE,
    domain_id uuid REFERENCES domains(id) ON DELETE SET NULL,
    domain text NOT NULL,
    role text NOT NULL DEFAULT 'primary',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX brand_domains_domain_id_idx ON brand_domains(domain_id);
CREATE UNIQUE INDEX brand_domains_unique_idx ON brand_domains(protected_brand_id, lower(domain));

CREATE TABLE brand_risk_findings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    protected_brand_id uuid NOT NULL REFERENCES protected_brands(id) ON DELETE CASCADE,
    domain_id uuid REFERENCES domains(id) ON DELETE SET NULL,
    finding_type text NOT NULL,
    observed_value text NOT NULL,
    severity text NOT NULL,
    status text NOT NULL DEFAULT 'open',
    evidence_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    recommended_action text NOT NULL DEFAULT '',
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX brand_risk_findings_brand_status_idx ON brand_risk_findings(protected_brand_id, status, severity, last_seen_at DESC);
CREATE INDEX brand_risk_findings_domain_id_idx ON brand_risk_findings(domain_id);

CREATE TABLE brand_risk_observations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    brand_risk_finding_id uuid NOT NULL REFERENCES brand_risk_findings(id) ON DELETE CASCADE,
    source text NOT NULL,
    observed_value text NOT NULL,
    evidence_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    observed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX brand_risk_observations_finding_observed_at_idx ON brand_risk_observations(brand_risk_finding_id, observed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS brand_risk_observations;
DROP TABLE IF EXISTS brand_risk_findings;
DROP TABLE IF EXISTS brand_domains;
DROP TABLE IF EXISTS brand_watch_terms;
DROP TABLE IF EXISTS protected_brands;
