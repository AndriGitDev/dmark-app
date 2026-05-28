# Dmark API And Webhooks

This document defines the external API and webhook contracts for paid tiers. API access requires an API key with the required scope.

## Public Browser Bridge

The separate `dmark-web` frontend should use these backend-owned JSON endpoints for public marketing flows:

```http
GET /api/public/scan?domain=example.com
POST /api/public/scan
POST /api/public/audit-claims
```

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
