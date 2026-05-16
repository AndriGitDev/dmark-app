# Architecture

Meil is a single Go binary with multiple roles: `serve`, `worker`, `ingest`, `migrate`, and `scan`.

The HTTP role serves the public checker, auth, saved domains, API, billing, and admin surfaces. The worker role runs River-backed scan and enrichment jobs against Postgres-backed storage. Reporter foundations use RUA ingestion and ClickHouse aggregate storage without replacing the Checker surfaces.
