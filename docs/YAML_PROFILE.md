# HyperBricks YAML Profile

Status: draft  
Scope: DSL migration spike

This document defines the first strict YAML profile for the next public
HyperBricks source format. The goal is to preserve the HyperBricks runtime
model while removing the old dotted/numeric DSL surface from new examples and
generated output.

The profile is intentionally small. YAML is the carrier format; HyperBricks
still owns the component model, inheritance rules, imports, and rendering
contract.

## File Shape

A HyperBricks YAML file is a top-level mapping.

```yaml
imports:
  - partials/site.hyperbricks.yaml

page:
  - type: hypermedia
  - route: index
  - hero:
      - type: html
      - value: <h1>Hello</h1>
```

Top-level keys define named HyperBricks objects, except `imports`, which is a
reserved file-level key.

Every HyperBricks object is an ordered sequence of single-key mapping entries.
Sequence order is source order.

## Node Entries

The reserved node entries are:

- `type`: canonical component type, for example `hypermedia`, `template`, or
  `html`
- `inherit`: named object reference to deep-copy before applying local entries

All other scalar or mapping entries are properties.

```yaml
card:
  - type: template
  - template: "{{TEMPLATE:card.html}}"
  - values:
      title: Hello
```

Entries whose value is another ordered node sequence are children.

```yaml
page:
  - type: hypermedia
  - hero:
      - type: html
      - value: <h1>Hello</h1>
```

The parser materializes children into the existing runtime map with an internal
`@order` list:

```json
{
  "page": {
    "@type": "<HYPERMEDIA>",
    "@order": ["hero"],
    "hero": {
      "@type": "<HTML>",
      "value": "<h1>Hello</h1>"
    }
  }
}
```

## Data Arrays

YAML arrays used inside data fields remain arrays. They are not child nodes
unless the array is explicitly a node sequence.

```yaml
card:
  - type: template
  - values:
      cards:
        - title: First
          body: First card
        - title: Second
          body: Second card
```

A sequence is treated as a child node when it contains `type` or `inherit`, or
when it appears directly as a child entry in a node and all entries are
single-key mappings.

Direct scalar arrays remain scalar arrays:

```yaml
api:
  - type: api_render
  - querykeys:
      - topic
      - limit
```

## Inheritance

`inherit` replaces the old reference assignment syntax.

```yaml
base_card:
  - type: template
  - values:
      title: Base title
      theme:
        tone: neutral
        density: compact

page:
  - type: hypermedia
  - card:
      - inherit: base_card
      - values:
          title: Override title
          theme:
            tone: strong
```

Rules:

- inherited nodes are deep-copied
- local properties override inherited properties
- nested maps merge recursively
- local children with the same name override/extend inherited children without
  moving their original order
- new local children are appended at their source position

## Imports

`imports` is a file-level sequence or string.

```yaml
imports:
  - partials/site.hyperbricks.yaml
  - partials/plan.hyperbricks.yaml
```

Import resolution is not part of the first parser slice. The parser records the
imports so a later preprocessor/runtime adapter can resolve them.

## Type Names

Types are written in lowercase canonical form:

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
- type: json
- type: json_render
- type: plugin
```

The materialized runtime map uses the current token form such as
`<HYPERMEDIA>` or `<API_RENDER>`.

Unknown types are rejected by the strict parser unless a future runtime adapter
supplies an extended type registry.

## Reserved Names

Child names must not collide with reserved field names for the parent component.

For example, this is invalid because `<HEAD>` already owns `css` as a property:

```yaml
head:
  - type: head
  - css:
      - type: css
      - inline: |
          body { color: green; }
```

Use a non-colliding child name:

```yaml
head:
  - type: head
  - css:
      - "{{RESOURCES}}/css/base.css"
  - inline_styles:
      - type: css
      - inline: |
          body { color: green; }
```

The current validation is deliberately conservative around known runtime field
names. This avoids mapstructure decoding collisions later.

## Quoting Rules

Use YAML quotes when a string could be parsed as another YAML construct.

Required or recommended:

```yaml
hx_target: "#status"
template: "{{TEMPLATE:card.html}}"
bodytag: '<body data-page="home">|</body>'
limit: "3"
```

Safe unquoted:

```yaml
hx_reswap: outerHTML
route: fragments/status
title: YAML Fixture
```

Composer's emitter should quote strings when they:

- start with YAML-sensitive characters such as `#`, `{`, `[`, `&`, `*`, `!`,
  `|`, `>`, `@`, `-`, `?`, or `:`
- contain ambiguous `: ` sequences
- must remain strings even if they look numeric or boolean
- contain template markers such as `{{...}}`
- contain HTML with quotes or leading/trailing whitespace

## Unsupported YAML Features

The strict profile rejects YAML aliases and anchors for HyperBricks source.
They make authoring state harder to reason about and overlap with
HyperBricks' own `inherit` model.

```yaml
base: &base
  - type: html

copy: *base
```

Use `inherit` instead.

## Current Test Corpus

The first corpus lives in:

```text
test/docs/hyperbricks-yaml-test-files/
```

It includes materialized JSON goldens and legacy-parity fixtures where the old
DSL can express the same structure cleanly.
