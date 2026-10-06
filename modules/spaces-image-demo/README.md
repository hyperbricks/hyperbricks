# Spaces image demo

A small photography collection with two ready-made Spaces. Each inherits a
native image mounted two template-value levels deep. Edit one Space's original
image, alt text, caption, or nested collection subtitle without changing the
other Space or the reusable source.

Requires a HyperBricks build containing native image Spaces editing
(`v1.3.0-beta` or later). Earlier builds carrying the same beta version may not
include this feature yet. The module runs offline with bundled CSS and originals;
no npm install, plugin, external API, or build hook is needed to start it.

## Run

From the HyperBricks repository root:

```sh
go run ./cmd/hyperbricks start -m spaces-image-demo
```

With a current executable, use `hyperbricks start -m spaces-image-demo` instead.
Open [First study](http://localhost:8080/),
[Second study](http://localhost:8080/second), and
[Spaces](http://localhost:8080/__hyperbricks/spaces?name=home).
Use `--port 8081` if port 8080 is occupied.

The module is registered in `starters.index.json`. To install it from the
published starter catalog in another project:

```sh
hyperbricks init-starter get spaces-image-demo
hyperbricks start -m spaces-image-demo
```

## Try it

1. Open **Edit first study**. Use **Select existing asset** on **Original image**
   to choose `olive-lamp.jpg`. Change the alt text to describe the olive lamp,
   update the caption, and save.
2. Refresh the public page after the development watcher reloads. The first
   study changes; the second keeps the cream lamp and its inherited text.
3. Upload a JPEG, PNG, or GIF through the same image field and save. The original
   goes into `resources/images/originals/`. Native processing creates the
   fingerprinted web image under `static/images/` at the inherited width of
   720 pixels and JPEG quality 90.
4. Edit **Collection subtitle**. This demonstrates an explicit property path
   inside a plain nested object: `path: [details, subtitle]`.

Uploads are limited to 10 MiB and the image formats declared by the field. Save
writes only the selected Space's overrides. Browser refresh alone does not
reload the runtime if watching has been disabled; restart it in that case.

The native image field ID is:

```text
/template/values/content/values/image/src
```

The instance persists a portable resolver, not an absolute filename or a
manually maintained generated image URL:

```yaml
- src:
    path:
      base: resources
      path: images/originals/olive-lamp.jpg
```

To restore the inherited image, remove only the `src` override from that Space's
YAML, save the file, and reload Spaces. There is no content-field reset button.
The source image remains a required native property; clearing the control is
not a reset operation.

## Files

| File | Responsibility |
| --- | --- |
| `hyperbricks/study.hyperbricks.yaml` | Reusable image, source-owned editable contract, and nested page composition |
| `hyperbricks/spaces/study_source/index.hyperbricks.yaml` | Managed imports of the two Spaces |
| `hyperbricks/spaces/study_source/home.hyperbricks.yaml` | First study, route `/`, and its local overrides |
| `hyperbricks/spaces/study_source/second.hyperbricks.yaml` | Independent second study, route `/second` |
| `templates/page.html` and `templates/photograph.html` | Shared presentation |
| `resources/images/originals/` | Selectable originals and uploaded images |
| `resources/css/site.css` | Tailwind/daisyUI source for the bundled stylesheet |

The source grants editing rights. Adding local `editable` declarations to a
Space cannot grant more fields. Width, quality, templates, and other inherited
properties stay source-owned. Resource previews use the protected editor
endpoint; originals are not exposed as a public resources directory.

Open Graph, Twitter, and JSON-LD image bindings are not part of this example.
Independently inherited images do not automatically share a live processed
result.

## Development access

To require login, set both credentials before startup:

```sh
export HB_DEVELOPER_USER=editor
export HB_DEVELOPER_PASSWORD='replace-with-your-local-password'
go run ./cmd/hyperbricks start -m spaces-image-demo
```

Spaces is enabled with writes. When both developer credentials are absent it
opens without login on allowed hosts. Set both `HB_DEVELOPER_USER` and
`HB_DEVELOPER_PASSWORD` before starting to require login. Partial credentials
block access. Add an explicit hostname or IP to
`hyperbricks.development.frontend_editing.spaces.allowed_hosts` for LAN access.
Production mode does not expose Spaces. The editor links are shown when the
package's configured mode is development or debug.

## Styles and sample assets

The bundled `static/css/site.css` is built with Tailwind CSS and daisyUI. To
rebuild it from the repository root with the repository's dependencies and a
Tailwind v4 CLI available:

```sh
tailwindcss -i modules/spaces-image-demo/resources/css/site.css \
  -o modules/spaces-image-demo/static/css/site.css --minify
```

The cream and olive lamp images were generated for the repository's
`catalog-store` example and are reused here. Third-party stylesheet licenses
are bundled under `static/vendor/`; see [VENDOR.md](VENDOR.md).
