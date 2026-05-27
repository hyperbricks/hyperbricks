# YAML ordered source model

Date: 2026-05-27

## Problem

The current public HyperBricks DSL can be framed as TypoScript-like because the
most visible authoring examples combine:

- dotted paths
- numeric child keys
- angle-bracket type names
- `key = value` assignments
- `{}` blocks
- `<<<` inheritance
- `@import`
- documented alphanumeric rendering order

The goal is not to redesign the runtime. The goal is to remove the public
TypoScript-like authoring surface while preserving the existing HyperBricks
object model, inheritance behavior, and mapstructure-based renderer contracts.

## Chosen Direction

Use a strict HyperBricks YAML profile as the new public source format.

Decision update, 2026-05-27:

Treat this as a real public syntax transition, not as a cosmetic patch to the
old DSL. The target is to remove the TypoScript-like public authoring surface
from new HyperBricks docs, examples, generated output, and standalone runtime
inputs. Old repositories have been made private so the project can take the time
to do this cleanly before the next public push.

The YAML profile keeps object names and nested object identity, but uses YAML
sequences to preserve source order instead of numeric keys.

Example:

```yaml
mijnComponent:
  - type: template
  - template: |
      <iframe width="{{width}}" height="{{height}}" src="{{src}}"></iframe>
  - values:
      width: 300
      height: 400
      src: https://www.youtube.com/embed/tgbNymZ7vqY

mijnHypermedia:
  - type: hypermedia
  - route: index
  - title: Home

  - head:
      - type: head
      - css:
          - type: css
          - inline: |
              .content {
                color: green;
              }

  - intro:
      - type: html
      - value: |
          <p>SOME CONTENT</p>

  - video:
      - inherit: mijnComponent
      - values:
          src: https://www.youtube.com/watch?v=Wlh6yFSJEms
      - enclose: <div class="youtube_video">|</div>
```

## Key Rule

Mapstructure remains the runtime bridge.

Architecture:

```text
YAML source
  -> HyperBricks YAML source pipeline
       - YAML-safe preprocessing
       - imports/includes
       - vars/env/resources/template markers
  -> ordered HyperBricks source model
  -> resolve inheritance, overrides, and order
  -> materialize per node to map[string]interface{}
  -> mapstructure.Decode(...)
  -> existing renderers
```

This avoids a full renderer rewrite. The new parser/source layer owns ordering
and inheritance resolution. Existing renderers continue to receive map-shaped
configuration.

Important correction: the preprocessor remains before parsing. The YAML parser
must therefore grow into a format-aware HyperBricks source pipeline, not just a
thin `yaml.v3` wrapper. The old `.hyperbricks` preprocessing rules cannot be
blindly reused because YAML has different comment and multiline rules.

## Ordered Model Sketch

```go
type OrderedDocument map[string]*OrderedNode

type OrderedNode struct {
    Name     string
    Type     string
    Inherit  string
    Props    map[string]any
    Children []*OrderedNode
}
```

Materialization to mapstructure input:

```go
func (n *OrderedNode) ToMap() map[string]interface{} {
    out := map[string]interface{}{}

    if n.Type != "" {
        out["@type"] = "<" + strings.ToUpper(n.Type) + ">"
    }

    for k, v := range n.Props {
        out[k] = v
    }

    order := make([]string, 0, len(n.Children))
    for _, child := range n.Children {
        order = append(order, child.Name)
        out[child.Name] = child.ToMap()
    }
    if len(order) > 0 {
        out["@order"] = order
    }

    return out
}
```

Example materialized shape:

```go
map[string]interface{}{
    "@type":  "<HYPERMEDIA>",
    "@order": []string{"head", "intro", "video"},
    "route":  "index",
    "title":  "Home",
    "intro": map[string]interface{}{
        "@type": "<HTML>",
        "value": "<p>SOME CONTENT</p>",
    },
    "video": map[string]interface{}{
        "@type": "<TEMPLATE>",
        "template": "...",
        "values": map[string]interface{}{
            "src": "https://www.youtube.com/watch?v=Wlh6yFSJEms",
        },
    },
}
```

## Runtime Impact

Minimal intended renderer change:

- add internal order metadata as `@order` inside the existing `Items` pass
- let composite rendering follow explicit `Items["@order"]` metadata exactly
- keep `SortedUniqueKeys` only as legacy fallback for old `.hyperbricks` input
- keep mapstructure tags and component structs as the runtime contract

Do not introduce a second child-pass contract. `Items` remains the pass. If a
typed internal order field is ever added later, it must be a convenience around
the same `Items["@order"]` contract, not a replacement for it.

### Decision: TREE Order Is Explicit

If a `<TREE>` receives `Items["@order"]`, it renders only the keys listed in
that order. Unlisted renderable items are not appended implicitly.

Rules:

- `@order` is local to one `Items` map.
- Nested composites need their own `@order`; parent order does not apply to
  grandchildren.
- If `@order` is absent, `<TREE>` uses legacy alphanumeric sorting for old
  `.hyperbricks` input.
- Runtime code that injects renderable items into a tree-owned map must also
  place those items in `@order` if deterministic YAML rendering should include
  them.
- Metadata/runtime fields such as `@type`, `hyperbricksfile`,
  `hyperbrickspath`, and `hyperbrickskey` are never render keys.

This removes the implicit "ordered keys first, then everything else sorted"
behavior from the YAML path. It makes missing `@order` entries visible instead
of silently changing output later.

### Decision: HEAD Generated Items

`<HEAD>` generated output remains part of the normal `Items` pass. There is no
separate slot or second render contract.

The legacy renderer used numeric implementation keys:

- `999` for the default generator meta tag
- `1000` for generated head payload from `favicon`, `title`, `meta`, `css`,
  and `js`

For the YAML deterministic render contract these generated items are named:

- `generator`
- `payload`

Rules:

- `generator` and `payload` are ordinary `HEAD.Items`.
- If a YAML source defines either item explicitly, that item overrides the
  runtime-generated version.
- If the YAML source does not define them, the runtime injects the missing
  generated items.
- If `@order` is present, missing generated items are appended to that order as
  `generator`, then `payload`.
- If the user-defined `@order` already contains `generator` or `payload`, their
  position is respected.
- There is no renderer-level compatibility fallback for `999`/`1000`. Those
  names are treated as normal legacy item names by the old parser path, not as a
  mode switch in `<HEAD>`.

This keeps the public YAML contract deterministic without introducing another
runtime abstraction:

```yaml
page:
  - type: hypermedia
  - head:
      - type: head
      - payload:
          - type: html
          - value: <title>Manual payload</title>
      - custom_head:
          - type: html
          - value: <meta name="custom" content="yes">
      - generator:
          - type: html
          - value: <meta name="generator" content="manual">
```

## Inheritance Semantics

`inherit` is the YAML replacement for `<<<`.

Rules:

- `inherit: mijnComponent` deep-copies the referenced ordered node
- local properties override inherited properties
- local nested objects override or extend inherited nested objects by name
- inherited child order is preserved
- overriding an existing child does not move it
- new local children are appended at the source location where they appear
- the resolved node is then materialized to a map for mapstructure

This preserves the current mental model:

```text
mijnHypermedia.video inherits mijnComponent
mijnHypermedia.video.values.src overrides the inherited value
```

## YAML Profile Rules

The profile should be strict and small:

- top-level keys define named HyperBricks objects
- a component object is a YAML sequence of ordered entries
- scalar properties are written as single-key entries
- child objects are written as single-key entries whose value is another ordered sequence
- `type` may use runtime tokens such as `<HYPERMEDIA>`, `<TREE>`, `<HTML>`, or
  canonical names such as `hypermedia`, `tree`, `html`
- `inherit` references another named object path
- block scalars (`|`) replace current multiline markers
- normal YAML maps/lists remain valid inside data fields such as `values`, `querykeys`, `headers`, and plugin data

The parser must distinguish ordered component sequences from ordinary data
arrays by context.

### Scalar Normalization

HyperBricks source remains typeless by default. YAML must not become the owner
of runtime typing.

In the old DSL, source fields effectively arrived as strings and the
mapstructure/typefactory hooks converted strings into ints, bools, slices, and
maps where a concrete runtime struct required them.

The YAML profile should preserve that authoring model while avoiding excessive
quotes:

```yaml
image:
  - type: <IMAGE>
  - src: "{{RESOURCES}}/screenshots/player.jpg"
  - width: 800
  - loading: lazy
  - is_static: true
```

Materialized scalar intent:

```go
map[string]interface{}{
    "@type": "<IMAGE>",
    "src": "{{RESOURCES}}/screenshots/player.jpg",
    "width": "800",
    "loading": "lazy",
    "is_static": "true",
}
```

The runtime may then convert `"800"` to `int` or `"true"` to `bool` through the
existing decode hooks when the target struct field requires it.

This rule should apply to component fields, nested component fields, template
values, plugin data, headers, query params, and normal YAML arrays/maps unless
we later introduce explicit typed literals. Structured YAML maps/lists remain
structured; their scalar leaves are normalized to strings.

YAML syntax still matters. Values that YAML would otherwise treat as comments or
ambiguous syntax must be written safely:

```yaml
response:
  hx_target: "#status"
```

### Guard Example

Route-owning composites keep explicit fields as fields. `guard` is not a child
item; it decodes to the composite's `Guard` field.

```yaml
page:
  - type: <HYPERMEDIA>
  - route: dashboard
  - title: Dashboard
  - guard:
      enabled: true
      redirect: login
      require:
        authenticated: true
        roles:
          - admin
          - editor
  - hero:
      - type: <HTML>
      - value: |
          <section class="hero">
            <h1>Dashboard</h1>
            <p>Private workspace overview.</p>
          </section>
```

Runtime intent:

```text
route/title/guard -> explicit HyperMediaConfig fields
hero              -> shared.Composite.Items["hero"]
@order            -> shared.Composite.Items["@order"]
```

## What This Removes From Public Syntax

- top-level `<HYPERMEDIA>` assignment declarations such as
  `page = <HYPERMEDIA>`
- numeric child slots such as `page.10`
- alphanumeric render order as an authoring concept
- dotted assignment syntax
- `<<<` inheritance syntax
- `@import` directive syntax
- custom multiline markers

`type: <HYPERMEDIA>` remains allowed as an explicit YAML field value because it
maps directly to the existing runtime type token. The syntax risk being removed
is the old assignment grammar, not the runtime token identity itself.

## What This Preserves

- named top-level objects
- nested object addressability
- object inheritance and local overrides
- existing component field names
- template values
- plugin data maps
- mapstructure-based decoding
- most renderer implementations

## Open Decisions

- file extension: `.hyperbricks.yaml`, `.hbr.yaml`, or another explicit suffix
- whether old `.hyperbricks` remains supported as legacy input or only via migrator
- exact path syntax for `inherit` references
- strict handling of duplicate keys and duplicate ordered entries
- whether `@order` should be hidden metadata or a typed internal field before materialization
- whether schema/docs generation should read the ordered source model or generated mapstructure metadata

## Import Semantics

Decision, 2026-05-27:

Use `imports:` as the YAML profile field name.

Do not use `include` for object imports. In YAML, `include` implies textual
insertion at the current source location, which is indentation-sensitive and
easy to make ambiguous. HyperBricks YAML imports should be document/model
imports instead.

Old `.hyperbricks` behavior:

```text
@import "partials/base.hyperbricks"
```

The old preprocessor performs textual replacement:

```go
hyperBricks = strings.Replace(hyperBricks, match[0], processedImport, 1)
```

That means the old import is effectively a string include. YAML should not copy
that implementation detail. It should preserve the authoring capability while
using a safer source model.

Target YAML:

```yaml
imports:
  - partials/base.hyperbricks.yaml
  - partials/cards.hyperbricks.yaml

page:
  - type: <HYPERMEDIA>
  - inherit: site_page
```

Semantics:

- `imports:` is top-level only
- import paths are relative to the file that declares them
- imported files must be full valid HyperBricks YAML documents with top-level
  roots
- imported roots are merged into the same ordered document namespace
- import order is deterministic: first imported file roots in listed order, then
  local roots
- nested imports are resolved recursively
- inheritance can reference imported roots
- duplicate top-level root names are rejected for now
- import cycles are rejected with a clear error
- textual source snippets are not supported through `imports:`

If source text/body inclusion is needed, that belongs to the file/template marker
feature, not to `imports:`.

Implementation path:

```text
root file
  -> parse YAML document and read imports
  -> recursively load imported files
  -> parse imported documents
  -> merge imported roots into one ordered Document
  -> append local roots
  -> resolve inherit references across merged document
  -> materialize
```

API shape:

```go
func ParseBytes(input []byte) (*Document, error)
func LoadFile(path string, opts Options) (*Document, error)

type Options struct {
    // grows later with env/template/resource/preprocessor settings
}
```

`ParseBytes` remains a pure parser with no filesystem access. `LoadFile` or a
future `ProcessFile` owns filesystem imports and YAML-safe preprocessing.

Required import fixtures:

- single import
- recursive import
- import order
- inherit from imported root
- duplicate root rejected
- cycle rejected
- relative path from importing file
- local root order after imported roots

## Immediate Next Step

Before implementing, audit current parser features and classify each as:

- must preserve in YAML profile
- replace with YAML-native equivalent
- legacy-only migration support

This keeps the migration grounded in the existing HyperBricks runtime instead of
inventing a new product model.

## Parser Dry Run Notes

The proposed YAML shape can be parsed into a materialized map that is close to
the current runtime input.

Input:

```yaml
mijnComponent:
  - type: template
  - template: |
      <iframe width="{{width}}" height="{{height}}" src="{{src}}"></iframe>
  - values:
      width: 300
      height: 400
      src: https://www.youtube.com/embed/tgbNymZ7vqY

mijnHypermedia:
  - type: hypermedia
  - route: index
  - title: Home

  - head:
      - type: head
      - styles:
          - type: css
          - inline: |
              .content {
                color: green;
              }

  - intro:
      - type: html
      - value: |
          <p>SOME CONTENT</p>

  - video:
      - inherit: mijnComponent
      - values:
          src: https://www.youtube.com/watch?v=Wlh6yFSJEms
      - enclose: <div class="youtube_video">|</div>
```

Materialized runtime shape:

```json
{
  "mijnHypermedia": {
    "@type": "<HYPERMEDIA>",
    "route": "index",
    "title": "Home",
    "head": {
      "@type": "<HEAD>",
      "styles": {
        "@type": "<CSS>",
        "inline": ".content {\n  color: green;\n}\n"
      },
      "@order": ["styles"]
    },
    "intro": {
      "@type": "<HTML>",
      "value": "<p>SOME CONTENT</p>\n"
    },
    "video": {
      "@type": "<TEMPLATE>",
      "template": "<iframe width=\"{{width}}\" height=\"{{height}}\" src=\"{{src}}\"></iframe>\n",
      "values": {
        "width": 300,
        "height": 400,
        "src": "https://www.youtube.com/watch?v=Wlh6yFSJEms"
      },
      "enclose": "<div class=\"youtube_video\">|</div>"
    },
    "@order": ["head", "intro", "video"]
  }
}
```

This confirms the basic adapter path:

- `type: template` becomes `@type: <TEMPLATE>`
- YAML block scalars replace `<<[ ... ]>>`
- `inherit` can deep-copy an earlier object and merge local overrides
- ordered object entries become `@order`
- output remains mapstructure-compatible for existing renderers

### Important Collision Finding

Numeric keys currently avoid collisions with component fields. Named children
can collide with existing mapstructure fields.

Concrete dry run:

```yaml
head:
  - type: head
  - css:
      - type: css
      - inline: |
          .content { color: green; }
```

Materializes to:

```json
{
  "@type": "<HEAD>",
  "css": {
    "@type": "<CSS>",
    "inline": ".content { color: green; }"
  },
  "@order": ["css"]
}
```

But `HeadConfig` already has:

```go
Css []string `mapstructure:"css"`
```

So mapstructure tries to decode the `css` child object into `[]string` and
fails. A non-colliding child name works:

```yaml
head:
  - type: head
  - styles:
      - type: css
      - inline: |
          .content { color: green; }
```

Materializes to `head.styles`, and mapstructure leaves it in `HeadConfig.Items`
as intended.

This means the YAML profile needs one of these rules:

1. Reserve all component field names. Child names must not collide with fields
   such as `css`, `js`, `template`, `values`, `route`, `title`, etc.
2. Introduce an explicit child namespace for collision cases.
3. Add a stronger adapter that can keep source child names while feeding
   non-colliding internal keys to mapstructure.

The first option is the smallest runtime change, but it must be documented and
validated clearly.

### Existing Runtime Fit

`HyperMediaConfig`, `FragmentConfig`, and `TreeConfig` already receive unknown
keys through embedded `Composite.Items` via `mapstructure:",remain"`.

That means a materialized map like:

```json
{
  "@type": "<HYPERMEDIA>",
  "route": "index",
  "intro": {
    "@type": "<HTML>",
    "value": "<p>SOME CONTENT</p>"
  },
  "@order": ["intro"]
}
```

decodes with:

```text
Route = "index"
Composite.Items["intro"] = map["@type":"<HTML>", "value":"..."]
Composite.Items["@order"] = []string{"intro"}
```

So the source-order feature can be added by teaching composite renderers to
prefer explicit order metadata over `SortedUniqueKeys`.

### Composite Items Contract

Important alignment, 2026-05-27:

`Items` is not an incidental remainder field. It is the existing composite
render pass.

The YAML migration must preserve this contract. It must not introduce a new
runtime contract such as `CompositeItems`, and it must not require component,
plugin, or renderer authors to learn a second child-pass model.

The existing mechanism is:

```text
mapstructure:",squash"
  -> flattens embedded shared.Composite into the concrete config

mapstructure:",remain"
  -> moves keys that are not explicit fields into shared.Composite.Items
```

For a composite input like:

```yaml
page:
  - type: <HYPERMEDIA>
  - route: nested-tree-3-level
  - main:
      - type: <TREE>
      - article:
          - type: <TREE>
```

the parser materializes ordinary map keys:

```json
{
  "@type": "<HYPERMEDIA>",
  "route": "nested-tree-3-level",
  "@order": ["main"],
  "main": {
    "@type": "<TREE>",
    "@order": ["article"],
    "article": {
      "@type": "<TREE>"
    }
  }
}
```

Then mapstructure decodes the known fields and keeps the unknown child keys in
the existing composite pass:

```go
config.Route = "nested-tree-3-level"
config.Items["@order"] = []string{"main"}
config.Items["main"] = map[string]interface{}{
    "@type": "<TREE>",
    "@order": []string{"article"},
    "article": map[string]interface{}{
        "@type": "<TREE>",
    },
}
```

This is the target runtime contract. `main`, `article`, `hero`, `intro`, etc.
are arbitrary child names. They are not framework keywords. The ordering signal
is `@order`, not the child name.

The current code has a confusing representation issue in `HyperMediaConfig` and
`FragmentConfig`: both embed `shared.Composite` and also declare their own
`Items map[string]interface{}` field with `mapstructure:",remain"`. A dry run
showed that the actual render pass is still the embedded
`shared.Composite.Items`, while normal JSON output can show the concrete
struct's `Items` as `null`.

That means:

- the architecture is still correct: `Items` remains the pass
- the parser should keep emitting normal child keys plus `@order`
- the readable YAML tests should avoid implying that `Items` is absent
- any cleanup should restore clarity around the existing `Items` field, not
  rename or replace the contract
- `HyperMediaConfig` and `FragmentConfig` should be audited against
  `ApiFragmentRenderConfig`, which uses `shared.Composite` without declaring a
  second `Items` field

`content` should not be treated as a special replacement for numeric child keys.
It is just another child name. Prefer examples that make named children explicit:

```yaml
page:
  - type: <HYPERMEDIA>
  - route: index
  - hero:
      - type: <HTML>
      - value: <h1>Hello</h1>
  - intro:
      - type: <TEXT>
      - value: Welcome
```

The only semantic ordering contract in that source is:

```text
@order = ["hero", "intro"]
```

### YAML Source Pipeline Requirements

`pkg/parser/postprocessor.go` is important because it shows that old
HyperBricks parsing is not just syntax parsing. It also owns comment stripping,
custom multiline handling, and a template store used by template markers.

For YAML, that behavior must move into `pkg/yaml-parser` carefully and
incrementally. The old `StripComments` rules are old-DSL aware and must not be
run blindly on YAML because YAML has different comment and scalar rules.

Old pipeline:

```text
.hyperbricks source
  -> StripComments / PreprocessHyperScript / template marker handling
  -> ParseHyperScript
  -> mapstructure-compatible map
```

Target YAML pipeline:

```text
.hyperbricks.yaml source
  -> YAML-safe HyperBricks preprocessing
       - imports/includes
       - vars
       - env
       - resources
       - template markers/template dirs
       - comments handled according to YAML rules
  -> YAML parser
  -> ordered source model
  -> inheritance and override resolution
  -> scalar normalization
  -> materialized mapstructure-compatible map
```

The YAML parser package should therefore grow from `ParseBytes` into a tested
source pipeline. Add complexity slowly, with one focused test per feature:

- scalar fields normalize to strings without requiring quotes
- block scalar values preserve intended multiline content
- `#` inside quoted scalar values remains a value, not a comment
- vars work in plain scalar values
- vars work inside block scalar values
- env placeholders work in scalar values
- resources placeholders work in paths
- template markers work as scalar references
- template markers work safely inside block scalars
- file markers work only through a YAML-safe representation
- imports/includes preserve deterministic source order
- imported roots and local roots materialize into the same ordered document
- inheritance works after imports
- reserved runtime field names cannot be used as child names
- named children land in `shared.Composite.Items`
- `@order` lands in the same `Items` pass

### Legacy-Only Features

Do not silently forget old parser features. Every old source feature must be
classified as either supported in the YAML profile, replaced by a YAML-native
construct, or intentionally legacy-only.

Decision, 2026-05-27:

Old macro syntax is legacy-only and intentionally not part of the YAML profile.

Excluded from YAML:

- `@macro as (...) { ... } = <<<[ ... ]>>>`
- `<<<[ ... ]>>>` macro template blocks
- `{{{.var}}}` macro variable replacement

Reason:

YAML gives HyperBricks native structure, native ordered sequences, and native
block scalars. The old macro layer belongs to the old assignment DSL and should
not be redesigned into the new public source format.

This must still be covered by tests. The YAML test suite should include a
guardrail case proving that macro syntax is not accepted as a YAML source
feature, while legacy `.hyperbricks` can continue to support it as long as the
old parser remains.

This staged approach keeps the YAML migration grounded in existing HyperBricks
behavior. Each old parser/preprocessor feature gets a YAML proof before it is
considered migrated.

## Composer Generation Impact

Composer changes this from a parser-only problem into a source-model and
serializer problem.

Important correction: Composer's database state remains the authoring source of
truth. Generated `.hyperbricks` output is a runtime artifact and can change
format. Composer import/restore should not reconstruct the Builder database by
parsing generated `.hyperbricks` output when Composer metadata is present.

Composer already uses reserved metadata:

```text
META-INF/hyperbricks-composer/manifest.json
META-INF/hyperbricks-composer/snapshot.json
```

That metadata carries the state description used to restore/import Composer
authoring state. Therefore the new HyperBricks source syntax only needs to be:

- parseable by standalone HyperBricks runtime
- generatable by Composer ProjectPublish
- deterministic and inspectable as a deploy artifact

It does not need to be a lossless Composer DB round-trip format. The lossless
round-trip belongs to `META-INF/hyperbricks-composer/snapshot.json`.

Relevant Composer v4 ownership:

- Builder stores order explicitly:
  - route roots use `bricks.root_sort_order`
  - child placements use `brick_placements.sort_order`
  - template value rows use `_composer.order.values`
- ProjectPublish already generates runtime `.hyperbricks` output from Composer
  data in:
  `../hyperbricks-composer-v4/modules/hyperbricks-composer/plugins/projectpublish/1.0.0/project_publish_plugin.go`
- The current emitter creates the old public syntax through:
  - `appendAssignLine`: `key = value`
  - `appendInheritLine`: `key <<< source`
  - `appendImportLine`: `@import "..."`
  - `generatedChildPath`: numeric fallback paths such as `.10`, `.20`
- Composer already materializes virtual authoring components such as
  `<SPACES>` into runtime-safe output, so changing the source format must keep
  that materializer boundary intact.

This is good news for removing numeric ordering. Composer already has the real
ordering data. The generated file does not need `page.10`, because the emitter
can write children in `sort_order` order and use stable child names from
`path_key` or normalized brick names.

Target generation model:

```text
Composer DB graph
  -> ordered publish document
  -> HyperBricks source serializer
  -> runtime parser
  -> mapstructure-compatible map
  -> existing renderers
```

The important contract is that parser and generator share the same ordered
document model. If the parser materializes `@order`, the Composer generator
should emit source order from the same ordered model instead of rebuilding old
numeric paths.

### Serializer Requirements

The Composer serializer must be deterministic and validate names before emit:

- preserve root order from `root_sort_order`
- preserve child order from `brick_placements.sort_order`
- preserve template value order from `_composer.order.values`
- generate stable child names from `path_key` first, then normalized brick name,
  then a collision-safe suffix
- reject or rewrite child names that collide with component fields such as
  `route`, `values`, `template`, `css`, `js`, `head`, and `response`
- keep inherited placements addressable by stable top-level names
- emit virtual components only after materialization, never as Composer-only
  runtime syntax

### Consequence

The clean implementation is not "add YAML parsing" in isolation.

It is:

1. Introduce a HyperBricks ordered source model in core.
2. Add parser and materializer from source into current mapstructure maps.
3. Teach Composer ProjectPublish to serialize that same ordered model instead
   of emitting old dotted/numeric assignment lines directly.

That keeps Composer generation, MCP authoring, runtime parsing, and static
publishing on one model.

## Prototype Status

Implemented on branch `codex/dsl-migration-sprint`:

- `pkg/yaml-parser`: strict YAML source parser with an ordered `Document` /
  `Node` model
- materialization to the current mapstructure-compatible runtime map shape
- inheritance resolution with deep map merge and child override/append behavior
- canonical type validation for the current core runtime component set
- validation for duplicate children, property/child collisions, internal
  runtime keys, and reserved runtime field-name collisions
- `cmd/hyperbricks-yaml-dryrun`: temporary CLI to inspect parser output
- `docs/YAML_PROFILE.md`: draft public source-profile contract
- YAML fixture area:
  `test/docs/hyperbricks-yaml-test-files/`
- readable `.hyperbricks.yaml.test` fixtures for representative runtime
  scenarios
- runtime typefactory decode tests for all core YAML fixtures

Current proof:

```text
go test ./pkg/yaml-parser
go test ./test/docs -run 'TestYAMLProfile'
go test ./...
go vet ./...
```

All passed on 2026-05-27.

## Next Proof Step

The next useful step is renderer-level proof for a small set of fixtures:

```text
YAML fixture
  -> materialized runtime map
  -> existing RenderManager
  -> rendered HTML
  -> compare with the expected output embedded in the YAML fixture
```

That will prove more than parser shape. It will prove that source order,
reserved-name handling, inheritance, and value-mounted components behave the
same once the existing renderers execute them.

## Executable Pipeline Pass

The first executable pass now covers the YAML phases before runtime integration:

```text
source YAML
  -> YAML-safe preprocessor
  -> ordered source model
  -> materialized runtime map
  -> runtime typefactory projection
  -> optional renderer output
```

The main readable fixture is:

```text
test/docs/hyperbricks-yaml-test-files/pipeline-pre-parse-post.hyperbricks.yaml.test
```

That fixture deliberately shows:

- `{{VAR:...}}`, `{{ENV:...}}`, `{{CONF:...}}`
- `{{RESOURCES}}`
- `{{FILE:...}}` inside a YAML block scalar
- YAML comments ignored by the parser
- comments preserved inside a `|`
- guard fields as regular structured runtime fields
- materialized `@order`
- the existing runtime `Items` pass, not a new child contract

Parser unit coverage now includes:

- scalar normalization to strings before mapstructure decode
- YAML comments and quoted `#status` values
- `{{TEMPLATE:...}}` template-store behavior when a template directory is
  provided
- model imports through `LoadFile`
- import ordering
- inheritance from imported roots
- duplicate imported-root rejection
- import-cycle rejection
- legacy macro guardrails
- `{{FILE:...}}` placement rules

Current verified commands after this pass:

```text
go test ./pkg/yaml-parser -count=1
go test ./test/docs -run TestYAMLProfileReadableCases -count=1
go test ./test/docs -run TestYAMLProfileFixturesParseAndMaterialize -count=1
go test ./test/docs -count=1
go test ./... -count=1
go vet ./...
```

## YAML Documentation Pipeline Decision

The existing `.hyperbricks` documentation generator and fixture test should stay
intact for now. It is the current baseline and should not be refactored while
the YAML source model is still being proven.

The YAML documentation/test pipeline should be implemented in parallel instead
of being folded into `test/docs/documentation_source_test.go` or
`test/docs/documentation_generation.go`.

Proposed new files:

```text
test/docs/yaml_documentation_source_test.go
test/docs/yaml_template.md
docs/REFERENCE_YAML.md
```

`docs/REFERENCE_YAML.md` can later replace `docs/REFERENCE.md` only after the
YAML runtime integration is accepted.

### Source Of Truth

- `pkg/schema.Definitions()` and struct tags remain the source of truth for
  fields.
- YAML fixtures remain the source of truth for behavior.
- The old `.hyperbricks` fixture set remains a baseline until the migration is
  intentionally completed.

### Fixture Layers

The YAML docs pipeline should distinguish three kinds of fixtures:

- field fixtures: one fixture per public documented field where field-level
  behavior matters
- curated component fixtures: compact public examples for each component or
  composite
- feature fixtures: deeper scenarios for behavior such as inheritance, imports,
  preprocessor markers, guard, response headers, `@order`, nested `Items`, and
  template values

The field fixtures are allowed to be exhaustive because they protect sync with
code. The public reference should not render every field fixture as a full
section.

### Public Reference Shape

Generated YAML reference output should be compact:

- component/composite heading
- short type description from `@doc` or schema description
- field table generated from struct tags
- one curated YAML config example
- expected output for the curated example
- links or short references to relevant feature fixtures when useful

It should avoid the current `REFERENCE.md` problem where every field-level
fixture becomes a large public documentation section.

### Sync Rules

The new YAML documentation test should fail when:

- a public `mapstructure` field has no generated field-table entry
- a field has an `example:"{!{...}}"` reference that points to a missing YAML
  fixture, unless explicitly skipped
- a curated YAML example does not parse
- a curated YAML example does not materialize into the existing runtime map
  shape
- a curated YAML example does not instantiate through the runtime typefactory
- a curated YAML example has expected output and the renderer output differs
- a YAML fixture uses a child name that collides with a runtime field without
  being intentionally modeled as a field

The test may allow explicit skips, but skips must be centralized and named so
they are reviewable.

### Non-Goals For The Next Pass

- Do not change the old `.hyperbricks` documentation generator.
- Do not switch runtime loading to YAML yet.
- Do not replace `docs/REFERENCE.md` yet.
- Do not require every field fixture to become public prose.

### Acceptance For The Next Implementation Pass

The next pass is accepted when:

```text
go test ./test/docs -run TestYAMLDocumentation -count=1
go test ./test/docs -run TestYAMLProfile -count=1
go test ./... -count=1
go vet ./...
```

all pass, and `docs/REFERENCE_YAML.md` can be generated from the same source
data without manually editing the generated content.

## Implemented YAML Documentation Pipeline Pass

The YAML documentation pipeline now exists as a parallel pass next to the old
`.hyperbricks` documentation generator.

Implemented files:

```text
test/docs/yaml_documentation_source_test.go
test/docs/yaml_template.md
docs/REFERENCE_YAML.md
```

The pass currently does four things:

- checks that schema `example:"{!{...}}"` references have matching YAML
  fixtures
- runs one curated YAML fixture per schema definition through preprocess,
  materialize, runtime decode, and render checks where output is present
- generates a compact YAML reference from `pkg/schema.Definitions()` and the
  curated executable fixtures
- fails when `docs/REFERENCE_YAML.md` is stale unless regenerated with
  `-update-yaml-docs`
- protects prose/table text in generated Markdown by rendering all-caps
  `<TYPE>` tokens as inline code, so markdown-to-HTML does not create raw HTML
  elements such as `<template>`

The old generator remains untouched. `docs/REFERENCE_YAML.md` is not yet a
replacement for `docs/REFERENCE.md`; it is the proving ground for the YAML
source model and the future public reference shape.

Verified commands after implementation:

```text
go test ./test/docs -run TestYAMLDocumentationReference -update-yaml-docs -count=1
go test ./test/docs -run TestYAMLDocumentationReferenceMarkdownIsHTMLSafe -count=1
go test ./test/docs -run TestYAMLDocumentation -count=1
go test ./test/docs -run TestYAMLProfile -count=1
go test ./test/docs -count=1
go test ./... -count=1
go vet ./...
```

## Implemented Runtime Loader Integration Pass

The runtime now has a narrow YAML source loader next to the legacy
`.hyperbricks` loader.

Implemented behavior:

- files ending in `.hyperbricks.yaml` are loaded from the configured
  `hyperbricks` directory
- each YAML file runs through the YAML source pipeline and materializes into the
  existing mapstructure-compatible runtime map
- legacy `.hyperbricks` files still load through the old parser path
- both source formats are passed into the same `processScript` route indexing
  path
- YAML-only modules are allowed; the loader errors only when neither
  `.hyperbricks` nor `.hyperbricks.yaml` files exist
- `package.hyperbricks` remains legacy for now and is not part of this pass

The first runtime integration proof is:

```text
temporary module with hyperbricks/page.hyperbricks.yaml
  -> PreProcessAndPopulateConfigs
  -> route config indexed in the global runtime config map
  -> ServeContent
  -> rendered HTML response
```

Verified command:

```text
go test ./cmd/hyperbricks -run 'TestPreProcessAndPopulateConfigsLoadsYAMLRouteThroughServerRenderFlow|TestProcessScript' -count=1 -v
```

## Implemented Runtime Preprocessing Integration Pass

The runtime YAML loader now passes the configured module directories into the
YAML preprocessor as both path markers and runtime variables.

Supported through the runtime loader:

- `imports:` model imports, resolved relative to the importing YAML file
- `{{ENV:NAME}}`, read from the process environment unless explicitly supplied
  in parser options
- `{{CONF:path.to.value}}`, read from the current parsed HyperBricks config map
- `{{VAR:module}}`, `{{VAR:module_root}}`, `{{VAR:root}}`,
  `{{VAR:resources}}`, `{{VAR:templates}}`, `{{VAR:static}}`,
  `{{VAR:hyperbricks}}`, and `{{VAR:render}}`
- path markers `{{MODULE_ROOT}}`, `{{ROOT}}`, `{{MODULE}}`,
  `{{RESOURCES}}`, `{{TEMPLATES}}`, `{{STATIC}}`, and `{{HYPERBRICKS}}`
- `{{TEMPLATE:path.html}}`, stored in the existing runtime template store
- `{{FILE:path}}`, only when the marker occupies a full YAML block-scalar line

The runtime proof uses a temporary module containing:

```text
hyperbricks/page.hyperbricks.yaml
hyperbricks/partials/shared.hyperbricks.yaml
templates/cards/runtime-card.html
resources/runtime-body.html
```

That test proves this full path:

```text
YAML source
  -> import shared component
  -> env/config/var/path/file/template preprocessing
  -> inheritance/materialization
  -> normal route indexing
  -> ServeContent
  -> rendered HTML response
```

The current runtime variable set is deliberately derived from
`core.ModuleDirectories`. Arbitrary user-defined YAML variables are supported by
the parser options, but package-level YAML variable configuration is not part of
this pass. `package.hyperbricks` remains legacy for now.

Verified command:

```text
go test ./cmd/hyperbricks -run 'TestPreProcessAndPopulateConfigs.*YAML|TestProcessScript' -count=1 -v
go test ./cmd/hyperbricks -count=1
go test ./pkg/yaml-parser -count=1
go test ./... -count=1
go vet ./...
```

## Default Init Source Decision

The default `hyperbricks init` module should introduce the YAML source format.
The embedded Hello World asset is now:

```text
cmd/hyperbricks/commands/assets/default/hyperbricks/hello-world.hyperbricks.yaml
cmd/hyperbricks/commands/assets/default/templates/hello-card.html
```

The default init command still writes `package.hyperbricks` for runtime package
configuration. Route source moves to YAML first; package configuration remains a
later migration step. The default route demonstrates both supported template
paths:

- `template: "{{TEMPLATE:hello-card.html}}"` with values loaded from
  `templates/hello-card.html`
- `inline: |` with local values directly in the YAML source

Verified command:

```text
go test ./cmd/hyperbricks/commands -run TestDefaultInitAssetsWriteYAMLHelloWorld -count=1 -v
```

## Demo Module Spectrum Example

`modules/demo` now contains a visible YAML runtime spectrum route:

```text
modules/demo/hyperbricks/yaml-spectrum.hyperbricks.yaml
modules/demo/hyperbricks/partials/yaml-spectrum.shared.hyperbricks.yaml
modules/demo/templates/spectrum/card.html
modules/demo/templates/spectrum/directories.html
modules/demo/templates/spectrum/status.html
modules/demo/resources/yaml-spectrum/lead.html
modules/demo/static/yaml-spectrum.css
```

The route `/yaml-spectrum` demonstrates the runtime YAML path with:

- `imports:` from a YAML partial
- inheritance from imported components
- `{{CONF:...}}` values from `package.hyperbricks`
- `{{ENV:SHELL}}`
- runtime `{{VAR:...}}` values
- path markers for module, resources, templates, static, and hyperbricks dirs
- `{{FILE:...}}` loading a resource file into a block scalar
- `{{TEMPLATE:...}}` loading reusable Go template files
- value-mounted bricks inside template values
- nested ordered trees without numeric keys
- a companion `/yaml-spectrum/status` fragment with HTMX response headers

Manual runtime proof:

```text
go run ./cmd/hyperbricks start -m demo -p 18089 --non-interactive
curl -sS http://127.0.0.1:18089/yaml-spectrum
curl -sS -D - http://127.0.0.1:18089/yaml-spectrum/status
```

Verified after adding the demo:

```text
go test ./... -count=1
go vet ./...
```

## Legacy Converter And Patterns Acceptance Pass

The migration now has a first source-aware legacy converter instead of a raw
map dump.

Implemented files:

```text
cmd/hyperbricks-yaml-convert/main.go
pkg/legacy-yaml-converter/converter.go
pkg/legacy-yaml-converter/converter_test.go
modules/hyperbricks-patterns-yaml/
```

The converter reads old `.hyperbricks` source operations and emits the new YAML
profile while preserving the important source model:

- `@import "x.hyperbricks"` becomes top-level `imports: ["x.hyperbricks.yaml"]`
- `name = <TYPE>` becomes `type: <type>`
- `target <<< source` becomes `inherit: source`
- source blocks become nested YAML nodes or maps
- multiline `<<[ ... ]>>` values become YAML block scalars
- numeric render slots are converted to stable semantic IDs such as
  `template_10`, `html_10`, and `plugin_10`
- module-local file references in the generated acceptance module use
  `{{MODULE}}/...` so tests and runtime starts do not depend on the current
  shell working directory

The numeric suffix is intentionally retained in generated names for now. The new
public model no longer uses numeric ordering, but the converter needs stable
names that avoid collisions with runtime fields such as `template`, `head`,
`route`, and `title`.

The duplicated acceptance module is:

```text
modules/hyperbricks-patterns-yaml
```

It is generated from `modules/hyperbricks-patterns` and proves that a
non-trivial real module can be converted and loaded through the normal runtime
flow.

Important parser fix discovered by this pass:

```text
pkg/yaml-parser/parser.go
```

Value-mounted inherited nodes must resolve recursively. A real patterns case
overrode an inherited template value with:

```yaml
content:
  - inherit: "menu_htmx_demo_intro_panel.template_10"
```

Before the fix, the inherited base value survived and the menu demo rendered an
empty panel. The YAML materializer now resolves inherited nodes inside props,
maps, and arrays before rendering.

Runtime proof:

```text
go run ./cmd/hyperbricks start -m hyperbricks-patterns-yaml -p 18090 --non-interactive
curl -sS -D - http://127.0.0.1:18090/menu-demo
curl -sS -D - http://127.0.0.1:18090/fragments/status-demo-summary
```

Observed result:

- the converted module registered 56 routes
- `/menu-demo` rendered as a full page with doctype, head, menu shell, and
  landing panel content
- `/fragments/status-demo-summary` rendered with
  `X-Hyperbricks-Render-Error-Count: 0`

Automated proof:

```text
go test ./pkg/legacy-yaml-converter -count=1 -v
go test ./pkg/yaml-parser -count=1 -v
go test ./cmd/hyperbricks -run 'TestPreProcessAndPopulateConfigsLoadsConvertedPatternsYAMLModule|TestPreProcessAndPopulateConfigs.*YAML|TestProcessScript' -count=1 -v
```

The remaining render errors in full pages are from stale local plugin binaries
already present in the legacy module path:

```text
plugin was built with a different version of package github.com/hyperbricks/hyperbricks/assets
```

That is a local plugin artifact issue, not a YAML parser or converter contract
issue.
