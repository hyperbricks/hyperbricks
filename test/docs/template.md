{{define "main"}}**Licence:** MIT  
**Version:** {{.version}}  
**Build time:** {{.buildtime}}

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

## HyperBricks type reference

{{range $category, $types := .data}}

# Category: **{{$category}}**

{{range $typeName, $fields := $types}}

{{ if eq $typeName "<FRAGMENT>" }}
   {{include "template_api_fragment_render.md"}}
{{end}}

## {{$typeName}}

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

{{if ne .Mapstructure "@doc"}}

### {{.Mapstructure}}

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