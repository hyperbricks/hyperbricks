# Markdown Plugin

This documentation page is itself rendered through `MarkdownPlugin@2.0.0`.

That means the docs browser is not using a special documentation system. It is using the same HyperBricks plugin mechanism that you can use in normal pages.

## What the plugin does

The Markdown plugin takes Markdown text and turns it into HTML.

In this module it is used for two things:

- rendering the docs pages under `/docs`
- proving that a simple plugin can be useful for content-focused pages too

## Basic shape

```yaml
my_doc:
  - type: plugin
  - plugin: MarkdownPlugin@2.0.0
  - data:
      class: pattern-docs-markdown
      content:
        file:
          base: module
          path: docs/pages/markdown-plugin.md
```

## Input fields

- `plugin` The plugin name. In this case: `MarkdownPlugin@2.0.0`
- `data.class` Optional CSS class added around the rendered HTML
- `data.content` The Markdown source string

## Why this docs browser uses it

The documentation menu on `/docs` loads full HTML pages, but the main article body is rendered from Markdown files through the plugin.

That gives a useful split:

- Markdown files stay easy to write and maintain
- readers still get styled HTML pages
- the docs shell can add navigation, layout, and HTMX behavior around the rendered content

## Native Markdown alternative

This module keeps `MarkdownPlugin@2.0.0` to demonstrate plugin rendering. For ordinary Markdown content, the current runtime also has a native `markdown` component that requires no plugin installation:

```yaml
my_doc:
  - type: markdown
  - file: guides/introduction.md
```

Put the document at `resources/guides/introduction.md`: a native Markdown `file` is relative to the module's **resources directory**. The plugin example above instead reads the existing article relative to the **module directory** through a file resolver.

Place the native component inside a page or fragment to give it a URL. It skips raw HTML and sanitizes the generated HTML. See `docs/MARKDOWN.md` in the repository root for its file and editing options.

## Notes

- the plugin converts Markdown to HTML without the native component's sanitization; use trusted project content
- the shell around it is still owned by normal HyperBricks templates
- for this module, the CSS class `pattern-docs-markdown` adds the base documentation styling
