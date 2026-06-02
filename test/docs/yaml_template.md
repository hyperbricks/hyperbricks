{{define "main"}}# HyperBricks Component Reference

This reference is generated from the runtime schema and YAML documentation
fixtures. It is intentionally compact: field tables come from Go struct tags,
while examples come from curated executable YAML fixtures.

Schema version: {{.Version}}

{{range .Categories}}## {{.Name}}

{{range .Types}}### `{{.Token}}`

{{mdText .Description}}

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
{{range .Fields}}| `{{.Path}}` | `{{.Kind}}` | {{boolText .Required}} | {{mdCell .Description}} |
{{end}}
#### Example

Fixture: `{{.Example.Name}}`

{{if .Example.Explainer}}{{mdBlock .Example.Explainer}}

{{end}}
```yaml
{{.Example.Source}}
```

{{if .Example.ExpectedOutput}}Expected output:

```html
{{.Example.ExpectedOutput}}
```
{{end}}

{{end}}
{{end}}
{{end}}
