# Dmark GA Launch Roadmap

This is the current working roadmap for taking Dmark from the deployed SPF-route persistence slice to a GA-ready release candidate.

**Planning date:** 2026-05-31
**Target posture:** GA if the gates below pass; otherwise public beta with explicit scope boundaries.
**Planned launch window:** Friday 2026-06-05, not a hard date. Quality, safety, and production recoverability outrank the calendar.

## Release Principle

Dmark should only be called GA when the public promise is backed by working product flows, tested backend control planes, live-provider evidence, and an operator rollback path.

The current backend foundation is strong: scanner, dashboard contracts, Reporter primitives, Brand Risk primitives, Coolify deployment, DNS provider guidance primitives, and the SPF resolver/Bunny/persistence/orchestration slice are in place. The remaining work is to make the SPF route feature operable from the product, prove live DNS publication, and complete launch-readiness drills.

## Current Evidence

- `dmark-app` latest verified main commit: `a2cac68 chore: commit generated sqlc models`.
- GitHub Actions run for the SPF persistence/orchestration slice passed lint and tests.
- Live backend smoke checks passed against `https://dmark.kastro.is`:
  - `/healthz` returned `200 ok`.
  - `/status` returned service `Dmark` with status `operational`.
  - `/api/public/scan?domain=kastro.is` returned `200` with a fresh scan response.
- Bunny parent-zone strategy is selected:
  - Write target: parent Bunny zone `dmark.is`.
  - Bunny zone ID: `801471`.
  - Route TXT records are relative names such as `customer.route` in the parent zone, resolving publicly as `customer.route.dmark.is`.
  - Do not rely on same-provider child delegation for `route.dmark.is`; previous testing produced lame delegation/SERVFAIL.

## GA Scope Decision

The intended GA promise includes:

- Public DMARC/SPF/DKIM/BIMI scanner.
- Protected-domain monitoring/dashboard foundations.
- Guided DMARC enforcement and DNS change planning.
- Dynamic SPF flattening with Bunny-backed route publication.
- Provider-specific DNS instructions, with Icelandic providers treated as first-class.
- Production operations posture sufficient to recover from bad deploys, bad DNS publication, provider failures, and data-store restore events.

If any of these are descoped before Friday, document the scope cut explicitly in this file and in release notes. No silent narrowing of the promise.

## Weekend + Launch-Week Workstreams

### 1. SPF Route Dashboard API

**Goal:** Expose the persisted SPF route state through authenticated dashboard contracts so the frontend can manage and display it.

**Status:** Not started as product API. Backend primitives exist.

**Tasks:**

- Add authenticated endpoints in `dmark-app`:
  - `GET /api/v1/dashboard/domains/{id}/spf-route`
  - `PUT /api/v1/dashboard/domains/{id}/spf-route`
  - `POST /api/v1/dashboard/domains/{id}/spf-route/sync`
- Enforce organization/domain ownership and the correct API-key scopes.
- Return frontend-ready fields:
  - route status
  - route FQDN
  - customer CNAME name and target
  - generated SPF preview
  - last published SPF
  - last sync timestamp and error
  - enabled sources
  - evidence summary
- Add API docs and handler tests.
- Keep Bunny credentials strictly server-side.

**Acceptance criteria:**

- Tests prove authorized access, forbidden cross-org access, validation failures, successful upsert, and manual sync trigger behaviour.
- `dmark-web` can call the API through its server-side dashboard proxy without exposing secrets.

### 2. SPF Route Worker And Scheduled Resync

**Goal:** Make SPF flattening continuous, not one-shot.

**Status:** Sync service exists; worker/scheduler integration remains.

**Tasks:**

- Trigger immediate sync after route/source configuration changes.
- Add scheduled resync for enabled SPF routes every 12-24 hours.
- Ensure no Bunny write occurs when generated SPF equals the last published SPF.
- Preserve last good published state on failed or blocked syncs.
- Add logs/metrics/alert hooks for `failed`, `blocked`, and `stale` route states.
- Document operator handling for failed, blocked, and stale routes.

**Acceptance criteria:**

- Automated tests cover changed-route sync, scheduled enabled-route listing, no-op sync, failed sync, blocked sync, and last-good-state preservation.
- Operators can see and respond to failed/blocked SPF route states.

### 3. Live Bunny DNS Publication Proof

**Goal:** Prove production credentials and configuration can create, update, no-op, and safely block a real route publication.

**Status:** Parent zone and env config are set; no live route TXT publication has been proven yet.

**Tasks:**

- Create a clearly named test route, for example `hermes-test.route.dmark.is`.
- Publish a safe flattened SPF TXT value through the application path.
- Verify public DNS resolution with `dig +short TXT hermes-test.route.dmark.is`.
- Re-run sync unchanged and verify no extra Bunny write.
- Force a controlled include-resolution failure and verify publication is blocked.
- Decide whether to leave the test route as a permanent health canary or clean it up.

**Acceptance criteria:**

- Live DNS resolves the expected TXT value.
- No-op sync is observable.
- A bad route does not overwrite the last good value.
- No provider/API secrets are stored in logs, docs, memento, or commits.

### 4. dmark-web SPF Route UI

**Goal:** Give operators a clear product surface for configuring and understanding SPF routes.

**Status:** Marketing copy and handoff docs exist; full UI is not yet wired to the new backend SPF route endpoints.

**Tasks:**

- Add dashboard UI for SPF Route state.
- Add source management for include domains and literal `ip4`/`ip6` values.
- Show CNAME delegation instructions, for example:
  - `_spf.customer-domain.tld CNAME customer.route.dmark.is`
- Show sync status, last published SPF, generated preview, and error/evidence summaries.
- Add manual sync button with clear loading/error states.
- Route all backend calls through server-side `/api/dashboard/*` proxy routes.
- Update stale frontend SPF docs to match the parent-zone Bunny strategy.

**Acceptance criteria:**

- `npm run lint`, `npm test`, and `npm run build` pass.
- Browser smoke confirms the dashboard state is readable on desktop and mobile.
- No Bunny/API credentials are present in browser bundles.

### 5. DNS Provider Autopicker Surface

**Goal:** Make provider-specific DNS guidance visible and useful, especially for Icelandic operators.

**Status:** Backend provider detection/instruction primitives exist; product API/UI surfacing remains.

**Tasks:**

- Add or extend backend endpoint(s) to return provider guidance for DMARC TXT, SPF TXT, and SPF route CNAME records.
- Return:
  - detected/ambiguous/unknown status
  - provider ID and name
  - evidence nameservers
  - provider-specific instructions
  - stable DNS dashboard/deep link when available
- Validate/provider-test ISNIC, 1984, Cloudflare, Route 53, Bunny, Microsoft 365, Google/Squarespace legacy patterns.
- Resolve or explicitly mark uncertain patterns for Opin Kerfi, Origo, and Advania.
- Render guidance in `dmark-web` without claiming false certainty.

**Acceptance criteria:**

- Real representative domains produce honest detected/ambiguous/unknown results.
- Unknown providers fall back to generic instructions.
- Icelandic providers are not treated as afterthoughts.

### 6. Backend Release Candidate Gates

**Goal:** Re-run the full backend release-candidate gate after final backend work lands.

**Status:** Recent SPF slice gates passed; rerun after all GA changes.

**Commands/gates:**

- `go test ./...`
- `go test -race ./...`
- `golangci-lint run ./...`
- `govulncheck ./...`
- `make generate-check`
- `go build ./cmd/meil`
- `docker build -f infra/coolify/Dockerfile .`
- Fresh Postgres migrations.
- Fresh ClickHouse migrations.
- `MEIL_DB_INTEGRATION=1 go test ./internal/store/pg -count=1`
- GitHub Actions green on latest pushed main.

**Acceptance criteria:**

- All gates pass from a clean checkout or CI-equivalent container.
- Any local-tooling exception is documented with the exact workaround.

### 7. dmark-web Release Candidate Gates And Browser QA

**Goal:** Verify the public and dashboard browser experience before declaring GA.

**Status:** Previous dashboard/frontend work is deployed and beta-locked; final QA must run after SPF/provider UI lands.

**Commands/gates:**

- `npm run lint`
- `npm test`
- `npm run build`
- Production smoke of public scanner and marketing CTAs.
- Production smoke of dashboard locked/beta state.
- If dashboard access is enabled for the release candidate, smoke authenticated/proxy paths.
- Manual responsive pass for mobile/tablet/desktop.
- Manual accessibility pass for keyboard flow, focus states, labels, and contrast.

**Acceptance criteria:**

- No unfinished dashboard/product surface is publicly accessible unless intentionally enabled.
- Public scanner and lead-capture flows work.
- SPF/provider guidance UI is readable and actionable.

### 8. Live Provider Checks

**Goal:** Prove each GA-scoped provider integration works against a real provider or is explicitly scoped as beta/operator-managed.

**Must prove for GA unless descoped:**

- Bunny DNS route publication.
- Lettermint magic-link/transactional email path.
- DNS provider detection against real representative domains.
- DMARC report forwarding/ingestion path if Reporter is part of the GA promise.

**Can remain beta/operator-managed if explicitly documented:**

- Billing webhooks.
- Enrichment/geolocation adapters.
- Advanced Reporter attribution.

**Acceptance criteria:**

- Release notes identify which integrations are GA, beta, or operator-managed.
- No marketing or UI copy promises a provider integration that has not been proven.

### 9. Backup, Restore, Rollback, And DNS Recovery Drills

**Goal:** Make production recoverable before the launch window.

**Tasks:**

- Run and document a Postgres backup/restore drill.
- Run and document a ClickHouse restore/rebuild or retention drill.
- Run a Coolify rollback drill for the app.
- Add worker rollback notes once the SPF worker exists.
- Document SPF route rollback:
  - how to identify last published value
  - how to restore a previous Bunny TXT value
  - how to disable a route safely
  - how to recover from failed/blocked publication

**Acceptance criteria:**

- Recovery steps are documented in `docs/OPERATIONS.md` or linked from there.
- The team can roll back app code and recover last-good SPF DNS publication without guessing.

### 10. Observability And Alerting

**Goal:** Ensure operators notice user-visible failures before customers do.

**Tasks:**

- Confirm production visibility for API scan errors/latency.
- Confirm scan job backlog and worker failures are visible.
- Confirm notification failures are visible.
- Confirm DMARC parser failures are visible if Reporter is GA-scoped.
- Add SPF route failed/blocked/stale alerting path.
- Add Bunny DNS API failure visibility.

**Acceptance criteria:**

- Operators can answer: what is broken, how many domains are affected, and what changed?
- Alerts are actionable, not merely noisy.

### 11. Documentation And Release Notes

**Goal:** Keep the launch source of truth current as work lands.

**Tasks:**

- Keep this roadmap updated after every major slice.
- Update `docs/GA_READINESS.md` after release gates run.
- Update `docs/SPF_FLATTENER_BUNNY_PLAN.md` as implementation phases move from pending to complete.
- Update `dmark-web` docs so they match the parent-zone Bunny strategy.
- Prepare launch notes with:
  - final scope
  - known limitations
  - rollback plan
  - live-provider proof
  - support/incident process

**Acceptance criteria:**

- A fresh agent can read the docs and continue without needing oral history.
- GA scope, beta scope, and future work are clearly separated.

## Suggested Execution Order

1. Backend SPF route dashboard API.
2. SPF worker/scheduled resync.
3. Live Bunny publication proof.
4. dmark-web SPF route UI.
5. DNS provider guidance API/UI surfacing.
6. Backend and frontend release-candidate gates.
7. Live-provider checks.
8. Backup/restore/rollback/DNS recovery drills.
9. Observability confirmation.
10. Final docs/release notes.

## Daily Operating Rhythm Through Launch Week

- Start each work block by checking this roadmap, memento notes, git status, and latest CI/deploy state.
- Keep changes sliced small enough to test and ship independently.
- Prefer TDD for backend contracts and sync edge cases.
- Deploy after each green main push via Coolify auto-deploy, then smoke live surfaces.
- Capture non-secret outcomes in memento after major pushes or launch-gate completions.

## Launch Decision Rule

On or before Friday 2026-06-05, choose one of:

- **GA:** all must-have gates above are complete and live-provider proof is documented.
- **Public beta:** public scanner/marketing and selected dashboard flows are live, but unfinished surfaces are labelled beta/locked and GA-only promises are softened.
- **Hold:** a safety, recovery, DNS, auth, privacy, or data-loss concern remains unresolved.

The launch date is a target, not a trap. Dmark may be ambitious, but DNS is not impressed by optimism.
