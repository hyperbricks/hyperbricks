# Native esbuild

Use the built-in `esbuild` component to bundle, transform, and minify JavaScript, TypeScript, and CSS. HyperBricks includes the esbuild Go library, so you do not need a separate esbuild installation.

You can also generate source maps, add fingerprints to output filenames, and cache builds.

## JavaScript example

```yaml
scripts:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/bundle.min.main.js}
  - minify: true
  - minify_identifiers: true
  - sourcemap: true
  - cache: true
  - enclose: '<script src="|" defer></script>'

page:
  - type: hypermedia
  - route: index
  - head:
      - type: head
      - application_script:
          - inherit: scripts
```

The child name `application_script` is arbitrary. Ordinary YAML ordering and inheritance determine where its result appears. This produces:

```html
<script src="/static/js/bundle.min.main.js" defer></script>
```

Use `path` to supply a filename. The `file` resolver reads the file's contents. The `resources` and `static` bases use the module's configured directories, so do not repeat `resources/` or `static/` inside the inner `path`.

Output must stay inside the configured static directory, including through symlinks. HyperBricks derives the public URL from the `/static/` mount, regardless of the directory name.

Local imports resolve relative to their importing file. Bare package imports require installed dependencies resolvable by esbuild; this component does not install npm packages. TypeScript is transpiled, not type-checked.

## CSS example

```yaml
styles:
  - type: esbuild
  - entry:
      path: {base: resources, path: css/site.css}
  - outfile:
      path: {base: static, path: css/site.css}
  - minify: true
  - sourcemap: true
  - cache: true
  - target: [chrome110, firefox115, safari16]
  - loader:
      .png: file
      .woff2: file
  - external:
      - /static/vendor/*
  - enclose: '<link rel="stylesheet" href="|">'
```

Inherit `styles` into the head in the same way as `scripts`. A CSS entry does not need a JavaScript entry. HyperBricks bundles CSS `@import` files.

Configure a loader for local `url()` images and fonts. Unknown types cause a build error. The `file` loader writes assets with hashed filenames beside the bundle. The `dataurl` loader embeds their bytes in the stylesheet.

The application must provide external references. This API does not compile CSS Modules, Sass, or Tailwind.

| CSS need | Option |
| --- | --- |
| Bundle local stylesheet imports | `.css` entry, automatic |
| Compact whitespace and syntax | `minify: true` |
| Source-level debugging | `sourcemap: true` |
| Supported browser syntax transforms | `target: [chrome110, safari16]` (example policy) |
| Copy images/fonts | `loader: {'.png': file, '.woff2': file}` |
| Inline small images | `loader: {'.png': dataurl}` |
| Keep separately served references | `external: ['/static/vendor/*']` |
| Reuse unchanged builds | `cache: true` |

## Options

Fields belong directly on the component. HyperBricks uses its embedded esbuild engine; selecting a separate executable with `binary` is no longer supported.

| Field | Default | Meaning |
| --- | --- | --- |
| `entry` | Required | One source filename, normally an explicit resources path. |
| `outfile` | Required | A `.js` or `.css` filename within static. CSS inputs require a `.css` outfile. |
| `enclose` | `""` | Replace `\|` with the public URL. Empty returns the URL without markup. |
| `minify` | `false` | Minify whitespace and syntax. |
| `minify_identifiers` | `false` | Minify identifiers independently. YAML also accepts legacy `minifyident`. |
| `mangle` | `false` | Advanced JS property mangling using `.*`. Can break DOM/library property contracts; rejected for CSS entries. |
| `sourcemap` | `false` | Emit a linked `.map` file, including source text. Consider source exposure before publishing. |
| `debug` | `false` | Log effective options, build/cache events, and the embedded engine version. |
| `cache` | `false` | False builds on every component render; true reuses a valid build. Not page caching. |
| `fingerprint` | `false` | Add an esbuild content hash to entry output filenames. Keep the configured directory; generation retention is controlled by `cache_keep`. Independent of `cache`. |
| `cache_keep` | `0` | Number of previous successful output generations retained in addition to current. Must be nonnegative. Independent of build reuse. |
| `target` | `[]` | esbuild browser/language targets. Empty keeps engine defaults; not a universal compatibility guarantee. |
| `loader` | `{}` | File-extension to loader overrides. |
| `external` | `[]` | Unbundled import/URL patterns, with at most one `*` per pattern. |

Supported targets: `esnext`, `es5`, `es6`, `es2015` through `es2024`, and versioned `chrome`, `edge`, `firefox`, `safari`, `ios`, `node`, `opera`, or `ie` names. Only one language target and one target per engine are allowed. Validation accepts the target spelling; esbuild may still reject syntax it cannot transform to it.

Supported loader values: `file`, `dataurl`, `text`, `binary`, `base64`, `json`, `css`, `js`, `jsx`, `ts`, `tsx`. Keys include the leading dot. Omitted extensions keep esbuild's defaults. These options do not constitute an arbitrary esbuild CLI passthrough: multiple entries, code splitting, output formats, and CSS Modules class mappings are not exposed in this first integration. A JS import of CSS can emit a sibling CSS file; declare its link yourself or use a separate CSS component.

## Build and cache lifecycle

See [Asset caching](ASSET_CACHING.md) for practical development/live settings
and examples shared with image components.

1. Module loading resolves inheritance and paths, validates options, and prepares the component. It does not compile assets. Conflicting builds for the same outfile are reported before serving that configuration.
2. Rendering restores a validated persistent build when possible, or invokes the compiler, publishes the files, and returns the URL/markup. A cold first request without a valid build waits for compilation. Unused declarations do not build.
3. With `cache: true`, subsequent renders return the remembered output URL until invalidation. They do not read or hash asset files, inspect ownership history, or acquire a filesystem lock. With `cache: false`, each component render compiles again.

Cached components with the same effective build options and paths share a build, even across inherited pages. `enclose`, `debug`, and `cache` do not change the build identity. Concurrent successful cached misses compile once. Asset builds within a module are serialized, including uncached builds, to protect output publication. Shared compilation has a 30-second deadline independent of the initiating request.

The generated files remain on disk; the browser fetches them through a separate `/static/` request. The in-memory build cache remembers the successful output path and build metadata. A result becomes reusable only after output publication and ownership registration succeed.

Module reload invalidates in-memory reuse and forces compilation on next use. With development watching enabled, put every relevant source/config directory in `development.watch_dirs`; the defaults do not include resources. Changes outside those directories, or with watching disabled, require a restart or an available explicit module reload. The manual reload interface is not available in every runtime mode. A warm cache does not detect externally deleted or modified files by polling them during page requests. Restart still validates persisted input/output bytes, including same-size edits and missing auxiliary files, before reusing a build. Existing asset files alone are never sufficient evidence. HyperBricks also invalidates affected cached builds when its own ESBUILD publication or cleanup changes their inputs.

Page caching is independent. A whole-page cache hit does not render components, so it cannot trigger either rebuild policy. Development renders the page on every request. In live mode, use `nocache: true` on a page when every request must reach its components. Changing a source alone does not invalidate an already cached HTML response outside the development reload. Browser/CDN caching is also separate; use `fingerprint: true` for versioned asset URLs. Fingerprinting does not change HTTP cache headers or invalidate cached HTML.

## Versioned output

```yaml
scripts:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.ts}
  - outfile:
      path: {base: static, path: js/app.js}
  - fingerprint: true
  - cache: true
  - enclose: '<script src="|" defer></script>'
```

The configured `js/app.js` supplies the output directory and base name. The actual file is `static/js/app.<hash>.js`, and the component returns its `/static/` URL. Generated entry hashes are lowercase, for example `app.ixxrt565.js`. Authored directory names and filename stems retain their original casing. Linked source maps and a JS entry's CSS sibling use the same lowercase entry-hash policy. Do not put `[hash]` in `outfile`; esbuild owns the hash insertion. CSS entries work the same way: `css/site.css` becomes `css/site.<hash>.css`.

The compiler generates names and links for the entry, linked source maps, and file-loader images/fonts. A JavaScript entry importing CSS also produces a hashed CSS sibling; its URL is not automatically inserted into HTML. Prefer a separate CSS component when its link must be rendered into the head.

Identical builds with the same engine and paths keep the same names. Changed emitted content or referenced assets produces new names. A source edit that does not affect the generated result need not change its URL. Hashes are not portable build IDs across esbuild versions or source paths.

`cache: false` still compiles every render, even with fingerprinting. Conversely, `cache: true` can reuse builds with either fixed or versioned filenames.

All files are published before the component returns the URL. Built-in cleanup keeps the current successful generation by default. Set `cache_keep: 2` on a component to retain two previous successful generations as well. A generation includes its entry, linked source maps, CSS siblings, and file-loader assets. Unchanged builds and cache hits do not create duplicate generations. A failed compile preserves the last successful output.

Start, restart, reload, and static export reconcile recorded ownership. Removing a component makes its recorded generations eligible for cleanup. In-flight rendering, cached HTML responses, and pages in the current static export protect their referenced generations. No cleanup hook or plugin is needed. Browser/CDN caches and previously downloaded HTML are outside this local protection: choose a retention value appropriate to your deployment and rollback policy.

Cleanup only removes files recorded as owned by esbuild, within the static root, whose contents still match the recorded output. Handwritten files and unrecorded legacy fingerprints are left alone. Shared files remain while any retained generation needs them. Existing valid native manifests can supply ownership for migration; filenames alone never establish ownership.

## Persistent build cache

With `cache: true`, successful stable builds write a private JSON manifest under the operating system's user cache directory, in `hyperbricks/esbuild/`. This is typically `~/Library/Caches/hyperbricks/esbuild/` on macOS, or `$XDG_CACHE_HOME/hyperbricks/esbuild/` / `~/.cache/hyperbricks/esbuild/` on Linux.

The manifest records the effective configuration, compiler identity, resolved dependency fingerprints, actual entry filename, and fingerprints of every output. It is not stored under static and is not part of static exports. A cache directory configured inside static is rejected for persistence. Manifests contain local source paths, have private file permissions, and should not be published.

At first use after restart, HyperBricks validates the manifest and reads/hashes its recorded inputs and outputs. Missing or modified sources, changed options or engine, missing/corrupt assets, and invalid manifests trigger compilation. Output ownership and watcher exclusions are restored along with the build. A fresh static export starts new output ownership and does not adopt a previous export's manifests.

No sources means no validated runtime build reuse: this is not a source-free server mode. A deployed static export, however, needs only its exported assets. Moving a checkout or changing effective paths starts a separate cache identity.

Build manifests control reuse; a separate generation record records ownership even with `cache: false`. Ownership uses a fixed private location per OS account: `~/Library/Caches/hyperbricks/esbuild/` on macOS, `~/.cache/hyperbricks/esbuild/` on Linux, and the account's Windows Local AppData directory under `hyperbricks/esbuild/`. Unlike build manifests, this location does not follow cache-directory environment overrides, so separate processes find the same ownership evidence.

When the fixed record is absent, a valid record for this exact output root in the current legacy cache location is migrated under the publication lock. The original is removed only after successful persistence. Arbitrary old custom cache directories are not searched: start once with the former environment when upgrading from one. Export copying only reads records; it does not migrate source ownership. Deleting ownership records loses cleanup evidence that cannot always be recovered from valid manifests. Prefer letting HyperBricks maintain them.

Ownership storage must be writable and outside static. Invalid history, an unavailable directory, or a competing operation holding the same output-root lock fails explicitly. Runtime sessions and static exports hold an OS file lock until cleanup; standalone component builds lock their publication operation. Locks use the canonical output root, independently of cache and temporary-directory environment overrides. Lock files are not removed or forcefully taken over; the OS releases a lock when its process exits. Separate processes must not write the same output root concurrently. Build failures return component errors, never a stale success tag.

## Development watch

Include resources in the existing module watcher:

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    watch_dirs: [hyperbricks, templates, resources]
    reload: false
```

The debounced reload invalidates asset builds. The next page render builds from current sources. Generated outputs and temporary publication files are ignored by this watcher, even if static is underneath a watched directory. The `reload` setting currently has no implemented automatic browser-refresh effect; refresh the browser manually. This is lazy rebuilding, not an esbuild HMR server or an eager background compiler.

For the original development workflow, use `cache: false`: every component render builds, without waiting for cache invalidation. No HyperBricks binary rebuild is needed in either case.

## Build errors and output

If a build fails, HyperBricks reports the error and tries again on the next render. Files from the last successful build stay available and are not replaced by a broken build.

HyperBricks finishes writing generated files before returning their URL. Give each build its own `outfile`, keep generated output separate from source files, and make sure the static output directory is writable. Generation retention handles owned fingerprints automatically; remote deployment retention remains the responsibility of your publishing workflow.

Keep entry paths and build options in trusted application configuration rather than creating them from request data.

## Deployment and performance

Static rendering reserves its export destination before hooks or deletion, copies authored static files, and builds assets directly into `rendered/static/` while rendering routes. A development server can keep its own static output and writer sessions. Source paths retain their authored resolution; a build that consumes generated static output reads the export's version. Images also write into the export. Public URLs remain `/static/...`.

The initial copy shares a short filesystem lock with built-in ESBUILD, image, Spaces, and scaffold mutations. Known ownership records are checked before copying; temporary publication files are excluded. Symlinks are materialized so later ZIP creation does not read their original targets. Hooks, plugins, external symlink targets, and manual edits are not a project-wide snapshot transaction. Overlapping source/output roots and symlinks at output bindings that will be replaced are rejected before destructive preparation. Export reservations remain held through preview and finish hooks.

Deploy the contents of `rendered/`; the destination does not need HyperBricks, esbuild, or the module's `resources/` directory to serve that static site. Bundled images/fonts are emitted into static output; intentionally external references still need to be available at their configured URLs.

### Source maps in a static export

A generated map may contain paths such as:

```json
"sources": ["../../resources/css/tokens.css", "../../resources/css/site.css"]
```

These identify the original files for debugging. The generated map also embeds their original text in the corresponding `sourcesContent` entries. That text is available to source-map consumers without fetching the original resource files; see [esbuild's sources content documentation](https://esbuild.github.io/api/#sources-content). The relative paths do not need to exist in the deployed site when their content is embedded. They are not CSS `@import` or `url()` dependencies.

For a focused public export, publish the generated bundles, required images/fonts, and maps when desired. The full resources directory can stay on the server because it may also contain scripts or data that are not browser assets. `sourcemap: false` omits maps from a fresh build; when switching it off, remove previously generated maps before deployment because old generated files are not automatically cleaned. Dependencies supplying their own source maps without embedded originals may require fixing those upstream maps for complete source-level debugging.

### Runtime archives

Runtime archive `build` packages the module files so assets can be built when a route is first rendered. Include the source files and dependencies, and allow writes to the static directory. To ship generated output as well, render the relevant pages before packaging. HyperBricks currently uses this on-demand flow rather than a separate prebuilt-only mode.

### Build resources

Because esbuild is embedded, deployments do not need a separate esbuild installation. Asset builds use CPU and memory when sources change, while cached renders reuse completed builds. For consistent first-request performance in production, warm the relevant routes before accepting traffic. Actual resource use depends on the size and structure of the project.

## Migration from EsbuildPlugin@2.0.0 (deprecated)

- Replace `type: plugin` with `type: esbuild`; remove `plugin` and the `data` wrapper.
- Replace relative `entry`/`outfile` strings with the explicit path resolvers above.
- Prefer `minify_identifiers`. The documented `minifyident` alias is normalized before inheritance; specifying both in one declaration is an error.
- Native YAML booleans are recommended; legacy quoted `"true"`/`"false"` are accepted.
- Change `src="/|"` to `src="|"`: the returned URL already starts with `/static/`.
- `cache: true` builds on first use without a valid manifest; unlike the old entry-only cache, native reuse validates build options, dependencies and output.
- Remove `binary`; the native component always uses embedded esbuild. The old external plugin's `--minify` also enabled identifier minification; set `minify_identifiers` explicitly when migrating that behavior.

The original esbuild and Tailwind plugins are unchanged.

---

## Credits and license

HyperBricks' native `esbuild` component uses the esbuild Go API. Credit goes to Evan Wallace and the esbuild contributors for creating and maintaining the compiler that powers this integration.

- **Website:** [esbuild.github.io](https://esbuild.github.io/)
- **Source repository:** [github.com/evanw/esbuild](https://github.com/evanw/esbuild)
- **License:** [MIT License — Copyright (c) 2020 Evan Wallace](https://github.com/evanw/esbuild/blob/main/LICENSE.md)
