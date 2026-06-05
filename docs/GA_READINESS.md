# Dmark GA Readiness

This tracker records the current release posture for general availability prep. It should be updated whenever a release gate is run or a GA blocker is accepted, fixed, or moved out of scope.

Current launch roadmap: `docs/GA_LAUNCH_ROADMAP.md`. Final founder/operator evidence checklist: `docs/GA_OPERATOR_HANDOFF.md`.

Last full historical local verification: 2026-05-27. Frontend cleanup spot-check: 2026-05-28. Latest backend launch-readiness closeout evidence: 2026-06-05.

## Current Status

- Repository state: `dmark-app` is backend-only; browser-facing surfaces and frontend build checks live in `dmark-web`.
- Latest backend main observed during 2026-06-05 18:53 UTC operator-gate pass: `752f806a533b6c373b27fe8231ee73327cd2c2d8` (`docs: record operator gate pass`), matching `origin/main`.
- Latest backend CI: GitHub Actions run `27031937186` for commit `752f806a533b6c373b27fe8231ee73327cd2c2d8` completed successfully.
- Latest frontend main observed during the 2026-06-05 redesign closeout: `4baa31eda218f6ead98c3fb11712921bf2e261cd` (`feat: refresh logo assets from original`), matching `origin/main`; no visible GitHub Actions runs exist for `dmark-web`, so local test/lint/build gates are the frontend evidence. The only local frontend untracked file at closeout was `DMARK.zip`.
- Public-site redesign posture: the approved logo assets and original-logo palette (`#44B6CD` / `#182E4D`) are implemented and pushed. Local verification passed `npm test` (`58/58`), `npm run lint`, `npm run build`, and browser scanner smoke for `kastro.is`.
- Code/product posture: public scanner, redesigned marketing surface, dashboard contracts, SPF route backend/dashboard/admin foundations, DNS guidance, operational events, privacy-safe funnel analytics, public endpoint hardening, synthetic smoke, backend CI, backend RC/service gates, frontend RC gates, unauthenticated admin/dashboard fail-closed checks, a local non-production Postgres restore drill, live Bunny SPF-route canary publication/no-op/blocking proof, and Coolify current-resource redeploy/recovery evidence have recent evidence. The product is ready for a public-beta posture if the public copy scopes unfinished live-email/previous-image-rollback/recovery claims honestly.
- GA posture: do **not** declare GA until the remaining human/operator-gated live Lettermint proof, production/approved backup restore evidence, previous-image Coolify rollback evidence if claimed, and public-copy approval items in `docs/GA_OPERATOR_HANDOFF.md` pass and are recorded. Current recommended posture before those gates is public beta, not GA.

## Verified Automated And Read-Only Gates

Historical broad gates are summarized for provenance; the latest release-candidate evidence is the 2026-06-05 closeout.

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

## Remaining Human/Operator-Gated GA Gates

These must pass before a GA release candidate is promoted. They require explicit operator approval, production-provider credentials, console work, live provider mutation, or restore/rollback authority.

1. Bunny DNS route canary proof through the application path: create/update/no-op/blocking publication behaviour in the parent `dmark.is` zone, public DNS verification, and rollback evidence. The create/update/no-op/blocking canary proof passed on 2026-06-05 18:35 UTC with public DNS verification; keep the canary in smoke coverage or run an explicit delete/restore rollback before claiming full rollback coverage.
2. Lettermint live email/magic-link delivery test to an operator-owned address without logging magic links or tokens. Attempted on 2026-06-05 18:40 UTC, but Lettermint rejected sends because the team is unverified and can send only to verified team members. Complete Lettermint team/domain/recipient verification or provide a verified-team-member target, then rerun without recording the magic link.
3. Recovery drills: production/approved-target Postgres backup/restore, ClickHouse restore/rebuild/retention if Reporter is GA-scoped, previous-image Coolify rollback for app/worker if rollback is claimed, and SPF route rollback from the Bunny canary. A local disposable Postgres restore drill passed on 2026-06-05 18:08 UTC and a current-resource Coolify redeploy/recovery drill passed on 2026-06-05 18:53 UTC, but they do not replace production backup evidence or a previous-image rollback proof.
4. Final public copy/release-note approval: promises must match proven capabilities; unfinished live-email, previous-image rollback, provider/recovery/billing/Reporter surfaces must be locked, labelled beta/operator-managed, or held.
5. Billing webhooks, enrichment/geolocation, DMARC mailbox/report forwarding, and advanced Reporter flows only if they are explicitly pulled into the launch scope and proven live.

## Public-Beta Scope If Human Gates Are Not Completed

Public beta is the recommended posture until the gates above are proven. The redesign is suitable for launch traffic, but GA language should wait until the remaining live-provider/recovery/copy gates pass.

- Launchable: public scanner, marketing funnel, selected dashboard/read flows, provider guidance, owner-only admin/read-only operator surfaces, and supportable manual onboarding.
- Beta/operator-managed: Bunny-backed SPF route publication, live email/provider delivery, billing webhooks, advanced Reporter ingestion, and recovery claims.
- Hold condition: any unresolved safety, auth, privacy, data-loss, DNS, provider-secret, or rollback concern.

## Immediate Next Workstream

Stop here before human-gated work unless Andri explicitly approves the next gate. The next action is to choose one of:

1. Run the operator-approved Bunny route canary mutation and rollback proof.
2. Run the operator-approved Lettermint live email proof.
3. Run the operator-approved backup/restore/Coolify rollback drills.
4. Approve public-beta copy/scope and publish with the beta limitations above.
5. Run the post-redesign public-site QA pass: exact production release-candidate verification, responsive/accessibility/performance/SEO checks, social-preview metadata, and CTA/upgrade smoke.

Proof-email dispatch remains isolated unless explicitly pulled into GA scope with ownership verification, rate limits, blocklists, consent, and audit logging.
