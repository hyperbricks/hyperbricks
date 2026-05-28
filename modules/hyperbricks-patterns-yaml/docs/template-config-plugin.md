# Template Config Plugin

## Summary

Use a `<PLUGIN>` brick as a transformer that accepts structured config, computes template-ready values, and returns a pipeline-native `<TREE>` containing a `<TEMPLATE>` brick.

This keeps transformation logic in the plugin and presentation markup in the template.

## Recommended Name

`Template Config Plugin`

Reason:

- `plugin renders html` is too broad
- this pattern is specifically about config-driven transformation plus template handoff
- the plugin does not need to own the final markup

## Problem

Sometimes a page needs logic that a plain `<TEMPLATE>` brick should not own.

Examples:

- normalize or preprocess content
- transform Markdown into HTML
- compute a values map for a template
- apply server-side shaping before presentation

Without a pattern, that logic tends to drift into inline HTML, duplicated template data shaping, or plugins that hardcode the full markup in Go.

## Pattern Rule

Prefer this shape:

- plugin receives config under `data`
- plugin computes values
- plugin returns a `<TREE>` with a `<TEMPLATE>` node
- template owns the final markup

Treat direct raw HTML return as a fallback or demo path, not as the preferred pattern.

## Contract

### Input

The plugin accepts a standard `<PLUGIN>` brick plus a structured `data` object.

Current example shape:

```hyperbricks
template_config_demo = <PLUGIN>
template_config_demo.plugin = TemplateConfigDemoPlugin__hyperbricks-patterns-yaml@2.0.0
template_config_demo.data.template = {{TEMPLATE:demo.html}}
template_config_demo.data.content = "# Hello\n\nThis is **cute rendered markdown**."
template_config_demo.data.class = template_config_demo-content
```

### Plugin responsibility

The plugin should:

- decode the `data` object
- validate or normalize its inputs
- perform transformation logic
- build a `values` map for the template
- return pipeline-native config instead of final string HTML when template rendering is desired

### Output

Preferred output shape:

```json
{
  "@type": "<TREE>",
  "10": {
    "@type": "<TEMPLATE>",
    "template": "{{TEMPLATE:demo.html}}",
    "values": {
      "class": "template_config_demo-content",
      "html": "<p>...</p>"
    }
  }
}
```

The exact value stored in `template` may already be the resolved template marker payload as provided by HyperBricks.

## Current Example

Files:

- Plugin: `plugins/template-config-demo/2.0.0/template_config_demo_plugin.go`
- Template: `templates/demo.html`
- HyperBricks example: `hyperbricks/hello-world.hyperbricks`

What the current example does:

1. Reads Markdown-like content from `data.content`
2. Normalizes the string input
3. Converts Markdown to HTML
4. Builds a values map with `class` and `html`
5. Returns a synthetic `<TREE>` containing a `<TEMPLATE>` brick

Template:

```html
<section class="{{ .class }}">
  <div class="cute-body">
    {{ .html | safe }}
  </div>
</section>
```

## Use When

Use this pattern when:

- the plugin needs server-side transformation before presentation
- the output still fits naturally into template rendering
- you want to keep HTML structure in template files
- multiple plugin instances can share the same template contract

## Avoid When

Avoid this pattern when:

- a plain `<TEMPLATE>` brick with `values` is already enough
- the plugin really must emit final HTML directly
- the output is not presentational and should remain data-oriented
- the logic belongs in a dedicated HyperBricks type instead of a custom plugin

## Design Guidance

Keep these boundaries strict:

- plugin owns transformation
- template owns markup
- HyperBricks owns final rendering

That separation is the main value of the pattern.

## Suggested Naming Rule For Future Patterns

Prefer names that describe the ownership model, not just the implementation detail.

Good:

- `Template Config Plugin`
- `Plugin To Template Handoff`
- `Markdown To Template Plugin`

Weak:

- `plugin renders html`
- `template plugin`
- `html plugin demo`
