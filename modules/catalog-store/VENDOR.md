# Local browser assets

The module bundles its browser dependencies so it can run without npm or a
font CDN at runtime.

| Asset | Source | License |
| --- | --- | --- |
| `resources/vendor/htmx-4.0.0.js` | [HTMX 4.0.0](https://github.com/bigskysoftware/htmx/tree/v4.0.0) | [Zero-Clause BSD](static/vendor/LICENSE.txt) |
| `static/fonts/fraunces-latin-variable.woff2` | [Fraunces](https://fonts.google.com/specimen/Fraunces), Latin variable subset | [SIL Open Font License](static/fonts/Fraunces-OFL.txt) |
| `static/fonts/dm-sans-latin-variable.woff2` | [DM Sans](https://fonts.google.com/specimen/DM+Sans), Latin variable subset | [SIL Open Font License](static/fonts/DM-Sans-OFL.txt) |

The HTMX file is imported by `resources/js/catalog.js` and bundled by native
esbuild. SHA-256: `5d0833e3b435d357221955566f46fa378cb653c4f119c0a8533b4bb4098cf9ad`.
The two WOFF2 files are self-hosted under `/static/fonts/` and preloaded in the
page head; both use `font-display: swap`.
