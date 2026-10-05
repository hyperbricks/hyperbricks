# Deploy

HyperBricks builds modules into `.hra` archives, runs those archives locally,
and can push them to a remote deployment host. All deployment workflows live
under one command:

```text
hyperbricks deploy init
hyperbricks deploy run
hyperbricks deploy local
hyperbricks deploy remote
```

The old `deploy-daemon` command and the deployment flags on `hyperbricks start`
have been removed. `start` serves source modules; `deploy run` serves an
extracted deployment build.

## Configuration Model

One `deploy.hyperbricks.yaml` can describe three independent roles:

| Role | Owner | Purpose |
| --- | --- | --- |
| `deploy.local` | Local deployment service | Browser interface, local modules/builds, local runtime ports and Basic Auth |
| `deploy.client` | Outbound deploy client | Named remote targets, target Basic Auth and request-signing secret |
| `deploy.remote` | Remote deployment service | Remote interface/API, archive root, runtime processes, Basic Auth and HMAC verification |

Each command reads only its own role. `deploy local` does not borrow runtime
settings from `deploy.remote`, and the remote service does not borrow secrets
from `deploy.client`. A local interface may start without client targets; its
remote push and sync actions remain unavailable until a target is configured.

Create one neutral starter configuration:

```bash
hyperbricks deploy init
```

Select another path with:

```bash
hyperbricks deploy init --config configs/deploy.hyperbricks.yaml
```

Initialization creates the file only when it does not exist. It does not create
a missing parent directory and never overwrites an existing file. The generated
configuration contains environment references, not default secrets.

`deploy init`, `deploy local`, and `deploy remote` resolve their configuration
in this order:

1. `--config <path>`
2. `HB_DEPLOY_CONFIG`
3. `deploy.hyperbricks.yaml` in the invocation directory

An explicitly selected file is authoritative. A missing, unreadable, or invalid
file causes the command to fail; HyperBricks does not create a configuration or
fall back to another path during service startup.

## Complete Configuration Example

```yaml
deploy:
  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy
    port_start: 8080
    logs_enabled: true
    credentials:
      user:
        env: HB_DEPLOY_LOCAL_USER
      password:
        env: HB_DEPLOY_LOCAL_PASSWORD

  client:
    target: production
    targets:
      production:
        api: https://deploy.example.com
        credentials:
          user:
            env: HB_DEPLOY_CLIENT_PRODUCTION_USER
          password:
            env: HB_DEPLOY_CLIENT_PRODUCTION_PASSWORD
        hmac_secret:
          env: HB_DEPLOY_CLIENT_PRODUCTION_HMAC_SECRET
        # key_id: production

  remote:
    bind: 127.0.0.1
    port: 9090
    root: deploy
    port_start: 8080
    logs_enabled: true
    credentials:
      user:
        env: HB_DEPLOY_REMOTE_USER
      password:
        env: HB_DEPLOY_REMOTE_PASSWORD
    hmac_secret:
      env: HB_DEPLOY_REMOTE_HMAC_SECRET
    auth:
      env_prefix: HB_DEPLOY_SECRET_
    # binary: /usr/local/bin/hyperbricks
```

The generic YAML resolver materializes each `env` value when the file is read.
An unset variable therefore produces an empty value; there is no built-in user,
password, or HMAC secret. Unknown fields are rejected. In particular, the old
top-level `deploy.hmac_secret` and remote `api_enabled`, `api_bind`, and
`api_port` fields are invalid.

A developer machine may keep only `local` and `client`; a deployment host may
keep only `remote`; a single machine may keep all three. The filename does not
select a role—the `local` or `remote` subcommand does. Unused valid sections do
not activate services and may remain in the file.

## Authentication

The local and remote deployment services protect their complete HTTP surfaces
with Basic Auth, including HTML, assets, status routes, and APIs.

- If both configured server credential values resolve, a missing or invalid
  `Authorization` header receives `401 Unauthorized` and a Basic challenge.
- If either server credential is missing, incomplete, or resolves empty, the
  service starts locked and every request receives `503 Service Unavailable`.
- A client target requires both `user` and `password`. Push and sync fail before
  making a network request when either value is missing.

Remote deployment operations are protected by a second layer: HMAC-SHA256
request signing. Basic Auth is evaluated first, before the remote service reads
the request body or performs HMAC verification.

The supported browser topology is same-origin: each deployment dashboard calls
the API served by the same scheme, host, and port. Browser-managed Basic Auth is
then reused for the dashboard's `fetch()` requests. The remote dashboard's
editable **API Base** must not point at another origin; credentialed cross-origin
requests and a CORS trust model are not part of this interface contract.

HTTP Basic Auth does not encrypt credentials. Loopback HTTP is suitable for the
local service; remote or LAN access requires HTTPS, a TLS reverse proxy, or a
private encrypted network. Browser-managed Basic Auth has no reliable
application-level logout.

Every signed request sends:

- `X-HB-Timestamp`
- `X-HB-Nonce`
- `X-HB-Signature`

Archive uploads also send:

- `X-HB-Build-ID`
- `X-HB-SHA256`

The canonical signing value starts with:

```text
METHOD
PATH
SHA256(body)
timestamp
nonce
```

When a key ID or build ID is present, it appends both slots:

```text
key_id
build_id
```

Without `key_id`, the client signs with its target `hmac_secret` and the remote
service verifies with `deploy.remote.hmac_secret`. Configure both environment
references with the same value.

For keyed mode, add `key_id` to the client target. The client still reads its
secret from that target's `hmac_secret`; the remote service reads the matching
secret from:

```text
<deploy.remote.auth.env_prefix><NORMALIZED_MODULE>_<NORMALIZED_KEY_ID>
```

For module `demo`, key `production`, and the default prefix, the remote variable
is `HB_DEPLOY_SECRET_DEMO_PRODUCTION`. Point the client target's `hmac_secret`
environment reference at the same value on the client machine. The shared remote
secret never authenticates a request that carries `X-HB-Key-ID`.

The server rejects stale timestamps, nonce reuse, invalid body hashes, and bad
signatures. HMAC provides integrity and request authentication, not
confidentiality; use HTTPS or a private network between machines.

## Package Metadata Lifecycle

Package metadata has two owners. The source
`package.hyperbricks.yaml` carries the module identity that a developer can
maintain:

```yaml
hyperbricks:
  metadata:
    module: demo
    moduleversion: "1.0.0"
    hyperbricks: <exact installed version>
```

`hyperbricks init` and `hyperbricks init-starter get` fill those fields for a
new module. The module is the destination directory's base name, a new module
starts at version `1.0.0`, and the HyperBricks version comes from the running
binary; the placeholder above represents that exact value. Existing packages
can be reconciled without changing scaffold files:

```bash
hyperbricks init -m demo --update-metadata
hyperbricks init -m ./modules/demo --update-metadata
hyperbricks init -m demo --bump-version
hyperbricks init -m demo --bump-version=minor
```

The metadata modes accept the same module names and relative or absolute paths
as `start -m` and `build -m`. A bare `--bump-version` means `patch`; the
accepted explicit values are `patch`, `minor`, and `major`.

An archive owns the immutable provenance of one build. At build time,
HyperBricks overlays these values into the package copy written to the HRA or
ZIP:

| Field | Archive value |
| --- | --- |
| `module` | Selected module directory's base name |
| `moduleversion` | Validated source module version |
| `format` | Actual selected format, `hra` or `zip` |
| `format_version` | Archive metadata schema version |
| `commit` | Source Git commit, or `unknown` when unavailable; a duplicate retains the parent build's recorded commit |
| `origin_build_id` | Immediate parent build ID for a build duplicated from an extracted runtime; omitted for ordinary source builds |
| `built_at` | Build timestamp in UTC RFC 3339 form |
| `hyperbricks` | Exact version of the building binary |

`hyperbricks build` leaves the source package unchanged. Values such as
`format`, `format_version`, `commit`, `origin_build_id`, and `built_at` do not
belong in source
metadata because they describe a particular artifact. `source_hash` belongs to
the build index rather than either package document. A metadata refresh removes
legacy source copies of all five fields.

## Build An Archive

```bash
hyperbricks build --hra -m demo
hyperbricks build --hra -m ./modules/demo
```

`build -m` accepts the same module names and paths as `start -m`. A path uses
the selected directory's base name as the deployment module name. Archives and
their index are stored as:

```text
deploy/<module>/
  <module>-<moduleversion>-<build_id>.hra
  hyperbricks.versions.json
```

Common build options:

| Flag | Purpose |
| --- | --- |
| `-m, --module <name-or-path>` | Module below `./modules`, or a relative/absolute module directory |
| `--out <dir>` | Output directory; default `deploy` |
| `--force` | Build even when the source hash is unchanged |
| `--replace` | Replace the current build |
| `--replace <build_id>` | Replace a specific build |
| `--push` | Build and push to a deploy target |
| `--target <name>` | Select a named target for `--push` |

ZIP builds remain available through `hyperbricks build --zip -m demo`, but HRA
is the deployment format.

## Run A Deployment Build

Run the current archived build:

```bash
hyperbricks deploy run -m demo
```

Run a specific build or use another archive root:

```bash
hyperbricks deploy run -m demo --build build-id
hyperbricks deploy run -m demo --deploy-dir ./artifacts
```

Select the effective runtime mode explicitly:

```bash
hyperbricks deploy run -m demo --mode development
hyperbricks deploy run -m demo --mode live
```

`--mode` accepts only `development` or `live` and overrides
`hyperbricks.mode` in the extracted package configuration. The legacy
`--production` flag remains a compatibility alias for live mode; it cannot be
combined with `--mode development`.

`deploy run -m` accepts the same module selection syntax as `start -m`. It
extracts the archive into:

```text
deploy/<module>/runtime/<build_id>/
```

and starts the runtime with the extracted `package.hyperbricks.yaml`. The
remote and local deployment services use this command when they launch an
archived build.

## Local Deployment Interface

```bash
hyperbricks deploy local
hyperbricks deploy local --config /path/to/deploy.hyperbricks.yaml
```

The local service uses only `deploy.local` for its listener, module/build roots,
runtime port allocation, logging, and interface credentials. It:

- scans source modules
- builds and manages local archives
- runs source or archived modules locally
- pushes builds through the selected `deploy.client` target
- synchronizes remote build state on demand

The service is intended for a developer workstation and defaults to loopback.

## Interface Navigation And Responsive Layout

Both deployment interfaces use the same navigation. The top-level **Modules**
and **Shared Plugins** tabs separate module builds from host-wide plugins; a
module's own plugins are under **Modules → Custom plugins** after selecting that
module. The module detail has **Overview** (builds) and **Custom plugins** tabs.
The module toolbar's **Actions** menu contains **Build** and **Sync Remote** on
the local host, or **Upload .hra** and **Rollback** on the remote host. Each build
has its own action menu for start/stop, package configuration, developer access,
download, duplicate, and details where applicable. The header's **DEPLOY API**
badge identifies the local or remote host; it is not a host switch.

The module and shared-plugin lists have independent search and scrolling. Below
1050px they become overlay drawers opened with **Browse modules** or **Browse
plugins**. At 710px and below, build and plugin rows become cards instead of a
horizontally scrolling table. The collapsible **Activity** panel stays at the
bottom of the active workspace. Connection settings and contextual help are
available from the header; help popovers open only when their icon is activated.

## Remote Deployment Interface And API

```bash
hyperbricks deploy remote
hyperbricks deploy remote --config /etc/hyperbricks/deploy.hyperbricks.yaml
```

The remote service uses only `deploy.remote`. It receives and activates
archives, starts and stops module processes, exposes logs/status, and serves the
remote deployment interface.

Remote storage layout:

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

Remote uploads are stored under `archives/`. When `deploy.local.build_root` and
`deploy.remote.root` intentionally share one deployment directory, the remote
interface also accepts an indexed HRA stored directly under the module
directory by the local build workflow. In both layouts the resolved archive
must remain inside that module directory.

The upload endpoint is:

```text
POST /deploy/v1/modules/{module}/releases
```

The remote interface also provides **Upload .hra** in the selected module's
**Actions** menu. It derives the module and build ID from the canonical filename
`<module>-<moduleversion>-<build_id>.hra`, signs the binary request, uploads it,
and activates it. Noncanonical names are rejected.

Before activation, the host reads `hyperbricks.metadata` from the archive's
`package.hyperbricks.yaml`. Its `module` must match the upload route and its
`hyperbricks` value must exactly match the host runtime version. Mismatches are
rejected before the archive can run.

## Response Cache Files and Deployment

HyperBricks automatically excludes `.cache` directories from module deployment packages and runtime snapshots, alongside the existing `.git` and `node_modules` exclusions. If `hyperbricks.directories.cache` selects another directory inside the module, that configured directory is excluded as well. These are built-in archive rules: `.gitignore` is not used to determine package contents, and users do not need to add a separate exclusion for the response cache. Archive directory settings come from the module’s default `package.hyperbricks.yaml`. When starting with a different configuration profile, keep its cache under `.cache`, outside the module, or at the same cache location declared in the default package.

Keep required source files and assets outside cache directories. The default disk response cache lives under `<module>/.cache/responses/<runtime-id>/` and contains disposable runtime data. The destination runtime creates its own files; response entries are not shipped, restored or reused across process restarts. A new deployment starts with an empty response cache. Configure a dedicated writable `directories.cache` location for read-only deployments, outside public static and rendered-output directories.

Per-route `cache` chooses memory/disk storage and expiry; package `live.cache: 0s` remains the global off switch. Development/debug modes render fresh. See [Output Cache](LIVE_MODE_HTTP.md#output-cache) for configuration, disk limits and cleanup.

To invalidate a running live build without restarting, run `hyperbricks cache purge --module /path/to/extracted/runtime --all` on that host as the runtime's operating-system user. Use `--route products` instead of `--all` for one route. The private local control connection supports macOS/Linux; when several instances share that module directory, select the instance reported by the command with `--instance`. This clears internal memory and disk entries in the selected process, not browser or proxy caches. See [purge controls](LIVE_MODE_HTTP.md#purging-memory-and-disk).

## Build Controls In The Local And Remote Interfaces

The local and remote deployment interfaces store runtime mode per archived
build. A build can be switched directly between **Development** and **Live**;
the selected mode is passed to `deploy run` and takes precedence over the
archive's `hyperbricks.mode` and legacy production environment flags. New
builds start in Development mode.

Changing the mode of a running archived build restarts that build immediately
so the displayed selection and effective process mode stay aligned. Changing an
inactive build stores the choice for its next start. Saving the choice and
restarting are separate outcomes: if the restart fails, the chosen mode remains
saved and the interface shows the restart error. The local `dev` row is the
source-module workflow rather than an archive, remains in Development mode, and
shows a disabled Development selector. Editing `hyperbricks.mode` in the source
package does not override this deployment-interface development workflow.

Live mode is not itself a promise that rendering is uncached. The effective
package configuration still owns `hyperbricks.live.cache`. Set it to `0s` when
the Live build must bypass the internal HyperBricks rendered-output cache on
every request. This does not disable browser, reverse-proxy, or CDN caches;
configure the appropriate HTTP response headers separately. Negative cache
durations are invalid. See [Live Mode HTTP Settings](LIVE_MODE_HTTP.md) for the
cache and verification contract.

The mode APIs are:

```text
PUT /local/modules/{module}/builds/{build_id}/mode
PUT /deploy/modules/{module}/builds/{build_id}/mode
```

Both accept either `{"mode":"development"}` or `{"mode":"live"}`. The old
per-build `production` endpoints remain available for compatibility and map
`false` to Development and `true` to Live.

A successful mode response includes `runtime_mode` and `restarted`. A nonempty
`restart_error` means the mode was saved but restarting failed; it is not a
rejected mode update. The next successful start uses the saved mode.

### Startup failures

Starting a module validates its package configuration before stopping a running
instance. The service then waits for the new child process to open its runtime
listener before reporting success. An early exit or startup timeout is returned
as an error instead of briefly marking the build Running.

An older archive can contain obsolete configuration even when its source module
has since been corrected. For example, a boolean `development.dashboard` must
be migrated to a mapping in the extracted `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  development:
    dashboard:
      enabled: false
```

Use `enabled: true` only when the developer dashboard is intended to be enabled.
Configure both developer credentials to require login; with both absent, a
development dashboard opens without login and emits a startup warning. Rebuild the
archive from corrected source for a durable fix, because changing the extracted
runtime configuration does not change the original archive.

### Open the developer dashboard

**Open dashboard** is shown only for a running Development process. It is
disabled with an explanation when the package configuration does not enable
`hyperbricks.development.dashboard.enabled` or availability cannot be established.
It is hidden for Live and stopped builds. Build details include a dashboard link
only when it is available. Availability follows the running process, not an
unapplied saved mode change. Local and remote status responses provide
`running_mode` and `dashboard_path`; the browser rechecks build status before
opening that path.

The developer endpoint is `/__hyperbricks/dashboard`. It uses the module's
developer Basic Auth credentials, not the deployment HMAC secret. With both
module credential values absent, Dashboard, Errors, and diagnostics open without
login and startup warns that anyone who can reach the server can view them.
A configured account requires login (`401` challenge); a partial account returns `503`.
After editing dashboard settings, restart the module to apply them.

### Legacy Build-Index Migration

Build indexes now persist `runtime_mode`. Local and remote services normalize
older rows deliberately as follows:

| Stored fields | Effective mode |
| --- | --- |
| Valid `runtime_mode: "development"` or `runtime_mode: "live"` | The explicit runtime mode |
| Missing or unrecognized `runtime_mode` with `production: true` | Live |
| Missing or unrecognized `runtime_mode` with `production: false` or no `production` field | Development |

The last row is an intentional migration from the earlier implicit
package/default behavior: legacy `production: false` and omitted production
state now mean Development. On a later index write, HyperBricks records the
normalized `runtime_mode` and keeps `production` as a compatibility mirror.

### Developer access settings

Each build's action menu includes **Developer access** in both the local and
remote deployment interfaces. It edits the module's shared developer login at
`hyperbricks.development.dashboard.credentials`, used by Dashboard, Errors and
Spaces. It does **not** change the deployment server login or HMAC secret.

The dialog shows the current username and a masked password. **Show credentials**
reveals the configured password; closing the dialog clears its values from the
form. Enter a new username, a new password, or both, then choose **Save
changes**. Empty new fields leave existing values unchanged. When no
credentials are configured, supply both fields to change the login; the missing
YAML hierarchy is inserted automatically. New values are stored as quoted,
plain-text YAML values.

The same dialog has independent Development-mode switches for **Dashboard**
(`hyperbricks.development.dashboard.enabled`), which shows the Overview and
Errors views, and **Spaces**
(`hyperbricks.development.frontend_editing.spaces.enabled`). They can be saved
without changing credentials. Dashboard defaults to disabled; Spaces defaults
to enabled for compatibility. The parent
`hyperbricks.development.frontend_editing.enabled` still controls all frontend
editors. If that parent is disabled, switching Spaces on alone does not make it
visible. Neither tool is available in Live mode. With both developer credential
values absent, the enabled Dashboard opens without login and warns at startup;
Spaces remains locked with `503` until credentials are configured and the module
is restarted. A partial account locks both tools.

Environment-backed values are displayed as their configured YAML reference,
never as resolved server environment secrets. Unchanged references are
preserved. Replacing a reference with a stored literal requires confirmation.
For production secrets, managing the referenced environment values outside this
dialog remains preferable. As with the deployment interface itself, use HTTPS
when accessing a non-loopback host.

The edited file follows the scope table below: the local source row updates the
source package; archived builds update only their extracted runtime. The
original `.hra` is unchanged. Saving preserves unrelated YAML, comments, free
variables, unchanged visibility switches, and the selected runtime mode. Unusual YAML
layouts that cannot be updated without rewriting unrelated text are rejected
with instructions to use **Edit package config** instead.

Saves require the opened file's content hash, use the Go configuration validator,
and write atomically. If another writer changed the file, close and reopen the
dialog before retrying. Developer access responses are not cached, and
credential values are not stored in browser local storage or written to the
activity log.

Saving never restarts a running module. Restart it explicitly to apply login or
visibility changes. The existing **Open dashboard** availability rules remain
unchanged until the running process reports the new setting.

### Editing `package.hyperbricks.yaml`

Both interfaces provide **Edit package config**, but the file being edited
depends on the selected build:

| Selection | Edited file | Scope |
| --- | --- | --- |
| Local `dev` source row | `<modules_dir>/<module>/package.hyperbricks.yaml` | Source configuration; a restart applies it to the source workflow and future archives copy it at build time. |
| Local archived build | `<build_root>/<module>/runtime/<build_id>/package.hyperbricks.yaml` | Extracted runtime configuration for only that build. |
| Remote archived build | `<remote.root>/<module>/runtime/<build_id>/package.hyperbricks.yaml` | Extracted runtime configuration for only that build. |

For an archived build, opening the editor extracts the HRA or ZIP runtime when
needed. The YAML editor is loaded only when this dialog is opened. It provides
line numbers, YAML highlighting, undo/redo, indentation, `Cmd/Ctrl+S`, and fast
syntax feedback from `js-yaml`. That browser-side check is informational: the
HyperBricks Go configuration validator remains authoritative when saving. The
built-in local and remote servers negotiate gzip for this separately loaded
editor asset while retaining an uncompressed response for older clients.

The editor always submits the literal source text rather than parsing and
serializing it. Comments, quoted scalars, blank lines, trailing newlines, and a
consistent CRLF/LF line-ending style are therefore preserved. Arbitrary
top-level configuration variables remain editable like any other YAML key.

The editor separately shows the **Saved runtime mode**. This is the per-build
override from the deployment index; it can differ from `hyperbricks.mode` in the
raw package text. Use the build's Development/Live selector to change the mode
that the deployment service will run. The source-only `dev` workflow always
runs in Development. Package-config responses expose this as `runtime_mode`
without rewriting the returned source text. If a restart fails validation while
the previous process stays running, the saved mode applies to the next
successful start, not to that unchanged process.

Saving validates the proposed configuration and writes only that extracted
runtime copy. It never rewrites the immutable source `.hra`, so deleting and
re-extracting the runtime restores the package configuration from the archive.
The build's per-build Development/Live selection still overrides any
`hyperbricks.mode` value edited in this runtime copy.

If the selected build is running, a successful package-config save reports that
a restart is required; the editor does not restart it automatically. Reload the
editor before retrying when another writer changed the file, because saves use
the opened content hash to prevent overwriting a newer revision.

The package-config APIs are:

```text
GET|PUT /local/modules/{module}/builds/{build_id}/package-config
GET|PUT /deploy/modules/{module}/builds/{build_id}/package-config
```

A `PUT` body contains `content` and may include the `expected_sha256` returned by
`GET` for stale-write protection.

### Downloading A Build Archive

Each archived HRA build in either interface provides **Download .hra**. The
download is the original immutable archive for that build, not a new archive of
the extracted runtime. Edits made through **Edit package config** are therefore
not included in the download.

The source-only local `dev` row and ZIP builds do not have an HRA download. The
download endpoints are:

```text
GET /local/modules/{module}/builds/{build_id}/archive
GET /deploy/modules/{module}/builds/{build_id}/archive
```

### Duplicate As A New Build

For an archived HRA build, **Duplicate as new build** packages the selected
build's extracted runtime into a separate HRA and adds it to the same module's
build list. It includes edits made to that runtime, including its saved package
configuration and files added under the runtime root. Known in-progress editor
and asset-generation staging files are excluded. The service checks for changes
while taking the snapshot and asks for a retry rather than publishing a mixed
archive if the runtime changes during that operation. A duplicate is limited
to 50,000 included entries and 2 GiB of runtime content. It does not rebuild
the source module. The selected runtime's HyperBricks version must match the
deploy host version; upgrading an older archive still requires rebuilding from
source.
The original archive is unchanged; an already extracted runtime is not edited
by duplication.

The duplicate gets a new content-derived build ID and build timestamp. Its
archive metadata records `origin_build_id` and retains the source commit as
provenance. The module version comes from the current runtime package
configuration and is not automatically bumped. The new build inherits the
selected build's saved Development/Live mode in the deployment index. As with
other builds, that saved mode can differ from `hyperbricks.mode` inside the
archive. Downloading the duplicate and uploading it to another host does not
transfer this index setting; choose Live on the destination if needed.

Duplicating does not start the new build, stop a running build, or change the
module's current-build pointer. The new row can be started or downloaded using
the existing actions. The local source-only `dev` row and ZIP builds cannot be
duplicated this way. Do not run a separate CLI build against the same module
concurrently with duplication: index writes from independent processes are not
yet serialized. The endpoints are:

```text
POST /local/modules/{module}/builds/{build_id}/duplicate
POST /deploy/modules/{module}/builds/{build_id}/duplicate
```

## Push Flow

Build and push using the configured default target:

```bash
hyperbricks build --hra -m demo --push
```

Select a target explicitly:

```bash
hyperbricks build --hra -m demo --push --target production
```

The client validates its selected target, attaches Basic Auth, signs the HRA,
uploads it, and reports the remote activation result. If upload succeeds but
activation fails, the local build remains available.

## Environment Overrides

The config file is the source of role-owned settings. These operational remote
overrides remain available:

| Variable | Purpose |
| --- | --- |
| `HB_DEPLOY_CONFIG` | Alternate deployment config path |
| `HB_DEPLOY_BIND` | Remote bind address |
| `HB_DEPLOY_PORT` | Remote listener port |
| `HB_DEPLOY_ROOT` | Remote archive/runtime root |
| `HB_DEPLOY_PORT_START` | First remote module runtime port |
| `HB_DEPLOY_LOGS` | Enable or disable remote runtime logs |
| `HB_DEPLOY_BIN` | Binary used to launch module processes |
| `HB_DEPLOY_SECRET_<MODULE>_<KEY_ID>` | Keyed remote HMAC secret, with the configured prefix |

Credential and shared HMAC variables are named by the YAML `env` references;
the names shown in the generated file are conventions, not fallback lookups.

## Service Examples

Systemd:

```ini
[Unit]
Description=HyperBricks Remote Deployment Service
After=network.target

[Service]
User=deploy
WorkingDirectory=/opt/hyperbricks
Environment=HB_DEPLOY_CONFIG=/opt/hyperbricks/deploy.hyperbricks.yaml
EnvironmentFile=/etc/hyperbricks/deploy.env
ExecStart=/usr/local/bin/hyperbricks deploy remote
Restart=always

[Install]
WantedBy=multi-user.target
```

OpenRC:

```sh
#!/sbin/openrc-run

name="hyperbricks-deploy"
description="HyperBricks Remote Deployment Service"
command="/usr/local/bin/hyperbricks"
command_args="deploy remote"
command_user="deploy:deploy"
directory="/opt/hyperbricks"
pidfile="/run/${name}.pid"
command_background="yes"

depend() {
  need net
}
```

Keep the actual credential and HMAC values in the service environment, with
permissions limited to the service account. See [Docker](DOCKER.md) for the
container deployment host.

## Rollbacks

The remote interface and API can activate an earlier archived build. Manual
rollback is also possible by changing `current` in
`deploy/<module>/hyperbricks.versions.json` and restarting the module.

## Static Export

Static export is independent of deployment archives:

```bash
hyperbricks static -m demo
hyperbricks static -m demo --zip --out ./exports/demo
```

Static rendering snapshots routes through the normal runtime HTTP path. Public
`api_render` content is fetched during export, so its upstream services must be
reachable from the build environment.
