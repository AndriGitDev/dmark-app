# RecordMaster Competitive Roadmap

RecordMaster is the Icelandic DMARC, email-authentication, and domain-trust platform for SMB and midmarket companies. The goal is to beat Dmarcian and EasyDMARC locally first, then expand feature parity where customers prove demand.

## Positioning

Promise: stop email impersonation before customers are fooled.

Target customer:

- Icelandic SMB and midmarket companies with customer-facing email, invoices, bookings, portals, or brand-sensitive communication.
- Companies that need trustworthy setup and monitoring but do not have a dedicated email security engineer.
- Founder-led sales first, with concierge setup and monthly proof.

Default product language:

- Product UI in English for GA.
- Icelandic sales material, onboarding checklists, executive reports, and support.

## Competitor Baseline

Dmarcian is strong at DMARC education, RUA processing, policy management, alerting, domain/user limits, API/SSO in higher tiers, and expert positioning.

EasyDMARC is broader and more productized: RUA/RUF, aggregate maps, vendor identification, TLS reporting, BIMI, managed DMARC/BIMI/DKIM/MTA-STS, SPF flattening, reputation monitoring, DNS/provider integrations, MSP packaging, and enterprise support.

RecordMaster should not try to match every enterprise checkbox before GA. The first win is a better local buyer experience:

- Local trust and `.is` context.
- Concierge DMARC enforcement.
- Clear executive reports.
- Evidence-backed Defender findings.
- Faster support and direct implementation help.

## Beat Points

1. Iceland-first trust monitoring
   - `.is` and Icelandic-character lookalike candidates.
   - Local sales, onboarding, and reports.
   - Icelandic business-hour support and founder-led implementation.

2. Better path to enforcement
   - Track every domain through `not_started`, `observing`, `ready_for_quarantine`, `quarantine`, `ready_for_reject`, and `reject`.
   - Explain why a domain is or is not ready for the next DMARC step.
   - Make the next DNS change explicit.

3. Better monthly proof
   - One executive summary per customer.
   - Evidence tables for SPF, DKIM, DMARC, BIMI, Reporter sender evidence, drift events, and Defender findings.
   - Customer-readable next actions, not only charts.

4. Defender before enterprise bloat
   - Start with local typo and homoglyph generation plus DNS/MX evidence.
   - Store and alert only on evidence-backed findings.
   - Avoid ownership or malicious-intent claims without corroboration.

5. Concierge setup as a product feature
   - DNS inventory, DMARC RUA setup, sender review, policy progression, and monthly review are part of the first paid offer.

6. Reality-check sales funnel
   - Reuse the Checkmate-style buyer moment: grade the domain, explain spoofing exposure, and ask the user to claim the audit.
   - Keep proof-email dispatch separate from the main app until the controls in `docs/CHECKMATE_INTEGRATION.md` are in place.

## 90-Day Roadmap

### Days 1-15: Competitive GA Hardening

- Keep `make qa` green.
- Finish browser QA, manual accessibility, load checks, backup/restore drill, rollback drill, Lettermint live test, and DMARC mailbox test.
- DMARC enforcement journey model is implemented and exposed in authenticated trust reports, with reporter sender evidence wired into the readiness calculation when available.
- Sales-audit grading, "claim this audit" CTA, claim-sent UX, and durable claim persistence are implemented locally.
- Create Icelandic sales assets: landing copy, audit report template, DMARC enforcement checklist, and demo script.

### Days 16-30: Audit Funnel

- Launch "Free Email Domain Trust Audit for Icelandic Companies."
- Run 30 audits manually or semi-manually.
- Convert 10 companies into monitored pilots.
- Use the audit format: current policy, sender evidence, SPF/DKIM alignment, BIMI readiness, DNS drift risk, Defender lookalike scan.

### Days 31-60: Paid Pilots

- Convert 5 pilots to paid.
- Ship monthly trust report workflow.
- Add Defender MVP: typo/homoglyph generation, bounded DNS/MX checks, severity scoring, acknowledge/ignore states.
- Add visible DMARC journey status for each protected domain.

### Days 61-90: GA Launch

- Reach 10 paying Icelandic customers.
- Publish approved or anonymized local case studies.
- Package three plans:
  - Starter: 3 protected domains, daily scans, drift alerts, monthly trust report.
  - Growth: 10 domains, Reporter, sender inventory, weekly report, concierge DMARC setup.
  - Business: 25 domains, teams, API/webhooks, quarterly executive review.

## Post-GA Expansion

Choose based on customer pull:

- Managed MTA-STS and TLS-RPT.
- Managed BIMI and DKIM.
- Reputation monitoring and blacklist signals.
- MSP multi-tenant packaging.
- DNS provider integrations.
- SSO and procurement/security package.
