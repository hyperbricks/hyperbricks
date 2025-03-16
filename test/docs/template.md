{{define "main"}}

**Licence:** MIT  
**Version:** {{.version}}  
**Build time:** {{.buildtime}}


{{include "template_note.md"}}

Go direct to:
- [Quickstart](#quickstart)
- [Installation Instructions for HyperBricks](#installation-instructions-for-hyperbricks)
- [Defining Hypermedia Documents and Fragments](#defining-hypermedia-documents-and-fragments)
- [Adding Properties to Configurations](#adding-properties-to-configurations)
- [Rendering Order and Property Rules](#rendering-order-and-property-rules)
- [Example Configurations](#example-configurations)
  - [Hypermedia Example](#hypermedia-example)
  - [Fragment Example with HTMX Trigger](#fragment-example-with-htmx-trigger)
- [Object Inheritance and Reusability](#object-inheritance-and-reusability)
- [Importing Predefined HyperScripts](#importing-predefined-hyperscripts)
- [HyperBricks type reference](#hyperbricks-type-reference)
- [API Serverside Render](#api-serverside-render)

{{include "template_general.md"}}
{{include "template_general_examples.md"}}
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

{{ if eq $typeName "<FRAGMENT>" }}
   {{include "template_api_fragment_render.md"}}
{{end}}

<h3><a id="{{$typeName}}">{{$typeName}}</a></h3>

**Type Description**

{{range $fields}}
{{if eq .Mapstructure "@doc"}}

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

{{end}}
{{end}}
{{end}}