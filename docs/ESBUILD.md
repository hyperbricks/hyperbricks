# Native esbuild

HyperBricks' built-in `esbuild` component lets you bundle, transform, and minify JavaScript, TypeScript, and ordinary CSS using the esbuild Go library embedded in HyperBricks. Optional, it can also generate source maps, fingerprint output filenames, and reuse cached builds without requiring a separate esbuild installation.

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

Use **`path`**, not `file`: `path` supplies a filename, whereas `file` reads its contents. The resources/static bases honor the module's configured directories. Do not repeat `resources/` or `static/` inside the inner `path`. Output must stay inside the configured static directory, including when symlinks are involved. The public URL is always derived from the `/static/` mount, not the directory name.

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

Inherit `styles` into the head exactly like `scripts`. A CSS entry works on its own, without a JS entry. CSS `@import` files are bundled. Local `url()` images/fonts need an appropriate loader; unknown types fail instead of being silently externalized. The `file` loader emits hashed asset filenames alongside the bundle. The `dataurl` loader embeds bytes in the stylesheet. External references remain the application's responsibility. CSS Modules and Sass/Tailwind compilation are not part of this API.

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

All ten esbuild 2.0.0 plugin options are retained. `target`, `loader`, `external`, and `fingerprint` are additional native-component options. Fields belong directly on the component.

| Field | Default | Meaning |
| --- | --- | --- |
| `entry` | Required | One source filename, normally an explicit resources path. |
| `outfile` | Required | A `.js` or `.css` filename within static. CSS inputs require a `.css` outfile. |
| `binary` | `""` | Embedded engine by default; optional external executable name/path. |
| `enclose` | `""` | Replace `\|` with the public URL. Empty returns the URL without markup. |
| `minify` | `false` | Minify whitespace and syntax. |
| `minify_identifiers` | `false` | Minify identifiers independently. YAML also accepts legacy `minifyident`. |
| `mangle` | `false` | Advanced JS property mangling using `.*`. Can break DOM/library property contracts; rejected for CSS entries. |
| `sourcemap` | `false` | Emit a linked `.map` file, including source text. Consider source exposure before publishing. |
| `debug` | `false` | Log effective options, build/cache events, and external engine version when applicable. |
| `cache` | `false` | False builds on every component render; true reuses a valid build. Not page caching. |
| `fingerprint` | `false` | Add an esbuild content hash to entry output filenames. Keep the configured directory and retain older assets. Independent of `cache`. |
| `target` | `[]` | esbuild browser/language targets. Empty keeps engine defaults; not a universal compatibility guarantee. |
| `loader` | `{}` | File-extension to loader overrides. |
| `external` | `[]` | Unbundled import/URL patterns, with at most one `*` per pattern. |

Supported targets: `esnext`, `es5`, `es6`, `es2015` through `es2024`, and versioned `chrome`, `edge`, `firefox`, `safari`, `ios`, `node`, `opera`, or `ie` names. Only one language target and one target per engine are allowed. Validation accepts the target spelling; esbuild may still reject syntax it cannot transform to it.

Supported loader values: `file`, `dataurl`, `text`, `binary`, `base64`, `json`, `css`, `js`, `jsx`, `ts`, `tsx`. Keys include the leading dot. Omitted extensions keep esbuild's defaults. These options do not constitute an arbitrary esbuild CLI passthrough: multiple entries, code splitting, output formats, and CSS Modules class mappings are not exposed in this first integration. A JS import of CSS can emit a sibling CSS file; declare its link yourself or use a separate CSS component.

## Build and cache lifecycle

1. Module loading resolves inheritance and paths, validates options, and prepares the component. It does not compile assets. Conflicting builds for the same outfile are reported before serving that configuration.
2. Rendering restores a validated persistent build when possible, or invokes the compiler, publishes the files, and returns the URL/markup. A cold first request without a valid build waits for compilation. Unused declarations do not build.
3. With `cache: true`, subsequent renders reuse output while its dependencies and files remain valid. With `cache: false`, each component render compiles again.

Cached components with the same effective build options and paths share a build, even across inherited pages. `enclose`, `debug`, and `cache` do not change the build identity. Concurrent successful cached misses compile once. Asset builds within a module are serialized, including uncached builds, to protect output publication. Shared compilation has a 30-second deadline independent of the initiating request.

Cache validation currently reads and hashes inputs and outputs on each cached component render. It detects same-size edits, modified/missing auxiliary assets, and nearby `package.json`, `tsconfig.json`, and `jsconfig.json` changes. This avoids recompilation but is **not a zero-I/O request path**. esbuild's resolved input graph does not describe every possible future module-resolution change; reload the module after changes outside that graph, such as an extended config elsewhere or a newly introduced alternative import target. Explicit module reload clears reuse and forces compilation on next use. Restart can restore a validated disk manifest; existing asset files alone are never enough.

Page caching is independent. A whole-page cache hit does not render components, so it cannot trigger either rebuild policy. Use `nocache: true` on a development page when every request must reach its components. Changing a source alone does not invalidate an already cached HTML response outside the development reload. Browser/CDN caching is also separate; use `fingerprint: true` for versioned asset URLs. Fingerprinting does not change HTTP cache headers or invalidate cached HTML.

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

Identical builds with the same engine and paths keep the same names. Changed emitted content or referenced assets produces new names. A source edit that does not affect the generated result need not change its URL. External and embedded engines can choose different hashes with source maps: the external engine hashes its staged map before HyperBricks rebases the source labels. Both modes retain valid map links and embedded source text; hashes are not portable build IDs.

`cache: false` still compiles every render, even with fingerprinting. Conversely, `cache: true` can reuse builds with either fixed or versioned filenames.

All files are published before the component returns the URL. New versions leave previous versions available for older HTML. There is no automatic pruning or `cache_keep` option. Deploy cleanup must account for old pages, browser caches, and rollbacks; retaining old files locally does not guarantee your deployment tool retains them remotely.

## Persistent build cache

With `cache: true`, successful stable builds write a private JSON manifest under the operating system's user cache directory, in `hyperbricks/esbuild/`. This is typically `~/Library/Caches/hyperbricks/esbuild/` on macOS, or `$XDG_CACHE_HOME/hyperbricks/esbuild/` / `~/.cache/hyperbricks/esbuild/` on Linux.

The manifest records the effective configuration, compiler identity, resolved dependency fingerprints, actual entry filename, and fingerprints of every output. It is not stored under static and is not part of static exports. A cache directory configured inside static is rejected for persistence. Manifests contain local source paths, have private file permissions, and should not be published.

At first use after restart or a new `static` command, HyperBricks validates the manifest and reads/hashes its recorded inputs and outputs. Missing or modified sources, changed options or engine, missing/corrupt assets, and invalid manifests trigger compilation. The external executable is included in input validation. Output ownership and watcher exclusions are restored along with the build.

No sources means no validated runtime build reuse: this is not a source-free server mode. A deployed static export, however, needs only its exported assets. Moving a checkout or changing effective paths starts a separate cache identity.

Deleting manifests is safe; later renders rebuild. If the cache directory cannot be used, HyperBricks logs a warning and keeps working with in-memory caching. Build failures still return component errors, never a stale success tag. Builds are coordinated within a module renderer, not across separate OS processes; do not run independent writers against the same fixed output directory.

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

The debounced reload invalidates asset builds. The next page render builds from current sources. Generated outputs and temporary publication files are ignored by this watcher, even if static is underneath a watched directory. `reload: true` can additionally use HyperBricks' existing browser-reload behavior. This is lazy rebuilding, not an esbuild HMR server or an eager background compiler.

For the original development workflow, use `cache: false`: every component render builds, without waiting for cache invalidation. No HyperBricks binary rebuild is needed in either case.

## Build errors and output

If a build fails, HyperBricks reports the error and tries again on the next render. Files from the last successful build stay available and are not replaced by a broken build.

HyperBricks finishes writing generated files before returning their URL. Give each build its own `outfile`, keep generated output separate from source files, and make sure the static output directory is writable. Fingerprinted files are kept so older pages can still load them; remove obsolete files during deployment when needed.

Keep entry paths and build options in trusted application configuration rather than creating them from request data.

## Deployment and performance

Static rendering builds assets while rendering routes, before the existing static directory copy/export. Deploy the contents of `rendered/`; the destination does not need HyperBricks, esbuild, or the module's `resources/` directory to serve that static site. Bundled images/fonts are emitted into static output; intentionally external references still need to be available at their configured URLs.

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
- External mode now respects independent minification switches, matching the Go API. The old external plugin's `--minify` also enabled identifier minification.

The original esbuild and Tailwind plugins are unchanged.

---

## Credits and license

HyperBricks' native `esbuild` component uses the esbuild Go API. Credit goes to Evan Wallace and the esbuild contributors for creating and maintaining the compiler that powers this integration.

- **Website:** [esbuild.github.io](https://esbuild.github.io/)
- **Source repository:** [github.com/evanw/esbuild](https://github.com/evanw/esbuild)
- **License:** [MIT License — Copyright (c) 2020 Evan Wallace](https://github.com/evanw/esbuild/blob/main/LICENSE.md)
