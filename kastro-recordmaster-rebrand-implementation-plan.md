# Kastro RecordMaster Rebrand and Product Vision Implementation Plan

**Repo:** `meil`
**New product name:** `Kastro RecordMaster`
**Recommended canonical hostname:** `recordmaster.kastro.is`
**Legacy hostname:** `meil.kastro.is`
**Prepared:** 2026-05-16

---

## 1. Executive Summary

The current project, `meil`, started as an email-auth monitoring tool for domain owners. The product currently has a strong foundation around SPF, DKIM, DMARC, BIMI, DNS posture checks, saved-domain monitoring, drift detection, and DMARC reporting.

The new vision is broader:

> **Kastro RecordMaster is brand-name risk defense for domain owners.**
>
> It monitors DNS records, email authentication, sender evidence, and spoofing risk so teams can catch trust failures before customers do.

The project should keep the repo name `meil` for now, but the public product should become **Kastro RecordMaster**. This avoids unnecessary technical churn while giving the product a stronger commercial and strategic identity.

The rebrand should not simply replace one name with another. It should reposition the product from:

```text
Email-auth monitoring
```

to:

```text
Brand-name risk defense for domains, DNS, email authentication, sender evidence, and spoofing risk
```

The strongest product direction is:

```text
Kastro RecordMaster
Defend your brand name from DNS drift, spoofing, and domain impersonation.
```

---

## 2. Core Naming Decision

### Product Name

Use:

```text
Kastro RecordMaster
```

Not just:

```text
RecordMaster
```

The Kastro prefix gives the product stronger ownership, better trust, and a clear relationship to the broader Kastro brand. It also helps distinguish the product from generic DNS tools.

### Category Name

Use:

```text
Brand-name risk defense
```

This is better than making “Brand Name Risk Defender” the formal product name. “Defender” is useful, but it works better as a module, plan, or capability.

Recommended hierarchy:

```text
Company: Kastro
Product: Kastro RecordMaster
Category: Brand-name risk defense
Modules:
  - RecordMaster Checker
  - RecordMaster Monitor
  - RecordMaster Reporter
  - RecordMaster Defender
```

This gives the product room to grow. It also allows “Defender” to become a paid tier or advanced module later.

---

## 3. Core Positioning

### Primary One-Liner

```text
Kastro RecordMaster helps teams defend their brand name by monitoring domains, DNS records, email authentication, sender evidence, and spoofing risk.
```

### Short Product Description

```text
Kastro RecordMaster is brand-name risk defense for domain owners. It monitors DNS records, SPF, DKIM, DMARC, BIMI, sender evidence, and spoofing indicators so teams can catch trust failures before customers do.
```

### Technical Description

```text
RecordMaster combines DNS posture checks, scheduled domain monitoring, drift detection, DMARC aggregate report ingestion, sender attribution, and brand-risk findings into one operator-friendly console.
```

### Buyer-Oriented Description

```text
Your domain is part of your brand. RecordMaster helps you protect it from silent DNS drift, broken email authentication, unknown senders, and impersonation risk.
```

### Homepage Hero Recommendation

Use this as the main hero:

```text
Defend your brand name from DNS drift, spoofing, and domain impersonation.
```

Use this as the subheading:

```text
RecordMaster watches the public records and sender signals that decide whether customers, mail providers, and security tools should trust your domain.
```

Use these CTAs:

```text
Check a domain
Protect my domains
```

---

## 4. What Changes from Meil to RecordMaster

The old product promise was strong:

```text
Know when your domain stops looking trustworthy.
```

Keep the spirit, but broaden it:

```text
Know when your brand name stops looking trustworthy.
```

The old scope was mainly:

```text
SPF, DKIM, DMARC, and BIMI monitoring
```

The new scope is:

```text
Domain trust, DNS records, email authentication, sender evidence, spoofing risk, and brand-name defense
```

This gives the product a stronger business reason to exist. It is no longer just a checker for mail authentication records. It becomes a system for protecting the public trust surface of a company’s domain and brand name.

---

## 5. Brand Architecture

Recommended structure:

```text
Kastro
└── RecordMaster
    ├── Checker
    │   Public domain, DNS, and email-auth posture checks
    ├── Monitor
    │   Protected domains, scheduled scans, history, and drift alerts
    ├── Reporter
    │   DMARC aggregate reports and sender attribution
    └── Defender
        Brand-name risk, lookalike domains, spoofing indicators, and evidence workflows
```

This lets the existing project evolve naturally without forcing all features into one generic dashboard.

### Module Definitions

#### RecordMaster Checker

The public top-of-funnel product.

It answers:

```text
What does the internet currently see for this domain?
```

Initial capabilities:

```text
- DNS posture snapshot
- SPF checks
- DKIM selector checks
- DMARC policy checks
- BIMI checks
- Domain trust score
- Recommended actions
```

#### RecordMaster Monitor

The continuous monitoring product.

It answers:

```text
What changed, and does the change matter?
```

Initial capabilities:

```text
- Protected domains
- Scheduled scans
- Drift detection
- Alert rules
- History
- Trust timeline
- Weekly posture summaries
```

#### RecordMaster Reporter

The sender evidence product.

It answers:

```text
Who is sending mail as this domain?
```

Initial capabilities:

```text
- DMARC aggregate report ingestion
- Sender inventory
- Unknown sender detection
- Alignment explanations
- Report health
- Sender attribution
```

#### RecordMaster Defender

The brand-name risk product.

It answers:

```text
Where could someone impersonate this brand?
```

Potential capabilities:

```text
- Protected brand names
- Protected root domains
- Brand watch terms
- Lookalike-domain monitoring
- Homoglyph checks
- Mail-enabled spoofing domains
- Evidence exports
- Takedown support workflow
```

---

## 6. Naming Map

| Old | New |
|---|---|
| Meil | Kastro RecordMaster |
| Meil Checker | RecordMaster Checker |
| Meil Reporter | RecordMaster Reporter |
| Email-auth monitoring | Domain trust monitoring |
| Email-auth drift monitoring | Brand-name risk defense |
| Saved domains | Protected domains |
| Domain checks | Record checks |
| Drift timeline | Trust timeline |
| Health score | Domain trust score |
| Domain owner | Brand/domain owner |
| Unknown sender | Suspicious sender |
| Reporter foundations | Sender evidence foundations |

Keep code-level names temporarily unless they are visible to users.

---

## 7. Product Pillars

### 7.1 Record Truth

RecordMaster should establish what is actually published in public DNS.

It answers:

```text
What does public DNS say about this domain right now?
```

Capabilities:

```text
- DNS posture snapshot
- SPF checks
- DKIM checks
- DMARC checks
- BIMI checks
- MX visibility
- NS visibility
- TXT visibility
- DNSSEC later
- Registrar and nameserver inventory later
```

### 7.2 Drift Defense

RecordMaster should explain what changed and whether the change creates risk.

It answers:

```text
What changed, and does it matter?
```

Capabilities:

```text
- Scheduled scans
- Before/after diffs
- Drift classification
- Severity levels
- Alert notifications
- Acknowledge and mute workflows
- Incident timeline
- Weekly summaries
```

### 7.3 Sender Evidence

RecordMaster should help domain owners understand who sends mail as them.

It answers:

```text
Who is sending mail as this domain?
```

Capabilities:

```text
- DMARC aggregate report ingestion
- Sender inventory
- Unknown sender detection
- SPF alignment analysis
- DKIM alignment analysis
- Sender attribution
- Provider mapping
```

### 7.4 Brand-Name Risk

RecordMaster should connect DNS and email-auth posture to brand impersonation risk.

It answers:

```text
Where could someone impersonate this brand?
```

Capabilities:

```text
- Protected brand names
- Protected domains
- Watch terms
- Similar-domain checks
- Lookalike-domain findings
- Mail-enabled impersonation candidates
- Risk scoring
- Evidence export
```

### 7.5 Operator Workflow

RecordMaster should not only show raw records. It should tell operators what to do next.

It answers:

```text
What should I do now?
```

Capabilities:

```text
- Prioritized findings
- Provider-neutral remediation guidance
- Evidence cards
- Weekly reports
- Alert routing
- Webhooks later
- Team access later
```

---

## 8. Visual Brand Direction

### Relationship to Kastro

RecordMaster should look like part of Kastro, not like a separate startup.

The product should inherit:

```text
- Kastro star motif
- Kastro yellow
- Kastro red
- Founder-led practical tone
- Security-first positioning
- Icelandic / European resilience angle
```

The product should keep:

```text
- Dark operator-console feel
- DNS record grid visuals
- Trust score cards
- Timeline / drift visuals
- Technical clarity
```

### Color Direction

Suggested visual system:

```text
Primary background:
  Deep near-black or blue-black

Primary accent:
  Kastro red for CTAs and high-risk moments

Secondary accent:
  Kastro yellow for star, highlights, warnings, and navigation

System colors:
  Green for healthy
  Amber for watch
  Red for risk
  Gray for missing or unknown
```

The current teal can remain for “healthy/pass” states, but it should stop being the dominant brand identity.

### Logo Lockup

Use:

```text
[ Kastro star ] Kastro RecordMaster
```

Compact nav version:

```text
[ star ] RecordMaster
```

Favicon or app icon ideas:

```text
R*
✶R
```

Avoid making “RM” the primary symbol unless it looks distinctive. The Kastro star gives the product more personality.

---

## 9. Repo Strategy

The repo should remain named:

```text
meil
```

for now.

Do not rename the repo immediately. The public product can become Kastro RecordMaster while the internal project remains `meil` during the transition.

### Keep Unchanged Initially

```text
- GitHub repo name: meil
- Go module path
- Binary name: meil
- Existing database table names
- Existing migration names
- Existing CLI command
- Existing internal package names
- Existing deployment service names
```

### Change Immediately

```text
- README title and intro
- Public app title
- Homepage copy
- Meta title
- Meta description
- Open Graph metadata
- Email sender display name
- Public docs headings
- Landing page navigation
- User-facing terminology
```

### Change Later

```text
- Repo name: recordmaster
- Binary name: recordmaster
- CLI alias: recordmaster scan
- Package namespace cleanup
- Database naming cleanup, only if necessary
```

The first rebrand should be a messaging and UI rebrand, not a deep technical rename.

---

## 10. Central Brand Constants

Add a central product identity package.

Possible path:

```text
internal/brand/brand.go
```

Example:

```go
package brand

const (
    CompanyName     = "Kastro"
    ProductName     = "RecordMaster"
    FullProductName = "Kastro RecordMaster"
    LegacyProduct   = "Meil"

    ProductTagline = "Brand-name risk defense for domains, DNS, and email authentication."

    PrimaryDomain = "recordmaster.kastro.is"
    LegacyDomain  = "meil.kastro.is"
)
```

Add module names:

```go
const (
    ModuleChecker  = "RecordMaster Checker"
    ModuleMonitor  = "RecordMaster Monitor"
    ModuleReporter = "RecordMaster Reporter"
    ModuleDefender = "RecordMaster Defender"
)
```

Use these constants in templates, email copy, page titles, and docs generation where applicable.

---

## 11. Landing Page Redesign

The public landing page should make the broader product vision obvious.

Recommended sections:

```text
1. Hero
2. Public checker form
3. What RecordMaster watches
4. Trust timeline / drift example
5. Brand-name risk defense
6. Reporter / sender attribution
7. Kastro-managed option
8. CTA
```

### 11.1 Hero

```text
Kastro RecordMaster

Defend your brand name from DNS drift, spoofing, and domain impersonation.

RecordMaster monitors the public records and sender signals that decide whether customers, mail providers, and security tools should trust your domain.
```

Primary CTA:

```text
Check a domain
```

Secondary CTA:

```text
Protect my domains
```

### 11.2 Checker Form

Keep the checker visible and simple.

Placeholder:

```text
yourdomain.com
```

Button:

```text
Check domain
```

Supporting steps:

```text
01 DNS posture snapshot
02 Drift and risk detection
03 Alert-ready history
```

### 11.3 Feature Cards

Use broader feature cards than the current email-auth-only cards.

```text
DNS Records
Know what your domain publishes.

Email Authentication
Watch SPF, DKIM, DMARC, and BIMI.

Drift Detection
Catch record changes before they become incidents.

Sender Evidence
Use DMARC reports to identify who sends as your domain.

Brand-Name Risk
Spot spoofing and impersonation signals.

Operator Alerts
Get actionable before/after context.
```

### 11.4 Trust Report Preview

Rename:

```text
Static report preview
```

to:

```text
Trust report preview
```

Example fields:

```text
Domain trust score: 92
DNS posture: Healthy
Email authentication: Watch
Brand-name risk: Low
Sender evidence: Collecting
```

Timeline copy:

```text
DMARC policy relaxed seven days ago. RecordMaster keeps the change visible so it does not disappear into DNS history.
```

### 11.5 Foundation Section

Rename:

```text
Checker and Reporter foundations
```

to:

```text
From DNS truth to brand-name defense
```

Copy:

```text
Start with public DNS truth. Add protected domains, scheduled drift monitoring, DMARC reporting, sender attribution, and brand-name risk defense as your needs grow.
```

---

## 12. Information Architecture

### Public URLs

Recommended canonical hostname:

```text
recordmaster.kastro.is
```

Suggested routes:

```text
/
/check
/login
/app
/app/domains
/app/records
/app/alerts
/app/reporter
/app/defender
```

### Existing Hostname

Keep:

```text
meil.kastro.is
```

as a legacy hostname or redirect.

### Public Navigation

```text
Checker
Monitor
Reporter
Defender
Pricing
Docs
```

### Authenticated Navigation

```text
Overview
Protected Domains
Records
Alerts
Reporter
Brand Risk
Settings
```

### Route Naming

Keep `/app/domains` initially. Do not rename the route immediately.

Visually rename it to:

```text
Protected Domains
```

Later, add:

```text
/app/brands
/app/risks
/app/defender
```

---

## 13. Documentation Updates

### README Intro

Replace the current README opening with:

```markdown
# Kastro RecordMaster

Kastro RecordMaster is brand-name risk defense for domain owners. It monitors DNS records, SPF, DKIM, DMARC, BIMI, sender evidence, and spoofing indicators so teams can catch trust failures before customers do.

The free Checker provides public DNS and email-auth posture checks. Monitor adds protected domains, scheduled scans, drift alerts, and history. Reporter ingests DMARC aggregate reports and models who is sending mail as your domain. Defender expands the platform into brand-name and lookalike-domain risk monitoring.

The repository is still named `meil` during the transition from the original email-auth monitoring product.
```

Keep the existing local development commands.

### Roadmap Intro

Replace the roadmap opening with:

```markdown
# Kastro RecordMaster Product Roadmap

RecordMaster is Kastro’s brand-name risk defense product for domain owners.

It starts with public DNS, SPF, DKIM, DMARC, and BIMI posture checks. It then grows into protected-domain monitoring, drift alerts, DMARC aggregate report ingestion, sender attribution, and brand-name risk defense for spoofing and impersonation risks.
```

### DNS Guide

Rename to:

```text
RecordMaster DNS and Domain Trust Guide
```

Suggested sections:

```text
- Why DNS records affect brand trust
- SPF
- DKIM
- DMARC
- BIMI
- MX
- NS
- DNSSEC later
- DMARC reporting setup
- Common brand-risk mistakes
```

### Threat Model

Add threats:

```text
- Lookalike domain impersonation
- Homoglyph domains
- Mail-enabled spoofing domains
- Weak DMARC policy abuse
- Unknown sender infrastructure
- DNS drift causing spoofing exposure
- Brand-risk alert abuse
- False positives in impersonation detection
```

### API Docs

Use these public terms:

```text
- Record checks
- Protected domains
- Findings
- Alerts
- Reporter reports
- Sender inventory
- Brand risk findings
```

Avoid exposing `meil` in public endpoint examples unless the path or command is still actually named `meil`.

---

## 14. Pull Request Plan

### PR 1: Product Constants and Copy Foundation

Goal:

```text
Create a single source of truth for product names and taglines.
```

Changes:

```text
- Add internal/brand package.
- Replace hardcoded “Meil” in templates with brand constants.
- Replace page title with “Kastro RecordMaster”.
- Update meta title and meta description.
- Update README opening.
- Keep binary and repo name unchanged.
```

Acceptance criteria:

```text
- No visible “Meil” on public pages except legacy notes.
- Tests pass.
- make qa-code passes.
```

### PR 2: Landing Page Rebrand

Goal:

```text
Update public site to the new Kastro RecordMaster positioning.
```

Changes:

```text
- New hero copy.
- New product description.
- Rename “email-auth drift monitoring”.
- Add brand-name risk language.
- Update feature cards.
- Add Brand Risk teaser section.
- Replace teal-first accents with Kastro accents.
- Add Kastro star / product lockup.
```

Acceptance criteria:

```text
- Homepage communicates domains, DNS, email auth, monitoring, spoofing, and brand risk.
- Public checker remains easy to access.
- Mobile layout works.
- Long DNS records render safely.
```

### PR 3: App Navigation and Terminology

Goal:

```text
Make authenticated app language match the new product.
```

Changes:

```text
- Rename “Saved Domains” UI to “Protected Domains”.
- Rename “Domain checks” to “Record checks” where appropriate.
- Rename “Drift timeline” to “Trust timeline” or “Record timeline”.
- Add placeholder nav item for “Brand Risk”.
- Add empty-state copy for broader vision.
```

Example empty state:

```text
Protect your first domain

RecordMaster will monitor DNS records, email-auth posture, and trust drift for this domain. Later, Reporter and Brand Risk modules can help identify unknown senders and spoofing indicators.
```

Acceptance criteria:

```text
- User-facing terminology is consistent.
- Routes do not need to change yet.
- Existing saved-domain workflow still works.
```

### PR 4: Documentation Rebrand

Goal:

```text
Update docs to describe the new vision without overpromising current functionality.
```

Changes:

```text
- README.md
- docs/ROADMAP.md
- docs/DNS_GUIDE.md
- docs/API.md
- docs/SECURITY.md
- docs/THREAT_MODEL.md
- docs/OPERATIONS.md
- infra/coolify/README.md
```

Important wording pattern:

```text
Current:
RecordMaster checks SPF, DKIM, DMARC, and BIMI.

Planned:
RecordMaster will expand into sender attribution and brand-name risk defense.
```

Acceptance criteria:

```text
- Docs separate current functionality from roadmap.
- Deployment docs mention both hostnames.
- README has a clear transition note.
```

### PR 5: Hostname and Metadata Migration

Goal:

```text
Make recordmaster.kastro.is the canonical product hostname.
```

Changes:

```text
- Add PUBLIC_BASE_URL=https://recordmaster.kastro.is.
- Update login magic-link URLs.
- Update canonical URL metadata.
- Update Open Graph URLs.
- Update Coolify environment.
- Add redirect support for meil.kastro.is if app controls host redirects.
```

Acceptance criteria:

```text
- Login links point to the new hostname.
- Existing links to meil.kastro.is still work.
- No mixed canonical metadata.
```

### PR 6: Brand-Risk Model Scaffolding

Goal:

```text
Prepare the database and app structure for the Defender vision.
```

Potential tables:

```text
protected_brands
brand_watch_terms
brand_domains
brand_risk_findings
brand_risk_observations
```

Minimal model:

```text
protected_brands
- id
- user_id or organization_id
- display_name
- primary_domain
- created_at
- updated_at

brand_watch_terms
- id
- protected_brand_id
- term
- kind
- created_at

brand_risk_findings
- id
- protected_brand_id
- finding_type
- observed_value
- severity
- status
- evidence_json
- first_seen_at
- last_seen_at
```

Suggested watch term kinds:

```text
- company_name
- product_name
- domain_root
- executive_name
```

Suggested finding types:

```text
- lookalike_domain
- homoglyph_domain
- typosquat_domain
- mail_enabled_lookalike
- web_active_lookalike
- dmarc_missing
- dmarc_policy_relaxed
- spf_permissive
- dkim_selector_removed
- bimi_missing
- unknown_sender
```

Acceptance criteria:

```text
- No user-facing claims that unsupported detection exists.
- Data model supports future Defender module.
- Migration tests pass.
```

### PR 7: Brand Risk MVP

Goal:

```text
Ship the first Defender feature that makes the new positioning real.
```

Initial MVP:

```text
Brand Risk page for each protected domain.
```

Use signals already available from DNS and email-auth checks:

```text
- DMARC missing
- DMARC p=none
- SPF missing
- SPF broken
- SPF too permissive
- SPF DNS lookup limit exceeded
- DKIM selector missing
- BIMI missing or invalid
- MX exists but email auth is incomplete
- Records changed recently
- Unknown senders from Reporter if available
```

Risk categories:

```text
- Domain trust
- Email spoofing
- Sender identity
- Brand indicators
- Monitoring coverage
```

Acceptance criteria:

```text
- Every risk finding has evidence.
- Every finding has a recommended action.
- The page is useful without third-party feeds.
```

### PR 8: Lookalike-Domain Research Spike

Goal:

```text
Decide how Defender will detect domain impersonation.
```

Explore:

```text
- Generating typo variants locally
- Generating homoglyph variants locally
- Checking DNS existence for candidates
- Checking MX on candidates
- Checking HTTP/S active sites
- Certificate transparency logs
- Passive DNS providers
- Domain intelligence providers
- RDAP / WHOIS limits
- Rate limits
- Abuse risks
- Privacy risks
```

Output:

```text
docs/BRAND_RISK_RESEARCH.md
```

Decision matrix:

```text
- Approach
- Accuracy
- Cost
- Abuse risk
- Privacy impact
- Operational burden
- MVP suitability
```

Acceptance criteria:

```text
- Clear MVP recommendation.
- Clear “do not build yet” list.
- Privacy and abuse risks documented.
```

---

## 15. Feature Roadmap for the New Vision

### Stage 1: RecordMaster Checker

Purpose:

```text
Public top-of-funnel domain trust check.
```

Scope:

```text
- DNS posture
- SPF
- DKIM
- DMARC
- BIMI
- Trust score
- Recommended actions
- Save/protect this domain CTA
```

Marketing label:

```text
Free domain trust check
```

### Stage 2: RecordMaster Monitor

Purpose:

```text
Continuous monitoring for protected domains.
```

Scope:

```text
- Protected domains
- Scheduled scans
- Drift detection
- Alert rules
- Email notifications
- History
- Trust timeline
- Weekly summary
```

Marketing label:

```text
Always-on DNS and email-auth monitoring
```

### Stage 3: RecordMaster Reporter

Purpose:

```text
DMARC evidence and sender attribution.
```

Scope:

```text
- DMARC aggregate ingestion
- Sender inventory
- Unknown sender detection
- Alignment analysis
- Report health
```

Marketing label:

```text
Know who is sending as your domain
```

### Stage 4: RecordMaster Defender

Purpose:

```text
Brand-name risk defense.
```

Scope:

```text
- Protected brand names
- Lookalike-domain checks
- Spoofing indicators
- Suspicious MX-enabled domains
- Evidence cards
- Takedown support workflow
```

Marketing label:

```text
Spot impersonation before customers do
```

### Stage 5: Kastro Managed RecordMaster

Purpose:

```text
Managed service wrapper for Icelandic SMBs.
```

Scope:

```text
- Kastro setup
- Kastro alert review
- Monthly summary
- DNS remediation support
- Registrar/provider coordination
- Escalation help
```

Marketing label:

```text
Managed brand-name risk defense
```

---

## 16. Pricing and Packaging Direction

Start simple.

### Free: RecordMaster Checker

Includes:

```text
- Public domain check
- SPF, DKIM, DMARC, and BIMI posture
- Basic score
- Basic recommendations
```

### Monitor

For small teams that own important domains.

Includes:

```text
- Protected domains
- Scheduled scans
- Drift alerts
- History
- Weekly summary
```

### Reporter

For teams that need to know who sends mail as them.

Includes:

```text
- DMARC aggregate ingestion
- Sender inventory
- Unknown sender alerts
- Alignment analysis
```

### Defender

For brand-name and spoofing risk.

Includes:

```text
- Brand watch terms
- Lookalike-domain checks
- Spoofing risk findings
- Evidence export
- Takedown workflow support
```

### Managed by Kastro

For SMBs that want Kastro to operate the product for them.

Includes:

```text
- Setup
- Monitoring
- Monthly report
- DNS remediation support
- Escalation help
```

This may be the strongest commercial path: a product-led checker with a managed-service upsell.

---

## 17. UI Copy System

### Status Names

Use:

```text
Healthy
Watch
At Risk
Broken
Unknown
```

Avoid showing only raw technical states as primary UI labels.

### Severity Names

Use:

```text
Info
Low
Medium
High
Critical
```

### CTA Language

Use:

```text
Check domain
Protect this domain
Start monitoring
View trust timeline
Review brand risk
Add expected sender
Acknowledge finding
Export evidence
```

### Finding Copy Examples

#### DMARC Policy Is Relaxed

```text
Your domain is monitoring-only. Spoofed mail may still be accepted by receivers.
```

Recommended action:

```text
Move toward DMARC enforcement once legitimate senders are verified.
```

#### SPF Record Is Too Permissive

```text
Your SPF record includes a mechanism that may allow unauthorized senders.
```

Recommended action:

```text
Review authorized senders and remove broad or unsafe mechanisms.
```

#### DKIM Selector Disappeared

```text
A previously observed DKIM selector is no longer published.
```

Recommended action:

```text
Confirm whether the sending service was retired or whether the selector was removed by mistake.
```

#### Unknown Sender Observed

```text
DMARC reports show mail from a source that is not in your expected sender inventory.
```

Recommended action:

```text
Classify the sender as expected, ignored, or suspicious.
```

#### Lookalike Domain Is Mail-Enabled

```text
A similar domain exists and publishes mail records, making it more useful for impersonation.
```

Recommended action:

```text
Review the evidence and decide whether to monitor, investigate, or prepare an abuse report.
```

---

## 18. Deployment and Hostname Migration

### New Canonical Hostname

```text
recordmaster.kastro.is
```

### Legacy Hostname

```text
meil.kastro.is
```

### Migration Sequence

```text
1. Add DNS record for recordmaster.kastro.is.
2. Add hostname to Coolify or reverse proxy.
3. Add TLS certificate.
4. Serve the same app on both hostnames.
5. Set canonical metadata to recordmaster.kastro.is.
6. Update PUBLIC_BASE_URL.
7. Update magic-link URLs.
8. Update Open Graph metadata.
9. Smoke test both hostnames.
10. Add 301 redirect from meil.kastro.is to recordmaster.kastro.is.
11. Keep the legacy redirect for at least 6–12 months.
```

### Initial Redirect Strategy

For the first week:

```text
Serve both hostnames.
```

After QA:

```text
301 redirect meil.kastro.is -> recordmaster.kastro.is
```

Optional transition note:

```text
Meil is now Kastro RecordMaster.
```

Do not over-explain the rename on the homepage unless existing users need the context.

---

## 19. QA Plan

### Code QA

Use the existing Make targets:

```sh
make qa-code
make qa-services
make test
make lint
make build
```

### Visual QA

Check:

```text
- Homepage desktop
- Homepage mobile
- Checker form
- Result cards with long DNS records
- Auth pages
- Protected domains page
- Email templates
- Empty states
- Error states
```

### Copy QA

Search for legacy strings:

```sh
rg -n "Meil|meil|email-auth monitoring|stops looking trustworthy|Reporter foundations"
```

Allowed leftovers:

```text
- Repo transition note
- Legacy domain docs
- CLI compatibility note
- Historical migration names
- Existing internal names
```

### Functional QA

Test:

```text
- Public domain check
- Login magic link
- Save domain
- Recheck domain
- Delete domain
- Scheduled scan if implemented
- Alert email if implemented
```

### Deployment QA

Test:

```text
https://recordmaster.kastro.is
https://meil.kastro.is
```

Verify:

```text
- TLS works
- Canonical URL is correct
- Redirects work
- Login links work
- Open Graph preview is correct
- No broken assets
```

---

## 20. Suggested GitHub Issues

### Branding Issues

```text
1. Define RecordMaster brand constants
2. Replace public Meil copy with Kastro RecordMaster
3. Add Kastro logo/star lockup to product header
4. Update homepage hero and feature sections
5. Update metadata, Open Graph, and favicon
6. Update email templates and sender display name
```

### Documentation Issues

```text
7. Rewrite README for RecordMaster transition
8. Rewrite roadmap around brand-name risk defense
9. Update DNS guide as Domain Trust Guide
10. Update threat model with brand impersonation risks
11. Update operations docs for new hostname
```

### App UX Issues

```text
12. Rename Saved Domains to Protected Domains
13. Rename Drift Timeline to Trust Timeline
14. Add Brand Risk placeholder page
15. Add product module labels: Checker, Monitor, Reporter, Defender
16. Update empty/error/success states
```

### Deployment Issues

```text
17. Add recordmaster.kastro.is hostname
18. Update PUBLIC_BASE_URL and magic-link URLs
19. Add canonical metadata
20. Add legacy redirect from meil.kastro.is
```

### Product Foundation Issues

```text
21. Add protected brand data model
22. Add brand watch terms
23. Add brand risk finding model
24. Add initial brand-risk scoring from existing DNS/email-auth checks
25. Research lookalike-domain monitoring options
```

---

## 21. First PR Recommendation

Start with one small but high-impact PR.

Branch:

```text
brand/kastro-recordmaster
```

Title:

```text
Rebrand public product copy to Kastro RecordMaster
```

Scope:

```text
- Add brand constants.
- Update README intro.
- Update homepage title, hero, tagline, and metadata.
- Replace visible “Meil” with “Kastro RecordMaster”.
- Keep repo, binary, routes, and deployment unchanged.
```

Acceptance criteria:

```text
- Homepage says Kastro RecordMaster.
- Hero says brand-name risk defense.
- Checker still works.
- Existing routes still work.
- No internal renaming churn.
- make test passes.
- make build passes.
```

This gives the product the new identity quickly without breaking the project.

---

## 22. Do Not Do Yet

Avoid these in the first rebrand pass:

```text
- Do not rename the repo immediately.
- Do not rename the Go module immediately.
- Do not rename database tables immediately.
- Do not promise lookalike-domain monitoring until the MVP exists.
- Do not present RecordMaster Defender as fully shipped if it is still a roadmap item.
- Do not bury the public checker.
```

The public checker is still the strongest entry point and should remain central.

---

## 23. Recommended Final Positioning

Use this as the north star:

```text
Kastro RecordMaster is brand-name risk defense for domain owners.

It monitors DNS records, email authentication, sender evidence, and spoofing risk so teams can catch trust failures before customers do.
```

Use this as the homepage hero:

```text
Defend your brand name from DNS drift, spoofing, and domain impersonation.
```

Use this as the subheading:

```text
RecordMaster watches the public records and sender signals that decide whether the internet should trust your domain.
```

Use these CTAs:

```text
Check a domain
Protect my domains
```

This preserves what made Meil useful while giving the product a bigger and more commercial mission.

---

## 24. Quick Implementation Checklist

### Immediate

```text
[ ] Add brand constants.
[ ] Replace public Meil copy.
[ ] Update homepage hero.
[ ] Update README intro.
[ ] Update metadata.
[ ] Add Kastro RecordMaster logo lockup.
[ ] Rename Saved Domains in UI to Protected Domains.
```

### Next

```text
[ ] Update full docs set.
[ ] Add recordmaster.kastro.is hostname.
[ ] Update PUBLIC_BASE_URL.
[ ] Update magic-link URLs.
[ ] Add legacy redirect.
[ ] Add Brand Risk placeholder page.
```

### Later

```text
[ ] Add protected brand model.
[ ] Add brand watch terms.
[ ] Add brand risk findings.
[ ] Add Brand Risk MVP.
[ ] Research lookalike-domain monitoring.
[ ] Decide whether to rename repo or binary.
```
