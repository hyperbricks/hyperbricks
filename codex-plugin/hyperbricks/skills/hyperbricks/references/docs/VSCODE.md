<!-- Generated from docs/VSCODE.md. Do not edit directly. -->

# Visual Studio Code

HyperBricks includes a language server and a matching Visual Studio Code
extension for `*.hyperbricks.yaml` source. The runtime executable remains the
source of truth for component types, fields, validation, formatting, and runtime
diagnostics; the extension does not carry a separately maintained DSL schema.

Use an extension build and `hyperbricks` executable from the same HyperBricks
revision. The editor protocol is versioned separately from the public Language
Server Protocol and rejects incompatible clients with an explicit message.

## Feature quick reference

| Feature | How to use it | Scope and result |
| --- | --- | --- |
| Syntax highlighting | Open a `*.hyperbricks.yaml` file and confirm the language mode is **HyperBricks YAML**. | Theme-driven TextMate scopes distinguish declarations, native type values, reserved fields, inheritance, resolvers, paths, and Go-template expressions. |
| Static diagnostics | Edit an owned source file, an untitled HyperBricks buffer, or the selected package profile and open VS Code's **Problems** panel. | Unsaved buffers are checked for YAML shape, native component fields, imports, inheritance, and package configuration. Corrected findings clear immediately. |
| Completion and snippets | Type normally or run **Trigger Suggest** in a `type`, component field, `inherit`, resolver, template, or recognized resource-path position. | Suggestions come from the executable's schema and effective import graph. Dotted object-path completion is limited to runtime-valid `inherit` references. |
| Hover help | Hover a native `type` value or schema-owned field. | Shows the matching description from the Go schema registry and, for fields, an example when available. Inherited components use their effective native type. |
| Go to Definition | Press F12 or Cmd/Ctrl-click an import, `inherit` value, `template.file`, or recognized local resource path. | Opens the effective declaration or existing file, including targets reached through transitive imports and unsaved buffers. |
| Formatting | Run **Format Document**, or separately enable VS Code's format-on-save setting. | Formats the complete document with two-space indentation while preserving ordered entries, mapping order, comments, scalar styles, and parsed meaning. |
| Module health check | Run **HyperBricks: Run Module Check**. | Runs `hyperbricks doctor` against the saved module and writes its complete report to the HyperBricks output channel. |
| Runtime feedback | Start the selected module in development or debug mode and use automatic discovery or **Connect Runtime Diagnostics**. | Adds safely mapped render issues to **Problems**, route coverage to the status bar, and the browser Errors view when the runtime advertises it. |

Component source and package configuration are different contracts even when a
profile also ends in `.hyperbricks.yaml`. Lexical highlighting works as soon as
VS Code recognizes the language. An untitled component buffer can also receive
isolated static language features, but a file-backed source must have a safely
selected owning module before it is sent to the language server. Import graphs
and module-local path completion remain confined to that module's configured
HyperBricks source directory. The selected package profile is validated as
package configuration rather than as a component tree.

## Install and first use

Use a VSIX and `hyperbricks` executable built from the same revision. The
extension is not currently installed from a marketplace.

If you do not already have a VSIX, build one from the checkout's
`editors/vscode/` directory:

```bash
npm ci
npm run package
```

The package command compiles the extension first and writes the versioned VSIX
in that directory.

1. Install the resulting VSIX with **Extensions: Install from VSIX...**, or run:

   ```bash
   code --install-extension path/to/hyperbricks-vscode-VERSION.vsix
   ```

2. Run **Developer: Reload Window**, open a trusted local HyperBricks workspace,
   and open a `*.hyperbricks.yaml` file.
3. Confirm that the language mode is **HyperBricks YAML**. If the status bar says
   **select module**, configure the owning module explicitly. Use
   **HyperBricks: Show Output** to verify the selected module and package profile.
4. Leave `hyperbricks.module` empty for automatic module ownership. Set
   `hyperbricks.executable` only when the matching executable is not available
   as `hyperbricks` on `PATH`, or when the workspace must use a checkout-specific
   build.

For extension development, compile the source and launch the tracked Extension
Development Host configuration:

```bash
npm run compile
```

The VSIX is a local build artifact and is not part of the HyperBricks runtime
archive. Publishing the extension is a separate release action.

## Language server

The extension starts:

```bash
hyperbricks language-server --stdio
```

The process communicates over standard input and output using LSP 3.17. Do not
send ordinary log output to its standard output; protocol diagnostics and trace
messages belong on standard error or in the editor's HyperBricks output view.

The extension supplies the selected module and package profile during
initialization. When `hyperbricks.module` is empty, it selects the nearest
ancestor of the active HyperBricks source that owns the configured package
profile. This supports a repository workspace containing several modules; moving
between modules restarts and scopes the client to the newly selected package.
The server keeps unsaved documents in an in-memory overlay, so fast diagnostics,
completion, and definition lookup reflect the editor buffer instead of lagging
one save behind.

## Settings

| Setting | Default | Change it when |
| --- | --- | --- |
| `hyperbricks.executable` | `hyperbricks` | The matching executable is not on `PATH`, or the workspace should use a checkout-specific build. |
| `hyperbricks.module` | empty (automatic) | Automatic ownership cannot identify the intended module, or another module must be selected explicitly. |
| `hyperbricks.config` | `package.hyperbricks.yaml` | The selected module uses a differently named package profile. |
| `hyperbricks.runtimeDiagnostics` | `auto` | Runtime feedback must report unavailability (`on`) or must not connect automatically (`off`). |
| `hyperbricks.runtimeUrl` | empty | The extension should use an explicit HTTP(S) runtime origin instead of local discovery. |
| `hyperbricks.trace.server` | `off` | You need `messages` or `verbose` language-server protocol tracing. |

`auto` connects automatically, using `runtimeUrl` when set or local discovery
otherwise, and quietly skips profiles such as live mode where diagnostics are
unavailable. `on` uses the same connection rules but reports unavailable
profiles and configuration errors. `off` disables automatic connection; the
manual Connect command remains available when the URL setting is empty or
valid.

Before constructing the language client, the extension requires every nonempty
runtime URL to use HTTP or HTTPS, include a host, and contain no username or
password. It sends only the URL origin, discarding any path, query, or fragment.
A malformed or credential-bearing value disables automatic and manual runtime
connections without disabling static language features; clear it to restore
local discovery. The original configured value is not sent through the language
protocol or written to extension logs.

Changing executable, module, or package-profile settings restarts the language
server so that the parser, schema, directories, and runtime identity remain from
one project snapshot.

Protocol version 1 follows one active workspace folder. In a multi-root window,
activating a HyperBricks document in another folder restarts the client for that
folder. Use separate VS Code windows when two modules need simultaneous
language-server sessions without that handoff.

## Commands

Open the Command Palette and use:

| Command | Use it when | Result or availability |
| --- | --- | --- |
| **HyperBricks: Restart Language Server** | Recovering from a server failure or manually reloading source intelligence. | Restarts the client with the current executable, module, profile, and runtime settings. |
| **HyperBricks: Connect Runtime Diagnostics** | Requesting a manual connection to a development or debug runtime. | Attempts the connection; the configured or discovered runtime must also be reachable, compatible, and authorized. |
| **HyperBricks: Disconnect Runtime Diagnostics** | Temporarily stopping runtime feedback. | Leaves highlighting, static diagnostics, completion, navigation, hover, and formatting active. |
| **HyperBricks: Run Module Check** | Checking the complete saved module and package profile. | Runs `hyperbricks doctor` and writes its full report to the HyperBricks output channel. |
| **HyperBricks: Open Runtime Errors** | Inspecting rendered failures in the browser. | Opens the view when the connected runtime advertises it; otherwise explains why it is unavailable. |
| **HyperBricks: Show Output** | Diagnosing executable, module, runtime, or protocol problems. | Opens the HyperBricks output channel. |

Doctor remains a saved-project health check; live typing diagnostics come
directly from the language server.

## Static feedback

Static Problems entries use source `hyperbricks-static`. The server reports the
tightest available source range for malformed YAML, invalid ordered-object
shape, unknown native types, unsupported native fields, missing required fields,
duplicate or reserved entries, and inheritance failures.

A fatal YAML error can prevent further analysis of that document. Fix the first
fatal structural error before treating the remaining Problems list as complete.
Protocol version 1 does not ingest plugin-owned schemas. When the selected
profile enables plugins, unknown types are warnings because their schema may be
plugin-owned; their fields remain structurally editable and are not validated.

## Completion and hover

Use normal typing or VS Code's **Trigger Suggest** command for:

- native component type values after `type:`;
- schema-owned fields and child slots inside an ordered component;
- resolver structures and path-base names;
- existing local template and resource paths in recognized schema fields; and
- reachable inheritance targets after `inherit:`.

Inheritance completion follows transitive imports and unsaved open buffers. It
also completes one dotted segment at a time. For example, after:

```yaml
probe:
  - inherit: about_page.b
```

the editor can offer and insert `body` without duplicating the already typed
`about_page.` prefix. These are inheritance paths, not arbitrary YAML object
paths: the runtime traverses roots and component children, but it does not
traverse ordinary mappings such as `about_page.body.values.content`.

Hover a native `type` value or schema-owned field to see its description and a
maintained example when one is available. Protocol version 1 does not provide
plugin-owned schema completion or hover, AI-generated code, Quick Fixes, or
general code actions.

## Source navigation

Use **Go to Definition**, F12, or Cmd/Ctrl-click on:

- a file listed under `imports` to open that HyperBricks source;
- an `inherit` value to open the matching component declaration, including a
  declaration reached through transitive imports;
- `template.file` to open the configured template file; or
- a local file/path resolver value to open the existing resource it resolves.

Navigation follows unsaved imported buffers and the selected package's
configured directories. It returns no destination for a missing path,
unreachable component, filesystem traversal, or symlink that resolves outside
the selected module. This keeps editor navigation under the same ownership
boundary as diagnostics and completion.

## Formatting

HyperBricks component objects are ordered YAML sequences. The formatter uses
two-space indentation and preserves component entry order, mapping order,
comments, block scalar styles, and the meaning of the parsed document. It does
not alphabetize components, insert defaults, or add required fields.

The server reparses formatted output before returning the edit. It rejects a
format operation if the output would change the YAML data model. Formatting is
idempotent, so enabling VS Code's format-on-save does not create repeated diffs.

## Runtime feedback

In development or debug mode, the language server can read the runtime's current
diagnostic snapshot from:

```text
/__hyperbricks/render-diagnostics?view=current
```

Runtime Problems entries use source `hyperbricks-runtime` and can include the
route, request ID, component path, render phase, and a related template, script,
or resource position. The current snapshot replaces the previous one. A
successful retry clears the earlier error for that request context, and a
configuration reload or restart clears diagnostics from the previous runtime
generation.

Runtime feedback covers only routes and request variants that have actually
run. The status item shows checked and total routes; unchecked routes are status
information, not source warnings. Its issue count includes every current
runtime severity, not only errors. When the runtime has evicted request contexts
from its retained snapshot, the status tooltip and HyperBricks output identify
that coverage gap. An empty runtime Problems list therefore does not prove that
every route has rendered successfully.

The language server attaches the runtime's `__config` marker to the real
selected package profile only when that file exists inside the selected module.
Issues with blank, external, or otherwise unsafe source locations do not create
fabricated Problems anchors. Their bounded, credential-redacted summaries are
shown in the status tooltip and HyperBricks output instead.

The JSON diagnostic connection does not require the visual dashboard to be
enabled, but the browser-based `/__hyperbricks/errors` view does. The language
server advertises that Errors view to the extension only when
`development.dashboard.enabled` is true.

Runtime diagnostics describe saved source loaded by the running process. Save a
dirty document and allow the watcher to reload it before using a runtime result
to judge the edited content. The extension also identifies already-dirty
buffers when a language server restarts, so saved-source runtime diagnostics
remain suppressed across settings changes and workspace handoffs.

### Authentication and remote runtimes

The diagnostics endpoint keeps its existing developer-interface Basic Auth and
is unavailable in live mode. Automatic credential use is limited to literal
loopback addresses and `localhost`. HyperBricks does not automatically forward
resolved local credentials to a remote hostname.

Protocol version 1 does not accept remote credentials from the extension. An
explicit non-loopback URL is therefore contacted without the selected module's
local credentials and will normally remain disconnected when the developer
interface is protected. Use the local loopback runtime for authenticated editor
feedback. Remote credential support requires a future explicit authorization
flow and secret storage; credentials must never be put in workspace settings,
process arguments, logs, diagnostics, or protocol traces.

Runtime Problem messages preserve the actionable failure, route path, request,
component, and phase while removing control characters, URL credentials and
query/fragment data, and obvious authorization, password, token, and secret
assignments. Structured route context likewise omits query and fragment data.

## Troubleshooting

### The language server does not start

Run the configured executable directly and confirm that the command exists:

```bash
hyperbricks language-server --help
```

Then check **HyperBricks: Show Output** for executable, module, package-profile,
runtime-connection, or protocol-version errors. The status tooltip repeats the
current runtime connection error. A development checkout can be newer than
another `hyperbricks` executable on `PATH`; **Run Module Check** provides the
complete saved-project configuration report.

### Static feedback uses the wrong module

Leave `hyperbricks.module` empty to select the nearest ancestor of the active
HyperBricks file that contains `hyperbricks.config`. If no safe owning package
is found, the status bar asks you to select a module and the extension does not
send that source to a fallback package. Set `hyperbricks.module` explicitly when
the source and package profile do not share that directory ancestry, then restart
the language server. Imports and local resource completion stay confined to the
selected module.

### A HyperBricks file has no highlighting

Confirm the editor's language mode is **HyperBricks YAML**. The extension uses
its own `hyperbricks-yaml` language identifier and `source.hyperbricks-yaml`
grammar scope so an older extension cannot take ownership through the same
identifier. After replacing a local VSIX build, run **Developer: Reload Window**
so VS Code reloads the manifest and grammar.

### Runtime feedback is disconnected

Confirm that the module is running in development or debug mode, that the
developer-interface credentials resolve, and that the configured URL and port
match the running process. A `401` means the credentials were rejected; `503`
means the developer interface is locked because complete credentials are not
available. Live mode deliberately returns `404` for the diagnostic endpoint.

See [Troubleshooting](TROUBLESHOOTING.md#find-the-reported-error) for the runtime
diagnostic lifecycle and [HyperBricks CLI](HYPERBRICKS_CLI.md#render-diagnostics)
for the underlying JSON endpoint.
