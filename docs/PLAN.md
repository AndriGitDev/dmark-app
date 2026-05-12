# Meil Bootstrap Plan

Email-auth monitoring for domain owners. The free Checker tier watches SPF, DKIM, DMARC, and BIMI records and alerts on drift. The paid Reporter tier will ingest DMARC aggregate reports and map every IP that sends mail as a customer's domain.

## P0 Scaffold

Goal: empty but well-formed repo, `make dev` works, a "hello world" page at `/` renders via templ, and CI is green.

Tasks:

1. Initialize `github.com/AndriGitDev/meil`.
2. Add core deps: chi, templ, scs, pgx, river, goose, sqlc, slog, otel, cobra, env.
3. Wire cobra root plus `serve`, `worker`, `ingest`, `migrate`, and `scan` subcommands.
4. Stand up `internal/web` with `GET /` rendering a templ page that says "Meil" using the dark design baseline.
5. Add Dockerfile, dev compose, and `.env.example`.
6. Add GitHub Actions for build, lint, test, and templ-generate-check.
7. Add `.golangci.yml` with govet, staticcheck, gofumpt, errcheck, and gosec.
8. Add README with product summary and `make dev` recipe.

Definition of Done:

1. `git clone && make dev` works on a fresh machine with Go, Docker, `templ`, `sqlc`, `tailwindcss`, and `air` installed.
2. CI passes on first PR.
3. `curl localhost:8080/` returns rendered HTML.

## Phase Boundary

Do not start P1 in the P0 PR. Auth, domain CRUD, scanning, drift detection, alerts, billing, RUA ingestion, attribution, and paid-tier alerts are deferred to later phases.
