<!-- Generated from docs/SPACES.md. Do not edit directly. -->

# Spaces

**Spaces** lets you create and edit pages that inherit an existing HyperBricks page. Each page can have its own content, URL, and metadata. Use Spaces for articles, landing pages, or translations that share a layout.

A **source** is a named configuration that resolves to `hypermedia`. It defines the shared layout, templates, default values, and editable fields. A **Space** inherits the source and stores its changes in YAML. HyperBricks renders it through the normal components and routes.

| Owner | Defines |
| --- | --- |
| Source page | Shared layout, defaults, and `editable` declarations |
| Space | Page name, route, title, content overrides, and metadata |
| Module configuration | Development editor access, write settings, and file watching |

Use the browser editor to create Spaces, edit declared fields, manage metadata and assets, or move a Space to recoverable **Trash**. Use `hyperbricks space` to create instances through a wizard or explicit flags. Agents and automation can use `hyperbricks author` to define sources and extend their configuration.

Spaces is built into HyperBricks v1.2.5-beta and requires no plugin. The editor runs only in development. Public pages also work with the editor disabled. You can deploy or export them through the normal HyperBricks workflows. Saving writes source files. It does not publish the site or translate content automatically.

Start with [Development Configuration](#development-configuration), declare the [Source Schema](#source-schema), then [create and edit a Space](#source-files-and-crud). The [Night Owl Café example](#run-the-night-owl-cafe-example) at the end is a complete runnable demonstration.

## Development Configuration

Enable the dashboard, file watching, and editor writes in the module's `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  mode: development
  development:
    dashboard:
      enabled: true
      credentials:
        user:
          env: HB_DEVELOPER_USER
        password:
          env: HB_DEVELOPER_PASSWORD
    watch: true
    watch_dirs: [hyperbricks, templates, resources]
    frontend_editing:
      enabled: true
      spaces:
        enabled: true
        write: true
```

Always run the command from the project root, which contains `modules/`:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
hyperbricks start -m demo --port 8080
```

Replace `demo` with your module name. Open the [Spaces
editor](http://localhost:8080/__hyperbricks/spaces) or the [Dashboard
Overview](http://localhost:8080/__hyperbricks/dashboard). Restart the module
after changing package settings.

Without an explicit `write: true`, the editor is read-only. File watching reloads saved YAML; refresh the public page separately to see the result.

### Optional settings

```yaml
hyperbricks:
  mode: development
  development:
    dashboard:
      enabled: true
      credentials:
        user:
          env: HB_DEVELOPER_USER
        password:
          env: HB_DEVELOPER_PASSWORD
    watch: true
    watch_dirs: [hyperbricks, templates, resources]
    frontend_editing:
      enabled: true
      spaces:
        enabled: true
        route: /__hyperbricks/spaces
        write: true
        public_origin: https://example.com
        # Explicitly opt in when editing over a trusted LAN:
        # allowed_hosts: [editor.example.test]
        sharing_image:
          accept: [.jpg, .jpeg, .png, .webp]
          max_bytes: 5242880
          directory: {base: static, path: uploads/images}
```

Restart after changing package configuration. Set `hyperbricks.development.frontend_editing.spaces.enabled: false` to hide only Spaces, while leaving other configured frontend editors available. This setting defaults to `true` for existing modules. The parent `hyperbricks.development.frontend_editing.enabled: false` still disables all frontend editors, including Spaces, regardless of the Spaces switch. Omitted settings enable the development mount but never writes. With `write: false`, you can read the catalog and forms, but cannot change files or upload assets.

`development.dashboard.credentials` is the module's one developer-interface
login. It protects the Dashboard's Overview and Errors views, render
diagnostics, Spaces, configured frontend-editor plugins, contextual editing
through `?edit=true`, and frontend error panels. `dashboard.enabled` controls
only the Dashboard's Overview and Errors views; independently enabled tools
such as Spaces still use the same credentials
when the Dashboard is disabled. Public application routes remain accessible
without this login and do not receive developer-only panels or edit controls.

There is no default developer username or password. If either resolved value is
empty, enabled developer routes return `503 Service Unavailable`. Missing or
incorrect browser credentials return `401 Unauthorized` with a Basic Auth
challenge. Environment changes require a process restart.

The editor rejects access outside development, including production, even when explicitly enabled. Spaces checks the configured host and browser origin. It ignores forwarded-host/origin headers. Opening the CMS creates no module files.

The route must be a clean `/__hyperbricks/` path. It cannot overlap diagnostics or another editor. Configure Spaces under `hyperbricks.development.frontend_editing.spaces`.

Configure external frontend-editor plugins under `development.frontend_editing.editors.<name>` with `plugin`, `route`, and `data`. These plugins still require explicit `plugins.enabled` entries. Spaces does not.

HTTP Basic Auth does not encrypt credentials. Loopback HTTP is suitable for
local development; LAN access needs HTTPS, a TLS reverse proxy, or a private
encrypted network. Browser-managed Basic Auth has no reliable application-level
logout, so closing the browser session or clearing its stored credentials may
be necessary. Spaces is not a native-code sandbox: native plugins remain
trusted application code and must not be treated as untrusted multi-tenant
extensions.

## Source Schema

Declare the fields users may edit in the source's `editable` configuration. On a `template`, each field targets `values.<key>`. On `markdown`, it targets the component's own `file` or `content`. An instance cannot add editing rights. See [Markdown and Spaces editing](MARKDOWN.md#spaces-editing) for native Markdown composition.

A minimal source declares a template, default values, and editable fields:

```yaml
page_source:
  - type: hypermedia
  - title: Page source
  - content:
      - type: template
      - inline: '<article><h1>{{.heading}}</h1><p>{{.intro}}</p></article>'
      - values:
          heading: Welcome
          intro: Start with a shared page layout.
      - editable:
          heading: {type: text, label: Heading, max: 120, required: true}
          intro: {type: textarea, label: Introduction, rows: 3, max: 800}
```

Place this in a loaded `*.hyperbricks.yaml` file. It has no route, so only its Spaces expose public URLs. An existing routed page can also be a source.

Spaces supports nested templates such as `content.values.body`. Use `editable: [heading, intro]` for single-line text fields, or compact mappings such as `intro: textarea`. Field metadata supports `type`, `label`, `help`, `placeholder`, `required`, `rows` (1-40), and `max` (Unicode code points). Spaces rejects unknown controls and metadata.

### Asset fields

An asset control stores a scalar image or document reference. Declare its storage directory and upload policy on the source:

```yaml
- editable:
    hero_image:
      type: asset
      label: Hero image
      upload:
        accept: [.jpg, .jpeg, .png, .webp]
        max_bytes: 5242880
        directory: {base: static, path: uploads/images}
```

The template must have a matching `values.hero_image` and render that value where the image belongs. A Markdown asset reference is a filename; use a native `markdown` component to render its contents. See [Markdown Example](#markdown-example).

Each asset field holds one scalar reference. Use `directory` to restrict selection without enabling uploads. Without either directory option, users can select existing supported assets under `static`.

Use `upload` to enable uploads and set extensions, size, and storage. Storage must be inside the module, under `static` or `resources`. If you set both `directory` and `upload.directory`, they must agree.

Spaces supports JPEG, PNG, WebP, GIF, and UTF-8 Markdown. Decoded image content must match the extension. Images cannot exceed 40 million pixels, and each policy is limited to 20 MiB. Spaces rejects SVG/HTML/executable uploads.

The source schema defines which fields you can save. An instance's local `editable` cannot add fields. The editor escapes labels and content. Each save validates all effective content, including required values, and accepts only declared field identities. Paths sent by the browser never choose a write destination.

## Source Files And CRUD

### Create and edit

Choose **Create Space** in the browser editor. Select a source and enter a unique name, title, and route. The new Space inherits the source's layout and editable defaults. Select it in the list, change its content or metadata, and choose **Save Changes**. After the watcher reloads the configuration, refresh the public page.

To create a Space from the CLI, open the wizard or list the available sources:

```sh
hyperbricks space -m demo
hyperbricks space -m demo --list
```

For the minimal `page_source` above, preview a new instance:

```sh
hyperbricks space -m demo \
  --source page_source \
  --name welcome_page \
  --title Welcome \
  --route welcome \
  --dry-run
```

Remove `--dry-run` to write the Space and required imports. For another project, choose a source name from `--list`. Add `--json` for structured output. The CLI creates the instance. Edit its values afterward through the browser or source YAML. See [Create a Space](AUTHOR.md#create-a-space) for the authoring contract. Use `hyperbricks author` to add a source or extend its structure.

### Files and inheritance

Sources are named roots/orphans that resolve to `hypermedia` through loaded top-level files and imports. Unimported files do not become active sources. Omit the source's `route` if only its instances should have public pages. The source remains available in the CMS. Keep its route if the source should also be public.

To create a Space, choose a source, component name, title, and route. Names must be ASCII identifiers of up to 80 characters. Editor-created routes must be clean relative paths; `index` denotes `/`. This version does not author dynamic/query/fragment routes or runtime-reserved prefixes.

Spaces stores generated instances in `hyperbricks/spaces/<source>/<name>.hyperbricks.yaml`. The file declaring the source imports their sibling `index.hyperbricks.yaml`. Each source keeps its own import scope. There is no global index for unrelated sources. This keeps the source and its instances in a valid runtime resolution graph.

You can create a Space from an existing Space. Its dependencies/imports prevent you from trashing the source Space until you remove those dependencies.

Instances inherit the source and copy editable defaults into the correct nested component containers. They leave the inherited template, noneditable values, head assets, and sibling instances unchanged. You can edit titles and routes. Instance/source names and filenames remain stable. Spaces checks names and routes for conflicts across active definitions.

### Trash and restore

Choose **Move to Trash** to deactivate a Space, or restore it from the editor's **Trash** list. Dependencies can prevent either action.

Trash comments out the instance's import. It does not move or delete the file:

```yaml
imports:
  - english.hyperbricks.yaml
  # - dutch.hyperbricks.yaml
```

Only commented scalar entries in a loaded, managed `spaces/<source>/index.hyperbricks.yaml` count as **Trash**. A managed index contains only block-style imports to sibling instance files. The index stays imported even when all entries are commented out. Restore validates the retained source, name, and route. Alternate active imports, shared files, and inheritance dependencies prevent unsafe trashing.

Whole-module deployment archives include trashed sources. Spaces has no expiry, permanent deletion, automatic asset cleanup, hidden registry, or database.

## Edit From The Page

Select a Space and choose **Edit page**, or open its development page with `?edit=true`. HyperBricks matches the served route to an active managed Space. It adds edit controls to content with explicit field mappings. Ordinary requests, live/production responses, and static exports do not include these controls.

The query parameter does not grant access or enable writes. A valid
`development.dashboard.credentials` login is required before contextual editing
is activated. The development, host/origin, and Spaces write settings still
apply after authentication.

Use `data-hb-space-field` in the application template to map an element to a source-owned field. Copy the exact canonical field ID from the Space's editor catalog. Do not use a label or guess a key:

```html
<h1 data-hb-space-field="/content/values/heading">{{.heading}}</h1>
<p data-hb-space-field="/content/values/intro">{{.intro}}</p>
```

These examples use a `content` template with `heading` and `intro` declared under `editable`. A nested template may have an ID such as `/content/values/body/values/heading`. Read the catalog to find the actual nesting.

Markdown fields target the component's own `file` or `content`, such as `/content/file`. Put the marker on the element containing the rendered document. IDs use JSON Pointer escaping: `~0` for `~` and `~1` for `/` inside a key.

Multiple elements may map to the same field, such as a repeated call-to-action.
Each edit control opens the same source field. A loop can generate IDs with Go
template `printf`, provided each result matches a declared catalog field:

```html
{{range $n := list 1 2 3}}
<h2 data-hb-space-field="{{printf "/content/values/card%d_title" $n}}">
  {{index $ (printf "card%d_title" $n)}}
</h2>
{{end}}
```

Outside edit mode, markers are inert HTML attributes. They do not grant editing rights, enable writes, or change rendered values. The source's `editable` declarations still control access. Unmapped code samples and hardcoded template text receive no edit controls.

Contextual links open the editor with the selected Space and field, for example:

```text
/__hyperbricks/spaces?name=landing_en&field=%2Fcontent%2Fvalues%2Fhero_line1
```

The editor selects the Space, scrolls to the field, and gives it focus and a highlight. Unknown or trashed Spaces and unknown fields produce an explicit message. The editor does not select another target.

Generated links use the configured `hyperbricks.development.frontend_editing.spaces.route`. Use that prefix when writing your own link. Opening a contextual link does not save or publish anything. Continue with the editor's save, validation, and conflict workflow.

## Metadata

Title and route use the existing page fields. The metadata tool's `+` and `-` controls edit `head.meta`. You can use custom keys or common/Open Graph presets. Removing an inherited tag writes an explicit null. Resetting it to the source removes the local override:

```yaml
welcome_page:
  - inherit: page_source
  - route: welcome
  - head:
      - type: head
      - meta:
          author: null
          description: An introduction to our site.
          og:title: Welcome
```

The standard HEAD renderer skips null entries. It uses `property` for `og:` keys and `name` for ordinary keys, and escapes keys, content, and titles. This works with the CMS disabled and survives reload, restart, and static rendering. Empty strings remain distinct from removal. Edits do not change source or sibling metadata.

Open Graph presets include title, description, image, image description, URL, type, site name, and locale. The editor indicates incomplete basic sharing metadata. Set explicit absolute HTTP(S) URLs for `og:url` and `og:image`.

Sharing-image uploads and selection require the module's `sharing_image` policy and `public_origin`. Spaces never substitutes the development host/IP for the public site origin. This map-shaped contract represents one sharing image.

## Persistence And Watching

Each mutation includes a catalog revision for loaded YAML, retained **Trash**, and the top-level source set. Changes to schemas, instances, or indexes invalidate stale forms. YAML node edits preserve ordering and comments, though whole-file encoding may normalize whitespace. Trash changes only the relevant import line.

Spaces writes through temporary files, sync, and replacement inside Go `os.Root` directory handles. It rejects existing symlinks, escaping paths, ambiguous definitions, and overwrite collisions. New instance/asset files use no-overwrite links.

Creation writes the instance first, then the index, then the parent import. An interruption therefore does not add an import to a missing file. Failure messages identify retained recovery files. They do not overwrite external changes.

Uploads and their reference saves share validation. Replacing or clearing a reference does not delete the old asset. If Spaces saves an asset but the YAML write fails, the old reference remains. You can recover the new, unreferenced asset manually. Spaces saves multiple selected uploads sequentially, without a cross-file transaction. Optimistic revisions do not provide multi-process locking.

Spaces does not publish, reload the runtime, or start a watcher. **Saved** means it persisted the source file. `development.watch` and `watch_dirs` control asynchronous configuration refresh. With watching disabled, reload or restart manually. Refresh the browser page separately. Package changes always require a restart.

## Markdown Example

Use a native `markdown` component to edit inline text or a resources-relative document reference:

```yaml
article_copy:
  - type: markdown
  - content: |
      ## Welcome

      Write the article here.
  - editable:
      content: {type: textarea, label: Article, rows: 8}
```

Compose this component inside the source page. For a file-backed document, use `file` instead of `content` and declare `editable.file` with an asset policy. Uploading or copying changes the reference used for rendering. File selection is explicit; query parameters do not choose another document.

Only `content` and `file` are editable on Markdown. The editor preview and public renderer share sanitized HTML generation. See [Markdown](MARKDOWN.md) for complete file-backed examples and rendering constraints.

## Editorial Workflow

At 850px and below, the desktop sidebar becomes a modal Spaces drawer opened
by the **Browse Spaces** button. It keeps the list, search, **Trash**, and
**Create** controls. Selecting a Space closes the drawer and returns keyboard
focus to its trigger.

Select an image thumbnail to open a preview modal. It offers fit/original-size controls, dimensions, and file size when available. Remote previews do not make cross-origin requests to probe image size.

Source fields can declare explicit order and a group:

```yaml
- editable:
    name: {type: text, label: Name, order: 10, group: Introduction}
    welcome: {type: textarea, label: Welcome, order: 20, group: Introduction}
    email: {type: email, label: Email, order: 30, group: Contact}
    article_markdown:
      type: asset
      label: Perspective document
      order: 40
      group: Documents
      directory: {base: resources, path: uploads/documents}
      edit: {type: markdown, max_bytes: 1048576}
```

Set `order` to an integer from -100000 to 100000. The default is zero. Fields with equal order use stable field identity, alphabetical within a template. Runtime data maps do not preserve declaration order. Groups follow their first ordered field and keep field order within the group. Ungrouped fields have no heading.

Declare `edit` separately from `upload`. It requires an asset field, an explicit `directory` or `upload.directory`, `type: markdown`, and `max_bytes` between 1 and 1048576. Upload size and extension rules still apply when present.

The example allows users to edit and copy existing Markdown, but not upload files. Add an `upload` block to enable both. Upload permission alone does not allow content editing.

Save or discard pending Space changes before opening a document. The document editor has its own **Write**, **Preview**, and **Save** actions. It edits the persisted field's file. The browser cannot choose an arbitrary filename.

Preview uses the public renderer's Blackfriday policy: raw HTML is disabled and links must be safe. It runs in a sandboxed iframe without scripts, forms, or parent navigation. The stylesheet is local. The preview disables embedded image/resource loading and shows document content without the website's template or layout.

Spaces finds known file references in declared asset fields of active named `hypermedia` sources and managed Spaces, including **Trash**. It stores no registry. It cannot discover references hidden in templates, plugin code, arbitrary strings, Markdown links, or external applications. An unresolvable trashed source cannot supply effective field relations.

The count shows **known references**. It does not prove that a file is globally unused.

- **Save Shared Document** requires confirmation and changes file bytes only.
  All consumers of that file see the edit. Source declarations and YAML references
  stay unchanged. Known referencing fields' upload size limits are also enforced.
- **Make a Copy** creates a unique sibling-policy file and updates only the
  selected Space's asset reference. Other references and the original file remain.
- Both operations validate the current source editing permission, YAML/catalog
  revision, and the Markdown file's own SHA-256 revision. Sharing membership
  changes invalidate old confirmations. Trash/read-only/non-development writes,
  path escapes, symlinks, oversized or invalid text are rejected.
- A document conflict preserves the draft and shows previously loaded/currently
  saved content. Keeping the draft adopts the current revision for a subsequent
  explicit Save; it does not immediately overwrite the file.

`GET <route>/api/document?name=<space>&field=<field-id>` reads a permitted document.
`POST <route>/api/document` accepts `action` (`preview`, `save`, or `copy`), `name`,
`field`, catalog `revision`, `file_revision`, `content`, and explicit `shared`
confirmation for a shared in-place save. Permission and origin checks are the
same as other editorial writes. The server derives all filesystem destinations.

If a Space save conflicts with newer changes, choose **Review Changes**. It compares the original, saved, and draft values. You can reapply nonoverlapping changes. Overlapping values require an explicit choice. A stale draft cannot restore removed fields or upload permissions.

Applying your choices updates the form to the newer snapshot. You must still choose **Save Changes**, which checks revisions again. The editor keeps pending uploads only for fields whose draft you keep and whose upload permission remains. Drafts stay in memory and do not survive tab closure or browser crashes.

Uploads show byte-transfer progress, followed by the persistence state. The editor disables **Clear** for required assets. Metadata uses URL inputs for sharing URLs and a content-type selector that preserves custom values. Browser feedback and server validation count text limits in Unicode code points.

## Editor API

For local development automation, use the configured editor route as the API prefix. The default is `/__hyperbricks/spaces`. The frontend uses this API too. It is not a production publishing API. Match the contract to the running version.

| Request | Contract |
| --- | --- |
| `GET <route>/api` | Catalog with `revision`, `sources`, `spaces`, fields and current values, metadata, and write/watch settings. |
| `POST <route>/api` | JSON mutation: `create`, `save`, `trash`, or `restore`. |
| `GET <route>/api/assets?name=...&field=...` | Selectable assets for that Space and catalog field ID. URL-encode query values. |
| `POST <route>/api/upload` | Multipart fields `mutation` (JSON save mutation), `field` (catalog field ID), and exactly one `file`. Validates the upload and saves its reference together. |
| `GET/POST <route>/api/document` | Document read/preview/save/copy; see the document protocol in [Editorial Workflow](#editorial-workflow). |

Every editor API request requires HTTP Basic Auth with the module's
`development.dashboard.credentials`. Write requests additionally require
`X-Spaces-Request: 1` and cannot be cross-origin. For local automation, send
the actual server origin in `Origin`. Use `Content-Type: application/json` for
JSON mutations. For uploads, let the HTTP client set the multipart boundary.
Development mode, allowed host, and explicit write policy still apply after
authentication.

Creation example, using a source name from the catalog:

```json
{
  "action": "create",
  "revision": "REVISION_FROM_LATEST_CATALOG",
  "source": "page_source",
  "name": "welcome_page",
  "title": "Welcome",
  "route": "welcome"
}
```

For `save`, send the current `revision`, instance `name`, `title`, and `route`.
Optional `values` maps the exact field IDs returned by the catalog to strings.
For the simple source in [Source Schema](#source-schema), a text save looks like:

```json
{
  "action": "save",
  "revision": "REVISION_FROM_LATEST_CATALOG",
  "name": "welcome_page",
  "title": "Welcome",
  "route": "welcome",
  "values": {"/content/values/heading": "Welcome back"}
}
```

Field IDs depend on component nesting. Check the catalog before using the example ID. `meta` maps keys to string overrides or null for removal. `reset_meta` lists keys to reset to inherited values. For `trash` or `restore`, send `action`, `revision`, and `name`.

Successful instance/upload mutations return `{"saved":true,"name":"..."}`. Fetch a fresh catalog before the next mutation. On HTTP 409, reread the data and resolve the conflict. Do not retry blindly with a new revision. Document writes also require their file revision and shared-save confirmation. Runtime refresh is asynchronous. Check the public response separately from file persistence.

## Verification

After creating or editing a Space, check the managed YAML, imports, editor catalog entry, and public route. Confirm the expected content and `X-Hyperbricks-Render-Error-Count: 0`. HTTP 200 alone does not prove that all components rendered successfully.

For translations, check each language route, document language, title, metadata, and link destination. An editor save persists YAML. Runtime reload and browser refresh are separate steps.

| Symptom | Check |
| --- | --- |
| A source or Space is absent from the catalog | The loaded import graph: top-level source files load automatically, nested files need explicit imports, and the managed `spaces/<source>/index.hyperbricks.yaml` must import active instances. |
| A translated field is missing from the editor | The `editable` declaration on the owning source component. For a nested template, `editable.heading` targets that template's `values.heading`, not a similarly named outer value. |
| A saved translation is not visible on its route | `development.watch` and `watch_dirs`, server reload or restart, route cache, and a separate browser refresh. Package changes always need a restart. |
| A route returns 200 but has incomplete output | Check `X-Hyperbricks-Render-Error-Count` and the request ID in `/__hyperbricks/render-diagnostics`; nested component errors can coexist with HTTP 200. |
| `no pages found for section 'primary'` after replacing navigation | Inspect inherited template `values` for an unused `menu` component. Component-valued fields execute during rendering even if the template no longer prints them; remove the stale field or configure its section. |
| YAML fails to parse after adding prose | Quote scalars containing `: `, or use a YAML block scalar for paragraphs. |
| `space` or `author` is unavailable | Compare the selected binary with this checkout, and check each command's `--help`; use a runtime that includes Spaces and the authoring commands. |

### Contributor checks

```sh
go test ./...
go test -race ./pkg/spaces ./pkg/shared
node --test pkg/spaces/recovery.test.mjs
go test ./pkg/markdown ./pkg/component ./pkg/spaces
```

Run Spaces and native Markdown tests with `go test ./...` from the repository root. The browser icon bundle uses Lucide 0.468.0 (ISC license). `pkg/spaces/web/lucide.js` retains its copyright notice. HyperBricks serves the bundle locally, without a CDN.

<a id="run-the-night-owl-cafe-example"></a>

## Run the Night Owl Café example

The [Swup navigation demo](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/navigation-demo-swup/README.md) includes an English café page and German and Dutch Spaces that inherit it.

Always run the command from the HyperBricks project root:

```sh
hyperbricks start -m navigation-demo-swup --port 8096
```

Open these pages:

| Language | Page | Definition |
| --- | --- | --- |
| English | [Night Owl Café](http://localhost:8096/night-owl-cafe) | `night_owl_cafe_page`, the source page |
| German | [Night Owl Café – Deutsch](http://localhost:8096/night-owl-cafe/de) | `night_owl_cafe_de`, a Space |
| Dutch | [Night Owl Café – Nederlands](http://localhost:8096/night-owl-cafe/nl) | `night_owl_cafe_nl`, a Space |

Use **English**, **Deutsch**, and **Nederlands** on the café page to switch languages. The translations cover the venue description, menu items, opening days, and visit labels. The surrounding neighbourhood guide stays English. Each translated Space sets its document language, title, and description metadata. Both share the source layout and assets.

### Edit a translation

Open [How it works](http://localhost:8096/how-it-works) and find **Translate the café with Spaces**. It links to [Manage Spaces](http://localhost:8096/__hyperbricks/spaces), each translation's editing form, and its page with contextual edit controls:

| Language | Editing form | Contextual page editor |
| --- | --- | --- |
| German | [Edit German Space](http://localhost:8096/__hyperbricks/spaces?name=night_owl_cafe_de) | [Edit German page](http://localhost:8096/night-owl-cafe/de?edit=true) |
| Dutch | [Edit Dutch Space](http://localhost:8096/__hyperbricks/spaces?name=night_owl_cafe_nl) | [Edit Dutch page](http://localhost:8096/night-owl-cafe/nl?edit=true) |

Change the introduction, description, opening hours, or menu text in the editing form. Choose **Save Changes**, then refresh the public page. The English source declares 15 editable fields for each translation. Saving one Space changes only its own YAML values. It leaves the other translation and English source unchanged.

Open a café page with `?edit=true` to use contextual editing. Hover over mapped content or choose a field. Follow **Edit … in Spaces** to open that field in the editing form. Choose **Exit** to return to the public page. The café template uses `data-hb-space-field` with catalog IDs such as `/body/values/content/values/intro`.

The module enables `dashboard.enabled` and Spaces writes for local development.
Set `HB_DEVELOPER_USER` and `HB_DEVELOPER_PASSWORD` before starting it, then use
that browser login for both Spaces and the
[Dashboard](http://localhost:8096/__hyperbricks/dashboard). The existing file watcher reloads
saved YAML; saving does not publish the site.

Editor links and contextual edit mode use full-page navigation. This lets the editor load and exit cleanly. Ordinary café language links use Swup to update content, title, and document language. Editing controls work only in development. Static exports contain the public pages.

### How the example is stored

The source `night_owl_cafe_page` in `hyperbricks/app.hyperbricks.yaml` owns the template and editable field declarations. Each Space overrides translated values under `body.values.content.values`. Its managed files are:

```text
modules/navigation-demo-swup/hyperbricks/spaces/night_owl_cafe_page/
  index.hyperbricks.yaml
  night_owl_cafe_de.hyperbricks.yaml
  night_owl_cafe_nl.hyperbricks.yaml
```

The source file imports the managed index, which imports both Spaces. Translations use a separate section so they do not add duplicate café entries to the guide's main menu.

To add a translation, list the sources and preview a new Space. Choose a unique name and route. The German and Dutch examples already exist:

```sh
hyperbricks space -m navigation-demo-swup --list
hyperbricks space -m navigation-demo-swup \
  --source night_owl_cafe_page \
  --name night_owl_cafe_fr \
  --title 'Night Owl Café – Français' \
  --route night-owl-cafe/fr \
  --dry-run
```

Remove `--dry-run` to create the Space with English source defaults. Translate its values and page labels. Set `htmltag: '<html lang="fr">'`, and set both `body.values.page_language` and `body.values.content.values.language` to `fr`.

Set `section: cafe_translations` to keep the page out of the main venue menu. Add its language link to `templates/place.html`. The CLI creates an inheriting page. It does not translate text automatically.

For another example with two page sources and English/German instances, see the
[localized Spaces pattern](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/localized-spaces.md).
