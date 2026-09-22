# Template Config Plugin

Use a `plugin` component to prepare values, then return a `<TEMPLATE>` configuration for HyperBricks to render. The plugin owns the transformation; the template owns the HTML.

## Files

- Plugin: `plugins/template-config-demo/2.0.0/template_config_demo_plugin.go`
- Template: `templates/demo.html`
- Panel configuration: `hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml`

The panel appears at `/status-demo/plugin` and `/fragments/status-demo-plugin`.

## Input

```yaml
template_config_demo:
  - type: plugin
  - plugin: TemplateConfigDemoPlugin__hyperbricks-patterns-yaml@2.0.0
  - data:
      template:
        file: demo.html
      content: |
        # Hello

        This is **rendered Markdown**.
      class: template_config_demo-content
```

HyperBricks resolves `data.template.file` to a template key before the plugin receives it. The plugin requires that key, normalizes `data.content`, converts Markdown to HTML, and prepares `class` and `html` values.

## Output

The current plugin returns a template configuration directly:

```json
{
  "@type": "<TEMPLATE>",
  "template": "demo.html",
  "values": {
    "class": "template_config_demo-content",
    "html": "<h1>Hello</h1><p>This is <strong>rendered Markdown</strong>.</p>"
  }
}
```

`template` contains the resolved template key; `demo.html` illustrates that value. This JSON is runtime configuration returned by Go code. In YAML source, declare components with `type: template`.

The template supplies the markup:

```html
<section class="{{ .class }}">
  <div class="cute-body">
    {{ .html | safe }}
  </div>
</section>
```

The demo uses `safe` to insert the generated HTML. Its Markdown conversion does not sanitize the result, so keep this input trusted. The native `markdown` component provides sanitized Markdown rendering without a plugin; see `docs/MARKDOWN.md` in the repository root.

## When to use it

Use this handoff when a plugin needs to validate, normalize, or calculate values before a template renders them. Several plugin instances can share the same template.

If configured values are enough, use `template` directly. If you only need Markdown rendering, use the native `markdown` component. A `<TREE>` wrapper is optional for composing several returned components; this plugin does not use one.

See `docs/PLUGINS.md` in the repository root for building and enabling plugins.
