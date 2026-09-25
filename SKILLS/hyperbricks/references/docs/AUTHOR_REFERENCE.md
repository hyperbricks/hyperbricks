<!-- Generated from docs/AUTHOR_REFERENCE.md. Do not edit directly. -->

# Author Command Reference

This document defines the input, output, validation, and write contracts of
`hyperbricks author`. It is intended for developers, agents, and automation that
need exact command behavior. Start with [Changing Existing Projects](AUTHOR.md)
for the task-oriented workflow.

All examples use structured output:

```sh
hyperbricks author <command> -m demo --json
```

`-m, --module` selects a module name or directory. `--config` selects a package
profile relative to that module and defaults to `package.hyperbricks.yaml`.

## Commands

| Command | Purpose |
| --- | --- |
| `context` | Load project configuration, source ownership, assets, and Space sources. |
| `context --list` | List effective roots without source or effective bodies. |
| `context --target NAME` | Return focused effective context, relevant template source, and an optional structural example. |
| `inspect --target NAME` | Inspect ownership, resolved components, template references, and editing contracts without source bodies. Repeat `--target` to inspect one snapshot in batch. |
| `recipes` | Return curated version 2 spec examples. |
| `explain-type TYPE` | Return the current schema fields and authoring hints for a registered component type. |
| `apply --spec FILE` | Validate and preview or apply one version 2 spec. Use `--spec -` to read JSON from stdin. |

`context --list` and `context --target` are mutually exclusive. `inspect`
requires at least one `--target`.

## Context contracts

Every context mode loads the complete configured project before shaping its
response. The returned `revision` therefore covers the same snapshot in full,
list, and focused modes.

### Full context

```sh
hyperbricks author context -m demo --json
```

| Field | Contents |
| --- | --- |
| `version` | Context format version; currently `2`. |
| `revision` | Snapshot hash used for stale-spec rejection. |
| `module` | Resolved module root. |
| `config` | Resolved package configuration path. |
| `package_yaml` | Complete package configuration source. |
| `directories` | Resolved `hyperbricks`, `templates`, `resources`, and `static` directories. |
| `files` | Loaded source files as `{path, yaml}`. Paths are relative to the module. The package file is reported separately. |
| `roots` | Effective loaded roots as `{name, file, effective}`. `file` is relative to the configured HyperBricks directory. |
| `assets` | Paths found below each configured template, resource, and static directory, relative to that directory. Asset contents are not returned. |
| `space_sources` | Active Hypermedia sources and their validated editable-field contracts. |

Effective objects contain runtime metadata such as `@type`. Do not copy runtime
metadata into author specs; author native fields through `brick.type`,
`brick.inherit`, `brick.properties`, and `brick.children`.

### List context

```sh
hyperbricks author context -m demo --list --json
```

The response contains `version`, `revision`, and `roots`. Each root contains:

| Field | Contents |
| --- | --- |
| `name` | Effective root name. |
| `type` | Canonical lowercase component type. |
| `file` | Owning file relative to the HyperBricks directory. |
| `route` | Effective route when present. |
| `page_source` | `true` for a route-less Hypermedia root. |
| `space_source` | `true` when Spaces recognizes the root as a source. |

List context omits YAML and effective bodies. An unknown focused or inspection
target reports a short candidate list and the `context --list --json` discovery
command.

### Focused context

```sh
hyperbricks author context -m demo --target scaffold_page --json
```

The response contains `version`, the full-project `revision`, `directories`, a
compact `roots` inventory, the requested `target`, its `effective` object, and
referenced `templates` as `{path, source}`. Dot-separated nested effective paths
are accepted.

For a Hypermedia root with `content.values.body`, focused context may also return
an `example` version 2 spec. The example selects a loaded scope in which the
inheritance reference resolves. When `body` is an editable scalar, the example
keeps it as scalar data instead of replacing the editing binding with a component.

## Version 2 specs

The spec input must contain exactly one JSON object. Unknown envelope and brick
fields are rejected, as are versions other than `2`.

### Envelope fields

| Field | Use |
| --- | --- |
| `version` | Required; must be `2`. |
| `revision` | Optional snapshot revision from context. When present, it must still match before planning. |
| `operation` | `add-root`, `add-child`, or `create-space` for a single operation. Omit for a batch. |
| `file` | Destination for `add-root`, relative to the HyperBricks directory. |
| `import_into` | Loaded file that should import a new nested `add-root` destination. |
| `target` | Effective root or dot-separated component path for `add-child`. |
| `brick` | Component definition. `create-space` uses only `brick.name`. |
| `source` | Existing Hypermedia source for `create-space`. |
| `title` | Space title for `create-space`. |
| `route` | Space route for `create-space`. Brick operations put routes in `brick.properties.route`. |
| `operations` | Non-empty ordered array for a batch. |

### Brick fields

| Field | Use |
| --- | --- |
| `name` | Required component name. For brick operations it starts with a letter, is at most 80 characters, otherwise uses letters, digits, `_`, or `-`, and cannot be `imports`, `vars`, `type`, or `inherit`. `create-space` applies the Spaces identity rules. |
| `type` | Required registered component type for `add-root` and `add-child`, including for an inheriting brick. Aliases are emitted as their canonical type. It must be omitted for `create-space`. |
| `inherit` | Optional named root or child-path reference in the resulting import scope. |
| `properties` | Native component fields as JSON values. `type`, `inherit`, unknown fields, and runtime metadata are not accepted here. |
| `property_order` | Optional priority order for supplied property keys. Every entry must exist exactly once. |
| `children` | Ordered nested brick definitions. |
| `slot` | Mount location when this brick is a child. A top-level `add-root` brick cannot set it. |

Version 2 bricks do not receive placeholder defaults. Nested schema paths are
expressed as nested JSON objects rather than dotted property keys.

## Operation contracts

### `add-root`

`add-root` appends a new named root to an existing or new
`*.hyperbricks.yaml` file.

- `file` is required. It must be a clean relative component-file path inside the
  configured HyperBricks directory.
- `target` and top-level `brick.slot` are not accepted.
- `source`, top-level `title`, and top-level `route` are not accepted. Put
  component fields in `brick.properties`.
- A nested file must already be loaded or supply `import_into`. The import parent
  must already be loaded, cannot be the destination itself, and cannot be a
  managed Spaces index.
- The new root must be reachable from a loaded top-level scope after planning.
- Root names and normalized routes cannot collide with loaded configuration.
  `route: index` represents the root route.
- Inheritance is resolved and the resulting component configuration is validated
  before a write is attempted.

An `inherit` reference is scoped by the resulting import graph. Separate
top-level files have separate runtime scopes.

### `add-child`

`add-child` appends a named component to an existing source-owned target.

- `target` is required; examples are `about_page` and
  `page_source.content`.
- `file` and `import_into` are rejected. The owning file is derived from the
  target root.
- Every target segment must exist in the owning native source. A path that exists
  only through inheritance requires an explicit source override, which this
  operation does not create.
- Managed Spaces import indexes cannot own child additions.
- An effective child or property with the same name is not overwritten.
- The changed graph, including effective inheritors, is validated before writing.

### `create-space`

`create-space` delegates to the Spaces creator. Its accepted payload is limited
to `source`, `brick.name`, `title`, and `route`, plus the envelope `version` and
optional `revision`.

The source must be an active Hypermedia Space source. Name, title, and route are
validated; component names and normalized routes must remain unique. Planning
creates the Space instance, creates or updates its managed import index, and adds
that index to the source owner when necessary. Declared editable source defaults
are copied into the instance.

`create-space` is a single operation and is not accepted inside a batch. Creating
a Space does not enable frontend editing in package configuration.

## Child slots

| `slot` | Contract |
| --- | --- |
| omitted | Mount as a regular child. For a values-child component such as `template`, it mounts below `values`. |
| `body` | Explicit regular body attachment; valid only for `hypermedia`. |
| `head` | Valid only for `hypermedia`. A `head` component is created when absent, then the child is attached there. |
| `values` | Explicit values attachment; valid only for component types with a values child model. |

Components whose schema has no child model reject children. A child name cannot
collide with an existing child, value, or native property.

## Batches

A batch envelope contains only `version`, optional `revision`, and a non-empty
`operations` array. Batch entries omit `version`, `revision`, and `operations`.
Nested batches are rejected.

Only `add-root` and `add-child` are supported in a batch. Operations run in array
order against one in-memory overlay, so a later operation can attach a child to a
root added earlier. Forward references to roots added later do not resolve.

Repeated changes to one file become one final planned write with the original
file as its precondition. All batch operations and the final effective project
are validated before the first file is written. Batching does not make a
multi-file apply globally transactional.

## Editing contracts

After brick operations are planned, HyperBricks validates the editing contracts
of every effective Hypermedia root. This includes inherited roots affected by a
source change.

An `editable` declaration belongs to a `template` or `markdown` component. Each
declared field must still resolve to a supported scalar value or reference.
Replacing an editable scalar with a component therefore fails unless the new
local configuration explicitly changes the editing contract. An empty
`editable` list is an explicit non-editable contract; `null` is not a deletion
marker.

## Inspection contract

```sh
hyperbricks author inspect -m demo --target about_page --json
```

A single target returns one inspection:

| Field | Contents |
| --- | --- |
| `version`, `revision` | Format version and current project snapshot. |
| `target`, `root`, `file`, `inherit` | Requested path, owning root/file, and source-level root inheritance when present. |
| `metadata` | Effective `route`, `title`, `section`, and `index` when present. |
| `components` | Resolved component paths and canonical types, plus source mode and reference where applicable. |
| `templates` | Sorted referenced template paths, without template bodies. |
| `editable_fields` | Editing paths and constraints, without current values or defaults. |
| `checks` | `loaded_configuration`, `resolved_inheritance`, `editing_contracts`, and `template_file_presence`. |
| `diagnostics` | Inspection findings, including missing template references. |

Repeating `--target` returns `{version, revision, results}`. Results preserve
request order and duplicates. Each result contains `target` and either
`inspection` or `error`; the shared revision stays on the envelope. Healthy
siblings remain in the response when another target fails, but any failed target
sets a nonzero exit status.

Inspection is read-only. It checks that referenced template files exist, not
template syntax, rendered bindings, HTTP behavior, or browser interactions. A
missing template is an inspection diagnostic with severity `error`; it is not a
target-resolution failure and does not become the response's top-level `error`.

## Apply output modes

`--dry-run` controls whether files are written. `--compact` and `--summary`
control the report shape.

| Invocation | Writes | Status | Main fields |
| --- | ---: | --- | --- |
| `--dry-run --json` | No | `preview` or `error` | `plan.files` with optional `before` and complete `after` YAML. |
| `--dry-run --compact --json` | No | `preview` or `error` | `changes`, `emissions`, optional `diagnostics`; no `plan`. |
| `--compact --json` | Yes | `created` or `error` | `changes`, `emissions`, optional `diagnostics`; no `plan`. |
| `--summary --json` | Yes | `created` or `error` | `input_revision`, `operations`, `changes`, optional `diagnostics`; no source bodies. |
| `--json` | Yes | `created` or `error` | Full `plan`, including source bodies. |

`--summary` cannot be combined with `--dry-run` or `--compact`.
The main fields above describe successful responses and errors that occur after
a plan exists. Earlier failures can contain only `status` and `error`.

### Full plan

`plan` contains `name`, `type`, `command`, and ordered `files`. Each file contains
`path`, `action` (`create` or `modify`), optional `before`, and complete `after`
source. New files omit `before`.

### Compact report

`changes` contains each planned file and action. For HyperBricks YAML files it
also reports exact `imports_added` and `imports_removed` references.

`emissions` contains the newly authored native YAML blocks and their
`operation`, `name`, optional `target`, `slot`, `route`, and `section`. An
`add-child` emission is the new named child, not the complete parent. For
`create-space`, emissions contain created-file YAML; modified import files remain
represented by `changes`.

### Summary report

`operations` contains operation/name/target/slot/route/section metadata without
YAML. `input_revision` is the revision supplied by the spec, not the resulting
project revision. Run `inspect` or `context` after applying to obtain the new
revision.

When present in a compact or summary response, `changes` describes the plan. It
is not a write-completion log.

## Diagnostics and errors

Apply `diagnostics`, when present, are advisory warnings. They do not replace the
error channel and do not by themselves reject or rewrite a spec. The current
`brick_emitted_as_data` warning identifies an object inside ordinary properties
that resembles a version 2 brick but will remain plain data.

An apply failure is always identified by these fields; mode-specific report
fields may also remain present:

```json
{
  "status": "error",
  "error": "failure message"
}
```

The CLI also sets a nonzero exit status. Failures are reported in the top-level
`error` string, not as an error diagnostic. A response can contain advisory
`diagnostics` alongside that failure.

Inspection diagnostics have a separate purpose: they report inspection findings
and may use severity `error`, such as for a missing template reference.

## YAML emission

New component YAML is emitted in this order:

1. `type`;
2. optional `inherit`;
3. keys named by `property_order`;
4. remaining common fields in this order: `route`, `title`, `section`, `index`,
   `template`, `inline`, `file`, `content`, `value`, `editable`, `values`;
5. remaining properties alphabetically;
6. children in spec order.

Multiline strings use literal scalar style. A string in `properties.template` is
emitted as the native `template: {file: ...}` structure. Other nested property
values retain their JSON/YAML structure.

Authoring edits the YAML node tree. Existing comments and scalar styles are
retained where supported by the YAML library, but re-encoding can normalize
whitespace and formatting. Use the full preview to review surrounding changes.

## Revisions and stale changes

The context revision hashes the package configuration, the paths and contents of
loaded HyperBricks sources, and the paths and contents of files below the
configured templates, resources, and static directories.

When a spec supplies `revision`, `apply` loads the project again and rejects a
mismatch before planning with `project changed since context; discover and
prepare again`. Omitting `revision` omits this snapshot-level check.

Every plan still records the sources and destinations it depends on. Immediately
before writing, it rejects a changed source, a planned destination that appeared
unexpectedly, or a changed top-level source set. Files are checked again at their
individual replacement boundary.

## Writes and partial recovery

Writes use clean contained paths, reject editable symlinks, and write files one
at a time through a temporary sibling after checking the planned prior state.

A multi-file apply is not one filesystem transaction. If a later file fails,
files completed earlier remain written. The top-level `error` includes
`completed files retained: [...]` for recovery. Consult that error and inspect
the filesystem; do not infer completed writes from `changes`.

After a partial failure, reload context before preparing another spec.

## Current limitations

- The command adds roots and children and creates one Space; it does not generally
  edit existing scalar fields, replace existing children, delete roots, or remove
  routes.
- Batches support `add-root` and `add-child`, not `create-space`.
- `add-child` cannot materialize an override for an inherited-only target path.
- Author specs do not create templates, resources, static assets, or package
  configuration. Referenced files must already exist unless another dedicated
  workflow creates them.
- Planning validates configuration, inheritance, editing contracts, supported
  referenced files, and applicable template syntax. It does not render routes,
  start the application server, call HTTP endpoints, or test browser behavior.
- Recipes and focused examples provide structure, not application intent. The
  caller must choose names, routes, composition, bindings, and content.

The implementation sources for this contract are
[`author.go`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.5-beta/cmd/hyperbricks/commands/author.go),
[`author_workflow.go`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.5-beta/cmd/hyperbricks/commands/author_workflow.go),
[`author_inspect.go`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.5-beta/cmd/hyperbricks/commands/author_inspect.go), and the
shared planning/writing code in
[`scaffold_source.go`](https://github.com/hyperbricks/hyperbricks/blob/v1.2.5-beta/cmd/hyperbricks/commands/scaffold_source.go).
