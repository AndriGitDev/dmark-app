# Dmark

Dmark is an interactive DMARC, SPF, and DKIM diagnostic tool and remediation platform for teams that need trusted email delivery.

The free diagnostic flow provides public DNS and email-auth posture checks. Monitor adds protected domains, scheduled scans, drift alerts, and history. Reporter ingests DMARC aggregate reports and models who is sending mail as your domain. Defender expands the platform into brand-name and lookalike-domain risk monitoring.

The repository is still named `meil` during the transition from the original email-auth monitoring product. The Go module, binary, CLI command, database names, and deployment service names intentionally remain `meil` for now.

## Local Development

Prerequisites: Go 1.25.10+, Docker, `templ`, `sqlc`, `tailwindcss`, and `air` installed locally.

```sh
make dev
```

This starts Postgres, Mailpit, and ClickHouse; runs Postgres app/River migrations; regenerates templ/sqlc/Tailwind artifacts; and starts `meil serve` with hot reload on `:8080`.

Useful commands:

```sh
make qa-code
make qa-services
make test
make lint
make build
```

## Coolify

The canonical deployment target is `https://dmark.is`. The legacy hostname `https://recordmaster.kastro.is` should remain available as a redirect during the transition. Use the Docker Compose file at `infra/coolify/docker-compose.yml`; deployment notes live in `infra/coolify/README.md`.

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
