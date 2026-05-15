-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX login_tokens_user_id_idx ON login_tokens(user_id);
CREATE INDEX login_tokens_valid_idx ON login_tokens(token_hash, expires_at) WHERE consumed_at IS NULL;

CREATE TABLE domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, domain)
);

CREATE INDEX domains_user_id_idx ON domains(user_id);

CREATE TABLE domain_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    spf_status text NOT NULL,
    spf_message text NOT NULL,
    spf_record text NOT NULL DEFAULT '',
    dkim_status text NOT NULL,
    dkim_message text NOT NULL,
    dkim_record text NOT NULL DEFAULT '',
    dmarc_status text NOT NULL,
    dmarc_message text NOT NULL,
    dmarc_record text NOT NULL DEFAULT '',
    bimi_status text NOT NULL,
    bimi_message text NOT NULL,
    bimi_record text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX domain_checks_domain_id_created_at_idx ON domain_checks(domain_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS domain_checks;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS login_tokens;
DROP TABLE IF EXISTS users;
