# Docker Deploy Host

The Docker setup runs the HyperBricks Deploy API and the module processes it starts from uploaded HRA archives. Use it to test deployments locally or run a remote deploy host. It includes Go and build tools so native plugins can be built for the same Linux runtime.

## Build And Start

From the repository root, set a secret shared with your deploy client:

```bash
export HB_DEPLOY_SECRET="$(openssl rand -hex 32)"
docker compose -f docker/docker-compose.yml up --build -d
```

Keep that secret for subsequent commands and container recreation. Compose refuses to start when it is missing or empty. Configure the same secret in the client as described in [Deploy](DEPLOY.md#authentication).

By default, the image builds HyperBricks from **the current checkout**, including local source changes. It does not download `@latest`. For reproducible builds, use a known commit and record any local changes.

To build a published release instead:

```bash
export HB_BUILD_SOURCE=release
export HB_VERSION=v2.0.0-beta
docker compose -f docker/docker-compose.yml up --build -d
```

The selected tag must already be published and accessible to Go. Use `HB_BUILD_SOURCE=checkout` for unpublished work. The two build modes use the same Go toolchain specified in the Dockerfile.

## Addresses And Ports

Ports bind to `127.0.0.1` on the host by default:

| Host port | Container port | Purpose |
| --- | --- | --- |
| `9090` | `9090` | Deploy API and dashboard |
| `8080–8100` | `8080–8100` | Deployed module processes |

Open [the deploy dashboard](http://localhost:9090/). Its availability alone does not prove that archive upload or module startup succeeds.

If local ports are occupied, choose a different API port and an equally sized runtime port range before starting:

```bash
export HB_API_PORT=19090
export HB_RUNTIME_PORTS=18080-18100
docker compose -f docker/docker-compose.yml up -d
```

The Deploy API reports container ports. In this example, runtime port `8080` is reached through host port `18080`. The deploy configuration sets the first runtime port; the published range does not limit the daemon's allocation.

For access from other machines, explicitly choose `HB_BIND_ADDRESS`, such as a host LAN address. Use HTTPS through a reverse proxy or a private network for remote deployment traffic. HMAC authenticates requests; it does not encrypt them.

## Upload And Activate A Module

Build an HRA on the client and configure a deploy target pointing to this host. The [deploy workflow](DEPLOY.md#push-flow) uploads to `POST /deploy/v1/modules/{module}/releases`, activates the archive, and starts its module process. Use the dashboard or authenticated API status to find its port.

Check the deployed page and its `/static/` assets. Native esbuild is part of HyperBricks; it requires no esbuild plugin installation. Include the source resources and required dependencies in runtime archives that build assets.

## Persistent Files

| Storage | Container path | Contents |
| --- | --- | --- |
| `docker/data/deploy` bind mount | `/opt/hyperbricks/deploy` | Archives, build index, extracted runtimes and module logs |
| Compose `plugin-builds` volume | `/opt/hyperbricks/bin/plugins` | Compiled global plugin artifacts |
| Read-only config bind mount | `/opt/hyperbricks/deploy.hyperbricks.yaml` | Deploy host configuration |

These survive container recreation. `docker compose down` keeps the named volume; `down -v` deletes it. Back up the deploy directory separately. Persisted builds are not a promise that every module process resumes automatically: check status after host startup and restart the selected module when needed.

Plugin source is copied from the checkout's `plugins/` into the image. Source changes require an image rebuild. Go caches are not mounted persistently.

## Build Plugins

Build native plugins inside the container, using the same runtime and toolchain. For the default checkout build:

```bash
docker compose -f docker/docker-compose.yml exec \
  -u deploy -w /opt/hyperbricks \
  -e HYPERBRICKS_LOCAL_PATH=/opt/hyperbricks-source \
  hyperbricks-deploy hyperbricks plugin build example@1.0.0
```

Replace `example@1.0.0` with an existing plugin source name and version. For a release build, omit the `HYPERBRICKS_LOCAL_PATH` override so the plugin resolves the released dependency. Enable the resulting artifact in the module package; building it does not enable it automatically.

Rebuild native plugins after changing the runtime or toolchain. Persistence does not make an old binary compatible. Do not copy macOS native plugin binaries into the Linux host. Windows cannot build or load native Go plugins directly; on a Windows machine, build and run them inside the Linux container. See [plugin platform support](PLUGINS.md#platform-support) for the platform limitation and [Plugins](PLUGINS.md) for module-local plugin names, build locations and compatibility requirements.

## Optional Tailwind CLI

Compose skips Tailwind installation by default. Projects using ordinary CSS through native esbuild do not need it. For projects requiring the standalone Tailwind CLI, select an explicit version:

```bash
export TAILWIND_VERSION=4.1.10
docker compose -f docker/docker-compose.yml up --build -d
```

Set `TAILWIND_VERSION` to an empty value to skip it again. The Dockerfile selects a Linux download for the container architecture and verifies it against the SHA-256 manifest published with that release. This does not install or enable a HyperBricks Tailwind plugin.

## Inspect And Stop

```bash
docker compose -f docker/docker-compose.yml logs --tail=100 hyperbricks-deploy
docker compose -f docker/docker-compose.yml exec hyperbricks-deploy hyperbricks version
docker compose -f docker/docker-compose.yml down
```

The entrypoint prepares writable directories and runs `deploy-daemon` in the foreground as `deploy`. Docker manages the container lifecycle; OpenRC is not used by this image. The old OpenRC service files are retained only as historical files, not as a supported Docker startup option.

When a module is unreachable, check its activation result and logs, assigned container port, host port mapping, and whether its process is still running. Rebuild the image after runtime source changes and rebuild affected plugins.
