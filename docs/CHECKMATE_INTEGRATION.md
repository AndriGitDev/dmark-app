# Checkmate Integration

Checkmate is a strong automated sales wedge for RecordMaster because it turns abstract DMARC risk into a concrete buyer moment:

1. Enter a work email or domain.
2. Show a simple grade and evidence.
3. If the domain is vulnerable, ask the user to verify ownership.
4. Optionally send a harmless proof email from `security@customer-domain`.
5. Convert the prospect into a monitored-domain setup.

RecordMaster should absorb the funnel and scoring model, but it should not copy the spoofing capability directly into the main app process.

## What To Reuse Now

- The "DMARC Reality Check" framing.
- A simple grade for sales conversations.
- SPF, DMARC, DKIM, BIMI, and MX evidence in one result.
- Clear copy for why `p=none`, missing SPF, or missing DKIM is risky.
- A verified-inbox flow before any proof action.
- Strong per-IP and per-domain rate limits.

## What To Keep Separate

The spoofed proof email is useful, but it is also abuse-sensitive. It should remain a separate worker/service until these controls exist:

- Verified recipient email domain must exactly match the scanned domain or an approved admin contact.
- One-time token with short TTL and single-use storage.
- Per-IP, per-domain, and per-organization rate limits.
- Blocklist for public mailboxes, government, military, education, major banks, infrastructure providers, and any operator-configured domain.
- Audit log entry for every proof request and dispatch attempt.
- Explicit consent copy before dispatch.
- Dedicated mail host, isolated network, and operator-owned HELO hostname.
- No arbitrary recipient support; send only to the verified inbox.

## RecordMaster-Native Flow

Phase 1, safe GA funnel:

1. `dmark-web` runs the browser funnel and calls backend scanner/API flows as needed.
2. `internal/salesaudit` converts the scan to a sales grade, risk level, and CTA.
3. Vulnerable domains get a "claim this audit" prompt.
4. Claimed audits call `POST /api/public/audit-claims`, verify matching work-email domain, and persist a `sales_audit_claims` lead record.
5. Passwordless verification and protected-domain attachment are the next backend flow after the split; the browser UI for that flow belongs in `dmark-web`.

Phase 2, controlled proof:

1. User enters a work email.
2. RecordMaster sends a magic link through the existing notifier path.
3. User confirms the link.
4. A separate proof service sends one educational demonstration email if the domain is eligible.
5. RecordMaster stores an audit event and offers protected-domain setup.

Phase 3, automated sales loop:

1. Generate a monthly trust report preview.
2. Send the prospect a link to claim the report.
3. Offer concierge setup for DMARC reporting and monitoring.
4. Convert to Starter/Growth/Business plan.

## First Implementation Slice

The first reusable implementation is in place locally. `internal/salesaudit` produces:

- Domain.
- Grade.
- Risk level.
- Safety status.
- Proof eligibility.
- Buyer-facing message.
- CTA.
- Evidence signals.

`internal/salesaudit` remains the backend grading engine. The public checker, claim UI, and proof/demo funnel now belong in `dmark-web`; this repository backs those flows through `/api/public/scan` and `/api/public/audit-claims` JSON endpoints rather than HTML form routes.
