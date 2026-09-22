package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/schema"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type scaffoldSpec struct {
	Module, Config, Category, Type, Name, File, ImportInto string
	Route, Title, Template, TemplateMode, Inline           string
	Properties, Children                                   string
	Starter                                                string
}

func scaffoldTypes(category string) []schema.Definition {
	var defs []schema.Definition
	for _, def := range schema.Definitions() {
		if category == "" || scaffoldCategory(def) == category {
			defs = append(defs, def)
		}
	}
	sort.Slice(defs, func(i, j int) bool { return scaffoldTypeName(defs[i]) < scaffoldTypeName(defs[j]) })
	return defs
}
func scaffoldCategory(def schema.Definition) string {
	// The terminal's two categories follow the actual Go component ownership;
	// the schema's data/menu/resources presentation categories are all components.
	if strings.HasSuffix(def.ConfigType.PkgPath(), "/composite") {
		return "composite"
	}
	return "component"
}
func scaffoldTypeName(def schema.Definition) string {
	return strings.ToLower(strings.Trim(def.Token, "<>"))
}
func scaffoldDefinition(kind string) (schema.Definition, error) {
	for _, def := range scaffoldTypes("") {
		for _, token := range schema.Tokens(def) {
			if strings.EqualFold(strings.Trim(kind, "<>"), strings.Trim(token, "<>")) {
				return def, nil
			}
		}
	}
	return schema.Definition{}, fmt.Errorf("unknown type %q; use hyperbricks author explain-type <type>", kind)
}

var scaffoldNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,79}$`)
var scaffoldRoutePattern = regexp.MustCompile(`^[A-Za-z0-9_~.-]+(?:/[A-Za-z0-9_~.-]+)*$`)

func scaffoldRouteOwner(kind string) bool {
	return kind == "hypermedia" || kind == "fragment" || kind == "api_fragment_render"
}
func validateScaffoldRoute(route string) error {
	if !scaffoldRoutePattern.MatchString(route) || path.Clean(route) != route || len(route) > 240 {
		return fmt.Errorf("route must be a clean relative path; use index for /")
	}
	for _, part := range strings.Split(route, "/") {
		if part == "." || part == ".." {
			return fmt.Errorf("route cannot contain dot segments")
		}
	}
	switch strings.Split(route, "/")[0] {
	case "static", "out", "__hyperbricks":
		return fmt.Errorf("route uses a reserved prefix")
	}
	return nil
}
func scaffoldDefaults(kind, name string) map[string]interface{} {
	switch kind {
	case "text":
		return map[string]interface{}{"value": name}
	case "html":
		return map[string]interface{}{"value": "<p>Content</p>"}
	case "markdown":
		return map[string]interface{}{"content": "## " + name + "\n"}
	case "template":
		return map[string]interface{}{"inline": "<section>{{.title}}</section>", "values": map[string]interface{}{"title": name}}
	case "api_render", "api_fragment_render":
		return map[string]interface{}{"method": "GET", "inline": "<pre>{{.Data}}</pre>"}
	case "css":
		return map[string]interface{}{"inline": "/* Styles */"}
	case "javascript", "js":
		return map[string]interface{}{"inline": "// Script"}
	case "goja_render":
		return map[string]interface{}{"script": "function main(input) { return {}; }", "inline": "<pre>{{.Data}}</pre>"}
	case "json_render":
		return map[string]interface{}{"inline": "<pre>{{.Data}}</pre>"}
	case "menu":
		return map[string]interface{}{"item": `<a href="{{if eq .Route "index"}}/{{else}}/{{.Route}}{{end}}">{{.Title}}</a>`, "active": `<a href="{{if eq .Route "index"}}/{{else}}/{{.Route}}{{end}}" aria-current="page">{{.Title}}</a>`}
	}
	return map[string]interface{}{}
}

func scaffoldProperties(spec scaffoldSpec, def schema.Definition) (map[string]interface{}, error) {
	props := scaffoldDefaults(scaffoldTypeName(def), spec.Name)
	var supplied map[string]interface{}
	if spec.Properties != "" {
		if err := json.Unmarshal([]byte(spec.Properties), &supplied); err != nil || supplied == nil {
			return nil, fmt.Errorf("--properties must be a JSON object")
		}
	}
	allowed := map[string]bool{}
	for _, f := range schema.ExtractDefinition(def).Fields {
		allowed[strings.Split(f.Path, ".")[0]] = true
	}
	for key, v := range supplied {
		if key == "type" || key == "inherit" || strings.HasPrefix(key, "@") || !allowed[key] {
			return nil, fmt.Errorf("%s has no configurable property %q; use --children for child objects", scaffoldTypeName(def), key)
		}
		props[key] = v
	}
	if _, file := supplied["file"]; file && scaffoldTypeName(def) == "markdown" {
		if _, content := supplied["content"]; !content {
			delete(props, "content")
		}
	}
	if _, template := supplied["template"]; template {
		if _, inline := supplied["inline"]; !inline {
			delete(props, "inline")
		}
	}
	for _, pair := range [][2]string{{"route", spec.Route}, {"title", spec.Title}, {"inline", spec.Inline}} {
		if pair[1] == "" {
			continue
		}
		if !allowed[pair[0]] {
			return nil, fmt.Errorf("%s does not support --%s", scaffoldTypeName(def), pair[0])
		}
		if _, exists := supplied[pair[0]]; exists {
			return nil, fmt.Errorf("specify %s once, not in both flags and --properties", pair[0])
		}
		props[pair[0]] = pair[1]
	}
	if spec.Template != "" {
		if !allowed["inline"] || !allowed["template"] {
			return nil, fmt.Errorf("--template applies to template-backed types; add a template child to a page or tree")
		}
		if spec.Inline != "" || supplied["inline"] != nil || supplied["template"] != nil {
			return nil, fmt.Errorf("choose one template source")
		}
		delete(props, "inline")
		props["template"] = map[string]interface{}{"file": spec.Template}
	} else if spec.TemplateMode != "" {
		return nil, fmt.Errorf("--template-mode requires --template")
	}
	for _, f := range schema.ExtractDefinition(def).Fields {
		if f.Required && !strings.Contains(f.Path, ".") {
			if v, exists := props[f.Path]; !exists || v == "" || v == nil {
				return nil, fmt.Errorf("%s requires property %s (provide --properties)", scaffoldTypeName(def), f.Path)
			}
		}
	}
	if scaffoldTypeName(def) == "plugin" {
		if name, _ := props["plugin"].(string); strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("plugin requires an enabled plugin name in --properties")
		}
	}
	return props, nil
}

func scaffoldObject(spec scaffoldSpec, def schema.Definition, props map[string]interface{}) (*yaml.Node, error) {
	n := yseq()
	yput(n, "type", ystr(scaffoldTypeName(def)))
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var v yaml.Node
		if err := v.Encode(props[key]); err != nil {
			return nil, err
		}
		yput(n, key, &v)
	}
	if strings.TrimSpace(spec.Children) == "" || strings.TrimSpace(spec.Children) == "[]" {
		return n, nil
	}
	if def.ChildModel == schema.ChildModelNone {
		return nil, fmt.Errorf("%s does not accept child objects", scaffoldTypeName(def))
	}
	if !json.Valid([]byte(spec.Children)) {
		return nil, fmt.Errorf("--children must be a JSON array of ordered single-key child mappings")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(spec.Children), &doc); err != nil {
		return nil, err
	}
	children := doc.Content[0]
	scaffoldBlockStyle(children)
	if children.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("--children must be an ordered JSON array")
	}
	seen := map[string]bool{}
	for _, entry := range children.Content {
		if entry.Kind != yaml.MappingNode || len(entry.Content) != 2 {
			return nil, fmt.Errorf("each child must be a single-key mapping")
		}
		key, child := entry.Content[0].Value, entry.Content[1]
		if !scaffoldNamePattern.MatchString(key) || seen[key] || key == "type" || key == "inherit" || key == "imports" || key == "vars" {
			return nil, fmt.Errorf("invalid or duplicate child name %q", key)
		}
		seen[key] = true
		if child.Kind != yaml.SequenceNode || (yget(child, "type") == nil && yget(child, "inherit") == nil) {
			return nil, fmt.Errorf("child %s must be an ordered component with type or inherit", key)
		}
		if def.ChildModel == schema.ChildModelValues {
			values := yget(n, "values")
			if values == nil {
				values = ymap()
				yput(n, "values", values)
			}
			if values.Kind != yaml.MappingNode || yget(values, key) != nil {
				return nil, fmt.Errorf("child %s conflicts with template values", key)
			}
			yput(values, key, child)
		} else {
			if yget(n, key) != nil {
				return nil, fmt.Errorf("child %s conflicts with a property", key)
			}
			for _, f := range schema.ExtractDefinition(def).Fields {
				if strings.Split(f.Path, ".")[0] == key && !(key == "head" && scaffoldTypeName(def) == "hypermedia") {
					return nil, fmt.Errorf("%s is a property, not a child slot", key)
				}
			}
			yput(n, key, child)
		}
	}
	// A newly generated template includes its selected value-mounted children.
	var supplied map[string]interface{}
	_ = json.Unmarshal([]byte(spec.Properties), &supplied)
	_, explicitInline := supplied["inline"]
	if def.ChildModel == schema.ChildModelValues && spec.Inline == "" && !explicitInline && (spec.Template == "" || spec.TemplateMode == "create") {
		var body strings.Builder
		body.WriteString("<section>{{.title}}")
		for _, entry := range children.Content {
			body.WriteString(scaffoldChildBinding(entry.Content[0].Value))
		}
		body.WriteString("</section>")
		if inline := yget(n, "inline"); inline != nil {
			inline.Value = body.String()
		}
	}
	return n, nil
}

func scaffoldBlockStyle(n *yaml.Node) {
	n.Style = 0
	for _, child := range n.Content {
		scaffoldBlockStyle(child)
	}
}

func scaffoldChildBinding(name string) string {
	if strings.Contains(name, "-") {
		return fmt.Sprintf("{{index . %q}}", name)
	}
	return "{{." + name + "}}"
}

func prepareScaffold(spec scaffoldSpec) (*scaffoldPlan, error) {
	return prepareScaffoldNode(spec, nil)
}

func prepareScaffoldNode(spec scaffoldSpec, authored *yaml.Node) (*scaffoldPlan, error) {
	return prepareScaffoldNodeInPlan(spec, authored, nil)
}

func prepareScaffoldNodeInPlan(spec scaffoldSpec, authored *yaml.Node, p *scaffoldPlan) (*scaffoldPlan, error) {
	return prepareScaffoldNodeInPlanValidated(spec, authored, p, true)
}

func prepareScaffoldNodeInPlanValidated(spec scaffoldSpec, authored *yaml.Node, p *scaffoldPlan, validateResources bool) (*scaffoldPlan, error) {
	if !scaffoldNamePattern.MatchString(spec.Name) || spec.Name == "imports" || spec.Name == "vars" {
		return nil, fmt.Errorf("--name must start with a letter and contain up to 80 letters, digits, underscores or hyphens; imports/vars are reserved")
	}
	def, err := scaffoldDefinition(spec.Type)
	if err != nil {
		return nil, err
	}
	if spec.Category != "" && spec.Category != scaffoldCategory(def) {
		return nil, fmt.Errorf("%s belongs to category %s", spec.Type, scaffoldCategory(def))
	}
	if p == nil {
		m, err := loadAuthoringModule(spec.Module, spec.Config)
		if err != nil {
			return nil, err
		}
		p, err = newScaffoldPlan(m)
		if err != nil {
			return nil, err
		}
	}
	m := p.module
	p.Name = spec.Name
	p.Type = scaffoldTypeName(def)
	p.Command = "hyperbricks scaffold"
	_, owners, err := p.graph(p.staged)
	if err != nil {
		return nil, err
	}
	if previous := owners[spec.Name]; previous != "" {
		return nil, fmt.Errorf("root %s already exists in %s", spec.Name, previous)
	}
	if !strings.HasSuffix(spec.File, ".hyperbricks.yaml") || filepath.Base(spec.File) == PackageConfigFileName {
		return nil, fmt.Errorf("--file must name a component .hyperbricks.yaml file, not package configuration")
	}
	parts := strings.Split(spec.File, "/")
	if len(parts) == 3 && parts[0] == "spaces" && parts[2] == "index.hyperbricks.yaml" {
		return nil, fmt.Errorf("managed Spaces indexes contain imports only")
	}
	target, err := authoringPath(m.Directories["hyperbricks"], spec.File)
	if err != nil {
		return nil, err
	}
	raw, err := p.overlay(target)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	doc, err := sourceYAML(raw)
	if err != nil {
		return nil, err
	}
	if yget(doc, spec.Name) != nil {
		return nil, fmt.Errorf("root %s already exists in destination", spec.Name)
	}
	n := authored
	if n == nil {
		props, err := scaffoldProperties(spec, def)
		if err != nil {
			return nil, err
		}
		n, err = scaffoldObject(spec, def, props)
		if err != nil {
			return nil, err
		}
	}
	if spec.Template != "" {
		tpl, err := authoringPath(m.Directories["templates"], spec.Template)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(spec.Template, ".html") {
			return nil, fmt.Errorf("template must end in .html")
		}
		_, err = p.read(tpl)
		switch spec.TemplateMode {
		case "create":
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("template already exists or cannot be created: %s", tpl)
			}
			body, _ := scaffoldDefaults(scaffoldTypeName(def), spec.Name)["inline"].(string)
			body += "\n"
			if def.ChildModel == schema.ChildModelValues && spec.Children != "" {
				var entries []map[string]interface{}
				_ = json.Unmarshal([]byte(spec.Children), &entries)
				body = "<section>{{.title}}"
				for _, entry := range entries {
					for key := range entry {
						body += scaffoldChildBinding(key)
					}
				}
				body += "</section>\n"
			}
			if err = p.add(tpl, []byte(body)); err != nil {
				return nil, err
			}
		case "reuse":
			if err != nil {
				return nil, fmt.Errorf("template reuse: %w", err)
			}
		default:
			return nil, fmt.Errorf("--template-mode must be create or reuse when --template is supplied")
		}
	}
	yput(doc.Content[0], spec.Name, n)
	out, err := yamlBytes(doc)
	if err != nil {
		return nil, err
	}
	if err = p.add(target, out); err != nil {
		return nil, err
	}
	loaded := false
	for _, owner := range owners {
		if owner == target {
			loaded = true
		}
	}
	// Imports-only destinations have no root owner; the read set also tracks them.
	if _, ok := p.inputs[target]; ok && raw != nil {
		for _, top := range p.tops {
			d, e := yamlparserDocumentPaths(p, top)
			if e != nil {
				return nil, e
			}
			if containsString(d, target) {
				loaded = true
			}
		}
	}
	needsImport := filepath.Dir(target) != m.Directories["hyperbricks"] && !loaded
	if needsImport && spec.ImportInto == "" {
		return nil, fmt.Errorf("nested file is not loaded; select --import-into relative to the HyperBricks directory")
	}
	if spec.ImportInto != "" {
		parent, err := authoringPath(m.Directories["hyperbricks"], spec.ImportInto)
		if err != nil {
			return nil, err
		}
		if parent == target {
			return nil, fmt.Errorf("file cannot import itself")
		}
		if _, ok := p.inputs[parent]; !ok {
			return nil, fmt.Errorf("import parent must already be loaded: %s", spec.ImportInto)
		}
		if strings.HasPrefix(spec.ImportInto, "spaces/") && filepath.Base(parent) == "index.hyperbricks.yaml" {
			return nil, fmt.Errorf("cannot attach definitions through a managed Spaces index")
		}
		b, err := p.overlay(parent)
		if err != nil {
			return nil, err
		}
		pd, err := sourceYAML(b)
		if err != nil {
			return nil, err
		}
		imports := yget(pd, "imports")
		if imports == nil {
			imports = yseq()
			yput(pd.Content[0], "imports", imports)
		}
		if imports.Tag == "!!null" {
			*imports = *yseq()
		}
		if imports.Kind == yaml.ScalarNode {
			old := *imports
			*imports = *yseq()
			imports.Content = []*yaml.Node{&old}
		}
		if imports.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("imports must be a sequence")
		}
		ref, err := filepath.Rel(filepath.Dir(parent), target)
		if err != nil {
			return nil, err
		}
		exists := false
		for _, item := range imports.Content {
			if filepath.Clean(filepath.Join(filepath.Dir(parent), item.Value)) == target {
				exists = true
			}
		}
		if !exists {
			imports.Content = append(imports.Content, ystr(filepath.ToSlash(ref)))
			b, err = yamlBytes(pd)
			if err != nil {
				return nil, err
			}
			if err = p.add(parent, b); err != nil {
				return nil, err
			}
		}
	}
	values, owners, err := p.graph(true)
	if err != nil {
		return nil, err
	}
	root := values[spec.Name]
	if root == nil {
		return nil, fmt.Errorf("new root is not reachable from loaded files")
	}
	if validateResources {
		if err = validateScaffoldObjects(p, root); err != nil {
			return nil, err
		}
	}
	if route, ok := root["route"]; ok {
		r, ok := route.(string)
		if !ok {
			return nil, fmt.Errorf("route must be a string")
		}
		if err = validateScaffoldRoute(r); err != nil {
			return nil, err
		}
		for name, obj := range values {
			if name == spec.Name {
				continue
			}
			if other, ok := obj["route"].(string); ok && normalizeScaffoldRoute(other) == normalizeScaffoldRoute(r) {
				owner, _ := filepath.Rel(p.module.Directories["hyperbricks"], owners[name])
				return nil, fmt.Errorf("route %s already belongs to %s in %s; additions do not replace existing content", r, name, filepath.ToSlash(owner))
			}
		}
	}
	return p, nil
}

func yamlparserDocumentPaths(p *scaffoldPlan, top string) ([]string, error) {
	var paths []string
	_, err := yamlparser.LoadFileWithReader(top, p.module.options(), func(path string) ([]byte, error) { paths = append(paths, path); return p.overlay(path) })
	return paths, err
}
func normalizeScaffoldRoute(r string) string {
	r = strings.Trim(r, "/")
	if r == "index" {
		return ""
	}
	return r
}

func validateScaffoldObjects(p *scaffoldPlan, obj map[string]interface{}) error {
	if token, ok := obj["@type"].(string); ok {
		def, err := scaffoldDefinition(token)
		if err != nil {
			return err
		}
		kind := scaffoldTypeName(def)
		f := typefactory.NewTypeFactory()
		f.RegisterType(token, def.ConfigType)
		if _, err = f.CreateInstance(typefactory.TypeRequest{TypeName: token, Data: obj}); err != nil {
			return fmt.Errorf("%s: %w", kind, err)
		}
		for _, field := range schema.ExtractDefinition(def).Fields {
			if field.Required && !strings.Contains(field.Path, ".") {
				if v, ok := obj[field.Path]; !ok || v == nil || reflect.ValueOf(v).IsZero() {
					return fmt.Errorf("%s requires %s", kind, field.Path)
				}
			}
		}
		if endpoint, ok := obj["endpoint"].(string); ok {
			u, err := url.Parse(endpoint)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fmt.Errorf("endpoint must be an explicit HTTP(S) URL")
			}
		}
		if kind == "markdown" {
			if file, ok := obj["file"].(string); ok {
				if !strings.HasSuffix(file, ".md") && !strings.HasSuffix(file, ".markdown") {
					return fmt.Errorf("markdown file must end in .md or .markdown")
				}
				path, err := authoringPath(p.module.Directories["resources"], file)
				if err != nil {
					return err
				}
				if _, err = p.overlay(path); err != nil {
					return fmt.Errorf("markdown file: %w", err)
				}
			}
		}
		if err := validateScaffoldFiles(p, kind, obj); err != nil {
			return err
		}
		templateBody, _ := obj["inline"].(string)
		if template, ok := obj["template"].(string); ok {
			path, err := authoringPath(p.module.Directories["templates"], template)
			if err != nil {
				return err
			}
			body, err := p.overlay(path)
			if err != nil {
				return fmt.Errorf("template file: %w", err)
			}
			if templateBody == "" {
				templateBody = string(body)
			}
		}
		switch kind {
		case "template", "api_render", "api_fragment_render", "json_render", "goja_render":
			if templateBody == "" {
				return fmt.Errorf("%s requires inline or template content", kind)
			}
			if _, err := shared.GenericTemplate().Parse(templateBody); err != nil {
				return fmt.Errorf("%s template: %w", kind, err)
			}
		}
	}
	for key, v := range obj {
		if key == "editable" || key == "data" {
			continue
		}
		if child, ok := v.(map[string]interface{}); ok {
			if err := validateScaffoldObjects(p, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateScaffoldFiles(p *scaffoldPlan, kind string, obj map[string]interface{}) error {
	field := map[string]string{"json_render": "file", "styles": "file", "css": "file", "js": "file", "image": "src", "images": "directory", "esbuild": "entry"}[kind]
	file, _ := obj[field].(string)
	if file == "" {
		return nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(p.module.Root, abs)
	if err != nil {
		return err
	}
	abs, err = authoringPath(p.module.Root, filepath.ToSlash(rel))
	if err != nil {
		return fmt.Errorf("%s.%s: use an existing module-local path (prefer a path resolver): %w", kind, field, err)
	}
	if kind == "images" {
		st, err := os.Stat(abs)
		if (err != nil || !st.IsDir()) && !p.hasPlannedFileIn(abs) {
			return fmt.Errorf("images.directory must reference an existing local directory")
		}
		return nil
	}
	b, err := p.overlay(abs)
	if err != nil {
		return fmt.Errorf("%s.%s: %w", kind, field, err)
	}
	if kind == "json_render" {
		var data map[string]interface{}
		if err := json.Unmarshal(b, &data); err != nil || data == nil {
			return fmt.Errorf("json_render.file must contain a JSON object")
		}
	}
	if kind == "esbuild" {
		outfile, _ := obj["outfile"].(string)
		abs, err := filepath.Abs(outfile)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(p.module.Directories["static"], abs)
		if err != nil {
			return err
		}
		if _, err := authoringPath(p.module.Directories["static"], filepath.ToSlash(rel)); err != nil {
			return fmt.Errorf("esbuild.outfile must remain inside static: %w", err)
		}
	}
	return nil
}
