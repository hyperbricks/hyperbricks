package yamlparser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"gopkg.in/yaml.v3"
)

const orderKey = "@order"

var (
	envPlaceholderPattern      = regexp.MustCompile(`\{\{ENV:([a-zA-Z0-9_]+)\}\}`)
	varPlaceholderPattern      = regexp.MustCompile(`\{\{VAR:([a-zA-Z0-9_]+)\}\}`)
	confPlaceholderPattern     = regexp.MustCompile(`\{\{CONF:([a-zA-Z0-9_.]+)\}\}`)
	templatePlaceholderPattern = regexp.MustCompile(`\{\{TEMPLATE:([^}]+)\}\}`)
	filePlaceholderPattern     = regexp.MustCompile(`\{\{FILE:([^}]+)\}\}`)
)

// Options controls the HyperBricks YAML source pipeline.
type Options struct {
	Variables   map[string]string
	Env         map[string]string
	Config      map[string]interface{}
	TemplateDir string
	Paths       PathMarkers
}

// PathMarkers are the standard HyperBricks path placeholders available during
// YAML-safe preprocessing.
type PathMarkers struct {
	ModuleRoot  string
	Root        string
	Module      string
	Resources   string
	Templates   string
	Static      string
	HyperBricks string
}

// Result is the phase-aware output of the YAML source pipeline.
type Result struct {
	Preprocessed string
	Document     *Document
	Materialized map[string]interface{}
}

// Document is the ordered HyperBricks YAML source model.
type Document struct {
	Imports []string
	Roots   []*Node
}

// Node is a named HyperBricks object or nested object extension.
type Node struct {
	Name     string
	Type     string
	Inherit  string
	Props    map[string]interface{}
	Children []*Node
	Line     int
	Column   int
}

// ProcessBytes applies YAML-safe HyperBricks preprocessing, parses the source
// model, and materializes it into the current mapstructure-compatible map.
func ProcessBytes(input []byte, opts Options) (*Result, error) {
	preprocessed, err := PreprocessBytes(input, opts)
	if err != nil {
		return nil, err
	}
	doc, err := ParseBytes(preprocessed)
	if err != nil {
		return nil, err
	}
	materialized, err := doc.Materialize()
	if err != nil {
		return nil, err
	}
	return &Result{
		Preprocessed: string(preprocessed),
		Document:     doc,
		Materialized: materialized,
	}, nil
}

// ProcessFile loads a YAML source file, resolves imports, and materializes the
// merged document. Runtime loader integration will use this later; tests can use
// it now without changing server startup.
func ProcessFile(path string, opts Options) (*Result, error) {
	doc, err := LoadFile(path, opts)
	if err != nil {
		return nil, err
	}
	materialized, err := doc.Materialize()
	if err != nil {
		return nil, err
	}
	return &Result{
		Document:     doc,
		Materialized: materialized,
	}, nil
}

// PreprocessBytes applies the YAML-safe subset of HyperBricks preprocessing.
func PreprocessBytes(input []byte, opts Options) ([]byte, error) {
	source := string(input)
	if err := rejectLegacyMacros(source); err != nil {
		return nil, err
	}
	source = applyPathMarkers(source, opts.Paths)
	source = replacePlaceholders(source, opts)
	var err error
	source, err = replaceTemplateMarkers(source, opts)
	if err != nil {
		return nil, err
	}
	source, err = replaceFileMarkers(source)
	if err != nil {
		return nil, err
	}
	return []byte(source), nil
}

// LoadFile loads a HyperBricks YAML file and all top-level model imports.
func LoadFile(path string, opts Options) (*Document, error) {
	state := &loadState{
		loading: make(map[string]bool),
		loaded:  make(map[string]*Document),
	}
	return loadFile(path, opts, state)
}

// ParseBytes parses a strict HyperBricks YAML profile document.
func ParseBytes(input []byte) (*Document, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(input))
	decoder.KnownFields(false)
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return &Document{}, nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 {
		return nil, fmt.Errorf("expected one YAML document")
	}
	body := root.Content[0]
	if body.Kind == yaml.ScalarNode && strings.TrimSpace(body.Value) == "" {
		return &Document{}, nil
	}
	if body.Kind != yaml.MappingNode {
		return nil, nodeError(body, "top-level document must be a mapping")
	}

	doc := &Document{}
	seenRoots := make(map[string]bool)
	for i := 0; i < len(body.Content); i += 2 {
		keyNode := body.Content[i]
		valueNode := body.Content[i+1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			return nil, nodeError(keyNode, "top-level key cannot be empty")
		}
		if key == "imports" {
			imports, err := parseImports(valueNode)
			if err != nil {
				return nil, err
			}
			doc.Imports = append(doc.Imports, imports...)
			continue
		}
		if seenRoots[key] {
			return nil, nodeError(keyNode, "duplicate top-level object %q", key)
		}
		seenRoots[key] = true

		if valueNode.Kind != yaml.SequenceNode {
			return nil, nodeError(valueNode, "object %q must be an ordered sequence", key)
		}
		node, err := parseNodeSequence(key, valueNode)
		if err != nil {
			return nil, err
		}
		doc.Roots = append(doc.Roots, node)
	}
	if err := validateDocument(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// Materialize resolves inheritance and converts the ordered model to the
// current mapstructure-compatible HyperBricks map shape.
func (d *Document) Materialize() (map[string]interface{}, error) {
	roots := make(map[string]*Node, len(d.Roots))
	for _, root := range d.Roots {
		if strings.TrimSpace(root.Name) == "" {
			continue
		}
		roots[root.Name] = root
	}

	out := make(map[string]interface{}, len(d.Roots))
	resolved := make(map[string]*Node, len(d.Roots))
	resolving := make(map[string]bool, len(d.Roots))
	for _, root := range d.Roots {
		node, err := resolveRootNode(root, roots, resolved, resolving)
		if err != nil {
			return nil, err
		}
		out[root.Name] = node.ToMap()
	}
	return out, nil
}

// ToMap converts a resolved node into the runtime map shape.
func (n *Node) ToMap() map[string]interface{} {
	out := make(map[string]interface{}, len(n.Props)+len(n.Children)+2)
	if typ := formatType(n.Type); typ != "" {
		out["@type"] = typ
	}
	for key, value := range n.Props {
		out[key] = materializeValue(value)
	}
	if len(n.Children) > 0 {
		order := make([]string, 0, len(n.Children))
		for _, child := range n.Children {
			if strings.TrimSpace(child.Name) == "" {
				continue
			}
			order = append(order, child.Name)
			out[child.Name] = child.ToMap()
		}
		if len(order) > 0 {
			out[orderKey] = order
		}
	}
	return out
}

func parseImports(node *yaml.Node) ([]string, error) {
	switch node.Kind {
	case yaml.SequenceNode:
		imports := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return nil, nodeError(item, "imports entries must be strings")
			}
			value := strings.TrimSpace(item.Value)
			if value != "" {
				imports = append(imports, value)
			}
		}
		return imports, nil
	case yaml.ScalarNode:
		value := strings.TrimSpace(node.Value)
		if value == "" {
			return nil, nil
		}
		return []string{value}, nil
	default:
		return nil, nodeError(node, "imports must be a string or string sequence")
	}
}

func parseNodeSequence(name string, seq *yaml.Node) (*Node, error) {
	node := &Node{
		Name:   strings.TrimSpace(name),
		Props:  make(map[string]interface{}),
		Line:   seq.Line,
		Column: seq.Column,
	}
	seenEntries := make(map[string]bool)
	seenReserved := make(map[string]bool)
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return nil, nodeError(item, "node entries must be single-key mappings")
		}
		keyNode := item.Content[0]
		valueNode := item.Content[1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			return nil, nodeError(keyNode, "node entry key cannot be empty")
		}

		switch key {
		case "type":
			if seenReserved[key] {
				return nil, nodeError(keyNode, "duplicate reserved entry %q", key)
			}
			seenReserved[key] = true
			value, err := scalarString(valueNode)
			if err != nil {
				return nil, err
			}
			node.Type = value
		case "inherit":
			if seenReserved[key] {
				return nil, nodeError(keyNode, "duplicate reserved entry %q", key)
			}
			seenReserved[key] = true
			value, err := scalarString(valueNode)
			if err != nil {
				return nil, err
			}
			node.Inherit = value
		default:
			if valueNode.Kind == yaml.SequenceNode && looksLikeChildNodeSequence(valueNode) {
				child, err := parseNodeSequence(key, valueNode)
				if err != nil {
					return nil, err
				}
				node.Children = append(node.Children, child)
				continue
			}
			if seenEntries[key] {
				return nil, nodeError(keyNode, "duplicate property %q", key)
			}
			seenEntries[key] = true
			value, err := parseGenericValue(valueNode)
			if err != nil {
				return nil, err
			}
			node.Props[key] = value
		}
	}
	return node, nil
}

func parseGenericValue(node *yaml.Node) (interface{}, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return parseScalar(node)
	case yaml.MappingNode:
		return parseGenericMap(node)
	case yaml.SequenceNode:
		if looksLikeNodeSequence(node) {
			return parseNodeSequence("", node)
		}
		values := make([]interface{}, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := parseGenericValue(item)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.AliasNode:
		return nil, nodeError(node, "YAML aliases are not supported in HyperBricks source")
	default:
		return nil, nodeError(node, "unsupported YAML node kind %d", node.Kind)
	}
}

func parseGenericMap(node *yaml.Node) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			return nil, nodeError(keyNode, "map key cannot be empty")
		}
		if _, exists := out[key]; exists {
			return nil, nodeError(keyNode, "duplicate map key %q", key)
		}
		value, err := parseGenericValue(valueNode)
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, nil
}

func validateDocument(doc *Document) error {
	if doc == nil {
		return nil
	}
	for _, root := range doc.Roots {
		if err := validateNode(root, root.Name); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(node *Node, path string) error {
	if node == nil {
		return nil
	}
	if isInternalRuntimeKey(node.Name) {
		return nodeErrorFromNode(node, fmt.Sprintf("node name %q is reserved", node.Name))
	}
	nodeType, err := validateNodeType(node)
	if err != nil {
		return err
	}

	for key, value := range node.Props {
		if isInternalRuntimeKey(key) {
			return nodeErrorFromNode(node, fmt.Sprintf("property %q at %s is reserved", key, path))
		}
		if nested, ok := value.(*Node); ok {
			nestedPath := joinPath(path, key)
			if err := validateNode(nested, nestedPath); err != nil {
				return err
			}
		}
	}

	childNames := make(map[string]*Node, len(node.Children))
	for _, child := range node.Children {
		if child == nil {
			continue
		}
		childPath := joinPath(path, child.Name)
		if isInternalRuntimeKey(child.Name) {
			return nodeErrorFromNode(child, fmt.Sprintf("child name %q at %s is reserved", child.Name, childPath))
		}
		if _, exists := node.Props[child.Name]; exists {
			return nodeErrorFromNode(child, fmt.Sprintf("child %q at %s collides with a property on the same node", child.Name, childPath))
		}
		if existing := childNames[child.Name]; existing != nil {
			return nodeErrorFromNode(child, fmt.Sprintf("duplicate child %q at %s; first defined at line %d:%d", child.Name, path, existing.Line, existing.Column))
		}
		childNames[child.Name] = child
		if reservedRuntimeChildName(nodeType, child.Name) {
			return nodeErrorFromNode(child, fmt.Sprintf("child %q at %s collides with a reserved %s field", child.Name, childPath, nodeType))
		}
		if err := validateNode(child, childPath); err != nil {
			return err
		}
	}
	return nil
}

func validateNodeType(node *Node) (string, error) {
	if node == nil {
		return "", nil
	}
	if strings.TrimSpace(node.Type) == "" {
		return "", nil
	}
	token, ok := canonicalTypeToken(node.Type)
	if !ok {
		return "", nodeErrorFromNode(node, fmt.Sprintf("unknown component type %q", node.Type))
	}
	return token, nil
}

func joinPath(base string, name string) string {
	name = strings.TrimSpace(name)
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + "." + name
}

func isInternalRuntimeKey(name string) bool {
	switch strings.TrimSpace(name) {
	case "@type", orderKey:
		return true
	default:
		return false
	}
}

func reservedRuntimeChildName(parentType string, childName string) bool {
	parentType = strings.TrimSpace(parentType)
	childName = strings.TrimSpace(childName)
	if parentType == "" || childName == "" {
		return false
	}
	if allowed := structuredFieldChildren[parentType]; allowed[childName] {
		return false
	}
	if globalRuntimeFields[childName] {
		return true
	}
	if fields := runtimeFieldsByType[parentType]; fields[childName] {
		return true
	}
	return false
}

var canonicalTypeTokens = map[string]string{
	"api_fragment_render": "<API_FRAGMENT_RENDER>",
	"api_render":          "<API_RENDER>",
	"css":                 "<CSS>",
	"fragment":            "<FRAGMENT>",
	"head":                "<HEAD>",
	"html":                "<HTML>",
	"hypermedia":          "<HYPERMEDIA>",
	"image":               "<IMAGE>",
	"images":              "<IMAGES>",
	"javascript":          "<JAVASCRIPT>",
	"js":                  "<JS>",
	"json":                "<JSON_RENDER>",
	"json_render":         "<JSON_RENDER>",
	"menu":                "<MENU>",
	"plugin":              "<PLUGIN>",
	"styles":              "<STYLES>",
	"template":            "<TEMPLATE>",
	"text":                "<TEXT>",
	"tree":                "<TREE>",
}

var globalRuntimeFields = map[string]bool{
	"@doc":            true,
	"attributes":      true,
	"enclose":         true,
	"hyperbricksfile": true,
	"hyperbrickskey":  true,
	"hyperbrickspath": true,
}

var runtimeFieldsByType = map[string]map[string]bool{
	"<API_FRAGMENT_RENDER>": fieldSet(
		"beautify", "body", "cache", "content_type", "debug", "debugpanel",
		"endpoint", "guard", "headers", "hx_response", "index", "inline",
		"jwtclaims", "jwtsecret", "method", "nocache", "password",
		"querykeys", "queryparams", "response", "route", "section",
		"setcookie", "setcookies", "static", "status", "template", "title",
		"username", "values",
	),
	"<API_RENDER>": fieldSet(
		"body", "debug", "debugpanel", "endpoint", "headers", "inline",
		"jwtclaims", "jwtsecret", "method", "password", "querykeys",
		"queryparams", "setcookie", "setcookies", "status", "template",
		"username", "values",
	),
	"<CSS>": fieldSet("file", "inline", "link"),
	"<FRAGMENT>": fieldSet(
		"beautify", "cache", "content_type", "guard", "hx_response", "index",
		"nocache", "response", "route", "section", "static", "template",
		"title",
	),
	"<HEAD>": fieldSet("css", "favicon", "js", "meta", "title"),
	"<HTML>": fieldSet("trimspace", "value"),
	"<HYPERMEDIA>": fieldSet(
		"beautify", "bodytag", "cache", "content_type", "cookies",
		"doctype", "favicon", "guard", "head", "headers", "htmltag", "index",
		"nocache", "route", "section", "static", "template", "title",
	),
	"<IMAGE>": fieldSet(
		"alt", "class", "height", "id", "is_static", "loading", "quality",
		"src", "title", "width",
	),
	"<IMAGES>": fieldSet(
		"alt", "class", "directory", "height", "id", "loading", "quality",
		"title", "width",
	),
	"<JAVASCRIPT>":  fieldSet("file", "inline", "link"),
	"<JS>":          fieldSet("file", "inline", "link"),
	"<JSON_RENDER>": fieldSet("debug", "file", "inline", "template", "values"),
	"<MENU>":        fieldSet("active", "item", "order", "section", "sort"),
	"<PLUGIN>":      fieldSet("classes", "data", "plugin"),
	"<STYLES>":      fieldSet("file"),
	"<TEMPLATE>":    fieldSet("inline", "querykeys", "queryparams", "template", "values"),
	"<TEXT>":        fieldSet("value"),
}

var structuredFieldChildren = map[string]map[string]bool{
	"<API_FRAGMENT_RENDER>": fieldSet("guard", "response"),
	"<FRAGMENT>":            fieldSet("guard", "response", "template"),
	"<HYPERMEDIA>":          fieldSet("guard", "head", "template"),
}

func fieldSet(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}

func canonicalTypeToken(value string) (string, bool) {
	normalized := normalizeTypeName(value)
	token, ok := canonicalTypeTokens[normalized]
	return token, ok
}

func normalizeTypeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSuffix(value, ">")
	value = strings.ReplaceAll(value, "-", "_")
	return strings.ToLower(value)
}

func looksLikeNodeSequence(seq *yaml.Node) bool {
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return false
		}
		key := strings.TrimSpace(item.Content[0].Value)
		if key == "type" || key == "inherit" {
			return true
		}
	}
	return false
}

func looksLikeChildNodeSequence(seq *yaml.Node) bool {
	if looksLikeNodeSequence(seq) {
		return true
	}
	if len(seq.Content) == 0 {
		return false
	}
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return false
		}
		if strings.TrimSpace(item.Content[0].Value) == "" {
			return false
		}
	}
	return true
}

func scalarString(node *yaml.Node) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", nodeError(node, "expected scalar string")
	}
	return strings.TrimSpace(node.Value), nil
}

func parseScalar(node *yaml.Node) (interface{}, error) {
	if node.Tag == "!!null" {
		return "", nil
	}
	return node.Value, nil
}

type loadState struct {
	loading map[string]bool
	loaded  map[string]*Document
}

func loadFile(path string, opts Options, state *loadState) (*Document, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	absolutePath = filepath.Clean(absolutePath)
	if state.loading[absolutePath] {
		return nil, fmt.Errorf("import cycle detected at %s", absolutePath)
	}
	if loaded := state.loaded[absolutePath]; loaded != nil {
		return cloneDocument(loaded), nil
	}

	raw, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}
	preprocessed, err := PreprocessBytes(raw, opts)
	if err != nil {
		return nil, fmt.Errorf("preprocess %s: %w", absolutePath, err)
	}
	doc, err := ParseBytes(preprocessed)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", absolutePath, err)
	}

	state.loading[absolutePath] = true
	defer delete(state.loading, absolutePath)

	merged := &Document{}
	seenRoots := make(map[string]string)
	for _, importPath := range doc.Imports {
		fullImportPath := importPath
		if !filepath.IsAbs(fullImportPath) {
			fullImportPath = filepath.Join(filepath.Dir(absolutePath), importPath)
		}
		importedDoc, err := loadFile(fullImportPath, opts, state)
		if err != nil {
			return nil, err
		}
		if err := appendDocumentRoots(merged, importedDoc, seenRoots, fullImportPath); err != nil {
			return nil, err
		}
	}
	if err := appendDocumentRoots(merged, docWithoutImports(doc), seenRoots, absolutePath); err != nil {
		return nil, err
	}
	state.loaded[absolutePath] = cloneDocument(merged)
	return cloneDocument(merged), nil
}

func appendDocumentRoots(target *Document, source *Document, seenRoots map[string]string, sourcePath string) error {
	for _, root := range source.Roots {
		if root == nil {
			continue
		}
		name := strings.TrimSpace(root.Name)
		if name == "" {
			continue
		}
		if existingSource := seenRoots[name]; existingSource != "" {
			return fmt.Errorf("duplicate top-level object %q from %s; first defined in %s", name, sourcePath, existingSource)
		}
		seenRoots[name] = sourcePath
		target.Roots = append(target.Roots, cloneNode(root))
	}
	return nil
}

func docWithoutImports(doc *Document) *Document {
	out := &Document{}
	for _, root := range doc.Roots {
		out.Roots = append(out.Roots, cloneNode(root))
	}
	return out
}

func cloneDocument(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	out := &Document{
		Imports: append([]string(nil), doc.Imports...),
	}
	for _, root := range doc.Roots {
		out.Roots = append(out.Roots, cloneNode(root))
	}
	return out
}

func rejectLegacyMacros(source string) error {
	switch {
	case strings.Contains(source, "@macro"):
		return fmt.Errorf("legacy @macro syntax is not supported in HyperBricks YAML")
	case strings.Contains(source, "<<<["):
		return fmt.Errorf("legacy macro template blocks are not supported in HyperBricks YAML")
	case strings.Contains(source, "{{{."):
		return fmt.Errorf("legacy macro variables are not supported in HyperBricks YAML")
	default:
		return nil
	}
}

func applyPathMarkers(source string, paths PathMarkers) string {
	replacements := []string{
		"{{MODULE_ROOT}}", paths.ModuleRoot,
		"{{ROOT}}", paths.Root,
		"{{MODULE}}", paths.Module,
		"{{RESOURCES}}", paths.Resources,
		"{{TEMPLATES}}", paths.Templates,
		"{{STATIC}}", paths.Static,
		"{{HYPERBRICKS}}", paths.HyperBricks,
	}
	pairs := make([]string, 0, len(replacements))
	for i := 0; i < len(replacements); i += 2 {
		if replacements[i+1] == "" {
			continue
		}
		pairs = append(pairs, replacements[i], replacements[i+1])
	}
	if len(pairs) == 0 {
		return source
	}
	return strings.NewReplacer(pairs...).Replace(source)
}

func replacePlaceholders(source string, opts Options) string {
	source = envPlaceholderPattern.ReplaceAllStringFunc(source, func(match string) string {
		key := envPlaceholderPattern.FindStringSubmatch(match)[1]
		if value, ok := opts.Env[key]; ok {
			return value
		}
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return match
	})
	source = varPlaceholderPattern.ReplaceAllStringFunc(source, func(match string) string {
		key := varPlaceholderPattern.FindStringSubmatch(match)[1]
		if value, ok := opts.Variables[key]; ok {
			return value
		}
		return match
	})
	source = confPlaceholderPattern.ReplaceAllStringFunc(source, func(match string) string {
		key := confPlaceholderPattern.FindStringSubmatch(match)[1]
		value, ok := lookupConfig(opts.Config, strings.Split(key, "."))
		if !ok {
			return match
		}
		return fmt.Sprint(value)
	})
	return source
}

func replaceTemplateMarkers(source string, opts Options) (string, error) {
	if opts.TemplateDir == "" {
		return source, nil
	}
	var firstErr error
	processed := templatePlaceholderPattern.ReplaceAllStringFunc(source, func(match string) string {
		if firstErr != nil {
			return match
		}
		templateName := strings.TrimSpace(templatePlaceholderPattern.FindStringSubmatch(match)[1])
		if templateName == "" {
			return match
		}
		content, err := os.ReadFile(filepath.Join(opts.TemplateDir, templateName))
		if err != nil {
			firstErr = fmt.Errorf("read template marker %q: %w", templateName, err)
			return match
		}
		parser.AddTemplate(templateName, string(content))
		return templateName
	})
	return processed, firstErr
}

func replaceFileMarkers(source string) (string, error) {
	lines := strings.Split(source, "\n")
	for index, line := range lines {
		matches := filePlaceholderPattern.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if len(matches) != 1 || trimmed != matches[0][0] {
			return "", fmt.Errorf("{{FILE:...}} markers must occupy a full YAML block-scalar line")
		}
		path := strings.TrimSpace(matches[0][1])
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read file marker %q: %w", path, err)
		}
		indent := line[:strings.Index(line, trimmed)]
		contentLines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
		for i, contentLine := range contentLines {
			contentLines[i] = indent + contentLine
		}
		lines[index] = strings.Join(contentLines, "\n")
	}
	return strings.Join(lines, "\n"), nil
}

func lookupConfig(config map[string]interface{}, path []string) (interface{}, bool) {
	if len(path) == 0 || config == nil {
		return nil, false
	}
	current := interface{}(config)
	for _, part := range path {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = currentMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func resolveRootNode(node *Node, roots map[string]*Node, resolved map[string]*Node, resolving map[string]bool) (*Node, error) {
	return resolveNode(node, roots, resolved, resolving, true)
}

func resolveNode(node *Node, roots map[string]*Node, resolved map[string]*Node, resolving map[string]bool, cacheByName bool) (*Node, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot resolve nil node")
	}
	if cacheByName && strings.TrimSpace(node.Name) != "" {
		if existing, ok := resolved[node.Name]; ok {
			return cloneNode(existing), nil
		}
		if resolving[node.Name] {
			return nil, fmt.Errorf("inheritance cycle at %q", node.Name)
		}
		resolving[node.Name] = true
		defer delete(resolving, node.Name)
	}

	var out *Node
	if ref := strings.TrimSpace(node.Inherit); ref != "" {
		base, err := resolveReference(ref, roots, resolved, resolving)
		if err != nil {
			return nil, nodeErrorFromNode(node, err.Error())
		}
		out = mergeNodes(base, nodeWithoutInherit(node))
	} else {
		out = cloneNode(node)
	}

	for index, child := range out.Children {
		resolvedChild, err := resolveNode(child, roots, resolved, resolving, false)
		if err != nil {
			return nil, err
		}
		out.Children[index] = resolvedChild
	}

	if cacheByName && strings.TrimSpace(node.Name) != "" {
		resolved[node.Name] = cloneNode(out)
	}
	return out, nil
}

func resolveReference(ref string, roots map[string]*Node, resolved map[string]*Node, resolving map[string]bool) (*Node, error) {
	parts := strings.Split(ref, ".")
	rootName := strings.TrimSpace(parts[0])
	root, ok := roots[rootName]
	if !ok {
		return nil, fmt.Errorf("inherit reference %q was not found", ref)
	}
	node, err := resolveRootNode(root, roots, resolved, resolving)
	if err != nil {
		return nil, err
	}
	for _, part := range parts[1:] {
		name := strings.TrimSpace(part)
		found := false
		for _, child := range node.Children {
			if child.Name == name {
				node = child
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("inherit reference %q was not found", ref)
		}
	}
	return cloneNode(node), nil
}

func nodeWithoutInherit(node *Node) *Node {
	out := cloneNode(node)
	out.Inherit = ""
	return out
}

func mergeNodes(base *Node, overlay *Node) *Node {
	out := cloneNode(base)
	out.Name = overlay.Name
	if strings.TrimSpace(overlay.Type) != "" {
		out.Type = overlay.Type
	}
	if out.Props == nil {
		out.Props = make(map[string]interface{})
	}
	for key, value := range overlay.Props {
		out.Props[key] = mergeValues(out.Props[key], value)
	}

	indexByName := make(map[string]int, len(out.Children))
	for index, child := range out.Children {
		indexByName[child.Name] = index
	}
	for _, child := range overlay.Children {
		if existingIndex, ok := indexByName[child.Name]; ok {
			out.Children[existingIndex] = mergeNodes(out.Children[existingIndex], child)
			continue
		}
		out.Children = append(out.Children, cloneNode(child))
	}
	return out
}

func mergeValues(base interface{}, overlay interface{}) interface{} {
	baseMap, baseOK := base.(map[string]interface{})
	overlayMap, overlayOK := overlay.(map[string]interface{})
	if baseOK && overlayOK {
		out := cloneMap(baseMap)
		for key, value := range overlayMap {
			out[key] = mergeValues(out[key], value)
		}
		return out
	}

	baseNode, baseOK := base.(*Node)
	overlayNode, overlayOK := overlay.(*Node)
	if baseOK && overlayOK {
		return mergeNodes(baseNode, overlayNode)
	}
	return cloneValue(overlay)
}

func materializeValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case *Node:
		return typed.ToMap()
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = materializeValue(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, value := range typed {
			out = append(out, materializeValue(value))
		}
		return out
	default:
		return typed
	}
}

func cloneNode(node *Node) *Node {
	if node == nil {
		return nil
	}
	out := &Node{
		Name:    node.Name,
		Type:    node.Type,
		Inherit: node.Inherit,
		Props:   cloneMap(node.Props),
		Line:    node.Line,
		Column:  node.Column,
	}
	if len(node.Children) > 0 {
		out.Children = make([]*Node, 0, len(node.Children))
		for _, child := range node.Children {
			out.Children = append(out.Children, cloneNode(child))
		}
	}
	return out
}

func cloneMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return make(map[string]interface{})
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case *Node:
		return cloneNode(typed)
	case map[string]interface{}:
		return cloneMap(typed)
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneValue(item))
		}
		return out
	default:
		return typed
	}
}

func formatType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if token, ok := canonicalTypeToken(value); ok {
		return token
	}
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") {
		return strings.ToUpper(value)
	}
	return "<" + strings.ToUpper(value) + ">"
}

func nodeError(node *yaml.Node, format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)
	if node == nil || node.Line == 0 {
		return errors.New(message)
	}
	return fmt.Errorf("line %d:%d: %s", node.Line, node.Column, message)
}

func nodeErrorFromNode(node *Node, message string) error {
	if node == nil || node.Line == 0 {
		return errors.New(message)
	}
	return fmt.Errorf("line %d:%d: %s", node.Line, node.Column, message)
}
