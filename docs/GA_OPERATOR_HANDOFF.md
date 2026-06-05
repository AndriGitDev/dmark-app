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
curl -fsS https://dmark.kastro.is/status
curl -fsS https://dmark.kastro.is/healthz
curl -fsS 'https://dmark.kastro.is/api/public/scan?domain=kastro.is' >/tmp/dmark-kastro-scan.json
```

Evidence to record:

- synthetic JSON has `failed:false`;
- frontend homepage returns `200`;
- backend `/status` and `/healthz` return `200`;
- public scan returns `200` and contains `kastro.is` plus a current scan marker;
- homepage source has no invasive analytics markers and no `dmark-privacy-analytics` marker unless privacy analytics were intentionally enabled.

## 4. Explicit operator-only mutation: Bunny route canary

Do this only after confirming the production Bunny DNS key is scoped appropriately and loaded in the backend environment. Never paste the key into logs, docs, Memento, Telegram, shell history, or issue comments.

Target canary:

- customer-facing CNAME: `route-published.canary.dmark.is`
- route target: `route-published.route.dmark.is`
- expected target TXT: a safe SPF value such as `v=spf1 ip4:192.0.2.10 ~all` or another non-customer test value

Procedure:

1. Create or select a protected-domain/test-route fixture through the normal application path, not by hand-editing the database unless this is explicitly a break-glass drill.
2. Configure route sources so the generated SPF value is deterministic and safe.
3. Trigger manual SPF route sync through the backend/dashboard API path.
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
