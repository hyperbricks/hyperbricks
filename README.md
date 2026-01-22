**Licence:** MIT  
**Version:** v0.7.7-alpha  
**Build time:** 2026-01-22T15:36:50Z

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fhyperbricks%2Fhyperbricks.svg?type=small)](https://app.fossa.com/projects/git%2Bgithub.com%2Fhyperbricks%2Fhyperbricks?ref=badge_small)

## HyperBricks

**HyperBricks** is a fullstack **Web App Build System** for [HTMX](https://htmx.org/)-powered [hypermedia](https://hypermedia.systems/book/contents/) applications. It enables you to build dynamic, modular web applications by describing your app’s state, structure, and behavior in declarative configuration files — called *hyperbricks*.

HyperBricks is designed to provide full control over both the front-end and back-end of an application — without the complexity of traditional fullstack frameworks or CMSs.

With HyperBricks, you can:

* **Design** your application’s structure and interactive behavior using readable, reusable configs
* **Dynamically update** parts of your site without a full page reload (thanks to HTMX)
* **Maintain** full control over templates, routing, and rendering — with no boilerplate or JavaScript lock-in
* **Manage** state and logic for your app in a modular, versionable, and scalable way

> **No JavaScript lock-in** — but if you want, you can still compose NPM packages using the **[esbuild plugin](/plugins.html#esbuild) and serve them however you like.

** [esbuild](https://esbuild.github.io/) is a third-party go library,
An extremely fast bundler for the web

<br>

## Docs

- [Introduction](docs/INTRODUCTION.md)
- [Quickstart](docs/QUICKSTART.md)
- [Routing](docs/ROUTING.md)
- [Reference](docs/REFERENCE.md)
- [Deploy Guide](docs/DEPLOY.md)
- [Plugins](docs/PLUGINS.md)
- [Docker Deploy](docs/DOCKER.md)
- [API Render](docs/API_RENDER.md)

---

<br>

## Disclaimer

This project is a personal experiment, initially built for my own use. You’re welcome to use it however you like, but please be aware that it’s currently in an alpha stage and not recommended for production environments.

The project is released under the [MIT License](https://github.com/hyperbricks/hyperbricks/blob/main/LICENSE) and provided “as-is,” without any warranties or guarantees.

