# Kastro RecordMaster Coolify Deployment

The production compose file deploys an `app` service running `meil serve` and a private Postgres `db` service with a persistent Docker volume. The binary also includes `meil worker` for River-backed scan and enrichment jobs; deploy it as a separate worker process when the target environment needs background scan/enrichment jobs.

## Hostnames

Use `recordmaster.kastro.is` as the canonical public domain. Keep `meil.kastro.is` as the legacy hostname during the transition.

Coolify settings:

1. Create a new application from the GitHub repository `AndriGitDev/meil`.
2. Select Docker Compose deployment.
3. Use `infra/coolify/docker-compose.yml` as the compose file.
4. Set the application domain to `https://recordmaster.kastro.is`.
5. Add `https://meil.kastro.is` as a legacy hostname or proxy alias while existing links are still active.
6. Set the exposed port to `8080` if Coolify asks for it.
7. Set the health check path to `/healthz` if Coolify asks for it.

The compose file assumes Coolify runs Docker Compose with the repository root as `--project-directory`, which is Coolify's default for Git deployments.

The Dockerfile builds from committed generated files. Run `templ generate` before committing template changes; CI checks that generated templ output is current.

Required environment variables:

```env
POSTGRES_PASSWORD=replace-with-a-long-url-safe-password
```

`HTTP_ADDR`, `PUBLIC_BASE_URL`, `ASSETS_DIR`, and `DATABASE_URL` are set by `infra/coolify/docker-compose.yml`.

Use an alphanumeric or URL-encoded Postgres password because the compose file embeds it in `DATABASE_URL`.

After the first deploy, run Postgres migrations from the Coolify terminal for the app container:

```sh
/meil migrate postgres up
```

Run the same command after deploys that add new Postgres migrations.

Optional environment variables:

```env
LETTERMINT_TOKEN=
LETTERMINT_FROM=Kastro RecordMaster <login@kastro.is>
BILLING_WEBHOOK_SECRET=
```

`LETTERMINT_TOKEN` and `LETTERMINT_FROM` are required for passwordless login emails. Without them, the public checker still works, but login returns "Email login is not configured for this environment."

DNS:

Point `recordmaster.kastro.is` and `meil.kastro.is` at the Coolify proxy according to the target server's normal Coolify DNS setup. Coolify should terminate TLS and proxy to the container on port `8080`. The app redirects requests for `meil.kastro.is` to `recordmaster.kastro.is`.
