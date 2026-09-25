# HyperBricks product UI

The Dashboard, built-in Spaces editor, and local/remote deployment dashboards
share this package. The approved reference remains in
`modules/hyperbricks-ui-lab`. Its fixtures are not used by the runtime.

## Ownership

- `ui.css`: Tailwind entry point, explicit production template sources, DaisyUI
  **lofi** (light) and **night** (dark). Do not scan application modules.
- `layout.css`: common header, wordmark sizing, spacing, typography, section
  layout, and accessibility states. No replacement button or input theme.
- `web/theme.js`: one `hb-theme` preference, applied before paint; accepts the
  previous `light`/`dark` values and synchronizes same-origin browser tabs.
- `web/brandmark.svg`: original supplied mark, embedded independently of the
  ignored design directory. Wordmark: **HyperBricks**, IBM Plex Mono Bold 700.
- `web/favicon.svg`: same geometry, with system light/dark contrast.
- `web/lucide.js`: the existing vendored icon library, now shared by all tools.
- `../../assets/src/deploy_yaml_editor.mjs`: deployment-only CodeMirror source;
  `../../assets/deploy_yaml_editor.js` is its generated, committed, lazy-loaded
  browser bundle. The shared deployment asset handler serves a negotiated gzip
  representation. The editor keeps the textarea as a failure fallback and
  never parses or serializes saved YAML.

Page-specific layout belongs to `assets/dashboard.css` or
`pkg/spaces/web/style.css`. The sandboxed Markdown preview imports the shared
stylesheet through `document.css`; it does not execute theme JavaScript.
Application pages and the contextual editor's isolated shadow root are not
given a global reset or product stylesheet.

## Components

Use explicit DaisyUI classes in templates and render functions:

| Purpose | Component |
| --- | --- |
| Command | `btn btn-sm`, with `btn-primary` for the principal action |
| Icon tool | `btn btn-sm btn-square btn-ghost`, label and title, Lucide icon |
| Fields | `input`, `select`, `textarea` |
| Views | `tabs tabs-border` with `tab` and `aria-selected` |
| Modes | `join` with `join-item` and `aria-pressed` |
| Status | Filled `badge-info`, `badge-success`, `badge-warning`, `badge-error` |
| Feedback | Filled semantic `alert`; errors retain readable details |
| Dialog | Native `dialog.modal` with `modal-box`; preserve focus and Escape |

Navigation rows and image tiles are not command buttons. They need intrinsic
height and wrapping. Do not infer component classes with a MutationObserver.
Avoid generic layout names that collide with DaisyUI (`hero`, `status`, `menu`).
Native editor IDs, API contracts, permissions, and draft recovery remain owned
by Spaces. Deployment owns HMAC, build selection, lifecycle, and plugin actions.

## Build And Verify

From the repository root:

```sh
npm ci
npm run ui:build:deploy-editor
npm run test:ui
go test ./...
```

Commit `web/hyperbricks.css` with its source changes and commit the generated
deployment editor bundle with its source changes. Go embeds these assets, the
theme script, mark, favicon, and icons; a running binary requires neither Node
nor npm. Google Fonts is optional at runtime; system fonts are the fallback.
The generated editor bundle retains the complete MIT notices for its bundled
dependencies.

When changing layout, verify the real screens in both themes at desktop and
320/390px widths. Check long Space titles, save bars, dialogs, inactive views,
deployment menus, and readable feedback. Use temporary source/deploy roots for
write tests, never a user's active module or production deployment.

The repository's dedicated YAML tests bind port 8090. Stop a verification
runtime on that port before running the entire Go suite.

With the default global rate limit enabled, a cold editor load can exhaust the
asset/API burst allowance. Spaces reports the limit explicitly; use Reload
source files after it replenishes. This UI migration does not change rate-limit
policy or retry source mutations automatically.
