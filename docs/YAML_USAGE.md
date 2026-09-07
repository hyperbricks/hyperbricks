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

### Ordered objects and ordinary mappings

HyperBricks component objects use a specific YAML shape: a name followed by an
ordered sequence. Every leading `-` adds one entry to that object.

```yaml
scripts:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/bundle.min.main.js}
  - cache: true

page:
  - type: hypermedia
  - route: index
  - head:
      - type: head
      - application_script:
          - inherit: scripts
```

In this example, `scripts` and `page` are named HyperBricks objects. The `type`
entry selects the component, entries such as `entry`, `outfile`, `cache`, and
`route` set fields, and `head` contains another component object.
`application_script` is a named child of `head`; it inherits the complete
`scripts` object at that position in the tree. The child name is chosen by the
author and can describe the child's purpose.

The sequence form is part of HyperBricks notation. It preserves the order of
fields, inherited overrides, and renderable children. A component object should
therefore keep the dashes instead of being rewritten as an ordinary YAML mapping.

The value under `path` is different: it is ordinary data passed to a field. YAML
allows that mapping to be written in compact **flow style**:

```yaml
path: {base: resources, path: js/main.js}
```

or in the equivalent **block style**:

```yaml
path:
  base: resources
  path: js/main.js
```

Both forms produce the same path-resolver data. Flow style is convenient for a
short mapping that fits on one line. Block style is easier to read when a mapping
contains more fields or nested values. These data mappings do not control render
order; the surrounding component sequence does.

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

`template.file` can be used anywhere a YAML value named `template` appears, not
only on `<TEMPLATE>` components. This is useful for plugin configuration that
passes a template key to plugin code while still preloading the template content:

```yaml
onboarding_starter_list:
  - type: plugin
  - plugin: projectonboarding
  - data:
      template:
        file: app/partials/onboarding/starter-list.html
```

The plugin receives:

```yaml
data:
  template: app/partials/onboarding/starter-list.html
```

and the template content is available through the runtime template provider
under that same key.

## String Values And Quoting

YAML provides several ways to declare a string. HyperBricks receives the
resulting string, so the best style depends on the characters in the value and
whether line breaks must be preserved.

| Style | Example | Use it for |
| --- | --- | --- |
| Plain | `title: Welcome` | Simple words, paths, and values without YAML punctuation. |
| Single-quoted | `selector: '#status'` | Literal strings, HTML, and JavaScript that may contain double quotes or backslashes. |
| Double-quoted | `message: "Line one\nLine two"` | Strings that need escapes such as `\n`, `\t`, `\"`, or `\\`. |
| Literal block | `value: \|-` | Multiline content whose line breaks must remain intact. |
| Folded block | `value: >-` | Multiline prose that should become a single wrapped line. |

Plain strings are the most readable choice for simple values:

```yaml
title: Estimate calculator
route: estimate
source: js/main.js
```

Add quotes when YAML punctuation could change how the value is read. In a plain
string, `#` can start a comment and a colon followed by a space can start a new
mapping entry. Quotes are also useful when a value such as `true`, `null`, or
`001` is intended to be text and that exact spelling should be obvious.

```yaml
selector: '#estimate-form'
label: 'Status: ready'
enabled_text: 'true'
release_code: '001'
```

Single quotes treat backslashes and double quotes as ordinary characters, which
makes them a good fit for HTML attributes and small template fragments. Write two
single quotes to include one literal apostrophe:

```yaml
enclose: '<script src="|" defer></script>'
message: 'It''s ready'
windows_path: 'C:\assets\main.js'
```

Double quotes interpret YAML escape sequences. Use them when the string needs an
escaped newline, tab, quote, or backslash:

```yaml
message: "First line\nSecond line"
quoted_word: "Say \"ready\""
empty_value: ""
```

If a value needs neither quoting nor escapes, plain, single-quoted, and
double-quoted forms produce the same text. Quoting controls YAML parsing; it does
not add quote characters to the value received by HyperBricks.

### Multiline strings

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

The literal marker `|` preserves line breaks. The folded marker `>` replaces
most line breaks with spaces, which is useful for long prose:

```yaml
summary: >-
  HyperBricks keeps this readable in YAML
  and receives it as one line of text.
```

By default, a block scalar keeps one final newline. Add `-` to strip that final
newline (`|-` or `>-`), or add `+` to preserve all trailing blank lines (`|+` or
`>+`). For HTML, CSS, JavaScript, templates, and other code, prefer `|` or `|-`
because folding can change the content. The indentation below the marker defines
which lines belong to the string.

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

The string styles described above do not prevent component decoding. For
example, a typed numeric or boolean component field can be converted from either
a plain or quoted scalar when its value is valid for that field.

## Template Syntax

HyperBricks templates use Go `html/template`. The
[template helper](../pkg/shared/helpers_templating.go) registers Sprig v3's
`GenericFuncMap()` and adds the HyperBricks helpers `safe`, `random`, and
`valueOrEmpty`. See the [Sprig function reference](https://masterminds.github.io/sprig/)
for the complete function list. Template syntax stays literal in YAML values;
the YAML pipeline does not resolve Go template expressions.

```yaml
card:
  - type: template
  - inline: |
      <h2>{{.title}}</h2>
      <p>{{ .body | upper }}</p>
```

### Using Sprig functions

You can call a function directly or build a pipeline with `|`. A pipeline reads
from left to right and passes its result as the final argument to the next
function. For example, `{{ .tags | join ", " }}` is equivalent to
`{{ join ", " .tags }}`.

This example shows four common uses: cleaning text, providing a default,
sorting and joining a list, and calculating a price:

```yaml
product_summary:
  - type: template
  - inline: |
      <article>
        <h2>{{ .title | trim | title }}</h2>
        <p>{{ .description | default "Description coming soon." }}</p>
        <p>Tags: {{ .tags | sortAlpha | join ", " }}</p>
        <p>Total: €{{ mulf .unit_price .quantity | printf "%.2f" }}</p>
      </article>
  - values:
      title: "  starter kit  "
      description: ""
      tags:
        - yaml
        - htmx
        - go
      unit_price: 19.95
      quantity: 3
```

The rendered values are `Starter Kit`, the default description,
`go, htmx, yaml`, and `€59.85`. Sprig's `default` function considers `0`,
`false`, empty strings, empty lists and maps, and `null` to be empty. When zero
or false is a meaningful value, check it explicitly instead of replacing it
with `default`.

Sprig helpers format or transform values inside the template; they do not add
new data to the template context. Keep application logic in components,
`goja_render`, or plugins, and use template functions for presentation tasks.
Function output is still escaped by Go's `html/template`. The HyperBricks
`safe` helper bypasses that escaping, so use it only for HTML you already trust.

Use YAML lists and maps directly when the data belongs to the template context:

```yaml
card_list:
  - type: template
  - inline: |
      <ul>
      {{range .cards}}
        <li>{{.title}}: {{.body}}</li>
      {{end}}
      </ul>
  - values:
      cards:
        - title: First
          body: Rendered from YAML data
        - title: Second
          body: Still ordinary template data
```

Use YAML block scalars when the template itself spans multiple lines.

Templates expose allowlisted request query parameters through `.Params`
independently of `values`. Omit `querykeys` to use the default allowlist, use an
empty list to expose none, or list the accepted keys explicitly. A key with one
value is a string; repeated values are a list.

```yaml
search_result:
  - type: template
  - querykeys: [q]
  - inline: '<p>Search: {{.Params.q}}</p>'
```

For a request such as `?q=hypermedia`, this renders the query value without an
otherwise unnecessary `values: {}` field. The separate `queryparams` field is
reserved and does not currently add values to `.Params`.

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

## YAML Errors And Recovery

HyperBricks reports errors in YAML sources and continues loading files that can
be parsed. The response depends on the kind of error:

| Situation | Runtime behavior |
| --- | --- |
| Invalid YAML syntax in one file | That source file is skipped; other valid sources can still load. |
| Unknown component type | Node stays in the runtime map; renderer reports no registered type. |
| Duplicate child names | Runtime recovers with `_2`, `_3`, etc. and reports diagnostics. |
| Missing var/env/config/file resolver | Value resolves to empty string or default and reports diagnostics. |

## Test Corpus

Executable YAML fixtures live in:

```text
test/docs/hyperbricks-yaml-test-files/
```

Those fixtures document source input, materialized JSON, expected diagnostics
where relevant, and rendered output.

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
  development:
    watch: true
    reload: true
    frontend_errors: false
  live:
    cache: 10m
  server:
    port: 8080
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: true
  rate_limit:
    enabled: true
    requests_per_second: 100
    burst: 500
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

Common `hyperbricks` package fields:

| Field | Purpose |
| --- | --- |
| `mode` | Runtime mode. Supported values are `development`, `live`, and `debug`. Invalid values fall back to live mode. |
| `development.watch` | Watch source directories in development mode. |
| `development.reload` | Enable development reload behavior. |
| `development.frontend_errors` | Render frontend error panels when component `debugpanel` is enabled. |
| `live.cache` | Default live-mode cache duration. Uses Go duration strings such as `10s`, `5m`, or `2h`. |
| `server.port` | HTTP server port, unless overridden by CLI flags. |
| `server.beautify` | Beautify rendered HTML when supported. |
| `server.self_closing_tags` | Render XHTML-style self-closing tags when enabled. |
| `server.read_timeout`, `server.write_timeout`, `server.idle_timeout` | HTTP server timeout durations. |
| `server.keep_alives_enabled` | Enable or disable HTTP keep-alive connections. |
| `server.routing` | Clean URL and extension routing settings. See [Routing](ROUTING.md). |
| `server.runtime_gateway` | Runtime host gateway settings. See [Runtime Gateway](RUNTIME_GATEWAY.md). |
| `rate_limit.enabled` | Enable the request rate limiter. Defaults to `true`; set it to `false` only when another layer owns rate limiting or for controlled measurements. |
| `rate_limit.requests_per_second`, `rate_limit.burst` | Token-bucket request rate and burst settings used when the limiter is enabled. |
| `plugins.enabled` | Plugin config names to preload, without `.so`. See [Plugins](PLUGINS.md). |
| `plugins.config` | Optional plugin-specific config map. |
| `directories` | Module directory locations. Resolver path objects are supported here. |
| `logger.level`, `logger.path` | File logging settings. |

The default module layout is:

```text
modules/<name>/
  hyperbricks/
  rendered/
  resources/
  static/
  templates/
  package.hyperbricks.yaml
```

Directory roles:

| Directory | Purpose |
| --- | --- |
| `hyperbricks` | Runtime source files. The runtime scans `*.hyperbricks.yaml` files in this directory. |
| `templates` | Go `html/template` files used by `template.file` and other template providers. |
| `resources` | Source assets or data that can be read through `file` and path resolvers. |
| `static` | Public files served directly by the runtime. |
| `rendered` | Static output written by `hyperbricks static`. |

> Note: `/static/somefile.ext` serves `somefile.ext` from the configured
> `hyperbricks.directories.static` directory, regardless of its name or `base`.
> A custom path does not require an additional directory named `static`.

Subdirectories below `hyperbricks/` are not loaded automatically. Add a root
source file and load shared files with `imports`.
