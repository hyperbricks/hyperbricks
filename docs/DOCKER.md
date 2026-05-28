# Docker Deploy

The repository includes a Docker-based deploy environment for running the
HyperBricks Deploy API, SSH upload access, plugin builds, and deployed module
processes.

Use it for local deploy testing or as a simple remote runtime host.

## Quick Start

From the repository root:

```bash
docker compose -f docker/docker-compose.yml up --build
```

Default exposed ports:

| Port | Purpose |
| ---: | --- |
| `9090` | Deploy API |
| `2222` | SSH upload access |
| `8080-8100` | Deployed module runtime ports |

## Required Setup

Set a shared deploy secret in `docker/docker-compose.yml` or through your
environment:

```yaml
environment:
  HB_DEPLOY_SECRET: "change-me"
```

Add an SSH public key:

```bash
ssh-keygen -y -f ~/.ssh/hyperbricks_deploy > docker/ssh/authorized_keys
```

The container uses this key for the `deploy` user.

## SSH Config

Example local SSH config:

```sshconfig
Host hyperbricks-docker-remote
  HostName localhost
  Port 2222
  User deploy
  IdentityFile ~/.ssh/hyperbricks_deploy
  IdentitiesOnly yes
```

Use `hyperbricks-docker-remote` as the deploy target host in your deploy client
configuration.

## Volumes

The compose setup mounts:

```text
docker/data/deploy        -> /opt/hyperbricks/deploy
docker/deploy.hyperbricks.yaml -> /opt/hyperbricks/deploy.hyperbricks.yaml
docker/ssh/authorized_keys -> /etc/hyperbricks/authorized_keys
```

The deploy folder is persistent on the host, so archives, extracted builds, and
logs survive container restarts.

## Plugin Builds

The container creates `/opt/hyperbricks/bin/plugins` at startup. Build plugins
inside the container as the `deploy` user:

```bash
docker exec -u deploy -w /opt/hyperbricks <container_name> \
  hyperbricks plugin build example@1.0.0
```

Plugin source is copied into `/opt/hyperbricks/plugins`.

## Tailwind CLI

The Docker image can install a Tailwind CLI binary based on the container
architecture. The version is controlled by the `TAILWIND_VERSION` build arg in
`docker/docker-compose.yml`.

Set `TAILWIND_VERSION` to an empty value if you want to skip Tailwind
installation.

## Runtime Ports

Deployed modules are assigned ports from the configured deploy port range. The
compose file exposes `8080-8100` by default.

If a deployed module is not reachable from the host:

- verify the runtime port in the Deploy UI or API status
- ensure that port is exposed by Docker
- ensure the module process binds inside the container

## Troubleshooting

Remove a stale SSH host key:

```bash
ssh-keygen -R "[localhost]:2222"
```

Check SSH access:

```bash
ssh hyperbricks-docker-remote
```

Check the Deploy API:

```bash
curl -i http://localhost:9090/
```

Rebuild the image after runtime or dependency changes:

```bash
docker compose -f docker/docker-compose.yml build --no-cache
```

See [Deploy](DEPLOY.md) for the deploy workflow and authentication model.
