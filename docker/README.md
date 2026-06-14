# Hyperbricks Alpine Deploy (Docker)

This setup runs the HyperBricks Deploy API inside an Alpine-based container with
HTTP HRA upload, optional OpenRC service wiring, and plugin build support.

## Build + run
From repo root:
```
docker compose -f docker/docker-compose.yml up --build
```

## Verify
- `curl http://localhost:9090/` should return the deploy UI HTML.

## Deploy API
The API is exposed on `http://localhost:9090` and uses HMAC-signed HTTP upload.
Set `HB_DEPLOY_SECRET` in `docker/docker-compose.yml` (or override via env).

## Plugin builds (manual)
Build plugins inside the running container as the `deploy` user:
```
docker exec -u deploy -w /opt/hyperbricks <container_name> hyperbricks plugin build tailwindcss@1.0.1
docker exec -u deploy -w /opt/hyperbricks <container_name> hyperbricks plugin build esbuild@1.0.1
docker exec -u deploy -w /opt/hyperbricks <container_name> hyperbricks plugin build markdown@1.0.0
```

## Build args
- `TAILWIND_VERSION`: set to empty to skip installing the Tailwind CLI.

Example:
```
docker build -f docker/Dockerfile --build-arg TAILWIND_VERSION="" .
```

## Notes
- Hyperbricks and plugins are built as the `deploy` user for plugin compatibility.
- Plugin build steps require network access to fetch the plugin index and sources.
- The deploy root is persisted at `docker/data/deploy`.
- The entrypoint runs the Deploy API directly by default; set `HB_USE_OPENRC=1` to start it through OpenRC.
