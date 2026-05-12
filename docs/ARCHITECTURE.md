# Architecture

Meil is a single Go binary with multiple roles: `serve`, `worker`, `ingest`, `migrate`, and `scan`.

P0 only wires the HTTP role. v1 will add auth, domain CRUD, DNS scanning, drift detection, and notifications on Postgres-backed storage and River jobs. v2 will add RUA ingestion, ClickHouse reporting, billing, and attribution without replacing the v1 Checker surfaces.
