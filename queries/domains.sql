-- name: CreateDomain :one
INSERT INTO domains (user_id, domain)
VALUES ($1, $2)
ON CONFLICT (user_id, domain) DO UPDATE SET updated_at = now()
RETURNING id, user_id, domain, created_at, updated_at;

-- name: ListDomainsWithLatestCheck :many
SELECT
    d.id,
    d.user_id,
    d.domain,
    d.created_at,
    d.updated_at,
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
    SELECT *
    FROM domain_checks dc
    WHERE dc.domain_id = d.id
    ORDER BY dc.created_at DESC
    LIMIT 1
) c ON true
WHERE d.user_id = $1
ORDER BY d.created_at DESC;

-- name: GetDomainForUser :one
SELECT id, user_id, domain, created_at, updated_at
FROM domains
WHERE id = $1 AND user_id = $2;

-- name: DeleteDomainForUser :exec
DELETE FROM domains
WHERE id = $1 AND user_id = $2;

-- name: CreateDomainCheck :one
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
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, domain_id, spf_status, spf_message, spf_record, dkim_status, dkim_message, dkim_record, dmarc_status, dmarc_message, dmarc_record, bimi_status, bimi_message, bimi_record, created_at;
