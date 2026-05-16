# Postfix

This directory contains an example pipe-transport configuration for direct DMARC aggregate report delivery.

Local development still uses Mailpit for login and alert email, but Reporter deployments can point a domain-specific RUA address at Postfix and pipe raw XML payloads into:

```sh
meil ingest -
```

For production mailboxes, prefer forwarding only DMARC aggregate attachments or extracting attachments before piping. The ingest command accepts raw XML from stdin and file paths for XML, gzip, and zip report payloads.
