# Dmark GA Launch Roadmap

This is the current working roadmap for taking Dmark from the deployed SPF-route persistence slice to a GA-ready release candidate.

**Planning date:** 2026-05-31
**Target posture:** GA if the gates below pass; otherwise public beta with explicit scope boundaries.
**Planned launch window:** Friday 2026-06-05, not a hard date. Quality, safety, and production recoverability outrank the calendar.

## Release Principle

Dmark should only be called GA when the public promise is backed by working product flows, tested backend control planes, live-provider evidence, and an operator rollback path.

The current backend foundation is strong: scanner, dashboard contracts, Reporter primitives, Brand Risk primitives, Coolify deployment, DNS provider guidance primitives, and the SPF resolver/Bunny/persistence/orchestration slice are in place. The remaining work is to make the SPF route feature operable from the product, prove live DNS publication, and complete launch-readiness drills.

## Current Evidence

- `dmark-app` latest observed main commit: `1e7a6b3c899b3b80f644e0d82d22de60416c5e5c` (`docs: record live operator gate evidence`), matching `origin/main` on 2026-06-05.
- `dmark-web` latest observed main commit: `4baa31eda218f6ead98c3fb11712921bf2e261cd` (`feat: refresh logo assets from original`), matching `origin/main` on 2026-06-05; the only local frontend untracked file is `DMARK.zip`.
- The redesigned public site has shipped in `dmark-web` with the approved logo assets and sampled original-logo palette (`#44B6CD` cyan, `#182E4D` navy). Local verification for the redesign passed `npm test` (`58/58`), `npm run lint`, `npm run build`, and browser scanner smoke for `kastro.is`.
- Latest backend CI/release-gate evidence is recorded in `docs/GA_READINESS.md` and `docs/GA_OPERATOR_HANDOFF.md`; rerun exact-head gates after any code changes before a final GA decision.
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

**Status:** Sync service exists, the River worker kind is registered for queued/manual route syncs, tested scheduler primitives compute due SPF route sync jobs on a 12-hour default cadence, and the Coolify worker service now runs a scheduler loop that enqueues due route sync jobs every 15 minutes by default. Further operator visibility for failed/blocked/stale states remains open.

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

**Status:** Parent zone and env config are set; no live route TXT publication has been proven yet. Any permanent health canary should be provisioned once through an explicit operator change and then used only through read-only DNS/synthetic smoke.

**Tasks:**

- Create a clearly named test route, for example `hermes-test.route.dmark.is`.
- Publish a safe flattened SPF TXT value through the application path.
- Verify public DNS resolution with `dig +short TXT hermes-test.route.dmark.is`.
- Re-run sync unchanged and verify no extra Bunny write.
- Force a controlled include-resolution failure and verify publication is blocked.
- Decide whether to leave the test route as a permanent read-only health canary, preferably `route-published.canary.dmark.is` -> `route-published.route.dmark.is`, or clean it up.

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

**Status:** Backend provider detection/instruction primitives exist. The dashboard API now exposes an authenticated `/api/v1/dashboard/domains/{id}/dns-guidance` contract with detected/ambiguous/unknown provider status, evidence nameservers, generic fallback instructions, and SPF route CNAME guidance when route data exists. Frontend rendering is in place in `dmark-web`. Representative backend fixture coverage now includes ISNIC, 1984, Cloudflare, Route 53, Bunny, Microsoft Azure DNS, and Google Domains legacy nameservers, with live NS spot-checks for `microsoft.com`, `dmark.is`, `kastro.is`, `cloudflare.com`, and `amazon.com`. Opin Kerfi, Origo, Advania, and Squarespace-specific hosted-DNS patterns remain explicitly uncertain until conservative production evidence is available.

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

### 6. Product Trust, Auditability, And Operator Control Plane

**Goal:** Add the highest-leverage pre-GA surfaces that make Dmark operable, supportable, and trustworthy before broad launch.

**Status:** New pre-GA hardening workstream added after roadmap review on 2026-05-31. Some primitives exist in logs, scans, provider detection, status, and dashboard contracts; the product/operator surfaces are not yet complete. Backend operator primitives now include owner-only operational/provider-health endpoints, SPF route publication audit, the `meil synthetic-check` public-flow/DNS monitoring command, optional repeated `--scan-fixture-domain` public-scan smoke fixtures with a documented production-safe fixture baseline, customer-facing remediation explanations on scan check responses, protected-domain scan evidence snapshots persisted on latest check APIs, durable public scan operational events, sanitized audit-claim operational events for created/blocked/failed public claim flows, best-effort API-key-use operational events, best-effort dashboard operational events for authenticated rechecks plus SPF route upsert/sync queue paths, best-effort SPF route worker publication outcome events for succeeded/blocked/failed syncs, public-scan rate limiting that ignores malformed `X-Forwarded-For` values instead of allowing spoofed arbitrary bucket keys, secure default HTTP response headers on backend routes, request ID middleware that rejects unsafe client-supplied IDs instead of echoing whitespace/overlong/path-like values into responses or operational metadata, strict single-object JSON parsing for public scan and audit-claim POST bodies so trailing or multiple payloads cannot be ambiguously accepted, scanner domain normalization that rejects IP-literal public scan inputs before DNS lookups, Coolify Compose build-arg pass-through for `/status` version/commit/build-time metadata, and a dmark-web privacy analytics foundation that is disabled by default and allowlists Plausible/Umami/self-hosted PostHog-style script configuration without exposing analytics API keys.

**Tasks:**

- Add synthetic monitoring for real public flows, not only liveness:
  - `GET https://dmark.is`
  - `GET https://dmark.kastro.is/healthz`
  - `GET /api/public/scan?domain=kastro.is`
  - authoritative/recursive DNS checks for Bunny route behaviour
  - response schema/freshness checks where practical
- Add a DNS publication audit command or protected admin endpoint that answers, for a domain/route:
  - expected TXT/CNAME records
  - current persisted route state and Bunny record IDs
  - live authoritative DNS result
  - live recursive DNS result
  - propagation/drift status
  - last publish attempt and last known good value
- Add durable operational events in Postgres for important product/control-plane activity:
  - scan started/completed
  - recommendation generated
  - SPF route generated
  - DNS publish attempted/succeeded/failed/blocked/no-op
  - customer/domain claimed
  - API key used
  - bootstrap ran/skipped
  - external provider error
- Add a minimal protected admin console, intentionally boring, for:
  - org/domain lookup
  - latest scans
  - SPF route/DNS publication state
  - provider health
  - recent operational events/errors
  - manual re-run scan
  - manual SPF route sync/republish
- Add a provider/dependency health page or endpoint covering:
  - Bunny API reachability
  - configured Bunny zone IDs
  - Postgres reachability and migration version
  - ClickHouse reachability/retention status if Reporter is GA-scoped
  - outbound DNS resolver health
  - public base URLs
  - bootstrap configured/skipped state
  - frontend proxy API-key auth health
  - deployed version/build metadata
- Keep `/status` non-secret build metadata trustworthy in production images:
  - git commit
  - build time
  - app version
- Add internal-only operator status fields for:
  - DB migration version
  - environment name
  - public base URL
  - relevant feature flags
  - worker/queue state
- Store evidence snapshots for every scan/recommendation:
  - DNS answers
  - queried resolvers
  - timestamps
  - MX results
  - SPF include chain
  - DMARC record
  - DKIM selector checks where applicable
  - parser warnings
  - recommendation inputs sufficient to audit why the result was produced
- Add a customer-facing explanation layer for scanner/recommendation output:
  - what the result means
  - severity and confidence
  - exact DNS record to add/change
  - provider-specific instructions
  - expected propagation timing
  - clear next action/CTA
- Add known-good domain fixtures for regression tests and production smoke checks:
  - valid DMARC/SPF/DKIM domain
  - broken SPF domain
  - SPF-too-many-lookups domain
  - missing DMARC domain
  - parked/no-MX domain
  - internationalized domain
  - route-published customer fixture
  - backend synthetic smoke now accepts repeated `--scan-fixture-domain` values so these fixtures can be exercised in one read-only operator run; the production fixture registry now reserves `broken-spf.canary.dmark.is`, `spf-too-many-lookups.canary.dmark.is`, `parked-no-mx.canary.dmark.is`, and `route-published.canary.dmark.is` as Dmark-owned canary names, but those records still need explicit out-of-band provisioning before they become enforced live smoke gates. Fixture-domain checks now support optional `--scan-fixture-expect-status domain=pass|warn|fail|missing|unknown` assertions so provisioned canaries can become class/status release gates without live DNS mutation.
- Add a security/abuse pass for public scan and dashboard-adjacent endpoints:
  - rate limits by IP/domain
  - strict domain normalization
  - DNS/internal-network guardrails for any network fetches
  - locked-down CORS
  - security headers
  - request IDs throughout
  - no secret-like values in logs
- Add privacy-respecting funnel analytics sufficient to understand launch performance:
  - homepage visits
  - scan starts/completions (public scanner lifecycle events are wired in `dmark-web` with only coarse status/source properties and no raw domains, emails, DNS records, URLs, or error text)
  - claim starts/completions/errors (wired in `dmark-web` upgrade modal with only coarse source/status properties)
  - recommendation views (wired when public scanner results render, without raw domains or records)
  - CTA clicks/bookings (CTA clicks wired for nav/pricing/footer/result upsell sources without plan, domain, email, or name payloads)
  - dashboard/beta-lock interactions (dashboard interaction events are wired in `dmark-web` for navigation, domain rechecks, SPF route sync/source saves, alert actions, settings saves, and exports with coarse source/status only; beta-lock CTA clicks are measured for public-scan and early-access exits with coarse source only)
  - Prefer Plausible, Umami, self-hosted PostHog, or similarly privacy-conscious tracking over invasive analytics.
- If scan/recommendation/route work starts stretching request lifecycles, introduce a simple Postgres-backed job table before adding new infrastructure:
  - status, attempts, retry-after, idempotency key, locked-by/locked-at
  - keep Redis or heavier queue infrastructure out until it is clearly needed.

**Acceptance criteria:**

- Operators can answer: what happened, what DNS should exist, what DNS actually exists, what changed, and how to recover.
- Customers receive understandable, evidence-backed remediation guidance rather than raw scanner output.
- Public flows have abuse controls and privacy-conscious measurement.
- At least the synthetic scanner/DNS checks and DNS publication audit path are in place before calling the SPF route feature GA.

### 7. Backend Release Candidate Gates

**Goal:** Re-run the full backend release-candidate gate after final backend work lands.

**Status:** Recent SPF slice gates passed; rerun after all GA changes. `make rc-backend` now wraps the backend release-candidate command ladder for a clean local/CI-equivalent pass: full tests, race tests, vet, golangci-lint, govulncheck, sqlc generated-file check, CLI build, Compose config validation, and Docker image build. `make rc-backend-services` now wraps the service-backed gate for dev Postgres/ClickHouse readiness, fresh migrations, and `MEIL_DB_INTEGRATION=1 go test ./internal/store/pg -count=1`. On Havana, pass `DOCKER_COMPOSE=docker-compose` unless the Compose v2 plugin is installed.

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
- `make rc-backend-services`.
- `MEIL_DB_INTEGRATION=1 go test ./internal/store/pg -count=1`
- GitHub Actions green on latest pushed main.

**Acceptance criteria:**

- All gates pass from a clean checkout or CI-equivalent container.
- Any local-tooling exception is documented with the exact workaround.

### 8. dmark-web Release Candidate Gates And Browser QA

**Goal:** Verify the public and dashboard browser experience before declaring GA.

**Status:** The public marketing redesign is implemented and pushed at `4baa31eda218f6ead98c3fb11712921bf2e261cd`, using the approved transparent logo assets and original-logo palette. Local redesign verification passed `npm test` (`58/58`), `npm run lint`, `npm run build`, and browser smoke of the scanner. Previous dashboard/frontend work remains deployed and beta-locked; final full-product QA must still run after SPF/provider UI lands and after the production deployment is confirmed at the exact release candidate.

**Commands/gates:**

- `npm run lint`
- `npm test`
- `npm run build`
- Production smoke of public scanner and marketing CTAs.
- Production smoke that the redesigned homepage is served from the intended release candidate and uses the approved logo/palette assets.
- Production smoke of dashboard locked/beta state.
- If dashboard access is enabled for the release candidate, smoke authenticated/proxy paths.
- Manual responsive pass for mobile/tablet/desktop.
- Manual accessibility pass for keyboard flow, focus states, labels, and contrast.
- Lighthouse or equivalent performance/SEO/accessibility spot-check for the public homepage and scanner result state.

**Acceptance criteria:**

- No unfinished dashboard/product surface is publicly accessible unless intentionally enabled.
- Public scanner and lead-capture flows work.
- Public marketing copy, logo, palette, responsive layout, and CTA paths match the approved release candidate.
- SPF/provider guidance UI is readable and actionable.

### 9. Live Provider Checks

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

### 10. Backup, Restore, Rollback, And DNS Recovery Drills

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

### 11. Observability And Alerting

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

### 12. Documentation And Release Notes

**Goal:** Keep the launch source of truth current as work lands.

**Tasks:**

- Keep this roadmap updated after every major slice.
- Update `docs/GA_READINESS.md` after release gates run.
- Use `docs/GA_OPERATOR_HANDOFF.md` for the final founder/operator evidence packet, live-provider proof steps, and GA/public-beta/hold decision.
- Update `docs/SPF_FLATTENER_BUNNY_PLAN.md` as implementation phases move from pending to complete.
- Update `dmark-web` docs so they match the parent-zone Bunny strategy.
- Prepare launch notes with:
  - final scope
  - known limitations
  - public-beta versus GA language
  - rollback plan
  - live-provider proof
  - support/incident process
- Freeze public-site launch copy after the redesign smoke passes:
  - hero and scanner promise
  - pricing/CTA language
  - beta/GA qualifiers
  - privacy/analytics wording
  - support/contact path

**Acceptance criteria:**

- A fresh agent can read the docs and continue without needing oral history.
- GA scope, beta scope, and future work are clearly separated.

## Recommended Additional Pre-GA Stages After The Redesign

The redesign materially improves first impression, but GA still needs a few release stages that are easy to overlook because they are not a single backend feature. These stages should be treated as pre-GA gates when the public promise depends on them; otherwise mark them public-beta or operator-managed.

### A. Public-site copy, scope, and conversion freeze

**Why:** The new design is strong enough to drive launch traffic; the words now need to be as precise as the product.

**Acceptance criteria:** public copy distinguishes GA, beta, and operator-managed capabilities; CTA/upgrade flows are tested; no copy promises live email, rollback, billing, Reporter, or SPF automation beyond proven evidence.

### B. Accessibility, performance, and SEO/metadata pass

**Why:** A polished homepage can still fail launch quality if it is slow, inaccessible, or hard to preview/share.

**Acceptance criteria:** mobile/desktop responsive checks pass; keyboard/focus/contrast checks pass; Lighthouse or equivalent does not reveal launch-blocking accessibility/performance/SEO issues; Open Graph/Twitter/social preview metadata and favicon/logo assets are verified.

### C. Release-candidate deploy verification

**Why:** Local gates are necessary but not sufficient; the exact pushed frontend/backend commits must be observed in production before GA language goes live.

**Acceptance criteria:** production serves the intended `dmark-web` and `dmark-app` release candidates; homepage/scanner/dashboard-beta/admin-boundary smokes pass; invasive analytics markers remain absent unless intentionally enabled.

### D. Founder/operator support readiness

**Why:** Public beta can be operated manually, but GA needs support and incident handling to be boring.

**Acceptance criteria:** support/contact path works; operator can inspect scans, route state, provider health, operational events, and recent errors; incident/recovery ownership is explicit for the first launch week.

### E. Post-launch monitoring and rollback watch

**Why:** The first 24-72 hours after launch are part of launch, not an afterthought.

**Acceptance criteria:** synthetic checks, provider health, scan errors, route publication status, and conversion/CTA telemetry are reviewed on a defined cadence; rollback/hold criteria are written before traffic arrives.

## Suggested Execution Order

1. Backend SPF route dashboard API.
2. SPF worker/scheduled resync.
3. Live Bunny publication proof.
4. DNS publication audit path and synthetic scanner/DNS monitoring.
5. dmark-web SPF route UI.
6. DNS provider guidance API/UI surfacing.
7. Evidence snapshots, operational events, and customer-facing explanations.
8. Minimal admin/provider-health/version surfaces.
9. Security/abuse pass and privacy-respecting funnel analytics.
10. Public-site copy/scope freeze plus accessibility, performance, SEO, and social-preview QA.
11. Backend and frontend release-candidate gates.
12. Exact production release-candidate deploy verification.
13. Live-provider checks.
14. Backup/restore/rollback/DNS recovery drills.
15. Observability confirmation and first-72-hours monitoring cadence.
16. Final docs/release notes and GA/public-beta decision.

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
