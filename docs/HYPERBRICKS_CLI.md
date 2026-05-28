# HyperBricks CLI

The `hyperbricks` command initializes modules, starts the runtime, renders
static output, builds deploy archives, and manages plugins.

Run commands from the repository or project root: the directory that contains
`modules/`.

## Install

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

Check the installed version:

```bash
hyperbricks version
```

## Commands

| Command | Purpose |
| --- | --- |
| `init` | Create `package.hyperbricks.yaml` and module directories |
| `init-starter` | Install an official starter module |
| `start` | Start the runtime server |
| `static` | Render static output |
| `build` | Build a deploy archive |
| `plugin` | List, install, build, update, and remove plugins |
| `select` | Select the active module |
| `version` | Show version information |

Use command help for the current flag list:

```bash
hyperbricks <command> --help
```

## Init

Create a module:

```bash
hyperbricks init -m demo
```

This creates:

```text
modules/demo/
  hyperbricks/
  rendered/
  resources/
  static/
  templates/
  package.hyperbricks.yaml
```

The generated module is YAML based and includes a working hello-world route,
template file usage, inline templates, imports, inheritance, resource loading,
and a fragment example.

## Start

Start a module:

```bash
hyperbricks start -m demo
```

Choose a port:

```bash
hyperbricks start -m demo --port 8080
```

Start in production mode:

```bash
hyperbricks start -m demo --production
```

Enable debug logging:

```bash
hyperbricks start -m demo --debug
```

Runtime gateway flags are available on `start`, but the full contract lives in
[Runtime Gateway](RUNTIME_GATEWAY.md).

```bash
hyperbricks start -m demo \
  --runtime-gateway \
  --runtime-domain runtime.local \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

## Static Rendering

Render static output:

```bash
hyperbricks static -m demo
```

Serve rendered static files:

```bash
hyperbricks static -m demo --serve
```

Overwrite existing output:

```bash
hyperbricks static -m demo --force
```

Export rendered output as a zip:

```bash
hyperbricks static -m demo --zip --out exports/demo
```

Exclude paths relative to the render root:

```bash
hyperbricks static -m demo --zip --exclude cache,tmp
```

## Build Archives

Build a deploy archive:

```bash
hyperbricks build --hra -m demo
```

Build a zip archive:

```bash
hyperbricks build --zip -m demo
```

Common build flags:

| Flag | Purpose |
| --- | --- |
| `--out <dir>` | Output directory, default `deploy` |
| `--force` | Build even when no source changes are detected |
| `--replace` | Replace the current build |
| `--replace <build_id>` | Replace a specific build |
| `--push` | Build and push to a deploy target |
| `--target <name>` | Select a deploy target for `--push` |

## Deploy Runtime Commands

Start a module from the deploy folder:

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

Start deploy services:

```bash
hyperbricks start --deploy-remote
hyperbricks start --deploy-local
```

Create a deploy config:

```bash
hyperbricks start --deploy-init-config local
hyperbricks start --deploy-init-config remote
```

## Starters

List compatible starters:

```bash
hyperbricks init-starter list
```

Install a starter:

```bash
hyperbricks init-starter get hello-world -m demo
```

Install a specific starter version:

```bash
hyperbricks init-starter get hello-world@1.0.0 -m demo
```

## Plugins

Plugin management is available under `hyperbricks plugin`.

```bash
hyperbricks plugin list
hyperbricks plugin install example@1.0.0
hyperbricks plugin build example@1.0.0
hyperbricks plugin update example
hyperbricks plugin remove example@1.0.0
```

Use `--module <module>` with `plugin build` or `plugin remove` for custom module
plugins.

See [Plugins](PLUGINS.md) for naming, manifests, and YAML usage.

## Non-Interactive Mode

Use `--non-interactive` when commands run in scripts or CI:

```bash
hyperbricks --non-interactive static -m demo --force
```

This disables keyboard-driven prompts where supported.
