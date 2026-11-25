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

Assets are built when a page first uses the component, not when the module loads.
That first request waits for the build unless HyperBricks can reuse a checked
build from a previous run.

- With `cache: true`, later renders reuse the output URL without reading or
  checking the asset files again.
- With `cache: false` (the default), each component render builds the asset again.

The generated files stay on disk. The page contains their URLs, and the browser
fetches them through separate `/static/` requests. Pages using the same source,
output path and build options can share a cached build, including through inheritance.
Changing `enclose` or `debug` does not require a separate build.

### Picking up changes

In development, enable watching and include all relevant source and configuration
directories in `development.watch_dirs`. Add `resources` explicitly: it is not
watched by default. See the [watch configuration](#development-watch) below.

A watched change reloads the module. The next page render rebuilds the asset.
Refresh your browser to see the result.

If watching is disabled, or you change files outside the watched directories,
restart HyperBricks. You can also reload the module explicitly if your runtime
provides that action. A module reload forces a rebuild on next use; a restart
first checks whether the saved build is still usable.

Avoid manually changing or deleting generated files while the server is running.
Cached renders do not check for those changes. Restart to detect changed or
missing files and rebuild as needed. When HyperBricks itself changes generated
files used by another esbuild component, it clears that component's cached build.

### Asset caching and page caching

A cached page returns its HTML without running its components. In live mode,
set `nocache: true` on a page if every request must render it. You can keep
`cache: true` on its assets. Development mode already renders every request.

Use `fingerprint: true` to give changed builds new URLs for browsers and CDNs.
This does not refresh cached page HTML or change HTTP cache headers.
See [Asset caching](ASSET_CACHING.md) for complete examples.

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

With `cache: true`, HyperBricks saves build information so it can reuse generated
files after a restart. At first use, it checks the saved information against the
sources and generated files. Changed sources, build options or compiler versions,
and missing or damaged output files cause a rebuild. These file checks happen
before reuse after a restart, not on every page request.

Keep the source files and dependencies available when running HyperBricks.
Generated files alone are not enough to reuse a build. A deployed static site
needs only the exported files; see [deployment](#deployment-and-performance).
Each fresh static export builds independently of previous exports.

Build information is stored in your operating system's user cache directory,
under `hyperbricks/esbuild/`. On macOS this is usually
`~/Library/Caches/hyperbricks/esbuild/`; on Linux it is
`$XDG_CACHE_HOME/hyperbricks/esbuild/` or `~/.cache/hyperbricks/esbuild/`.
Keep this private directory outside static output: it contains local source paths
and must not be deployed with your site.

### File tracking and write conflicts

HyperBricks also keeps records of the files it generated so cleanup can leave
handwritten files alone. Let HyperBricks manage these records. Deleting them can
leave old generated files behind.

These records use a fixed location under your account, even if you override the
build-cache directory:

- macOS: `~/Library/Caches/hyperbricks/esbuild/`
- Linux: `~/.cache/hyperbricks/esbuild/`
- Windows: `hyperbricks/esbuild/` under Local AppData

When upgrading from a custom cache location, start once with your previous cache
environment settings so HyperBricks can migrate the records. It does not search
all former cache locations.

The cache and tracking directories must be writable. If another process is
already writing to the same output directory, HyperBricks reports a conflict.
Stop that process or choose a different output directory; do not delete lock
files to bypass the check. The lock is released when its process exits.

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

When a watched file changes, HyperBricks reloads the module. The next page request builds the assets from the updated sources. Generated assets are ignored by the watcher, even if static is inside a watched directory. Refresh your browser manually: the `reload` setting does not currently refresh it automatically. Restart HyperBricks after changing the package configuration itself.

For the original development workflow, use `cache: false`: every component render builds, without waiting for cache invalidation. No HyperBricks binary rebuild is needed in either case.

## Build errors and output

If a build fails, HyperBricks reports the error and tries again on the next render. Files from the last successful build stay available and are not replaced by a broken build.

HyperBricks finishes writing generated files before returning their URL. Give each build its own `outfile`, keep generated output separate from source files, and make sure the static output directory is writable. Generation retention handles owned fingerprints automatically; remote deployment retention remains the responsibility of your publishing workflow.

Keep entry paths and build options in trusted application configuration rather than creating them from request data.

## Deployment and performance

Static export copies your static files and writes generated scripts, styles and
images into `rendered/static/`. It leaves your development assets in place, so
you can export while the development server is running. Public asset URLs stay
`/static/...`.

If a build or image component uses a file generated earlier in the export, it
reads the exported version. Other source files keep their usual paths. Linked
files are copied into the export as regular files, so the exported site does not
depend on the original symlink targets.

Keep the export destination separate from your source directories and other
running exports. HyperBricks rejects conflicting or unsafe destinations before
clearing them. Avoid editing sources or running custom scripts that change them
during export: it does not freeze the entire project while building.

Deploy the contents of `rendered/`. The destination does not need HyperBricks,
esbuild, or the module's `resources/` directory. Assets marked `external` still
need to be available at their configured URLs.

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
