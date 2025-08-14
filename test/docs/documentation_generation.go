// cmd/docgen/main.go
// Generate component/composite reference in your requested format.
//
// Usage:
//
//	go run ./cmd/docgen -examples ./hyperbricks-test-files -out ./REFERENCE.generated.md
//
// Notes:
// - <TYPE> tokens (e.g. <HTML>, <FRAGMENT>) render as inline code so Markdown won't treat them as HTML.
// - If a general example isn't found, you get a TODO code block.
// - If a property example isn't found, you get "leaf = test" as a one-liner.
//
// Requires your repo packages on GOPATH/MODULE: github.com/hyperbricks/hyperbricks/...
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

// Flags
var examplesDirFlag = flag.String("examples", "hyperbricks-test-files", "directory containing *.hyperbricks example files")
var outPathFlag = flag.String("out", "REFERENCE.generated.md", "output markdown path")

// Types we use to drive the doc
type DocumentationTypeStructII struct {
	Name            string            // logical name used for example file fallbacks (lowercased)
	TypeDescription string            // fallback description if @doc is missing
	ConfigType      string            // the visible <TYPE> token
	ConfigCategory  string            // "composite" => Composites; anything else => Components
	Embedded        map[string]string // map[StructName]prefix, e.g. {"HxResponse":"response"}
	ExcludeFields   []string          // mapstructure keys to exclude (e.g. "attributes", "is_static")
	Config          any               // zero value of the config struct
}

type parsedContent struct {
	HBConfig   string
	Result     string
	Scope      string
	Explainer  string
	More       string
	hasExample bool
}

type propDoc struct {
	Key         string
	Description string
	Example     string
}

func main() {
	flag.Parse()

	types := typesToDocument()

	var components []DocumentationTypeStructII
	var composites []DocumentationTypeStructII
	for _, t := range types {
		if strings.EqualFold(t.ConfigCategory, "composite") {
			composites = append(composites, t)
		} else {
			components = append(components, t)
		}
	}

	var b strings.Builder

	// Components
	b.WriteString("## Category: Components\n\n")
	for _, t := range components {
		writeTypeDoc(&b, t)
		b.WriteString("\n")
	}

	// Composites
	b.WriteString("## Category: Composites\n\n")
	for _, t := range composites {
		writeTypeDoc(&b, t)
		b.WriteString("\n")
	}

	if err := os.WriteFile(*outPathFlag, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s\n", *outPathFlag)
}

// ----------------- Types to document (edit here) -----------------

func typesToDocument() []DocumentationTypeStructII {
	return []DocumentationTypeStructII{
		// Composites
		{
			Name:            "Fragment",
			TypeDescription: "A <FRAGMENT> dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience.",
			Embedded:        map[string]string{"HxResponse": "response"},
			ConfigType:      "<FRAGMENT>",
			ConfigCategory:  "composite",
			Config:          composite.FragmentConfig{},
		},
		{
			Name:            "Hypermedia",
			TypeDescription: "HYPERMEDIA description",
			Embedded:        map[string]string{},
			ConfigType:      "<HYPERMEDIA>",
			ConfigCategory:  "composite",
			Config:          composite.HyperMediaConfig{},
		},
		{
			Name:            "Head",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<HEAD>",
			ConfigCategory:  "composite",
			Config:          composite.HeadConfig{},
		},
		{
			Name:            "Template",
			TypeDescription: "TEMPLATE description",
			Embedded:        map[string]string{},
			ConfigType:      "<TEMPLATE>",
			ConfigCategory:  "composite",
			Config:          composite.TemplateConfig{},
		},
		{
			Name:            "Tree",
			TypeDescription: "Tree composite element can render types in alphanumeric order. Tree elements can have nested types.",
			Embedded:        map[string]string{},
			ConfigType:      "<TREE>",
			ConfigCategory:  "composite",
			Config:          composite.TreeConfig{},
		},
		{
			Name:            "ApiFragmentRender",
			TypeDescription: "A <FRAGMENT> dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience.",
			Embedded:        map[string]string{"HxResponse": "response"},
			ConfigType:      "<API_FRAGMENT_RENDER>",
			ConfigCategory:  "composite",
			Config:          composite.ApiFragmentRenderConfig{},
		},

		// Components (resources/data/menu show under Components)
		{
			Name:            "Html",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ExcludeFields:   []string{"attributes"},
			ConfigType:      "<HTML>",
			ConfigCategory:  "component",
			Config:          component.HTMLConfig{},
		},
		{
			Name:            "Text",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ExcludeFields:   []string{"attributes"},
			ConfigType:      "<TEXT>",
			ConfigCategory:  "component",
			Config:          component.TextConfig{},
		},
		{
			Name:            "Css",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<CSS>",
			ConfigCategory:  "resources",
			Config:          component.CssConfig{},
		},
		{
			Name:            "Javascript",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<JS>",
			ConfigCategory:  "resources",
			Config:          component.JavaScriptConfig{},
		},
		{
			Name:            "Image",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<IMAGE>",
			ConfigCategory:  "resources",
			Config:          component.SingleImageConfig{},
		},
		{
			Name:            "Images",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ExcludeFields:   []string{"is_static"},
			ConfigType:      "<IMAGES>",
			ConfigCategory:  "resources",
			Config:          component.MultipleImagesConfig{},
		},
		{
			Name:            "Json",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<JSON>",
			ConfigCategory:  "data",
			Config:          component.LocalJSONConfig{},
		},
		{
			Name:            "Api_Render",
			TypeDescription: "<API_RENDER> description",
			Embedded:        map[string]string{},
			ExcludeFields:   []string{"attributes"},
			ConfigType:      "<API_RENDER>",
			ConfigCategory:  "data",
			Config:          component.APIConfig{},
		},
		{
			Name:            "Menu",
			TypeDescription: "MENU description",
			Embedded:        map[string]string{},
			ExcludeFields:   []string{"attributes"},
			ConfigType:      "<MENU>",
			ConfigCategory:  "menu",
			Config:          component.MenuConfig{},
		},
	}
}

// ----------------- Doc rendering -----------------

func writeTypeDoc(b *strings.Builder, cfg DocumentationTypeStructII) {
	desc := findTypeDescription(cfg)
	gen := findGeneralExample(cfg)
	props := collectProps(cfg)

	// fmt.Printf(" ./test/docs/%s ", full)
	fmt.Printf(" %s ", desc)

	// Header
	b.WriteString(fmt.Sprintf("### %s\n", codeifyTags(cfg.ConfigType)))
	b.WriteString(fmt.Sprintf("%s\n\n", escCell(desc)))

	// Table
	b.WriteString("Property | Description\n")
	b.WriteString("---|---\n")
	sort.Slice(props, func(i, j int) bool { return props[i].Key < props[j].Key })
	for _, p := range props {
		dd := p.Description
		if strings.TrimSpace(dd) == "" {
			dd = "-"
		}

		b.WriteString(fmt.Sprintf("%s | %s\n", escCell(p.Key), escCell(dd)))
	}

	if gen.hasExample {
		b.WriteString("\n#### General example:\n")
		b.WriteString("````hyperbricks\n")
		b.WriteString(strings.TrimRight(gen.HBConfig, "\n"))
		b.WriteString("\n````\n\n")
		b.WriteString("\n#### Result:\n")
		b.WriteString("````html\n")
		b.WriteString(strings.TrimRight(gen.Result, "\n"))
		b.WriteString("\n````\n\n")

		b.WriteString("\n")
		b.WriteString(strings.TrimRight(gen.Explainer, "\n"))
		b.WriteString("\n")

		b.WriteString("\n")
		b.WriteString(strings.TrimRight(gen.More, "\n"))
		b.WriteString("\n")

	}
}

func findTypeDescription(cfg DocumentationTypeStructII) string {
	// Prefer @doc field description, else TypeDescription
	rv := reflect.ValueOf(cfg.Config)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Tag.Get("mapstructure") == "@doc" {

			if d := strings.TrimSpace(f.Tag.Get("description")); d != "" {
				return d
			}
		}
		if f.Tag.Get("mapstructure") == ",squash" {
			sub := findTypeDescription(DocumentationTypeStructII{Config: zeroValue(f.Type).Interface()})
			if sub != "" {
				return sub
			}
		}
	}
	return strings.TrimSpace(cfg.TypeDescription)
}

func findGeneralExample(cfg DocumentationTypeStructII) parsedContent {

	// Use @doc example file if present; else fall back to <lower(name)>-@doc.hyperbricks
	rv := reflect.ValueOf(cfg.Config)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Tag.Get("mapstructure") == "@doc" {
			return readExampleOrEmpty(f.Tag.Get("example"))
		}
		if f.Tag.Get("mapstructure") == ",squash" {
			pc := findGeneralExample(DocumentationTypeStructII{Config: zeroValue(f.Type).Interface()})
			if pc.hasExample {
				return pc
			}
		}
	}
	file := fmt.Sprintf("%s-@doc.hyperbricks", strings.ToLower(cfg.Name))
	return readExampleOrEmpty("{!{" + file + "}}")
}

// Collect properties from struct, including:
// - normal fields with mapstructure:"key"
// - embedded/squashed fields (mapstructure:",squash")
// - nested groups (struct fields with a concrete map key; we prefix with that key + ".")
// - extra embedded groups listed in cfg.Embedded (e.g. HxResponse -> response.*)
func collectProps(cfg DocumentationTypeStructII) []propDoc {
	props := make([]propDoc, 0)
	seen := map[string]bool{}

	// main struct
	props = append(props, walkStruct(reflect.ValueOf(cfg.Config), "", cfg, seen)...)

	// user-declared embedded groups
	rv := reflect.ValueOf(cfg.Config)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	for embeddedName, prefix := range cfg.Embedded {
		field := findFieldByNameII(rv, embeddedName)
		if field.IsValid() {
			props = append(props, walkStruct(field, prefix+".", cfg, seen)...)
		}
	}
	return props
}

func IsExcludedFieldGen(tag string, excludeFields []string) bool {
	for _, field := range excludeFields {
		if field == tag {
			return true
		}
	}
	return false
}

func walkStruct(val reflect.Value, prefix string, cfg DocumentationTypeStructII, seen map[string]bool) []propDoc {
	out := []propDoc{}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if !val.IsValid() || val.Kind() != reflect.Struct {
		return out
	}
	rt := val.Type()

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)

		// exclude:"true" fields are not documented
		if f.Tag.Get("exclude") == "true" {
			continue
		}

		ms := f.Tag.Get("mapstructure")
		if ms == "" {
			continue // not exposed
		}
		if IsExcludedFieldGen(ms, cfg.ExcludeFields) {
			continue
		}
		// skip collectors
		if strings.Contains(ms, ",remain") {
			continue
		}
		// squash embedded fields inline
		if ms == ",squash" {
			out = append(out, walkStruct(zeroValue(f.Type), prefix, cfg, seen)...)
			continue
		}
		// nested group key -> struct or *struct
		if !strings.HasPrefix(ms, "@") {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				out = append(out, walkStruct(zeroValue(ft), prefix+ms+".", cfg, seen)...)
				continue
			}
		}
		// skip meta fields like @doc
		if strings.HasPrefix(ms, "@") {
			continue
		}

		key := prefix + ms
		if seen[key] {
			continue
		}
		seen[key] = true

		desc := strings.TrimSpace(f.Tag.Get("description"))

		// one-liner example: prefer field example, else fallback to <lower(name)>-<key>.hyperbricks
		example := ""
		if ex := strings.TrimSpace(f.Tag.Get("example")); ex != "" {
			if pc := readExampleOrEmpty(ex); pc.hasExample {
				example = firstLineMentioning(pc.HBConfig, leaf(key))
			}
		}
		if example == "" {
			file := fmt.Sprintf("%s-%s.hyperbricks", strings.ToLower(cfg.Name), ms)
			pc := readExampleOrEmpty("{!{" + file + "}}")
			if pc.hasExample {
				example = firstLineMentioning(pc.HBConfig, leaf(key))
			}
		}
		if example == "" {
			example = fmt.Sprintf("%s = test", leaf(key))
		}

		out = append(out, propDoc{Key: key, Description: desc, Example: example})
	}
	return out
}

// ----------------- Helpers -----------------

func zeroValue(t reflect.Type) reflect.Value {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return reflect.New(t).Elem()
}

func leaf(key string) string {
	parts := strings.Split(key, ".")
	return parts[len(parts)-1]
}

// Keep ALL-CAPS <TYPE> tokens visible in markdown without turning into HTML.
// Matches <HTML>, <JS>, <FRAGMENT>, <API_RENDER> (not <div>).
var angleTag = regexp.MustCompile(`<[A-Z_][A-Z0-9_]*>`)

func codeifyTags(s string) string {
	return angleTag.ReplaceAllStringFunc(s, func(m string) string { return "`" + m + "`" })
}

// Escape Markdown table cells and protect <TYPE> tokens
func escCell(s string) string {
	s = codeifyTags(s)
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

var reTag = regexp.MustCompile(`\{\!\{([^}]+)\}\}`)

func extractFilename(input string) string {
	m := reTag.FindStringSubmatch(input)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func readExampleOrEmpty(exampleTag string) parsedContent {

	fn := extractFilename(exampleTag)

	if fn == "" {
		return parsedContent{}
	}
	full := filepath.Join(*examplesDirFlag, fn)

	raw, err := os.ReadFile("./test/docs/" + full)
	if err != nil {

		return parsedContent{}
	}
	// fmt.Printf(" ./test/docs/%s ", full)
	// fmt.Printf(" %s ", full)
	pc, _ := parseContent(string(raw))

	pc.hasExample = strings.TrimSpace(pc.HBConfig) != ""
	return pc
}

// Parse .hyperbricks multi-section files and extract the "hyperbricks config" block.
// Sections look like:
// ==== hyperbricks config {!{fragment}} ====
// ...config...
// ==== explainer ====
// ...text...
func parseContent(content string) (parsedContent, error) {
	header := regexp.MustCompile(`(?m)^====\s*([^!]+?)(?:\s*\{\!\{([^}]+)\}\})?\s*====\s*$`)
	sections := map[string]string{}
	var cur string
	var scope string
	var sb strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if m := header.FindStringSubmatch(line); m != nil {
			if cur != "" {
				sections[strings.ToLower(cur)] = sb.String()
				sb.Reset()
			}
			cur = strings.TrimSpace(m[1])
			if strings.EqualFold(cur, "hyperbricks config") && len(m) >= 3 {
				scope = strings.TrimSpace(m[2])
			}
			continue
		}
		if cur != "" {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	if cur != "" {
		sections[strings.ToLower(cur)] = sb.String()
	}
	return parsedContent{
		HBConfig: sections["hyperbricks config"],
		Result:   sections["expected output"],
		Scope:    scope,
		Explainer: func() string {
			if v, ok := sections["explainer"]; ok {
				return v
			}
			return ""
		}(),
		More: func() string {
			if v, ok := sections["more details"]; ok {
				return v
			}
			return ""
		}(),
	}, nil
}

// Deep search by field name across embedded structs (handles struct and *struct)
func findFieldByNameII(val reflect.Value, fieldName string) reflect.Value {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if !val.IsValid() || val.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	fv := val.FieldByName(fieldName)
	if fv.IsValid() {
		return fv
	}
	rt := val.Type()
	for i := 0; i < rt.NumField(); i++ {
		sub := val.Field(i)
		if !sub.IsValid() {
			continue
		}
		if sub.Kind() == reflect.Struct || (sub.Kind() == reflect.Ptr && sub.Elem().Kind() == reflect.Struct) {
			if found := findFieldByNameII(sub, fieldName); found.IsValid() {
				return found
			}
		}
	}
	return reflect.Value{}
}

// Return the first non-empty config line that mentions the property leaf
func firstLineMentioning(cfg string, prop string) string {
	for _, line := range strings.Split(cfg, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.Contains(s, prop) {
			return s
		}
	}
	return ""
}
