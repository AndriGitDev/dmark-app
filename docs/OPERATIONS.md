# Dmark Operations

## Release Gates

Before production promotion, run the automated gates from `docs/QA_TEST_PLAN.md`:

- `make qa-code` for code-level gates
- `make qa-services` from an environment with Docker, Postgres, and ClickHouse access
- `make qa` when both local code tooling and service dependencies are available
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `golangci-lint run`
- `govulncheck ./...`
- `make fuzz-smoke`
- `make generate-check` to verify sqlc outputs are current
- `go build ./cmd/meil`
- `docker build -f infra/coolify/Dockerfile .`
- `go run ./cmd/meil migrate up` against a fresh Postgres database
- `go run ./cmd/meil migrate clickhouse up` against a fresh ClickHouse database

## Load Checks

The repository includes a small HTTP load utility for release and incident drills:

```sh
BASE_URL=https://dmark.example PATH=/healthz REQUESTS=200 CONCURRENCY=20 make load-api
BASE_URL=https://dmark.example API_TOKEN=... DOMAIN_ID=... REQUESTS=200 CONCURRENCY=20 make load-reporter
BASE_URL=https://dmark.example API_TOKEN=... DOMAIN_ID=... REQUESTS=50 CONCURRENCY=5 make load-scan-job
```

Use `api-get`/`reporter-get` for authenticated API and Reporter read endpoints, and `api-post` for domain recheck or other job-triggering endpoints. Browser load checks and frontend bundle checks belong in `dmark-web`. Record p50, p95, max latency, status distribution, target commit, and environment size with the release notes.

## Incident Process

1. Confirm user-visible impact and affected surfaces: API scan, protected domains, Brand Risk findings, jobs, notifications, or Reporter ingestion.
2. Capture current deploy version, recent migrations, and relevant logs with request IDs.
3. Mitigate first: disable noisy alerts, pause ingestion, roll back deploy, or scale workers as appropriate.
4. Preserve source material for auditability: DNS snapshots, DMARC raw reports, notification dispatch rows, and drift events.
5. Write a post-incident note with trigger, detection gap, customer impact, remediation, and follow-up tasks.

## Backups And Restore

- Postgres must be backed up at least daily once customer data is present.
- DMARC raw XML retention must match the customer plan and privacy policy.
- Restore drills should validate users, domains, checks, drift events, reports, and audit logs.
- Deployment rollback drills should record the current deploy artifact, promote a candidate, run smoke and service-backed QA gates, roll back to the previous artifact, and verify health, API scan, worker startup, and Reporter reads.

## Observability

Track at minimum:

- API scan request count, latency, and DNS resolver error class.
- Scan job queue latency, attempts, failures, and dead letters.
- Drift event volume by severity and check type.
- Notification dispatch success/failure rate by provider.
- DMARC ingestion count, duplicate count, parser failures, and payload size.

Metric, trace, dashboard, and alert-rule definitions live in `internal/observability`. Production monitoring should include API scan, scanner, worker, ingestion, and Reporter traces; API scan, worker, and Reporter dashboards; plus alerts for high API scan error rate, scan backlog, notification failures, and DMARC parser failures.
