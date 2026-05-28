# Dmark Architecture

Dmark is currently delivered by the `meil` Go binary during the transition. The binary has multiple roles: `serve`, `worker`, `ingest`, `migrate`, and `scan`.

The HTTP role serves backend JSON/API, public scan/audit JSON, billing, webhook, status, and admin surfaces. Browser-facing checker, login, protected-domain dashboard, and marketing pages live in the separate `dmark-web` repository. This repository does not build or serve frontend assets; legacy browser routes return JSON `404` handoff responses so stale links fail clearly.

The worker role runs River-backed scan and enrichment jobs against Postgres-backed storage. Sender evidence foundations use RUA ingestion and ClickHouse aggregate storage.
