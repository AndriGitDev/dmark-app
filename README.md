# Meil

Meil is email-auth monitoring for domain owners. The free Checker tier watches SPF, DKIM, DMARC, and BIMI records for drift; Reporter foundations ingest DMARC aggregate reports and model who is sending mail as your domain.

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

The P0 deployment target is `https://meil.kastro.is`. Use the Docker Compose file at `infra/coolify/docker-compose.yml`; deployment notes live in `infra/coolify/README.md`.

## Planning

- Long-term product and engineering roadmap: `docs/ROADMAP.md`
- Roadmap implementation audit: `docs/ROADMAP_AUDIT.md`
- Staged QA and release test plan: `docs/QA_TEST_PLAN.md`
- Operational release and incident process: `docs/OPERATIONS.md`
- DNS setup and report forwarding guidance: `docs/DNS_GUIDE.md`
- Security and data-handling notes: `docs/SECURITY.md`
- API and webhook contracts: `docs/API.md`
- Threat model: `docs/THREAT_MODEL.md`
