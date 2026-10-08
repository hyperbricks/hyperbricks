# Project lifecycle test fixture

This module tests HyperBricks rendering, API routes, native plugins, static export, and `.hra` deployment. It is an integration test fixture.

## Run the tests

From the repository root:

```sh
python3 scripts/test_project_lifecycle.py
```

The script builds a temporary runtime from this checkout, copies the fixture into temporary projects, and checks their HTTP responses and exports. It requires Python 3.9+ and Go.

Include the native plugin checks with:

```sh
python3 scripts/test_project_lifecycle.py --with-plugin
```

Native plugins require a compatible platform and Go toolchain. The plugin profile is also checked by `./tests.sh --with-plugins`.

Use `--binary /path/to/hyperbricks` to test an existing runtime built from this checkout. Add `--keep` to retain the temporary projects and logs after a successful run.

## Configuration contracts

```sh
python3 scripts/test_configuration_lifecycle.py
```

This focused check stages `package.configuration.hyperbricks.yaml` as the entry package in a disposable project. It imports the static profile and `config/lifecycle.hyperbricks.yaml`; the entry overrides the imported port variable and clears an inherited list. Hooks only record their phase and outcome in `lifecycle-events.jsonl`.

The check covers opt-in execution, successful and failed exports, ZIP completion before `after_static`, finish outcome/exit status, start/restart cancellation, fingerprint cleanup on cache hits and changed builds, failed-build preservation, unrelated asset preservation, archive imports, and missing-import rejection. It accepts `--binary` and `--keep` like the main lifecycle check.

The configuration and process CI workflow runs this profile on Linux and macOS alongside Go contracts for import merge rules, settings tree navigation, source ownership, entry overrides, external edits/moves/deletions, symlink changes, and partial saves. The default all-tests script also runs the profile. See the canonical [package configuration](../../docs/PACKAGE_CONFIGURATION.md) and [CLI settings](../../docs/HYPERBRICKS_CLI.md) documentation for behavior and usage.

## What is tested

| Configuration | Checks |
| --- | --- |
| `package.hyperbricks.yaml` | Pages and fragments, native assets, request-specific `goja_render`, concurrent requests, development reload, production rendering, and `.hra` deployment |
| `package.api.hyperbricks.yaml` | API reads and writes, validation errors, version conflicts, cookies, and route guards |
| `package.plugin.hyperbricks.yaml` | Native plugin builds, configured actions, template rendering, and HTML escaping |
| `package.configuration.hyperbricks.yaml` | Imported settings, lifecycle outcomes, asset cleanup, and archive preservation |
| `package.static.hyperbricks.yaml` | An isolated static route, generated assets, and zip export |

See [Fixture profiles](docs/profiles.md) for the routes and expected responses. The API profile uses a local API that keeps its data in memory; restarting it resets the data.

## Source layout

| Path | Purpose |
| --- | --- |
| `hyperbricks/` | Default pages, fragments, and shared components |
| `profiles/` | Separate API, plugin, and static configurations and templates |
| `fixtures/` | Files added during development reload checks |
| `plugins/lifecycle-test/` | Native plugin source and unit tests |
| `tools/fixture-api/` | Local API for the API and guard checks |
| `tools/stage_source.py` | Copies the fixture into a temporary project |
| `SOURCE_FILES.txt` | Lists the files copied by staging and checked during packaging |

Generated assets, rendered pages, archives, logs, credentials, and caches are excluded from `SOURCE_FILES.txt`. Tests build the outputs and check the archive contents.

## Run a profile manually

Run these commands from the repository root using a runtime built from this checkout.

### Default rendering

```sh
go run ./cmd/hyperbricks start -m project-lifecycle-test --port 8104
```

### API and guards

Start the fixture API in one terminal:

```sh
go run modules/project-lifecycle-test/tools/fixture-api/main.go -port 8099
```

Start HyperBricks in another:

```sh
HYPERBRICKS_LIFECYCLE_API_URL=http://127.0.0.1:8099 \
  go run ./cmd/hyperbricks start -m project-lifecycle-test \
  --config package.api.hyperbricks.yaml
```

### Native plugin

Build the plugin and start its profile with the same local runtime:

```sh
HYPERBRICKS_LOCAL_PATH="$PWD" \
  go run ./cmd/hyperbricks plugin build lifecycle-test@1.0.0 \
  --module project-lifecycle-test

go run ./cmd/hyperbricks start -m project-lifecycle-test \
  --config package.plugin.hyperbricks.yaml
```

`HYPERBRICKS_LOCAL_PATH` selects the local HyperBricks checkout for development builds. See [Local runtime development](../../docs/PLUGINS.md#local-runtime-development) and the [plugin build and smoke scripts](../../scripts/plugins/README.md).

### Static export

Stage the static profile in a disposable project, then export it with a runtime built from this checkout:

```sh
python3 modules/project-lifecycle-test/tools/stage_source.py \
  /tmp/project-lifecycle-static \
  --static-profile

cd /tmp/project-lifecycle-static
hyperbricks static -m project-lifecycle-test --force --zip
```

## Attribution

HTMX **4.0.0** is stored locally at `resources/vendor/htmx-4.0.0.js`, copied unchanged from the npm package's `dist/htmx.esm.js`. Its SHA-256 is `077b8017a057e3e6dd6834d20012f75c6387bde189bbd26172c6b3a853125cbc`. The upstream [0BSD license](static/vendor/HTMX-LICENSE.txt) is included. See the [HTMX website](https://htmx.org/) and [source repository](https://github.com/bigskysoftware/htmx).
