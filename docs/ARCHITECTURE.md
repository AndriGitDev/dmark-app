# Kastro RecordMaster Architecture

Kastro RecordMaster is currently delivered by the `meil` Go binary during the transition. The binary has multiple roles: `serve`, `worker`, `ingest`, `migrate`, and `scan`.

The HTTP role serves the public checker, auth, protected domains, Brand Risk findings, API, billing, and admin surfaces. The worker role runs River-backed scan and enrichment jobs against Postgres-backed storage. Sender evidence foundations use RUA ingestion and ClickHouse aggregate storage without replacing the Checker surfaces.
