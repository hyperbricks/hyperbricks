# Authoring pages, fragments, templates, and assets

Use this reference for a small working composition or when adapting a lesson
into another project. Core manuals: `docs/YAML_USAGE.md`, `docs/ROUTING.md`,
`docs/ESBUILD.md`, and `docs/REFERENCE.md`.

## One view, a full page, and a fragment

Add this as `hyperbricks/help.hyperbricks.yaml` inside a CLI-initialized module.
Named reusable views have no route. The two route owners reuse the same view:

```yaml
help_content:
  - type: template
  - inline: |
      <section>
        <h1>{{ .heading }}</h1>
        <p>{{ .message }}</p>
      </section>
  - values:
      heading: Project help
      message: Edit this message and reload the page.

help_page:
  - type: hypermedia
  - route: help
  - title: Project help
  - head:
      - type: head
      - metadata:
          - type: html
          - value: '<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">'
  - body:
      - type: template
      - inline: '<main id="content">{{ .content }}</main>'
      - values:
          content:
            - inherit: help_content

help_fragment:
  - type: fragment
  - route: fragments/help
  - response:
      hx_push_url: /help
  - body:
      - inherit: help_content
```

After starting the module, `/help` returns the page with `#content`;
`/fragments/help` returns just the section. A component supplied under a template
value renders into that slot: `.content` above is rendered HTML. Ordinary strings
remain escaped. In a growing app, put the shell in a shared template and inherit
it across full pages, as the dashboard does.

Keep UTF-8 charset and viewport metadata in the shared head. The default page
head does not supply them; the charset prevents non-ASCII text from displaying
incorrectly, and the viewport lets the layout adapt to mobile screens.

In the shared navigation, enhance a normal link:

```html
<a href="/help"
   hx-get="/fragments/help"
   hx-target="#content"
   hx-swap="innerHTML">Help</a>
```

Load the project's pinned HTMX browser dependency once in the full page's head.
The YAML recipe itself uses no browser dependency; its optional `hx-*` enhancement
requires HTMX. The integrated dashboard README supplies its reproducible setup.
The source page must contain `#content`; the fragment replaces that element's
contents. `HX-Push-Url: /help` makes the full-page URL the history entry. Do not
push the fragment URL. Verify reload, Back, and Forward with the project's HTMX
configuration. Without JavaScript, `href` still opens the full page.

For a refresh button that stays on the same page, use a separate fragment
without `hx_push_url`. Give it an existing target and choose `innerHTML` for
contents or `outerHTML` when the response includes the replacement target itself.
Avoid nested `<main>` or duplicate target IDs after a swap.

## Move reusable pieces without losing them

As source grows, put shared definitions in `hyperbricks/partials/` and import
them from each entry file that needs them:

```yaml
imports:
  - partials/site.hyperbricks.yaml

help_page:
  - inherit: site_page
  - route: help
  - title: Project help
  - body:
      - values:
          content:
            - inherit: help_content
```

This override assumes `site_page.body` is the shell template and `help_content`
is loaded by the shared source. Preserve the existing child name when overriding.
`inherit` makes a deep copy; dotted paths can select a nested component.

Imports are relative to the importing source file. Only root source files are
scanned automatically; a file moved into a subdirectory must be imported. Keep
component order in sequences and application data in normal maps/lists. The same
ordered-object rules apply to nested template component values.

## Configuration and navigation data

Use the project's existing custom package namespace for data shared by several
views. A small example uses:

```yaml
myconf:
  site:
    name: Project desk
    navigation:
      "01_overview":
        label: Overview
        page: /
        fragment: /fragments/overview
      "02_help":
        label: Help
        page: /help
        fragment: /fragments/help
```

Pass that data to a `template` component:

```yaml
navigation:
  - type: template
  - template:
      file: navigation.html
  - values:
      links:
        config: myconf.site.navigation
```

In `templates/navigation.html`, render a link for each item:

```html
<nav aria-label="Main">
  {{ range .links }}
    <a href="{{ .page }}" hx-get="{{ .fragment }}"
       hx-target="#content" hx-swap="innerHTML">{{ .label }}</a>
  {{ end }}
</nav>
```

Define both routes for each entry. Navigation data describes existing routes;
it does not create them. This runtime's template renderer does not preserve a
list of objects passed directly as a `values` entry, so use a keyed map for this
record collection. Go templates visit these string map keys in sorted order;
the key prefixes above make the intended order explicit. This is a current
renderer limitation, not a rule that all application data must be a map. The `menu` component is another supported approach for
route/section-driven menus. Keep whichever owner the application already uses.

## Template values and Sprig

`template.file` is relative to the configured templates directory. For short
examples use `inline`; use YAML `|` for multiline template source. Neither YAML
resolvers nor inheritance evaluate Go template expressions.

```yaml
summary:
  - type: template
  - inline: |
      <h2>{{ .heading | trim | title }}</h2>
      <p>{{ .description | default "Details coming soon." }}</p>
      <p>{{ .tags | sortAlpha | join ", " }}</p>
  - values:
      heading: "  project summary  "
      description: ""
      tags: [yaml, htmx, go]
```

This displays `Project Summary`, the default description, and `go, htmx, yaml`.
A pipeline passes its value as the final argument to the next function. Sprig
`default` treats `0` and `false` as empty too. Use explicit checks when either is
meaningful. `safe` bypasses HTML escaping and should only receive trusted markup.

Request input for templates is separate from configured values:

```yaml
search:
  - type: template
  - querykeys: [q]
  - inline: '<p>Search: {{ .Params.q }}</p>'
```

Place this under a route with `nocache: true` when the response depends on `q`.
This controls HyperBricks' internal response cache; it does not promise an HTTP
`Cache-Control` header on every route type. Inspect actual headers when browser
or proxy caching matters.
Omitting `querykeys` uses the default list (`id`, `name`, `order`); `[]` allows
none. One query value is a string; repeated values are a list. Validate according
to the operation; `queryparams` does not populate template `.Params`.

## Resolvers and quoting

| Task | YAML value |
| --- | --- |
| File-level variable | `{var: page.title}` |
| App configuration | `{config: myconf.site.name}` |
| Environment with fallback | `{env: {name: SITE_NAME, default: Project desk}}` |
| Filename under resources | `{path: {base: resources, path: js/main.js}}` |
| Contents of a resource file | `{file: {base: resources, path: copy/intro.txt}}` |
| Preloaded Go template | `template: {file: cards/project.html}` |

Flow mappings (`{base: resources, path: js/main.js}`) and equivalent indented
block mappings mean the same thing. Use whichever is clearest. The resolver
name and its owning field still matter: esbuild needs filenames with `path`;
Goja `script` needs contents with `file`.

Quote identifiers such as `"0012"`, version labels, and strings resembling
booleans/nulls. Single quotes work well for one-line HTML with double-quoted
attributes. Double quotes interpret escapes. Use `|` for preserved line breaks,
`>` for folded prose, and normal YAML booleans/numbers for actual typed values.

## Native asset bundling

For a compatible runtime, use native `esbuild` rather than the deprecated
`EsbuildPlugin@2.0.0`. It compiles on first component render and returns the public
asset URL. Put this declaration in shared YAML and inherit it into the page head:

```yaml
browser_script:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/app.js}
  - cache: true
  - fingerprint: true
  - enclose: '<script src="|" defer></script>'

site_head:
  - type: head
  - application_script:
      - inherit: browser_script
```

Create `resources/js/main.js`, then inherit `site_head` as the full page's `head`
child. The returned URL already begins `/static/`; do not prefix another slash
or hardcode a fingerprinted filename. Output must remain inside configured
static. `cache: true` reuses builds independently of page response caching.

Use a second esbuild component with a `.css` entry, `.css` output, and
`enclose: '<link rel="stylesheet" href="|">'` for ordinary CSS. A JS import of
CSS can emit a sibling CSS file; it does not automatically add its HTML link.
Bare package imports require installed dependencies. Native esbuild does not run
npm installation or TypeScript type checking, and ordinary CSS bundling does
not replace Tailwind/Sass processing.

For development, include `resources` in `hyperbricks.development.watch_dirs`.
Avoid attaching per-element listeners only at first page load: use event
delegation or the existing HTMX lifecycle hooks so behavior survives swaps. Load
shared scripts once in the shell; repeated fragments should supply content.
