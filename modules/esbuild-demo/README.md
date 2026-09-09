# Native esbuild demo

This module contains a working estimate calculator, imported TypeScript and CSS, a copied logo, source maps, and development watch configuration.

## Development

From the repository root, using a HyperBricks build containing the native component:

```sh
hyperbricks start -m esbuild-demo
```

Open <http://localhost:8097/>. Adjust quantities or VAT and check the estimate. The calculation is browser-side TypeScript bundled by embedded esbuild, not server-side JavaScript execution. No plugin, Tailwind, Node runtime, or npm install is needed to run this module.

- `hyperbricks/page.hyperbricks.yaml` declares independent JS and CSS builds and inherits them into the page head.
- `resources/js/main.ts` imports the calculation from `calculate.ts`.
- `resources/css/site.css` imports `tokens.css` and references the logo. The file loader copies the logo next to the generated stylesheet with a hashed name.
- Both entries use `fingerprint: true`. The first uncached page request creates `static/js/app.<hash>.js`, `static/css/site.<hash>.css`, their source maps, and the image. Later renders reuse valid builds, including after restart when the private persistent manifest and its recorded files validate.
- Editing a resource triggers the existing development reload; refreshing the page rebuilds on demand. Generated static assets do not trigger a reload loop.
- Set a component's `cache: false` to rebuild it on every render. The page has `nocache: true` so HTML caching cannot bypass component rendering in this demo.
- Changed generated content produces a new asset URL. Previous versions remain available; there is no automatic cleanup. Set `fingerprint: false` for fixed names.

The `static` and `rendered` directories are generated and ignored. Source maps include source text; turn them off before publishing if that is undesirable.

See [ESBUILD.md](../../docs/ESBUILD.md) for options, migration, and limitations.

## Static export

```sh
hyperbricks static -m esbuild-demo --force --zip
```

The deployable site is in `modules/esbuild-demo/rendered/`, with the zip in the usual export directory. Deploy only the contents of `rendered/`. No running HyperBricks process or resources directory is needed at the destination. Source-map `sources` paths identify the original filenames; `sourcesContent` embeds the original CSS and TypeScript inside each map, so those filenames need not be public files.
