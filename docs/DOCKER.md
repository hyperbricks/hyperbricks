# Docker Deploy Setup (Hyperbricks)

This repo includes a Docker-based Alpine deploy setup that mirrors
`docs/alpine-hyperbricks-compile.md`. It builds Hyperbricks as the `deploy`
user, supports plugin compilation, and exposes SSH + the Deploy API.

## Quick start
From repo root:
```
docker compose -f docker/docker-compose.yml up --build
```

Deploy API:
- http://localhost:9090/

SSH:
- Host: localhost
- Port: 2222

## Required configuration
1) Set the HMAC secret (same value for client + server):
- `docker/docker-compose.yml` -> `HB_DEPLOY_SECRET`

2) Add your public key for SSH:
- Put your public key line in `docker/ssh/authorized_keys`

Example:
```
ssh-keygen -y -f ~/.ssh/proxmox_lxc > docker/ssh/authorized_keys
```

3) SSH config for convenience:
```
Host hyperbricks-docker-remote
  HostName localhost
  Port 2222
  User deploy
  IdentityFile ~/.ssh/proxmox_lxc
  IdentitiesOnly yes
```

Then use `hyperbricks-docker-remote` as the deploy target host in
`deploy.hyperbricks`.

## Ports
- 9090: Deploy API
- 2222: SSH for push
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
- SSH host key changed:
  - `ssh-keygen -R "[localhost]:2222"`
- SSH permission denied (publickey):
  - Confirm `docker/ssh/authorized_keys` contains your public key.
  - Ensure your SSH config includes `Port 2222` and `IdentityFile`.
- Module not reachable from host:
  - Ensure the module binds to `0.0.0.0` inside the container.
  - Verify the runtime port shown in the Deploy UI matches the exposed range.
- Plugin build fails on Go version:
  - The container uses Go 1.23.4. Rebuild if you were on 1.23.2.

