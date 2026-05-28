# HyperBricks YAML Usage

This document defines the supported HyperBricks YAML source format. It covers
syntax, ordering, inheritance, imports, value resolvers, and recovery behavior.

Component fields such as `route`, `response`, `guard`, `values`, `inline`, and
`endpoint` are documented in [REFERENCE.md](REFERENCE.md).

## Files

Runtime source files use:

```text
*.hyperbricks.yaml
```

Module runtime configuration uses:

```text
package.hyperbricks.yaml
```

Component source files and package configuration both use YAML, but they have
different shapes:

- `*.hyperbricks.yaml` files define ordered HyperBricks component trees.
- `package.hyperbricks.yaml` is normal configuration data under keys such as
  `hyperbricks` and `myconf`.

## Component Source Shape

A component source file is a top-level YAML mapping. Most top-level keys define
named HyperBricks objects.

```yaml
page:
  - type: hypermedia
  - route: index
  - title: Home
  - main:
      - type: tree
      - hero:
          - type: html
          - value: |
              <h1>Hello</h1>
```

Each object is an ordered sequence of single-key entries. Source order matters.

The reserved entries inside a component node are:

| Entry | Meaning |
| --- | --- |
| `type` | Component type, such as `hypermedia`, `tree`, `html`, or `template`. |
| `inherit` | Deep-copy another named object before applying local entries. |

All other entries are fields or child nodes.

## Fields And Children

A scalar or mapping entry is a field:

```yaml
hero:
  - type: html
  - value: <h1>Hello</h1>
  - enclose: <section>|</section>
```

An entry whose value is another ordered component sequence is a child:

```yaml
page:
  - type: hypermedia
  - main:
      - type: tree
      - hero:
          - type: html
          - value: <h1>Hello</h1>
      - intro:
          - type: text
          - value: Welcome
```

The runtime materializes ordered children into the current mapstructure-facing
shape with `@type` and `@order`:

```json
{
  "@type": "<HYPERMEDIA>",
  "route": "index",
  "@order": ["main"],
  "main": {
    "@type": "<TREE>",
    "@order": ["hero", "intro"],
    "hero": {
      "@type": "<HTML>",
      "value": "<h1>Hello</h1>"
    },
    "intro": {
      "@type": "<TEXT>",
      "value": "Welcome"
    }
  }
}
```

Do not write `@type` or `@order` in source YAML. They are runtime keys.

## Types

Use lowercase type names in YAML:

```yaml
- type: hypermedia
- type: fragment
- type: api_render
- type: api_fragment_render
- type: tree
- type: head
- type: template
- type: html
- type: text
- type: css
- type: javascript
- type: js
- type: image
- type: images
- type: json
- type: json_render
- type: menu
- type: plugin
- type: styles
```

Known types are normalized to runtime tokens such as `<HYPERMEDIA>` and
`<TEMPLATE>`.

Unknown types are preserved into the runtime map in normal runtime loading. The
renderer/typefactory layer then reports that no renderer is registered for that
type, while valid sibling components continue to render.

## Ordering

Ordered renderable children come from YAML sequence order.

```yaml
main:
  - type: tree
  - first:
      - type: html
      - value: <p>first</p>
  - second:
      - type: html
      - value: <p>second</p>
```

Maps are data, not render order:

```yaml
card:
  - type: template
  - values:
      title: Hello
      body: Welcome
```

Only component children in ordered node sequences participate in render order.

## Duplicate Child Names

Duplicate child names are not recommended. Runtime loading recovers by assigning
real runtime paths with `_2`, `_3`, and so on, and records diagnostics.

```yaml
main:
  - type: tree
  - text_summary:
      - type: text
      - value: Intro
  - text_summary:
      - type: text
      - value: Summary
```

Materialized paths become:

```text
main.text_summary
main.text_summary_2
```

Those recovered names are real paths and can be referenced by inheritance after
materialization. Authors should still prefer unique semantic names in source.

## Inheritance

Use `inherit` to deep-copy another object and override selected fields.

```yaml
base_card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title: Base title
      body: Base body

page:
  - type: hypermedia
  - route: index
  - featured_card:
      - inherit: base_card
      - values:
          title: Featured
```

Rules:

- Inherited nodes are deep-copied.
- Local fields override inherited fields.
- Nested maps merge recursively.
- Local children with the same name override or extend inherited children.
- New local children are appended in source order.
- References use dotted paths, such as `base_card`, `layout.header`, or
  `page.main.hero`.

## Imports

Use file-level `imports` to load shared objects before the current file.

```yaml
imports:
  - partials/site.hyperbricks.yaml
  - partials/cards.hyperbricks.yaml

page:
  - type: hypermedia
  - route: index
  - hero:
      - inherit: shared_hero
```

Relative import paths are resolved from the importing file's directory. Imported
objects are loaded before the current file. Duplicate top-level object names
across imported files are source errors for that file.

## Vars

Use file-level `vars` for reusable source values.

```yaml
vars:
  page:
    title: Home
    heading: Hello

page:
  - type: hypermedia
  - route: index
  - title:
      var: page.title
  - hero:
      - type: html
      - value:
          format: <h1>%s</h1>
          args:
            - var: page.heading
```

Runtime-provided variables are also available. Current standard variables are:

| Variable | Meaning |
| --- | --- |
| `module_root` | Parent directory containing modules. |
| `root` | Runtime root marker. |
| `module` | Current module directory. |
| `resources` | Current module resources directory. |
| `templates` | Current module templates directory. |
| `static` | Current module static directory. |
| `hyperbricks` | Current module hyperbricks source directory. |
| `render` | Current module render output directory. |

## Value Resolvers

Resolvers are YAML mappings that materialize into scalar or structured values
before runtime rendering.

### `var`

Resolve a value from file-level or runtime variables.

```yaml
title:
  var: page.title
```

With default:

```yaml
title:
  var:
    name: page.title
    default: Untitled
```

Missing vars resolve to an empty string unless `default` is provided.

### `env`

Resolve an environment variable.

```yaml
token:
  env: API_TOKEN
```

With default:

```yaml
label:
  env:
    name: CTA_LABEL
    default: Start now
```

With required diagnostic:

```yaml
token:
  env:
    name: API_TOKEN
    required: true
```

Missing required env values produce an error-level diagnostic and resolve to an
empty string so the runtime can keep rendering what it can.

### `config`

Resolve a dotted path from runtime configuration.

```yaml
title:
  config: myconf.site.title
```

With default:

```yaml
title:
  config:
    path: myconf.site.title
    default: Untitled
```

### `path`

Resolve a path from a base and a relative path.

```yaml
asset:
  path:
    base: static
    path: css/app.css
```

Equivalent with parts:

```yaml
asset:
  path:
    base: resources
    parts:
      - images
      - logo.svg
```

Supported bases are:

```text
module_root
root
module
resources
templates
static
hyperbricks
render
```

A plain string path is returned as-is:

```yaml
href:
  path: /docs/index.html
```

### `file`

Read a file and use its contents as the value.

```yaml
body:
  file:
    base: resources
    path: copy/intro.html
```

Missing files produce a warning diagnostic and resolve to an empty string.

### `format`

Format a string with resolved arguments. HyperBricks uses Go `fmt.Sprintf`
semantics.

```yaml
value:
  format: <a href="%s">%s</a>
  args:
    - var: links.home
    - config: myconf.navigation.home_label
```

Arguments can use other resolvers.

### `template.file`

The `template` field has a special file resolver. It preloads the template file
from the module templates directory and keeps the template name in the runtime
config.

```yaml
card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title: Hello
```

This replaces the old template marker style. Go template syntax inside template
files or inline template strings is not interpreted by the YAML resolver.

## Multiline Strings

Use YAML block scalars for multiline HTML, CSS, JavaScript, JSON, or text.

```yaml
inline:
  - type: template
  - inline: |
      <section>
        <h2>{{.title}}</h2>
        <p>{{.body}}</p>
      </section>
```

Comments inside block scalars are part of the string:

```yaml
value: |
  <!-- this remains HTML content -->
  <p>Hello</p>
```

Comments outside block scalars are YAML comments and are ignored:

```yaml
# this is ignored by YAML
text:
  - type: text
  - value: Hello
```

## Scalars And Type Conversion

HyperBricks keeps YAML scalar source values as strings during YAML parsing.
Component decoding then converts strings into typed fields where the component
expects `bool`, `int`, string slices, or maps.

These are valid:

```yaml
image:
  - type: image
  - width: 800
  - quality: "85"

fragment:
  - type: fragment
  - nocache: true
```

Quote values when YAML would otherwise treat them as syntax or when preserving
the exact string matters:

```yaml
hx_target: "#status"
bodytag: '<body data-page="home">|</body>'
```

## Template Syntax

Go `html/template` and Sprig syntax stays literal in YAML values. The YAML
pipeline does not resolve Go template expressions.

```yaml
card:
  - type: template
  - inline: |
      <h2>{{.title}}</h2>
      <p>{{ .body | upper }}</p>
```

## Unsupported Source Features

The HyperBricks YAML source profile does not support YAML anchors or aliases.
Use `inherit` for component reuse.

```yaml
# unsupported
base: &base
  - type: html

copy: *base
```

The YAML source profile also does not support the old macro system.

The old uppercase marker forms such as `{{VAR:...}}`, `{{ENV:...}}`,
`{{FILE:...}}`, and `{{TEMPLATE:...}}` are not YAML resolvers. Use the resolver
mappings documented above.

## Recovery And Diagnostics

HyperBricks treats user-authored runtime config like browser input: it tries to
render what it can and reports diagnostics for the rest.

Current behavior:

| Situation | Runtime behavior |
| --- | --- |
| Invalid YAML syntax in one file | That source file is skipped; other valid sources can still load. |
| Unknown component type | Node stays in the runtime map; renderer reports no registered type. |
| Duplicate child names | Runtime recovers with `_2`, `_3`, etc. and reports diagnostics. |
| Missing var/env/config/file resolver | Value resolves to empty string or default and reports diagnostics. |
| Bootstrap infrastructure failure | Startup can still fail when required directories, listener, watcher, or gateway config are structurally unusable. |

Render diagnostics include request/source context such as file, path, key, type,
and message where available.

## Package Configuration

`package.hyperbricks.yaml` is normal YAML configuration, not an ordered component
tree.

```yaml
vars:
  module: modules/demo

myconf:
  demo:
    title: YAML Runtime Fixture

hyperbricks:
  mode: development
  directories:
    render:
      path:
        base: module
        path: rendered
    templates:
      path:
        base: module
        path: templates
    hyperbricks:
      path:
        base: module
        path: hyperbricks
```

The same resolver model is available for configuration values. `vars` is used
for resolver input and is not copied into the materialized configuration.
