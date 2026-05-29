# Dmark Product Roadmap

Dmark is a DMARC, email-authentication, and brand-name risk defense product for domain owners.

It starts with public DNS, SPF, DKIM, DMARC, and BIMI posture checks. It then grows into protected-domain monitoring, drift alerts, DMARC aggregate report ingestion, sender attribution, and brand-name risk defense for spoofing and impersonation risks.

**Status:** the roadmap breadth documented here is implemented and audited. This file is now a historical product record; current browser/frontend work lives in `dmark-web` and the backend implementation evidence lives in `docs/ROADMAP_AUDIT.md`.

## Current Baseline

The repository already contains these working surfaces:

- Single Go binary with `serve`, `scan`, `migrate`, `worker`, and `ingest` command boundaries.
- Backend-only HTTP surface; browser routes are handed off to `dmark-web`.
- Scanner package for SPF, DMARC, BIMI, and common DKIM selector probing.
- Passwordless auth service with hashed single-use login tokens.
- Lettermint-backed magic-link email sender.
- API foundations for protected domains, scanning, reporting, billing, teams, and admin operations.
- Postgres schema for users, login tokens, domains, and domain check snapshots.
- sqlc-generated Postgres query layer.
- Coolify deployment assets and local Docker Compose development services.
- Unit and handler tests for scanner, auth, domains, notifier, CLI scan output, API routes, and browser route handoff.

The repository now includes package boundaries and implementation slices for drift detection, jobs, ingestion, attribution, ClickHouse storage, and web middleware. It intentionally excludes the old in-repo frontend build; browser UI and marketing source live in `dmark-web`.

The competitive 90-day SaaS roadmap lives in `docs/COMPETITIVE_ROADMAP.md`. That roadmap positions Dmark as the Icelandic Dmarcian/EasyDMARC alternative for SMB and midmarket companies, with concierge DMARC enforcement, monthly trust reports, and `.is` Defender coverage as the first local beat points.

## Product Principles

- Public checks must remain useful without signup.
- Account-backed monitoring should reduce operator toil, not just display DNS records.
- Alerting must explain the material risk and changed record, not only say "failed".
- Reporter features should be built on DMARC evidence and preserve raw source material for auditability.
- The paid tier should unlock automation, history, attribution, collaboration, and higher-volume monitoring rather than hiding the basic checker.

## Phase 1: Scanner And API Reliability

Goal: make the scanner and backend API trustworthy enough to back the `dmark-web` top-of-funnel product and operator workflows.

Features:

- Improve SPF parsing beyond simple string checks: mechanisms, redirects, includes, DNS lookup count, softfail/hardfail, `+all`, `?all`, `ptr`, duplicate records, and syntax errors.
- Improve DMARC parsing: policy inheritance, `sp`, `pct`, `adkim`, `aspf`, `rua`, `ruf`, reporting URI validation, and malformed tag detection.
- Improve BIMI parsing: SVG location validation, VMC/CMC certificate URI presence, DMARC enforcement prerequisite, and common publishing mistakes.
- Expand DKIM support with custom selector entry, provider presets, selector result display, and key-strength warnings where detectable.
- Add a normalized score or posture summary that explains the highest-priority issue first.
- Add clear remediation text and provider-neutral example records.
- Add scanner result metadata: checked timestamp, DNS lookup duration, resolver error class, and record source name.
- Add copyable CLI-friendly output and JSON output for `meil scan`.
- Keep one-off API checks persistence-free by default unless they are explicitly attached to a protected domain.

Engineering:

- Replace ad hoc scanner classification with parser-oriented helpers and table-driven tests.
- Add bounded concurrency for DKIM selector probes.
- Add API-level abuse controls for scan requests.
- Add structured scanner errors that distinguish not found, timeout, malformed record, and resolver failure.
- Keep browser result components and responsive layouts in `dmark-web`.

Definition of Done:

- API scan handles common valid and invalid DNS configurations without hanging.
- `meil scan <domain>` and API scan share the same scanner engine and status semantics.
- Scanner package has table-driven unit coverage for SPF, DKIM, DMARC, BIMI, normalization, and timeout behavior.
- `dmark-web` owns mobile/browser presentation for long records.

## Phase 2: Saved Domains Completion

Goal: make the authenticated saved-domain workflow reliable for real users.

Features:

- Complete login lifecycle polish: resend link, expired-token screen, consumed-token screen, logout confirmation behavior, and session renewal.
- Add account dashboard states: empty state, failed scan state, scan in progress, last checked, and next scheduled check.
- Add custom DKIM selectors per saved domain.
- Add manual recheck feedback without losing the user's place.
- Add domain ownership verification before enabling automation-heavy features.
- Add domain-level settings: notification email, selectors, expected senders, scan cadence, and ignored findings.
- Add historical checks per saved domain with filters by status and check type.
- Add domain import from CSV and export of protected domains and latest checks.

Engineering:

- Add CSRF protection for state-changing routes.
- Add database integration tests for auth token consumption, domain uniqueness, latest-check selection, and delete cascades.
- Add background scan job skeleton with idempotency keys and retry policy.
- Add app-level flash messages and consistent error rendering instead of raw `http.Error` pages.
- Add structured logging with request IDs.

Definition of Done:

- A user can sign in, save domains, configure selectors, recheck, delete, and review latest results.
- Auth flows are covered by unit, integration, and handler tests.
- State-changing routes have CSRF protection.
- Saved-domain data access is covered by database-backed tests.

## Phase 3: Drift Detection And Alerts

Goal: turn snapshots into monitoring by detecting meaningful changes and notifying users.

Features:

- Scheduled scans for protected domains.
- Drift classifier for material changes: record missing, policy relaxed, include removed, DKIM selector removed, BIMI removed, DNS lookup count exceeded, and status regressions.
- Alert rules by domain and check type.
- Email notifications with before/after records, severity, and recommended action.
- Notification digest mode for low-priority changes.
- Alert acknowledgement, mute, and suppression windows.
- Incident timeline per domain.
- Weekly posture summary for protected domains.

Engineering:

- Implement River worker role and queue-backed scan jobs.
- Add drift package with deterministic snapshot comparison.
- Persist drift events separately from raw check snapshots.
- Add notification dispatch audit table.
- Add retry, backoff, and dead-letter behavior for scanner and notification jobs.
- Add metrics for scan duration, DNS failures, alert volume, and job queue latency.

Definition of Done:

- Protected domains are scanned on schedule without web requests.
- Material drift creates a durable event and sends the correct notification once.
- Users can acknowledge or mute noisy findings.
- Job processing is observable and failures are retryable.

## Phase 4: Reporter Ingestion

Goal: ingest DMARC aggregate reports and show authenticated sending sources.

Features:

- Per-domain RUA mailbox/address provisioning or documented forwarding target.
- DMARC aggregate report ingestion for zipped and gzipped XML attachments.
- XML parser for common report generator variants.
- Report validation, deduplication, and raw report retention.
- ClickHouse-backed storage for aggregate rows.
- Reporter dashboard: sending IPs, organizations, aligned/pass/fail counts, disposition, source country/ASN, and date ranges.
- Domain-level sender inventory and unknown-sender detection.
- Report health: last report received, reporter organizations, missing expected reports, and ingestion failures.

Engineering:

- Implement `ingest` command and ingestion package.
- Add ClickHouse migration/setup path and local development service.
- Add parser fixtures from sanitized real-world reports.
- Add idempotent report import keyed by report metadata and content hash.
- Add data retention policy for raw reports and aggregate rows.
- Add ingestion metrics and dead-letter review workflow.

Definition of Done:

- Users can configure DMARC reporting and see aggregate sender data by day.
- Duplicate reports are ignored safely.
- Parser supports representative reports from major mailbox providers.
- Aggregate storage supports date-range queries at expected volumes.

## Phase 5: Sender Attribution And Intelligence

Goal: help users understand whether sending sources are legitimate and what to do next.

Features:

- Source IP enrichment: reverse DNS, ASN, geolocation, cloud/provider mapping, and known ESP identification.
- Expected sender allowlist per domain.
- Unknown sender alerts based on aggregate reports.
- Alignment explanations for SPF and DKIM failures.
- Recommendations to add, remove, or fix sending services.
- Sender inventory over time: first seen, last seen, volume, failure rate, and confidence.
- Risk scoring that combines DNS posture, drift, and report evidence.

Engineering:

- Build attribution package around deterministic provider rules plus enrichment caches.
- Add enrichment refresh jobs and cache expiration.
- Preserve attribution confidence and evidence.
- Add tests for provider matching and ambiguous sender classification.

Definition of Done:

- Reporter can separate known providers from suspicious or unclassified sources.
- Users can mark a sender as expected, ignored, or suspicious.
- Attribution decisions are explainable and test-covered.

## Phase 6: Paid Product, Teams, And Billing

Goal: turn Dmark into a sustainable SaaS product with collaboration and plan boundaries.

Features:

- Plans for free Checker, monitored domains, Reporter, and higher-volume usage.
- Billing provider integration.
- Team accounts, invitations, roles, and domain ownership transfer.
- Usage limits for protected domains, scan cadence, report retention, and alert channels.
- Webhooks and Slack notifications for paid tiers.
- Audit log for security-sensitive actions.
- API keys for programmatic checks and exports.
- Account deletion, data export, and billing cancellation flows.

Engineering:

- Add organization model separate from users.
- Add RBAC middleware and permission tests.
- Add entitlement checks at service boundaries.
- Add billing webhook verification and idempotent subscription updates.
- Add audit log table and append-only write path.

Definition of Done:

- Plan limits are enforced server-side.
- Team roles protect domain settings and billing actions.
- Billing lifecycle updates are idempotent and observable.

## Phase 7: Platform Hardening And Compliance

Goal: make Dmark reliable, secure, and maintainable as customer data and background volume grow.

Features:

- Operational status page and incident process.
- User-facing data retention controls.
- Admin console for support and abuse handling.
- Security page with disclosure contact and subprocessors.
- Documentation for DNS setup, DMARC reporting, and alert interpretation.

Engineering:

- CI gates for lint, tests, generated files, migrations, container build, and security checks.
- Production observability: logs, metrics, traces, dashboards, and alerting.
- Backup and restore drills for Postgres and ClickHouse.
- Load tests for API scan, scan jobs, and report queries.
- Threat model for auth, API scan abuse, report ingestion, and billing webhooks.
- Secrets management and environment validation.
- Dependency update process and vulnerability triage.

Definition of Done:

- Production deploys are repeatable and reversible.
- Core flows have automated coverage and documented manual QA gates.
- Operational alerts catch user-visible failures before support reports them.

## Cross-Phase Backlog

- Coordinate design-system work with `dmark-web`.
- Keep backend JSON contracts stable for public, auth, and app surfaces.
- Admin/support tooling.
- User education content and DNS provider guides.
- Import/export flows.
- API and webhook documentation.
- International domain name support after core ASCII behavior is solid.
- Accessibility audits for keyboard, screen reader, color contrast, and focus states in `dmark-web`.
- Abuse prevention for unauthenticated DNS scans and login attempts.
- Privacy review for raw DMARC report storage.
