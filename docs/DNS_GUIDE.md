# DNS Guide

## SPF

Publish one SPF TXT record at the root domain. Avoid multiple SPF records, `+all`, `?all`, deprecated `ptr`, and more than 10 DNS-querying mechanisms.

Example:

```txt
example.com. TXT "v=spf1 include:_spf.example.net -all"
```

## DKIM

Publish DKIM public keys under selector names used by your email provider.

Example:

```txt
selector1._domainkey.example.com. TXT "v=DKIM1; k=rsa; p=PUBLICKEY"
```

Saved-domain settings can track custom selectors when provider defaults are not enough.

## DMARC

Publish DMARC at `_dmarc.example.com`. Start with monitoring if needed, then move to enforcement after reports confirm legitimate senders are aligned.

Monitoring:

```txt
_dmarc.example.com. TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"
```

Enforcement:

```txt
_dmarc.example.com. TXT "v=DMARC1; p=reject; adkim=s; aspf=s; rua=mailto:dmarc@example.com"
```

## BIMI

BIMI requires a valid SVG logo URI and usually a VMC or CMC certificate URI. Broad mailbox support also requires enforced DMARC.

Example:

```txt
default._bimi.example.com. TXT "v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem"
```

## DMARC Aggregate Reporting

Reporter ingestion expects mailbox forwarding or direct delivery of DMARC aggregate attachments. Supported payloads are plain XML, gzip-compressed XML, and zip archives containing XML files.

When Meil provisions a reporting target, it uses a domain-specific address under the configured ingestion mailbox domain:

```txt
reports+example=com@ingest.meil.example
```

Publish that address in the DMARC `rua` tag, or forward aggregate report attachments from your existing reporting mailbox to that address:

```txt
_dmarc.example.com. TXT "v=DMARC1; p=none; rua=mailto:reports+example=com@ingest.meil.example"
```
