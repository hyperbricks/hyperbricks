# Changelog

## 2026-09-05 updates

### Goja render component

- Add built-in `goja_render` for running trusted project JavaScript on the server
  and rendering its result through Go HTML templates.
- Pass configured values and allowed query parameters to `main(input)`; expose
  the returned object as `.Data` in the template.
- Compile scripts during module loading and execute each request in a fresh
  runtime, with timeouts and request cancellation. Disable response caching for
  routes containing `goja_render`.
- Add [component documentation](docs/GOJA_RENDER.md) and a runnable
  `goja-render-demo` module with a printable materials worksheet.

### CLI module paths

- Start modules by name or directory path: `hyperbricks start -m demo`,
  `-m ./modules/demo`, `-m ../site/demo`, or `-m /srv/demo`.
- Resolve module paths and package configuration without duplicate directory
  prefixes. Keep `--config` relative to the selected module.
- Add module-name completion and report selection/configuration failures with
  exit status `1`.
- Update CLI help, documentation, and skill examples.

### Init

- Create missing scaffold directories and files while preserving existing files,
  including `package.hyperbricks.yaml`.
- Check scaffold paths for conflicts before creating files or directories.
- Reject empty and path-like module names; return exit status `1` on failure.
- Document `hyperbricks init -m demo` and the default `hyperbricks init` usage.

### Esbuild lowercase hashes

- Generate lowercase hashes in fingerprinted JavaScript and CSS entry filenames.
- Keep source-map filenames and references aligned with the entry filenames.
- Invalidate older build caches and correct filename casing on case-insensitive
  filesystems.

### Template query parameters

- Fix `.Params` access when template `values` is omitted or null, removing the
  need for `values: {}`.
- Apply the fix to both legacy rendering and compiled plans, preserving query
  allowlists and request isolation.
- Add regression tests and a YAML usage example.

### Skill and static-directory guidance

- Extend the CLI skill with concise examples for `goja_render`, native esbuild,
  lowercase fingerprinted asset URLs, and template `.Params` without `values`.
- Link the Goja and esbuild documentation and replace the obsolete esbuild
  plugin-install example with a generic plugin example.
- Clarify that direct `start` keeps the working directory unchanged and resolves
  `--config` inside the selected module. Synchronize the installed skill copy.
- Add side notes to the skill, CLI docs, and YAML docs: `/static/somefile.ext`
  serves `somefile.ext` from `hyperbricks.directories.static`, regardless of the
  configured directory name or `base`. A custom path does not require an
  additional directory named `static`.
- Distinguish filesystem paths resolved with `base: static` from browser URLs.
  No runtime path-resolution or directory-validation behavior was changed.
- Add `TestStaticPathsDemo` covering module/root bases, changed working
  directories, root/nested pages, asset responses, and incorrect-URL 404s.

The committed `modules/static-paths-demo` fixture uses two alternative package
configurations and distinct marker files to demonstrate the same `/static/`
URL serving different configured directories.

### Verification

The earlier runtime changes passed `go test ./...` before their commits.
Compiled CLI checks confirmed module-path startup, route and static asset
serving, preservation of existing init files, and failure exit statuses.

For the skill/static-directory follow-up:

- `TestStaticPathsDemo` and related route/`ServeContent` tests passed locally.
  The full suite was not rerun for this follow-up.
- Live CLI checks returned HTTP 200 for `/static/thefile.txt` with different
  marker contents under `base: module` and `base: root`. Both root and nested
  pages used the same asset URLs; disk-path URLs and `/nested/static/thefile.txt`
  returned HTTP 404.
- Changing the CLI working directory left module-based assets unchanged and
  made root-based assets resolve from the new working directory.
- An isolated run with `root-assets` configured and no directory named `static`
  started successfully, served `/static/thefile.txt` with HTTP 200, and did not
  create a `static` directory.
- Skill frontmatter and all 19 YAML examples parsed successfully; diff checks
  passed. The Python skill validator could not run because PyYAML was missing.

## 2026-09-09 updates

### Image rendering and generated assets

- Escape image attribute values, including extra attributes passed through the
  shared renderer. Preserve quotes and ampersands as text instead of allowing
  them to create additional HTML attributes.
- Generate root-relative `/static/images/` URLs that work from nested routes.
- Include source content and processing settings in generated filenames so
  different images, dimensions, and JPEG quality settings cannot overwrite
  one another. Handle long source filenames and publish completed files
  atomically for concurrent renders.
- Report invalid gallery files with their filenames and reject the gallery
  output instead of silently omitting failed images. Skip subdirectories and
  unsupported extensions; only add indexed ids when a base id is configured.
- Render an empty `alt` attribute for decorative images and support both
  `lazy` and `eager` loading. Informative images still need meaningful alt text.
- Correct image and gallery examples, regenerate the component reference, and
  add the [image usage guide](docs/IMAGES.md). Document local JPEG/PNG/GIF
  inputs, integer pixel dimensions, responsive CSS, and encoding behavior.

Generated image filenames have changed. Regenerate and deploy HTML together
with its static assets; older generated files remain available for previously
rendered pages.

### Static export configuration

- Reject malformed `hyperbricks.static` and `static.crawl` blocks with
  actionable errors before snapshotting routes.
- Add a complete [package configuration example](docs/HYPERBRICKS_CLI.md#package-configuration)
  covering routes, query variants, repeated query values, headers, Host, and
  output paths. Link it from the routing documentation.
- Clarify that configured targets supplement automatic discovery rather than
  restrict it. Explain selected-page exports, existing crawl aliases, output
  conflicts, and the difference between rendering targets and ZIP exclusions.
- Document standalone Node and Python serving, clean URLs, and separate output
  URLs for query variants.

### Mixed live-cache policies

- Add runtime regression coverage for interleaved cached and dynamic routes,
  expiry, query/authentication/cookie/body/method/header/Host variants, guard
  revocation, API failure recovery, and fresh stream producers.
- Expand the [cache operating guide](docs/LIVE_MODE_HTTP.md#mixing-cached-and-dynamic-routes)
  with route-policy examples and content-update guidance. `nocache: true`
  controls HyperBricks' internal cache; HTTP `Cache-Control: no-store` alone
  does not bypass it.
- Document the current process-wide duration, per-process storage, invalid
  duration fallback, and partial-render diagnostic behavior. Cache semantics
  are unchanged. Size limits, periodic eviction, and a public per-route
  invalidation API remain separate design work.

### Verification of these updates

- Full `pkg/component`, `pkg/shared`, `cmd/hyperbricks`, and `test/docs` suites
  passed. Targeted image, static-export, and mixed-cache race checks passed.
- The static configuration test loads the actual documented YAML, snapshots
  default and query/Host/header variants, exports a ZIP, and serves the extracted
  files after deleting the source and render directories.
- An actual CLI export in a temporary workspace produced a nested page and its
  processed image. After removing the source module, Python served both files
  from the extracted ZIP with HTTP 200; the image decoded at the expected size
  and its escaped alternative text round-tripped correctly.
- A corrupt gallery image caused the CLI to exit with status 1, identify the
  failed file in render diagnostics, and produce no ZIP.
- Independent reviews and final diff checks completed before the three commits.

## 2026-09-10 updates

### Go execution parallelism

- Replace the hard-coded four-slot `GOMAXPROCS` policy with validated
  `hyperbricks.server.gomaxprocs` package configuration.
- Use Go's CPU- and container-aware automatic policy by default. Accept fixed
  integer values from one through the host's logical CPU count and reject invalid
  values during startup.
- Keep automatic runtime CPU-limit updates enabled in `auto` mode and report the
  effective setting in the startup log.
- Document the setting and add parsing, validation, startup, and log coverage.

### API render cache ownership

- Define the cache contract for `api_render` and `api_fragment_render` in the
  component metadata, generated reference, API guide, project patterns, schema,
  fixtures, and shipped HyperBricks skill.
- Clarify that neither component caches upstream API responses. A nested
  `api_render` makes a fresh upstream request whenever it executes, while its
  parent route owns rendered-output caching. A parent cache hit skips the nested
  render and API request.
- Clarify that `api_render` has no `route` or `nocache` field. Fresh data on every
  route request requires `nocache: true` on the containing `hypermedia` or
  `fragment` route.
- Clarify that route-owning `api_fragment_render` always bypasses rendered-output
  caching and therefore calls its upstream on every invocation.
- Add regression tests that count upstream calls for cached parents, uncached
  parents, and API fragment routes, plus schema assertions that keep unsupported
  cache fields out of `api_render`.

### Benchmark and documentation maintenance

- Move the beautification measurement benchmark into `cmd/hyperbricks`, where
  its shared SSR benchmark fixture is defined, so `go vet ./...` and clean builds
  no longer encounter an undefined helper in a separate package.
- Preserve the Windows plugin-platform note and recommended project-patterns
  link in the README generator source, then regenerate the root README and
  component reference.

### Repository documentation and fixtures

- Add a categorized module index that records which modules are examples,
  verification fixtures, test fixtures, or benchmark fixtures.
- Refactor the former basics module into the `project-lifecycle-test` fixture
  and document how a public HyperBricks skill resolves its versioned sources.
- Document canonical HTMX page and fragment URLs, browser history behavior, and
  static-export compatibility.
- Add a generator for standalone Markdown documentation and skill handbooks,
  sourced from an exact committed Git snapshot.
- Add a Git-archive ZIP helper for sharing the current committed repository
  state without local or ignored output.

### Public-repository cleanup

- Keep generated documentation, benchmark workspaces, private article drafts,
  dependency trees, distribution folders, and machine-local test results out of
  version control.
- Mark the root README as generated and verify it against its committed source
  template in the documentation tests.
- Remove reproducible browser bundles and image derivatives from example
  modules, and correct public documentation links and HyperBricks website URLs.
- Align stale release, install, Docker, and plugin references with
  `v1.2.3-beta`.
- Make the reserved `plugin update` command fail with an explicit nonzero
  `not implemented` error instead of printing a simulated successful update.
- Expand the repository plugin builder to cover every plugin-backed demo and
  fixture, document the source-matched build workflow, and remove the obsolete
  Esbuild plugin from the patterns module in favor of the native component.
- Harden the dedicated PostgREST and pgAdmin test stack with generated local
  credentials, loopback-only ports, pinned image versions, safer privileged
  functions, and row-level security that rejects ownerless task inserts.
- Preserve concrete streaming request-body read errors when cancellation races
  with the read, retaining both causes while returning a stable timeout response.
- Verify the downloaded Tailwind executable against the release checksum and
  include the complete jsontr.ee license beside the copied frontend files.
- Remove the stale root Docker guide and make the committed-source ZIP helper
  derive its filename from the release version and committed revision.

### Verification of these updates

- `./tests.sh --with-docs --with-plugins` passed, including source-matched plugin
  builds and smoke checks, `go vet`, all Go package tests, Docker-backed API
  rendering tests, generated documentation checks, template tests, marker
  tests, and HTTP header, cookie, and cache suites.
- A follow-up documentation regeneration and `go test ./test/docs` passed after
  the README generator source was corrected.
- The dedicated API suite passed against the real Docker stack, including the
  negative ownerless-task authorization case, and the source-export helper
  produced a valid archive without ignored local output.
- A 72-sample local SSR matrix exercised one, four, and eight Go execution slots
  across 1,211,692 validated responses with no status or content failures. These
  measurements are retained as local benchmark evidence rather than a portable
  cross-environment performance guarantee.

## 2026-09-11 updates

### API query and body mapping

- Clarify that `querykeys` filters browser parameters appended to the upstream
  URL, while configured `$key` body placeholders use separate parsed input.
  Document append-only query collisions, static `queryparams`, form/JSON field
  precedence, missing fields, and the current bodyless-request difference.
- Correct JSON string placeholder escaping in both API components so values
  ending in a quote retain their original value and produce valid JSON string
  content. Repeated and structured values retain their existing display format.
- Add HTTP request/output regression coverage for both components and synchronize
  the component field descriptions, generated reference/schema, and skill guide.

### Explicit API credentials and validated response cookies

- Add string `forwardtoken` to `api_render` and `api_fragment_render`. Omission or
  an empty string disables browser-token forwarding. A configured value selects
  one exact incoming cookie; duplicate names and malformed values fail before
  the upstream request. Browser Authorization is never an implicit source.
- Replace authentication assignment-order precedence with one explicit source:
  named cookie, configured Authorization, signed JWT, or complete Basic Auth.
  Reject conflicting sources, incomplete credentials, and endpoint userinfo.
- Require HTTPS for credential sources, potentially sensitive headers, and
  request bodies, with literal loopback HTTP allowed in development/debug mode.
  Restrict all API redirects to the initial origin, remove the API cookie jar,
  and propagate request cancellation.
- Replace API request/response dumps with metadata-only diagnostics and sanitize
  transport errors so payloads, credential values, and sensitive URL details do
  not enter API debug output.
- Stop both API renderers before the upstream call when request-body preparation
  fails, and keep the underlying read error out of rendered diagnostics.
- Add structured `setcookies` entries and a constrained legacy-string path.
  Validate names, attributes, dynamic values, domains, and cookie prefixes; keep
  token bytes separate from cookie syntax and HTML escaping. Reject missing,
  empty, null, or non-string dynamic values and reserved Data/Status overrides.
- Emit API fragment cookies only after successful upstream processing, fragment
  rendering, and validation of the entire cookie group. Preserve explicit logout
  after bodyless 204 responses and reject malformed upstream JSON.
  Stage cookies until the HTTP response commit so later render/source errors or
  handled plugin responses cannot inherit them accidentally.
- Preserve raw types only for API security fields and validate them in the
  components before weak decoding. Keep other YAML scalar behavior unchanged.
- Add the runnable `api-security-test` module, mock upstream, complete research
  article, and end-to-end regression tests. Migrate authenticated lifecycle
  fixtures, regenerate the reference/schema, and update API documentation and
  skills with the new contract and migration requirements.

**Migration:** add `forwardtoken: token` only to API components approved to
receive that existing browser carrier. Public APIs should leave it omitted.
Remove competing auth settings, use HTTPS for deployed credential-bearing
endpoints, and migrate arbitrary cookie-header templates to structured entries.

Verification: `./tests.sh --with-docs` passed after the final changes, including
Docker-backed API fixtures and generated documentation. Targeted API HTTP tests
and the changed shared/component/composite/parser/render packages also passed
with `-race`. The project lifecycle fixture passed its API, guard, runtime-archive,
and static-export profiles without plugins.
