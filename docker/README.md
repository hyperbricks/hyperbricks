# Hyperbricks Alpine Deploy (Docker)

This setup mirrors `docs/alpine-hyperbricks-compile.md` inside a container and
includes SSH, OpenRC service wiring, and plugin builds.

## Build + run
From repo root:
```
docker compose -f docker/docker-compose.yml up --build
```

## SSH access
Put your public key in `docker/ssh/authorized_keys` (single-line key).
The entrypoint copies it into `/opt/hyperbricks/.ssh/authorized_keys` so
ownership and permissions are valid inside the container.
The container exposes SSH on port `2222` by default.

Example:
```
ssh -p 2222 deploy@localhost
```

## Verify
- `curl http://localhost:9090/` should return the deploy UI HTML.
- `ssh -p 2222 deploy@localhost` should succeed once keys are installed.

## Deploy API
The API is exposed on `http://localhost:9090` and uses HMAC.
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
- The entrypoint starts the OpenRC service by default; set `HB_USE_OPENRC=0` to run the deploy API directly.
