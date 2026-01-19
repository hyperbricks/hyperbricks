# Deploy Guide (Local + Remote)

This guide covers building deployable archives, running the remote Deploy API,
using the local build dashboard, and pushing builds to a remote server.

## Goals
- Upload `.hra` archives to a remote host and activate them via a web API.
- Support multiple modules from the remote `deploy/` folder.
- Use HMAC authentication (no HTTPS requirement for test networks).
- Port assignment starts at 8080 when `server.port` is missing in the archive.
- Always run managed process mode to start/stop modules without shell scripts.
- Provide a local build dashboard to create builds, push them, and sync remote status.

## Scope Decisions
- Use `hyperbricks start --deploy-remote` to run the Deploy API daemon.
- Use `hyperbricks start --deploy-local` to run the local build dashboard.
- No reverse-proxy mapping (handled externally).
- No remote module registry service (derive state from deploy folders).
- Deploy configuration lives in `deploy.hyperbricks` at the project root.

## Roles and Responsibilities
Local (build hub):
- Scan `modules/` and list local modules.
- Build new versions and update the local versions index.
- Push a selected build to a chosen remote target.
- Sync remote status only after push or on demand.
- Does not manage runtime processes.

Remote (runtime hub):
- Accept builds, activate them, update the remote versions index.
- Start/stop/restart/rollback processes and provide logs.
- Act as the source of truth for remote build existence.
- Does not build or push.

## Remote Folder Layout
```
deploy/
  <module>/
    incoming/       # uploaded .hra files (staging)
    archives/       # immutable .hra files
    runtime/
      <build_id>/   # extracted archive
    hyperbricks.versions.json
```

## Activation Flow
1) Upload `.hra` to `deploy/<module>/incoming/`.
2) Call API: `POST /deploy/modules/<module>/activate` with `build_id`.
3) Controller:
   - Validates archive metadata/checksum.
   - Extracts to `deploy/<module>/runtime/<build_id>/`.
   - Updates `hyperbricks.versions.json` current pointer.
   - Chooses port (see Port Allocation).
   - Restarts the module service.

## Build
Create a deploy archive for a module:

```bash
hyperbricks build --hra -m <module>
```

Optional flags:
- `--zip` to create a `.zip` instead of `.hra`
- `--out <dir>` to change the deploy folder (default: `deploy`)
- `--force` to rebuild even when the source hash is unchanged
- `--replace[=<build_id>]` to replace the current build or a specific build ID

Build outputs:
- Archive: `deploy/<module>/<module>-<moduleversion>-<build_id>.hra|.zip`
- Index: `deploy/<module>/hyperbricks.versions.json`

The index includes:
- `current` build ID pointer
- `versions[]` entries with file path, build metadata, and `source_hash`

## Run From Deploy
Start the server using the current deploy build:

```bash
hyperbricks start --deploy -m <module>
```

Optional flags:
- `--deploy-dir <dir>` to point at a custom deploy folder
- `--build <build_id>` to start a specific build

Runtime extraction:
- Archives are extracted to `deploy/<module>/runtime/<build_id>/`
- The server reads `package.hyperbricks` from that runtime directory

If you need a fresh extraction, remove the runtime folder and start again:

```bash
rm -rf deploy/<module>/runtime/<build_id>
hyperbricks start --deploy -m <module>
```

## Port Allocation
- If `server.port` exists in the archive's `package.hyperbricks`, prefer it.
  If the port is already in use, auto-assign the next free port starting from
  that value.
- Otherwise assign the first free port starting at `deploy.remote.port_start` (default 8080).

## Remote Deploy API Endpoints (concept)
- `GET /deploy/modules`
- `GET /deploy/status`
- `GET /deploy/modules/<module>/status`
- `GET /deploy/modules/<module>/builds`
- `GET /deploy/modules/<module>/builds/<build_id>/status`
- `GET /deploy/modules/<module>/builds/<build_id>/logs?lines=200`
- `POST /deploy/modules/<module>/builds/<build_id>/production`
- `POST /deploy/modules/<module>/builds/<build_id>/delete`
- `POST /deploy/modules/<module>/activate`
- `POST /deploy/modules/<module>/rollback`
- `POST /deploy/modules/<module>/restart`
- `POST /deploy/modules/<module>/stop`
- `POST /deploy/admin/kill-all` (best-effort kill of hyperbricks processes, keeps deploy API port)

Notes:
- Deleting the last build removes the entire `deploy/<module>/` directory.

## Local Build API Endpoints (concept)
- `GET /local/status`
- `GET /local/modules`
- `GET /local/modules/<module>/status`
- `GET /local/modules/<module>/builds`
- `GET /local/modules/<module>/builds/<build_id>/status`
- `GET /local/modules/<module>/builds/<build_id>/logs` (optional)
- `POST /local/modules/<module>/build`
- `POST /local/modules/<module>/push/<build_id>`
- `POST /local/remote/sync` (or `?target=<name>`)

If no local server is used, the UI can call the CLI directly instead of these
endpoints.

## HMAC Authentication
Use HMAC-SHA256 on a canonical string:
```
METHOD + "\n" +
PATH + "\n" +
SHA256(body) + "\n" +
timestamp + "\n" +
nonce
```
Headers:
- `X-HB-Timestamp`
- `X-HB-Nonce`
- `X-HB-Signature`

Server checks:
- Timestamp window (for example +/- 60s).
- Nonce not reused.
- Signature matches.

## No-HTTPS Environments
HMAC provides integrity/authentication, not confidentiality.
For test networks:
- Bind to `127.0.0.1` and use SSH tunneling, or
- Restrict by IP allowlist.

## Deploy Configuration (Shared)
Create a root config file named `deploy.hyperbricks`:
```
deploy {
  hmac_secret = {{ENV:HB_DEPLOY_SECRET}}

  remote {
    api_enabled = true
    api_bind = 127.0.0.1
    # api_bind controls exposure:
    # - localhost/LAN: use SSH tunnel or LAN access
    # - WAN: bind to public IP and put HTTPS in front (reverse proxy)
    api_port = 9090
    root = deploy
    port_start = 8080
    logs_enabled = true
  }

  # local dashboard (build/push) settings
  local {
    bind = 127.0.0.1
    port = 9091
    modules_dir = modules
    build_root = deploy
  }

  # push targets used by --deploy-local
  client {
    target = prod
    targets {
      prod {
        host = 192.168.2.35
        user = deploy
        port = 22
        root = /opt/hyperbricks/deploy
        api = http://192.168.2.35:9090
        # For WAN use, point api to your public HTTPS endpoint instead of SSH tunnel.
        # Use SSH keys for push (recommended, no passwords).
      }
    }
  }
}
```

### Environment Overrides (optional)
- `HB_DEPLOY_SECRET`
- `HB_DEPLOY_BIND`
- `HB_DEPLOY_PORT`
- `HB_DEPLOY_ROOT`
- `HB_DEPLOY_PORT_START`
- `HB_DEPLOY_LOGS` (true/false)
- `HB_DEPLOY_BIN` (override binary path used to start module processes)

Notes:
- `HB_DEPLOY_BIND`, `HB_DEPLOY_PORT`, `HB_DEPLOY_ROOT`, `HB_DEPLOY_PORT_START`,
  `HB_DEPLOY_LOGS`, and `HB_DEPLOY_BIN` override `deploy.remote` when set.
- `HB_DEPLOY_SECRET` is used when `deploy.hmac_secret` is empty.
- `HB_DEPLOY_CONFIG` selects an alternate `deploy.hyperbricks` file for both
  local and remote.

## CLI / Service
Create a default deploy config:
```
hyperbricks start --deploy-init-config local
hyperbricks start --deploy-init-config remote
```

Start the deploy API daemon:
```
hyperbricks start --deploy-remote
```

Start the local build dashboard:
```
hyperbricks start --deploy-local
```

## Remote Process Management (Always On)
The deploy API starts and stops modules directly:
- Uses the same `hyperbricks` binary (or `HB_DEPLOY_BIN`).
- Writes a PID file to `deploy/<module>/hyperbricks.pid`.
- Writes logs to `deploy/<module>/logs/<build_id>.log` when logs are enabled.
- Uses `HB_DEPLOY_CONFIG` to pick a custom deploy config file.

## Per-build Production Flag
Each build can store a `production` override in `hyperbricks.versions.json`.
Set it via:
```
POST /deploy/modules/<module>/builds/<build_id>/production
{ "production": true }
```
Behavior:
- The deploy API passes `--production` when starting that build.
- The process receives `HB_DEPLOY_PRODUCTION=1` (or `0`) in its environment.

## Local Dashboard (Build/Push)
- Starts with `hyperbricks start --deploy-local`.
- Scans `deploy.local.modules_dir` for modules.
- Writes builds to `deploy.local.build_root`.
- Uses `deploy.client` targets for push and remote sync.
- Uses `deploy.hmac_secret` (or `HB_DEPLOY_SECRET`) to sign remote API calls.
- Local API requests are not HMAC-signed; keep it bound to `127.0.0.1`.
- Remote changes made outside the local dashboard will not show until the next
  manual sync, by design.

## UI Behavior (Local vs Remote)
- Local view: Build, Push, Sync Remote.
- Remote view: Start, Stop, Restart, Rollback, Logs, Delete.
- The build list layout stays identical; only the action column changes.

### Build List States (Local View)
Status is derived by comparing local builds to remote builds:
- Local only: never pushed.
- Pushed/Active: present on remote after last sync.
- Missing on remote: was pushed before, but not found on remote after sync.

No background polling; remote changes made outside the local dashboard
won't show until the next sync.

## Local vs Remote State
Two separate `hyperbricks.versions.json` files exist:
- Local: builds created on the developer machine.
- Remote: builds available/activated on the server.

They share the same schema but represent different inventories.
Build IDs must stay stable across push so lists stay aligned.

### Local Push Metadata
When a build is pushed, store local-only metadata such as:
- `pushed_at`
- `remote_target`
- optional `remote_build_id`

This keeps local history without coupling to remote storage.

Remote existence is validated by querying the remote Deploy API. Status refresh
only happens after a successful push/activate or when the user clicks Sync.

Do not auto-delete local builds when remote removal is detected;
explicitly surface the mismatch so the user can decide.

## Push Workflow
### Local Dashboard Flow
- Start `hyperbricks start --deploy-local`.
- Use the Build button to create a new version in `deploy.local.build_root`.
- Use Push to upload and activate on the selected target.
- Use Sync Remote to refresh remote status (no background polling).

### CLI Behavior (`--push`)
- `hyperbricks build -m <module>` builds only.
- `hyperbricks build -m <module> --push` builds, then prompts to push to the
  default target in `deploy.client.target`.
- `hyperbricks build -m <module> --push --target prod` builds, then pushes to a
  named target without extra selection.

### Push Flow
1) Build completes.
2) If `--push` is set, prompt: "Push this build to <target>?"
3) If yes:
   - Upload archive to:
     `<target.root>/<module>/incoming/<archive>`
   - Use `scp` (or `rsync -e ssh`).
   - If no SSH key is available, the SSH client prompts for a password.
4) After upload, call Deploy API:
   `POST /deploy/modules/<module>/activate` with `build_id`.

On success: update local metadata, then sync remote status.
On failure: keep the local build and show the error; do not alter local history.

### Remote Reaction (Deploy API)
On activation:
- Verify archive exists in `incoming/` (or `archives/`).
- Move to `archives/`.
- Update `hyperbricks.versions.json`.
- Extract to `runtime/<build_id>/`.
- Start/restart the module.

### Optional Modes
- "Push only": upload to `incoming/` without activation.
- "Push + activate": upload and immediately activate (default).

### Delete Behavior
- Remote delete: if the last build is removed, the entire `deploy/<module>/`
  directory is deleted.
- Local delete: if the last build is removed, the entire local build folder
  for that module is deleted.

## Security and Credentials
- SSH keys stored in `~/.ssh/config`.
- SSH password prompts are handled by the OS SSH client.
- HMAC secret provided via `HB_DEPLOY_SECRET`.
- Keep the local dashboard bound to `deploy.local.bind` (default `127.0.0.1`).
- Deploy API should bind to `deploy.remote.api_bind` on LAN or localhost (no `0.0.0.0`).
- Block inbound `9090` from WAN (public internet) at the firewall/router.
- Hyperbricks does not terminate HTTPS; assume reverse proxy/infra handles TLS.
- For `--push`, trigger activation via SSH tunnel or a remote `curl` to
  `http://localhost:9090` to keep the API local to the server.
- SSH tunnel example:
  `ssh -L 9090:localhost:9090 deploy@192.168.2.11` then call
  `http://localhost:9090/deploy/modules/<module>/activate`.

## Example Use Case (Web Developer)
Maya is a web developer with a staging server in the LAN. She wants to build
locally and push builds to the server with `--push`.

Server setup:
1) Install Hyperbricks and set up `deploy.hyperbricks`:
   - `hmac_secret = {{ENV:HB_DEPLOY_SECRET}}`
   - `remote.api_enabled = true`
   - `remote.api_bind = 127.0.0.1` (or LAN IP)
   - `remote.api_port = 9090`
   - `remote.root = /opt/hyperbricks/deploy`
2) Export `HB_DEPLOY_SECRET` on the server (same value as client).
3) Start the deploy API daemon so it listens on `remote.api_bind:remote.api_port`.
4) Ensure `/opt/hyperbricks/deploy` is writable by the deploy user.
5) Create the remote folder layout:
   - `/opt/hyperbricks/deploy/<module>/incoming`
   - `/opt/hyperbricks/deploy/<module>/archives`
   - `/opt/hyperbricks/deploy/<module>/runtime`
   - `/opt/hyperbricks/deploy/<module>/hyperbricks.versions.json`
6) Run the deploy API as a service (systemd/OpenRC) so it stays up across reboots.
7) Ensure the service has permission to start/stop module processes and write
   to the deploy root.
8) Store `HB_DEPLOY_SECRET` in the service environment (not in the repo).

Example systemd unit:
```
[Unit]
Description=Hyperbricks Deploy API
After=network.target

[Service]
User=deploy
WorkingDirectory=/opt/hyperbricks
Environment=HB_DEPLOY_SECRET=changeme
Environment=HB_DEPLOY_CONFIG=/opt/hyperbricks/deploy.hyperbricks
ExecStart=/usr/local/bin/hyperbricks start --deploy-remote
Restart=always

[Install]
WantedBy=multi-user.target
```

Example OpenRC (Alpine) service:
```
#!/sbin/openrc-run

name="hyperbricks-deploy"
description="Hyperbricks Deploy API"
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
```
HB_DEPLOY_SECRET="changeme"
HB_DEPLOY_CONFIG="/opt/hyperbricks/deploy.hyperbricks"
```

## OpenRC Setup (Beginner Friendly)
These steps configure the deploy API service on Alpine using OpenRC.

1) Install the Hyperbricks binary (adjust the source path if needed):
```
install -m 0755 /root/go/bin/hyperbricks /usr/local/bin/hyperbricks
```

2) Create the deploy root and give the deploy user access:
```
mkdir -p /opt/hyperbricks/deploy
chown -R deploy:deploy /opt/hyperbricks
```

3) Create the service config (`/etc/conf.d/hyperbricks-deploy`):
```
cat <<'EOF' > /etc/conf.d/hyperbricks-deploy
HB_DEPLOY_SECRET="your-secret-here"
HB_DEPLOY_CONFIG="/home/source/hyperbricks/deploy.hyperbricks"
EOF
```

4) Create the OpenRC service (`/etc/init.d/hyperbricks-deploy`):
```
cat <<'EOF' > /etc/init.d/hyperbricks-deploy
#!/sbin/openrc-run

name="hyperbricks-deploy"
description="Hyperbricks Deploy API"
command="/usr/local/bin/hyperbricks"
command_args="start --deploy-remote"
command_user="deploy:deploy"
directory="/home/source/hyperbricks"
pidfile="/run/${name}.pid"
command_background="yes"

depend() {
  need net
}
EOF
chmod +x /etc/init.d/hyperbricks-deploy
```

5) Enable and start the service:
```
rc-update add hyperbricks-deploy default
rc-service hyperbricks-deploy start
rc-service hyperbricks-deploy status
```

Optional check:
```
ss -lntp | grep 9090
```

## Alpine Linux Service Install (OpenRC)
This is a minimal install guide to run the deploy API on Alpine and ensure it
starts on reboot.

1) Create a deploy user and directories:
```
adduser -D -h /opt/hyperbricks deploy
mkdir -p /opt/hyperbricks/bin /opt/hyperbricks/deploy
chown -R deploy:deploy /opt/hyperbricks
```

2) Install the binary:
```
/opt/hyperbricks/bin/hyperbricks
```
Make it executable:
```
chmod +x /opt/hyperbricks/bin/hyperbricks
```

3) Optional environment config:
```
HB_DEPLOY_SECRET="your-hmac-secret"
HB_DEPLOY_BIND="127.0.0.1"
HB_DEPLOY_PORT="9090"
```
Note: OpenRC will source `/etc/conf.d/hyperbricks-deployd` automatically.

4) OpenRC service script:
```
#!/sbin/openrc-run

name="hyperbricks-deployd"
command="/opt/hyperbricks/bin/hyperbricks"
command_args="start --deploy-remote"
command_user="deploy:deploy"
pidfile="/run/${name}.pid"
directory="/opt/hyperbricks"
command_background="yes"

depend() {
  need net
}
```
Make it executable:
```
chmod +x /etc/init.d/hyperbricks-deployd
```

5) Enable and start:
```
rc-update add hyperbricks-deployd default
rc-service hyperbricks-deployd start
```

6) Verify:
```
rc-service hyperbricks-deployd status
```

If you want logging, use your preferred OpenRC log method or wrap the command
in a small shell script that redirects stdout/stderr to a log file.

## Notes (Current Flow)
- Keep `HB_DEPLOY_SECRET` consistent between client and server; mismatches cause 401s.
- Restart the deploy API service after editing `/etc/conf.d/hyperbricks-deploy`.
- Keep clocks in sync (HMAC timestamps allow ~60s drift).
- If upload succeeds but activation fails, archives can be stranded in `incoming/`.
- Keep the deploy API (`:9090`) LAN-only; use SSH tunnel or remote `curl` for activation.
- Remote changes made outside the local dashboard will not appear until Sync.

## SSH Key Setup (Mac -> Alpine)
Goal: allow passwordless SSH from a Mac to an Alpine server for `--push`.

On the Mac:
```
ssh-keygen -t ed25519 -C "hyperbricks-push" -f ~/.ssh/hyperbricks_push
```

On the Alpine server console (create a non-root user and enable SSH):
```
adduser deploy
apk add --no-cache openssh
rc-service sshd start
rc-update add sshd
```

Copy the public key from Mac to Alpine:
```
cat ~/.ssh/hyperbricks_push.pub | ssh deploy@192.168.2.35 'mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'
```

Test from the Mac:
```
ssh -i ~/.ssh/hyperbricks_push -o IdentitiesOnly=yes deploy@192.168.2.35
```

## Web Client (UI)
The deploy API includes a small signed web client for operators.
It is served directly by the deploy API at `/` (also `/deploy-ui`).

Notes:
- The UI signs requests in the browser using Web Crypto.
- The HMAC secret is typed into the UI and stays in-memory for the session.
- The local dashboard does not sign requests and should stay on localhost.
- Use it only on trusted networks or behind SSH/VPN.

### Core UI Views
- Modules list: status, current build, port, last deploy.
- Module detail: build history, metadata, current pointer.
- Activate existing build: choose a `build_id` already in `incoming/` or `archives/`.
- Rollback: select previous build and switch.
- Restart: trigger a restart for a module.
- Activity log: list actions and errors.

### API Usage (UI)
- Uses the same deploy endpoints and HMAC authentication.
- If static UI is used, restrict access by IP or serve behind a VPN/SSH tunnel.

## Static Rendering + Export
Render static output without serving it:

```bash
hyperbricks static -m <module>
```

Optional flags:
- `--serve` to start the static file server after rendering
- `--force` to overwrite the rendered output without confirmation
- `--zip` to export the rendered output as a zip archive
- `--out <dir>` to set the export folder (default: `./exports/<module>`)
- `--exclude a,b,c` to remove paths from the export (relative to the render root, commas trimmed)

Example export:

```bash
hyperbricks static -m <module> --zip --out ./exports/<module> --exclude "editor, about, blog"
```

Export output:
- `./exports/<module>/export-<module>-YYYYmmdd-HHMMSS.zip`

## Rollbacks
- Remote API: `POST /deploy/modules/<module>/rollback`.
- Manual: set `current` in `deploy/<module>/hyperbricks.versions.json` to an older
  build ID and restart with `--deploy`.

## Required Metadata
Each module must have `hyperbricks.metadata.moduleversion` in `package.hyperbricks`.
The `build` command injects or updates dynamic fields on archive creation.

## Future Considerations
- Optional signature verification of `.hra` (GPG or similar).
- Optional health check before making a build current.
- Optional automatic port pool range configuration.
- Multi-target support for push/sync status per target.

## Glossary
- LAN: local/private network (for example `192.168.x.x` or `10.x.x.x`).
- WAN: the public internet, anything outside your LAN.
