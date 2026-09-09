# Changelog — 2026-09-05

## Goja render component (`3a32149`)

- Add built-in `goja_render` for running trusted project JavaScript on the server
  and rendering its result through Go HTML templates.
- Pass configured values and allowed query parameters to `main(input)`; expose
  the returned object as `.Data` in the template.
- Compile scripts during module loading and execute each request in a fresh
  runtime, with timeouts and request cancellation. Disable response caching for
  routes containing `goja_render`.
- Add [component documentation](docs/GOJA_RENDER.md) and a runnable
  `goja-render-demo` module with a printable materials worksheet.

## CLI module paths (`549c5f7`)

- Start modules by name or directory path: `hyperbricks start -m demo`,
  `-m ./modules/demo`, `-m ../site/demo`, or `-m /srv/demo`.
- Resolve module paths and package configuration without duplicate directory
  prefixes. Keep `--config` relative to the selected module.
- Add module-name completion and report selection/configuration failures with
  exit status `1`.
- Update CLI help, documentation, and skill examples.

## Init (`51e1488`)

- Create missing scaffold directories and files while preserving existing files,
  including `package.hyperbricks.yaml`.
- Check scaffold paths for conflicts before creating files or directories.
- Reject empty and path-like module names; return exit status `1` on failure.
- Document `hyperbricks init -m demo` and the default `hyperbricks init` usage.

## Esbuild lowercase hashes (`9262098`)

- Generate lowercase hashes in fingerprinted JavaScript and CSS entry filenames.
- Keep source-map filenames and references aligned with the entry filenames.
- Invalidate older build caches and correct filename casing on case-insensitive
  filesystems.

## Template query parameters (`82a62f9`)

- Fix `.Params` access when template `values` is omitted or null, removing the
  need for `values: {}`.
- Apply the fix to both legacy rendering and compiled plans, preserving query
  allowlists and request isolation.
- Add regression tests and a YAML usage example.

## Skill and static-directory guidance (`1bacf56`)

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

The local `modules/static-paths-demo` example uses two alternative package
configurations and distinct marker files to demonstrate the same `/static/`
URL serving different configured directories. The module is ignored and is not
included in this commit.

## Verification

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

The committed regression test currently reads the ignored demo module, so its
fixtures must be included or made self-contained for clean-checkout test runs.

## 2026-09-09 updates

### Image rendering and generated assets (`26a281f`)

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

### Static export configuration (`65ba142`)

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

### Mixed live-cache policies (`d7d9ff7`)

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
