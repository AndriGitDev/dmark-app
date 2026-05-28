# Kastro RecordMaster Security

Report security issues privately to the project owner before opening public issues.

## Sensitive Surfaces

- Passwordless login tokens are generated randomly, hashed at rest, expire, and are single-use.
- API scan requests must be authenticated and body-limited where applicable.
- DMARC report ingestion rejects unsupported compression, oversized payloads, invalid XML, and unsafe zip paths.
- Billing webhooks must be verified with `BILLING_WEBHOOK_SECRET` before subscription state changes are applied.
- API keys and invitation tokens must be stored as hashes only.

## Data Handling

- One-off API checks should not be persisted unless explicitly attached to a protected domain.
- Protected-domain snapshots, Brand Risk findings, drift events, notifications, and audit logs are customer data.
- Raw DMARC XML is retained for auditability and must follow plan-level retention controls.
- Audit logs should be append-only and should not include secret tokens, raw API keys, or magic links.

## Subprocessors

Current subprocessors:

- Lettermint for passwordless login and alert email delivery.
- Deployment/database infrastructure configured by the operator.
- Future billing provider for paid plans.

Update this list before production use when providers or regions change.

## Dependency And Vulnerability Triage

1. Review dependency alerts at least weekly and before every production release.
2. Classify each alert by reachable surface: API scan, auth/session, report ingestion, billing/webhooks, background jobs, or development-only tooling.
3. Patch critical reachable runtime vulnerabilities immediately and hold release until the fix is deployed.
4. Patch high severity runtime vulnerabilities within seven days or document a compensating control.
5. Patch medium/low and development-only findings in the next routine dependency update window.
6. Record the affected package, version, exploitability decision, fix version, validation command, and deploy date in the security changelog or issue tracker.
7. Rerun `go test ./...`, `go vet ./...`, `golangci-lint run`, and container build checks after dependency changes.
