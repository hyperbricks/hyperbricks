# Project lifecycle

Use this reference to create, run, troubleshoot, and package an application. Core manuals: `docs/HYPERBRICKS_CLI.md`, `docs/QUICKSTART.md`, `docs/DEPLOY.md`. Resolve those paths using the Source Of Truth rules in [the skill](../SKILL.md).

## Get a working module

Use the version pinned by the project. If no CLI is installed and the user is setting up a new project, the documented Go installation is:

```sh
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
hyperbricks version
```

A released CLI can lag behind a development checkout. When the task needs a local core build, follow that checkout's build instructions and invoke the resulting binary explicitly. Do not claim a native component exists solely because it appears in a newer manual.

From the project root, initialize and start a module:

```sh
hyperbricks init -m demo
hyperbricks start -m demo --port 8080
```

Open `http://localhost:8080/`. The generated module already demonstrates YAML composition and templates. `init` fills missing scaffold files while preserving existing ones. For scripted commands use `--non-interactive` where supported:

```sh
hyperbricks init -m demo --non-interactive
```

Put `--non-interactive` after the subcommand. Some commands dispatch before global flags are parsed, so placing it before `start` can fail.

A project normally contains:

```text
modules/demo/
  package.hyperbricks.yaml   Runtime config and application data
  hyperbricks/               Component YAML; imports for nested directories
  templates/                Go HTML templates
  resources/                Input scripts, CSS, data, and copy
  static/                   Files publicly served at /static/...
  rendered/                 Generated static export
bin/plugins/                Compiled plugins, when the module uses them
```

`init` also creates log directories. Keep generated output and local logs out of the intended source publication set according to the project's existing policy.

Use the maintained `hyperbricks init` scaffold as the starting point for a new project. Continue with [the authoring recipe](authoring.md) for shared pages and fragments, and select runnable examples through the patterns module source guide. Copy source files and declare the receiving project's dependencies; generated binaries and caches are not setup inputs.

## Choose paths deliberately

`-m demo` means `./modules/demo`. Direct `start` additionally accepts paths:

```sh
hyperbricks start -m ./modules/demo
hyperbricks start -m /srv/sites/demo
hyperbricks start -m demo --config profiles/development.hyperbricks.yaml
```

Path selection applies to direct `start`; init, static, build, plugin, and deploy commands retain their documented module-name contracts. `--config` is relative to the selected module, must remain inside it, and cannot combine with `--deploy`. Module selection does not change the working directory. `root` is the command's working directory; `module` is the selected module; `module_root` is its parent.

Prefer explicit module-relative package directories for portable applications:

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    reload: true
  server:
    port: 8080
  directories:
    hyperbricks:
      path: {base: module, path: hyperbricks}
    templates:
      path: {base: module, path: templates}
    resources:
      path: {base: module, path: resources}
    static:
      path: {base: module, path: static}
    render:
      path: {base: module, path: rendered}
    plugins: ./bin/plugins
```

The plugins path above intentionally remains relative to the project root. Merge relevant settings into the generated package; preserve other application configuration and metadata. Package config is ordinary YAML, not component sequences. Top-level custom application data can live under `myconf` and be read with `config: myconf.some.path`.

## Develop and diagnose

Use `hyperbricks start --help` for the chosen runtime's port, debug, and production flags. Development `watch` rebuilds loaded configuration; `reload` updates the browser. Source asset watching has its own `development.watch_dirs` rules in `docs/ESBUILD.md`. Do not assume arbitrary resource directories are watched.

| Symptom | Check the owning input first |
| --- | --- |
| A page is missing | Selected package/source directory; `route`; explicit imports for nested files; startup diagnostics |
| An override appears ignored | Inheritance path and child name; ordered sequence shape; loaded file |
| A template value is empty | Context owner (`.heading`, `.Params.q`, `.Data.name`); source values; explicit query keys |
| CSS/JS is missing | `static` disk directory, `/static/` URL, asset entry path, build error, dependency installation |
| A change remains stale | Selected binary/module; configured watch directories; page cache versus esbuild cache |
| A plugin cannot load | Exact artifact name, project working directory, enabled list, matching runtime/toolchain |

Use the server's startup and request diagnostics. `hyperbricks static --serve` serves previously generated files, so it will not prove a dynamic source change.

## Choose the delivery format

| User needs | Delivery |
| --- | --- |
| Public HTML and assets frozen at build time | Static snapshot |
| Server calculations, forms, protected views, APIs, or plugins at request time | Runtime archive plus a compatible HyperBricks runtime |

Build a static snapshot:

```sh
hyperbricks static -m demo
hyperbricks static -m demo --serve
```

The static command starts a local runtime and requests discovered routes. Nested API reads therefore need a reachable upstream while building. Entries under `hyperbricks.static.routes` and `variants` add targets; they do not restrict discovery to that list. For example, this adds a snapshot target:

```yaml
hyperbricks:
  static:
    routes:
      - path: /about
        output: about.html
```

For a public subset of a dynamic application, stage a separate module configuration whose source directory loads only public, static-ready pages. Check discovered routes and output names for collisions before publishing.

Use `hyperbricks static --help` for export paths, zip, and overwrite flags. A static snapshot does not retain server actions. A static page should use static navigation and avoid controls that still require fragment routes.

Build and run a runtime archive locally:

```sh
hyperbricks build --hra -m demo
hyperbricks start --deploy -m demo --port 8081
```

The default archive/index location is `deploy/<module>/`. `build --zip` packages a runtime zip; it differs from `static --zip`, which packages rendered output. Inspect the archive and request the packaged route, including required assets. Native esbuild needs its source resources and dependencies in a runtime archive unless the application's deployment design explicitly changes that contract.

Remote delivery uses an existing deploy target, credentials, and the deployment manual. Building or testing locally does not authorize `--push` or remote activation.

The optional Docker deploy host runs the Deploy API, accepts HRA uploads, and starts deployed module processes. It builds the current checkout by default; published-release selection is explicit. Archives and compiled global plugins use persistent storage, and plugins can be built inside the container against the matching runtime. Read `docs/DOCKER.md` for setup, the required deploy secret, port mappings, and restart behavior; `docker/README.md` includes the repeatable deploy-chain test. Resolve these paths from the HyperBricks repository root.

## Starters

The official starter workflow is an alternative to `init` when the user wants a published starter:

```sh
hyperbricks init-starter list
hyperbricks init-starter get hello-world -m demo
```

Use the returned compatible version or an explicit published version. The target module directory must be missing or empty. Do not assume an example under `modules/` is also a published starter.

## Runtime gateway

Read `docs/RUNTIME_GATEWAY.md` only for an application that needs isolated runtime views routed by hostname. The gateway is disabled by default. Its resolver must be a trusted internal endpoint; the domain or host suffix must resolve to the HyperBricks host. A typical existing setup uses:

```sh
hyperbricks start -m demo --port 8080 \
  --runtime-gateway \
  --runtime-domain runtime.local \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

`--runtime-host-suffix` supports configured suffixes instead of a domain. CLI flags override package settings under `hyperbricks.server.runtime_gateway`. Startup requires a resolver and a domain or host suffix. This is an advanced hosting feature; it is not part of ordinary module initialization.
