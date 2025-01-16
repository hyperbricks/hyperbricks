{{define "main"}}
# HyperBricks
**Version:** {{.version}}  
**Build time:** {{.buildtime}}

Go direct to:

- [HyperBricks type reference](#hyperbricks-type-reference)
- [HyperBricks examples](#hyperbricks-examples)

{{include "template_general.md"}}
---

## HyperBricks type reference
 {{range $category, $types := .data}}

### Category: **{{$category}}**
{{range $typeName, $fields := $types}}
- {{$typeName}}{{end}}
{{end}}


{{range $category, $types := .data}}
---
### Category: **{{$category}}**

{{range $typeName, $fields := $types}}
#### {{$typeName}}

**Type Description**
{{range $fields}}
{{if eq .Name "MetaDocDescription"}}
{{.Description}}

**Main Example**
````properties
{{.Example}}
````
{{end}}
{{end}}

---
**Properties**
{{range $fields}}
{{if ne .Name "MetaDocDescription"}}- {{.FieldLink}}{{end}}{{end}}
{{range $fields}}
{{if ne .Name "MetaDocDescription"}}
---

{{.FieldAnchor}}
#### {{.Mapstructure}}

**Description**  
{{.Description}}

**Example**
````properties
{{.Example}}
````
{{if .Result}}
**Expected Result**

````html
{{.Result}}
````
{{end}}
{{end}}
{{end}}
---
{{end}}
{{end}}
{{end}}