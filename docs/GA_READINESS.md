# Dmark GA Readiness

This tracker records the current release posture for general availability prep. It should be updated whenever a release gate is run or a GA blocker is accepted, fixed, or moved out of scope.

Current launch roadmap: `docs/GA_LAUNCH_ROADMAP.md`.

Last full historical local verification: 2026-05-27. Frontend cleanup spot-check: 2026-05-28. SPF route persistence/orchestration slice verified and deployed after 2026-05-31 CI success.

## Current Status

- Repository state: `dmark-app` is backend-only; browser-facing surfaces and frontend build checks live in `dmark-web`.
- Code-level readiness: broad backend gates have historical passing evidence; the SPF route persistence/orchestration slice has recent Docker/CI verification and live backend smoke evidence.
- Service-level readiness: Coolify auto-deploy is enabled for the project, and `https://dmark.kastro.is` returned healthy `/healthz`, `/status`, and `/api/public/scan?domain=kastro.is` responses after the latest SPF route slice. The backend RC code/container ladder is available through `make rc-backend`; service-backed Postgres/ClickHouse RC checks are now available through `make rc-backend-services`.
- Product posture: broad backend implementation exists for scanning, saved-domain APIs, drift alerts, Reporter primitives, attribution, billing/team foundations, API/webhooks, admin/privacy/abuse/status, observability definitions, non-secret `/status` build metadata, and Coolify deployment. SPF resolver/generator, Bunny DNS client, Postgres persistence, sync orchestration, queued River SPF route sync worker registration, tested 12-hour SPF route due-sync scheduler primitives, and default Coolify worker scheduler enqueueing are implemented; dashboard API endpoints, live Bunny route proof, and frontend SPF route UI remain open.
- Competitive posture: `docs/COMPETITIVE_ROADMAP.md` defines the Icelandic Dmarcian/EasyDMARC alternative strategy, including concierge DMARC enforcement, monthly trust reports, `.is` Defender coverage, and the Checkmate-style sales-audit funnel documented in `docs/CHECKMATE_INTEGRATION.md`. Backend grading and evidence primitives remain here; public funnel UX belongs in `dmark-web`.
- GA posture: planned launch window is Friday 2026-06-05, not a hard date. Do not declare GA until the must-have gates in `docs/GA_LAUNCH_ROADMAP.md` pass or are explicitly descoped into a public-beta launch posture.

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

These must pass before a GA release candidate is promoted. `docs/GA_LAUNCH_ROADMAP.md` is the current execution roadmap for these gates.

1. SPF route dashboard API endpoints for route read/upsert/manual sync, with org/scope checks and frontend-ready response contracts.
2. SPF route operator visibility for failed/blocked/stale scheduled resync states.
3. Live Bunny DNS publication proof in the parent `dmark.is` zone, including create/update/no-op/blocking failure behaviour.
4. DNS publication audit command/protected admin endpoint plus synthetic monitoring for homepage, backend health, public scan, and Bunny route DNS behaviour.
5. `dmark-web` SPF route UI and provider-guidance UI through server-side dashboard proxy routes.
6. Nameserver/provider autopicker surfacing: detect the customer's DNS host from authoritative NS records, map common Icelandic and global DNS providers to step-by-step SPF/DMARC/CNAME instructions, and show deep links into the provider's DNS management surface when available.
7. Evidence snapshots and durable operational events for scans, recommendations, SPF generation/publication, bootstrap, API-key use, and external-provider failures. Bootstrap ran/skipped events are now emitted by startup and covered by direct unit tests for safe metadata shape.
8. Minimal admin/provider-health surfaces so operators can inspect orgs/domains, scans, SPF route state, provider health, recent events, and manual scan/sync actions. `/status` now has non-secret build metadata; admin provider-health now reports non-secret worker queue name, max worker count, and registered job kinds, while deeper database/queue liveness still needs an internal operator surface.
9. Customer-facing explanation layer for scanner/recommendation output, including severity, confidence, exact DNS changes, provider instructions, propagation timing, and next action.
10. Security/abuse pass for public scan and dashboard-adjacent endpoints: rate limits, strict domain normalization, DNS/internal-network guardrails where relevant, locked-down CORS, security headers, request IDs, and secret-safe logs.
11. Known-good domain fixtures for regression tests and production smoke checks, including valid, broken, too-many-lookups, missing-DMARC, parked/no-MX, IDN, and route-published cases. The fixture registry now reserves Dmark-owned canary names and synthetic smoke can enforce expected scan summary status per provisioned fixture; live records still need out-of-band provisioning before those canaries become hard gates.
12. Privacy-respecting funnel analytics for homepage visits, scan starts/completions, claim starts/completions, recommendation views, CTA clicks/bookings, and beta-lock/dashboard interactions.
13. Browser QA for public checker, auth, saved-domain dashboard, API-key/team/billing surfaces, SPF route UI, provider guidance, and responsive layouts in `dmark-web`.
14. Manual accessibility pass for keyboard flow, focus states, screen reader labels, color contrast, and mobile layouts in `dmark-web`.
15. Load checks for authenticated API health/read endpoints, Reporter reads, SPF flattener sync/status endpoints, DNS-provider autopicker endpoints, and scan-job trigger endpoints.
16. Backup and restore drill for Postgres and ClickHouse.
17. Production rollback drill for Coolify app and worker roles, including SPF route publication rollback.
18. Live-provider checks for Bunny.net DNS, Lettermint email, billing webhooks if GA-scoped, enrichment/geolocation adapters if GA-scoped, DNS provider detection, and DMARC mailbox/report forwarding if Reporter is GA-scoped.

## Immediate GA Workstream

Priority order is now tracked in `docs/GA_LAUNCH_ROADMAP.md`:

1. Backend SPF route dashboard API.
2. SPF route operator visibility for failed/blocked/stale resyncs.
3. Live Bunny publication proof.
4. DNS publication audit path and synthetic scanner/DNS monitoring.
5. `dmark-web` SPF route UI.
6. DNS provider guidance API/UI surfacing.
7. Evidence snapshots, operational events, and customer-facing explanations.
8. Minimal admin/provider-health/version surfaces.
9. Security/abuse pass and privacy-respecting funnel analytics.
10. Backend and frontend release-candidate gates.
11. Live-provider checks.
12. Backup/restore/rollback/DNS recovery drills.
13. Observability confirmation.
14. Final docs/release notes.

Proof-email dispatch remains isolated unless explicitly pulled into GA scope with ownership verification, rate limits, blocklists, consent, and audit logging.
