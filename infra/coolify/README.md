# Dmark Coolify Deployment

The production compose file deploys the Dmark backend `app` service running `meil serve` and a private Postgres `db` service with a persistent Docker volume. The binary also includes `meil worker` for River-backed scan and enrichment jobs; deploy it as a separate worker process when the target environment needs background scan/enrichment jobs. Frontend and marketing deployment, including Node/Next.js build configuration, belong to the separate `dmark-web` repository.

## Hostnames

Use `dmark.kastro.is` as the current public domain until `dmark.is` is purchased. Keep `recordmaster.kastro.is` as the legacy hostname during the transition.

Coolify settings:

1. Create a new application from the GitHub repository `AndriGitDev/dmark-app`.
2. Select Docker Compose deployment.
3. Use `infra/coolify/docker-compose.yml` as the compose file.
4. Set the application domain to `https://dmark.kastro.is`.
5. Add `https://meil.kastro.is` as a legacy hostname or proxy alias while existing links are still active.
6. Set the exposed port to `8080` if Coolify asks for it.
7. Set the health check path to `/healthz` if Coolify asks for it.

The compose file assumes Coolify runs Docker Compose with the repository root as `--project-directory`, which is Coolify's default for Git deployments.

The Dockerfile builds the Go backend only. Generated files are limited to backend code generation such as sqlc.

Required environment variables:

```env
POSTGRES_PASSWORD=replace-with-a-long-url-safe-password
```

`HTTP_ADDR`, `PUBLIC_BASE_URL`, and `DATABASE_URL` are set by `infra/coolify/docker-compose.yml`.

Use an alphanumeric or URL-encoded Postgres password because the compose file embeds it in `DATABASE_URL`.

The app runs Postgres and River migrations automatically when `meil serve` starts. No manual terminal step is required after deploy.

Optional environment variables:

```env
LETTERMINT_TOKEN=
LETTERMINT_FROM=Dmark <login@dmark.kastro.is>
BILLING_WEBHOOK_SECRET=
FRONTEND_ORIGINS=https://dmark.is,https://www.dmark.is
```

`FRONTEND_ORIGINS` enables CORS for browser calls from `dmark-web`. It must contain exact origins, not paths. `LETTERMINT_TOKEN` and `LETTERMINT_FROM` are reserved for backend email flows. The browser login surface now belongs in `dmark-web`.

DNS:

Point `dmark.kastro.is` and `recordmaster.kastro.is` at the Coolify proxy according to the target server's normal Coolify DNS setup. Coolify should terminate TLS and proxy to the container on port `8080`. The app redirects requests for `recordmaster.kastro.is` to `dmark.kastro.is`.
