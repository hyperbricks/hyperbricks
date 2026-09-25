{{/* Source template for the generated repository README.md. */}}
{{define "main"}}<!-- Generated from test/docs/readme.md by scripts/build_docs.sh. Do not edit README.md directly. -->

**Licence:** MIT

**Version:** {{.version}}
{{if .buildtime}}
**Build time:** {{.buildtime}}
{{end}}

HyperBricks is under active development. Features, configuration, and APIs may change as the project evolves. See [CHANGELOG.md](CHANGELOG.md) for details about `{{.version}}`.

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

## HyperBricks

HyperBricks is a native full-stack build system with an integrated rendering engine for hypermedia web applications. You configure and connect components using a declarative, YAML-based language. HyperBricks renders and serves HTML pages, fragments, and application endpoints. It brings the application lifecycle together in one project: compose, build, serve, export, and deploy. HyperBricks provides these workflows through a single command-line executable that also runs the application server.

- **Hybrid rendering:** Choose per page or fragment whether to serve cached output or render it dynamically.
- **Backend logic:** Integrate native Go plugins and server-side JavaScript through components.
- **Frontend assets:** Bundle JavaScript, TypeScript, and CSS with the integrated esbuild component.
- **Static export:** Export your configured application as a ready-to-deploy ZIP archive of rendered HTML and assets.
- **Spaces:** Create inheriting pages with source-defined editable fields for content editing and localization.
- **Deployment:** Build and push applications, compile native Go plugins, and manage remote deployments through dedicated local and remote web interfaces.

HyperBricks includes a development interface with three sections: **Dashboard**, **Spaces**, and **Errors**.

```yaml
home:
  - type: hypermedia
  - route: index
  - title: Hello HyperBricks
  - content:
      - type: html
      - value: <h1>Hello HyperBricks</h1>
```

## Installation

Requires Go 1.26.1 or newer.

Install HyperBricks:

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

Create a demo module:

```bash
hyperbricks init -m demo
```

Start the module:

```bash
hyperbricks start -m demo
```

## New to HyperBricks?

Start with the [Introduction](docs/INTRODUCTION.md) to understand the component model. Follow the [Quickstart](docs/QUICKSTART.md) to create a working module. Continue with [How-to guides](docs/HOWTOS.md) or the focused [YAML pattern examples](modules/hyperbricks-patterns-yaml/README.md). For component fields, see [Component reference](docs/REFERENCE.md).

**Windows limitation:** Native Go plugins (`.so`) cannot be built or loaded when HyperBricks runs directly on Windows. This restriction comes from Go's plugin system and does not apply to the separate WebAssembly (`.wasm`) plugin format. See [plugin platform support](docs/PLUGINS.md#platform-support) for details and the upstream Go reference.

## For agents

The [general HyperBricks skill](SKILLS/hyperbricks/SKILL.md) helps agents apply the same workflows in your project.

## Docs

- **Start:** [Introduction](docs/INTRODUCTION.md), [Quickstart](docs/QUICKSTART.md), [How-to guides](docs/HOWTOS.md), and [Troubleshooting](docs/TROUBLESHOOTING.md)
- **Application model:** [Routing](docs/ROUTING.md), [Component reference](docs/REFERENCE.md), [Markdown](docs/MARKDOWN.md), [Spaces CMS](docs/SPACES.md), and [Authoring](docs/AUTHOR.md)
- **Logic and assets:** [API Render](docs/API_RENDER.md), [Server Scripts](docs/GOJA_RENDER.md), [Plugins](docs/PLUGINS.md), and [JavaScript and CSS](docs/ESBUILD.md)
- **Delivery:** [Deploy Guide](docs/DEPLOY.md), [Docker Deploy](docs/DOCKER.md), and [Migration Guide](docs/MIGRATION.md)

---

{{include "template_end_note.md"}}

{{end}}
