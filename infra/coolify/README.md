# Coolify Deployment

P0 deploys a single `app` service running `meil serve`. The `worker` role exists in the binary but is intentionally not deployed until real River jobs exist.

## Temporary Domain

Use `meil.kastro.is` as the temporary public domain.

Coolify settings:

1. Create a new application from the GitHub repository `AndriGitDev/meil`.
2. Select Docker Compose deployment.
3. Use `infra/coolify/docker-compose.yml` as the compose file.
4. Set the application domain to `https://meil.kastro.is`.
5. Set the exposed port to `8080` if Coolify asks for it.
6. Set the health check path to `/healthz` if Coolify asks for it.

The compose file assumes Coolify runs Docker Compose with the repository root as `--project-directory`, which is Coolify's default for Git deployments.

The Dockerfile builds from committed generated files. Run `templ generate` before committing template changes; CI checks that generated templ output is current.

Required environment variables:

```env
HTTP_ADDR=:8080
PUBLIC_BASE_URL=https://meil.kastro.is
ASSETS_DIR=/assets
```

DNS:

Point `meil.kastro.is` at the Coolify proxy according to the target server's normal Coolify DNS setup. Coolify should terminate TLS and proxy to the container on port `8080`.
