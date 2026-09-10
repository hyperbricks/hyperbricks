{{define "main"}}**Licence:** MIT
**Version:** {{.HyperBricksVersion}}
{{if .BuildTime}}
**Build time:** {{.BuildTime}}
{{end}}

# HyperBricks Component Reference

This reference is generated from the runtime schema and YAML documentation fixtures. It is intentionally compact: field tables come from Go struct tags, while examples come from curated executable YAML fixtures.

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
