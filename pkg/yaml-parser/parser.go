package yamlparser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

const orderKey = "@order"

// Options controls the HyperBricks YAML source pipeline.
type Options struct {
	Variables                map[string]string
	Env                      map[string]string
	Config                   map[string]interface{}
	TemplateDir              string
	Paths                    PathMarkers
	RecoverDuplicateChildren bool
	AllowUnknownTypes        bool
}

// PathMarkers are the standard HyperBricks path bases available to YAML value
// resolvers.
type PathMarkers struct {
	ModuleRoot  string
	Root        string
	Module      string
	Resources   string
	Templates   string
	Static      string
	HyperBricks string
	Render      string
}

// Result is the phase-aware output of the YAML source pipeline.
type Result struct {
	Preprocessed string
	Document     *Document
	Materialized map[string]interface{}
	Diagnostics  []Diagnostic
}

// Document is the ordered HyperBricks YAML source model.
type Document struct {
	Imports     []string
	Vars        map[string]interface{}
	Roots       []*Node
	Diagnostics []Diagnostic
}

// Diagnostic describes a non-fatal YAML source normalization performed before
// runtime materialization.
type Diagnostic struct {
	Level            string
	Code             string
	Message          string
	Source           string
	Path             string
	OriginalName     string
	MaterializedName string
	Line             int
	Column           int
}

// ParseOptions controls strict parser behavior without changing the default
// source contract used by docs and parser tests.
type ParseOptions struct {
	RecoverDuplicateChildren bool
	AllowUnknownTypes        bool
}

// Node is a named HyperBricks object or nested object extension.
type Node struct {
	Name           string
	Type           string
	Inherit        string
	Props          map[string]interface{}
	Children       []*Node
	Line           int
	Column         int
	nativeAPIProps map[string]interface{}
}

// ProcessBytes applies YAML-safe HyperBricks preprocessing, parses the source
// model, and materializes it into the current mapstructure-compatible map.
func ProcessBytes(input []byte, opts Options) (*Result, error) {
	preprocessed, err := PreprocessBytes(input, opts)
	if err != nil {
		return nil, err
	}
	doc, err := ParseBytesWithOptions(preprocessed, ParseOptions{
		RecoverDuplicateChildren: opts.RecoverDuplicateChildren,
		AllowUnknownTypes:        opts.AllowUnknownTypes,
	})
	if err != nil {
		return nil, err
	}
	materialized, diagnostics, err := doc.MaterializeWithOptions(opts)
	if err != nil {
		return nil, err
	}
	return &Result{
		Preprocessed: string(preprocessed),
		Document:     doc,
		Materialized: materialized,
		Diagnostics:  append(append([]Diagnostic(nil), doc.Diagnostics...), diagnostics...),
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
	materialized, diagnostics, err := doc.MaterializeWithOptions(opts)
	if err != nil {
		return nil, err
	}
	applyDiagnosticSource(diagnostics, path)
	return &Result{
		Document:     doc,
		Materialized: materialized,
		Diagnostics:  append(append([]Diagnostic(nil), doc.Diagnostics...), diagnostics...),
	}, nil
}

// PreprocessBytes keeps the public preprocessing hook while YAML parsing and
// materialization own validation and value substitution.
func PreprocessBytes(input []byte, _ Options) ([]byte, error) {
	return append([]byte(nil), input...), nil
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
	return ParseBytesWithOptions(input, ParseOptions{})
}

// ParseBytesWithOptions parses a HyperBricks YAML profile document. The default
// remains strict; runtime loading can opt into recoverable source diagnostics.
func ParseBytesWithOptions(input []byte, opts ParseOptions) (*Document, error) {
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
	ctx := &parseContext{
		recoverDuplicateChildren: opts.RecoverDuplicateChildren,
		allowUnknownTypes:        opts.AllowUnknownTypes,
	}
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
		if key == "vars" {
			vars, err := parseVars(valueNode, ctx)
			if err != nil {
				return nil, err
			}
			if doc.Vars == nil {
				doc.Vars = make(map[string]interface{})
			}
			for varName, varValue := range vars {
				if _, exists := doc.Vars[varName]; exists {
					return nil, nodeError(keyNode, "duplicate top-level var %q", varName)
				}
				doc.Vars[varName] = varValue
			}
			continue
		}
		if seenRoots[key] {
			return nil, nodeError(keyNode, "duplicate top-level object %q", key)
		}
		seenRoots[key] = true

		if valueNode.Kind != yaml.SequenceNode {
			return nil, nodeError(valueNode, "object %q must be an ordered sequence", key)
		}
		node, err := parseNodeSequence(key, valueNode, ctx, key)
		if err != nil {
			return nil, err
		}
		doc.Roots = append(doc.Roots, node)
	}
	if err := validateDocument(doc, ctx); err != nil {
		return nil, err
	}
	doc.Diagnostics = append(doc.Diagnostics, ctx.diagnostics...)
	return doc, nil
}

// Materialize resolves inheritance and converts the ordered model to the
// current mapstructure-compatible HyperBricks map shape.
func (d *Document) Materialize() (map[string]interface{}, error) {
	materialized, _, err := d.MaterializeWithOptions(Options{})
	return materialized, err
}

// MaterializeWithOptions resolves inheritance, applies value resolvers, and
// converts the ordered model to the current mapstructure-compatible map shape.
func (d *Document) MaterializeWithOptions(opts Options) (map[string]interface{}, []Diagnostic, error) {
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
	ctx := newValueResolverContext(d, opts)
	for _, root := range d.Roots {
		node, err := resolveRootNode(root, roots, resolved, resolving)
		if err != nil {
			return nil, nil, err
		}
		out[root.Name] = materializeNodeToMap(node, ctx, root.Name)
	}
	return out, ctx.diagnostics, nil
}

// ToMap converts a resolved node into the runtime map shape.
func (n *Node) ToMap() map[string]interface{} {
	return materializeNodeToMap(n, nil, n.Name)
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

func parseVars(node *yaml.Node, ctx *parseContext) (map[string]interface{}, error) {
	if node.Kind != yaml.MappingNode {
		return nil, nodeError(node, "vars must be a mapping")
	}
	return parseGenericMap(node, ctx, "vars")
}

type parseContext struct {
	recoverDuplicateChildren bool
	allowUnknownTypes        bool
	diagnostics              []Diagnostic
}

type sourcePosition struct {
	line   int
	column int
}

func parseNodeSequence(name string, seq *yaml.Node, ctx *parseContext, path string) (*Node, error) {
	node := &Node{
		Name:   strings.TrimSpace(name),
		Props:  make(map[string]interface{}),
		Line:   seq.Line,
		Column: seq.Column,
	}
	explicitAPIType := isAPIComponentType(explicitNodeType(seq))
	seenEntries := make(map[string]bool)
	seenReserved := make(map[string]bool)
	seenChildren := make(map[string]bool)
	firstChildByOriginalName := make(map[string]sourcePosition)
	reservedSiblingNames := collectSiblingEntryNames(seq)
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
			preserveNative := isNativeAPIProperty(key)
			if preserveNative {
				value, err := parseNativeAPIValue(valueNode)
				if err != nil {
					return nil, err
				}
				if node.nativeAPIProps == nil {
					node.nativeAPIProps = make(map[string]interface{})
				}
				node.nativeAPIProps[key] = value
			}
			if valueNode.Kind == yaml.SequenceNode && looksLikeChildNodeSequence(valueNode) && !(preserveNative && explicitAPIType) {
				if looksLikeNodeSequence(valueNode) && reservedRuntimeChildName(formatType(node.Type), key) {
					return nil, nodeError(keyNode, "child %q at %s collides with a reserved %s field", key, joinPath(path, key), formatType(node.Type))
				}
				if !reservedRuntimeChildName(formatType(node.Type), key) {
					childName := normalizeChildName(key, keyNode, ctx, path, seenChildren, firstChildByOriginalName, reservedSiblingNames)
					child, err := parseNodeSequence(childName, valueNode, ctx, joinPath(path, childName))
					if err != nil {
						return nil, err
					}
					node.Children = append(node.Children, child)
					continue
				}
			}
			if seenEntries[key] {
				return nil, nodeError(keyNode, "duplicate property %q", key)
			}
			seenEntries[key] = true
			value, err := parseGenericValue(valueNode, ctx, joinPath(path, key))
			if err != nil {
				return nil, err
			}
			node.Props[key] = value
		}
	}
	return node, nil
}

func collectSiblingEntryNames(seq *yaml.Node) map[string]bool {
	names := make(map[string]bool)
	if seq == nil {
		return names
	}
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			continue
		}
		name := strings.TrimSpace(item.Content[0].Value)
		if name == "" || name == "type" || name == "inherit" {
			continue
		}
		names[name] = true
	}
	return names
}

func normalizeChildName(original string, keyNode *yaml.Node, ctx *parseContext, path string, seenChildren map[string]bool, firstByOriginal map[string]sourcePosition, reservedSiblingNames map[string]bool) string {
	name := strings.TrimSpace(original)
	if ctx == nil || !ctx.recoverDuplicateChildren {
		return name
	}
	if !seenChildren[name] {
		seenChildren[name] = true
		if _, exists := firstByOriginal[name]; !exists {
			firstByOriginal[name] = sourcePosition{line: keyNode.Line, column: keyNode.Column}
		}
		return name
	}

	materializedName := nextAvailableSiblingName(name, seenChildren, reservedSiblingNames)
	first := firstByOriginal[name]
	seenChildren[materializedName] = true
	ctx.diagnostics = append(ctx.diagnostics, Diagnostic{
		Level:            "warning",
		Code:             "duplicate_child_name",
		Message:          fmt.Sprintf("duplicate child %q at %s; first defined at line %d:%d; using %q as runtime path", name, path, first.line, first.column, materializedName),
		Path:             path,
		OriginalName:     name,
		MaterializedName: materializedName,
		Line:             keyNode.Line,
		Column:           keyNode.Column,
	})
	return materializedName
}

func nextAvailableSiblingName(base string, used map[string]bool, reserved map[string]bool) string {
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s_%d", base, index)
		if !used[candidate] && !reserved[candidate] {
			return candidate
		}
	}
}

func parseGenericValue(node *yaml.Node, ctx *parseContext, path string) (interface{}, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return parseScalar(node)
	case yaml.MappingNode:
		return parseGenericMap(node, ctx, path)
	case yaml.SequenceNode:
		if looksLikeNodeSequence(node) {
			return parseNodeSequence("", node, ctx, path)
		}
		values := make([]interface{}, 0, len(node.Content))
		for index, item := range node.Content {
			value, err := parseGenericValue(item, ctx, fmt.Sprintf("%s[%d]", path, index))
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

func parseGenericMap(node *yaml.Node, ctx *parseContext, path string) (map[string]interface{}, error) {
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
		value, err := parseGenericValue(valueNode, ctx, joinPath(path, key))
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, nil
}

func validateDocument(doc *Document, ctx *parseContext) error {
	if doc == nil {
		return nil
	}
	for _, root := range doc.Roots {
		if err := validateNode(root, root.Name, ctx); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(node *Node, path string, ctx *parseContext) error {
	if node == nil {
		return nil
	}
	if isInternalRuntimeKey(node.Name) {
		return nodeErrorFromNode(node, fmt.Sprintf("node name %q is reserved", node.Name))
	}
	nodeType, err := validateNodeType(node, ctx)
	if err != nil {
		return err
	}

	for key, value := range node.Props {
		if isInternalRuntimeKey(key) {
			return nodeErrorFromNode(node, fmt.Sprintf("property %q at %s is reserved", key, path))
		}
		if nested, ok := value.(*Node); ok {
			nestedPath := joinPath(path, key)
			if err := validateNode(nested, nestedPath, ctx); err != nil {
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
		if err := validateNode(child, childPath, ctx); err != nil {
			return err
		}
	}
	return nil
}

func validateNodeType(node *Node, ctx *parseContext) (string, error) {
	if node == nil {
		return "", nil
	}
	if strings.TrimSpace(node.Type) == "" {
		return "", nil
	}
	token, ok := canonicalTypeToken(node.Type)
	if !ok {
		if ctx != nil && ctx.allowUnknownTypes {
			return formatType(node.Type), nil
		}
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
	"goja_render":         "<GOJA_RENDER>",
	"esbuild":             "<ESBUILD>",
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
		"endpoint", "forwardtoken", "guard", "headers", "index", "inline",
		"jwtclaims", "jwtsecret", "method", "nocache", "password",
		"querykeys", "queryparams", "response", "route", "section",
		"setcookie", "setcookies", "static", "status", "template", "title",
		"username", "values",
	),
	"<API_RENDER>": fieldSet(
		"body", "debug", "debugpanel", "endpoint", "forwardtoken", "headers", "inline",
		"jwtclaims", "jwtsecret", "method", "password", "querykeys",
		"queryparams", "setcookie", "setcookies", "status", "template",
		"username", "values",
	),
	"<CSS>": fieldSet("file", "inline", "link"),
	"<FRAGMENT>": fieldSet(
		"beautify", "cache", "content_type", "guard", "index",
		"nocache", "response", "route", "section", "static", "template",
		"title",
	),
	"<HEAD>": fieldSet("css", "favicon", "js", "meta", "title"),
	"<HTML>": fieldSet("trimspace", "value"),
	"<HYPERMEDIA>": fieldSet(
		"beautify", "bodytag", "cache", "content_type", "cookies",
		"doctype", "favicon", "guard", "head", "headers", "htmltag", "index",
		"nocache", "response", "route", "section", "static", "template", "title",
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
	"<HYPERMEDIA>":          fieldSet("guard", "head", "response", "template"),
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

func explicitNodeType(seq *yaml.Node) string {
	if seq == nil {
		return ""
	}
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 || strings.TrimSpace(item.Content[0].Value) != "type" {
			continue
		}
		if item.Content[1].Kind == yaml.ScalarNode {
			return formatType(item.Content[1].Value)
		}
		return ""
	}
	return ""
}

func isAPIComponentType(componentType string) bool {
	switch formatType(componentType) {
	case "<API_RENDER>", "<API_FRAGMENT_RENDER>":
		return true
	default:
		return false
	}
}

func isNativeAPIProperty(name string) bool {
	switch name {
	case "forwardtoken", "setcookie", "setcookies":
		return true
	default:
		return false
	}
}

func parseNativeAPIValue(node *yaml.Node) (interface{}, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot preserve a nil YAML value")
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var value interface{}
		if err := node.Decode(&value); err != nil {
			return nil, nodeError(node, "could not decode native API value: %v", err)
		}
		return value, nil
	case yaml.MappingNode:
		out := make(map[string]interface{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			keyNode := node.Content[index]
			valueNode := node.Content[index+1]
			if keyNode.Kind != yaml.ScalarNode {
				return nil, nodeError(keyNode, "native API map keys must be strings")
			}
			key := strings.TrimSpace(keyNode.Value)
			if key == "" {
				return nil, nodeError(keyNode, "native API map key cannot be empty")
			}
			if _, exists := out[key]; exists {
				return nil, nodeError(keyNode, "duplicate native API map key %q", key)
			}
			value, err := parseNativeAPIValue(valueNode)
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]interface{}, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := parseNativeAPIValue(item)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		return out, nil
	case yaml.AliasNode:
		return nil, nodeError(node, "YAML aliases are not supported in native API fields")
	default:
		return nil, nodeError(node, "unsupported YAML node kind %d in native API field", node.Kind)
	}
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
	doc, err := ParseBytesWithOptions(preprocessed, ParseOptions{
		RecoverDuplicateChildren: opts.RecoverDuplicateChildren,
		AllowUnknownTypes:        opts.AllowUnknownTypes,
	})
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", absolutePath, err)
	}
	applyDiagnosticSource(doc.Diagnostics, absolutePath)

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
		merged.Diagnostics = append(merged.Diagnostics, importedDoc.Diagnostics...)
	}
	if err := appendDocumentRoots(merged, docWithoutImports(doc), seenRoots, absolutePath); err != nil {
		return nil, err
	}
	merged.Diagnostics = append(merged.Diagnostics, doc.Diagnostics...)
	state.loaded[absolutePath] = cloneDocument(merged)
	return cloneDocument(merged), nil
}

func appendDocumentRoots(target *Document, source *Document, seenRoots map[string]string, sourcePath string) error {
	if len(source.Vars) > 0 {
		if target.Vars == nil {
			target.Vars = make(map[string]interface{})
		}
		for key, value := range source.Vars {
			if _, exists := target.Vars[key]; exists {
				return fmt.Errorf("duplicate top-level var %q from %s", key, sourcePath)
			}
			target.Vars[key] = cloneValue(value)
		}
	}
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
	out := &Document{Vars: cloneMap(doc.Vars)}
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
		Imports:     append([]string(nil), doc.Imports...),
		Vars:        cloneMap(doc.Vars),
		Diagnostics: append([]Diagnostic(nil), doc.Diagnostics...),
	}
	for _, root := range doc.Roots {
		out.Roots = append(out.Roots, cloneNode(root))
	}
	return out
}

func applyDiagnosticSource(diagnostics []Diagnostic, source string) {
	for index := range diagnostics {
		if diagnostics[index].Source == "" {
			diagnostics[index].Source = source
		}
	}
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
		override := nodeWithoutInherit(node)
		if err := normalizeEsbuildAlias(override, base.Type); err != nil {
			return nil, err
		}
		out = mergeNodes(base, override)
	} else {
		out = cloneNode(node)
		if err := normalizeEsbuildAlias(out, ""); err != nil {
			return nil, err
		}
	}

	for index, child := range out.Children {
		resolvedChild, err := resolveNode(child, roots, resolved, resolving, false)
		if err != nil {
			return nil, err
		}
		out.Children[index] = resolvedChild
	}
	for key, value := range out.Props {
		resolvedValue, err := resolveNodeValue(value, roots, resolved, resolving)
		if err != nil {
			return nil, err
		}
		out.Props[key] = resolvedValue
	}

	if cacheByName && strings.TrimSpace(node.Name) != "" {
		resolved[node.Name] = cloneNode(out)
	}
	return out, nil
}

// Normalize before merging so either spelling can override an inherited value.
func normalizeEsbuildAlias(node *Node, inheritedType string) error {
	typeName := node.Type
	if typeName == "" {
		typeName = inheritedType
	}
	if formatType(typeName) != "<ESBUILD>" {
		return nil
	}
	if value, ok := node.Props["minifyident"]; ok {
		if _, duplicate := node.Props["minify_identifiers"]; duplicate {
			return nodeErrorFromNode(node, "use only one of minifyident and minify_identifiers")
		}
		delete(node.Props, "minifyident")
		node.Props["minify_identifiers"] = value
	}
	return nil
}

func resolveNodeValue(value interface{}, roots map[string]*Node, resolved map[string]*Node, resolving map[string]bool) (interface{}, error) {
	switch typed := value.(type) {
	case *Node:
		return resolveNode(typed, roots, resolved, resolving, false)
	case map[string]interface{}:
		out := cloneMap(typed)
		for key, nested := range out {
			resolvedValue, err := resolveNodeValue(nested, roots, resolved, resolving)
			if err != nil {
				return nil, err
			}
			out[key] = resolvedValue
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			resolvedValue, err := resolveNodeValue(nested, roots, resolved, resolving)
			if err != nil {
				return nil, err
			}
			out = append(out, resolvedValue)
		}
		return out, nil
	default:
		return value, nil
	}
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
	if len(overlay.nativeAPIProps) > 0 {
		if out.nativeAPIProps == nil {
			out.nativeAPIProps = make(map[string]interface{})
		}
		for key, value := range overlay.nativeAPIProps {
			out.nativeAPIProps[key] = mergeValues(out.nativeAPIProps[key], value)
		}
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
		if strings.TrimSpace(overlayNode.Inherit) != "" {
			return cloneNode(overlayNode)
		}
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
		Name:           node.Name,
		Type:           node.Type,
		Inherit:        node.Inherit,
		Props:          cloneMap(node.Props),
		Line:           node.Line,
		Column:         node.Column,
		nativeAPIProps: cloneMap(node.nativeAPIProps),
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
