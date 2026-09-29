# Markdown

Use the native `markdown` component to render inline Markdown or a resource file as HTML. It does not produce a complete page. Place it inside `hypermedia` or `fragment` to define a route, response settings, guards, and caching. Use templates and CSS for layout.

The component is built into `v1.2.5-beta` and newer releases and needs no plugin
installation, build, or activation. Use documentation that matches your runtime
version and, for a development build, its source revision.

## Inline Content

```yaml
introduction:
  - type: markdown
  - content: |
      ## Welcome

      This is **Markdown** with a [link](https://example.com).
  - class: prose
```

`content` contains Markdown text. The component does not execute Go template expressions. An explicitly empty string is valid.

`class` adds a wrapping `div` with an escaped class value. Without it, the component returns only the rendered Markdown HTML. Use `enclose` to wrap the final output. Supply class names and enclosure markup through trusted developer configuration.

## Declared Files

```yaml
perspective_markdown:
  - type: markdown
  - file: uploads/documents/perspective.md
  - max_bytes: 1048576

perspective_fragment:
  - type: fragment
  - route: perspective
  - nocache: true
  - content:
      - inherit: perspective_markdown
```

Set `file` to a clean, slash-separated path relative to the module's configured `resources` directory. The component reads the file when it renders.

Only regular files with lowercase `.md` or `.markdown` extensions are accepted. The component rejects absolute paths, URLs, backslashes, dot traversal, and symlinks that escape the resources directory. Configure that directory through trusted module settings.

Exactly one of `content` and `file` must be set. Both inputs must be valid UTF-8
without NUL characters. `max_bytes` defaults to 1 MiB (1,048,576 bytes); explicit
values must be between 1 and 20 MiB (20,971,520 bytes). Missing or invalid files
produce source-aware render diagnostics, not filesystem details in page content.

The component never reads query parameters. `/perspective?file=another.md` renders
the same declared document as `/perspective`. Unknown document routes remain 404;
a component error on an existing route does not independently set its HTTP status.
Use a route guard when a declared document should not be public.

For developer-controlled load-time input, the existing file resolver also works:

```yaml
introduction:
  - type: markdown
  - content: {file: {base: resources, path: copy/introduction.md}}
```

That resolver reads before component execution; the component's byte limit is
checked after resolution and does not bound the resolver's read. Prefer the
component's direct `file` field for uploaded documents and bounded render-time reads.

## Rendering And Caching

Blackfriday v2 handles Markdown parsing, including tables and fenced code blocks.
Raw HTML is skipped; Bluemonday's user-content policy sanitizes generated HTML,
including link and image attributes. There is no raw-HTML bypass switch. Use an
explicit `html` component or a template for trusted application markup instead.
See [Blackfriday's sanitization guidance](https://github.com/russross/blackfriday#sanitize-untrusted-content)
and [Bluemonday](https://github.com/microcosm-cc/bluemonday).

The renderer does not fetch remote content. Visitors' browsers may load allowed remote images. Use the route's Content Security Policy to restrict those requests. Sanitization does not determine which documents you should publish.

The component has no output/file cache of its own. A cached containing route may
skip rendering; `nocache: true` on the route forces fresh reads. Development
resource watching retains its existing reload/cache invalidation responsibilities.
With watching disabled, refresh or reload as appropriate to the route's cache
policy. Package configuration changes still require a restart.

Markdown works in development, live mode, and static rendering. It needs no
request data and also supports requestless component rendering. Static output is
a snapshot and must be rebuilt after document edits.

## Spaces Editing

Declare editing metadata on a Markdown component nested in a hypermedia source:

```yaml
document_page:
  - type: hypermedia
  - route: document
  - title: Document
  - content:
      - type: markdown
      - file: uploads/documents/perspective.md
      - editable:
          file:
            type: asset
            label: Perspective document
            required: true
            edit: {type: markdown, max_bytes: 1048576}
            upload:
              accept: [.md, .markdown]
              max_bytes: 1048576
              directory: {base: resources, path: uploads/documents}
```

Create a Space from `document_page`. Uploads, selection, and **Make a copy** update
the Space's `content.file` reference. Editing a document updates the referenced
file, with the existing shared-document confirmation and revision protection.
The renderer reads that same field; no duplicate reference or document registry.

Only `file` and `content` can be editable on Markdown. File editing requires an
asset field with an explicit resources directory, accepts only Markdown, cannot
clear the required file reference, and must respect the renderer's byte limit.
For editing existing files without uploading, replace `upload` with a `directory`
and retain `edit`. Existing template `editable` fields still target template values.

Inline content can use the existing textarea control:

```yaml
introduction:
  - type: markdown
  - content: '## Welcome'
  - editable:
      content: {type: textarea, label: Introduction, rows: 8}
```

Place this inside a hypermedia source to edit it in Spaces. File-backed fields use
the Markdown document editor; inline fields use the ordinary textarea. CMS preview
and public Markdown share the same rendering/sanitization function; the preview
also retains its sandboxed iframe and restrictive CSP.

For a component inside template `values`, use a named reusable definition with
`inherit`. Repeat that `inherit` reference in instance overrides;
inheritance references cannot traverse through nested template `values` mappings.

Spaces remains development-only with writes explicitly enabled. The Markdown
renderer is independent of the CMS. Uploading an unreferenced file creates no
route. Trashing a Space removes its route but does not delete shared files or
remove other explicitly declared routes that reference them.

## Further Examples

See [Markdown type examples](HYPERBRICKS_TYPE_EXAMPLES.md#markdown) for inline and
file-backed components, and [Spaces](SPACES.md) for document editing and uploads.
The [Localized Spaces pattern](../modules/hyperbricks-patterns-yaml/docs/pages/localized-spaces.md)
provides a runnable example of page sources and translated instances.
