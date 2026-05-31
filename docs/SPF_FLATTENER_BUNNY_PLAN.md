# Bunny.net SPF Flattening Service Plan

This is a GA-scope implementation plan for Dmark's automated SPF flattening service. The service belongs in `dmark-app` because it owns protected-domain state, background jobs, backend API contracts, secrets, and DNS publication. `dmark-web` should only render the operator UI and call server-side dashboard endpoints.

## GA Position

SPF flattening is a GA blocker, not a future/nice-to-have feature. Dmark's public promise includes keeping customer SPF records below the 10-DNS-lookup receiver limit and giving operators a safe, auditable path to publish delegated SPF routes.

Minimum GA outcome:

- Customer can configure sender includes/tools for a protected domain.
- Backend recursively resolves SPF includes with loop/depth protection.
- Backend generates a deterministic flattened SPF TXT value with evidence.
- Backend publishes the customer route TXT record through Bunny.net DNS.
- Dashboard can show the customer CNAME delegation instruction and sync status.
- Dashboard can auto-detect the customer's DNS provider from authoritative nameservers and show provider-specific, step-by-step DNS edit instructions instead of asking the user to choose a provider manually.
- Failed, stale, oversized, or incomplete flattening states are visible and do not silently publish bad records.

## DNS Control Plane

Use delegated customer SPF routes under a Dmark-controlled Bunny.net DNS zone.

Founder/operator prep:

1. Use the existing Bunny DNS zone for `dmark.is` as the authoritative write target.
2. Publish SPF route records as relative names under that parent zone, for example `customer-example.route` inside `dmark.is`, which resolves publicly as `customer-example.route.dmark.is`.
3. Do not delegate `route.dmark.is` to the same Bunny nameservers while `dmark.is` is also on Bunny; testing showed same-provider child delegation produced lame delegation/SERVFAIL.
4. Generate a Bunny account API key with the minimum DNS permissions available.
5. Store Bunny credentials only in backend/server-side environment variables.
6. Configure the Bunny DNS zone ID for the parent `dmark.is` zone.

Backend environment variables:

```env
BUNNY_DNS_API_KEY=...          # preferred; BUNNY_API_KEY is accepted as a backwards-compatible alias
BUNNY_DNS_ZONE_ID=801471       # parent dmark.is Bunny zone
DMARK_SPF_DNS_ZONE=dmark.is    # Bunny write-zone apex for BUNNY_DNS_ZONE_ID
DMARK_SPF_ROUTE_ZONE=route.dmark.is
DMARK_SPF_ROUTE_TTL=300
```

Client DNS shape:

```txt
_spf.customer.example CNAME customer-example.route.dmark.is
```

Dmark-owned Bunny DNS shape:

```txt
customer-example.route.dmark.is TXT "v=spf1 ip4:192.0.2.10 ip6:2001:db8::/32 ~all"
```

## Backend Data Model

Add persisted SPF route state in Postgres.

Recommended tables/fields:

- `spf_routes`
  - `id`
  - `organization_id`
  - `domain_id`
  - `route_label`
  - `route_fqdn`
  - `customer_cname_name`
  - `customer_cname_target`
  - `enabled`
  - `mechanism_policy` (`softfail`, `fail`, `neutral`) with a conservative default of `softfail`
  - `bunny_record_id`
  - `last_generated_spf`
  - `last_published_spf`
  - `last_sync_status` (`pending`, `healthy`, `stale`, `failed`, `blocked`)
  - `last_sync_error`
  - `last_synced_at`
  - `created_at`, `updated_at`

- `spf_route_sources`
  - `id`
  - `spf_route_id`
  - `kind` (`include`, `ip4`, `ip6`)
  - `value`
  - `label`
  - `enabled`
  - `created_at`, `updated_at`

- `spf_route_evidence`
  - `id`
  - `spf_route_id`
  - `source_value`
  - `resolved_domain`
  - `mechanism`
  - `evidence_kind` (`ip4`, `ip6`, `include`, `error`)
  - `message`
  - `created_at`

Implemented first persistence/orchestration slice:

- Migration `migrations/postgres/00006_spf_routes.sql` creates `spf_routes`, `spf_route_sources`, and `spf_route_evidence`.
- `internal/spfroutes.Service` loads a persisted route, resolves enabled sources with `internal/spfflatten`, generates a deterministic SPF value, skips Bunny publication when the current published SPF already matches, publishes changed TXT values through the `internal/bunnydns` client interface, and persists status/evidence.
- `internal/store/pg.AppStore` implements route creation/upsert, source replacement, route lookup, enabled-route listing, and sync-state/evidence persistence.
- `internal/jobs.SyncSPFRouteWorker` registers a River job kind (`sync_spf_route`) that calls the route sync service with River retry semantics. Periodic enqueueing of enabled routes remains a follow-up GA work item.

Keep migration names monotonic after the current latest migration, currently `migrations/postgres/00006_spf_routes.sql`.

## Resolver Rules

Implement in Go under a new backend package, e.g. `internal/spfflatten`.

Input:

- route config
- enabled include domains
- enabled literal `ip4:` / `ip6:` mechanisms
- DNS TXT resolver interface for testability

Rules:

- Normalize include domains with existing scanner/domain normalization helpers where appropriate.
- Resolve TXT records for each include domain.
- Select exactly one SPF record beginning with `v=spf1`; zero or multiple SPF records are blocking errors for that include.
- Extract `ip4:` and `ip6:` mechanisms.
- Recursively follow `include:` mechanisms.
- Cap recursion depth at 10.
- Track visited domains to prevent include loops.
- Deduplicate final mechanisms deterministically.
- Preserve enough evidence to explain which include produced each mechanism.
- Do not publish when any enabled source has an unresolved/blocking error.

Initial non-goals:

- Do not infer that a provider can be removed without evidence.
- Do not flatten `a`, `mx`, `ptr`, `exists`, or redirect mechanisms in the first GA slice unless explicit tests and safety rules are added.
- Do not expose Bunny credentials or route mutation endpoints to browser JavaScript.

## SPF TXT Generation

Output format:

```txt
v=spf1 ip4:192.0.2.10 ip4:198.51.100.0/24 ip6:2001:db8::/32 ~all
```

Rules:

- Generate one SPF policy string for the route.
- Keep deterministic ordering for stable diffs and idempotent publication.
- Bound the final TXT value length; mark the route `blocked` if the flattened result is too large to publish safely.
- Preserve the configured terminal mechanism (`~all` by default; `-all` only when explicitly configured).
- If Bunny requires TXT chunk representation, chunk only at the DNS-client boundary and keep stored generated SPF as the canonical single string.

## Bunny DNS Client

Use Bunny's Core Platform API.

Known record API shape to verify against current Bunny docs before implementation:

- Base URL: `https://api.bunny.net`
- Records endpoint: `/dnszone/{ZONE_ID}/records`
- Headers:
  - `AccessKey: <BUNNY_DNS_API_KEY>`
  - `Content-Type: application/json`

Create payload shape:

```json
{
  "Type": 3,
  "Name": "customer-example",
  "Value": "v=spf1 ip4:192.0.2.10 ~all",
  "Ttl": 300
}
```

Implementation requirements:

- Wrap Bunny API calls behind an interface and fake HTTP client in tests.
- Convert a route label into the Bunny-relative record name for the configured write zone:
  - `route.dmark.is` zone: `customer-example`
  - `dmark.is` parent zone: `customer-example.route`
- List records and find existing TXT record by route label.
- Create the TXT record when absent.
- Update the existing record only when generated SPF differs from the currently published value.
- Never call Bunny when the generated output is unchanged.
- Redact API keys from logs and errors.
- Persist Bunny record ID, generated value, published value, status, and last sync error.

## Worker And Scheduling

Use the existing backend worker/job structure rather than adding a new runtime.

Cadence:

- Sync immediately when a route/source configuration changes.
- Resync all active routes every 12 to 24 hours.
- Alert on `failed`, `stale`, and `blocked` states.
- Surface route status in dashboard change plans.

## Dashboard/API Contract

Backend should expose dashboard-owned endpoints under `/api/v1/dashboard/*`; `dmark-web` should call them through its `/api/dashboard/*` server-side proxy.

Minimum endpoints:

- `GET /api/v1/dashboard/domains/{id}/spf-route`
- `PUT /api/v1/dashboard/domains/{id}/spf-route`
- `POST /api/v1/dashboard/domains/{id}/spf-route/sync`

Minimum response data:

- route status
- customer CNAME name and target
- generated SPF preview
- last published SPF
- last sync timestamp and error
- enabled include/source list
- evidence list or summary

Visible UI copy must be operator-facing: "SPF Route", "Delegation", "Last sync", "Blocked because...". Avoid exposing Bunny/API implementation details in customer-facing UI.

## Nameserver Provider Autopicker

Use authoritative NS records to remove DNS-provider selection friction for Icelandic and international users.

Minimum GA outcome:

- Resolve authoritative NS records for the protected domain.
- Normalize nameserver hostnames to lowercase FQDNs.
- Match nameservers against a maintained provider pattern registry.
- Return one of `detected`, `ambiguous`, or `unknown`.
- When detected, show exact provider-specific instructions for the record being edited: DMARC TXT, SPF TXT, and SPF route CNAME.
- When the provider supports stable console URLs, show a deep link to the DNS management surface.
- When unknown or ambiguous, fall back to generic DNS-host instructions and let the operator manually select a provider.

Initial Icelandic-first provider registry:

- ISNIC / `.is` domain management
- 1984 Hosting
- Opin Kerfi / hosted DNS where nameserver patterns are reliable
- Origo / hosted DNS where nameserver patterns are reliable
- Advania / hosted DNS where nameserver patterns are reliable
- Cloudflare
- Microsoft 365 delegated DNS, where applicable
- Google Domains/Squarespace legacy mappings, where applicable
- Route 53
- Bunny DNS

Implementation notes:

- Keep provider detection in backend/server-side code so scan and dashboard results use the same source of truth.
- Store the registry as code or data with tests for every pattern.
- Never claim certainty when multiple providers match; expose the evidence nameservers in the response.
- Deep links should prefer stable DNS-zone entry points. If a provider does not support a stable domain-specific URL, link to the provider DNS dashboard and show the exact domain/record name to search for.
- Instruction copy should be localization-ready and written in plain operator language; Icelandic provider flows should be treated as first-class, not generic afterthoughts.

Suggested API response fields:

```json
{
  "status": "detected",
  "provider_id": "isnic",
  "provider_name": "ISNIC",
  "confidence": "high",
  "evidence_nameservers": ["ns1.isnic.is", "ns2.isnic.is"],
  "dns_dashboard_url": "https://...",
  "instructions": [
    "Open DNS management for the domain.",
    "Create a CNAME record named _spf.",
    "Point it at customer-example.route.dmark.is."
  ]
}
```

## TDD Implementation Phases

Follow strict TDD for each production-code slice.

1. Migration and store model for SPF route config/state.
2. Recursive SPF resolver with fake DNS TXT resolver.
3. Deterministic SPF generator and oversized-record blocking.
4. Bunny DNS client with fake HTTP client.
5. Sync service that combines resolver, generator, persistence, and Bunny publication.
6. Worker/scheduler integration.
7. Dashboard API contracts and mapper tests.
8. Nameserver provider autopicker with fixture-backed provider pattern tests and fallback handling.
9. Dashboard UI in `dmark-web` behind server-side proxy routes.
10. Load/smoke checks against a test Bunny DNS zone and representative provider-detection domains.

Required test coverage:

- `ip4:` and `ip6:` extraction.
- Nested includes.
- Include loop detection.
- Max-depth blocking.
- Deduplication and deterministic ordering.
- Multiple TXT strings in one SPF record.
- Zero SPF and multiple SPF record errors.
- Oversized generated SPF blocking.
- Bunny create/update/no-op paths.
- Nameserver provider detection for Icelandic provider patterns, global provider patterns, ambiguous matches, and unknown fallbacks.
- Provider instruction rendering for DMARC TXT, SPF TXT, and SPF route CNAME records.
- No publication when resolution has blocking errors.
- Status persistence for healthy, stale, failed, and blocked routes.

## GA Acceptance Criteria

Dmark is not GA-ready until this service has either shipped or been explicitly removed from the public GA promise.

For GA with SPF flattening included:

- Backend automated tests pass for resolver, generator, Bunny client, sync service, and API contracts.
- A test Bunny zone publishes and updates a real route TXT record successfully.
- Re-running sync without changes makes no Bunny write.
- A forced include-resolution failure blocks publication and surfaces an operator-visible error.
- Dashboard shows CNAME instructions, detected DNS provider guidance, provider deep links where stable, and sync status without exposing secrets.
- Production runbook documents Bunny credentials, zone delegation, provider-guidance fallback behavior, rollback, and failure handling.
