# Dmark API And Webhooks

This document defines the external API and webhook contracts for paid tiers. API access requires an API key with the required scope.

## Public Browser Bridge

The separate `dmark-web` frontend should use these backend-owned JSON endpoints for public marketing flows:

```http
GET /api/public/scan?domain=example.com
POST /api/public/scan
POST /api/public/audit-claims
GET /status
GET /healthz
```

`GET /status` is unauthenticated and returns the service name, operational status, component list, and non-secret build metadata (`version.version`, `version.commit`, and `version.build_time`) so operators can connect smoke-test evidence to the deployed artifact. `GET /healthz` remains the minimal liveness probe and returns `ok`.

`GET /api/public/scan` and `POST /api/public/scan` are unauthenticated, rate-limited by client address, and return the same scan contract as `GET /api/v1/scan` without persisting the check. `POST /api/public/scan` accepts:

```json
{
  "domain": "example.com"
}
```

`POST /api/public/audit-claims` supports the claim/lead capture flow after a public scan. It validates that the work email domain matches the audited domain, scans the domain again, stores the audit claim, and returns the claim status plus audit result. It accepts:

```json
{
  "domain": "example.com",
  "email": "security@example.com"
}
```

Allowed browser origins are configured with `FRONTEND_ORIGINS` as a comma-separated list of absolute origins such as `https://dmark.is,https://www.dmark.is`. If `dmark-web` proxies calls through its own server-side route, point that route at `/api/public/scan` on this backend and leave `DMARK_API_TOKEN` unset for public scans.

## Authentication

Send API keys with the `Authorization` header:

```http
Authorization: Bearer meil_example
```

Keys are stored as hashes only. Revoke and rotate keys from organization settings.

## Scopes

- `scan:read`: run or read Record checks.
- `domain:read`: list or export Protected domains and latest checks.
- `domain:write`: add, update, recheck, import, or delete Protected domains.
- `reporter:read`: read Reporter reports, sender inventory, and attribution.
- `billing:manage`: request billing cancellation and manage billing lifecycle actions.
- `privacy:manage`: read and update organization data retention settings.
- `api_key:manage`: create, list, and revoke organization API keys.
- `team:manage`: create and list organization team invitations.
- `admin:manage`: create, list, and resolve support or abuse cases.
- `webhook:manage`: create, update, disable, or rotate webhook endpoints.

## Endpoints

```http
GET /api/v1/scan?domain=example.com
GET /api/v1/dashboard
GET /api/v1/dashboard/domains
GET /api/v1/dashboard/domains/{id}
POST /api/v1/dashboard/domains/{id}/recheck
GET /api/v1/dashboard/domains/{id}/spf-route
PUT /api/v1/dashboard/domains/{id}/spf-route
POST /api/v1/dashboard/domains/{id}/spf-route/sync
GET /api/v1/dashboard/domains/{id}/change-plan
GET /api/v1/dashboard/domains/{id}/brand-risk
GET /api/v1/dashboard/domains/{id}/reporter
GET /api/v1/dashboard/domains/{id}/reporter?filter=unknown
GET /api/v1/dashboard/domains/{id}/reporter?format=csv
GET /api/v1/dashboard/alerts
POST /api/v1/dashboard/alerts/{id}/ack
POST /api/v1/dashboard/alerts/{id}/mute
GET /api/v1/dashboard/settings
POST /api/v1/dashboard/settings
GET /api/v1/domains
GET /api/v1/domains/export
POST /api/v1/domains
POST /api/v1/domains/import
POST /api/v1/domains/{id}/check
GET /api/v1/domains/{id}/history
GET /api/v1/domains/{id}/drift
GET /api/v1/domains/{id}/reporter/sources
GET /api/v1/domains/{id}/reporter/senders
GET /api/v1/domains/{id}/reporter/health
GET /api/v1/webhooks
POST /api/v1/webhooks
POST /api/v1/webhooks/{id}/rotate-secret
POST /api/v1/webhooks/{id}/disable
POST /api/v1/billing/cancel
GET /api/v1/api-keys
POST /api/v1/api-keys
POST /api/v1/api-keys/{id}/revoke
GET /api/v1/team/invitations
POST /api/v1/team/invitations
POST /api/v1/domains/{id}/transfer
GET /api/v1/privacy/retention
POST /api/v1/privacy/retention
GET /api/v1/admin/cases
POST /api/v1/admin/cases
POST /api/v1/admin/cases/{id}/resolve
GET /api/v1/admin/abuse-decisions
POST /api/v1/admin/abuse-decisions
GET /api/v1/status/components
POST /api/v1/status/components
GET /api/v1/status/incidents
POST /api/v1/status/incidents
POST /api/v1/status/incidents/{id}/resolve
POST /billing/webhook
```

API responses include stable IDs, timestamps in RFC3339, and machine-readable status strings that match internal scanner, drift, report-health, and risk models. Browser-facing pages, static assets, and public form flows belong in the separate `dmark-web` repository.

Public terminology maps these endpoints to Record checks, Protected domains, Findings, Alerts, Reporter reports, Sender inventory, and Brand risk findings. Endpoint paths keep `/domains` for compatibility while the UI presents them as Protected domains.

## Dashboard API

The `/api/v1/dashboard/*` endpoints are the frontend-ready product contract for `dmark-web`. They require bearer API keys attached to an organization. Read endpoints require `domain:read` or `reporter:read` where Reporter data is returned. Recheck, alert actions, and settings updates require `domain:write`.

Dashboard responses are JSON unless `format=csv` is explicitly requested on the Reporter endpoint. IDs are opaque strings. Timestamps are RFC3339 UTC.

Load checks are available for the aggregate dashboard, Reporter, Brand Risk, Alerts, Settings, and scan-job endpoints through the `make load-dashboard*` targets.

### GET `/api/v1/dashboard`

```json
{
  "generated_at": "2026-05-28T12:00:00Z",
  "protected_domain_count": 1,
  "average_score": 90,
  "alert_count": 1,
  "unknown_sender_count": 1,
  "domains": [
    {
      "id": "domain_opaque",
      "domain": "example.com",
      "created_at": "2026-05-16T12:00:00Z",
      "last_checked_at": "2026-05-28T11:58:00Z",
      "status": "monitoring",
      "score": 90,
      "dmarc_policy": "reject",
      "spf_lookup_count": 1,
      "dkim_coverage": "covered",
      "alignment_rate": 98.5,
      "unknown_sender_count": 1,
      "next_action": "Review unknown Reporter senders before advancing enforcement."
    }
  ]
}
```

Dashboard domain status values are `protected`, `monitoring`, and `needs-work`.

### GET `/api/v1/dashboard/domains/{id}`

Returns one Protected domain and recent history.

```json
{
  "domain": {
    "id": "domain_opaque",
    "domain": "example.com",
    "created_at": "2026-05-16T12:00:00Z",
    "last_checked_at": "2026-05-28T11:58:00Z",
    "status": "monitoring",
    "score": 90,
    "dmarc_policy": "reject",
    "spf_lookup_count": 1,
    "dkim_coverage": "covered",
    "alignment_rate": 98.5,
    "unknown_sender_count": 1,
    "next_action": "Review unknown Reporter senders before advancing enforcement."
  },
  "history": []
}
```

Missing or out-of-organization domains return:

```json
{
  "error": "domain was not found"
}
```

### POST `/api/v1/dashboard/domains/{id}/recheck`

Queues a non-blocking recheck when the job queue is configured. The API returns `202 Accepted`.

```json
{
  "job_id": "job_opaque",
  "domain_id": "domain_opaque",
  "status": "queued",
  "queued_at": "2026-05-28T12:00:00Z",
  "next_allowed_at": "2026-05-28T12:05:00Z"
}
```

Rapid duplicate rechecks are coalesced per domain into the same five-minute idempotency window.

### SPF route dashboard endpoints

`GET /api/v1/dashboard/domains/{id}/spf-route` requires `domain:read`. `PUT /api/v1/dashboard/domains/{id}/spf-route` and `POST /api/v1/dashboard/domains/{id}/spf-route/sync` require `domain:write`. All three endpoints first verify that the requested Protected domain belongs to the API key organization. Bunny DNS credentials, zone IDs, provider secrets, and raw provider responses are never returned.

`GET` returns frontend-safe route state:

```json
{
  "id": "route_opaque",
  "domain_id": "domain_opaque",
  "domain": "example.com",
  "status": "healthy",
  "route_fqdn": "example-com.route.dmark.is",
  "customer_cname_name": "example.com",
  "customer_cname_target": "example-com.route.dmark.is",
  "generated_spf_preview": "v=spf1 include:_spf.mailgun.org ~all",
  "last_published_spf": "v=spf1 include:_spf.mailgun.org ~all",
  "last_synced_at": "2026-05-31T12:00:00Z",
  "enabled_sources": [
    {"kind": "include", "value": "_spf.mailgun.org", "label": "Mailgun"}
  ],
  "evidence_summary": [
    {"source_value": "_spf.mailgun.org", "resolved_domain": "_spf.mailgun.org", "mechanism": "include:_spf.mailgun.org", "kind": "include", "message": "resolved"}
  ]
}
```

`PUT` accepts `route_fqdn`, `customer_cname_name`, `customer_cname_target`, `enabled`, `mechanism_policy` (`softfail`, `fail`, or `neutral`), and `sources` with `kind` (`include`, `ip4`, or `ip6`), `value`, `label`, and `enabled`. It stores the configuration, queues an immediate sync through the backend job dependency, and returns the same route response after upsert.

`POST /sync` queues a manual sync through the backend job dependency and returns `202 Accepted`:

```json
{
  "job_id": "job_opaque",
  "route_id": "route_opaque",
  "domain_id": "domain_opaque",
  "status": "queued",
  "queued_at": "2026-05-31T12:00:00Z"
}
```

### GET `/api/v1/dashboard/domains/{id}/change-plan`

Returns staged DNS changes owned by the backend recommendation engine.

```json
{
  "domain_id": "domain_opaque",
  "domain": "example.com",
  "generated_at": "2026-05-28T12:00:00Z",
  "journey_state": "ready_for_reject",
  "next_action": "Move the DMARC policy to \"p=reject\" after notifying mail owners and watching the next reporting window.",
  "changes": [
    {
      "id": "dmarc-enforcement-ladder",
      "title": "Advance the DMARC enforcement ladder",
      "status": "review",
      "record_name": "_dmarc.example.com",
      "record_type": "TXT",
      "record_text": "v=DMARC1; p=reject; rua=mailto:dmarc@example.com",
      "evidence": "Reporter evidence shows enough aligned traffic and low unknown-sender risk for the next enforcement step.",
      "risk": "DMARC enforcement can reject legitimate mail if sender alignment evidence is incomplete.",
      "rollback_note": "Move the policy back to \"p=none\" or the previous policy while investigating failures."
    }
  ]
}
```

Change status values are `ready`, `review`, `waiting`, and `blocked`.

### GET `/api/v1/dashboard/domains/{id}/brand-risk`

Returns email-auth Brand Risk findings plus bounded Defender lookalike evidence. Lookalike rows are only returned by default when DNS address or MX records are observed. Use `include_inactive=true` to return watchlist candidates without DNS evidence. Query options are `tld` and `limit` (`1..100`, default `25`).

```json
{
  "domain_id": "domain_opaque",
  "domain": "example.com",
  "generated_at": "2026-05-28T12:00:00Z",
  "findings": [
    {
      "category": "Domain trust",
      "type": "dmarc_policy_relaxed",
      "title": "DMARC policy is relaxed",
      "severity": "Medium",
      "evidence": "DMARC: DMARC monitor Record: v=DMARC1; p=none",
      "recommended_action": "Move toward DMARC enforcement once legitimate senders are verified."
    }
  ],
  "lookalikes": [
    {
      "domain": "exmple.is",
      "kind": "typo",
      "reason": "single-character deletion",
      "has_address_records": true,
      "has_mx_records": false,
      "severity": "Medium",
      "evidence": "DNS evidence: A/AAAA records are published.",
      "recommended_action": "Review ownership and web content for impersonation risk, then acknowledge or monitor the candidate."
    }
  ]
}
```

### GET `/api/v1/dashboard/domains/{id}/reporter`

Requires `reporter:read`. `filter=unknown` returns only unknown sender rows. `format=csv` returns deterministic CSV with no raw XML, secrets, or report payloads.

```json
{
  "domain_id": "domain_opaque",
  "domain": "example.com",
  "report_volume": 1200,
  "aligned_percentage": 98.5,
  "reporter_org_count": 3,
  "failures": 0,
  "reject_candidates": 2,
  "sender_inventory": [
    {
      "source_ip": "203.0.113.10",
      "provider": "Google Workspace",
      "volume": 900,
      "aligned_percentage": 99.8,
      "classification": "expected",
      "trend": "stable",
      "risk": "expected",
      "first_seen": "2026-05-01T00:00:00Z",
      "last_seen": "2026-05-28T00:00:00Z"
    }
  ]
}
```

Empty Reporter state returns setup guidance:

```json
{
  "domain_id": "domain_opaque",
  "domain": "example.com",
  "report_volume": 0,
  "aligned_percentage": 0,
  "reporter_org_count": 0,
  "failures": 0,
  "reject_candidates": 0,
  "setup_guidance": "Enable DMARC aggregate reporting with a rua mailbox to populate Reporter sender inventory.",
  "sender_inventory": []
}
```

### GET `/api/v1/dashboard/alerts`

Returns active operator Alerts by default.

```json
{
  "alerts": [
    {
      "id": "alert_opaque",
      "domain_id": "domain_opaque",
      "domain": "example.com",
      "check_name": "DMARC",
      "kind": "record_changed",
      "severity": "critical",
      "message": "DMARC policy was relaxed.",
      "evidence": "v=DMARC1; p=none",
      "created_at": "2026-05-28T12:00:00Z",
      "acknowledged": false
    }
  ]
}
```

### POST `/api/v1/dashboard/alerts/{id}/ack`

Optional body:

```json
{
  "reason": "Intentional DNS migration completed."
}
```

Response:

```json
{
  "id": "alert_opaque",
  "status": "acknowledged",
  "applied_at": "2026-05-28T12:00:00Z"
}
```

### POST `/api/v1/dashboard/alerts/{id}/mute`

Accepts either `until` or `duration`.

```json
{
  "duration": "7d",
  "reason": "Provider cutover window."
}
```

Response:

```json
{
  "id": "alert_opaque",
  "status": "muted",
  "applied_at": "2026-05-28T12:00:00Z"
}
```

### GET `/api/v1/dashboard/settings`

```json
{
  "require_alignment_before_reject": 95,
  "pause_automation_on_unknown_senders_spike": true,
  "auto_generate_monthly_executive_reports": false,
  "notification_channels": ["email"],
  "updated_at": "2026-05-28T12:00:00Z"
}
```

### POST `/api/v1/dashboard/settings`

```json
{
  "require_alignment_before_reject": 95,
  "pause_automation_on_unknown_senders_spike": true,
  "auto_generate_monthly_executive_reports": true,
  "notification_channels": ["email", "webhook"]
}
```

Thresholds outside `1..100` return `400 Bad Request` with a JSON error.

Status component and incident operations require `admin:manage`. Components accept status values such as `operational`, `degraded`, and `outage`; incidents keep an operator-facing status string and can be closed through the resolve endpoint.

## Product State Contracts

DMARC enforcement journeys should use these stable state strings when exposed through reports, exports, or future API responses:

- `not_started`
- `observing`
- `ready_for_quarantine`
- `quarantine`
- `ready_for_reject`
- `reject`

Monthly trust reports should summarize protected-domain posture, DMARC journey state, Reporter sender risk, Brand Risk findings, and prioritized next actions. They should not include raw secrets, API keys, login tokens, webhook secrets, or raw DMARC XML.

Sales-audit and reality-check surfaces should use these stable risk strings:

- `low`
- `medium`
- `high`
- `critical`

Sales-audit responses should include a grade, buyer-facing message, CTA, proof eligibility, and evidence signals. Public audit claims should require a matching work-email domain, send the existing passwordless magic link, persist a durable claim record, and attach the claimed domain to protected domains after callback when the same browser session is available. Proof eligibility does not mean a proof email can be sent directly; any future proof dispatch must require verified ownership, rate limits, blocklist checks, explicit consent, and audit logging.

## Webhook Signatures

Webhook payloads are JSON and signed with HMAC-SHA256 using the endpoint secret. The signature header value uses the same format as the internal verifier:

```txt
sha256=<hex digest>
```

Consumers must verify the signature over the raw request body before parsing JSON.

## Webhook Event Shape

```json
{
  "id": "evt_123",
  "type": "drift.created",
  "created_at": "2026-05-16T12:00:00Z",
  "data": {
    "domain": "example.com",
    "check": "DMARC",
    "severity": "critical",
    "message": "DMARC policy was relaxed."
  }
}
```

## Webhook Events

- `drift.created`
- `drift.acknowledged`
- `sender.unknown_detected`
- `report.ingested`
- `report.ingestion_failed`
- `domain.scan_failed`
- `billing.subscription_updated`

Webhook deliveries are idempotent by endpoint and event ID. Receivers should also treat event IDs as idempotency keys.
