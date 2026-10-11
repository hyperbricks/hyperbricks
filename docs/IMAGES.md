# Images

Use `image` to resize a local JPEG, PNG, or GIF and render an HTML image element. Use `images` to process files in a local directory.

These components do not download remote images or process SVG files. To display a public image URL or an SVG without processing, add an image element to your template.

## Single image

For an image stored in `modules/demo/resources/images/team.jpg`:

```yaml
team_photo:
  - type: image
  - src:
      path: {base: resources, path: images/team.jpg}
  - width: 800
  - cache: true
  - cache_keep: 1
  - cache_grace: 1h
  - alt: The project team outside the office
  - class: content-image
  - quality: 85
  - loading: lazy
  - attributes:
      decoding: async
```

Place this component inside a page or reference it through `inherit`. The `path` resolver uses the module's configured resources directory. A plain relative `src` instead resolves from the HyperBricks working directory, which normally is the project root.

The processed file is written to the configured static directory under `images/`. Its HTML URL starts with `/static/images/`, so it works from nested routes as well as `/`. Serve the application or static export at the site root.

## Size, encoding, and layout

| Setting | Behavior |
| --- | --- |
| Only `width` | Calculates height from the source aspect ratio. |
| Only `height` | Calculates width from the source aspect ratio. |
| Both dimensions | Resizes to that exact width and height, which can stretch the image. |
| Both omitted or `0` | Keeps the source dimensions. |
| `quality` | JPEG quality from 1 to 100; `0` or omission means 90. PNG and GIF ignore this setting. |
| `loading` | `lazy` or `eager`; omitted means no loading attribute. |

Dimensions are non-negative integer pixel counts. Percentage values belong in CSS. The generated HTML includes the processed width and height; use CSS to adapt its display size:

```css
.content-image {
  display: block;
  max-width: 100%;
  height: auto;
}
```

Processing re-encodes the source in its decoded format. GIF processing uses the first frame and does not preserve animation. To retain an animated GIF, serve the original static asset through a template instead.

## Attributes and alternative text

Attribute values are HTML-escaped automatically, including quotes and ampersands. Supply plain text; do not pre-escape `alt`, `title`, `id`, or `class`.

Supply meaningful `alt` text for informative images. An empty or omitted `alt` produces `alt=""`, which is appropriate for decorative images. An empty attribute alone does not make an informative image accessible.

The extra `attributes` map accepts `loading`, `decoding`, `srcset`, `sizes`, `crossorigin`, `usemap`, `longdesc`, `referrerpolicy`, `ismap`, `class`, and `tabindex`. Other keys, including event handlers, are ignored. Explicit `class` and `loading` fields take precedence over the same keys in `attributes`. `srcset` and `sizes` are passed through as escaped text; HyperBricks does not generate those additional variants automatically.

## Directory of images

```yaml
gallery:
  - type: images
  - directory:
      path: {base: resources, path: images/gallery}
  - width: 400
  - cache: true
  - id: gallery-image-
  - class: content-image
  - loading: lazy
  - alt: ""
```

Files are processed in filename order. JPEG, PNG, and GIF extensions are matched without case sensitivity; unsupported extensions and subdirectories are skipped. The example produces ids such as `gallery-image-0`. Omit `id` to produce no ids. `enclose` wraps the complete gallery.

The same `alt`, `title`, and other settings apply to every image. Use separate `image` components when images need individual descriptions. The empty `alt` in this example assumes decorative images.

An unreadable or invalid supported image reports its filename in the render diagnostics. The gallery component rejects its output if any file fails; it does not silently publish a partial gallery. Static export fails on these render diagnostics.

## Generated files and updates

Filenames include a fingerprint of the source bytes and processing settings, followed by the output dimensions. Different source content or JPEG quality produces different files even when the original basenames and dimensions match. Repeating the same content and settings yields the same filename. Files are published only after encoding completes, so concurrent renders cannot expose partially written images.

Do not construct generated filenames in templates. Use the HTML returned by the
component. After an upgrade from the older basename-only naming scheme,
regenerate and deploy HTML together with its static assets.

## Reuse and freshness

See [Asset caching](ASSET_CACHING.md) for practical development/live settings,
image examples and guidance on retaining old URLs.

| Field | Default | Behavior |
| --- | --- | --- |
| `cache` | `false` | True reuses a recorded valid output when source bytes and transformation settings match. False processes on each component render. |
| `cache_keep` | `0` | Number of previous successful generations retained per image, in addition to current. Must be nonnegative. |
| `cache_grace` | `1h` | Minimum retention after a generation is retired. A nonnegative Go duration, such as `15m`, `1h` or `0s`. Independent of `cache_keep`. |

Development renders the page on each request. With image caching enabled, changed
source bytes are detected even when the file size and timestamp stay unchanged;
unchanged requests reuse the generated output. Missing or modified output is
regenerated. Source checks and output validation still read bytes: a cache hit
avoids decoding, resizing, encoding and publication, not all filesystem work.

Watch reloads reread component definitions but preserve valid generated files.
Include resources in `development.watch_dirs` if their changes should trigger a
module reload. Generated image files and temporary publication files do not
trigger reloads. Browser refresh remains manual; the existing `reload` setting
does not implement automatic browser refresh.

Live mode can serve cached page HTML without checking images. When the page
cache expires or `nocache: true` requires rendering, the image cache policy
applies. Alt text, title, IDs, classes and other HTML attributes are produced
from the current component values, independently of the cached image bytes.

For `images`, every render reads the directory list. Added images are generated,
changed images are regenerated, and successfully discovering a removed file
retires that component's ownership of its output. Different components can
share identical generated files; another retained owner protects the file.

## Publication and cleanup

Init discovers owners without generating images. A successful generation is
published as a complete file, then recorded in private ownership history under
the operating system's HyperBricks image cache directory. A runtime/export
session excludes another process writing to the same output root. Concurrent
component renders serialize publication and reuse a successful cached result.

Cleanup removes only explicitly recorded image files with matching output bytes.
Unrecorded files are never inferred to be owned from a filename. Existing output
from older HyperBricks versions is not scanned and automatically deleted.
Externally modified obsolete files are preserved and their ownership released.
Symlink escapes, corrupt ownership records and non-regular owned outputs report
errors rather than authorizing deletion.

A generation is eligible only after it exceeds `cache_keep` and its retirement
grace has elapsed. Active requests, cached HTML and static export add protection.
Pending cleanup is retried after requests, or during init/module reload; there is
no periodic background cleanup when the server is idle. Cache hits and identical
uncached output do not rewrite unchanged ownership records.

If requests are active during reload, cleanup is scheduled for a later exclusive
maintenance window. Reload does not block a nested local asset request that an
API render needs in order to finish.

External consumers and open browser pages are not tracked runtime references.
Choose a grace period and retained-version count suitable for the lifetime of
URLs you distribute. `0s` explicitly removes that time allowance. An external
API that stores image URLs must refresh its manifest; `catalog-store` demonstrates
refreshing on each image-dependent request while reusing generated image files.
Generation or manifest errors remain visible and preserve previous files/data;
there is no silent successful fallback to stale source results.

See [live caching](LIVE_MODE_HTTP.md#storage-expiry-and-cleanup) when planning
content updates, and [static export configuration](HYPERBRICKS_CLI.md#package-configuration)
when generating a static site. The complete field list is generated in the
[component reference](REFERENCE.md#image).
