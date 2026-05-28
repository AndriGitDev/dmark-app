# Roadmap Implementation Audit

This audit maps `docs/ROADMAP.md` to concrete repository artifacts. It is intentionally evidence-based: an item is not considered release-ready unless there is an implementation artifact and a runnable verification path.

## Audit Status

- Implementation breadth: Phase 1 through Phase 7 now have code and documentation artifacts.
- Local verification: `make qa-code` passes using a temporary local Go 1.25.10 toolchain and covers unit tests, race tests, vet, lint, `govulncheck`, short fuzz smoke runs, generated-file idempotence for sqlc, and build.
- Repository boundary: browser-facing checker/auth/app views and frontend build files moved to `dmark-web`; this repository is backend/API only.
- Completion status: code and service-backed QA gates pass against the local dev Postgres and ClickHouse stack.
- Latest service-gate evidence:
  - `make qa-code` passes: unit tests, race tests, vet, golangci-lint, `govulncheck`, fuzz smoke runs, generated-file idempotence, and build.
  - `make qa-services` passes after starting `infra/coolify/docker-compose.dev.yml`: Docker image build, Postgres migrations through `00004_sales_audit_claims.sql`, River migrations, and ClickHouse migrations.

## Phase 1: Checker Reliability And UX

Evidence:

- Scanner parsing, metadata, selector probing, and summaries: `internal/scanner/scanner.go`, `internal/scanner/summary.go`, `internal/scanner/*_test.go`.
- International domain normalization via IDNA/punycode: `internal/scanner/scanner.go`, `internal/scanner/scanner_test.go`.
- Shared API/CLI scanner surface and JSON output: `internal/web/server.go`, `cmd/meil/main.go`, `cmd/meil/main_test.go`.
- Browser route handoff controls: `internal/web/server.go`, `internal/web/server_test.go`.

Verification gaps:

- Scanner, web, and CLI tests pass with the temporary local Go toolchain used during implementation.
- Browser/mobile layout and full accessibility audits are owned by `dmark-web`.

## Phase 2: Saved Domains Completion

Evidence:

- Auth/domain foundations, request IDs, API import/export, and browser route handoff: `internal/web/server.go`, `internal/web/server_test.go`.
- Domain settings, ownership verification, history, dashboard states, weekly summaries, CSV import/export: `internal/domains/*.go`.
- River-backed scan/enrichment workers, retry policy, idempotency keys, and scheduler: `internal/jobs/scan.go`, `internal/jobs/enrichment.go`, `internal/scheduler/scheduler.go`, `cmd/meil/main.go`.
- Postgres schema foundations plus migration/store integration coverage for auth token states, domain uniqueness, latest checks, and cascades: `migrations/postgres/00002_roadmap_foundations.sql`, `internal/store/pg/app_store_integration_test.go`.

Verification gaps:

- Database-backed integration tests are implemented but could not pass against the local default DSN because Postgres rejected the documented `meil:meil` credentials.
- Browser-visible auth, dashboard, and focus behavior is owned by `dmark-web`.

## Phase 3: Drift Detection And Alerts

Evidence:

- Drift classifier: `internal/drift/drift.go`.
- Alert rules, acknowledgement, mute, suppression, timeline, and notification payloads: `internal/alerts/*.go`, `internal/notifier/*.go`.
- River worker registration, queue retry/dead-letter helpers, and monitoring pipeline: `internal/jobs/*.go`, `internal/monitoring/pipeline.go`, `cmd/meil/main.go`.
- Durable tables: `drift_events`, `notification_dispatches`, `scan_jobs`, and `job_dead_letters` in `migrations/postgres/00002_roadmap_foundations.sql`.

Verification gaps:

- River workers have unit coverage and are wired into `meil worker`; database-backed worker execution and one-notification-only delivery still need a service-backed integration pass.

## Phase 4: Reporter Ingestion

Evidence:

- RUA provisioning, XML/gzip/zip/stdin parsing, validation, dedupe keys, raw report retention, fixtures, and dead letters: `internal/ingest/*.go`, `cmd/meil/main.go`, `infra/postfix/README.md`.
- ClickHouse setup, migration runner, status command, and aggregate model: `internal/store/ch/aggregate.go`, `internal/store/ch/migrate.go`, `migrations/clickhouse/00001_dmarc_aggregate.sql`, `cmd/meil/main.go`, `infra/clickhouse/README.md`.
- Reporter dashboard, health, filters, inventory, alignment, and unknown sender logic: `internal/reporter/*.go`.
- CLI ingest command boundary: `cmd/meil/main.go`.

Verification gaps:

- ClickHouse migration loading and HTTP request behavior are unit-tested. Fresh service-backed migration execution and write/query performance checks require a running ClickHouse service.

## Phase 5: Sender Attribution And Intelligence

Evidence:

- Provider rules, explainable attribution, deterministic review fixtures, and ambiguous-provider candidate detection: `internal/attribution/attribution.go`, `internal/attribution/fixtures.go`, `internal/attribution/attribution_test.go`.
- Enrichment cache, refresh service, River refresh worker, and Postgres cache persistence: `internal/attribution/cache.go`, `internal/attribution/enrichment.go`, `internal/jobs/enrichment.go`, `internal/store/pg/app_store.go`.
- Unknown sender, recommendations, alignment explanations, inventory over time, and risk scoring: `internal/reporter/*.go`, `internal/recommendations/recommendations.go`, `internal/risk/risk.go`.

Verification gaps:

- Live ASN/geolocation enrichment still needs external provider fixtures or sandboxed adapters for provider-specific API behavior; deterministic attribution and ambiguous-provider review fixtures are covered locally.

## Phase 6: Paid Product, Teams, And Billing

Evidence:

- Plans, limits, RBAC, and permissions: `internal/entitlements/entitlements.go`.
- Billing webhook verification, idempotent organization updates, and cancellation request flow: `internal/billing/billing.go`, `internal/web/server.go`, `internal/store/pg/app_store.go`.
- Team invitations, roles, invitation API, and domain transfer request API: `internal/teams/teams.go`, `internal/web/server.go`, `internal/store/pg/app_store.go`.
- API keys, Postgres API-key authentication with last-used tracking, API key lifecycle management, programmatic scan/domain-list/domain-export/domain-create/domain-import/domain-recheck/domain-history/domain-drift/reporter/webhook-management routes, contracts, webhooks, and Slack payloads: `internal/apikeys/apikeys.go`, `internal/store/pg/app_store.go`, `internal/web/server.go`, `internal/api/contracts.go`, `internal/webhooks/webhooks.go`.
- Audit log model and append-only validation: `internal/audit/audit.go`.

Verification gaps:

- Live billing provider flows and organization-scoped API handlers need database-backed integration coverage before release.

## Phase 7: Platform Hardening And Compliance

Evidence:

- Status and incident model, JSON status/security endpoints, status component/incident admin API, and browser route handoff controls: `internal/statuspage/status.go`, `internal/web/server.go`, `internal/store/pg/app_store.go`.
- Data retention, organization retention settings API, admin/support case API, abuse decision API, operations drills, and HTTP load-check tooling for API/Reporter/scan-job endpoints: `internal/retention/retention.go`, `internal/privacy/privacy.go`, `internal/web/server.go`, `internal/store/pg/app_store.go`, `internal/admin/admin.go`, `internal/abuse/abuse.go`, `internal/ops/drills.go`, `scripts/load/main.go`, `Makefile`.
- CI gates, Makefile QA targets, River/app migration path, and operational docs: `.github/workflows/ci.yml`, `Makefile`, `cmd/meil/main.go`, `docs/QA_TEST_PLAN.md`, `docs/OPERATIONS.md`, `docs/SECURITY.md`, `docs/THREAT_MODEL.md`, `docs/DNS_GUIDE.md`, `docs/API.md`.
- Config validation and secret inventory: `internal/config/config.go`, `internal/config/secrets.go`, `.env.example`.
- Observability definitions for metrics, traces, dashboards, and alerts: `internal/observability/*.go`.
- Fuzz seed entry points for scanner parser and DMARC XML ingestion hardening: `internal/scanner/fuzz_test.go`, `internal/ingest/fuzz_test.go`.
- Security dependency gate and remediation: `Makefile` includes `make security`, CI installs/runs `govulncheck`, `go.mod` targets Go 1.25.10 and `github.com/jackc/pgx/v5` v5.9.0.

Verification gaps:

- Backend load testing against a running app, backup/restore drills, fresh service migrations, and container build gates still need an operator-provisioned QA environment. Browser accessibility belongs in `dmark-web`. Docker socket access may require operator-provisioned service access depending on the environment.

## Required QA Gate Before Completion

Run these from a machine with the required tools and service access:

0. `make qa`, or the equivalent individual gates below
1. `make test`
2. `go test -race ./...`
3. `go vet ./...`
4. `make lint`
5. `govulncheck ./...`
6. `make generate-check`
7. `make generate`
8. `make build`
9. `docker build -f infra/coolify/Dockerfile .`
10. Fresh Postgres migration check with `go run ./cmd/meil migrate postgres up`
11. Fresh ClickHouse migration check with `go run ./cmd/meil migrate clickhouse up`

The goal should not be marked complete until those gates pass and the verification gaps above are either closed or explicitly accepted as non-release scope.
