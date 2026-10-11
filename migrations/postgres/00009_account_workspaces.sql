-- +goose Up
CREATE TABLE account_workspaces (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 domains jsonb NOT NULL DEFAULT '[]'::jsonb,
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE account_workspaces;
