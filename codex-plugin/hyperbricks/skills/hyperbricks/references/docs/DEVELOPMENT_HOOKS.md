<!-- Generated from docs/DEVELOPMENT_HOOKS.md. Do not edit directly. -->

# Development hooks and managed services

Use development hooks to prepare a module and check the running application.
Use managed services to run a local API or microservice for the same development
session. HyperBricks owns the processes it starts and stops them on shutdown.

Execution is optional and explicit:

```sh
hyperbricks start -m my-module --with-processes
```

The flag enables both hooks and services. Ordinary `start` executes neither.
This feature requires HyperBricks v1.2.9-beta or newer. Check
`hyperbricks start --help` for `--with-processes` before trying the examples;
build the current checkout if your installed binary does not have the flag.

Try the runnable [`development-hooks-demo`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/development-hooks-demo/README.md)
or the separate [`development-hooks-plugin-demo`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/development-hooks-plugin-demo/README.md).
The [`catalog-store`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/catalog-store/README.md) also uses a managed
service to start its demo API with the storefront in one command.

The [patterns module](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/hyperbricks-patterns-yaml/README.md#install-the-browser-dependencies)
uses a finite `before_start` hook to install its pinned browser dependencies:

```yaml
hyperbricks:
  development:
    hooks:
      before_start:
        - name: install-frontend-dependencies
          command: [npm, ci, --include=dev, --ignore-scripts, --no-audit, --no-fund]
          timeout: 2m
```

After building its separate native plugins, run it with
`hyperbricks start -m hyperbricks-patterns-yaml --with-processes`.
The hook runs `npm ci` in the module directory on each opt-in start;
`--include=dev` installs its Tailwind CLI even when npm would otherwise omit
development dependencies. The Tailwind component uses a module path to
`node_modules/.bin/tailwindcss`, so no `PATH` export is needed. Ordinary
`start` and build workflows skip this hook and need a manual `npm ci` first.

## Choose a hook or a service

| Need | Configuration | Success means |
| --- | --- | --- |
| Prepare data or build a development plugin | `development.hooks.before_start` | The command exits successfully before components/plugins initialize. |
| Keep a local API running | `development.services` | Its HTTP readiness check succeeds and the foreground process stays alive. |
| Check the application over HTTP | `development.hooks.after_start` | The command exits successfully after HTTP serving begins. |

All entries are required. Hooks run sequentially in list order. Services start
sequentially, waiting for readiness before starting the next. Services stop in
reverse order. There is no automatic restart or optional-failure continuation.

A hook must finish. A service must stay in the foreground. Do not append `&`, use
`nohup`, or start a daemon that detaches and exits. A service script should end in
`exec`, replacing its shell with the actual server process.

## Configure the module

Add declarations to `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  mode: development
  development:
    hooks:
      before_start:
        - name: prepare-api
          command: [python3, prepare.py]
          cwd: {path: {base: module, path: demo-api}}
          timeout: 30s
      after_start:
        - name: verify-page
          command: [python3, verify.py]
          cwd: {path: {base: module, path: demo-api}}
          timeout: 10s
    services:
      - name: demo-api
        command: [sh, server.sh]
        cwd: {path: {base: module, path: demo-api}}
        env:
          PYTHONUNBUFFERED: "1"
          API_PORT: {env: {name: API_PORT, default: "4331"}}
        ready:
          http:
            format: "http://127.0.0.1:%s/health"
            args:
              - env: {name: API_PORT, default: "4331"}
          timeout: 10s
        stop_timeout: 5s
```

The script files belong to the application author. `cwd` uses the existing
[`path` resolver](YAML_USAGE.md#path), including its outer `path` key. No
process-wide directory change is performed.

For example, `demo-api/server.sh` could contain:

```sh
#!/bin/sh
set -eu
exec python3 server.py --port "${API_PORT:-4331}" \
  --site-port "${HB_SERVER_PORT}"
```

The Python server must implement these arguments. The complete example does so
in [`demo-api/server.py`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/development-hooks-demo/demo-api/server.py).

## Configuration reference

Fields apply to each hook or service unless identified otherwise.

| Field | Required | Meaning / default |
| --- | --- | --- |
| `name` | Yes | Readable identifier, unique across both hook lists and all services. Appears in diagnostics and child output. |
| `command` | Yes | Nonempty argument list; first item is the executable. Command strings are rejected. |
| `cwd` | No | Selected module directory by default. Explicit values use normal resolvers; relative results are normalized against the invocation directory. |
| `env` | No | String map of overrides for this child, after resolver evaluation. Inherits the parent environment. |
| Hook `timeout` | No | Positive duration; default `30s`. Termination after timeout has a separate bounded cleanup budget. |
| Service `ready.http` | Yes | Absolute HTTP URL with a literal loopback IP: IPv4 `127.0.0.0/8` or IPv6 `[::1]`. |
| Service `ready.timeout` | No | Positive duration from process spawn; default `10s`. |
| Service `stop_timeout` | No | Positive graceful termination duration; default `5s`, then forced termination. |

Empty lists are valid. Unknown fields, invalid types, duplicate names, and
invalid durations fail validation with the package field path. Structure is
validated even without opt-in. Executable availability and working-directory
existence are checked only when commands are enabled and about to run. An
earlier task can therefore create a later task's executable or working directory.

Commands are argument lists, not shell expressions. Arguments such as `$PORT`,
`~`, pipes, redirects, and `&&` remain literal. Use an explicit shell script for
shell behavior. Bare executable names use the inherited `PATH`; `./tool` is
relative to the child working directory. Child `env` cannot override `PATH`.
Choose an absolute executable or set `PATH` when launching HyperBricks to select
another toolchain.

### Child context

HyperBricks supplies these reserved variables to every hook and service:

| Variable | Value |
| --- | --- |
| `HB_EXECUTABLE` | Absolute path of the running HyperBricks executable. |
| `HB_MODULE_ROOT` | Absolute directory of the selected module. |
| `HB_SERVER_PORT` | Effective site port after package settings and CLI overrides. |

An entry's `env` cannot override these names. They are child environment variables,
not new YAML resolvers. The package is resolved before hooks run and remains
fixed for the invocation. Exporting variables or rewriting the package in a hook
does not reconfigure its parent process.

Set a shared API-port variable in the environment that launches HyperBricks and
resolve it in the service, readiness URL, and application's `api_render` endpoint.
Setting it only in a service's `env` does not change the parent's endpoint.

## Startup and shutdown

1. Load/validate the package and apply CLI overrides. Run `before_start` hooks.
2. Start services and wait for readiness; initialize components and plugins;
   begin HTTP serving; run `after_start` hooks; enable normal source reloads.
3. On interruption or failure, stop reloads, finish bounded HTTP shutdown, then
   stop services in reverse order and collect child exit results.

`after_start` can request the running site. HTTP is already accepting requests
at that point, so a failed verification cannot undo requests already served.
Preparation and verification run once per invocation. Watch events and manual
reload do not rerun hooks, rebuild plugins, or restart services. Package and
service changes need a full CLI restart. Manual reload is unavailable while
startup or shutdown is in progress.

During a normal shutdown, APIs remain alive while HTTP requests drain. HyperBricks
then sends SIGTERM to each owned process group, waits for its stop budget, and
uses SIGKILL if necessary. Task timeout cleanup has a `5s` termination budget.
Background descendants remaining after a hook exits are also cleaned up.

The runner supports macOS and Linux. Other platforms reject enabled, nonempty
process configurations before executing them. Ordinary modules without process
execution keep their normal behavior. SIGKILL of the parent, a machine failure,
or a deliberately detached child cannot be reliably handled by in-process cleanup.

Cleanup stops processes; it does not roll back files or external side effects
created by scripts. Make preparation safe to repeat.

## Readiness checks

Provide a lightweight endpoint that can answer without calling HyperBricks or a
later service. Otherwise startup creates a dependency cycle.

Before spawning, HyperBricks checks whether the readiness address already accepts
connections. An occupied address fails startup; the existing listener is neither
adopted nor terminated. The foreground service must itself fail if binding fails.

After spawning, HyperBricks polls with GET every `100ms`, with a request timeout
up to `500ms` bounded by the remaining readiness deadline. Any 2xx response means
ready. Redirects are not followed and proxy environment settings are ignored.
DNS names, HTTPS, embedded credentials, and URL fragments are unsupported for
this local startup probe.

Readiness is checked during startup only. Process exits are monitored throughout
the session, including exit code zero. A running API that later becomes
unresponsive is handled by normal application upstream-error behavior; the
runner does not continuously poll health or restart it.

## Where commands execute

| Workflow | Behavior |
| --- | --- |
| Direct `start --with-processes`, effective development/debug mode | Runs configured hooks and services. |
| Direct `start` | Skips them; configured entries produce an opt-in reminder. |
| `start --with-processes --production` / effective live mode | Rejects process execution before spawning. |
| Deployment-managed runtime, including development preview | Does not execute them; attempted opt-in is rejected. |
| `init`, `init-starter`, `doctor`, `author`, editor validation | Does not execute them. |
| Build, static export/serving, deployment packaging | Does not execute them. |
| Template/source reload | Does not execute them again. |

`--with-processes` executes module-authored commands with the current user's OS
permissions and inherited environment. A module-relative path is not a sandbox.
There is no interactive prompt or remembered approval state. An explicit opt-in
also works in a noninteractive development/CI session.

## Build a development plugin

A `before_start` hook finishes before plugin loading, so it can call the existing
`hyperbricks plugin build` command. Use `HB_EXECUTABLE` to select the same CLI.
Align the command working directory, source module, output directory, and
`hyperbricks.directories.plugins` deliberately: the plugin CLI writes to
`./bin/plugins` relative to its own working directory.

The executable [plugin fixture](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/development-hooks-plugin-demo/README.md)
embeds a fresh build ID, builds the native artifact, then verifies that exact ID
through the running route. It also demonstrates failure with an old artifact
still present: a failed preparation hook stops startup before that artifact can
be loaded.

The existing [plugin compatibility rules](PLUGINS.md#local-runtime-development)
still apply. Build production artifacts during build/package preparation.
Development hooks do not add a deployment compiler or change plugin loader rules.

## Failures and diagnostics

| Event | Result |
| --- | --- |
| Hook returns nonzero, times out, or cannot execute | Fail startup; clean up already-started processes. |
| API address is occupied | Fail without touching the existing listener. |
| Readiness times out or a service exits | Name the service and report the last readiness result/output; clean up. |
| Site cannot bind after services start | Stop the services and return failure. |
| `after_start` fails | Finish HTTP shutdown and stop services. |
| Required service exits during serving, even with code zero | Fail the session and clean up. |
| Child ignores SIGTERM | Force termination after its budget and report it. |

Logs identify the phase and process name. Child stdout/stderr are streamed with
their process name; bounded output tails help explain failures. Environment maps
and full command arguments are not printed automatically. Child scripts remain
responsible for what they log.

Lifecycle failures and cleanup failures return exit code `1`. Clean Ctrl+C
returns `130`; clean SIGTERM or the normal quit action returns `0`. A cleanup
failure takes precedence over an otherwise clean interruption.

See [Troubleshooting](TROUBLESHOOTING.md#development-hooks-or-managed-services-do-not-start)
and the [CLI start reference](HYPERBRICKS_CLI.md#development-hooks-and-services).
