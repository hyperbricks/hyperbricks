# Development hooks: one command, one session

Start HyperBricks and a small Python API together. HyperBricks runs a preparation
task, waits for the API to become ready, serves the page, and verifies that the
page contains this session's API data. Stopping HyperBricks stops its API too.

This is a new feature demonstration. It does not change Catalog Store or require
npm, a Python package installation, or a database.

## Requirements

- macOS or Linux.
- Python 3.8 or newer (`python3 --version`). All scripts use the standard library.
- Go 1.26.1 or newer to build the HyperBricks checkout containing this feature.
- Free local ports **4330** (HyperBricks) and **4331** (API), or the overrides below.

An older globally installed binary may not have `--with-processes`, even if its
version label matches the checkout. Build this checkout from the repository root:

```sh
go build -o /tmp/hyperbricks-hooks ./cmd/hyperbricks
/tmp/hyperbricks-hooks start -m development-hooks-demo --with-processes
```

Open [127.0.0.1:4330](http://127.0.0.1:4330/). The card shows the session ID, API
process ID, preparation time, and effective HyperBricks URL. Styling uses daisyUI 5
and Tailwind 4 from their public CDNs; the server-rendered content and lifecycle
checks work without those CDN requests.

Press **Ctrl+C** in the terminal to stop the whole session. Exit code `130` is
the expected result of a clean Ctrl+C. A clean SIGTERM returns `0`.

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
HOOKS_DEMO_API_PORT=14331 /tmp/hyperbricks-hooks start \
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

Process execution is optional. In terminal one, from the repository root:

```sh
cd modules/development-hooks-demo/demo-api
python3 prepare.py
HB_SERVER_PORT=4330 sh server.sh
```

In terminal two, from the repository root:

```sh
/tmp/hyperbricks-hooks start -m development-hooks-demo
```

Without `--with-processes`, HyperBricks executes neither hooks nor services. You
own the API terminal and must stop it yourself. If the API is absent, the page
shows an explanatory message through its normal API-error template.

## Try a failure

Stop the previous session before each exercise:

```sh
HOOKS_DEMO_FAIL_PREPARE=1 /tmp/hyperbricks-hooks start \
  -m development-hooks-demo --with-processes
```

Preparation fails and startup stops before launching the API or serving HTTP.

```sh
HOOKS_DEMO_FAIL_VERIFY=1 /tmp/hyperbricks-hooks start \
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

After building the CLI from this checkout, run from the repository root:

```sh
python3 modules/development-hooks-demo/tests/smoke.py \
  --binary /tmp/hyperbricks-hooks
```

The standard-library-only suite makes temporary module copies and allocates
separate loopback ports. It checks the rendered API data, site-port propagation,
source reload without another API or preparation run, Ctrl+C exit `130`, failed
preparation, failed verification, and a required API process exiting. Every
scenario checks listener and child-process cleanup. The fixture includes bounded
fallback cleanup for failed assertions and does not adopt an existing service.
