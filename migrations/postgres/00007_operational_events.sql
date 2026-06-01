-- +goose Up
CREATE TABLE operational_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    severity text NOT NULL DEFAULT 'info',
    message text NOT NULL,
    actor text NOT NULL DEFAULT '',
    resource text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (kind <> ''),
    CHECK (message <> ''),
    CHECK (severity IN ('info', 'warn', 'error'))
);

CREATE INDEX operational_events_recent_idx ON operational_events(occurred_at DESC, id DESC);
CREATE INDEX operational_events_severity_recent_idx ON operational_events(severity, occurred_at DESC, id DESC);
CREATE INDEX operational_events_kind_recent_idx ON operational_events(kind, occurred_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS operational_events;
