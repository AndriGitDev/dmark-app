-- name: UpsertUserByEmail :one
INSERT INTO users (email)
VALUES ($1)
ON CONFLICT (email) DO UPDATE SET updated_at = now()
RETURNING id, email, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, created_at, updated_at
FROM users
WHERE id = $1;

-- name: CreateLoginToken :one
INSERT INTO login_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, consumed_at, created_at;

-- name: ConsumeLoginToken :one
UPDATE login_tokens
SET consumed_at = now()
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING id, user_id, token_hash, expires_at, consumed_at, created_at;
