<!-- Generated from docs/VSCODE.md. Do not edit directly. -->

# Visual Studio Code

HyperBricks includes a language server and a matching Visual Studio Code
extension for `*.hyperbricks.yaml` source. The runtime executable remains the
source of truth for component types, fields, validation, formatting, and runtime
diagnostics; the extension does not carry a separately maintained DSL schema.

Use an extension build and `hyperbricks` executable from the same HyperBricks
revision. The editor protocol is versioned separately from the public Language
Server Protocol and rejects incompatible clients with an explicit message.

## Features

The extension provides:

- HyperBricks-aware YAML highlighting;
- static syntax and native-component diagnostics in the Problems panel;
- completion for component types, fields, inheritance targets, and local
  template/resource paths;
- component and field hover information from the Go schema registry;
- whole-document formatting that preserves ordered component entries, comments,
  and scalar styles;
- authenticated render diagnostics from a running local development or debug
  runtime.

Component source and package configuration are different contracts even when a
profile also ends in `.hyperbricks.yaml`. Syntax and native-schema feedback can
still operate on an isolated component buffer, while import graphs and
module-local path completion are confined to the selected module's configured
HyperBricks source directory. The selected package profile is validated as
package configuration rather than as a component tree.

## Build the extension from this checkout

The extension source lives under `editors/vscode/`. From that directory, install
its pinned dependencies and build the extension:

```bash
npm ci
npm run compile
```

Open the directory in VS Code and run its extension-development launch target,
or package a local VSIX with:

```bash
npm run package
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
initialization. The server keeps unsaved documents in an in-memory overlay, so
fast diagnostics and completion reflect the editor buffer instead of lagging one
save behind.

## Settings

| Setting | Default | Purpose |
| --- | --- | --- |
| `hyperbricks.executable` | `hyperbricks` | Executable that supplies `language-server` and `doctor`. |
| `hyperbricks.module` | `default` | Module name or directory, using the normal HyperBricks module-selection contract. |
| `hyperbricks.config` | `package.hyperbricks.yaml` | Package profile relative to the selected module. |
| `hyperbricks.runtimeDiagnostics` | `auto` | `auto`, `on`, or `off` for runtime feedback. |
| `hyperbricks.runtimeUrl` | empty | Explicit HTTP(S) runtime base URL; the extension sends only its origin, while empty allows local discovery. |
| `hyperbricks.trace.server` | `off` | `off`, `messages`, or `verbose` protocol tracing. |

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

- **HyperBricks: Restart Language Server**;
- **HyperBricks: Connect Runtime Diagnostics**;
- **HyperBricks: Disconnect Runtime Diagnostics**;
- **HyperBricks: Run Module Check**;
- **HyperBricks: Open Runtime Errors** (when the development dashboard is
  enabled);
- **HyperBricks: Show Output**.

**Run Module Check** uses the configured executable, module, and package profile
with `hyperbricks doctor`. Doctor remains a saved-project health check; live
typing diagnostics come directly from the language server.

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

Set `hyperbricks.module` to the module name or directory and
`hyperbricks.config` to its active package profile, then restart the language
server. Imports and local resource completion are resolved within that selected
module.

### Runtime feedback is disconnected

Confirm that the module is running in development or debug mode, that the
developer-interface credentials resolve, and that the configured URL and port
match the running process. A `401` means the credentials were rejected; `503`
means the developer interface is locked because complete credentials are not
available. Live mode deliberately returns `404` for the diagnostic endpoint.

See [Troubleshooting](TROUBLESHOOTING.md#find-the-reported-error) for the runtime
diagnostic lifecycle and [HyperBricks CLI](HYPERBRICKS_CLI.md#render-diagnostics)
for the underlying JSON endpoint.
