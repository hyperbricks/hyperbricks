<!-- Generated from docs/YAML_USAGE.md. Do not edit directly. -->

# HyperBricks YAML Usage

Use YAML to define HyperBricks components and module settings. This guide covers source syntax, render order, inheritance, imports, value resolvers, and error recovery.

See [Component reference](REFERENCE.md) for fields such as `route`, `response`, `guard`, `values`, `inline`, and `endpoint`.

## Files

Name component source files with this suffix:

```text
*.hyperbricks.yaml
```

Store module runtime settings in:

```text
package.hyperbricks.yaml
```

Component files and package configuration use different YAML structures:

- `*.hyperbricks.yaml` files define ordered HyperBricks component trees.
- `package.hyperbricks.yaml` is normal configuration data under keys such as `hyperbricks` and `myconf`. See [Package Configuration](PACKAGE_CONFIGURATION.md) for runtime settings and defaults.

HyperBricks loads `*.hyperbricks.yaml` files directly inside the configured `hyperbricks/` directory. To load files in subdirectories, add file-level [imports](#imports) to a loaded source file. Import paths are relative to that file.

## Component Source Shape

Start a component source file with a top-level YAML mapping. Most top-level keys name HyperBricks objects.

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

Write each object as an ordered sequence of single-key entries. Their order matters.

### Ordered objects and ordinary mappings

A component object starts with a name, followed by an ordered sequence. Each leading `-` adds one entry to the object.

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

In this example `scripts` and `page` name HyperBricks objects. `type` selects the component. Entries such as `entry`, `outfile`, `cache`, and `route` set its fields. `head` contains a child component.

The child `application_script` inherits the complete `scripts` object at that position in `head`. Choose a child name that describes its purpose.

Keep the dashes when writing a component object. The sequence preserves the order of fields, inherited overrides, and rendered children. An ordinary YAML mapping does not preserve this order contract.

The mapping under `path` holds ordinary field data. You can write it in compact **flow style**:

```yaml
path: {base: resources, path: js/main.js}
```

Or use the equivalent **block style**:

```yaml
path:
  base: resources
  path: js/main.js
```

Both forms produce the same path-resolver data. Use flow style for short mappings on one line. Use block style for more fields or nested values. The surrounding component sequence controls render order. Data mappings do not.

Component nodes reserve two entries:

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

At runtime, HyperBricks converts ordered children into maps for component decoding through mapstructure. It records the component type in `@type` and child order in `@order`:

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

| YAML type | Component reference |
| --- | --- |
| `hypermedia` | [HYPERMEDIA](REFERENCE.md#hypermedia) |
| `fragment` | [FRAGMENT](REFERENCE.md#fragment) |
| `api_render` | [API_RENDER](REFERENCE.md#api_render) |
| `api_fragment_render` | [API_FRAGMENT_RENDER](REFERENCE.md#api_fragment_render) |
| `tree` | [TREE](REFERENCE.md#tree) |
| `head` | [HEAD](REFERENCE.md#head) |
| `template` | [TEMPLATE](REFERENCE.md#template) |
| `html` | [HTML](REFERENCE.md#html) |
| `text` | [TEXT](REFERENCE.md#text) |
| `css` | [CSS](REFERENCE.md#css) |
| `javascript` | [JS](REFERENCE.md#js) |
| `js` | [JS](REFERENCE.md#js) |
| `image` | [IMAGE](REFERENCE.md#image) |
| `images` | [IMAGES](REFERENCE.md#images) |
| `json` | [JSON_RENDER](REFERENCE.md#json_render) |
| `json_render` | [JSON_RENDER](REFERENCE.md#json_render) |
| `menu` | [MENU](REFERENCE.md#menu) |
| `plugin` | [PLUGIN](REFERENCE.md#plugin) |
| `styles` | [STYLES](REFERENCE.md#styles) |
| `markdown` | [MARKDOWN](REFERENCE.md#markdown) |
| `goja_render` | [GOJA_RENDER](REFERENCE.md#goja_render) |
| `esbuild` | [ESBUILD](REFERENCE.md#esbuild) |

HyperBricks converts known types to runtime tokens such as `<HYPERMEDIA>` and `<TEMPLATE>`.

During normal runtime loading, HyperBricks keeps unknown types in the runtime map. The renderer/typefactory layer reports that no renderer is registered for the type. Valid sibling components continue to render.

## Ordering

HyperBricks combines ordered child output in YAML sequence order. Tree children can render concurrently; the sequence defines output order, not a guaranteed order for side effects.

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

The `values` mapping supplies data to a template. Listing `title` before `body` does not make the title render first:

```yaml
card:
  - type: template
  - values:
      title: Hello
      body: Welcome
```

The template decides where `title` and `body` appear in its output. For child components, such as `first` and `second` above, the YAML sequence determines their output order.

## Duplicate Child Names

Prefer unique child names. If names repeat, runtime loading assigns paths with `_2`, `_3`, and so on, and records diagnostics.

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

The runtime creates these paths:

```text
main.text_summary
main.text_summary_2
```

You can reference the recovered paths through inheritance after HyperBricks creates them. Still use unique, descriptive names in source files.

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
- References use dotted paths, such as `base_card`, `layout.header`, or `page.main.hero`.

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

HyperBricks resolves relative import paths from the importing file's directory. It loads imported objects before the current file. Duplicate top-level object names across imported files are source errors for that file.

### Two-level imports

Imports can load files that have their own imports. This example uses two import levels:

```text
modules/demo/hyperbricks/
├── main.hyperbricks.yaml                  # imports partials/site.hyperbricks.yaml
└── partials/
    ├── site.hyperbricks.yaml              # imports cards/card.hyperbricks.yaml
    └── cards/
        └── card.hyperbricks.yaml
```

**`main.hyperbricks.yaml`** imports the shared layout:

```yaml
imports:
  - partials/site.hyperbricks.yaml

page:
  - type: hypermedia
  - route: index
  - main:
      - inherit: site_layout
```

**`partials/site.hyperbricks.yaml`** imports a card from its own directory:

```yaml
imports:
  - cards/card.hyperbricks.yaml

site_layout:
  - type: tree
  - featured_card:
      - inherit: shared_card
```

**`partials/cards/card.hyperbricks.yaml`** defines the card:

```yaml
shared_card:
  - type: html
  - value: <article>Welcome</article>
```

HyperBricks loads `main.hyperbricks.yaml` automatically because it is directly inside `hyperbricks/`. Its imports load the nested files. Each import path starts from the directory of the file containing it: `cards/card.hyperbricks.yaml` resolves under `partials/`, not under `hyperbricks/`.

HyperBricks loads the card, then the layout, then the page. The layout can inherit `shared_card`, and the page can inherit `site_layout`.

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

In component source, you can also use these runtime variables:

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

A value resolver is a YAML mapping that HyperBricks converts to a scalar or structured value before rendering. Block and compact (flow) mappings are equivalent. Keep the ordered sequence (`-`) around component entries.

### `var`

Resolve a value from file-level or runtime variables.

```yaml
title:
  var: page.title
```

To provide a default:

```yaml
title:
  var:
    name: page.title
    default: Untitled
```

Equivalent compact form:

```yaml
title:
  var: {name: page.title, default: Untitled}
```

Missing variables resolve to an empty string unless you provide `default`.

### `env`

Read an environment variable into a component field:

```yaml
cta_label:
  - type: text
  - value:
      env: CTA_LABEL
```

To provide a default:

```yaml
cta_label:
  - type: text
  - value:
      env:
        name: CTA_LABEL
        default: Start now
```

Equivalent compact form:

```yaml
cta_label:
  - type: text
  - value:
      env: {name: CTA_LABEL, default: Start now}
```

To report a missing required value:

```yaml
cta_label:
  - type: text
  - value:
      env:
        name: CTA_LABEL
        required: true
```

Equivalent compact form:

```yaml
cta_label:
  - type: text
  - value:
      env: {name: CTA_LABEL, required: true}
```

If a required environment value is missing, HyperBricks reports an error-level diagnostic and resolves it to an empty string. The runtime continues rendering what it can.

### `config`

Read a dotted path from runtime configuration into a component field:

```yaml
page:
  - type: hypermedia
  - route: index
  - title:
      config: myconf.site.title
```

To provide a default:

```yaml
page:
  - type: hypermedia
  - route: index
  - title:
      config:
        path: myconf.site.title
        default: Untitled
```

Equivalent compact form:

```yaml
page:
  - type: hypermedia
  - route: index
  - title:
      config: {path: myconf.site.title, default: Untitled}
```

### `path`

Resolve a path from a base and a relative path. In a component object, place the resolver under the field that needs the path:

```yaml
stylesheet:
  - type: css
  - file:
      path:
        base: static
        path: css/app.css
```

The resolver supplies the stylesheet path to `file`. You can also write the same path object in compact form:

```yaml
stylesheet:
  - type: css
  - file:
      path: {base: static, path: css/app.css}
```

You can also provide path parts:

```yaml
asset:
  path:
    base: resources
    parts:
      - images
      - logo.svg
```

Equivalent compact form:

```yaml
asset:
  path: {base: resources, parts: [images, logo.svg]}
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

HyperBricks returns a plain string path unchanged:

```yaml
href:
  path: /docs/index.html
```

### `file`

Read a file into a component field. This HTML component uses the file contents as its `value`:

```yaml
intro:
  - type: html
  - value:
      file:
        base: resources
        path: copy/intro.html
```

Equivalent compact form:

```yaml
intro:
  - type: html
  - value:
      file: {base: resources, path: copy/intro.html}
```

If a file is missing, HyperBricks reports a warning diagnostic and resolves the value to an empty string.

### `format`

Format a string with resolved arguments using Go `fmt.Sprintf` semantics.

```yaml
value:
  format: <a href="%s">%s</a>
  args:
    - var: links.home
    - config: myconf.navigation.home_label
```

Equivalent compact form:

```yaml
value: {format: '<a href="%s">%s</a>', args: [{var: links.home}, {config: myconf.navigation.home_label}]}
```

Arguments can use other resolvers.

### `template.file`

The `template` field has a special file resolver. HyperBricks preloads the file from the module templates directory and keeps its name in the runtime config.

```yaml
card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title: Hello
```

Equivalent compact form:

```yaml
card:
  - type: template
  - template: {file: cards/card.html}
  - values:
      title: Hello
```

Use this resolver instead of the old template marker style. The YAML resolver leaves Go template syntax unchanged inside template files and inline strings.

Use `template.file` anywhere a YAML value named `template` appears, including outside `<TEMPLATE>` components. For example, a plugin can receive a template key while HyperBricks preloads the content:

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

The runtime template provider makes the content available under the same key.

## String Values And Quoting

Choose a string style based on its characters and whether you need to keep line breaks. HyperBricks receives the string that YAML produces.

| Style | Example | Use it for |
| --- | --- | --- |
| Plain | `title: Welcome` | Simple words, paths, and values without YAML punctuation. |
| Single-quoted | `selector: '#status'` | Literal strings, HTML, and JavaScript that may contain double quotes or backslashes. |
| Double-quoted | `message: "Line one\nLine two"` | Strings that need escapes such as `\n`, `\t`, `\"`, or `\\`. |
| Literal block | `value: \|-` | Multiline content whose line breaks must remain intact. |
| Folded block | `value: >-` | Multiline prose that should become a single wrapped line. |

Use plain strings for simple values:

```yaml
title: Estimate calculator
route: estimate
source: js/main.js
```

Add quotes when punctuation could change how YAML reads a value. In a plain string, `#` can start a comment. A colon followed by a space can start a mapping entry. Quotes also make the intended spelling clear when values such as `true`, `null`, or `001` should be text.

```yaml
selector: '#estimate-form'
label: 'Status: ready'
enabled_text: 'true'
release_code: '001'
```

Single quotes keep backslashes and double quotes as ordinary characters. Use them for HTML attributes and small template fragments. Write two single quotes for one literal apostrophe:

```yaml
enclose: '<script src="|" defer></script>'
message: 'It''s ready'
windows_path: 'C:\assets\main.js'
```

Double quotes interpret YAML escape sequences. Use them when the string needs an escaped newline, tab, quote, or backslash:

```yaml
message: "First line\nSecond line"
quoted_word: "Say \"ready\""
empty_value: ""
```

If a value needs no quotes or escapes, plain, single-quoted, and double-quoted forms produce the same text. Quotes control YAML parsing. HyperBricks does not receive the surrounding quote characters.

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

Use `|` to preserve line breaks. Use `>` to replace most line breaks with spaces, for example in long prose:

```yaml
summary: >-
  HyperBricks keeps this readable in YAML
  and receives it as one line of text.
```

A block scalar keeps one final newline by default. Add `-` to remove it (`|-` or `>-`). Add `+` to keep all trailing blank lines (`|+` or `>+`).

Use `|` or `|-` for HTML, CSS, JavaScript, templates, and other code. Folding can change code content. Indentation below the marker determines which lines belong to the string.

Comments inside block scalars are part of the string:

```yaml
value: |
  <!-- this remains HTML content -->
  <p>Hello</p>
```

Outside block scalars, YAML ignores comments:

```yaml
# this is ignored by YAML
text:
  - type: text
  - value: Hello
```

## Scalars And Type Conversion

HyperBricks normally keeps YAML scalar source values as strings during parsing. Component decoding then converts them to the expected field type, such as `bool`, `int`, string slices, or maps.

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

Quoting does not prevent component decoding. A numeric or boolean component field can accept a plain or quoted scalar, provided the value is valid for that field.

Some fields preserve and validate their original YAML types before decoding. For example, HTTP response headers require strings, `forwardtoken` requires a cookie-name string, and structured cookie flags require booleans. Follow the component's field contract.

## Template Syntax

HyperBricks templates use Go `html/template`. The [template helper](https://github.com/hyperbricks/hyperbricks/blob/v1.3.0-beta/pkg/shared/helpers_templating.go) registers Sprig v3's `GenericFuncMap()` and adds `safe`, `random`, and `valueOrEmpty`. See the [Sprig function reference](https://masterminds.github.io/sprig/) for the complete list.

Go template expressions stay literal in YAML values. The YAML pipeline does not resolve them.

```yaml
card:
  - type: template
  - inline: |
      <h2>{{.title}}</h2>
      <p>{{ .body | upper }}</p>
```

### Using Sprig functions

Call functions directly or combine them in a pipeline with `|`. A pipeline reads from left to right. It passes each result as the final argument to the next function. For example, `{{ .tags | join ", " }}` is equivalent to `{{ join ", " .tags }}`.

This example shows four uses: cleaning text, supplying a default, sorting and joining a list, and calculating a price:

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

The rendered values are `Starter Kit`, the default description, `go, htmx, yaml`, and `€59.85`. Sprig's `default` treats `0`, `false`, empty strings, empty lists and maps, and `null` as empty. If zero or false is meaningful, check it explicitly instead of using `default`.

Use Sprig helpers to format and transform values already in the template context. They do not add data. Keep application logic in components, `goja_render`, or plugins. Template functions handle presentation.

Go's `html/template` escapes function output. The HyperBricks `safe` helper bypasses escaping. Use it only for HTML you already trust.

Use a keyed mapping for a template collection of objects. The current template renderer preserves lists of strings but drops object entries in a list passed directly through `values`. Go templates visit string map keys in sorted order, so prefixes can set the intended order:

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
        "01_first":
          title: First
          body: Rendered from YAML data
        "02_second":
          title: Second
          body: Still ordinary template data
```

Use YAML block scalars when the template itself spans multiple lines.

Templates expose allowed request query parameters through `.Params`, independently of `values`. Omit `querykeys` to use the default allowlist. Use an empty list to expose none, or list accepted keys explicitly. A single value becomes a string. Repeated values become a list.

```yaml
search_result:
  - type: template
  - querykeys: [q]
  - inline: '<p>Search: {{.Params.q}}</p>'
```

For `?q=hypermedia`, this renders the query value without a `values: {}` field. The separate `queryparams` field is reserved. It does not currently add values to `.Params`.

## Unsupported Source Features

The HyperBricks YAML source profile does not support anchors or aliases. Use `inherit` to reuse components.

```yaml
# unsupported
base: &base
  - type: html

copy: *base
```

The YAML source profile also does not support the old macro system.

Uppercase marker forms such as `{{VAR:...}}`, `{{ENV:...}}`, `{{FILE:...}}`, and `{{TEMPLATE:...}}` are not YAML resolvers. Use the resolver mappings above.

## YAML Errors And Recovery

HyperBricks reports YAML source errors and continues loading files it can parse. Recovery depends on the error:

| Situation | Runtime behavior |
| --- | --- |
| Invalid YAML syntax in one file | That source file is skipped; other valid sources can still load. |
| Unknown component type | Node stays in the runtime map; renderer reports no registered type. |
| Duplicate child names | Runtime recovers with `_2`, `_3`, etc. and reports diagnostics. |
| Missing var/env/config/file resolver | Value resolves to empty string or default and reports diagnostics. |

## Editor Feedback And Common Mistakes

The separately maintained
[HyperBricks VS Code extension](https://github.com/hyperbricks/hyperbricks-vscode)
understands ordered components, inherited types, imports, and block/flow
resolvers. Type `- ` inside a component or `: ` before a value for
context-specific suggestions. Press **Control+Space** to request suggestions
explicitly. Hover fields and resolver keys for help; Cmd/Ctrl-click references
or static file paths to open their source.

A resolver still needs the field's colon and separating space:

```yaml
mydoc:
  - type: text
  - value: {file: {base: resources, path: docs/llms.md}} # read this file
```

Use that `value` entry in place of a literal such as `- value: ok`. Writing
`- value {file: ...}` leaves out the assignment separator; adding braces does
not supply it. The editor reports the missing colon at `value`. The example
requires `docs/llms.md` to exist under the configured resources directory.

Use `#` for comments outside quoted strings and block scalars. In plain text,
separate a trailing comment with whitespace: `value: ok # comment`.
`value: 'ok # literal'` and indented content under `value: |` retain the hash as
text. Syntax colors follow your VS Code theme.

When a valid resolver produces a warning, check its target: `file` reads content,
`path` constructs a path, and `template.file` preloads a template from the
configured templates directory. These are different operations. Fix the first
YAML structural error before interpreting later diagnostics. When testing
editor changes, use extension and executable builds that support the same
HyperBricks editor protocol version.

## Test Corpus

Executable YAML fixtures live in:

```text
test/docs/hyperbricks-yaml-test-files/
```

The fixtures show source input, materialized JSON, expected diagnostics where relevant, and rendered output.

## Package Configuration

Configure runtime modes, developer tools, server settings, caching, plugins, logging, and module directories in `package.hyperbricks.yaml`. See [Package Configuration](PACKAGE_CONFIGURATION.md) for a minimal example, grouped settings, and the module directory layout.

Package files use ordinary YAML mappings; component source uses the ordered trees described in this guide.
