# Dmark Backend

Dmark is an interactive DMARC, SPF, and DKIM diagnostic and remediation platform for teams that need trusted email delivery. This repository contains the Go backend, worker, ingestion, scanner, API, and deployment assets.

The frontend and marketing website live in the separate `dmark-web` repository. This repository intentionally has no Node, Next.js, Tailwind, templ, or static browser asset build. Keep visual marketing work there; keep domain logic, persistence, jobs, API contracts, ingestion, and operational work here.

The repository is named `dmark-app` after the Dmark app split. The Go module, binary, CLI command, database names, and deployment service names intentionally remain `meil` for now.

## Local Development

Prerequisites: Go 1.26.4+, Docker, `sqlc`, and `air` installed locally.

```sh
make dev
```

This starts Postgres, Mailpit, and ClickHouse; runs Postgres app/River migrations; regenerates sqlc artifacts; and starts `meil serve` with hot reload on `:8080`.

Useful commands:

```sh
make qa-code
make qa-services
make test
make lint
make build
```

## Frontend Connection

`dmark-web` can connect to this backend through the public JSON bridge:

```sh
DMARK_SCAN_API_URL=https://dmark.kastro.is/api/public/scan
DMARK_USE_MOCK=false
```

If browser code calls the backend directly, set `FRONTEND_ORIGINS` on this service to a comma-separated list of exact frontend origins, for example `https://dmark.is,https://www.dmark.is`.

## Coolify

The current deployment target is `https://dmark.kastro.is` until `dmark.is` is purchased. The legacy hostname `https://recordmaster.kastro.is` should remain available as a redirect during the transition. Use the Docker Compose file at `infra/coolify/docker-compose.yml`; deployment notes live in `infra/coolify/README.md`.

## Planning

- Long-term product and engineering roadmap: `docs/ROADMAP.md`
- Roadmap implementation audit: `docs/ROADMAP_AUDIT.md`
- GA readiness tracker: `docs/GA_READINESS.md`
- Competitive Icelandic DMARC SaaS roadmap: `docs/COMPETITIVE_ROADMAP.md`
- Checkmate sales-audit integration notes: `docs/CHECKMATE_INTEGRATION.md`
- Staged QA and release test plan: `docs/QA_TEST_PLAN.md`
- Operational release and incident process: `docs/OPERATIONS.md`
- DNS setup and report forwarding guidance: `docs/DNS_GUIDE.md`
- Security and data-handling notes: `docs/SECURITY.md`
- API and webhook contracts: `docs/API.md`
- Threat model: `docs/THREAT_MODEL.md`
- Brand-risk research: `docs/BRAND_RISK_RESEARCH.md`
