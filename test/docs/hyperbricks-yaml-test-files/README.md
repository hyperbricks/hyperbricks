# HyperBricks YAML Fixture Set

This directory is the migration fixture set for the new HyperBricks YAML
profile.

Migration rules for this directory:

- keep assets local under `assets/`
- use `.hyperbricks.yaml.test` as the executable fixture/documentation format
- keep source-order cases explicit
- include inheritance, nested values, imports, arrays, route composites,
  template values, guards, API render configs, and collision/reserved-name
  cases
- do not add legacy `.hyperbricks` fixtures; YAML readable fixtures are the
  executable documentation corpus
- do not add detached JSON golden files; expected materialized/runtime JSON
  belongs inside the readable `.hyperbricks.yaml.test` fixture
- avoid generated numeric child keys such as `.10`, `.20`, `.30`

Readable executable cases can use the same section style as the legacy docs
tests:

```text
==== hyperbricks yaml {!{page}} ====
...
==== explainer ====
...
==== expected json ====
...
==== expected output ====
...
```

The `{!{page}}` value is the selected root object for the expected JSON and
rendered output checks.

## Core Corpus

The first migration corpus is intentionally scenario-based and uses readable
executable cases:

- `text-html-tree.hyperbricks.yaml.test`
- `template-values-data.hyperbricks.yaml.test`
- `inheritance-override.hyperbricks.yaml.test`
- `ordered-children.hyperbricks.yaml.test`
- `nested-tree-3-level.hyperbricks.yaml.test`
- `head-assets.hyperbricks.yaml.test`
- `head-generated-items.hyperbricks.yaml.test`
- `hypermedia-route.hyperbricks.yaml.test`
- `fragment-response.hyperbricks.yaml.test`
- `menu-items.hyperbricks.yaml.test`
- `api-render-request.hyperbricks.yaml.test`
- `reserved-name-collision.hyperbricks.yaml.test`

These fixtures are the first layer of proof for the new YAML profile and can
also serve as compact documentation examples.
