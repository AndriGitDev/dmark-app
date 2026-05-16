# Threat Model

This threat model covers the current roadmap surfaces: auth, public checker abuse, protected-domain monitoring, Brand Risk findings, DMARC report ingestion, API keys, outbound webhooks, and billing webhooks.

## Assets

- User accounts, sessions, and login-token hashes.
- Protected domains, domain settings, DNS snapshots, Brand Risk findings, drift events, and notification dispatches.
- DMARC aggregate report metadata, raw XML, aggregate rows, and sender attribution evidence.
- Organization membership, roles, billing state, API keys, webhook secrets, and audit logs.

## Trust Boundaries

- Browser to public HTTP routes.
- Browser session to authenticated app routes.
- Public API clients to API-key authenticated routes.
- External mailbox providers to DMARC report ingestion.
- Billing provider to billing webhook receiver.
- RecordMaster to outbound email, Slack, and webhook delivery providers.
- Application workers to Postgres and ClickHouse.

## Auth And Sessions

Threats:

- Magic-link token theft or replay.
- Session fixation after callback.
- Cross-site form submission against authenticated routes.
- User data access across account or organization boundaries.

Controls:

- Login tokens are random, hashed at rest, expire, and are single-use.
- Successful callbacks renew the session token before setting user identity.
- State-changing login and app routes require same-origin checks and CSRF tokens.
- Domain service and store methods accept user or organization scope for all sensitive operations.

## Public Checker Abuse

Threats:

- DNS scan amplification.
- Large request bodies or malformed domains consuming resources.
- Resolver timeouts causing web handler exhaustion.

Controls:

- Public check requests are body-limited, domain-length guarded, rate-limited per client address, and persistence-free by default.
- Scanner results carry resolver error classes and lookup timing for monitoring.
- DKIM selector probes use bounded concurrency.

## Brand-Name Risk

Threats:

- Lookalike domain impersonation.
- Homoglyph domains.
- Mail-enabled spoofing domains.
- Weak DMARC policy abuse.
- Unknown sender infrastructure.
- DNS drift causing spoofing exposure.
- Brand-risk alert abuse.
- False positives in impersonation detection.

Controls:

- Current Brand Risk findings are limited to evidence already present in DNS, email-auth checks, drift history, and Reporter data.
- Every finding must carry evidence and a recommended action.
- Unsupported lookalike-domain detection must remain clearly marked as planned until the feed exists.
- Future external intelligence feeds need rate limits, privacy review, and abuse controls before production use.

## DMARC Report Ingestion

Threats:

- Zip traversal, decompression abuse, oversized reports, invalid XML, duplicate replay, and raw XML leakage.
- Malicious report metadata poisoning dashboards.

Controls:

- Ingestion accepts plain XML, gzip, and zip XML only.
- Zip entries reject absolute paths and parent traversal.
- Payloads are size-limited before parsing.
- Reports are keyed by normalized org, report ID, policy domain, and content hash for deduplication.
- Raw XML retention follows plan-level retention controls and should not be rendered in user-facing pages.

## API Keys And Webhooks

Threats:

- Raw API key disclosure.
- Overbroad API key scopes.
- Webhook spoofing or replay.
- Delivery endpoint misconfiguration.

Controls:

- API keys are generated with the `meil_` prefix, stored as hashes, and authorized by scope.
- Webhook endpoints must use HTTPS.
- Outbound webhook payloads are signed with HMAC-SHA256 over the raw JSON body.
- Webhook deliveries are idempotent by endpoint and event ID.

## Billing Webhooks

Threats:

- Forged subscription events.
- Duplicate or out-of-order lifecycle updates.
- Billing privilege misuse by team members.

Controls:

- Billing webhook payloads require HMAC verification with `BILLING_WEBHOOK_SECRET`.
- Subscription event application is idempotent by event ID.
- RBAC gates billing management to owner role.
- Billing state changes should be written to the audit log.

## Residual Risks

- Live DNS and IP enrichment depend on external network behavior and may be incomplete or stale.
- DMARC aggregate reports are not proof of all outbound mail; they depend on receiver reporting coverage.
- Provider attribution is probabilistic unless multiple strong signals agree.
- Full browser, database, worker, and container QA still must pass before production use.
