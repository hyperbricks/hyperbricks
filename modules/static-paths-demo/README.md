# Static paths demo

Run from the repository root:

```bash
go run ./cmd/hyperbricks start -m static-paths-demo --port 8096
```

Open <http://localhost:8096/> and <http://localhost:8096/nested/demo>.
Both pages load `/static/demo.css` and `/static/thefile.txt`. The green page's
marker says it comes from the module-relative static directory.

Stop the server, then select the root-relative configuration:

```bash
go run ./cmd/hyperbricks start -m static-paths-demo --config package.root.hyperbricks.yaml --port 8096
```

The page turns blue and the marker changes, but the asset URLs stay the same.

| Configuration | Static directory on disk | Marker URL |
| --- | --- | --- |
| `package.hyperbricks.yaml` | `<module>/static` (`base: module`) | `/static/thefile.txt` |
| `package.root.hyperbricks.yaml` | `<working directory>/modules/static-paths-demo/root-assets` (`base: root`) | `/static/thefile.txt` |

The page also displays the filesystem path produced by `base: static`; do not
use that path as an HTML URL. A leading `/static/` works from nested routes,
whereas `static/thefile.txt` on `/nested/demo` resolves to
`/nested/static/thefile.txt`.

With a compiled CLI and an absolute `-m` path, the module-relative configuration
also works from another working directory. The root-relative configuration
looks under that new working directory instead; selecting a module does not
change it.

Inspect the served marker with:

```bash
curl --fail http://localhost:8096/static/thefile.txt
```
