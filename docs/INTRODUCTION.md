# Introduction

HyperBricks is a Go-based system that can **build and serve** web applications from small, modular configuration files (`.hyperbricks`). It was designed to ship **fast, high-quality hypermedia apps** (full pages + HTMX fragments) without pulling in a large JavaScript framework just to get structure, templating, routing, and build tooling.

At a high level: you describe **what your site is** as a hierarchy of components, HyperBricks wires the output, and it can render either **statically** or **dynamically**.

## HyperBricks started from a practical standpoint:

* A **granular templating system** that stays readable and modular as projects grow
* Great **runtime performance** (Go’s templating + concurrency)
* First-class integration with **HTMX**
* Optional modern JS bundling for **TypeScript / ESM modules** (without “JS dependency clutter”)
* Tailwind CLI integration
* A simple “native component” model that stays extensible through plugins
* **Bi-directional SSR**: render API → HTML and return either full pages or HTMX fragments

The guiding idea is to keep the authoring model *simple*, while still supporting advanced composition and data-driven rendering.

---

## The mental model

HyperBricks projects are built from small `.hyperbricks` files. Think of each file as a module of configuration that defines components and how they nest.

### 1) Components are the building blocks

HyperBricks uses **two kinds of components**:

#### Standard components (leaf nodes)

Leaf components render only what you put in them.

Examples: `<HTML>`, `<TEXT>`, `<IMAGE>`, `<CSS>`, `<JS>`, `<JSON>`, `<MENU>`, `<PLUGIN>`

```hyperbricks
intro = <TEXT>
intro.value = Welcome to HyperBricks!
```

#### Composite components (structural nodes)

Composite components contain other components and define how output is assembled.

Examples: `<HYPERMEDIA>`, `<FRAGMENT>`, `<TREE>`, `<TEMPLATE>`, `<API_RENDER>`, `<API_FRAGMENT_RENDER>`

```hyperbricks
myfragment = <FRAGMENT>
myfragment.10 = <HTML>
myfragment.10.value = <p>Fragment content 1</p>
myfragment.20 = <HTML>
myfragment.20.value = <p>Fragment content 2</p>
```

---

## Root types and routes

Some composite components are **Root Types**. Root types initiate frontend output and typically correspond to a **route**.

### Root Types

* `<HYPERMEDIA>` — full-page documents, controls `<head>`, `<body>`, routing
* `<FRAGMENT>` — HTMX-powered partial responses
* `<API_FRAGMENT_RENDER>` — authenticated API fragment proxy (returns HTMX-ready responses)

A minimal root looks like this:

```hyperbricks
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.title = Welcome!

hypermedia.10 = <HTML>
hypermedia.10.value = <p>Hello from HyperBricks.</p>
```

This creates a route at `/index` with a title and body output.

---

## Aggregation types (composition tools)

Aggregation types are composite components you use to structure and reuse output.

* `<API_RENDER>` — fetch and render public API data (cache-friendly patterns)
* `<TREE>` — hierarchical/nested components
* `<TEMPLATE>` — reusable Go template logic (with Sprig extensions)

These are the “glue” that lets you scale beyond single-page config files.

---

## Modular configuration: `@import`

By default, HyperBricks loads `.hyperbricks` files from the module’s `hyperbricks/` directory. Files in subfolders are typically **not loaded unless you import them**.

Use `@import` to include external config *inline*:

```hyperbricks
@import "plugins/esbuild.hyperbricks"
@import "page/menu.hyperbricks"
```

Best practice: keep your configs small and reusable—split plugins, themes, menus, and page fragments into separate files and import as needed.

---

## Dynamic generation: `@macro`

Macros let you generate repeated config blocks from a compact “table + template” form. Most projects don’t need it early, but it becomes useful for repeated route definitions, menus, mappings, etc.

```hyperbricks
@macro as (index, title, route, doc) {
1|Introduction|introduction_fragment|introduction
2|Quickstart|quickstart_fragment|quickstart
} = <<<[
    {{{.route}}} < docs_fragment
    {{{.route}}} {
        index = {{{.index}}}
        route = {{{.route}}}
        title = {{{.title}}}

        10.data.source = {{RESOURCES}}/docs/{{{.doc}}}.md
    }
]>>>
```

---

## Where JavaScript fits

HyperBricks does **not** require a heavy JS framework. You can build highly interactive experiences using **HTMX** and fragments, and only add JavaScript where it’s clearly useful.

If you *do* want modern JS/TS bundling, the ecosystem supports that through plugins (e.g., an esbuild-based workflow). The intent is: **no lock-in**, and no forced dependency sprawl.

---

## What a typical project looks like

When you initialize a module, you get a structure like:

* `hyperbricks/` — your `.hyperbricks` configs
* `templates/` — Go templates (if you use them)
* `resources/` — source assets (docs, images, content)
* `static/` — static files served as-is
* `rendered/` — build output

HyperBricks then parses your configs, resolves imports/macros, renders, and serves (or builds) concurrently.

---

## Next steps

1. **Quickstart**: create a minimal `<HYPERMEDIA>` route and serve it
2. **Fragments**: add a `<FRAGMENT>` and update it via HTMX
3. **Composition**: extract repeating parts into `<TEMPLATE>` and/or `<TREE>`
4. **Data**: introduce `<API_RENDER>` for public API rendering
5. **Modularity**: split configs with `@import`, use `@macro` only when repetition hurts