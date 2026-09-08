{{define "main"}}**Licence:** MIT
**Version:** {{.version}}
{{if .buildtime}}
**Build time:** {{.buildtime}}
{{end}}

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)



**HyperBricks** is a fullstack **Web App Build System** [hypermedia](https://hypermedia.systems/book/contents/) applications. It enables you to build dynamic, modular web applications by describing your app’s state, structure, and behavior in declarative configuration files, called *hyperbricks*.

HyperBricks is designed to provide full control over both the front-end and back-end of an application — without the complexity of traditional fullstack frameworks or CMSs.

With HyperBricks, you can:

* **Design** your application’s structure and interactive behavior using readable, reusable configs
* **Dynamically update** parts of your site without a full page reload (thanks to HTMX)
* **Maintain** full control over templates, routing, and rendering — with no boilerplate or JavaScript lock-in
* **Manage** state and logic for your app in a modular, versionable, and scalable way

HyperBricks uses `*.hyperbricks.yaml` configuration files to define pages and HTML
fragments, supply template data, and connect server-side logic. Templates control
the HTML markup using Go’s `html/template` and Sprig functions. 

**YAML is the source format; the runtime contract is component-based.** YAML
source becomes ordered runtime configuration maps, which the runtime uses to
configure and dispatch registered components. When implementing or diagnosing
behavior, distinguish source parsing, runtime configuration, and component
execution, and make changes in the layer that owns the behavior.

**[esbuild](https://esbuild.github.io/)** is a third-party Go library and a fast bundler for web assets.

**No JavaScript lock-in** — The hyperbricks native `esbuild` component bundles JavaScript, TypeScript, and CSS. See the documentation on how to use **[esbuild ](docs/ESBUILD.md)** component.

For server-side logic, projects can call APIs, run trusted JavaScript with `goja_render`, or use Go
plugins. The CLI creates and runs modules, exports static pages, and packages modules for deployment.

<br>

## Docs

New to HyperBricks? Start with the [Quickstart](docs/QUICKSTART.md), then explore
[Project Desk](modules/hyperbricks-basics/README.md), a small application that
connects shared pages, fragments, assets, and server-side calculations. Its
[pattern guide](docs/PROJECT_PATTERNS.md) explains when to use each piece, and
the [general HyperBricks skill](SKILLS/hyperbricks/SKILL.md) helps agents apply the
same workflows in your project.

- [Introduction](docs/INTRODUCTION.md)
- [Quickstart](docs/QUICKSTART.md)
- [Routing](docs/ROUTING.md)
- [Reference](docs/REFERENCE.md)
- [Deploy Guide](docs/DEPLOY.md)
- [Plugins](docs/PLUGINS.md)
- [Server Scripts (Goja Render)](docs/GOJA_RENDER.md)
- [JavaScript and CSS (esbuild)](docs/ESBUILD.md)
- [Docker Deploy](docs/DOCKER.md)
- [API Render](docs/API_RENDER.md)

---

{{include "template_end_note.md"}}

{{end}}
