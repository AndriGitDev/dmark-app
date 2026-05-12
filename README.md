# Meil

Meil is email-auth monitoring for domain owners. The free Checker tier watches SPF, DKIM, DMARC, and BIMI records for drift; the later paid Reporter tier will ingest DMARC aggregate reports and show who is sending mail as your domain.

## Local Development

Prerequisites: Go 1.23+, Docker, `templ`, `sqlc`, `tailwindcss`, and `air` installed locally.

```sh
make dev
```

This starts Postgres and Mailpit, runs the current migration stub, regenerates templ/sqlc/Tailwind artifacts, and starts `meil serve` with hot reload on `:8080`.

Useful commands:

```sh
make test
make lint
make build
```

## Coolify

The P0 deployment target is `https://meil.kastro.is`. Use the Docker Compose file at `infra/coolify/docker-compose.yml`; deployment notes live in `infra/coolify/README.md`.
