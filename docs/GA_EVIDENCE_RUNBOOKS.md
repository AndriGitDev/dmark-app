# Release verification runbooks

Record the revision, environment, UTC time and result of each verification in your release record. Never publish magic links, tokens, connection strings, customer domains or private report contents.

## Automated checks

Run `make qa-code` and the configured database and container checks in CI. Run the web repository's tests, lint, build, launch checks and browser suite. Unit tests alone do not prove a deployed service or email delivery.

## Account saving

Confirm frontend and backend origins match the documented configuration. Send a controlled email link to an operator-owned address. Confirm delivery, deliberate sign-in confirmation, workspace save and reload from another device. Check unavailable email and backend errors remain visible. Verify account isolation, sign-out, stale-save conflicts and explicit guest import.

## Backup and restore

Take an encrypted backup using the production backup procedure. Restore it into an isolated disposable database. Verify migrations, record counts and representative account/domain reads. Test the documented rollback procedure. Never restore over the running production database as a verification step.

## Domain checks and DNS publication

Run an on-demand check for an operator-owned domain and inspect SPF, DMARC, explicit DKIM-selector and BIMI findings. Verify timeouts remain distinct from missing records. Review guidance against legitimate sender evidence before changing DNS.

If DNS publication workers are configured, test with an operator-owned canary, inspect the generated record, verify authoritative propagation and rehearse rollback to the last known good record. Inventory all dependent domains before retiring a worker or publishing credentials. The public web app's SPF generator only produces a manually published snapshot.

## Release review

Confirm public descriptions match the enabled features, both source repositories and license links are available, and no private data or credentials are included in the release. Run a keyboard and mobile accessibility pass. Check deployed health, indexing, social images and actual account/email behavior after rollout.
