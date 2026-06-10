# GA Evidence Runbooks

**Status: all four gates passed and GA was declared on 2026-06-10** (see `docs/GA_READINESS.md`). These runbooks remain the canonical procedures for re-running any gate after material changes (provider config, infra moves, restore-target changes) and for the SPF route gate when it graduates from operator dog-fooding to customer rollout.

Step-by-step operator runbooks for the four human-gated GA gates in `docs/GA_READINESS.md`. Each runbook ends with an explicit evidence line to paste into `docs/GA_READINESS.md` and the evidence packet (`docs/GA_OPERATOR_HANDOFF.md` section 8). Never record magic links, tokens, API keys, or connection strings.

Run the non-mutating preflight from `docs/GA_OPERATOR_HANDOFF.md` section 1 before any runbook below.

## Runbook 1 — Lettermint live magic-link proof

Blocked on 2026-06-05: Lettermint rejected controlled sends with `VALIDATION_ERROR` because the team is unverified and can send only to verified team members.

1. **Console (human):** in the Lettermint console, complete team verification for the sending domain used by `LETTERMINT_FROM` (see Coolify env; currently a `dmark.kastro.is` sender). Verify the SPF/DKIM/return-path records Lettermint asks for, or add the operator address as a verified team member as an interim step.
2. **Confirm config without secrets:** in Coolify, confirm `LETTERMINT_BASE_URL`, `LETTERMINT_TOKEN` (set, not echoed), and `LETTERMINT_FROM` on the app service.
3. **Trigger a controlled magic-link send** to an operator-owned (verified) address through the production login flow:

   ```sh
   curl -sS -X POST https://dmark.kastro.is/login \
     -d "email=<operator-owned-address>" -o /dev/null -w "%{http_code}\n"
   ```

4. **Verify delivery:** open the operator mailbox, confirm the mail arrived from the configured sender, and complete the login once to prove the link works end to end. Do not paste the link or token anywhere.
5. **Check dispatch result server-side** (no tokens are logged by design): app logs in Coolify should show a successful Lettermint dispatch; on failure they show the provider error class.
6. **Evidence line:** `Lettermint live proof <UTC timestamp>: team verified, controlled magic-link send to operator-owned address delivered and login completed; message-id <id>; no links/tokens recorded.`

## Runbook 2 — Production Postgres backup/restore proof

The 2026-06-05 drill used a local disposable Compose Postgres; GA needs evidence against the production database or an explicitly approved restore target.

1. **Dump (production credentials, Coolify host shell):**

   ```sh
   pg_dump -Fc "$DATABASE_URL" -f /tmp/dmark-prod-$(date -u +%Y%m%dT%H%M%SZ).dump
   ls -la /tmp/dmark-prod-*.dump   # record size + timestamp
   ```

2. **Restore into a scratch database on an approved target** (never over the live database):

   ```sh
   createdb dmark_restore_drill
   pg_restore -d dmark_restore_drill --no-owner /tmp/dmark-prod-<ts>.dump
   ```

3. **Verify the restored schema and data:**

   ```sh
   psql dmark_restore_drill -c "SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1;"   # expect 8
   psql dmark_restore_drill -c "SELECT count(*) FROM organizations;"
   psql dmark_restore_drill -c "SELECT count(*) FROM domains;"
   psql dmark_restore_drill -c "SELECT count(*) FROM domain_checks;"
   psql dmark_restore_drill -c "SELECT count(*) FROM dmarc_reports;"
   ```

   Spot-check the counts against production expectations.
4. **Destroy the scratch target and the dump:**

   ```sh
   dropdb dmark_restore_drill
   rm /tmp/dmark-prod-*.dump
   ```

5. **Evidence line:** `Production Postgres restore drill <UTC timestamp>: pg_dump -Fc (<bytes>) restored into scratch DB on <target>, goose version 8, org/domain/check/report counts verified, scratch destroyed.`

ClickHouse restore/rebuild is only required if Reporter is GA-scoped; otherwise label Reporter beta and skip.

## Runbook 3 — Coolify previous-image rollback proof

The 2026-06-05 drill redeployed the *current* resource; GA rollback claims need a previous-image rollback for app and worker.

1. **Record the current deployment:** note the running commit from `https://dmark.kastro.is/status` (the `version` block) and the image digest from Coolify for both the app and worker services.
2. **Roll back:** in Coolify, redeploy the previous successful image/commit for the app service, then the worker service.
3. **Verify the rollback took:**

   ```sh
   curl -sS https://dmark.kastro.is/healthz          # expect ok (allow a brief 503 during deploy)
   curl -sS https://dmark.kastro.is/status | grep -o '"commit":"[^"]*"'   # expect the PREVIOUS commit
   go run ./cmd/meil synthetic-check --timeout=20s --json
   ```

4. **Roll forward** to the current image/commit and re-verify `/healthz`, `/status` (current commit), and synthetic smoke `failed:false`.
5. **Evidence line:** `Coolify previous-image rollback drill <UTC timestamp>: app+worker rolled back from <current-sha> to <previous-sha>, healthz/status/synthetic verified on both directions; deployment IDs <ids>.`

## Runbook 4 — Public copy lock

Compare every public claim against proven capability before flipping GA language. Claim-by-claim status as of 2026-06-10:

| Public claim | Proof status | GA copy rule |
| --- | --- | --- |
| Public scanner / instant diagnostic | Proven (live, synthetic smoke) | May state plainly |
| SPF flattening / route publication | Bunny canary create/update/no-op/blocking proven 2026-06-05; rollback covered by canary restore | Keep "beta/early access" until Runbook 3 + route delete/restore evidence, then may state plainly |
| Magic-link / email login | Blocked (Runbook 1) | Must stay "early access" until proven |
| Reporting / sender intelligence | Dashboard reads proven; live RUA mailbox ingestion not wired | Label "roadmap/beta"; do not claim live report ingestion |
| Recoverability / safe rollout | Local drills only until Runbooks 2-3 pass | No durability/rollback claims until proven |
| Billing/subscriptions | Webhook handler implemented, no live provider proof | Keep "request early access", no self-serve billing claims |

1. Re-read the marketing copy in `dmark-web/components/dmark-marketing.tsx` (hero, trust signals, workflow, features, pricing) against the table.
2. The current copy already scopes automation/reporting as beta/early-access ("Beta automation", "Early access", "SPF flattening in beta", "Readable reporting roadmap") — confirm no regression before each release.
3. Andri approves the final wording; record approval as: `Copy lock <UTC timestamp>: public claims reviewed against proven capabilities, approved by Andri.`

## After all four runbooks pass

1. Rerun full gates and record SHAs/run IDs per `docs/GA_OPERATOR_HANDOFF.md` sections 1-3:
   - backend: `make rc-backend` and `make rc-backend-services` (use `DOCKER_COMPOSE=docker-compose` where the compose plugin is unavailable), exact-head CI run ID;
   - frontend: `npm test`, `npm run lint`, `npm run build`.
2. Update `docs/GA_READINESS.md`: move each completed gate from "Remaining Human/Operator-Gated GA Gates" into the verified evidence list with its evidence line, and flip the "Current Status" posture from public beta to GA only when all four are recorded.
3. Capture the evidence packet per `docs/GA_OPERATOR_HANDOFF.md` section 8.
