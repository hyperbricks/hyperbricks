{{/* Source template for the generated repository README.md. */}}
{{define "main"}}<!-- Generated from test/docs/readme.md by scripts/build_docs.sh. Do not edit README.md directly. -->

**Licence:** MIT
**Version:** {{.version}}
{{if .buildtime}}
**Build time:** {{.buildtime}}
{{end}}

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

## HyperBricks

**HyperBricks** is a fullstack **Web App Build System and component runtime for hypermedia applications**. It enables you to build dynamic, modular web applications by describing your app’s state, structure, and behavior in declarative configuration files, called *hyperbricks*.

HyperBricks is designed to provide full control over both the front-end and back-end of an application — without the complexity of traditional fullstack frameworks or CMSs.

With HyperBricks, you can:

* **Design** your application’s structure and interactive behavior using readable, reusable configs
* **Dynamically update** parts of your site without a full page reload with HTML fragments and custom HTTP response headers
* **Maintain** full control over templates, routing, and rendering — with no boilerplate or JavaScript lock-in
* **Manage** state and logic for your app in a modular, versionable, and scalable way

HyperBricks uses `*.hyperbricks.yaml` configuration files to define pages and HTML fragments, supply template data, and connect server-side logic. Templates control the HTML markup using Go’s `html/template` and Sprig functions.

**YAML is the source format; the runtime contract is component-based.** YAML source becomes ordered runtime configuration maps, which the runtime uses to configure and dispatch registered components such as `hypermedia`, `fragment`, and `template`.

HyperBricks’ native `esbuild` component bundles JavaScript, TypeScript, and CSS. See the [esbuild component documentation](docs/ESBUILD.md) for usage. It uses [esbuild](https://esbuild.github.io/), a third-party Go library for fast web asset bundling.

For server-side logic, projects can call APIs, run trusted JavaScript with `goja_render`, or use Go plugins. The CLI creates and runs modules, exports static pages, and packages modules for deployment.

**Windows limitation:** Native Go plugins (`.so`) cannot be built or loaded when HyperBricks runs directly on Windows. This restriction comes from Go's plugin system and does not apply to the separate WebAssembly (`.wasm`) plugin format. See [plugin platform support](docs/PLUGINS.md#platform-support) for details and the upstream Go reference.

<br>

## Docs

New to HyperBricks? Start with the [Quickstart](docs/QUICKSTART.md) and create the maintained starter with `hyperbricks init`. Continue with [Recommended project patterns](docs/PROJECT_PATTERNS.md) or the focused [YAML pattern examples](modules/hyperbricks-patterns-yaml/README.md). The [general HyperBricks skill](SKILLS/hyperbricks/SKILL.md) helps agents apply the same workflows in your project.
- [Introduction](docs/INTRODUCTION.md)
- [Quickstart](docs/QUICKSTART.md)
- [Recommended project patterns](docs/PROJECT_PATTERNS.md)
- [Routing](docs/ROUTING.md)
- [Reference](docs/REFERENCE.md)
- [Deploy Guide](docs/DEPLOY.md)
- [Plugins](docs/PLUGINS.md)
- [Server Scripts (Goja Render)](docs/GOJA_RENDER.md)
- [JavaScript and CSS (esbuild)](docs/ESBUILD.md)
- [Docker Deploy](docs/DOCKER.md)
- [API Render](docs/API_RENDER.md)

Build the complete documentation and skills handbooks as standalone Markdown files:

```shell
python3 scripts/build_markdown_handbooks.py
```

The command reads from the last committed version (`HEAD`) and writes both handbooks to `output/markdown/`. Pass `--ref <commit-or-tag>` to build another committed snapshot, or `--check` to verify that existing outputs match the selected revision.

---

{{include "template_end_note.md"}}

{{end}}
