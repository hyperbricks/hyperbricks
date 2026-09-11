# HyperBricks Docker Deploy Host

This container runs the Deploy API, accepts HRA uploads, and starts deployed module processes. Go and native build tools are included for plugin builds.

From the repository root:

```bash
export HB_DEPLOY_SECRET="$(openssl rand -hex 32)"
docker compose -f docker/docker-compose.yml up --build -d
```

Keep the secret and use it for the deploy client too. It is required by Compose. The default image builds the current checkout, including local source changes. The dashboard is at http://localhost:9090/; runtime ports 8080–8100 bind locally.

See [Docker Deploy Host](../docs/DOCKER.md) for release selection, alternate ports, upload/activation, plugin builds, persistence and troubleshooting.

| Variable | Default | Purpose |
| --- | --- | --- |
| `HB_BUILD_SOURCE` | `checkout` | Build local source, or use `release`. |
| `HB_VERSION` | `v1.2.4-beta` | Published version installed in release mode. |
| `HB_DEPLOY_SECRET` | Required | Shared client/server HMAC secret. |
| `HB_BIND_ADDRESS` | `127.0.0.1` | Host address for published ports. |
| `HB_API_PORT` | `9090` | Host Deploy API port. |
| `HB_RUNTIME_PORTS` | `8080-8100` | Host range mapped to container ports 8080–8100. |
| `TAILWIND_VERSION` | Empty | Optional standalone Tailwind CLI version; the download is checked against that release's published SHA-256 manifest. |

Archives and extracted runtimes persist in `docker/data/deploy`; compiled global plugins persist in the Compose `plugin-builds` volume. Rebuild plugins when the runtime/toolchain changes. Native esbuild requires no external plugin.

The image runs the daemon directly as `deploy`; OpenRC is no longer used.

## Verify The Deploy Chain

Build the checkout image, then run the isolated test (Python 3 and recent Docker Compose with `!override` support are required):

```bash
docker build -f docker/Dockerfile -t hyperbricks-deploy-check:local .
python3 docker/tests/smoke.py
```

The test uses its own Compose project, temporary deploy directory, plugin volume, and loopback ports 29090 and 28080–28100. It checks HMAC rejection, native plugin build/load, HRA upload/activation, a page with native esbuild CSS, and persistence after container recreation. It restarts the persisted module explicitly and removes only its own containers and volume afterward. Change `--api-port` and `--runtime-port` if those ports are occupied. It does not test the optional Tailwind download, release installation, external TLS proxy or production load.
