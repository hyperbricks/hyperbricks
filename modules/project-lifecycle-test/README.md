# Project lifecycle test fixture

This module is repository-owned input for HyperBricks integration tests. It verifies that a representative project works through development, production rendering, API-backed routes, a native plugin, static export, and runtime archive deployment.

This is not a starter or a recommended application structure. Create a new project with `hyperbricks init`; use the main documentation and focused pattern modules to learn individual features.

## Test owner

[`scripts/test_project_lifecycle.py`](../../scripts/test_project_lifecycle.py) stages the declared sources into temporary projects, starts local servers on free loopback ports, makes HTTP requests, and removes the temporary workspace after success. Pass `--keep` to retain its diagnostics.

From the repository root, run the server and packaging checks with a source-matched binary:

```sh
python3 scripts/test_project_lifecycle.py \
  --binary "$(command -v hyperbricks)"
```

Include the native Go plugin profile with:

```sh
python3 scripts/test_project_lifecycle.py \
  --binary "$(command -v hyperbricks)" \
  --with-plugin
```

The complete plugin-backed check is also part of:

```sh
./tests.sh --with-plugins
```

Python 3.9+, Go, and a HyperBricks runtime built from this checkout are required. Without `--binary`, the script builds a temporary HyperBricks binary from `./cmd/hyperbricks`.

## Profiles

| Configuration | Coverage | Additional dependency |
| --- | --- | --- |
| `package.hyperbricks.yaml` | Development and production rendering, pages, fragments, native assets, request-bound `goja_render`, concurrency, and source reload | None |
| `package.api.hyperbricks.yaml` | `api_render`, `api_fragment_render`, validation and conflict results, cookies, and route guards | Local fixture API |
| `package.plugin.hyperbricks.yaml` | Native plugin build, configured actions, template handoff, and escaped output | Compatible Go plugin platform and toolchain |
| `package.static.hyperbricks.yaml` | An isolated static route, generated assets, and zip export | Selected by the staging helper |

The detailed route and response contracts are recorded in [Fixture profiles](docs/profiles.md).

## Source layout

| Path | Test responsibility |
| --- | --- |
| `hyperbricks/` | Default pages, fragments, shared components, and request-specific calculation |
| `profiles/api/` | API and guard components loaded by `package.api.hyperbricks.yaml` |
| `profiles/plugin/` | Plugin routes and templates loaded by `package.plugin.hyperbricks.yaml` |
| `profiles/static/` | The only HyperBricks source loaded by the static profile |
| `fixtures/` | Source files copied into a running temporary project to verify development reload |
| `plugins/lifecycle-test/` | Native Go plugin source and unit tests |
| `tools/fixture-api/` | Memory-only loopback API used by the API and guard profile |
| `tools/stage_source.py` | Copies the declared fixture sources into a clean temporary project |
| `SOURCE_FILES.txt` | Explicit source allowlist used by staging and archive checks |

Generated CSS, JavaScript, rendered pages, archives, logs, credentials, and local caches are intentionally absent from `SOURCE_FILES.txt`. The test builds outputs from the source files and checks that runtime archives exclude local or generated material.

## Behaviors under test

The lifecycle check verifies:

- Complete HTML documents for application routes and shell-free fragment responses
- Fingerprinted CSS and JavaScript built by the native `esbuild` component
- Query allowlisting, invalid input, and `Cache-Control: no-store` for a request-specific calculation
- Isolated results during concurrent `goja_render` requests
- Development reload after changing YAML and after adding a page, fragment, and navigation entry
- Fresh request-specific output through the production rendering path
- Clean `.hra` creation and startup through `hyperbricks start --deploy`
- A static profile that exports only its static-ready route and assets
- API reads, writes, validation errors, stale-version conflicts, login cookies, and route guards
- Native plugin compilation, configured action selection, template rendering, and HTML escaping

The fixture deliberately uses several runtime profiles because the test owns those profiles. Public documentation should use small examples centered on one concern.

## Run a profile while diagnosing it

Start the default fixture:

```sh
hyperbricks start -m project-lifecycle-test --port 8104
```

For the API profile, start the memory-only fixture API in one terminal:

```sh
go run modules/project-lifecycle-test/tools/fixture-api/main.go -port 8099
```

Then start HyperBricks in another terminal:

```sh
HYPERBRICKS_LIFECYCLE_API_URL=http://127.0.0.1:8099 \
  hyperbricks start -m project-lifecycle-test \
  --config package.api.hyperbricks.yaml
```

For local core development, build the native plugin against this checkout:

```sh
HYPERBRICKS_LOCAL_PATH="$PWD" \
  go run ./cmd/hyperbricks plugin build lifecycle-test@1.0.0 \
  --module project-lifecycle-test

hyperbricks start -m project-lifecycle-test \
  --config package.plugin.hyperbricks.yaml
```

`HYPERBRICKS_LOCAL_PATH` is development-only and selects the local HyperBricks source used for the plugin build. Leave it unset when building against an installed published release. See [plugin build modes](../../docs/PLUGINS.md#local-runtime-development).

To inspect the isolated static profile in a disposable project:

```sh
python3 modules/project-lifecycle-test/tools/stage_source.py \
  /tmp/project-lifecycle-static \
  --static-profile

cd /tmp/project-lifecycle-static
hyperbricks static -m project-lifecycle-test --force --zip
```

## Attribution

HTMX **4.0.0** is stored locally at `resources/vendor/htmx-4.0.0.js`, copied unchanged from the npm package's `dist/htmx.esm.js`. Its SHA-256 is `077b8017a057e3e6dd6834d20012f75c6387bde189bbd26172c6b3a853125cbc`. The upstream [0BSD license](static/vendor/HTMX-LICENSE.txt) is included. See the [HTMX website](https://htmx.org/) and [source repository](https://github.com/bigskysoftware/htmx).
