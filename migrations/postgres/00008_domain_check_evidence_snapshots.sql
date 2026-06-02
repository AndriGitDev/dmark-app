-- +goose Up
ALTER TABLE domain_checks
ADD COLUMN evidence_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE domain_checks
DROP COLUMN evidence_snapshot;
