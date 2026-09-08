# Quickstart verification module

This module contains the example from [Quickstart](../../docs/QUICKSTART.md),
created with `hyperbricks init` and populated from the guide. It keeps the
scaffold's additional template and resource files alongside the quickstart files.

From the repository root, using the runtime built from this checkout:

```bash
hyperbricks start -m test-demo-001 --port 8131
```

Open http://localhost:8131/. Click **Load fragment** repeatedly: the card changes
and the JavaScript status counter increases. A full reload resets the counter.
External templates live in `templates/`; native esbuild bundles `resources/js/app.js`
and `resources/css/app.css`. The HTMX source is imported into the application
bundle. Generated assets and static export output are not committed.

HTMX 4.0.0 is vendored from
https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/htmx.js;
its license is included in `resources/vendor/LICENSE.txt`.
