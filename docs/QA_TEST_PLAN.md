# Kastro RecordMaster QA Test Plan

This plan stages QA from the current Checker/Auth baseline through the long-term Reporter product. Each stage should be treated as a release gate: automated checks run in CI, and manual exploratory checks are completed before promoting to production.

## Current Automated Coverage

Existing tests cover:

- Domain normalization and scanner classification basics.
- Missing DNS records and DNS resolver errors.
- Passwordless auth token creation and single-use consumption.
- Domain service add-and-scan behavior.
- Lettermint request shape.
- Public home, stylesheet, `/check`, invalid domain, `/healthz`, `/status`, and `/security` routes.
- Static rendered-view accessibility contracts for labels, named links, form buttons, document metadata, live regions, and letter spacing.
- Auth callback/session behavior, hardened session cookie attributes, CSRF rejection, authenticated domain import/export, and alert ack/mute handlers.
- CLI scan argument validation, formatted output, JSON output, ingest command validation, and worker job listing.
- Drift classification, alert lifecycle rules, job retry/dead-letter helpers, DMARC ingestion with sanitized provider/gateway fixtures, reporter summaries, attribution, risk, recommendations, DMARC enforcement journey states, monthly trust report models, billing, teams, entitlements, API-key helpers, webhooks, privacy, admin, status, and operations policy logic.
- Defender lookalike candidate generation for typo and Icelandic-character homoglyph variants.
- Sales-audit grading, risk labels, proof eligibility, and proof blocklist behavior.
- Dependency vulnerability scanning is wired through `govulncheck`; the local scan passes after upgrading the project toolchain target to Go 1.25.10, `github.com/jackc/pgx/v5` to v5.9.0, and `golang.org/x/net` to v0.55.0.

Known gaps:

- Database-backed integration tests now cover fresh migrations, auth token states, domain uniqueness/isolation, latest-check selection, and delete cascades when `MEIL_DB_INTEGRATION=1` and `DATABASE_URL` are set.
- No browser-level tests for the public checker or saved-domain dashboard; static rendered-view accessibility contracts are covered.
- Local unit, race, vet, lint, build, templ, sqlc, Tailwind, Docker build, Postgres migration, River migration, and ClickHouse migration gates have successful evidence from the current local toolchain and Colima-backed services.
- No live-provider billing, email, enrichment, geolocation, or DMARC mailbox tests; current coverage uses deterministic service-level tests and attribution fixtures.

## Stage 0: Static And Repository Hygiene

Run on every pull request.

Automated checks:

- `make qa-code` for repeatable code-level verification.
- `make qa-services` from an environment with Docker, Postgres, and ClickHouse access.
- `make qa` for the complete automated QA phase when all dependencies are available.
- `go test ./...`
- `go test -race ./...` for packages with concurrency or shared state.
- `go vet ./...`
- `golangci-lint run`
- `govulncheck ./...`
- `make generate-check` to verify templ, sqlc, and Tailwind generated files are current.
- `go build ./cmd/meil`
- Container build for `infra/coolify/Dockerfile`.
- Migration syntax check on a fresh Postgres database.
- Migration syntax check on a fresh ClickHouse database using `meil migrate clickhouse up`.

Manual checks:

- Review `git status --short` for unintended generated-file churn.
- Confirm `.env.example` includes every required environment variable.
- Confirm README setup commands match the Makefile and Docker Compose files.

Release gate:

- All automated checks pass from a clean checkout.
- Generated artifacts are committed intentionally.

## Stage 1: Scanner And Public Checker

Run before changes to scanner logic, public `/check`, or CLI `scan`.

Unit tests:

- Domain normalization accepts uppercase, whitespace, URL input, and trailing dot.
- Domain normalization rejects empty input, localhost, spaces, double dots, leading/trailing hyphen labels, invalid characters, and labels over 63 characters.
- SPF classification covers missing record, multiple records, hardfail, softfail, neutral/all, redirect, include chains, DNS lookup limit, malformed mechanisms, and deprecated `ptr`.
- DMARC classification covers missing record, multiple records, missing `p`, `p=none`, `p=quarantine`, `p=reject`, `sp`, `pct`, alignment tags, invalid reporting URIs, and malformed tags.
- BIMI classification covers missing record, valid record, missing logo URI, invalid logo URI, VMC/CMC URI presence, and DMARC prerequisite failure.
- DKIM classification covers no selectors, one valid selector, malformed record, missing public key, custom selector, provider preset selector, partial resolver failure, and all-selector timeout.
- DNS resolver behavior maps not-found to `missing`, timeout to `unknown`, and unexpected resolver errors to `unknown`.

Handler and CLI tests:

- POST `/check` renders all four check cards for valid input.
- POST `/check` returns `400` for invalid input.
- POST `/check` enforces request body size.
- GET `/` renders the checker form, static assets, and accessible labels.
- `meil scan <domain>` prints the same statuses as scanner results.
- Future `meil scan --json <domain>` returns stable JSON suitable for scripts.

Manual exploratory checks:

- Check representative domains: fully configured, missing DMARC, `p=none`, no SPF, multiple SPF, known Google Workspace DKIM, and known Microsoft DKIM.
- Test long TXT records on desktop and mobile.
- Disable network/DNS locally and confirm graceful error messages.
- Submit empty, malformed, extremely long, and URL-like inputs.

Release gate:

- Public checks finish within the configured timeout budget.
- Invalid input and DNS failure never panic or hang.
- UI remains readable with long records and all status combinations.

## Stage 2: Authentication And Saved Domains

Run before changes to login, sessions, users, domains, or check snapshots.

Unit tests:

- Email normalization accepts reasonable valid addresses and rejects malformed addresses.
- Login token generation stores only hashes.
- Login token consumption is single-use.
- Expired login tokens are rejected.
- Store errors are propagated without leaking token material.
- Domain service normalizes input before persistence.
- Domain service does not create checks for invalid domains.
- Domain service scopes get, delete, and scan operations by current user.

Database integration tests:

- Fresh migrations create all required tables and indexes.
- `UpsertUserByEmail` is idempotent.
- `CreateLoginToken` stores unique hashes.
- `ConsumeLoginToken` rejects consumed and expired tokens.
- `CreateDomain` enforces unique `(user_id, domain)` without cross-user conflicts.
- `ListDomainsWithLatestCheck` returns only the newest check per domain.
- Deleting a user cascades tokens, domains, and checks.
- Deleting a domain cascades checks.

Coverage artifact: `internal/store/pg/app_store_integration_test.go`, enabled in CI with `MEIL_DB_INTEGRATION=1`.

Web tests:

- GET `/login` renders the login form.
- POST `/login` sends a magic link when auth and sender are configured.
- POST `/login` handles invalid email, sender failure, and oversized body.
- GET `/auth/callback` creates a session for a valid token.
- GET `/auth/callback` rejects invalid, expired, and consumed tokens.
- POST `/logout` destroys the session.
- GET `/app/domains` redirects anonymous users to `/login`.
- Authenticated domain add, list, recheck, and delete paths call the domain service with the session user ID.
- State-changing routes enforce CSRF once CSRF middleware is added.

Manual exploratory checks:

- Complete magic-link login through Mailpit/local email.
- Reuse a magic link and confirm it fails.
- Wait for token expiry and confirm the expired flow is understandable.
- Save duplicate domains and confirm behavior is clear.
- Sign in as two users and confirm domains are isolated.
- Try direct POSTs to another user's domain IDs.

Release gate:

- Authenticated data is always scoped to the session user.
- Magic links are single-use, expire, and are never logged in plaintext.
- Saved-domain flows work after a fresh migration.

## Stage 3: Drift, Jobs, And Notifications

Run before enabling scheduled scans or alerts.

Unit tests:

- Drift comparison detects status regressions and recoveries.
- Drift comparison detects material record changes and ignores harmless ordering/whitespace changes where appropriate.
- Severity mapping is deterministic.
- Alert suppression, mute windows, and acknowledgement rules are enforced.
- Notification payloads include domain, check type, before/after, severity, and action text.

Integration tests:

- River job enqueue is idempotent per domain and schedule slot.
- Worker retries transient scanner failures.
- Worker does not send duplicate alerts for the same drift event.
- Dead-lettered jobs preserve enough context for debugging.
- Notification dispatch records success and failure.

Manual exploratory checks:

- Change a test domain from DMARC `p=reject` to `p=none` and confirm an alert.
- Remove DKIM selector and confirm the drift timeline.
- Trigger repeated scanner failures and confirm alert noise is controlled.
- Test email rendering in Mailpit and a real mailbox.

Release gate:

- Scheduled scans can run without web traffic.
- One material change creates one durable drift event and one expected notification.
- Job failures are visible in logs and metrics.

## Stage 4: DMARC Report Ingestion

Run before enabling Reporter features.

Unit tests:

- Parse plain XML, gzip, and zip report payloads.
- Reject zip traversal, oversized files, invalid XML, and unsupported compression.
- Extract report metadata, policy, source IP, counts, disposition, SPF auth results, DKIM auth results, and alignment results.
- Deduplicate reports by metadata and content hash.
- Preserve parser errors with file/report context.

Fixture tests:

- Sanitized reports from Google, Microsoft, Yahoo, Fastmail, Proton, and common enterprise gateways.
- Reports with multiple records, multiple auth results, missing optional fields, unknown vendors, and malformed but common variants.

Integration tests:

- Ingest command imports reports idempotently.
- ClickHouse writes and date-range queries return expected aggregate rows.
- ClickHouse migration loader and HTTP runner apply the committed aggregate schema statements.
- Raw report retention policy is enforced.
- Ingestion failure creates a reviewable dead-letter entry.

Manual exploratory checks:

- Forward sample reports through the configured ingestion path.
- Confirm dashboard counts match known fixture totals.
- Upload corrupt and duplicate reports.
- Confirm no raw XML leaks into user-facing pages unexpectedly.

Release gate:

- Representative provider reports import without manual repair.
- Duplicate reports do not inflate counts.
- Query performance is acceptable for the target retention window.

## Stage 5: Attribution And Reporter Analytics

Run before surfacing sender intelligence or unknown-sender alerts.

Unit tests:

- IP enrichment cache stores ASN, reverse DNS, and provider hints.
- Provider matching handles exact IP ranges, reverse DNS patterns, and ambiguous matches.
- Expected sender allowlist suppresses expected alerts without hiding aggregate evidence.
- Unknown sender detection respects volume thresholds and confidence.
- Alignment explanations match the raw report evidence.

Integration tests:

- Enrichment jobs refresh stale records without blocking ingestion.
- Reporter dashboard filters by domain, date range, source, result, and provider.
- Marking a sender expected/ignored changes unknown-sender alert behavior.

Manual exploratory checks:

- Validate sender attribution against known SaaS email providers.
- Review ambiguous sources and confirm confidence is displayed honestly.
- Test large date ranges and high-cardinality source lists.

Release gate:

- Attribution is explainable from stored evidence.
- Unknown-sender alerts avoid obvious expected provider false positives.

## Stage 6: Billing, Teams, And API

Run before monetization or multi-user collaboration.

Unit tests:

- Entitlement checks enforce plan limits at service boundaries.
- RBAC permissions protect billing, domain settings, team invitations, API keys, and exports.
- Billing webhook signature verification rejects invalid payloads.
- Billing webhook handling is idempotent.
- API key authentication accepts active keys and rejects revoked or expired keys.

Integration tests:

- Plan changes update domain limits, scan cadence, report retention, and alert channels.
- Team invitation accept/revoke flows work.
- Domain transfer preserves checks, drift events, and report data.
- API endpoints enforce organization scope.
- Audit log writes append-only records for sensitive actions.

Manual exploratory checks:

- Upgrade, downgrade, cancel, and payment-failure flows.
- Invite users with different roles and test forbidden actions.
- Rotate and revoke API keys.
- Export account and domain data.

Release gate:

- Billing state cannot grant accidental access.
- Organization boundaries are enforced by automated tests.
- Sensitive actions produce audit records.

## Stage 7: Nonfunctional QA

Run continuously, with full passes before major releases.

Security:

- Threat model public DNS checker abuse, login flow, sessions, report ingestion, billing webhooks, and admin tooling.
- Check CSRF, session cookie attributes, rate limits, body limits, redirect behavior, and secret logging.
- Run dependency vulnerability scans.
- Fuzz scanner parsers and DMARC XML ingestion. Seed corpus entry points are `FuzzNormalizeDomain`, `FuzzParseSPF`, `FuzzParseTagRecord`, and `FuzzParseAggregatePayload`; `make fuzz-smoke` runs short smoke passes.

Performance:

- Load test public checker with mocked resolver and controlled live DNS.
- Load test scheduled scan workers at target saved-domain volume.
- Load test ClickHouse report queries over target retention data.
- Use `scripts/load` or the Makefile `load-checker`, `load-api`, `load-reporter`, and `load-scan-job` targets for HTTP checker, API, Reporter read endpoint, and job-triggering endpoint load checks.
- Track p95/p99 latencies and resource use.

Accessibility:

- Keyboard navigation for public checker, login, and dashboard.
- Focus states on all interactive controls.
- Screen reader labels for forms, status cards, and alert actions.
- Color contrast for pass, warn, fail, missing, and unknown states.

Reliability:

- Backup and restore drills for Postgres and ClickHouse.
- Deployment rollback drill.
- Worker restart and duplicate-job recovery.
- DNS provider outage simulation.
- Email provider outage simulation.
- Deployment rollback drill from `internal/ops.DeploymentRollbackDrill`.

Observability:

- Request logs include route, status, duration, and request ID.
- Scanner metrics include duration, result status, resolver failure type, and timeout count.
- Job metrics include queue depth, attempts, duration, failures, and dead-letter count.
- Ingestion metrics include reports received, rows imported, duplicate count, parser failures, and source provider.
- Trace spans cover HTTP requests, scanner runs, worker jobs, DMARC ingestion, and Reporter queries.

Release gate:

- Security and reliability risks have owners and target dates.
- Performance meets agreed thresholds for the phase.
- Production alerts exist for user-visible failures.

## Smoke Test Checklist

Run after every deploy:

- GET `/healthz` returns `200 ok`.
- GET `/` renders the public checker.
- POST `/check` for a known-good domain renders SPF, DKIM, DMARC, and BIMI cards.
- POST `/check` for invalid input returns `400`.
- GET `/login` renders.
- POST `/login` sends a magic link in the target environment.
- Magic link callback signs in and redirects to `/app/domains`.
- Add a saved domain and confirm latest check appears.
- Recheck the saved domain.
- Delete the saved domain.
- Logout redirects to `/`.
- Static CSS and JS assets return `200`.
- Logs show no panic, migration error, or missing environment variable.

## Test Data

Maintain deterministic fixtures for:

- DNS records for SPF, DKIM, DMARC, and BIMI.
- Public checker HTML snapshots for all statuses.
- Magic-link email request payloads.
- Domain snapshot and drift event pairs.
- DMARC aggregate report XML, gzip, and zip files.
- Attribution provider rules and source IP examples.
- Billing webhook examples.

Fixtures should avoid real customer data. Sanitized real-world DMARC reports must remove domains, emails, IPs where required, message IDs, and organization identifiers unless explicit permission exists.
