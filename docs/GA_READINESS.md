# Kastro RecordMaster GA Readiness

This tracker records the current release posture for general availability prep. It should be updated whenever a release gate is run or a GA blocker is accepted, fixed, or moved out of scope.

Last local verification: 2026-05-23.

## Current Status

- Repository state: clean `main` before GA prep; current prep changes update `golang.org/x/net` to clear a reachable `govulncheck` finding in IDNA domain normalization.
- Code-level readiness: passing locally after installing Go and required QA tools.
- Service-level readiness: passing locally after installing Docker CLI, Docker Compose, and Colima, then starting the dev Postgres, Mailpit, and ClickHouse services.
- Product posture: broad implementation exists for Checker, saved domains, drift alerts, Reporter ingestion, attribution, billing/team foundations, API/webhooks, admin/privacy/abuse/status, observability definitions, and Coolify deployment.
- Competitive posture: `docs/COMPETITIVE_ROADMAP.md` defines the Icelandic Dmarcian/EasyDMARC alternative strategy, including concierge DMARC enforcement, monthly trust reports, `.is` Defender coverage, and the Checkmate-style sales-audit funnel documented in `docs/CHECKMATE_INTEGRATION.md`.
- GA posture: not ready to declare GA until browser/manual QA, production operations drills, load checks, backup/restore and rollback drills, and live-provider checks are completed or explicitly scoped out.

## Verified Automated Gates

Run from `/Users/andri/Documents/GitHub/meil` with Go caches pointed at writable temp directories because the default macOS cache path is sandbox-blocked in this shell.

- `make qa`: pass with `PATH=/private/tmp/meil-bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`, writable Go/golangci-lint cache env vars, `DOCKER_COMPOSE=docker-compose`, and Colima running.
- `go test ./...`: pass.
- `go test -race ./...`: pass when localhost binding is allowed for `httptest` packages.
- `go vet ./...`: pass.
- `golangci-lint run`: pass with `GOLANGCI_LINT_CACHE=/private/tmp/meil-golangci-cache`.
- `govulncheck ./...`: pass after upgrading `golang.org/x/net` from `v0.54.0` to `v0.55.0`.
- `make generate-check`: pass.
- `make templ-check`: pass.
- `make fuzz-smoke`: pass.
- `go build -o bin/meil ./cmd/meil`: pass.
- `make qa-services`: pass against fresh local dev Postgres and ClickHouse services.
- `docker build -f infra/coolify/Dockerfile .`: pass.
- Fresh Postgres migrations: pass through `00003_brand_risk_foundations.sql` plus River migrations.
- Fresh ClickHouse migrations: pass.
- Competitive primitives: focused tests pass for `internal/dmarcjourney`, `internal/brandrisk`, `internal/trustreport`, and `internal/salesaudit`.

## Remaining GA Gates

These must pass before a GA release candidate is promoted:

1. Browser QA for public checker, auth, saved-domain dashboard, status/security pages, API-key/team/billing surfaces, and responsive layouts with long DNS/report content.
2. Manual accessibility pass for keyboard flow, focus states, screen reader labels, color contrast, and mobile layouts.
3. Load checks for public checker, authenticated API health/read endpoints, Reporter reads, and scan-job trigger endpoints.
4. Backup and restore drill for Postgres and ClickHouse.
5. Production rollback drill for Coolify app and worker roles.
6. Live-provider checks for Lettermint email, billing webhooks, enrichment/geolocation adapters, and DMARC mailbox/report forwarding.

## Immediate GA Workstream

Priority order:

1. Keep dependency/security gates green.
2. Add browser-level regression coverage for the public and authenticated web surfaces.
3. Surface DMARC journey status, monthly trust report sections, and sales-audit grading in the web product.
4. Decide whether proof-email dispatch is GA scope; keep it isolated until ownership verification, rate limits, blocklists, consent, and audit logging are implemented.
5. Execute and document load, backup/restore, and rollback drills.
6. Decide which live-provider integrations are GA scope and which remain beta/operator-managed.
7. Keep `make qa` reproducible in CI-equivalent infrastructure and document any local setup requirements.
