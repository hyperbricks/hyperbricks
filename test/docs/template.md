{{define "main"}}

![HyperBricks Logo](https://raw.githubusercontent.com/hyperbricks/hyperbricks/refs/heads/main/docs/hyperbricks_logo_ibm.png)

**Licence:** MIT  
**Version:** {{.version}}  
**Build time:** {{.buildtime}}

{{include "template_note.md"}}

Go direct to:

- [Installation](#installation)
- [Defining Hypermedia Documents and Fragments](#defining-hypermedia-documents-and-fragments)
- [Adding Properties to Configurations](#adding-properties-to-configurations)
- [Rendering Order and Property Rules](#rendering-order-and-property-rules)
- [Example Configurations](#example-configurations)
  - [Hypermedia Example](#hypermedia-example)
  - [Fragment Example with HTMX Trigger](#fragment-example-with-htmx-trigger)
- [Object Inheritance and Reusability](#object-inheritance-and-reusability)
- [Importing Predefined HyperScripts](#importing-predefined-hyperscripts)
- [HyperBricks type reference](#hyperbricks-type-reference)

{{include "template_general.md"}}
{{include "template_install.md"}}

<h1><a id="hyperbricks-type-reference">HyperBricks type reference</a></h1>

### Component categories:
 {{range $category, $types := .data}}

### **{{$category}}**
{{range $typeName, $fields := $types}}
- [{{$typeName}}](#{{html $typeName}}) {{end}}
{{end}}


{{range $category, $types := .data}}

### Category: **{{$category}}**

{{range $typeName, $fields := $types}}
<h3><a id="{{$typeName}}">{{$typeName}}</a></h3>

**Type Description**

{{range $fields}}
{{if eq .Mapstructure "@doc"}}

{{.Description}}

**Main Example**
````yaml
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


**Properties**
{{range $fields}}
{{if ne .Mapstructure "@doc"}}- {{.FieldLink}}{{end}}{{end}}
{{range $fields}}

{{if ne .Mapstructure "@doc"}}


{{.FieldAnchor}}
#### {{.Mapstructure}}

**Description**  
{{.Description}}

**Example**
````yaml
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

{{end}}
{{end}}
{{end}}