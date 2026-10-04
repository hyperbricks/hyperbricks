# Build a plugin before HyperBricks loads it

This focused fixture demonstrates a `before_start` hook using the existing
HyperBricks plugin builder. Each build embeds a fresh identifier into the native
plugin. An `after_start` hook fetches the running route and checks that exact
identifier, proving the newly built artifact was loaded.

The simpler API example is [`development-hooks-demo`](../development-hooks-demo/README.md).

## Run it

Use macOS or Linux, Python 3.8+, a Go toolchain matching the checkout, and the
normal native Go plugin build prerequisites described in [Plugins](../../docs/PLUGINS.md).
An initial build may download Go dependencies. Run from this repository root:

```sh
go build -o /tmp/hyperbricks-hooks ./cmd/hyperbricks
/tmp/hyperbricks-hooks start -m development-hooks-plugin-demo --with-processes
```

Open [127.0.0.1:4332](http://127.0.0.1:4332/). The plain-text response reads
`Hooks plugin built before load: <build-id>`. The terminal reports that the loaded
identifier matches this invocation's build. Ctrl+C stops the server.

Use `--port 14332` if port 4332 is occupied. Verification follows the effective
port through `HB_SERVER_PORT`.

## Why the paths matter

[`scripts/build-plugin.sh`](scripts/build-plugin.sh) starts with the selected
module as its working directory. It resolves the repository root through
`HB_MODULE_ROOT`, then invokes:

```sh
"$HB_EXECUTABLE" plugin build hook-greeting@1.0.0 \
  --module development-hooks-plugin-demo
```

This uses the same executable as the server. The script sets
`HYPERBRICKS_LOCAL_PATH` to this checkout because both the plugin and runtime must
use matching local source and dependencies. This fixture is intended to run from
the repository with a binary built from that checkout.

| Item | Location, relative to the repository root |
| --- | --- |
| Source and manifest | `modules/development-hooks-plugin-demo/plugins/hook-greeting/1.0.0/` |
| Generated embedded ID | `modules/development-hooks-plugin-demo/plugins/hook-greeting/1.0.0/build-id.txt` |
| Compiled artifact | `bin/plugins/HooksGreetingPlugin__development-hooks-plugin-demo@1.0.0.so` |
| Runtime plugin directory | `./bin/plugins`, declared in the package |

The existing plugin CLI manages `go.mod` and `go.sum` for the selected runtime.
During a local build it can add an absolute local replacement; inspect those
dependency-file changes before committing. The generated build ID and compiled
artifact are ignored. Compiling plugins is a preparation task; this feature does
not introduce another compiler or relax native plugin compatibility requirements.

`hyperbricks doctor -m development-hooks-plugin-demo` does not execute the build
hook. Before the first successful build it correctly reports a missing plugin
artifact; after the opted-in session builds it, the same inspection can validate
that artifact's presence without running any commands from the package.

## Prove a failed build stops startup

After a successful session has been stopped, leave its compiled artifact in
place and run:

```sh
HOOKS_DEMO_FAIL_PLUGIN_BUILD=1 /tmp/hyperbricks-hooks start \
  -m development-hooks-plugin-demo --with-processes
```

The build hook fails before component/plugin initialization. The old artifact
does not allow the opted-in session to continue. Without `--with-processes`, the
hook is skipped and the runtime uses its normal existing-artifact workflow.

For production, compile during build/package preparation. Deployment-managed
runtimes do not execute development hooks. See the
[feature reference](../../docs/DEVELOPMENT_HOOKS.md#build-a-development-plugin).
