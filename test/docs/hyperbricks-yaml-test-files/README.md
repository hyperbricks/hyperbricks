# HyperBricks YAML Fixture Set

This directory is the migration fixture set for the new HyperBricks YAML
profile.

The legacy documentation/runtime fixture set remains in:

```text
test/docs/hyperbricks-test-files/
```

Do not mutate that legacy directory for the DSL migration. Convert and expand
fixtures here instead, so old runtime behavior stays covered while the new YAML
parser, materializer, and later renderer integration get their own corpus.

Migration rules for this directory:

- keep assets local under `assets/`
- prefer `.hyperbricks.yaml` for new YAML profile fixtures
- keep source-order cases explicit
- include inheritance, nested values, imports, arrays, route composites,
  template values, guards, API render configs, and collision/reserved-name
  cases
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

The first migration corpus is intentionally scenario-based:

- `text-html-tree.hyperbricks.yaml`
- `template-values.hyperbricks.yaml`
- `inheritance-override.hyperbricks.yaml`
- `ordered-children.hyperbricks.yaml`
- `nested-tree-3-level.hyperbricks.yaml`
- `head-assets.hyperbricks.yaml`
- `hypermedia-route.hyperbricks.yaml`
- `fragment-response.hyperbricks.yaml`
- `menu-items.hyperbricks.yaml`
- `api-render-request.hyperbricks.yaml`
- `reserved-name-collision.hyperbricks.yaml`

These fixtures are the first layer of proof for the new YAML profile. They are
not generated docs examples yet.

Readable executable cases mirror the core corpus one-to-one:

- `api-render-request.hyperbricks.yaml.test`
- `fragment-response.hyperbricks.yaml.test`
- `head-assets.hyperbricks.yaml.test`
- `hypermedia-route.hyperbricks.yaml.test`
- `inheritance-override.hyperbricks.yaml.test`
- `menu-items.hyperbricks.yaml.test`
- `nested-tree-3-level.hyperbricks.yaml.test`
- `ordered-children.hyperbricks.yaml.test`
- `reserved-name-collision.hyperbricks.yaml.test`
- `template-values.hyperbricks.yaml.test`
- `text-html-tree.hyperbricks.yaml.test`

Comparable corpus fixtures also have a `.legacy.hyperbricks` pair. Those files
are used only to prove semantic parity with the existing parser where the old
DSL can express the same structure cleanly.
