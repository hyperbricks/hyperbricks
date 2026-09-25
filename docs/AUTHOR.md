# Changing Existing Projects with `hyperbricks author`

This guide is for developers, agents, and automation that need to change an
existing HyperBricks project non-interactively.

`hyperbricks author` reads the loaded project before it plans a change. It takes
source ownership, imports, inheritance, routes, templates, and editing contracts
into account. A structured JSON spec describes the requested change; `author`
validates it, previews the resulting native YAML, applies it, and can inspect the
result.

## Choose the right command

Do not choose `author` only because the caller is an agent.

| Command | Use it when |
| --- | --- |
| `hyperbricks scaffold` | A built-in starter matches the root you want to create. It supports an interactive wizard and a flag-driven non-interactive mode. |
| `hyperbricks space` | You want to create one inheriting Space from an existing Hypermedia source. |
| `hyperbricks author` | The change must fit existing roots, children, ownership, imports, inheritance, editing contracts, nested files, or a batch. |

See the CLI reference for the [`scaffold`](HYPERBRICKS_CLI.md#scaffold) and
[`space`](HYPERBRICKS_CLI.md#space) workflows.

## Current scope

`author` currently supports:

- adding a root;
- adding a child to an existing source-owned component;
- creating one Space;
- batching multiple root and child additions.

It does not currently:

- edit existing scalar values or replace existing children;
- replace existing roots or routes;
- add a child to a path that exists only through inheritance;
- create project assets;
- create multiple Spaces in one batch;
- decide which application structure or component semantics you intend.

## Basic workflow

Run these commands from the project root. The example below adds an `/about`
page to the `demo` module.

### 1. Discover the project

Start with the compact root list:

```sh
hyperbricks author context -m demo --list --json
```

Use returned names when you target or inherit existing components. For new roots
and files, choose names that do not collide with the inventory. Copy the returned
`revision` into your spec.

If the change extends or inherits an existing component, request focused context:

```sh
hyperbricks author context -m demo --target scaffold_page --json
```

Focused context includes the effective target, its owner and relevant templates.
For matching page shells it can also include a structural example to adapt.

### 2. Prepare one spec

Save the following as a temporary `spec.json`, replacing the revision placeholder
with the value returned by `context`:

```json
{
  "version": 2,
  "revision": "<context revision>",
  "operation": "add-root",
  "file": "about.hyperbricks.yaml",
  "brick": {
    "name": "about_page",
    "type": "hypermedia",
    "properties": {
      "route": "about",
      "title": "About"
    },
    "children": [
      {
        "name": "content",
        "type": "markdown",
        "properties": {
          "content": "# About\n\nHello.\n"
        }
      }
    ]
  }
}
```

The CLI does not save the input spec. Use a temporary file or pass it through
stdin with `--spec -`. Keep a spec in the project only when it is intentionally
part of that project.

### 3. Preview the exact YAML

```sh
hyperbricks author apply -m demo --spec spec.json --dry-run --json
```

Review each planned file's existing source, when present, and its complete
resulting YAML. A dry run writes nothing.

### 4. Apply the same spec

```sh
hyperbricks author apply -m demo --spec spec.json --summary --json
```

Do not rebuild the spec between preview and apply. If the project changed since
the recorded revision, `author` rejects the operation and you must read context
again.

### 5. Inspect the result

```sh
hyperbricks author inspect -m demo --target about_page --json
```

Inspection reports the current owner, effective component structure, templates,
route metadata, editing fields, and diagnostics without returning source bodies.
Repeat `--target` to inspect several changed owners in one snapshot.

### 6. Test runtime behavior when it matters

Authoring validation does not render routes or serve the application. For a page
or endpoint change, start the module and request the affected route:

```sh
hyperbricks start -m demo --port 8080
```

In another terminal:

```sh
curl -i http://localhost:8080/about
```

Also test browser interactions, request input, and external services when the
change depends on them.

The workflow above is enough for a new top-level page. Continue only when your
change involves Spaces, existing components, inheritance, nested files, or
batches.

## Operations

| Operation | Purpose |
| --- | --- |
| `add-root` | Add a page, fragment, reusable component, or Space source to an existing or new YAML file. |
| `add-child` | Add a named child to a component that exists in its owning source file. |
| `create-space` | Create one Space from a discovered Hypermedia source. |
| Batch (`operations`) | Apply several `add-root` and `add-child` operations under one shared revision. |

Use `hyperbricks author recipes --json` for maintained examples and
`hyperbricks author explain-type TYPE --json` to inspect the current fields for a
component type.

### Create a Space

Use `create-space` to create one Space that inherits from an existing Hypermedia
Space source:

```json
{
  "version": 2,
  "revision": "<context revision>",
  "operation": "create-space",
  "source": "page_source",
  "brick": {
    "name": "about"
  },
  "title": "About",
  "route": "about"
}
```

Choose a root with `space_source: true` from
`hyperbricks author context -m MODULE --list --json`, then preview and apply the
spec as shown in the basic workflow. This creates the instance and required
import; it does not enable browser editing by itself. See [Spaces](SPACES.md) for
the content workflow and
[the command reference](AUTHOR_REFERENCE.md#create-space) for all fields.

## Working with the existing structure

### Source ownership

`add-child` resolves the target's owning file and writes there. Supply `target`,
but do not supply `file` or `import_into`. The target path must exist in native
source; an inherited-only path cannot be changed implicitly.

### Imports and nested files

Top-level `*.hyperbricks.yaml` files load automatically. A new file in a nested
directory needs both its `file` path and an `import_into` file that is already
loaded. Both paths are relative to the configured HyperBricks directory.

### Inheritance

An `inherit` reference must resolve in the resulting import scope. Use focused
context to find the actual source owner, imports, and page structure before
building an inherited root.

### Editing contracts

When a changed source has inheriting pages, `author` validates their effective
editing contracts too. It does not silently replace an editable scalar with a
component or disable editing. Such a change requires an explicit contract change.

### Revision guard

The context revision represents the loaded project snapshot. Including it in the
spec prevents a reviewed change from being applied after the project has changed.
After a stale-revision error, read context and preview again.

## Advanced workflows

### Inherited pages

Request focused context for the page source or shell. When it returns an inherited
page example, adapt its names, route, content, and navigation metadata rather than
copying assumed starter names. Preserve scalar fields required by its editing
contract unless you intentionally revise that contract.

### Nested files

For a root in a nested file, add these fields to the operation:

```json
{
  "file": "pages/guides.hyperbricks.yaml",
  "import_into": "app.hyperbricks.yaml"
}
```

`import_into` names the loaded file that should import the new file. It is not a
field inside `brick`.

### Batches

A batch has one `version` and `revision`, with ordered `operations`. It accepts
`add-root` and `add-child`; create each Space separately. Later operations can add
children to roots created earlier in the same batch, but forward references are
not supported. All operations are planned and validated before writing. A
multi-file apply is not one global filesystem transaction.

## Validation boundaries

| Check | Where it happens |
| --- | --- |
| Exact before-and-after native YAML | Full `apply --dry-run --json` preview |
| Component fields, source structure, imports, inheritance, name and route collisions, referenced files, and applicable template syntax | Preview |
| Ownership, effective components, route metadata, editing contracts, and referenced template-file presence | `inspect` |
| Rendered HTML, component execution, HTTP behavior, browser or HTMX behavior, request data, and external services | Separate runtime test |

Preview is source validation, not a complete asset analysis or proof of rendered
behavior. Read diagnostics as well as the command's exit status.

## Further reference

- [Author command reference](AUTHOR_REFERENCE.md) — exact JSON fields, command
  output, flags, diagnostics, write behavior, and current limitations.
- [HyperBricks CLI](HYPERBRICKS_CLI.md#author) — command overview and flags.
- [YAML usage](YAML_USAGE.md) — imports, inheritance, resolvers, and source syntax.
- [Component reference](REFERENCE.md) — component fields and supported values.
- [Spaces](SPACES.md) — Space sources, editing contracts, and content workflows.
- [HyperBricks type examples](HYPERBRICKS_TYPE_EXAMPLES.md) — complete native YAML
  examples for built-in types.
