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

## P1 Public Checker

Goal: ship the public one-off Checker. Anyone can enter a single domain and see the current SPF, DKIM, DMARC, and BIMI posture without creating an account. Account creation is only required when a user wants to save and list multiple domains.

Product boundary:

1. Public/free: one-off domain checks with current DNS posture.
2. Account/free: save multiple domains and revisit their latest checks.
3. Premium/later: alerts, historical reports, suggested DNS records, DMARC aggregate reporting, attribution, and paid-tier automation.

Tasks:

1. Add DNS lookup and classification code for SPF, DMARC, BIMI, and DKIM selector probing.
2. Define result states for each check: `pass`, `warn`, `fail`, `missing`, and `unknown`.
3. Add a public checker route where submitting a domain runs a live one-off scan and renders the result.
4. Add a `meil scan <domain>` CLI path for manual verification of the same scanner logic.
5. Add Postgres schema only for data needed by the account-backed saved-domain path; keep public one-off checks usable without persistence.
6. Add domain/account tables behind the boundary for a later authenticated saved-domain dashboard, but do not build premium features in P1.
7. Add tests for DNS parsing, classification, invalid domains, timeout/error handling, and the public checker response.

Definition of Done:

1. `make dev` still works on a fresh machine.
2. A visitor can enter a domain on `/` and see live SPF, DKIM, DMARC, and BIMI results.
3. `meil scan example.com` prints the same classifications as the web checker.
4. DNS lookups have bounded timeouts and return `unknown` instead of hanging or crashing.
5. Scanner tests use mocked DNS and cover common SPF, DMARC, BIMI, and DKIM cases.
6. No alerts, reports, suggested records, billing, RUA ingestion, or sender attribution are implemented in P1.
