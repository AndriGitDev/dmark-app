# Brand Risk Research

This spike evaluates how RecordMaster Defender should detect domain impersonation beyond the current DNS and email-auth findings.

## Decision Matrix

| Approach | Accuracy | Cost | Abuse risk | Privacy impact | Operational burden | MVP suitability |
|---|---:|---:|---:|---:|---:|---|
| Generate typo variants locally | Medium | Low | Low | Low | Low | High |
| Generate homoglyph variants locally | Medium | Low | Medium | Low | Medium | High |
| DNS existence checks for candidates | Medium | Low | Medium | Low | Medium | High |
| MX checks on candidates | High for mail risk | Low | Medium | Low | Medium | High |
| HTTP/S active-site checks | Medium | Medium | Medium | Medium | Medium | Medium |
| Certificate Transparency logs | High | Low to medium | Low | Low | Medium | Medium |
| Passive DNS providers | High | Medium to high | Low | Medium | Medium | Later |
| Domain intelligence providers | High | High | Low | Medium | Medium | Later |
| RDAP / WHOIS checks | Medium | Low | Medium | Medium | High | Later |

## MVP Recommendation

Build the first Defender feed from local candidate generation plus bounded DNS and MX checks:

1. Generate typo and homoglyph candidates from protected brand names and root domains.
2. Normalize candidates with IDNA handling and dedupe them before lookup.
3. Check A/AAAA, NS, and MX existence with strict concurrency, timeout, and per-domain limits.
4. Create findings only when evidence exists, such as a candidate domain resolving or publishing MX.
5. Label mail-enabled lookalikes higher than parked or DNS-only candidates.

This fits the current product because it uses transparent evidence, has predictable cost, and does not require third-party data contracts.

## Do Not Build Yet

- Automated takedown workflows.
- Continuous HTTP scraping of candidate sites.
- Passive DNS enrichment.
- Domain-intelligence provider dependency.
- RDAP/WHOIS ownership conclusions as a primary signal.
- Executive-name monitoring until customer consent and privacy rules are explicit.

## Privacy and Abuse Risks

Candidate generation can create sensitive strings from company names, people, products, and domains. Store only terms the customer intentionally protects. Rate-limit all DNS probing. Avoid alerting on weak evidence, and always show why a finding exists. Do not imply ownership or malicious intent from a lookalike candidate without corroborating evidence.

## Output Contract

Every Defender finding should include:

- Finding type.
- Observed value.
- Severity.
- Evidence JSON.
- First and last seen timestamps.
- Recommended action.
- Status for acknowledge, monitoring, resolved, or ignored workflows.
