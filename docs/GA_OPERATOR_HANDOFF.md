# Dmark GA Operator Handoff

This is the founder/operator checklist for the final GA-or-public-beta decision. It is intentionally boring: every step either produces evidence, names a blocker, or keeps the launch promise scoped honestly.

**Current posture:** launch candidate if the gates below pass; otherwise public beta with SPF-route/live-provider limitations stated explicitly.

## 1. Non-mutating preflight

Run these from clean `main` checkouts before touching production providers.

```sh
# Backend repo
cd /root/workspace/github/AndriGitDev/dmark-app
git fetch origin main
git status --short
git rev-parse HEAD
git rev-parse origin/main
gh run list --branch main --limit 5 --json databaseId,headSha,status,conclusion,workflowName,url

# Frontend repo
cd /root/workspace/github/AndriGitDev/dmark-web
git fetch origin main
git status --short
git rev-parse HEAD
git rev-parse origin/main
gh run list --branch main --limit 5 --json databaseId,headSha,status,conclusion,workflowName,url
```

Expected evidence:

- local and remote `main` match in both repos;
- `dmark-app` latest exact head SHA has successful CI;
- `dmark-web` may have no visible GitHub Actions; use local `npm test`, `npm run lint`, and `npm run build` as the gate evidence when no exact-head CI exists.

## 2. Local release gates

Use the exact local host workaround where needed: Havana has `docker-compose` v1 but not the Docker Compose v2 plugin, so pass `DOCKER_COMPOSE=docker-compose` for backend RC gates.

```sh
cd /root/workspace/github/AndriGitDev/dmark-app
make rc-backend DOCKER_COMPOSE=docker-compose
make rc-backend-services DOCKER_COMPOSE=docker-compose

cd /root/workspace/github/AndriGitDev/dmark-web
npm test
npm run lint
npm run build
```

If `make generate-check` fails because `sqlc` is missing, install/use sqlc `v1.31.1` and rerun. Do not commit generated-file header churn from a different sqlc version.

## 3. Read-only production smoke

These checks must not mutate Bunny, Coolify, databases, or customer data.

```sh
cd /root/workspace/github/AndriGitDev/dmark-app
go run ./cmd/meil synthetic-check --timeout=20s --json \
  --scan-fixture-domain dmark.is \
  --scan-fixture-domain example.com \
  --scan-fixture-domain xn--bcher-kva.example

curl -fsS https://dmark.is/ >/tmp/dmark-home.html
curl -fsS https://dmark.kastro.is/status >/tmp/dmark-status.json
curl -fsS https://dmark.kastro.is/healthz >/tmp/dmark-healthz.txt
curl -fsS 'https://dmark.kastro.is/api/public/scan?domain=kastro.is' >/tmp/dmark-kastro-scan.json

python3 - <<'PY'
import json
from pathlib import Path

html = Path('/tmp/dmark-home.html').read_text(errors='replace')
scan = Path('/tmp/dmark-kastro-scan.json').read_text(errors='replace')
status = Path('/tmp/dmark-status.json').read_text(errors='replace') if Path('/tmp/dmark-status.json').exists() else ''
needles = {
    'dmark-privacy-analytics': 'privacy analytics marker',
    'googletagmanager.com': 'Google Tag Manager',
    'google-analytics.com': 'Google Analytics',
    'connect.facebook.net': 'Meta/Facebook tracking',
    'facebook.com/tr': 'Meta pixel',
}
print('homepage_bytes', len(html))
for needle, label in needles.items():
    print(f"{label}: {'FOUND' if needle in html else 'absent'}")
print('status_contains_operational', 'operational' in status.lower())
print('scan_contains_kastro_is', 'kastro.is' in scan)
print('scan_top_level_keys', ','.join(sorted(json.loads(scan).keys())[:20]))
PY
```

Evidence to record:

- synthetic JSON has `failed:false`;
- frontend homepage returns `200`;
- backend `/status` and `/healthz` return `200`;
- public scan returns `200` and contains `kastro.is` plus a current scan marker;
- homepage source has no invasive analytics markers and no `dmark-privacy-analytics` marker unless privacy analytics were intentionally enabled.

Latest verified non-mutating smoke evidence before human-gated work:

- 2026-06-05 17:46 UTC: `go run ./cmd/meil synthetic-check --timeout=20s --json --scan-fixture-domain dmark.is --scan-fixture-domain example.com --scan-fixture-domain xn--bcher-kva.example` returned `failed:false` for backend commit `01ddf4fe75c31cd76ac13400e836cf5be0d2c180`.
- 2026-06-05 17:47 UTC homepage/API marker smoke: `https://dmark.is/` returned HTML (`37196` bytes), `https://dmark.kastro.is/status` contained `operational`, `https://dmark.kastro.is/healthz` returned `ok`, and `https://dmark.kastro.is/api/public/scan?domain=kastro.is` returned JSON containing `kastro.is` with top-level keys `audit,checked_at,checks,domain,duration_ms,summary`.
- The homepage source marker check found `dmark-privacy-analytics`, Google Tag Manager, Google Analytics, Meta/Facebook script, and Meta pixel markers all absent.

Latest operator-gate pass evidence:

- 2026-06-05 18:08 UTC preflight: `dmark-app` was clean on `main...origin/main` at `ca8db82b09071aac2e8c50706508f83b58008aa3`; exact-head CI run `27030871109` completed successfully. `dmark-web` was aligned at `cced9852b2bb7ce0bab7789f904ee913769e1721`; local untracked `DMARK.zip` was outside the app repo and no frontend CI runs were visible.
- 2026-06-05 18:02 UTC backend RC gates: `PATH=/root/go/bin:$PATH make rc-backend DOCKER_COMPOSE=docker-compose` passed, including tests, race tests, vet, golangci-lint (`0 issues`), govulncheck (`No vulnerabilities found`), sqlc generated-file check, build, Compose config validation, and Docker image build.
- 2026-06-05 18:02 UTC service-backed RC gates: `PATH=/root/go/bin:$PATH make rc-backend-services DOCKER_COMPOSE=docker-compose` passed fresh Postgres migrations through version 8, River migrations through version 6, fresh ClickHouse migrations, and `internal/store/pg` integration tests.
- 2026-06-05 18:03 UTC frontend gates: `npm test` passed 57/57, `npm run lint` passed, and `npm run build` passed.
- 2026-06-05 18:06 UTC read-only canary DNS check: `route-published.canary.dmark.is`, `route-published.route.dmark.is`, `broken-spf.canary.dmark.is`, and `spf-too-many-lookups.canary.dmark.is` did not yet resolve; no Bunny mutation was performed.
- 2026-06-05 18:06 UTC unauthenticated production API boundary check: backend `/healthz` returned `200`; admin provider health and dashboard domain list endpoints returned `401` with `valid bearer API key is required`.
- 2026-06-05 18:08 UTC local non-production Postgres backup/restore drill passed against disposable Compose Postgres: migrations applied, `pg_dump` produced a `78591` byte dump, the local database was dropped/recreated, restore succeeded, `goose_db_version` remained queryable with `9` rows, and a harmless marker row restored as `dmark local restore drill`. This is local restore evidence only, not production backup evidence.

## 4. Explicit operator-only mutation: Bunny route canary

Do this only after confirming the production Bunny DNS key is scoped appropriately and loaded in the backend environment. Never paste the key into logs, docs, Memento, Telegram, shell history, or issue comments.

Target canary:

- customer-facing CNAME: `route-published.canary.dmark.is`
- route target: `route-published.route.dmark.is`
- expected target TXT: a safe SPF value such as `v=spf1 ip4:192.0.2.10 ~all` or another non-customer test value

Procedure:

1. Create or select a protected-domain/test-route fixture through the normal application path, not by hand-editing the database unless this is explicitly a break-glass drill. The API path is `POST /api/v1/domains` with a bearer API key scoped for domain writes.
2. Configure route sources so the generated SPF value is deterministic and safe through `PUT /api/v1/dashboard/domains/{id}/spf-route`, with a payload like:

```json
{
  "route_fqdn": "route-published.route.dmark.is",
  "customer_cname_name": "route-published.canary.dmark.is",
  "customer_cname_target": "route-published.route.dmark.is",
  "enabled": true,
  "mechanism_policy": "softfail",
  "sources": [
    {"kind": "ip4", "value": "192.0.2.10", "label": "GA canary TEST-NET-1", "enabled": true}
  ]
}
```

3. Trigger manual SPF route sync through `POST /api/v1/dashboard/domains/{id}/spf-route/sync`, or use the dashboard's SPF route sync action. Do not call Bunny directly unless this is an explicit rollback/break-glass action.
4. Verify public DNS:

```sh
dig +short CNAME route-published.canary.dmark.is
dig +short TXT route-published.route.dmark.is
```

5. Rerun the same sync without source changes and verify the route remains no-op/unchanged from application audit evidence.
6. Force a controlled resolver/include failure and verify publication is blocked and the last good TXT remains resolvable.
7. If retained as a permanent canary, add it to the read-only synthetic smoke with route CNAME/TXT expectations.

Rollback expectation:

- identify the last good persisted SPF value and Bunny record ID from the SPF route audit/admin surface;
- restore that TXT in Bunny or disable the route safely;
- re-run `dig +short TXT route-published.route.dmark.is` and synthetic smoke;
- record the exact operator action without secrets.

## 5. Live provider checks

Before claiming GA, either prove or explicitly scope each integration:

- Bunny DNS route publication: must be proven for GA SPF Route.
- Lettermint magic-link/transactional email: send one controlled test to an operator-owned address and verify delivery without logging tokens.
- DNS provider detection: run representative owned/public domains and record detected/ambiguous/unknown outcomes honestly.
- Billing webhooks, enrichment/geolocation, and advanced Reporter ingestion: label beta/operator-managed unless live proof exists.

## 6. Recovery drills

At minimum before GA, record evidence for:

- Postgres backup and restore drill against a non-production or explicitly approved restore target;
- ClickHouse restore/rebuild/retention drill if Reporter is GA-scoped;
- Coolify rollback drill for app and worker services;
- SPF route rollback drill from the canary above.

If any drill cannot be run safely before launch, choose public beta or hold. DNS does not negotiate.

## 7. Launch decision

Current recommended launch posture unless the operator-gated proof below is completed: **public beta**, not GA.

Public beta scope:

- Public scanner, public marketing flows, and selected dashboard/read flows may launch if frontend/backend gates and read-only production smoke pass.
- SPF route publication, live email/provider delivery, billing webhooks, advanced Reporter ingestion, and recovery claims remain beta/operator-managed unless separately proven.
- Do not claim GA for Bunny-backed SPF route publication until live create/update/no-op/blocking behaviour and SPF rollback evidence are documented.
- Do not claim GA recoverability until Postgres/ClickHouse restore and Coolify rollback drills have been run and recorded.

Use this exact decision rule:

- **GA:** all must-have release gates pass, live Bunny proof is documented, recovery drills are complete, and public copy promises only proven capabilities.
- **Public beta:** public scanner/marketing and selected dashboard flows are live, unfinished provider/recovery/billing/Reporter surfaces are locked or labelled beta/operator-managed, and support expectations are explicit.
- **Hold:** any unresolved safety, recovery, DNS, auth, privacy, data-loss, or provider-secret handling concern remains.

## 8. Evidence packet to capture

For the final release note / Memento handoff, record:

- backend commit SHA, app CI run URL, backend RC gate outputs;
- frontend commit SHA, local frontend gate outputs, and CI visibility status;
- production smoke command and result summaries;
- live-provider proof outcomes or explicit beta scope cuts;
- recovery drill outcomes;
- final decision: GA / public beta / hold;
- any steps delegated to the founder/operator.

Never record raw API keys, magic links, bearer tokens, connection strings, or customer-private DNS/report data.
