# ClickHouse

Reporter aggregate rows use ClickHouse for date-range and high-cardinality source queries.

Local development starts ClickHouse through `infra/coolify/docker-compose.dev.yml`.

Apply the initial schema through the ClickHouse migration runner:

```sh
CLICKHOUSE_URL=http://localhost:8123 \
CLICKHOUSE_DATABASE=meil \
CLICKHOUSE_USER=meil \
CLICKHOUSE_PASSWORD=meil \
go run ./cmd/meil migrate clickhouse up
```

Use `go run ./cmd/meil migrate clickhouse status` to list tables visible to the configured database.

The `dmarc_aggregate_rows` table is partitioned by report month and ordered by domain, report start, source IP, and header-from domain. The default TTL deletes aggregate rows after 730 days; plan-level retention can shorten this in application code.
