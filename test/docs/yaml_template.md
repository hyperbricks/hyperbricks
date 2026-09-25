{{define "main"}}**Licence:** MIT
**Version:** {{.HyperBricksVersion}}
{{if .BuildTime}}
**Build time:** {{.BuildTime}}
{{end}}

# HyperBricks Component Reference

Use this reference to check component fields, types, and required values. HyperBricks generates the field tables from Go struct tags and the examples from executable YAML fixtures in the runtime documentation tests.

Regenerate this reference and the root README with:

```bash
bash scripts/build_docs.sh
```

Schema version: {{.SchemaVersion}}

{{range .Categories}}## {{.Name}}

{{range .Types}}### `{{.Token}}`

{{if .Aliases}}Aliases: {{range $index, $alias := .Aliases}}{{if $index}}, {{end}}`{{$alias}}`{{end}}

{{end}}
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
