# YAML Migration Next Pass

Date: 2026-05-28
Branch: `codex/dsl-migration-sprint`

This document continues:

- `logs/20260527-yaml-ordered-source-model.md`
- `logs/20260527-yaml-runtime-test-audit.md`

The previous work proved that a strict HyperBricks YAML profile can materialize
into the existing runtime-compatible map shape. This pass is about hardening the
contract before the migration is broadened.

## Current Position

The migration is past the first viability checkpoint.

Implemented and proven:

- YAML source files can be loaded by the normal runtime loader.
- The YAML parser/materializer emits normal `map[string]interface{}` data for
  the existing registry, typefactory, and render pipeline.
- `inherit` is a HyperBricks YAML source feature. It resolves before runtime,
  like legacy `<<<`, and the runtime only sees the materialized result.
- Source sequence order materializes to `@order`.
- Imports, env/config/var markers, path markers, `{{TEMPLATE:...}}`, and
  `{{FILE:...}}` have YAML-safe coverage.
- `modules/hyperbricks-patterns-yaml` runs through the real runtime.
- Plugin-backed YAML pattern routes can now be rebuilt and smoke-tested through
  `./tests.sh --with-plugins`.

Recent proof commit:

```text
929ab91 Add plugin smoke coverage for YAML patterns
```

## Ownership Boundary

Keep this boundary strict:

```text
YAML parser/materializer
  owns source semantics:
  - inherit
  - duplicate item name normalization
  - sequence order -> @order
  - imports
  - preprocessing markers

Runtime/render layer
  owns rendering of already-normalized config:
  - mapstructure decode
  - component rendering
  - composite Items traversal
  - plugin execution
```

The runtime should not learn about YAML-only concepts such as `inherit` or
duplicate source item normalization.

## Duplicate Item Names Decision

YAML source uses ordered sequences to represent component entries. That means
the user may naturally write the same item name twice:

```yaml
fragment:
  - type: fragment

  - text_summary:
      - type: text
      - value: Intro

  - text_summary:
      - type: text
      - value: Summary
```

Duplicate child names are not recommended source because item names become
paths. The parser/materializer should report a clear diagnostic, then recover by
assigning deterministic unique runtime keys when recovery is enabled.

Runtime loading uses recovery: the runtime should try to render the normalized
map. Documentation and CI tooling may choose to fail on this diagnostic as a
quality policy, but hard-failing is not the parser contract itself.

Contract:

- duplicate child names produce a diagnostic
- runtime-load mode recovers automatically and keeps rendering
- recoverable YAML source diagnostics are attached to the existing route render
  error pipeline
- docs/CI may fail on the diagnostic as quality policy
- the first occurrence keeps the original name
- the second occurrence receives `_2`
- the third occurrence receives `_3`
- if the generated name already exists, keep incrementing until the key is
  unique in that local item scope
- generated names become real runtime paths
- `@order` must use the generated runtime keys
- inheritance and local overrides must refer to the generated runtime key after
  materialization
- the runtime render layer must receive only the normalized map; it must not
  learn about duplicate YAML source entries

Current runtime error-pipeline direction:

- the render diagnostics endpoint contract stays unchanged
- recoverable YAML source diagnostics are attached as existing
  `shared.ComponentError` entries when an affected route renders
- the runtime still renders best-effort; the error entry is diagnostic only
- users can inspect these through the existing request-bound render diagnostics
  flow
- warning-only source diagnostics must not be written as warning/error noise to
  stdout

Example materialized child keys:

```json
{
  "@order": ["text_summary", "text_summary_2"],
  "text_summary": {
    "@type": "<TEXT>",
    "value": "Intro"
  },
  "text_summary_2": {
    "@type": "<TEXT>",
    "value": "Summary"
  }
}
```

Important: this is not display naming. It is address normalization. Once
materialized, `text_summary_2` is the actual path.

Example diagnostic:

```text
duplicate child "text_summary" at fragment; materialized second occurrence as "text_summary_2"
```

The legacy converter follows the same recovery rule for generated-name
collisions. It should write unique YAML output and print warnings so the user
can clean the source later.

Progress on 2026-05-28:

- strict `ParseBytes` still rejects duplicate child names
- runtime-load mode enables duplicate-child recovery
- deep nested duplicate children are normalized before materialization
- normalized names are included in the generated `@order`
- existing sibling names such as `hero_2` are respected; generated names keep
  incrementing until unique
- runtime routes keep rendering the normalized map
- recoverable YAML source diagnostics are converted to `shared.ComponentError`
  and attached to the affected route
- the existing `/__hyperbricks/render-diagnostics?request_id=...` contract is
  unchanged
- warning-only YAML source diagnostics are recorded in render diagnostics
  without producing the `Render diagnostics recorded` error log line

User story:

```text
As a HyperBricks author migrating a legacy page to YAML,
I accidentally define the same child name twice inside a TREE because both
blocks describe summary text.

I want the runtime to render both blocks in the source order instead of dropping
one block or failing the route,
so that the page remains usable while I inspect and clean up the migrated source.
```

Example source:

```yaml
page:
  - type: hypermedia
  - route: duplicate-demo
  - main:
      - type: tree
      - text_summary:
          - type: text
          - value: Intro
      - text_summary:
          - type: text
          - value: Summary
```

Expected runtime behavior:

- the route renders both `Intro` and `Summary`
- the materialized children become `text_summary` and `text_summary_2`
- `@order` becomes `["text_summary", "text_summary_2"]`
- render diagnostics for that request include a YAML warning such as:

```text
duplicate child "text_summary" at page.main; first defined at line ...; using "text_summary_2" as runtime path
```

Important alignment note: this is not a new diagnostics endpoint, not a SaaS
contract change, and not a general config-warning layer. It is only a YAML
source diagnostic attached to the existing render error pipeline for the route
that was rendered.

## Inheritance Contract

YAML `inherit` remains equivalent in outcome to legacy copy-by-reference:

```yaml
video:
  - inherit: myComponent
  - values:
      src: override
```

Materialization flow:

```text
parse YAML
  -> build ordered Document/Node model
  -> resolve inherit reference
  -> deep-copy referenced node
  -> merge local overlay
  -> resolve nested/value-mounted inherited nodes
  -> emit runtime map
```

Runtime must never receive `inherit`.

Duplicate-name normalization must happen early enough that inherited child paths
remain stable and addressable. The safest model is:

```text
parse node sequence
  -> normalize duplicate child names inside that local sequence
  -> validate reserved names and property/child collisions
  -> resolve inherit
  -> materialize
```

## Deterministic Render Contract

The public YAML contract should be deterministic.

Rules already accepted:

- ordered component children render according to `@order`
- nested component children carry their own local `@order`
- data maps under fields such as `values`, `headers`, `queryparams`, plugin
  `data`, and normal arrays/maps are data, not render order
- `TEMPLATE.values` may remain sorted map data when rendered by the component
- TREE/composite children are the order-sensitive layer

Open deterministic edge cases to keep visible:

- generated head items such as `generator` and `payload`
- runtime-injected items that are not listed in `@order`
- fallback behavior when legacy `.hyperbricks` data has no `@order`
- whether YAML-generated `@order` should be strict for YAML paths and legacy
  sorted fallback should be legacy-only
- missing or extra keys in a runtime `@order` map

Current direction:

- YAML should emit `@order` for every ordered child sequence.
- Generated runtime items should be named and represented in `@order`.
- Legacy fallback can stay as a migration bridge, but should not define the new
  YAML authoring contract.
- If runtime receives unlisted renderable child keys, append them in
  deterministic alphanumeric order and emit a render diagnostic.
- If runtime receives missing keys in `@order`, skip the missing key and emit a
  render diagnostic.

## Fixture And Documentation Direction

YAML fixtures are both tests and documentation.

Continue using:

```text
test/docs/hyperbricks-yaml-test-files/*.hyperbricks.yaml.test
```

Fixture sections should stay readable:

```text
==== hyperbricks yaml {!{scope}} ====
...
==== explainer ====
...
==== expected materialized json ====
...
==== expected output ====
...
```

Guardrails:

- keep old `.hyperbricks` fixtures as reference only
- do not copy old source files into the YAML fixture directory
- keep expected JSON/output embedded in the YAML fixture when it proves behavior
- prefer curated examples over one generated fixture for every field
- field-level docs can stay compact; deeper behavior belongs in feature
  fixtures

## Next Implementation Pass

### 1. YAML Dependency Hardening

The original implementation used:

```go
import "gopkg.in/yaml.v3"
```

That package path points at the old `go-yaml/yaml` line. It is stable, but the
project is now effectively frozen/legacy. The maintained YAML-org continuation
is:

```go
import "go.yaml.in/yaml/v4"
```

Initial compatibility check:

- our parser uses the classic API surface: `yaml.Node`, `yaml.NewDecoder`,
  `Decode`, `KnownFields`, and node kinds such as `DocumentNode`,
  `MappingNode`, `SequenceNode`, `ScalarNode`, and `AliasNode`
- `go.yaml.in/yaml/v4` keeps that API available
- v4 classic mode keeps v3-compatible defaults where possible
- this should be treated as a dependency-hardening pass, not as a YAML contract
  redesign

Migration checks:

- block scalars still preserve multiline content
- YAML comments remain handled by the YAML parser
- `!!null` still materializes to the current empty-string contract
- anchors/aliases are still rejected by HyperBricks source validation
- duplicate map keys and duplicate child handling remain explicit HyperBricks
  decisions, not accidental library behavior
- fixture materialization output remains stable

Proof commands for this pass:

```text
go test ./pkg/yaml-parser -count=1
go test ./test/docs -run TestYAMLProfile -count=1
go test ./test/docs -run TestYAMLDocumentation -count=1
go test ./cmd/hyperbricks -run TestPreProcessAndPopulateConfigsLoadsConvertedPatternsYAMLModule -count=1
```

Applied on 2026-05-28:

```text
pkg/yaml-parser/parser.go now imports go.yaml.in/yaml/v4
go.mod requires go.yaml.in/yaml/v4 v4.0.0-rc.4
gopkg.in/yaml.v3 is no longer a direct HyperBricks parser dependency
```

Verification after applying the dependency switch:

```text
go test ./pkg/yaml-parser -count=1
go test ./test/docs -run TestYAMLProfile -count=1
go test ./test/docs -run TestYAMLDocumentation -count=1
go test ./cmd/hyperbricks -run TestPreProcessAndPopulateConfigsLoadsConvertedPatternsYAMLModule -count=1
go vet ./...
go test ./... -count=1
```

Note: `gopkg.in/yaml.v3` can still appear in `go.sum` transitively. At the
time of the switch, `go mod why -m gopkg.in/yaml.v3` traced it through
`go.uber.org/zap/zapcore.test`, not through the HyperBricks YAML parser.

### 2. Duplicate Item Name Normalization

Implemented in `pkg/yaml-parser` for runtime-load recovery, with tests for:

- duplicate child names emit diagnostics
- runtime-load/recovery mode materializes normalized keys and keeps going
- duplicate child names become `_2`, `_3`
- `@order` contains generated names
- existing `name_2` collision increments further
- duplicates are scoped locally, not globally
- reserved names still fail
- property/child collisions still fail
- top-level duplicate root names still fail

Still open:

- docs/CI callers can choose to fail on duplicate diagnostics
- legacy converter generated-name collisions are renamed with warnings

Likely fixture:

```text
test/docs/hyperbricks-yaml-test-files/duplicate-item-names.hyperbricks.yaml.test
```

### 3. Inheritance Plus Generated Paths

Add coverage proving that normalized duplicate names remain addressable.

Needed cases:

- a duplicated child can be inherited by its generated path
- inherited duplicate-name children preserve stable order
- local override of an inherited generated child updates the intended child

### 4. Deterministic Generated Items

Write tests for named generated items before broadening the converter.

Needed cases:

- `HEAD` generated `generator` item appears deterministically
- generated CSS/JS payload item appears deterministically
- explicit user-defined generated item name overrides the default
- generated item order is controlled by `@order`

### 5. YAML Runtime Acceptance Expansion

Broaden runtime integration after the source contract is stable.

Candidates:

- more routes from `modules/hyperbricks-patterns-yaml`
- API fragment cases with local/mock endpoints
- guard + fragment combinations
- menu section ordering through YAML routes
- plugin-backed routes with rebuilt binaries

### 6. Converter Hardening

Only after the above contract is stable:

- convert duplicate legacy numeric or semantic keys into normalized YAML names
- reduce generated names such as `template_10` and `plugin_10` where safe
- keep converter output readable enough to serve as migration documentation
- keep generated paths stable for inheritance references

## Proof Commands For This Pass

Run the narrow source-contract checks first:

```text
go test ./pkg/yaml-parser -count=1
go test ./test/docs -run TestYAMLProfile -count=1
```

Then runtime integration:

```text
go test ./cmd/hyperbricks -run TestPreProcessAndPopulateConfigsLoadsConvertedPatternsYAMLModule -count=1
bash scripts/plugins/build_hyperbricks_plugins.sh
bash scripts/plugins/test_hyperbricks_patterns_plugins.sh
```

Full confidence before a major migration checkpoint:

```text
go vet ./...
go test ./... -count=1
./tests.sh --with-plugins
```

Use `./tests.sh --with-docs --with-plugins` only when intentionally
regenerating documentation artifacts.

## Success Criteria

This pass is complete when:

- duplicate YAML item names materialize to stable unique runtime paths
- duplicate YAML item names emit clear diagnostics
- runtime-load mode recovers duplicate child names with diagnostics and renders
  the normalized map
- docs/CI can fail on duplicate-name diagnostics as quality policy
- YAML parsing uses the maintained `go.yaml.in/yaml/v4` package unless a tested
  incompatibility blocks the migration
- inheritance works against normalized paths
- every ordered YAML child sequence emits accurate `@order`
- generated runtime items are deterministic and tested
- YAML fixtures clearly show the source, materialized JSON, and rendered output
- plugin-backed YAML patterns still pass the smoke tests
- no new YAML semantics leak into the runtime render layer
