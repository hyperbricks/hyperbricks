{{define "main"}}**Licence:** MIT  
**Version:** {{.version}}  
**Build time:** {{.buildtime}}

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

---

<br>

{{include "template_disclaimer_note.md"}}

{{end}}