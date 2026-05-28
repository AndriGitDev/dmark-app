# Dmark Backend Implementation Plan

This document records the current implementation posture. The long-form product roadmap lives in `docs/ROADMAP.md`; this file is the short release-oriented companion for the repository state.

## Current Scope

Dmark is now split across two repositories: `dmark-app` is the Go backend/app service, and `dmark-web` owns the frontend and marketing website. This repository intentionally excludes frontend source, Node dependencies, and browser asset generation. It includes implementation slices for:

1. SPF, DKIM, DMARC, and BIMI checking with shared API and CLI scanner semantics.
2. Protected-domain APIs, domain CSV import/export, and backend foundations for future auth flows.
3. Scanner metadata, posture summaries, drift classification, alert lifecycle rules, and background job helpers.
4. DMARC aggregate report ingestion, ClickHouse aggregate models, Reporter summaries, sender attribution, recommendations, risk scoring, and initial Brand Risk findings from existing DNS/email-auth checks.
5. Paid-product foundations: plans, entitlements, teams, billing webhook handling, API-key helpers, webhooks, audit, privacy, admin, abuse, status, and operations support packages.
6. Operational docs for DNS setup, API/webhooks, security, threat modeling, QA, and incident handling.

## Release Gates

Before treating this roadmap implementation as releasable, run the QA plan in `docs/QA_TEST_PLAN.md` from a machine with Go, Docker, `sqlc`, `govulncheck`, and `golangci-lint` installed.

Required gates:

1. `make qa-code`
2. `make qa-services`
3. `make qa` when both code tooling and service dependencies are available
4. Fresh Postgres and ClickHouse migration checks through `make qa-services`
5. Browser/manual QA gates from `docs/QA_TEST_PLAN.md`

## Current GA Prep Status

The current local environment can run the full automated gate after installing Go, QA tooling, Docker CLI, Docker Compose, and Colima, using writable temp directories for Go and golangci-lint caches. The latest local pass covers unit tests, race tests, vet, lint, `govulncheck`, fuzz smoke tests, generated-file idempotence, build, Docker image build, Postgres migrations, River migrations, and ClickHouse migrations.

Browser QA and manual accessibility checks remain GA blockers in `dmark-web`. Backend load checks, backup/restore drills, rollback drills, and live-provider checks remain GA blockers here until completed or explicitly scoped out.

The active GA readiness tracker lives in `docs/GA_READINESS.md`.
