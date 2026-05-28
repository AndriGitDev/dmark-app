# Dmark GA Readiness

This tracker records the current release posture for general availability prep. It should be updated whenever a release gate is run or a GA blocker is accepted, fixed, or moved out of scope.

Last full local verification: 2026-05-27. Frontend cleanup spot-check: 2026-05-28.

## Current Status

- Repository state: local `main` is backend-only after removing the stale in-repo frontend build files; `.DS_Store` remains untracked and should not be committed.
- Code-level readiness: passing locally after installing Go and required QA tools.
- Service-level readiness: passing locally after installing Docker CLI, Docker Compose, and Colima, then starting the dev Postgres, Mailpit, and ClickHouse services.
- Product posture: broad backend implementation exists for scanning, saved-domain APIs, drift alerts, Reporter ingestion, attribution, billing/team foundations, API/webhooks, admin/privacy/abuse/status, observability definitions, and Coolify deployment. Browser-facing surfaces and frontend build checks now live in `dmark-web`.
- Competitive posture: `docs/COMPETITIVE_ROADMAP.md` defines the Icelandic Dmarcian/EasyDMARC alternative strategy, including concierge DMARC enforcement, monthly trust reports, `.is` Defender coverage, and the Checkmate-style sales-audit funnel documented in `docs/CHECKMATE_INTEGRATION.md`. Backend grading and evidence primitives remain here; public funnel UX belongs in `dmark-web`.
- GA posture: not ready to declare GA until `dmark-web` browser/manual QA plus backend production operations drills, load checks, backup/restore and rollback drills, and live-provider checks are completed or explicitly scoped out.

## Verified Automated Gates

Run from `/Users/andri/Documents/GitHub/meil` with Go caches pointed at writable temp directories because the default macOS cache path is sandbox-blocked in this shell.

- `make qa`: pass with `PATH=/private/tmp/meil-bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`, writable Go/golangci-lint cache env vars, `DOCKER_COMPOSE=docker-compose`, and Colima running.
- `go test ./...`: pass.
- `go test -race ./...`: pass when localhost binding is allowed for `httptest` packages.
- `go vet ./...`: pass.
- `golangci-lint run`: pass with `GOLANGCI_LINT_CACHE=/private/tmp/meil-golangci-cache`.
- `govulncheck ./...`: pass after upgrading `golang.org/x/net` from `v0.54.0` to `v0.55.0`.
- `make generate-check`: pass.
- `make fuzz-smoke`: pass.
- `go build -o bin/meil ./cmd/meil`: pass.
- `make qa-services`: pass against fresh local dev Postgres and ClickHouse services.
- `docker build -f infra/coolify/Dockerfile .`: pass.
- Fresh Postgres migrations: pass through `00004_sales_audit_claims.sql` plus River migrations.
- 2026-05-27 Postgres migration check: `go run ./cmd/meil migrate up` applied `00004_sales_audit_claims.sql`; `go run ./cmd/meil migrate status` shows versions 1-4 applied plus River migrations 1-6.
- Fresh ClickHouse migrations: pass.
- Competitive primitives: focused tests pass for `internal/dmarcjourney`, `internal/brandrisk`, `internal/trustreport`, and `internal/salesaudit`.
- 2026-05-27 follow-up verification: `go test ./...`, `go build -o /private/tmp/meil-build-check ./cmd/meil`, `git diff --check`, and `MEIL_DB_INTEGRATION=1 go test ./internal/store/pg -count=1` pass.
- 2026-05-28 frontend cleanup spot-check: stale Next.js/Tailwind/Node files were removed from this repository; CI now runs Go/sqlc/security gates without frontend generation; `go test ./...`, `go build -o /private/tmp/meil-build-check ./cmd/meil`, and `git diff --check` pass locally. `make generate-check` was not rerun because `sqlc` is not installed in this shell.

## Remaining GA Gates

These must pass before a GA release candidate is promoted:

1. Browser QA for public checker, auth, saved-domain dashboard, API-key/team/billing surfaces, and responsive layouts belongs in `dmark-web`.
2. Manual accessibility pass for keyboard flow, focus states, screen reader labels, color contrast, and mobile layouts belongs in `dmark-web`.
3. Load checks for authenticated API health/read endpoints, Reporter reads, and scan-job trigger endpoints.
4. Backup and restore drill for Postgres and ClickHouse.
5. Production rollback drill for Coolify app and worker roles.
6. Live-provider checks for Lettermint email, billing webhooks, enrichment/geolocation adapters, and DMARC mailbox/report forwarding.

## Immediate GA Workstream

Priority order:

1. Keep dependency/security gates green.
2. Keep browser-level regression coverage for public and authenticated web surfaces in `dmark-web`.
3. Deploy and smoke backend API/status/reporting surfaces after the frontend handoff.
4. Decide whether proof-email dispatch is GA scope; keep it isolated until ownership verification, rate limits, blocklists, consent, and audit logging are implemented.
5. Execute and document load, backup/restore, and rollback drills.
6. Decide which live-provider integrations are GA scope and which remain beta/operator-managed.
7. Keep `make qa` reproducible in CI-equivalent infrastructure and document any local setup requirements.
