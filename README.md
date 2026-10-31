# dmark

dmark is an open-source email security tool for inspecting email-authentication records and helping people understand and improve their configuration. This repository contains its Go backend, account persistence, scanner, ingestion, workers and deployment assets. The bilingual [web app](https://github.com/AndriGitDev/dmark-web) lives in a separate repository.

The repository is named `dmark-app`. The Go module, binary, CLI command, database names and deployment service names remain `meil` for compatibility.

See [project scope](docs/PROJECT.md) for available web features and backend operating boundaries.

## Local Development

Prerequisites: Go 1.26.9+, Docker, `sqlc`, and `air` installed locally.

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

The public web app performs anonymous checks through Kastro Lookup and supplementary public DNS queries. This service provides optional account saving through `/api/account/*`.

Apply PostgreSQL migrations, configure the email sender and set `FRONTEND_BASE_URL=https://dmark.is`. On the web app, set `DMARK_ACCOUNT_API_URL` to this backend's HTTPS origin and `DMARK_FRONTEND_ORIGIN` to the web app origin. For local development, use `http://localhost:8080` and `http://localhost:3000` respectively, with a matching backend `FRONTEND_BASE_URL`.

If an API client calls this service from a browser directly, set `FRONTEND_ORIGINS` to exact allowed origins. Do not accept browser-supplied callback URLs. Sessions currently live in memory for 24 hours; use shared session and rate-limit storage before serving accounts across multiple replicas.

## Coolify

The legacy backend deployment target is `https://dmark.kastro.is`; the public free tool is at `https://dmark.is`. The legacy hostname `https://recordmaster.kastro.is` should remain available as a redirect during the transition. Use the Docker Compose file at `infra/coolify/docker-compose.yml`; deployment notes live in `infra/coolify/README.md`.

## Technical documentation

- [API contracts](docs/API.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Operations](docs/OPERATIONS.md)
- [DNS guidance](docs/DNS_GUIDE.md)
- [Security and data handling](docs/SECURITY.md)
- [Threat model](docs/THREAT_MODEL.md)
- [QA test plan](docs/QA_TEST_PLAN.md)
- [Release verification runbooks](docs/GA_EVIDENCE_RUNBOOKS.md)

## License

Copyright (C) 2026 dmark contributors. Licensed under [GNU AGPL version 3](LICENSE) (`AGPL-3.0-only`). See [CONTRIBUTING.md](CONTRIBUTING.md).
