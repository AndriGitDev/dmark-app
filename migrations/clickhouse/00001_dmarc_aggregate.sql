CREATE TABLE IF NOT EXISTS dmarc_aggregate_rows
(
    domain String,
    report_id String,
    org_name String,
    begin_at DateTime,
    end_at DateTime,
    source_ip IPv6,
    source_org String,
    source_asn String,
    source_country FixedString(2),
    header_from String,
    disposition LowCardinality(String),
    dkim_aligned_result LowCardinality(String),
    spf_aligned_result LowCardinality(String),
    provider String,
    confidence LowCardinality(String),
    count UInt64,
    ingested_at DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(begin_at)
ORDER BY (domain, begin_at, source_ip, header_from)
TTL begin_at + INTERVAL 730 DAY DELETE;

CREATE TABLE IF NOT EXISTS dmarc_report_imports
(
    domain String,
    org_name String,
    report_id String,
    content_hash String,
    source_file String,
    row_count UInt64,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (domain, org_name, report_id, content_hash);
