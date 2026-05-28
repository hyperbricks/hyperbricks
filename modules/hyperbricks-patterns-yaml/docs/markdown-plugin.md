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
          path: docs/markdown-plugin.md
```

## Input fields

- `plugin`
  The plugin name. In this case: `MarkdownPlugin@2.0.0`
- `data.class`
  Optional CSS class added around the rendered HTML
- `data.content`
  The Markdown source string

## Why this docs browser uses it

The documentation menu on `/docs` loads full HTML pages, but the main article body is rendered from Markdown files through the plugin.

That gives a useful split:

- Markdown files stay easy to write and maintain
- readers still get styled HTML pages
- the docs shell can add navigation, layout, and HTMX behavior around the rendered content

## Notes

- the plugin converts Markdown to HTML
- the shell around it is still owned by normal HyperBricks templates
- for this module, the CSS class `pattern-docs-markdown` adds the base documentation styling
