# Development hooks: one command, one session

Start HyperBricks and a small Python API together. HyperBricks runs a preparation
task, waits for the API to become ready, serves the page, and verifies that the
page contains this session's API data. Stopping HyperBricks stops its API too.

This is a new feature demonstration. It does not change Catalog Store or require
npm, a Python package installation, or a database.

## Requirements

- macOS or Linux.
- Python 3.8 or newer (`python3 --version`). All scripts use the standard library.
- A HyperBricks binary with `start --with-processes`. Go 1.26.1 or newer is
  needed only if you build the binary from source.
- Free local ports **4330** (HyperBricks) and **4331** (API), or the overrides below.

From the project root (the directory containing `modules/`), run:

```sh
hyperbricks start -m development-hooks-demo --with-processes
```

If you are developing the HyperBricks repository and your installed binary is
older, build the current checkout with `go build -o /tmp/hyperbricks-hooks
./cmd/hyperbricks` and use `/tmp/hyperbricks-hooks` in place of `hyperbricks`
in these commands.

Open [127.0.0.1:4330](http://127.0.0.1:4330/). The card shows the session ID, API
process ID, preparation time, and effective HyperBricks URL. Styling uses daisyUI 5
and Tailwind 4 from their public CDNs; the server-rendered content and lifecycle
checks work without those CDN requests.

Press **Ctrl+C** in the terminal to stop the whole session. Exit code `130` is
the expected result of a clean Ctrl+C. A clean SIGTERM returns `0`.

The public demo runs without a developer login. Its dashboard is disabled by
default. If you enable Dashboard in `package.hyperbricks.yaml`, it opens without
login when both developer credentials are absent, with a startup warning. To
require login, set both values before startup; there is no built-in account:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
```

A partial account blocks access with `503`. Restart after changing package
settings or these variables. Dashboard is available in development and debug
mode, and excluded from live mode, production runtimes, and static output.
Frontend editing is explicitly disabled in this module. See
[developer access settings](../../docs/SPACES.md#development-configuration).

## What runs, and when

| Step | File | What it demonstrates |
| --- | --- | --- |
| Before startup | [`demo-api/prepare.py`](demo-api/prepare.py) | Creates a fresh session ID and local data, then exits. |
| Managed service | [`demo-api/server.sh`](demo-api/server.sh) | Uses `exec` to keep the Python API in the foreground. |
| Readiness | [`demo-api/server.py`](demo-api/server.py) | `/health` answers independently of HyperBricks. |
| Page render | [`hyperbricks/main.hyperbricks.yaml`](hyperbricks/main.hyperbricks.yaml) | Nested `api_render` retrieves `/message` and renders a template. |
| After HTTP starts | [`demo-api/verify.py`](demo-api/verify.py) | Fetches the actual page and checks session data, diagnostics, and port propagation. |
| Shutdown | HyperBricks runtime | Finishes HTTP shutdown, then terminates its API process. |

The declaration lives in [`package.hyperbricks.yaml`](package.hyperbricks.yaml).
Templates live separately under [`templates/`](templates/). The API is deliberately
small: `/health` reports readiness; `/message` returns session JSON.

Refresh the page: the session ID and API PID stay the same. Edit `page.html` and
refresh: the template changes without rerunning preparation or starting a second
API. A full CLI restart creates a new session. Changes to package configuration
or API code require a restart.

Generated state lives in ignored `demo-api/.runtime/`. `session.json` is preparation
output; `verified.json` proves the after-start task completed. Shutdown preserves
those files because stopping processes does not roll back script output.

## Change the ports

```sh
HOOKS_DEMO_API_PORT=14331 hyperbricks start \
  -m development-hooks-demo --port 14330 --with-processes
```

Open [127.0.0.1:14330](http://127.0.0.1:14330/). `HOOKS_DEMO_API_PORT` configures
the API process, readiness URL, and `api_render` endpoint. HyperBricks supplies
`HB_SERVER_PORT` to child scripts after applying `--port`, so verification and the
API's displayed site URL use `14330` too.

The variables serve different purposes: package resolvers read the environment
of the HyperBricks process; an entry's `env` changes only that child process.
Setting a variable only in a service's `env` cannot reconfigure the parent page.

## Run the API yourself

Process execution is optional. In terminal one, from the project root:

```sh
cd modules/development-hooks-demo/demo-api
python3 prepare.py
HB_SERVER_PORT=4330 sh server.sh
```

In terminal two, from the project root:

```sh
hyperbricks start -m development-hooks-demo
```

Without `--with-processes`, HyperBricks executes neither hooks nor services. You
own the API terminal and must stop it yourself. If the API is absent, the page
shows an explanatory message through its normal API-error template.

## Try a failure

Stop the previous session before each exercise:

```sh
HOOKS_DEMO_FAIL_PREPARE=1 hyperbricks start \
  -m development-hooks-demo --with-processes
```

Preparation fails and startup stops before launching the API or serving HTTP.

```sh
HOOKS_DEMO_FAIL_VERIFY=1 hyperbricks start \
  -m development-hooks-demo --with-processes
```

Verification fails after serving begins. HyperBricks shuts down HTTP and its API,
and exits with failure. The logs name the failed task.

To see the occupied-port diagnostic, run the manual API first and then opt in to
the managed session. Startup refuses to adopt that existing listener and leaves
it running. Stop that API in its own terminal.

Read the full [development hooks and managed services reference](../../docs/DEVELOPMENT_HOOKS.md).
For build-before-plugin-load, try the separate
[`development-hooks-plugin-demo`](../development-hooks-plugin-demo/README.md).

## Run the repeatable smoke test

From the project root, pass the absolute path of the installed binary:

```sh
python3 modules/development-hooks-demo/tests/smoke.py \
  --binary "$(command -v hyperbricks)"
```

The standard-library-only suite makes temporary module copies and allocates
separate loopback ports. It checks the rendered API data, site-port propagation,
source reload without another API or preparation run, Ctrl+C exit `130`, failed
preparation, failed verification, and a required API process exiting. Every
scenario checks listener and child-process cleanup. The fixture includes bounded
fallback cleanup for failed assertions and does not adopt an existing service.
