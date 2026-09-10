# HyperBricks modules

This directory contains runnable examples, learning applications, browser-library
integrations, and fixtures used to verify HyperBricks itself. Start a module from
the repository root with:

```sh
hyperbricks start -m <module-name>
```

Some modules need a plugin build, an external service, or another preparation
step. Follow the module's own README when it has one.

## Categories and support markers

| Category | Purpose |
| --- | --- |
| Learning application | A guided application intended for learning HyperBricks. |
| Pattern reference | Small, reusable examples of recommended application structures. |
| Feature demo | A focused demonstration of a HyperBricks runtime or build feature. |
| Browser integration | A HyperBricks application integrated with a browser-side navigation or hypermedia library. |
| Test fixture | Input owned by an automated repository test or test script. |
| Verification fixture | A module kept for targeted or manual compatibility checks. |
| Benchmark fixture | A stable workload used by performance tests. |

**Tests** in the Required by column means test code or a dedicated test script
references that module by name or path. **Benchmarks** means benchmark code
depends on it. Do not remove or rename those modules without updating their
consumers. The general package-configuration test also discovers every
`package.hyperbricks.yaml` under this directory; that broad validation is not
listed as a direct dependency for every row.

## Module index

| Module | Category | Required by | Description |
| --- | --- | --- | --- |
| [`esbuild-demo`](esbuild-demo/) | Feature demo | — | Native esbuild example with TypeScript and CSS imports, copied assets, source maps, fingerprinted output, caching, and development watching. |
| [`goja-render-demo`](goja-render-demo/) | Feature demo | **Tests** — [`goja_yaml_test.go`](../cmd/hyperbricks/goja_yaml_test.go) | Server-side calculations with `goja_render`, including query validation, resource scripts, request isolation, and development reload behavior. |
| [`headers-test`](headers-test/) | Test fixture | **Tests** — [`test_headers_module.sh`](../scripts/test_headers_module.sh) | Development-mode half of the header regression fixture, covering configured response headers, cookies, routes, and generated output. |
| [`headers-test-live`](headers-test-live/) | Test fixture | **Tests** — [`test_headers_module.sh`](../scripts/test_headers_module.sh) | Live-mode counterpart to `headers-test`, used to verify the same response metadata through the production rendering and cache path. |
| [`hyperbricks-patterns-yaml`](hyperbricks-patterns-yaml/) | Pattern reference | **Tests** — Go integration tests and the optional plugin smoke suite | Runnable patterns for page and fragment composition, menus, section rails, guards, API actions, plugins, HTMX, Unpoly, and rendered Markdown documentation. |
| [`markers-test`](markers-test/) | Test fixture | **Tests** — [`run_marker_tests.sh`](../scripts/run_marker_tests.sh) | Resolver and directory-marker fixture for module, repository, resource, template, static, and HyperBricks paths. |
| [`navigation-demo-swup`](navigation-demo-swup/) | Browser integration | — | Text-only neighbourhood guide whose complete server-rendered pages use Swup for animated navigation and browser-history transitions. |
| [`project-lifecycle-test`](project-lifecycle-test/) | Test fixture | **Tests** — [`test_project_lifecycle.py`](../scripts/test_project_lifecycle.py), run with `--with-plugins` for the complete profile set | End-to-end project lifecycle fixture covering development and production rendering, fragments, assets, Goja, APIs, guards, native plugins, static export, and runtime archives. |
| [`sampleapis-coffee-static`](sampleapis-coffee-static/) | Feature demo | — | Static snapshot example that fetches the public SampleAPIs coffee endpoint through nested `api_render` and exports the rendered result. |
| [`self-closing-tag`](self-closing-tag/) | Verification fixture | — | Small image-rendering fixture used for manually checking generated image markup and the `self_closing_tags` server option. |
| [`ssr-proof-hyperbricks`](ssr-proof-hyperbricks/) | Benchmark fixture | **Tests + benchmarks** — render-plan tests, Go rendering benchmarks, and [`benchmarks/ssr-proof`](../benchmarks/ssr-proof/) | Minimal nested SSR workload with request-specific data, health endpoints, cached and raw server profiles, and stable output for throughput and allocation measurements. |
| [`static-paths-demo`](static-paths-demo/) | Test fixture | **Tests** — [`static_paths_demo_test.go`](../cmd/hyperbricks/static_paths_demo_test.go) | Demonstrates module-relative and repository-root static directories while proving that public asset URLs remain rooted at `/static/`. |
| [`streaming-demo`](streaming-demo/) | Feature demo | — | Native Go plugin example that streams several HTML progress updates over one response while HTMX swaps the target as chunks arrive. |
| [`test-demo-001`](test-demo-001/) | Verification fixture | — | Materialized Quickstart and `hyperbricks init` example for manually verifying the generated scaffold, fragments, templates, assets, and HTMX 4 integration. |
| [`todo-demo-htmx`](todo-demo-htmx/) | Browser integration | — | The Little List todo application using HTMX 4 fragment updates and browser `localStorage`. |
| [`todo-demo-swup`](todo-demo-swup/) | Browser integration | — | The same Little List application adapted to Swup, making its navigation and update model directly comparable with the other todo demos. |
| [`todo-demo-turbo`](todo-demo-turbo/) | Browser integration | — | The same Little List application adapted to Hotwire Turbo. |
| [`todo-demo-unpoly`](todo-demo-unpoly/) | Browser integration | — | The same Little List application adapted to Unpoly. |
| [`unpoly-guard-demo`](unpoly-guard-demo/) | Browser integration | — | Protected full-page and fragment routes with an application-owned authentication plugin and Unpoly-enhanced navigation. |
| [`wasm-plugin-test`](wasm-plugin-test/) | Verification fixture | — | Manual WebAssembly plugin compatibility fixture that renders Markdown and a card through two WASM components. |
| [`yaml-invalid-source`](yaml-invalid-source/) | Test fixture | **Tests** — [`initialize_processing_hyperbricks_test.go`](../cmd/hyperbricks/initialize_processing_hyperbricks_test.go) | Deliberately malformed YAML beside valid routes, used to verify source diagnostics, partial initialization, and development error reporting. |

## Maintenance

Add new modules to this index in alphabetical order. Mark a module as required
only when a test or benchmark names its path directly; include a link to that
consumer where practical. Keep generated files, dependency directories, compiled
plugins, and local runtime output out of this inventory.
