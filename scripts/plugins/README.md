# Plugin build and smoke scripts

These maintainer scripts rebuild native Go plugins against the current
HyperBricks checkout and exercise the plugin-backed patterns module. For the
normal plugin workflow, see [Plugin system](../../docs/PLUGINS.md).

Run the commands below from the repository root. A clean checkout also needs
the pinned browser dependencies before the patterns smoke test:

```sh
npm ci
```

The smoke test requires `curl` and a standalone `tailwindcss` executable on
`PATH`. The root `tailwindcss` v4 package supplies library imports but does not
include the separate Tailwind CLI.

## Build the repository plugins

```sh
scripts/plugins/build_hyperbricks_plugins.sh
```

The default build includes the shared Markdown and Tailwind CSS plugins plus
every versioned custom plugin in these modules:

- `hyperbricks-patterns-yaml`
- `project-lifecycle-test`
- `streaming-demo`
- `unpoly-guard-demo`

A custom plugin is discovered from
`modules/<module>/plugins/<name>/<version>/manifest.json`. Add its owning module
to `DEFAULT_MODULES` in the build script when introducing another module-local
plugin. Native components such as `type: esbuild` do not need a plugin build.

The wrapper uses `go run ./cmd/hyperbricks`, forces `GOWORK=off`, and points
`HYPERBRICKS_LOCAL_PATH` at this checkout so the host and plugins use matching
Go code. It restores each plugin's `go.mod` and `go.sum` after building. Output
is written to the ignored `bin/plugins/` directory; the success marker is
written only after the complete selected build succeeds.

Useful focused commands:

```sh
# Show every command without building.
scripts/plugins/build_hyperbricks_plugins.sh --dry-run

# Build one module's custom plugins only.
scripts/plugins/build_hyperbricks_plugins.sh \
  --module streaming-demo \
  --skip-core

# Repeat --module to select several modules.
scripts/plugins/build_hyperbricks_plugins.sh \
  --module streaming-demo \
  --module unpoly-guard-demo \
  --skip-core

# Build only the shared plugins.
scripts/plugins/build_hyperbricks_plugins.sh --skip-custom
```

The first `--module` replaces the default module list; later occurrences append
to that selection. Run `scripts/plugins/build_hyperbricks_plugins.sh --help`
for every option.

## Run the patterns smoke test

Build the required plugins first, then run:

```sh
scripts/plugins/test_hyperbricks_patterns_plugins.sh
```

The HTTP-level script starts `hyperbricks-patterns-yaml` on port `18092`,
verifies the menu and plugin fragment, Markdown documentation and HTMX
out-of-band markup, and the guarded redirect, login, and authenticated
response. It stops the server and removes temporary files on exit. Override a
busy port with:

```sh
HYPERBRICKS_PLUGIN_TEST_PORT=18093 \
  scripts/plugins/test_hyperbricks_patterns_plugins.sh
```

To run plugin building, smoke coverage, the lifecycle plugin fixture, and the
rest of the repository tests together, use:

```sh
./tests.sh --with-plugins
```
