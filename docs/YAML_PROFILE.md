# YAML Profile

This document summarizes the HyperBricks YAML source profile. The full source
contract is [YAML Usage](YAML_USAGE.md). Component fields are generated in
[Reference](REFERENCE.md).

The profile exists to keep YAML source predictable for people, generated output,
the runtime parser, and external tools.

## Core Shape

A HyperBricks YAML file is a top-level mapping. Top-level keys define named
objects, except reserved file-level keys such as `imports` and `vars`.

```yaml
imports:
  - partials/site.hyperbricks.yaml

vars:
  site:
    title: Example

page:
  - type: hypermedia
  - route: index
  - title:
      var: site.title
  - main:
      - type: tree
      - hero:
          - type: html
          - value: <h1>Hello</h1>
```

Each HyperBricks object is an ordered YAML sequence of single-key mapping
entries. Source order becomes runtime `@order`.

## Entries

Reserved node entries:

- `type` selects the component type.
- `inherit` deep-copies another named object before local overrides apply.

All other entries are fields or child components.

```yaml
base_card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title: Default title

page:
  - type: hypermedia
  - route: reuse
  - main:
      - type: tree
      - card:
          - inherit: base_card
          - values:
              title: Local title
```

## Type Names

Use lowercase YAML type names:

```yaml
- type: hypermedia
- type: fragment
- type: api_render
- type: api_fragment_render
- type: tree
- type: template
- type: head
- type: text
- type: html
- type: image
- type: images
- type: menu
- type: css
- type: styles
- type: javascript
- type: json_render
- type: plugin
```

The materialized runtime map still uses runtime tokens such as `<HYPERMEDIA>`
and `<API_RENDER>`.

## Order

Tree-like render order comes from YAML sequence order.

```yaml
main:
  - type: tree
  - heading:
      - type: html
      - value: <h1>First</h1>
  - body:
      - type: text
      - value: Second
```

Maps are data and should not be used to express render order.

## Data

Arrays and maps inside data fields stay data:

```yaml
cards:
  - type: template
  - values:
      items:
        - title: First
          body: First card
        - title: Second
          body: Second card
```

Only component node sequences create child components.

## Reuse

Use `inherit` for component reuse. HyperBricks inheritance is the supported
reuse model; YAML anchors and aliases are not part of the public profile.

```yaml
warning:
  - inherit: base_card
  - values:
      tone: warning
```

## Resolvers

Resolvers are structured YAML nodes, not string markers.

```yaml
title:
  format: "%s | %s"
  args:
    - var: page.title
    - env: APP_NAME

template:
  file: cards/card.html
```

See [YAML Usage](YAML_USAGE.md) for all supported resolvers.

## Quoting

Use quotes when YAML would otherwise parse a value as another type or comment.

```yaml
response:
  hx_target: "#status"
  hx_reswap: outerHTML

values:
  limit: "3"
  enabled_label: "true"
```

Block scalars are preferred for multiline HTML, CSS, JavaScript, JSON, and text.

```yaml
value: |
  <section>
    <h1>Hello</h1>
  </section>
```

## Diagnostics

The runtime should render what it can and surface configuration or render
problems through diagnostics. Bad user config should not terminate the runtime
process in normal serving flows.

Duplicate child names and generated-name collisions are recoverable diagnostics.
The materialized map uses stable suffixed names so the runtime still has unique
paths.

## Test Corpus

The executable YAML fixtures live in:

```text
test/docs/hyperbricks-yaml-test-files/
```

Those fixtures document source input, materialized JSON, expected diagnostics
where relevant, and rendered output.
