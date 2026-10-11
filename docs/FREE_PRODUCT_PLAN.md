# Dmark: free email security by Kastro

Decision date: 2026-10-10; public-app revision: 2026-10-11. This is the current product direction and overrides the commercial SaaS and hosted SPF plans in this repository.

Dmark is a free, provider-neutral email-authentication tool and a source of qualified enquiries for Kastro. There is no paid Dmark subscription, premium security tier, hosted SPF flattening, customer DNS publishing or promise of managed email delivery. Revenue belongs to optional, separately quoted Kastro implementation work. A user must be able to get useful findings and setup guidance without contacting Kastro.

## Why this direction

Cloudflare made DMARC Management generally available free for its customers on 2026-06-16, including report analysis and SPF auditing. Microsoft has strengthened Outlook consumer sender-authentication requirements; that is a delivery requirement, not evidence of an equivalent free hosted DMARC product. Dmark should help people understand and implement these standards across providers, and point them toward useful existing reporting services.

Sources checked 2026-10-10:

- https://blog.cloudflare.com/dmarc-management-ga/
- https://developers.cloudflare.com/dmarc-management/
- https://support.microsoft.com/en-us/outlook/fix-ndr-error-550-5-7-515-in-outlook-com
- https://bimigroup.org/faqs-for-senders-esps/

## Useful free product

1. Public domain check without signup: SPF and DMARC observations, common DKIM discovery, optional exact DKIM selector lookup, and default BIMI publication lookup.
2. Add domains and work through a per-domain sender/setup checklist. The homepage is the working app, with domain navigation, records and a setup checklist. Guest domains, selectors and manual progress persist in a bounded first-party cookie. Registration is optional: email-link accounts can explicitly import and save the guest workspace across devices. There is no scheduled monitoring or automatic validation.
3. Explain findings and next steps in natural Icelandic and English. A DNS rating is not proof of safe mail delivery, message signing or alignment.
4. Help users inventory staff mail, invoicing, CRM, newsletters, support tools, website forms and devices before changing policies.
5. Guide users to verify sender alignment and report evidence before tightening DMARC. Preserve existing enforcement while investigating. Aligned SPF **or** aligned DKIM can satisfy DMARC.
6. Offer optional Kastro review/setup assistance through an ordinary contact link. No report is gated behind sales contact and no domain is automatically submitted as a lead.

### Standards and evidence boundaries

| Standard | What Dmark helps with | What a DNS lookup does not establish |
| --- | --- | --- |
| SPF | Published record, duplicates/lookup issues supplied by the existing scanner, verified sender cleanup | Actual message authorization or alignment; automatically safe removal of a sender |
| DKIM | Common selectors and one user-supplied selector; provider activation instructions | Signature correctness, full key validation, rotation or alignment of actual messages |
| DMARC | Published policy and a report-informed enforcement journey | All legitimate senders have been discovered; all receivers honor policy; every phishing attempt is prevented |
| BIMI | Default BIMI record publication and readiness guidance | SVG/certificate compliance, logo ownership or guaranteed inbox display |

BIMI is optional branding, after authentication. Missing BIMI must not lower the security grade. Some mailbox providers require VMC/CMC certificates with external costs; a free Dmark tool must not imply that those costs disappear.

## Current implementation boundary

The live public frontend uses Kastro Lookup for SPF/DMARC/common-selector evidence, not this Go service's authenticated dashboard. `dmark-web` owns the app-first workspace, guest cookie persistence and bounded supplementary DNS publication lookups. New `/api/account/*` Go routes provide passwordless browser sessions and user-scoped PostgreSQL workspace saving, independent of legacy commercial entitlements. Deploy migration 00009 and the account API alongside the frontend; configure `FRONTEND_BASE_URL`, email delivery and matching frontend proxy origins. This release does not add hosted DMARC report ingestion, a reporting mailbox, scheduled scans or a public automation API.

The Go repository retains substantial legacy account, worker, reporting, billing and SPF publication code. This document changes product scope; it does **not** certify that old runtime endpoints have been disabled. Preserve existing migrations, tests and data until an evidence-based operational retirement is complete. Do not present the legacy feature audit as proof of the new free product's release readiness.

## Backend work to retain and prioritize

- Reuse and improve parsers, scan evidence, domain normalization, explicit selectors, recommendations and API abuse controls.
- Correct recommendations that imply mandatory SPF flattening, blanket `p=none`, automatic safe sender removal or safe enforcement from DNS alone.
- Free saved-domain accounts reuse existing email-link auth and add user-isolated workspace storage with one free capability set. Keep any resource limits as explicit fair-use limits, never upgrade prompts.
- Ownership verification is required before enabling domain-specific report ingestion, notifications or other automation. Public lookup and a private list of public domain names need no claim of ownership.
- Add bounded scheduled rechecks and change summaries only after job, email delivery, consent, abuse and cost controls are proven. Never advertise these as available before release verification.
- Use existing reporting providers initially. Consider safe manual aggregate-report import only if users show demand; no raw message or forensic-report collection by default.
- Keep retention, account deletion, export, backups and tenant isolation for any retained hosted data.

## Retire commercial and SPF publication paths safely

1. Inventory real subscriptions, organizations, active SPF routes and all domains depending on published `spf.dmark.is` records. Existing canary evidence is not evidence that no customer depends on them.
2. Freeze new paid-plan/checkout and hosted-route onboarding before promoting a free account experience. Existing runtime entitlement/billing routes need explicit implementation changes and tests; this documentation PR does not remove them.
3. For active SPF routes, prepare provider-managed replacements, verify sender coverage and DNS propagation, and confirm the customer has removed dependence on Dmark's hosted records.
4. Retain last-known-good DNS records and any necessary resync until migration is verified. Do not delete Bunny records, switch off workers or revoke publishing credentials while senders depend on them.
5. Disable route mutation/scheduling, then retire publication credentials and integrations once the dependency inventory is empty. Keep an auditable migration record without publishing credentials.
6. Disable billing/checkout endpoints and remove upgrade wording and commercial entitlements from future exposed account/API flows. Preserve historical transactions and billing records according to the actual retention requirements.
7. Archive obsolete modules and docs only after callers, jobs, operational dependencies and data compatibility have been checked. Avoid destructive schema changes in the positioning release.

## Stages and release acceptance

**This revision:** free positioning, no pricing grid, real anonymous scans, app-first domain workspace, guest cookie storage, optional passwordless account import/saving, exact DKIM/BIMI publication checks, official setup references, optional Kastro assistance and an authoritative scope reset in both repositories.

**Next, if needed:** retire legacy paid/hosted-SPF runtime paths using the inventory above; audit the backend recommendation and DNS-guidance builders; validate real account adoption and move sessions/rate limits to shared stores before multi-replica deployment.

**Later, demand-led:** verified free account monitoring, report imports and source summaries. Enterprise teams, premium retention, paid API plans, hosted SPF infrastructure and broad brand-risk surveillance are outside the current product scope.

Before merging a functional release, check both languages, mobile and keyboard flows, invalid/duplicate domains, unavailable browser storage, domain deletion, DNS failures vs absent records, revoked DKIM keys, duplicate TXT records, selector validation and absent BIMI without a grade penalty. Keep scan domains and selectors out of analytics, do not fetch user-supplied SVG/certificate URLs, and enforce public endpoint resource limits. A browser checklist is user-declared progress, never a security certificate.

Success means useful repeat visits, completed setup work and qualified voluntary Kastro enquiries. Measure aggregate actions without storing checked domain names in marketing analytics. Do not optimize for fear-based scores or compulsory lead forms.
