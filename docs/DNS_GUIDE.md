# RecordMaster DNS and Domain Trust Guide

DNS records affect brand trust because mail providers, security tools, and customers use public domain evidence to decide whether a message or site should be trusted. RecordMaster currently checks public DNS and email-auth posture; planned Defender work extends that evidence into broader brand-name risk monitoring.

## SPF

Publish one SPF TXT record at the root domain. Avoid multiple SPF records, `+all`, `?all`, deprecated `ptr`, and more than 10 DNS-querying mechanisms.

```txt
example.com. TXT "v=spf1 include:_spf.example.net -all"
```

## DKIM

Publish DKIM public keys under selector names used by your email provider. Protected-domain settings can track custom selectors when provider defaults are not enough.

```txt
selector1._domainkey.example.com. TXT "v=DKIM1; k=rsa; p=PUBLICKEY"
```

## DMARC

Publish DMARC at `_dmarc.example.com`. Start with monitoring if needed, then move to enforcement after reports confirm legitimate senders are aligned.

```txt
_dmarc.example.com. TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"
_dmarc.example.com. TXT "v=DMARC1; p=reject; adkim=s; aspf=s; rua=mailto:dmarc@example.com"
```

## BIMI

BIMI requires a valid SVG logo URI and usually a VMC or CMC certificate URI. Broad mailbox support also requires enforced DMARC.

```txt
default._bimi.example.com. TXT "v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem"
```

## MX, NS, and DNSSEC

MX records show whether the domain receives mail. NS records show the delegated DNS operators. DNSSEC support is planned for later posture checks and should be treated as roadmap work until implemented.

## DMARC Reporting Setup

Reporter ingestion expects mailbox forwarding or direct delivery of DMARC aggregate attachments. Supported payloads are plain XML, gzip-compressed XML, and zip archives containing XML files.

When RecordMaster provisions a reporting target, it uses a domain-specific address under the configured ingestion mailbox domain:

```txt
reports+example=com@ingest.recordmaster.example
```

Publish that address in the DMARC `rua` tag, or forward aggregate report attachments from your existing reporting mailbox to that address:

```txt
_dmarc.example.com. TXT "v=DMARC1; p=none; rua=mailto:reports+example=com@ingest.recordmaster.example"
```

## Common Brand-Risk Mistakes

- Missing DMARC or leaving `p=none` indefinitely.
- SPF records that are missing, duplicated, syntactically broken, or too permissive.
- DKIM selectors removed before a sender is retired.
- Unknown sender infrastructure appearing in DMARC aggregate reports.
- Lookalike domains with active MX or web services. Detection for this is planned Defender work, not a current feed.
