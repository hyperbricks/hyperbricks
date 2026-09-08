**Licence:** MIT
**Version:** v1.2.2-beta

**Build time:** 2026-09-05 19:16 UTC


## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

## HyperBricks

**HyperBricks** is a fullstack **web application build system and component runtime** for [hypermedia](https://hypermedia.systems/book/contents/) applications. It enables you to build dynamic, modular web applications by describing your app’s state, structure, and behavior in declarative configuration files, called *hyperbricks*.

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
configure and dispatch registered components such as `hypermedia`, `fragment`,
and `template`.

HyperBricks’ native `esbuild` component bundles JavaScript, TypeScript, and CSS.
See the [esbuild component documentation](docs/ESBUILD.md) for usage. It uses
[esbuild](https://esbuild.github.io/), a third-party Go library for fast web asset bundling.

For server-side logic, projects can call APIs, run trusted JavaScript with `goja_render`, or use Go
plugins. The CLI creates and runs modules, exports static pages, and packages modules for deployment.

<br>

## Docs

New to HyperBricks? Start with the [Quickstart](docs/QUICKSTART.md), then explore
[Project Desk](modules/hyperbricks-basics/README.md), a small application that
connects shared pages, fragments, assets, and server-side calculations. The [general HyperBricks skill](SKILLS/hyperbricks/SKILL.md) helps agents apply the
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

The project is released under the [MIT License](https://github.com/hyperbricks/hyperbricks/blob/main/LICENSE) and provided “as-is,” without any warranties or guarantees.
