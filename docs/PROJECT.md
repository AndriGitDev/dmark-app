# dmark

dmark is an open-source email security tool. Its purpose is to make email-authentication findings understandable and independently reviewable.

## Available in the web app

- On-demand SPF and DMARC checks, common DKIM-selector discovery, explicit selector checks and BIMI publication checks.
- A transparent five-check DNS readiness score, with explanations of incomplete evidence and limitations.
- Domain checklists saved in the browser, with optional email-link accounts for saving across devices.
- English and Icelandic setup guidance and an on-demand SPF snapshot generator.

Users publish DNS changes through their own provider. Scores describe observed DNS configuration; they do not certify message authentication or guarantee delivery. The web app does not provide continuous monitoring or receive DMARC reports.

## Source and data flow

The [web app](https://github.com/AndriGitDev/dmark-web) and [Go backend](https://github.com/AndriGitDev/dmark-app) form dmark. A public web check currently uses Kastro Lookup for SPF/DMARC evidence and queries public DNS for supplementary records. The backend is needed for account saving. See the README for deployment configuration, storage behavior and operational limitations.

Resource limits protect the service and apply consistently. Contributions should improve accuracy, clear guidance, privacy, accessibility and reproducible operation.

## Backend compatibility

Organization APIs use one shared capability set, with a 250-domain resource limit. Existing database migrations and generated schema models remain compatible with deployed databases. Historical columns do not determine capabilities. The public web app uses its own bounded workspace and account endpoints.

Report ingestion, background jobs and DNS publication modules require explicit operator configuration and ownership controls; their presence in the backend does not make them available in the public web app. Before stopping DNS publication workers, inventory dependent records, migrate senders to reviewed replacements and verify propagation. Never remove records or publishing credentials while mail delivery depends on them.
