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
- [{{$typeName}}](#{{html $typeName}}) {{end}}
{{end}}


{{range $category, $types := .data}}
---
### Category: **{{$category}}**

{{range $typeName, $fields := $types}}
<a id="{{html $typeName}}">{{html $typeName}}</a>

**Type Description**
{{range $fields}}
{{if eq .Name "MetaDocDescription"}}
{{.Description}}

**Main Example**
````properties
{{.Example}}
````

{{if .Result}}
**Expected Result**

````html
{{.Result}}
````


{{end}}

**more**
{{.MoreDetails}}

{{end}}
{{end}}

---
**Properties**
{{range $fields}}
{{if ne .Mapstructure "@doc"}}- {{.FieldLink}}{{end}}{{end}}
{{range $fields}}

{{if ne .Mapstructure "@doc"}}
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

{{.MoreDetails}}

{{end}}
{{end}}
---
{{end}}
{{end}}
{{end}}