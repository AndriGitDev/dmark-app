-- +goose Up
CREATE TABLE sales_audit_claims (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain text NOT NULL,
    email text NOT NULL,
    grade text NOT NULL,
    risk text NOT NULL,
    safe boolean NOT NULL DEFAULT false,
    proof_eligible boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'magic_link_sent',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sales_audit_claims_domain_created_at_idx ON sales_audit_claims(lower(domain), created_at DESC);
CREATE INDEX sales_audit_claims_email_created_at_idx ON sales_audit_claims(lower(email), created_at DESC);
CREATE INDEX sales_audit_claims_status_created_at_idx ON sales_audit_claims(status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS sales_audit_claims;
