# Images

Use `image` to resize a local JPEG, PNG, or GIF and render an HTML image element. Use `images` for files in a local directory. These components do not download remote URLs or process SVG. To display an existing public URL or SVG without processing, use an ordinary image element in a template.

## Single image

For an image stored in `modules/demo/resources/images/team.jpg`:

```yaml
team_photo:
  - type: image
  - src:
      path: {base: resources, path: images/team.jpg}
  - width: 800
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

Do not construct generated filenames in templates. Use the HTML returned by the component. Old generated files remain available for already-rendered pages; image processing does not delete previous versions. After an upgrade from the older basename-only naming scheme, regenerate and deploy HTML together with its static assets. Remove unused generations as part of a controlled rebuild or deployment cleanup.

In live mode, a cached page can continue referencing its earlier image until the page is rendered again. See [live caching](LIVE_MODE_HTTP.md#expiry-updates-and-memory) when planning content updates, and [static export configuration](HYPERBRICKS_CLI.md#package-configuration) when generating a static site.

The complete field list is generated in the [component reference](REFERENCE.md#image).
