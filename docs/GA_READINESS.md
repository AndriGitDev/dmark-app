# Dmark GA Readiness

This tracker records the current release posture for general availability prep. It should be updated whenever a release gate is run or a GA blocker is accepted, fixed, or moved out of scope.

Current launch roadmap: `docs/GA_LAUNCH_ROADMAP.md`. Final founder/operator evidence checklist: `docs/GA_OPERATOR_HANDOFF.md`. Step-by-step runbooks for the remaining operator gates: `docs/GA_EVIDENCE_RUNBOOKS.md`.

Last full historical local verification: 2026-05-27. Frontend cleanup spot-check: 2026-05-28. Backend launch-readiness closeout evidence: 2026-06-05. Latest engineering wave (dashboard enrichment + overhaul + launch-quality hardening): 2026-06-10.

## Current Status

- Repository state: `dmark-app` is backend-only; browser-facing surfaces and frontend build checks live in `dmark-web`.
- Latest backend main observed during 2026-06-05 18:53 UTC operator-gate pass: `752f806a533b6c373b27fe8231ee73327cd2c2d8` (`docs: record operator gate pass`), matching `origin/main`.
- Latest backend CI: GitHub Actions run `27031937186` for commit `752f806a533b6c373b27fe8231ee73327cd2c2d8` completed successfully.
- Latest frontend main observed during the 2026-06-05 redesign closeout: `4baa31eda218f6ead98c3fb11712921bf2e261cd` (`feat: refresh logo assets from original`), matching `origin/main`; no visible GitHub Actions runs exist for `dmark-web`, so local test/lint/build gates are the frontend evidence. The only local frontend untracked file at closeout was `DMARK.zip`.
- Public-site redesign posture: the approved logo assets and original-logo palette (`#44B6CD` / `#182E4D`) are implemented and pushed. Local verification passed `npm test` (`58/58`), `npm run lint`, `npm run build`, and browser scanner smoke for `kastro.is`.
- Code/product posture: public scanner, redesigned marketing surface, dashboard contracts, SPF route backend/dashboard/admin foundations, DNS guidance, operational events, privacy-safe funnel analytics, public endpoint hardening, synthetic smoke, backend CI, backend RC/service gates, frontend RC gates, unauthenticated admin/dashboard fail-closed checks, a local non-production Postgres restore drill, live Bunny SPF-route canary publication/no-op/blocking proof, and Coolify current-resource redeploy/recovery evidence have recent evidence. The product is ready for a public-beta posture if the public copy scopes unfinished live-email/previous-image-rollback/recovery claims honestly.
- GA posture: **GA declared 2026-06-10** with explicit scope. All four operator gates passed (see "Operator-Gated GA Gates" below). GA scope: public scanner, marketing funnel, dashboard read/write flows, magic-link login, provider guidance, and operator surfaces. Explicitly beta/operator-managed at GA: SPF route publication (operator dog-fooding on own domains before customer rollout), live Reporter RUA ingestion, and billing webhooks — public copy already scopes these as beta/early-access and must keep doing so until separately proven.

## Verified Automated And Read-Only Gates

Historical broad gates are summarized for provenance; the latest release-candidate evidence is the 2026-06-10 engineering wave on top of the 2026-06-05 closeout.

- 2026-06-10 engineering wave (local gates; rerun and record CI run IDs after push):
  - Backend: new dashboard endpoints (`/api/v1/dashboard/domains/{id}/trends`, `/api/v1/dashboard/domains/{id}/spf-route/history`, `/api/v1/dashboard/plan`), plan saved-domain limit enforcement on domain create/import, and journey readiness exposure in change-plan; documented in `docs/API.md`. Legacy bootstrap plan `pro` now maps to business-tier entitlements so limit enforcement cannot cap the provisioned production organization.
  - Backend toolchain: Go bumped to 1.26.4 in `go.mod`, `infra/coolify/Dockerfile`, and CI; this clears two called stdlib govulncheck findings (GO-2026-5039, GO-2026-5037) that fail the security gate on Go ≤1.26.3. `govulncheck` reports no vulnerabilities on 1.26.4.
  - Backend gates passed locally: `go test ./...`, `go test -race ./...`, `go vet`, `golangci-lint` (0 issues), `govulncheck` (clean), sqlc generated-file diff, and `DOCKER_COMPOSE=docker-compose make rc-backend-services` including new real-Postgres integration coverage for the trend/history/plan queries and cross-organization isolation.
  - Frontend: dashboard decomposed into per-tab components and fully restyled to the marketing brand system; trends sparklines, SPF sync-history timeline, plan/usage card, and journey readiness panel added; launch-quality hardening shipped (brand 404/500 pages, robots/sitemap, session-expired re-login banner on any 401, retry buttons and skeleton loaders on all panels, empty-state CTAs, mobile add-domain focus, responsive Reporter cards) and OIDC nonce verification added to both the customer dashboard and admin Authentik callbacks (fail-closed id_token nonce binding).
  - Frontend gates passed locally: `npm test` 83/83 (new coverage: Authentik session/state/nonce crypto, proxy fail-closed 401/403 and mock paths, backend path allowlist, new mappers), `npm run lint`, `npm run build`.

- 2026-06-05 18:08 UTC backend exact-head CI: run `27030871109` completed success for `ca8db82b09071aac2e8c50706508f83b58008aa3`.
- 2026-06-05 18:02 UTC backend RC gate: `PATH=/root/go/bin:$PATH make rc-backend DOCKER_COMPOSE=docker-compose` passed, including tests, race tests, vet, golangci-lint, govulncheck, generated-file check, build, Compose config validation, and Docker image build.
- 2026-06-05 18:02 UTC service-backed RC gate: `PATH=/root/go/bin:$PATH make rc-backend-services DOCKER_COMPOSE=docker-compose` passed fresh Postgres migrations through version 8, River migrations through version 6, fresh ClickHouse migrations, and `internal/store/pg` integration tests.
- 2026-06-05 18:03 UTC frontend local gates in `dmark-web`: `npm test` passed 57/57 tests; `npm run lint` passed; `npm run build` passed with `/`, `/admin`, `/dashboard`, `/api/admin/[[...path]]`, `/api/auth/*`, `/api/dashboard/[[...path]]`, `/api/dashboard/auth/*`, `/api/scan`, and `/api/upgrade` routes built.
- 2026-06-05 18:06 UTC production auth boundary smoke: `/healthz` returned `200`; unauthenticated admin provider health and dashboard domain-list endpoints returned `401` with `valid bearer API key is required`.
- 2026-06-05 18:08 UTC local non-production Postgres backup/restore drill: disposable Compose Postgres was migrated, dumped (`78591` bytes), dropped/recreated, restored, and verified via `goose_db_version` plus a restored harmless marker row. This is not production backup proof.
- 2026-06-05 18:35 UTC live Bunny SPF-route canary: application-level domain/route/sync calls created or selected canary domain `75480ff7-5a55-4188-bcf9-e2fd2388d2fb` and route `829d34cb-fe07-4b6f-a34f-3cb0299fe7be`; public DNS verified `route-published.canary.dmark.is CNAME route-published.route.dmark.is.` and `route-published.route.dmark.is TXT "v=spf1 ip4:192.0.2.10 ~all"`. A repeated no-op sync preserved the records, and a controlled bad include update left the last good TXT resolvable before the route was restored.
- 2026-06-05 18:40 UTC Lettermint live delivery proof attempt: controlled sends were rejected with Lettermint `VALIDATION_ERROR` because the team is unverified and can send only to verified team members. No tokens, message bodies, or magic links were logged. This remains a GA blocker.
- 2026-06-05 18:47-18:53 UTC Coolify current-resource redeploy/recovery drill: `dmark-web` deployment `meex2k2wwwqmzfmpj6pvmru9` finished for commit `cced9852b2bb7ce0bab7789f904ee913769e1721`; `dmark-app` deployment `srl8cozby7a1ardl0c8u45h0` finished for commit `752f806a533b6c373b27fe8231ee73327cd2c2d8`. Backend `/healthz` briefly returned `503` during deployment and recovered to `200`; final backend/frontend/DNS/synthetic smoke passed with `failed:false` at `2026-06-05T18:53:25Z`.
- 2026-06-05 17:46 UTC read-only synthetic production smoke: `go run ./cmd/meil synthetic-check --timeout=20s --json --scan-fixture-domain dmark.is --scan-fixture-domain example.com --scan-fixture-domain xn--bcher-kva.example` returned `failed:false`; checks passed for `frontend_homepage`, `backend_healthz`, default `public_scan`, `public_scan:dmark.is`, `public_scan:example.com`, and `public_scan:xn--bcher-kva.example`. No provider mutation was performed.
- 2026-06-05 17:47 UTC homepage/API marker smoke: `https://dmark.is/` returned HTML (`37196` bytes); `https://dmark.kastro.is/status` contained `operational`; `https://dmark.kastro.is/healthz` returned `ok`; `https://dmark.kastro.is/api/public/scan?domain=kastro.is` returned JSON containing `kastro.is` with top-level keys `audit,checked_at,checks,domain,duration_ms,summary`.
- 2026-06-05 17:47 UTC homepage source marker check: `dmark-privacy-analytics`, Google Tag Manager, Google Analytics, Meta/Facebook script, and Meta pixel markers were all absent.
- 2026-06-05 17:47 UTC frontend local gates in `dmark-web`: `npm test` passed 57/57 tests; `npm run lint` passed; `npm run build` passed with `/`, `/admin`, `/dashboard`, `/api/admin/[[...path]]`, `/api/auth/*`, `/api/dashboard/[[...path]]`, `/api/dashboard/auth/*`, `/api/scan`, and `/api/upgrade` routes built.
- Earlier backend RC/service gates: `make rc-backend DOCKER_COMPOSE=docker-compose` and `make rc-backend-services DOCKER_COMPOSE=docker-compose` have recent closeout evidence from the launch-readiness workstream; rerun them before a final GA/public-beta declaration if code changes after this point.
- Earlier historical local gates from 2026-05-27/28 included `make qa`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `golangci-lint run`, `govulncheck ./...`, `make generate-check`, `make fuzz-smoke`, backend build, Docker image build, fresh Postgres migrations, fresh ClickHouse migrations, and frontend cleanup spot checks.

## Operator-Gated GA Gates — All Passed

All four gates from `docs/GA_EVIDENCE_RUNBOOKS.md` passed; GA was declared on 2026-06-10. Detailed evidence (timestamps, message IDs, dump sizes, deployment IDs) is held by the operator per the runbook evidence lines and should be appended here when transcribed.

1. **Bunny DNS route publication** — create/update/no-op/blocking canary proof passed 2026-06-05 18:35 UTC with public DNS verification; rollback coverage closed by the previous-image rollback drill (gate 3). Kept in beta scope at GA for operator dog-fooding (see below).
2. **Lettermint live magic-link delivery** — operator-confirmed passed 2026-06-10 per Runbook 1 (team verified, controlled send to an operator-owned address delivered; no links or tokens recorded).
3. **Recovery drills** — operator-confirmed passed 2026-06-10 per Runbooks 2 and 3: production-target Postgres backup/restore into a scratch database, and previous-image Coolify rollback/roll-forward for app and worker. ClickHouse restore was not required because Reporter ingestion is not GA-scoped.
4. **Public copy lock** — approved by Andri 2026-06-10 per Runbook 4 with **no copy changes**: SPF flattening intentionally remains "beta/early access" while the operator dog-foods route publication on his own domains; reporting remains "roadmap"; billing remains "request early access". Copy lock evidence line: `Copy lock 2026-06-10: public claims reviewed against proven capabilities; SPF flattening deliberately kept in beta for operator dog-fooding; approved by Andri.`

Billing webhooks, enrichment/geolocation, and DMARC mailbox/report forwarding remain out of GA scope until separately proven live.

## GA Scope

- **GA:** public scanner, marketing funnel, dashboard read/write flows (domains, trends, change plan, DNS guidance, alerts, settings, plan/usage), magic-link login, provider guidance, owner-only admin and read-only operator surfaces, and supportable manual onboarding.
- **Beta/operator-managed at GA:** Bunny-backed SPF route publication (operator dog-fooding on own domains before customer rollout), live Reporter RUA ingestion, and billing webhooks. Public copy keeps these labelled beta/early-access/roadmap.
- **Hold condition (unchanged):** any unresolved safety, auth, privacy, data-loss, DNS, provider-secret, or rollback concern triggers a hold.

## Immediate Next Workstream

1. Push `dmark-app` and `dmark-web` main to origin and record the exact-head backend CI run ID here (the 2026-06-10 engineering wave is currently local-gate evidence only).
2. Transcribe the operator evidence lines for gates 2-3 (timestamps, message ID, dump size, deployment IDs) into the gate list above.
3. Run the post-deploy production smoke (`go run ./cmd/meil synthetic-check --json` plus homepage/API marker checks) after the next deploy and record the result.
4. SPF flattening dog-food: onboard the operator's own domains through the dashboard SPF route flow, watch the sync-history timeline and `spf-route audit`, and record graduation criteria before offering route publication to customers.

Proof-email dispatch remains isolated unless explicitly pulled into scope with ownership verification, rate limits, blocklists, consent, and audit logging.
