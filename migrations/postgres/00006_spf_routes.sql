-- +goose Up
CREATE TABLE spf_routes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    route_label text NOT NULL,
    route_fqdn text NOT NULL,
    customer_cname_name text NOT NULL,
    customer_cname_target text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    mechanism_policy text NOT NULL DEFAULT 'softfail',
    bunny_record_id bigint NOT NULL DEFAULT 0,
    last_generated_spf text NOT NULL DEFAULT '',
    last_published_spf text NOT NULL DEFAULT '',
    last_sync_status text NOT NULL DEFAULT 'pending',
    last_sync_error text NOT NULL DEFAULT '',
    last_synced_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (domain_id),
    UNIQUE (route_label),
    CHECK (mechanism_policy IN ('softfail', 'fail', 'neutral')),
    CHECK (last_sync_status IN ('pending', 'healthy', 'stale', 'failed', 'blocked'))
);

CREATE INDEX spf_routes_enabled_status_idx ON spf_routes(enabled, last_sync_status, updated_at DESC);
CREATE INDEX spf_routes_organization_id_idx ON spf_routes(organization_id);

CREATE TABLE spf_route_sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    spf_route_id uuid NOT NULL REFERENCES spf_routes(id) ON DELETE CASCADE,
    kind text NOT NULL,
    value text NOT NULL,
    label text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (kind IN ('include', 'ip4', 'ip6'))
);

CREATE INDEX spf_route_sources_route_id_idx ON spf_route_sources(spf_route_id, enabled);

CREATE TABLE spf_route_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    spf_route_id uuid NOT NULL REFERENCES spf_routes(id) ON DELETE CASCADE,
    source_value text NOT NULL DEFAULT '',
    resolved_domain text NOT NULL DEFAULT '',
    mechanism text NOT NULL DEFAULT '',
    evidence_kind text NOT NULL,
    message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX spf_route_evidence_route_created_at_idx ON spf_route_evidence(spf_route_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS spf_route_evidence;
DROP TABLE IF EXISTS spf_route_sources;
DROP TABLE IF EXISTS spf_routes;
