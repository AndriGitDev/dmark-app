# Kastro RecordMaster Coolify Deployment

P0 deploys a single `app` service running `meil serve`. The binary also includes `meil worker` for River-backed scan and enrichment jobs; deploy it as a separate worker process when the target environment provides Postgres migrations and service credentials.

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
HTTP_ADDR=:8080
PUBLIC_BASE_URL=https://recordmaster.kastro.is
ASSETS_DIR=/assets
```

DNS:

Point `recordmaster.kastro.is` and `meil.kastro.is` at the Coolify proxy according to the target server's normal Coolify DNS setup. Coolify should terminate TLS and proxy to the container on port `8080`. The app redirects requests for `meil.kastro.is` to `recordmaster.kastro.is`.
