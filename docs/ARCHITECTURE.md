# Kastro RecordMaster Architecture

Kastro RecordMaster is currently delivered by the `meil` Go binary during the transition. The binary has multiple roles: `serve`, `worker`, `ingest`, `migrate`, and `scan`.

The HTTP role serves backend JSON/API, billing, webhook, status, and admin surfaces. Browser-facing checker, login, protected-domain dashboard, and marketing pages live in the separate `dmark-web` repository. The worker role runs River-backed scan and enrichment jobs against Postgres-backed storage. Sender evidence foundations use RUA ingestion and ClickHouse aggregate storage.
