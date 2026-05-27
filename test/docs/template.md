{{define "main"}}**Licence:** MIT  
**Version:** {{.version}}  
{{if and .buildtime (ne .buildtime "undefined")}}
**Build time:** {{.buildtime}}
{{end}}

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

## HyperBricks type reference

{{range $category, $types := .data}}
# Category: **{{$category}}**

{{range $typeName, $fields := $types}}
{{if eq $typeName "<FRAGMENT>"}}
{{include "template_api_fragment_render.md"}}
{{end}}

## {{$typeName}}

**Type Description**
{{if not (hasTypeDoc $fields)}}

{{trimText (typeDescription $fields)}}
{{end}}
{{range $fields}}{{if eq .Mapstructure "@doc"}}

{{trimText .Description}}

**Main Example**
````properties
{{trimHTML .Example}}
````

{{if .Result}}
**Expected Result**
````html
{{trimHTML .Result}}
````
{{end}}

{{if .MoreDetails}}
**More**
{{trimText .MoreDetails}}
{{end}}

{{end}}{{end}}

**Properties**
{{range $fields}}{{if ne .Mapstructure "@doc"}}

### {{.Mapstructure}}

**Description**  
{{trimText .Description}}

**Example**
````properties
{{trimHTML .Example}}
````
{{if .Result}}
**Expected Result**

````html
{{trimHTML .Result}}
````


{{end}}

{{if .MoreDetails}}
{{trimText .MoreDetails}}
{{end}}

{{end}}{{end}}

{{end}}
{{end}}
{{end}}
