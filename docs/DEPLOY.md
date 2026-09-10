# Deploy

HyperBricks can build a module into a deploy archive, push it to a remote deploy host, activate it through the Deploy API, and run the selected build from the remote `deploy/` folder.

Deploy has two roles:

- Local build hub: builds archives, keeps local build history, and can push to a remote target.
- Remote runtime hub: accepts builds, activates one build per module, starts or stops module processes, and serves logs/status.

## Build A Deploy Archive

Build a `.hra` archive:

```bash
hyperbricks build --hra -m demo
```

Build a `.zip` archive:

```bash
hyperbricks build --zip -m demo
```

Common flags:

| Flag | Purpose |
| --- | --- |
| `--out <dir>` | Output directory, default `deploy` |
| `--force` | Build even when no source changes are detected |
| `--replace` | Replace the current build |
| `--replace <build_id>` | Replace a specific build |
| `--push` | Build and push to a deploy target |
| `--target <name>` | Select a deploy target for `--push` |

Build output is stored under:

```text
deploy/<module>/
  <module>-<moduleversion>-<build_id>.hra
  hyperbricks.versions.json
```

The versions index stores the current build pointer, build metadata, archive path, and source hash.

## Run From Deploy

Start the current build from the deploy folder:

```bash
hyperbricks start --deploy -m demo
```

Start a specific build:

```bash
hyperbricks start --deploy -m demo --build build-id
```

Use a custom deploy directory:

```bash
hyperbricks start --deploy -m demo --deploy-dir deploy
```

At startup, HyperBricks extracts the selected archive into:

```text
deploy/<module>/runtime/<build_id>/
```

The runtime reads `package.hyperbricks.yaml` from that extracted build.

## Remote Deploy API

For Composer-managed deploys, start the remote daemon directly:

```bash
hyperbricks deploy-daemon
```

If `deploy.hyperbricks.yaml` does not exist yet, the command writes a minimal remote config and prints the next steps. Composer shows the exact env var name for each deploy target, using this pattern:

```text
HB_DEPLOY_SECRET_<NORMALIZED_MODULE>_<NORMALIZED_KEY_ID>
```

Example:

```bash
export HB_DEPLOY_SECRET_OWNER_EXAMPLE_TEST_TEST_PROD="hbd_..."
hyperbricks deploy-daemon
```

Composer can then upload and activate an already-built HRA through:

```text
POST /deploy/v1/modules/{module}/releases
```

That endpoint writes the archive to the normal deploy root and reuses the same activation path as the dashboard. Uploaded builds therefore appear in the remote dashboard build list, status, logs, restart, stop, rollback, and production controls.

Legacy start command:

Create a remote deploy config:

```bash
hyperbricks start --deploy-init-config remote
```

Start the remote Deploy API:

```bash
hyperbricks start --deploy-remote
```

The remote hub:

- accepts uploaded archives
- activates builds
- updates the remote `hyperbricks.versions.json`
- extracts the selected build
- starts, stops, restarts, and rolls back module processes
- exposes status and logs

Remote layout:

```text
deploy/
  <module>/
    incoming/
    archives/
    runtime/
      <build_id>/
    logs/
    hyperbricks.versions.json
```

## Local Dashboard

Create a local deploy config:

```bash
hyperbricks start --deploy-init-config local
```

Start the local build dashboard:

```bash
hyperbricks start --deploy-local
```

The local dashboard:

- scans local modules
- builds archives
- pushes builds to configured targets
- syncs remote status on demand
- keeps local and remote state separate

Local dashboard requests are not HMAC-signed. Keep the local dashboard bound to `127.0.0.1`.

## Deploy Config

Deploy commands read `deploy.hyperbricks.yaml` from the project root unless `HB_DEPLOY_CONFIG` points to another file.

Create a starter config with:

```bash
hyperbricks start --deploy-init-config local
hyperbricks start --deploy-init-config remote
```

Example local build and push config:

```yaml
deploy:
  hmac_secret:
    env: HB_DEPLOY_SECRET

  remote:
    api_enabled: true
    api_bind: 127.0.0.1
    api_port: 9090
    root: deploy
    port_start: 8080
    logs_enabled: true

  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy

  client:
    target: staging
    targets:
      staging:
        api: https://deploy.example.com
        key_id: staging
```

The deploy config uses the same generic YAML resolver model as other HyperBricks YAML config files. `hmac_secret.env` reads the value from the environment at load time.

## Push Flow

Build and push using the default target:

```bash
hyperbricks build --hra -m demo --push
```

Build and push to a named target:

```bash
hyperbricks build --hra -m demo --push --target staging
```

The push flow is:

1. Build archive locally.
2. Upload the archive to `POST /deploy/v1/modules/{module}/releases`.
3. The remote daemon validates, stores, and activates the archive.
4. Refresh local metadata from the remote status.

If upload succeeds but activation fails, the local build stays intact and the error is surfaced to the operator.

## Authentication

Remote Deploy API requests are signed with HMAC-SHA256.

The canonical string is:

```text
METHOD
PATH
SHA256(body)
timestamp
nonce
```

Headers:

- `X-HB-Timestamp`
- `X-HB-Nonce`
- `X-HB-Signature`

Composer-style keyed deploys also send:

- `X-HB-Key-ID`
- `X-HB-Build-ID`
- `X-HB-SHA256`

For keyed deploys, the canonical string appends the key id and build id:

```text
METHOD
PATH
SHA256(body)
timestamp
nonce
key_id
build_id
```

The server checks timestamp drift, nonce reuse, and signature validity.

Set the same secret on local and remote:

```bash
export HB_DEPLOY_SECRET="change-me"
```

## Environment Overrides

Deploy services support these environment variables:

| Variable | Purpose |
| --- | --- |
| `HB_DEPLOY_CONFIG` | Alternate deploy config path |
| `HB_DEPLOY_SECRET` | Shared HMAC secret |
| `HB_DEPLOY_SECRET_<MODULE>_<KEY_ID>` | Module/key scoped deploy secret for HTTP HRA uploads |
| `HB_DEPLOY_BIND` | Override remote API bind address |
| `HB_DEPLOY_PORT` | Override remote API port |
| `HB_DEPLOY_ROOT` | Override remote deploy root |
| `HB_DEPLOY_PORT_START` | First port used for deployed module processes |
| `HB_DEPLOY_LOGS` | Enable or disable remote logs |
| `HB_DEPLOY_BIN` | Binary path used to start module processes |

Module processes also receive runtime environment values such as module name, build ID, assigned port, deploy root, and production mode.

## Security

- Keep `HB_DEPLOY_SECRET` out of the repository.
- Keep the local dashboard on `127.0.0.1`.
- Bind the remote API to localhost or a private network unless it is behind trusted HTTPS infrastructure.
- Use HTTPS, VPN, firewall rules, or a reverse proxy for remote access.
- Keep clocks in sync; HMAC timestamps allow only limited drift.

HMAC provides request integrity and authentication. It does not provide confidentiality. Use HTTPS or a private network when secrets or archives cross an untrusted network.

## Systemd Example

```ini
[Unit]
Description=HyperBricks Deploy API
After=network.target

[Service]
User=deploy
WorkingDirectory=/opt/hyperbricks
Environment=HB_DEPLOY_SECRET=change-me
Environment=HB_DEPLOY_CONFIG=/opt/hyperbricks/deploy.hyperbricks.yaml
ExecStart=/usr/local/bin/hyperbricks start --deploy-remote
Restart=always

[Install]
WantedBy=multi-user.target
```

## OpenRC Example

```sh
#!/sbin/openrc-run

name="hyperbricks-deploy"
description="HyperBricks Deploy API"
command="/usr/local/bin/hyperbricks"
command_args="start --deploy-remote"
command_user="deploy:deploy"
directory="/opt/hyperbricks"
pidfile="/run/${name}.pid"
command_background="yes"

depend() {
  need net
}
```

Example `/etc/conf.d/hyperbricks-deploy`:

```sh
HB_DEPLOY_SECRET="change-me"
HB_DEPLOY_CONFIG="/opt/hyperbricks/deploy.hyperbricks.yaml"
```

## Static Export

Static rendering is separate from deploy archives:

```bash
hyperbricks static -m demo
hyperbricks static -m demo --zip --out ./exports/demo
```

Static rendering snapshots routes through the normal runtime HTTP path before writing files. Public `api_render` content is fetched during this step, so the build environment must be able to reach those upstream APIs.

See [HyperBricks CLI](HYPERBRICKS_CLI.md) for static flags.

## Rollbacks

The remote Deploy API can roll back a module to an earlier build. Manual rollback is also possible by changing the `current` build pointer in `deploy/<module>/hyperbricks.versions.json` and restarting the module.

## Metadata

Each module should define `hyperbricks.metadata.moduleversion` in `package.hyperbricks.yaml`. The build command reads module metadata and updates archive metadata when creating deploy artifacts.
