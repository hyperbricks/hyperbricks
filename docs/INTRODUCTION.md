# Introduction

HyperBricks is a Go runtime and build system for hypermedia applications. You describe pages, fragments, templates, data calls, and route behavior in `*.hyperbricks.yaml` files; HyperBricks materializes that configuration into the same runtime component model for serving or static rendering.

The goal is simple: keep the authoring model readable, reusable, and versionable while still giving developers full control over HTML, routing, templates, and deployment.

## Authoring Format

As of `v1.2.0-beta`, HyperBricks uses YAML as its canonical authoring format. Earlier internal configuration experiments have been retired in favor of a single, readable format for modules, routes, components, and examples.

## Mental Model

HyperBricks has three layers:

- YAML source files describe route owners and reusable components.
- The parser materializes those files into ordered runtime configuration maps.
- The runtime registry decodes those maps into components and renders them.

That separation matters. YAML is the source format, but the runtime contract is still component based. A YAML page, fragment, or template must become the same shape the renderer expects.

## Route Owners

Route owners are top-level components that can answer a request.

- `hypermedia` renders full HTML documents.
- `fragment` renders partial HTML responses.
- `api_fragment_render` proxies an API request and renders the response as a fragment.

Example full page:

```yaml
page:
  - type: hypermedia
  - route: index
  - title: Welcome
  - main:
      - type: tree
      - hero:
          - type: html
          - value: |
              <main>
                <h1>Hello HyperBricks</h1>
                <p>This page is rendered from YAML.</p>
              </main>
```

Example fragment for optional hypermedia library [HTMX 4](https://four.htmx.org/):

```yaml
status:
  - type: fragment
  - route: fragments/status
  - response:
      headers:
        HX-Retarget: "#status"
        HX-Reswap: outerHTML
  - body:
      - type: html
      - value: |
          <div id="status">Ready</div>
```

This example assumes HTMX is loaded on the page and requests this route. `HX-Retarget` tells HTMX which element to update; `HX-Reswap: outerHTML` tells it to replace that element, including its wrapper.

HyperBricks renders the fragment and sends the configured headers. HTMX interprets those headers in the browser. The `fragment` component itself does not depend on HTMX, and HyperBricks does not add HTMX headers automatically. See [HTTP responses](HTTP_RESPONSES.md) for the shared status and header contract.

## Components

Components are the building blocks of a route.

Leaf components render their own output. Common examples are `html`, `text`, `image`, `css`, `javascript`, `json_render`, `menu`, and `plugin`.

Composite components contain or transform other components. Common examples are `tree`, `template`, `head`, `api_render`, `fragment`, and `hypermedia`.

The most important rule is that ordered render content belongs in component children. HyperBricks records the YAML sequence order as `@order`, then the runtime uses that order when rendering tree-like structures.

```yaml
page:
  - type: hypermedia
  - route: ordered
  - main:
      - type: tree
      - heading:
          - type: html
          - value: <h1>First</h1>
      - copy:
          - type: text
          - value: Second
```

Maps are data. Trees are ordered content.

## Templates

Templates use Go `html/template` with Sprig functions. Template files can be loaded explicitly through YAML resolvers, or inline content can live directly in the config.

```yaml
card:
  - type: template
  - template:
      file: cards/product.html
  - values:
      title: YAML templates
      body: Template values stay ordinary data.
```

Template syntax is separate from YAML resolver syntax. `{{ .title }}` belongs to Go templates. YAML resolvers are structured YAML nodes such as `file`, `var`, `env`, `format`, and `template.file`.

## Reuse

Reusable objects can live in the same file or be brought in through imports. Inheritance copies the referenced component and then applies local overrides.

```yaml
base_card:
  - type: template
  - template:
      file: cards/simple.html
  - values:
      title: Default title
      body: Default body

page:
  - type: hypermedia
  - route: reuse
  - main:
      - type: tree
      - welcome:
          - inherit: base_card
          - values:
              title: Welcome
              body: This card overrides inherited values.
```

## Runtime Behavior

HyperBricks is intentionally tolerant at runtime. If a user edits a config file and introduces a bad component, the runtime should still render what it can and surface diagnostics for the broken parts.

That browser-like behavior is important for local development and for hosted editing flows. Configuration diagnostics belong in the render diagnostics pipeline, not as process-ending failures.

## Project Layout

A typical module contains:

- `hyperbricks/` for `*.hyperbricks.yaml` source files.
- `templates/` for Go HTML templates.
- `resources/` for source content and images.
- `static/` for files served directly.
- `rendered/` for static output.

Directory locations are configured in `package.hyperbricks.yaml`. See [YAML Usage](YAML_USAGE.md) for the YAML source contract and [Reference](REFERENCE.md) for generated component fields.

## Next Steps

- [Quickstart](QUICKSTART.md) creates a working YAML module.
- [Routing](ROUTING.md) explains route owners and URL matching.
- [YAML Usage](YAML_USAGE.md) documents the YAML syntax and resolvers.
- [Reference](REFERENCE.md) lists runtime component fields.
- [Route Guard](ROUTE_GUARD.md) documents request-time authorization.
