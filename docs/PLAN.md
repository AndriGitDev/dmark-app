# Kastro RecordMaster Implementation Plan

This document records the current implementation posture. The long-form product roadmap lives in `docs/ROADMAP.md`; this file is the short release-oriented companion for the repository state.

## Current Scope

Kastro RecordMaster is now beyond the original P0/P1 bootstrap. The repository is still named `meil`, and includes implementation slices for:

1. Public SPF, DKIM, DMARC, and BIMI checking with shared web and CLI scanner semantics.
2. Passwordless login, protected domains, CSRF-protected app routes, and domain CSV import/export.
3. Scanner metadata, posture summaries, drift classification, alert lifecycle rules, and background job helpers.
4. DMARC aggregate report ingestion, ClickHouse aggregate models, Reporter summaries, sender attribution, recommendations, risk scoring, and initial Brand Risk findings from existing DNS/email-auth checks.
5. Paid-product foundations: plans, entitlements, teams, billing webhook handling, API-key helpers, webhooks, audit, privacy, admin, abuse, status, and operations support packages.
6. Operational docs for DNS setup, API/webhooks, security, threat modeling, QA, and incident handling.

## Release Gates

Before treating this roadmap implementation as releasable, run the QA plan in `docs/QA_TEST_PLAN.md` from a machine with Go, Docker, `templ`, `sqlc`, `tailwindcss`, `govulncheck`, and `golangci-lint` installed.

Required gates:

1. `make qa-code`
2. `make qa-services`
3. `make qa` when both code tooling and service dependencies are available
4. Fresh Postgres and ClickHouse migration checks through `make qa-services`
5. Browser/manual QA gates from `docs/QA_TEST_PLAN.md`

## Known Local Blocker

The current environment can run `make qa-code` with the temporary local Go 1.25.10 toolchain and cached CLI tools. It cannot complete `make qa-services` because Docker socket access is denied, localhost Postgres rejects the documented `meil:meil` credentials, and ClickHouse is not reachable on `localhost:8123`.
