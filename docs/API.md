# API And Webhooks

This document defines the external API and webhook contracts for paid tiers. API access requires an API key with the required scope.

## Authentication

Send API keys with the `Authorization` header:

```http
Authorization: Bearer meil_example
```

Keys are stored as hashes only. Revoke and rotate keys from organization settings.

## Scopes

- `scan:read`: run or read scan results.
- `domain:read`: list or export saved domains and latest checks.
- `domain:write`: add, update, recheck, import, or delete saved domains.
- `reporter:read`: read DMARC aggregate report summaries, sender inventory, and attribution.
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
GET /app/privacy/export
GET /app/privacy/deletion-plan
POST /app/privacy/delete
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

Responses include stable IDs, timestamps in RFC3339, and machine-readable status strings that match internal scanner, drift, report-health, and risk models.

Status component and incident operations require `admin:manage`. Components accept status values such as `operational`, `degraded`, and `outage`; incidents keep an operator-facing status string and can be closed through the resolve endpoint.

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
