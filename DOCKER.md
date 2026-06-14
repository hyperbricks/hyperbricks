# Docker Deploy Setup (Hyperbricks)

This repo includes a Docker-based Alpine deploy setup. It builds HyperBricks as
the `deploy` user, supports plugin compilation, and exposes the HTTP Deploy API.

## Quick start
From repo root:
```
docker compose -f docker/docker-compose.yml up --build
```

Deploy API:
- http://localhost:9090/

## Required configuration
1) Set the HMAC secret (same value for client + server):
- `docker/docker-compose.yml` -> `HB_DEPLOY_SECRET`

2) Use the API URL as the deploy target in `deploy.hyperbricks.yaml`:

```yaml
client:
  target: docker
  targets:
    docker:
      api: http://localhost:9090
```

## Ports
- 9090: Deploy API
- 8080-8100: runtime ports for deployed modules

## Plugin builds
The container creates `/opt/hyperbricks/bin/plugins` on startup. You can build
plugins via the Deploy UI or manually inside the container as `deploy`:
```
docker exec -u deploy -w /opt/hyperbricks <container_name> \
  hyperbricks plugin build tailwindcss@1.0.1
```

Plugin source is available at `/opt/hyperbricks/plugins` (copied from this repo).

## Tailwind CLI
The image downloads a Tailwind CLI binary based on the container architecture
(x86_64 or arm64). For arm64 Alpine, the image includes `gcompat` so glibc-linked
binaries work. To skip Tailwind installation, set `TAILWIND_VERSION` to empty in
`docker/docker-compose.yml`.

## Troubleshooting
- Module not reachable from host:
  - Ensure the module binds to `0.0.0.0` inside the container.
  - Verify the runtime port shown in the Deploy UI matches the exposed range.
- Plugin build fails on Go version:
  - The container uses Go 1.23.4. Rebuild if you were on 1.23.2.
