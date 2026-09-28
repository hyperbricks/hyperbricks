package language

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/hyperbricks/hyperbricks/pkg/analysis"
	"github.com/hyperbricks/hyperbricks/pkg/schema"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

const staticDiagnosticSource = "hyperbricks-static"

type AnalyzerOptions struct {
	WorkspaceRoot string
	Module        string
	Config        string
}

// Analyzer provides the source-only language features. It has no runtime
// connection and is safe to use in tests and editor processes that are not
// running a HyperBricks application.
type Analyzer struct {
	workspaceRoot string
	module        string
	config        string
	types         map[string]*typeDescriptor
	typeList      []*typeDescriptor
}

type typeDescriptor struct {
	schema            schema.TypeSchema
	name              string
	aliases           []string
	registeredAliases []string
	topFields         map[string][]schema.Field
	fieldByPath       map[string]schema.Field
}

func NewAnalyzer(opts AnalyzerOptions) *Analyzer {
	root := filepath.Clean(opts.WorkspaceRoot)
	if root == "." || root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	config := strings.TrimSpace(opts.Config)
	if config == "" {
		config = "package.hyperbricks.yaml"
	}
	a := &Analyzer{
		workspaceRoot: root,
		module:        strings.TrimSpace(opts.Module),
		config:        config,
		types:         make(map[string]*typeDescriptor),
	}
	for _, typeSchema := range schema.ExtractRegistry(schema.Definitions()).Types {
		name := normalizeTypeName(typeSchema.Token)
		descriptor := &typeDescriptor{
			schema:      typeSchema,
			name:        name,
			topFields:   make(map[string][]schema.Field),
			fieldByPath: make(map[string]schema.Field),
		}
		for _, field := range typeSchema.Fields {
			top := strings.Split(field.Path, ".")[0]
			descriptor.topFields[top] = append(descriptor.topFields[top], field)
			descriptor.fieldByPath[field.Path] = field
		}
		allNames := append([]string{typeSchema.Token, name}, typeSchema.Aliases...)
		seenAlias := make(map[string]bool)
		for _, alias := range allNames {
			alias = normalizeTypeName(alias)
			if alias == "" || seenAlias[alias] {
				continue
			}
			seenAlias[alias] = true
			descriptor.aliases = append(descriptor.aliases, alias)
			a.types[alias] = descriptor
		}
		for _, alias := range typeSchema.Aliases {
			alias = normalizeTypeName(alias)
			if alias != "" && alias != descriptor.name {
				descriptor.registeredAliases = append(descriptor.registeredAliases, alias)
			}
		}
		sort.Strings(descriptor.registeredAliases)
		a.typeList = append(a.typeList, descriptor)
	}
	sort.Slice(a.typeList, func(i, j int) bool { return a.typeList[i].name < a.typeList[j].name })
	return a
}

// Diagnostics validates a single editor buffer. documents is an optional map
// of URI to unsaved content used as an overlay while imports are loaded.
func (a *Analyzer) Diagnostics(uri, text string, documents map[string]string) []Diagnostic {
	if a.isPackageConfig(uri) {
		if _, err := shared.ValidatePackageConfigBytesWithResourceReader([]byte(text), a.moduleRoot(), a.readResourceFile); err != nil {
			return []Diagnostic{diagnosticFromError(text, err, "yaml.configuration")}
		}
		return nil
	}

	path, readFile, confined := a.sourceReader(uri, text, documents)
	options := analysis.SourceOptions{ParserOptions: a.sourceParserOptions()}
	if confined {
		options.Filename, options.ReadFile = path, readFile
	}
	if a.hasEnabledPlugins() {
		options.UnknownTypeSeverity = analysis.SeverityWarning
	}
	issues := analysis.AnalyzeSource([]byte(text), options)
	diagnostics := make([]Diagnostic, 0, len(issues))
	for _, issue := range issues {
		diagnosticRange := analysisRange(text, issue.Range)
		message := issue.Message
		data := map[string]interface{}{"path": issue.Path, "file": issue.File}
		if confined && issue.File != "" && canonicalPath(issue.File) != canonicalPath(path) {
			// This API publishes diagnostics for one URI. Anchor imported-source
			// failures at the import declaration rather than applying the imported
			// file's coordinates to an unrelated buffer.
			diagnosticRange = importReferenceRange(text, issue.File)
			message = fmt.Sprintf("import %s: %s", filepath.Base(issue.File), issue.Message)
			data["imported"] = true
		}
		diagnostics = append(diagnostics, Diagnostic{
			Range:    diagnosticRange,
			Severity: int(issue.Severity),
			Code:     issue.Code,
			Source:   staticDiagnosticSource,
			Message:  message,
			Data:     data,
		})
	}
	return diagnostics
}

// Completions returns deterministic, schema-backed suggestions at position.
func (a *Analyzer) Completions(uri, text string, position Position, documents map[string]string) []CompletionItem {
	return a.yamlCompletions(uri, text, position, documents)
}

func (a *Analyzer) typeCompletions() []CompletionItem {
	type candidate struct {
		label      string
		descriptor *typeDescriptor
		alias      bool
	}
	candidates := make([]candidate, 0, len(a.typeList))
	seen := make(map[string]bool)
	for _, descriptor := range a.typeList {
		if !seen[descriptor.name] {
			seen[descriptor.name] = true
			candidates = append(candidates, candidate{label: descriptor.name, descriptor: descriptor})
		}
		for _, alias := range descriptor.registeredAliases {
			if seen[alias] {
				continue
			}
			seen[alias] = true
			candidates = append(candidates, candidate{label: alias, descriptor: descriptor, alias: true})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].label < candidates[j].label })
	items := make([]CompletionItem, 0, len(candidates))
	for index, candidate := range candidates {
		detail := fmt.Sprintf("%s · %s", candidate.descriptor.schema.Token, candidate.descriptor.schema.Category)
		if candidate.alias {
			detail = fmt.Sprintf("Alias of %s · %s", candidate.descriptor.schema.Token, candidate.descriptor.schema.Category)
		}
		items = append(items, CompletionItem{
			Label: candidate.label, Kind: CompletionItemKindClass, Detail: detail,
			Documentation: MarkupContent{Kind: "markdown", Value: candidate.descriptor.schema.Description},
			InsertText:    candidate.label, SortText: fmt.Sprintf("%04d", index),
		})
	}
	return items
}

// Hover returns schema documentation for a type token or field at position.
// documents supplies the same unsaved import overlays used by diagnostics and
// completion; callers without an index may omit it.
func (a *Analyzer) Hover(uri string, text string, position Position, documentOverlays ...map[string]string) *Hover {
	if hover := a.resolverHover(uri, text, position); hover != nil {
		return hover
	}
	var documents map[string]string
	if len(documentOverlays) > 0 {
		documents = documentOverlays[0]
	}
	var result *Hover
	visitSourceComponents(text, func(_ string, sequence *yaml.Node) {
		if result != nil {
			return
		}
		entries := sequenceEntries(sequence)
		var descriptor *typeDescriptor
		if typeNode := entries["type"]; typeNode != nil {
			descriptor = a.types[normalizeTypeName(typeNode.Value)]
			if containsPosition(text, typeNode, position) && descriptor != nil {
				value := "**" + descriptor.name + "** (`" + descriptor.schema.Token + "`)"
				if descriptor.schema.Description != "" {
					value += "\n\n" + descriptor.schema.Description
				}
				result = &Hover{Contents: MarkupContent{Kind: "markdown", Value: value}, Range: rangePointer(yamlNodeRange(text, typeNode))}
				return
			}
		} else if inheritNode := entries["inherit"]; inheritNode != nil {
			descriptor = a.types[normalizeTypeName(a.inheritedType(uri, text, inheritNode.Value, documents))]
		}
		if descriptor == nil {
			return
		}
		for _, item := range sequence.Content {
			if result != nil || item.Kind != yaml.MappingNode || len(item.Content) != 2 {
				continue
			}
			keyNode, valueNode := item.Content[0], item.Content[1]
			key := strings.TrimSpace(keyNode.Value)
			if key == "type" || key == "inherit" {
				continue
			}
			if containsPosition(text, keyNode, position) {
				result = hoverForSchemaFields(text, key, descriptor.topFields[key], keyNode)
				if result != nil {
					return
				}
			}
			if !isComponentSequence(valueNode) {
				result = hoverNestedSchemaField(text, descriptor, key, valueNode, position)
			}
		}
	})
	return result
}

func hoverNestedSchemaField(text string, descriptor *typeDescriptor, prefix string, node *yaml.Node, position Position) *Hover {
	if descriptor == nil || node == nil || isComponentSequence(node) {
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			keyNode, valueNode := node.Content[index], node.Content[index+1]
			path := prefix + "." + strings.TrimSpace(keyNode.Value)
			if containsPosition(text, keyNode, position) {
				if field, ok := descriptor.fieldByPath[path]; ok {
					return hoverForSchemaFields(text, path, []schema.Field{field}, keyNode)
				}
			}
			if schemaHasPathPrefix(descriptor, path+".") {
				if hover := hoverNestedSchemaField(text, descriptor, path, valueNode, position); hover != nil {
					return hover
				}
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if hover := hoverNestedSchemaField(text, descriptor, prefix, item, position); hover != nil {
				return hover
			}
		}
	}
	return nil
}

func schemaHasPathPrefix(descriptor *typeDescriptor, prefix string) bool {
	for path := range descriptor.fieldByPath {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func hoverForSchemaFields(text, label string, fields []schema.Field, node *yaml.Node) *Hover {
	if len(fields) == 0 {
		return nil
	}
	field := fields[0]
	kind := field.Kind
	if len(fields) > 1 {
		kind = "object"
	}
	value := "**" + label + "** · `" + kind + "`"
	if field.Required {
		value += " · required"
	}
	if field.Description != "" {
		value += "\n\n" + field.Description
	}
	if example := editorHoverExample(field.Example); example != "" {
		value += "\n\nExample: `" + strings.ReplaceAll(example, "`", "\\`") + "`"
	}
	return &Hover{Contents: MarkupContent{Kind: "markdown", Value: value}, Range: rangePointer(yamlNodeRange(text, node))}
}

func editorHoverExample(example string) string {
	example = strings.TrimSpace(example)
	if strings.HasPrefix(example, "{!{") && strings.HasSuffix(example, "}}") {
		return ""
	}
	return example
}

func (a *Analyzer) inheritTargets(uri, text string, position Position, documents map[string]string) []string {
	targets, _, definitions := a.inheritanceReferenceIndex(uri, text, documents)
	currentPath := componentPathAtPosition(text, position)
	for target := range targets {
		if inheritanceWouldCycle(currentPath, target, definitions) {
			delete(targets, target)
		}
	}
	out := make([]string, 0, len(targets))
	for target := range targets {
		out = append(out, target)
	}
	sort.Strings(out)
	return out
}

// inheritCompletionItems keeps root completion compatible with the complete
// target list while making dotted completion segment-aware. VS Code treats a
// dot as a word boundary, so inserting the full target after a typed prefix
// would otherwise duplicate that prefix (for example base.c -> base.base.card).
func inheritCompletionItems(targets []string, prefix string) []CompletionItem {
	lastDot := strings.LastIndex(prefix, ".")
	if lastDot < 0 {
		items := make([]CompletionItem, 0, len(targets))
		for _, target := range targets {
			items = append(items, CompletionItem{Label: target, Kind: CompletionItemKindReference, InsertText: target})
		}
		return items
	}

	parent := prefix[:lastDot]
	partial := prefix[lastDot+1:]
	if parent == "" {
		return nil
	}
	parentPrefix := parent + "."
	segments := make(map[string]string)
	for _, target := range targets {
		if !strings.HasPrefix(target, parentPrefix) {
			continue
		}
		remainder := strings.TrimPrefix(target, parentPrefix)
		segment := remainder
		if dot := strings.Index(segment, "."); dot >= 0 {
			segment = segment[:dot]
		}
		if segment == "" || !strings.HasPrefix(segment, partial) {
			continue
		}
		fullPath := parentPrefix + segment
		segments[segment] = fullPath
	}
	labels := make([]string, 0, len(segments))
	for label := range segments {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	items := make([]CompletionItem, 0, len(labels))
	for _, label := range labels {
		items = append(items, CompletionItem{
			Label: label, Kind: CompletionItemKindReference,
			Detail: "HyperBricks object " + segments[label], InsertText: label,
		})
	}
	return items
}

type pathCompletionSpec struct {
	directoryKey       string
	fallback           string
	includeDirectories bool
	resolver           string
	extensions         []string
}

func pathCompletionFor(descriptor *typeDescriptor, key string) (pathCompletionSpec, bool) {
	fields := descriptorFields(descriptor, key)
	if len(fields) != 1 || fields[0].Path != key || fields[0].Kind != "string" {
		return pathCompletionSpec{}, false
	}
	resourcePath := pathCompletionSpec{directoryKey: "resources", fallback: "resources", resolver: "path"}
	switch descriptor.name + "." + key {
	case "json_render.file", "styles.file", "css.file", "js.file", "image.src", "esbuild.entry":
		return resourcePath, true
	case "images.directory":
		resourcePath.includeDirectories = true
		return resourcePath, true
	case "goja_render.script":
		resourcePath.resolver = "file"
		return resourcePath, true
	case "markdown.file":
		resourcePath.resolver = "relative"
		resourcePath.extensions = []string{".md", ".markdown"}
		return resourcePath, true
	case "template.template", "json_render.template", "api_render.template", "goja_render.template", "api_fragment_render.template":
		return pathCompletionSpec{directoryKey: "templates", fallback: "templates", resolver: "template"}, true
	default:
		// Favicon and link are rendered URLs; neither has a schema-owned
		// filesystem base, so no module path completion is offered.
		return pathCompletionSpec{}, false
	}
}

func descriptorFields(descriptor *typeDescriptor, key string) []schema.Field {
	if descriptor == nil {
		return nil
	}
	return descriptor.topFields[key]
}

func (a *Analyzer) pathCompletions(spec pathCompletionSpec) []CompletionItem {
	base := a.directory(spec.directoryKey, spec.fallback)
	if _, err := a.confinedPath(base); err != nil {
		return nil
	}
	items := make([]CompletionItem, 0)
	_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if _, pathErr := a.confinedPath(path); pathErr != nil {
			return nil
		}
		relative, relErr := filepath.Rel(base, path)
		if relErr == nil && relative != "." && ((!entry.IsDir() && !spec.includeDirectories) || (entry.IsDir() && spec.includeDirectories)) {
			value := filepath.ToSlash(relative)
			if !entry.IsDir() && !completionExtensionAllowed(value, spec.extensions) {
				return nil
			}
			insertText := ""
			switch spec.resolver {
			case "path", "file":
				insertText = fmt.Sprintf("{%s: {base: %s, path: %s}}", spec.resolver, spec.directoryKey, strconv.Quote(value))
			case "template":
				insertText = fmt.Sprintf("{file: %s}", strconv.Quote(value))
			case "relative":
				insertText = strconv.Quote(value)
			default:
				return nil
			}
			items = append(items, CompletionItem{Label: value, Kind: CompletionItemKindFile, InsertText: insertText})
		}
		if len(items) >= 500 {
			return fs.SkipAll
		}
		return nil
	})
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

func completionExtensionAllowed(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}
	extension := filepath.Ext(path)
	for _, allowed := range extensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func (a *Analyzer) directory(key, fallback string) string {
	root := a.moduleRoot()
	if config := a.packageConfig(); config != nil {
		if configured := configuredDirectory(config, key); configured != "" {
			candidate := configured
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(root, filepath.FromSlash(candidate))
			}
			if confined, err := a.confinedPath(candidate); err == nil {
				return confined
			}
		}
	}
	return filepath.Join(root, fallback)
}

func configuredDirectory(config map[string]interface{}, key string) string {
	hyperbricks, _ := config["hyperbricks"].(map[string]interface{})
	directories, _ := hyperbricks["directories"].(map[string]interface{})
	configured, _ := directories[key].(string)
	return strings.TrimSpace(configured)
}

func (a *Analyzer) moduleRoot() string {
	module := strings.TrimSpace(a.module)
	if module == "" {
		return a.workspaceRoot
	}
	if filepath.IsAbs(module) {
		return filepath.Clean(module)
	}
	if module == "." || module == ".." || strings.ContainsAny(module, `/\`) {
		return filepath.Clean(filepath.Join(a.workspaceRoot, filepath.FromSlash(module)))
	}
	return filepath.Join(a.workspaceRoot, "modules", module)
}

func (a *Analyzer) packageConfig() map[string]interface{} {
	root := a.moduleRoot()
	configPath := a.config
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(root, filepath.FromSlash(configPath))
	}
	var err error
	configPath, err = a.confinedPath(configPath)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	paths := a.basePathMarkers()
	result, err := yamlparser.ProcessConfigBytes(raw, yamlparser.Options{
		Variables:                map[string]string{"module": root},
		TemplateDir:              paths.Templates,
		Paths:                    paths,
		ResourceReadFile:         a.readResourceFile,
		SkipTemplateRegistration: true,
	})
	if err != nil {
		return nil
	}
	return result.Materialized
}

func (a *Analyzer) basePathMarkers() yamlparser.PathMarkers {
	root := a.moduleRoot()
	return yamlparser.PathMarkers{
		Root:        ".",
		ModuleRoot:  filepath.Dir(root),
		Module:      root,
		HyperBricks: filepath.Join(root, "hyperbricks"),
		Templates:   filepath.Join(root, "templates"),
		Resources:   filepath.Join(root, "resources"),
		Static:      filepath.Join(root, "static"),
		Render:      filepath.Join(root, "rendered"),
	}
}

func (a *Analyzer) sourceParserOptions() yamlparser.Options {
	paths := a.basePathMarkers()
	config := a.packageConfig()
	paths.HyperBricks = a.directoryFromConfig(config, "hyperbricks", paths.HyperBricks)
	paths.Templates = a.directoryFromConfig(config, "templates", paths.Templates)
	paths.Resources = a.directoryFromConfig(config, "resources", paths.Resources)
	paths.Static = a.directoryFromConfig(config, "static", paths.Static)
	paths.Render = a.directoryFromConfig(config, "render", paths.Render)
	return yamlparser.Options{
		Variables:                editorPathVariables(paths),
		Config:                   config,
		TemplateDir:              paths.Templates,
		Paths:                    paths,
		ResourceReadFile:         a.readResourceFile,
		SkipTemplateRegistration: true,
	}
}

// Editor analysis may inspect local resources but must not follow a resolver
// outside the selected module, including through a symlink. The runtime keeps
// its existing reader; this boundary belongs specifically to source tooling.
func (a *Analyzer) readResourceFile(filename string) ([]byte, error) {
	path, err := a.confinedPath(filename)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func editorPathVariables(paths yamlparser.PathMarkers) map[string]string {
	return map[string]string{
		"module_root": paths.ModuleRoot, "root": paths.Root, "module": paths.Module,
		"resources": paths.Resources, "templates": paths.Templates, "static": paths.Static,
		"hyperbricks": paths.HyperBricks, "render": paths.Render,
	}
}

func (a *Analyzer) directoryFromConfig(config map[string]interface{}, key, fallback string) string {
	configured := configuredDirectory(config, key)
	if configured == "" {
		return fallback
	}
	candidate := configured
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(a.moduleRoot(), filepath.FromSlash(candidate))
	}
	if confined, err := a.confinedPath(candidate); err == nil {
		return confined
	}
	return fallback
}

func (a *Analyzer) hasEnabledPlugins() bool {
	config := a.packageConfig()
	plugins, _ := config["plugins"].(map[string]interface{})
	enabled, exists := plugins["enabled"]
	if !exists {
		if hyperbricks, ok := config["hyperbricks"].(map[string]interface{}); ok {
			plugins, _ = hyperbricks["plugins"].(map[string]interface{})
			enabled = plugins["enabled"]
		}
	}
	switch value := enabled.(type) {
	case []interface{}:
		return len(value) > 0
	case []string:
		return len(value) > 0
	case map[string]interface{}:
		return len(value) > 0
	case string:
		return strings.TrimSpace(value) != ""
	default:
		return false
	}
}

func (a *Analyzer) isPackageConfig(uri string) bool {
	path, err := uriToPath(uri)
	if err != nil {
		return false
	}
	configured := a.config
	if !filepath.IsAbs(configured) {
		configured = filepath.Join(a.moduleRoot(), filepath.FromSlash(configured))
	}
	return canonicalPath(path) == canonicalPath(configured)
}

// sourceReader builds the only reader used by source-graph analysis. Both
// unsaved overlays and disk reads pass through the configured source-directory
// confinement check, including symlink resolution, so an import cannot escape
// the HyperBricks source tree by changing where its path points.
func (a *Analyzer) sourceReader(uri, text string, documents map[string]string) (string, func(string) ([]byte, error), bool) {
	path, err := uriToPath(uri)
	if err != nil || path == "" {
		return "", nil, false
	}
	path, err = a.confinedSourcePath(path)
	if err != nil {
		return "", nil, false
	}
	overlays := make(map[string][]byte, len(documents)+1)
	for documentURI, content := range documents {
		documentPath, pathErr := uriToPath(documentURI)
		if pathErr != nil || documentPath == "" {
			continue
		}
		if documentPath, pathErr = a.confinedSourcePath(documentPath); pathErr == nil {
			overlays[canonicalPath(documentPath)] = []byte(content)
		}
	}
	overlays[canonicalPath(path)] = []byte(text)
	readFile := func(filename string) ([]byte, error) {
		confined, pathErr := a.confinedSourcePath(filename)
		if pathErr != nil {
			return nil, pathErr
		}
		if content, ok := overlays[canonicalPath(confined)]; ok {
			return append([]byte(nil), content...), nil
		}
		return os.ReadFile(confined)
	}
	return path, readFile, true
}

func (a *Analyzer) confinedPath(path string) (string, error) {
	return confinedToRoot(a.moduleRoot(), path, "selected module root")
}

func (a *Analyzer) confinedSourcePath(path string) (string, error) {
	return confinedToRoot(a.directory("hyperbricks", "hyperbricks"), path, "configured HyperBricks source directory")
}

func confinedToRoot(root, path, label string) (string, error) {
	root = canonicalPath(root)
	candidate := canonicalPath(path)
	if !pathWithin(root, candidate) {
		return "", fmt.Errorf("path %q escapes %s %q", candidate, label, root)
	}
	resolvedRoot, err := resolvePathWithExistingAncestor(root)
	if err != nil {
		return "", fmt.Errorf("resolve %s %q: %w", label, root, err)
	}
	resolvedCandidate, err := resolvePathWithExistingAncestor(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve source path %q: %w", candidate, err)
	}
	if !pathWithin(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("path %q resolves outside %s %q", candidate, label, root)
	}
	return candidate, nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolvePathWithExistingAncestor(path string) (string, error) {
	path = canonicalPath(path)
	current := path
	missing := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func (a *Analyzer) reachableSources(uri, text string, documents map[string]string) []string {
	sources := []string{text}
	path, readFile, confined := a.sourceReader(uri, text, documents)
	if !confined {
		return sources
	}
	visited := map[string]bool{canonicalPath(path): true}
	var walk func(string, string)
	walk = func(filename, source string) {
		for _, imported := range sourceImports(source) {
			importPath := filepath.FromSlash(imported)
			if !filepath.IsAbs(importPath) {
				importPath = filepath.Join(filepath.Dir(filename), importPath)
			}
			importPath, err := a.confinedSourcePath(importPath)
			if err != nil || visited[canonicalPath(importPath)] {
				continue
			}
			visited[canonicalPath(importPath)] = true
			raw, err := readFile(importPath)
			if err != nil {
				continue
			}
			importSource := string(raw)
			sources = append(sources, importSource)
			walk(importPath, importSource)
		}
	}
	walk(path, text)
	return sources
}

func sourceImports(text string) []string {
	document, err := decodeYAMLDocument(text)
	if err != nil {
		return nil
	}
	body := yamlDocumentBody(document)
	if body == nil || body.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(body.Content); index += 2 {
		if strings.TrimSpace(body.Content[index].Value) != "imports" {
			continue
		}
		value := body.Content[index+1]
		imports := make([]string, 0)
		switch value.Kind {
		case yaml.ScalarNode:
			if strings.TrimSpace(value.Value) != "" {
				imports = append(imports, strings.TrimSpace(value.Value))
			}
		case yaml.SequenceNode:
			for _, item := range value.Content {
				if item.Kind == yaml.ScalarNode && strings.TrimSpace(item.Value) != "" {
					imports = append(imports, strings.TrimSpace(item.Value))
				}
			}
		}
		return imports
	}
	return nil
}

func importReferenceRange(text, importedFile string) Range {
	document, err := decodeYAMLDocument(text)
	if err != nil {
		return rangeAtLineColumn(text, 1, 1, 1)
	}
	body := yamlDocumentBody(document)
	if body == nil || body.Kind != yaml.MappingNode {
		return rangeAtLineColumn(text, 1, 1, 1)
	}
	base := filepath.Base(importedFile)
	for index := 0; index+1 < len(body.Content); index += 2 {
		key, value := body.Content[index], body.Content[index+1]
		if strings.TrimSpace(key.Value) != "imports" {
			continue
		}
		if value.Kind == yaml.ScalarNode && filepath.Base(filepath.FromSlash(value.Value)) == base {
			return yamlNodeRange(text, value)
		}
		if value.Kind == yaml.SequenceNode {
			for _, item := range value.Content {
				if item.Kind == yaml.ScalarNode && filepath.Base(filepath.FromSlash(item.Value)) == base {
					return yamlNodeRange(text, item)
				}
			}
		}
		return yamlNodeRange(text, key)
	}
	return rangeAtLineColumn(text, 1, 1, 1)
}

func decodeYAMLDocument(text string) (*yaml.Node, error) {
	if strings.TrimSpace(text) == "" {
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}, nil
	}
	decoder := yaml.NewDecoder(strings.NewReader(text))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("exactly one YAML document is required")
		}
		return nil, err
	}
	return &document, nil
}

func yamlDocumentBody(root *yaml.Node) *yaml.Node {
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) != 1 {
		return nil
	}
	return root.Content[0]
}

func sequenceEntries(sequence *yaml.Node) map[string]*yaml.Node {
	entries := make(map[string]*yaml.Node)
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return entries
	}
	for _, item := range sequence.Content {
		if item.Kind == yaml.MappingNode && len(item.Content) == 2 {
			entries[strings.TrimSpace(item.Content[0].Value)] = item.Content[1]
		}
	}
	return entries
}

func isComponentSequence(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range node.Content {
		if item.Kind == yaml.MappingNode && len(item.Content) == 2 {
			switch strings.TrimSpace(item.Content[0].Value) {
			case "type", "inherit":
				return true
			}
		}
	}
	return false
}

func visitMountedYAML(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	if isComponentSequence(node) {
		visit(node)
		return
	}
	for _, child := range node.Content {
		visitMountedYAML(child, visit)
	}
}

type targetType struct {
	typeName string
	inherit  string
}

func (a *Analyzer) inheritedType(uri, text, inherit string, documents map[string]string) string {
	_, types, _ := a.inheritanceReferenceIndex(uri, text, documents)
	return types[strings.TrimSpace(inherit)]
}

func (a *Analyzer) inheritanceReferenceIndex(uri, text string, documents map[string]string) (map[string]bool, map[string]string, map[string]targetType) {
	document := &yamlparser.Document{}
	definitions := make(map[string]targetType)
	for _, source := range a.reachableSources(uri, text, documents) {
		sourceDocument, err := parseInheritanceReferenceSource(source)
		if err != nil {
			continue
		}
		for _, root := range sourceDocument.Roots {
			if root == nil || strings.TrimSpace(root.Name) == "" {
				continue
			}
			document.Roots = append(document.Roots, root)
			collectInheritanceDefinitions(root.Name, root, definitions)
		}
	}

	targets := make(map[string]bool)
	types := make(map[string]string)
	for _, target := range document.InheritanceTargets() {
		targets[target.Path] = true
		if strings.TrimSpace(target.Type) != "" {
			types[target.Path] = target.Type
		}
	}
	return targets, types, definitions
}

func parseInheritanceReferenceSource(source string) (*yamlparser.Document, error) {
	options := yamlparser.ParseOptions{AllowUnknownTypes: true, RecoverDuplicateChildren: true}
	document, err := yamlparser.ParseBytesWithOptions([]byte(source), options)
	if err == nil {
		return document, nil
	}
	// A bare dash is the normal transient state while requesting field
	// completion. It is not a component entry yet, so omit it from the
	// reference index while preserving all surrounding source and line breaks.
	lines := splitLines(source)
	changed := false
	for index, line := range lines {
		if strings.TrimSpace(line) == "-" {
			lines[index] = ""
			changed = true
		}
	}
	if !changed {
		return nil, err
	}
	return yamlparser.ParseBytesWithOptions([]byte(strings.Join(lines, "\n")), options)
}

func collectInheritanceDefinitions(path string, node *yamlparser.Node, definitions map[string]targetType) {
	if node == nil || path == "" {
		return
	}
	definitions[path] = targetType{typeName: node.Type, inherit: node.Inherit}
	for _, child := range node.Children {
		collectInheritanceDefinitions(joinTargetPath(path, child.Name), child, definitions)
	}
}

func visitSourceComponents(text string, visit func(string, *yaml.Node)) {
	root, err := decodeYAMLDocument(text)
	if err != nil {
		return
	}
	body := yamlDocumentBody(root)
	if body == nil || body.Kind != yaml.MappingNode {
		return
	}
	var walkComponent func(string, *yaml.Node)
	var walkMounted func(string, *yaml.Node)
	walkComponent = func(path string, sequence *yaml.Node) {
		visit(path, sequence)
		for _, item := range sequence.Content {
			if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
				continue
			}
			key := strings.TrimSpace(item.Content[0].Value)
			if key == "type" || key == "inherit" {
				continue
			}
			walkMounted(joinTargetPath(path, key), item.Content[1])
		}
	}
	walkMounted = func(path string, node *yaml.Node) {
		if node == nil {
			return
		}
		if isComponentSequence(node) {
			walkComponent(path, node)
			return
		}
		switch node.Kind {
		case yaml.MappingNode:
			for index := 0; index+1 < len(node.Content); index += 2 {
				key := strings.TrimSpace(node.Content[index].Value)
				walkMounted(joinTargetPath(path, key), node.Content[index+1])
			}
		case yaml.SequenceNode:
			for index, item := range node.Content {
				walkMounted(fmt.Sprintf("%s[%d]", path, index), item)
			}
		}
	}
	for index := 0; index+1 < len(body.Content); index += 2 {
		key, value := strings.TrimSpace(body.Content[index].Value), body.Content[index+1]
		if key != "imports" && key != "vars" && isComponentSequence(value) {
			walkComponent(key, value)
		}
	}
}

func joinTargetPath(base, key string) string {
	key = strings.TrimSpace(key)
	if base == "" {
		return key
	}
	if key == "" {
		return base
	}
	return base + "." + key
}

func componentPathAtPosition(text string, position Position) string {
	result := ""
	visitSourceComponents(text, func(path string, sequence *yaml.Node) {
		if result != "" {
			return
		}
		for _, item := range sequence.Content {
			if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
				continue
			}
			key := item.Content[0]
			if strings.TrimSpace(key.Value) == "inherit" && key.Line-1 == position.Line {
				result = path
				return
			}
		}
	})
	return result
}

func inheritanceWouldCycle(current, candidate string, definitions map[string]targetType) bool {
	if current == "" || candidate == "" {
		return false
	}
	if relatedTargetPaths(current, candidate) {
		return true
	}
	seen := make(map[string]bool)
	for candidate != "" && !seen[candidate] {
		seen[candidate] = true
		next := strings.TrimSpace(definitions[candidate].inherit)
		if relatedTargetPaths(current, next) {
			return true
		}
		candidate = next
	}
	return false
}

func relatedTargetPaths(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return left == right || strings.HasPrefix(left, right+".") || strings.HasPrefix(right, left+".") ||
		strings.HasPrefix(left, right+"[") || strings.HasPrefix(right, left+"[")
}

func fieldSnippet(key string, fields []schema.Field, indent int) string {
	if snippet, ok := nestedFieldSnippet(key, fields, indent); ok {
		return snippet
	}
	if len(fields) == 0 {
		return key + ": ${1}"
	}
	if fields[0].Kind == "map" {
		return key + ":\n" + strings.Repeat(" ", indent+4) + "${1:key}: ${2:value}"
	}
	switch fields[0].Kind {
	case "bool":
		return key + ": ${1|true,false|}"
	case "int", "uint", "float":
		return key + ": ${1:0}"
	case "list":
		return key + ":\n" + strings.Repeat(" ", indent+4) + "- ${1:value}"
	default:
		return key + ": ${1:value}"
	}
}

type snippetFieldNode struct {
	field    *schema.Field
	children map[string]*snippetFieldNode
}

// nestedFieldSnippet turns schema leaf paths such as response.status and
// response.headers into an insertable YAML object. It deliberately emits only
// paths present in the schema; generic placeholder keys belong only inside
// schema-declared dynamic maps.
func nestedFieldSnippet(key string, fields []schema.Field, indent int) (string, bool) {
	root := &snippetFieldNode{children: make(map[string]*snippetFieldNode)}
	leafCount := 0
	for index := range fields {
		parts := strings.Split(fields[index].Path, ".")
		if len(parts) < 2 || parts[0] != key {
			continue
		}
		node := root
		for _, part := range parts[1:] {
			if part == "" {
				continue
			}
			if node.children[part] == nil {
				node.children[part] = &snippetFieldNode{children: make(map[string]*snippetFieldNode)}
			}
			node = node.children[part]
		}
		node.field = &fields[index]
		leafCount++
	}
	if leafCount == 0 {
		return "", false
	}
	placeholder := 1
	lines := []string{key + ":"}
	lines = append(lines, renderSnippetFields(root, indent+4, &placeholder)...)
	return strings.Join(lines, "\n"), true
}

func renderSnippetFields(node *snippetFieldNode, indent int, placeholder *int) []string {
	keys := make([]string, 0, len(node.children))
	for key := range node.children {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	padding := strings.Repeat(" ", indent)
	for _, key := range keys {
		child := node.children[key]
		if len(child.children) > 0 {
			lines = append(lines, padding+key+":")
			lines = append(lines, renderSnippetFields(child, indent+2, placeholder)...)
			continue
		}
		if child.field == nil {
			continue
		}
		switch child.field.Kind {
		case "map":
			lines = append(lines, padding+key+":")
			lines = append(lines, strings.Repeat(" ", indent+2)+fmt.Sprintf("${%d:key}: ${%d:value}", *placeholder, *placeholder+1))
			*placeholder += 2
		case "list":
			lines = append(lines, padding+key+":")
			lines = append(lines, strings.Repeat(" ", indent+2)+fmt.Sprintf("- ${%d:value}", *placeholder))
			*placeholder++
		case "bool":
			lines = append(lines, padding+key+fmt.Sprintf(": ${%d|true,false|}", *placeholder))
			*placeholder++
		case "int", "uint", "float":
			lines = append(lines, padding+key+fmt.Sprintf(": ${%d:0}", *placeholder))
			*placeholder++
		default:
			lines = append(lines, padding+key+fmt.Sprintf(": ${%d:value}", *placeholder))
			*placeholder++
		}
	}
	return lines
}

func resolverCompletions(indent int) []CompletionItem {
	type resolverSpec struct {
		label  string
		detail string
		text   string
	}
	continuation := strings.Repeat(" ", indent+2)
	specs := []resolverSpec{
		{label: "config", detail: "Resolve a package configuration value", text: "config:\n" + continuation + "path: ${1:server.port}\n" + continuation + "default: ${2:value}"},
		{label: "env", detail: "Resolve an environment value", text: "env: ${1:NAME}"},
		{label: "file", detail: "Read a file through a path specification", text: "file:\n" + continuation + "base: ${1|module,resources,templates,static,hyperbricks,render,module_root,root|}\n" + continuation + "path: ${2:relative/path}"},
		{label: "format", detail: "Format values deterministically", text: "format: ${1:%s}\n" + strings.Repeat(" ", indent) + "args:\n" + continuation + "- ${2:value}"},
		{label: "path", detail: "Resolve a path from a named base", text: "path:\n" + continuation + "base: ${1|module,resources,templates,static,hyperbricks,render,module_root,root|}\n" + continuation + "path: ${2:relative/path}"},
		{label: "var", detail: "Resolve a HyperBricks variable", text: "var: ${1:name}"},
	}
	items := make([]CompletionItem, 0, len(specs))
	for index, spec := range specs {
		items = append(items, CompletionItem{
			Label: spec.label, Kind: CompletionItemKindField, Detail: spec.detail,
			InsertText: spec.text, InsertTextFormat: InsertTextFormatSnippet,
			SortText: fmt.Sprintf("%04d", index),
		})
	}
	return items
}

func pathBaseCompletions() []CompletionItem {
	bases := []string{"hyperbricks", "module", "module_root", "render", "resources", "root", "static", "templates"}
	items := make([]CompletionItem, 0, len(bases))
	for index, base := range bases {
		items = append(items, CompletionItem{
			Label: base, Kind: CompletionItemKindReference, Detail: "HyperBricks path base",
			InsertText: base, SortText: fmt.Sprintf("%04d", index),
		})
	}
	return items
}

func authoringSlotOwnsCompletion(descriptor *typeDescriptor, key string) bool {
	if descriptor == nil || descriptor.schema.Authoring == nil {
		return false
	}
	for _, slot := range descriptor.schema.Authoring.Slots {
		if slot.Key == key && !slot.PathKeyRequired {
			return true
		}
	}
	return false
}

func childCompletions(descriptor *typeDescriptor, existing map[string]bool, indent int) []CompletionItem {
	if descriptor == nil || descriptor.schema.ChildModel == schema.ChildModelNone || descriptor.schema.ChildModel == schema.ChildModelValues {
		return nil
	}
	items := make([]CompletionItem, 0)
	if descriptor.schema.Authoring != nil && len(descriptor.schema.Authoring.Slots) > 0 {
		for _, slot := range descriptor.schema.Authoring.Slots {
			if slot.PathKeyRequired || slot.Key == "" || existing[slot.Key] {
				continue
			}
			childType := "html"
			if slot.Model == string(schema.ChildModelHead) {
				childType = "css"
				if slot.Key == "head" && !slot.Default {
					childType = "head"
				}
			}
			items = append(items, CompletionItem{
				Label: slot.Key, Kind: CompletionItemKindField,
				Detail:           "HyperBricks " + slot.Model + " child slot",
				Documentation:    MarkupContent{Kind: "markdown", Value: slot.Description},
				InsertText:       slot.Key + ":\n" + strings.Repeat(" ", indent+4) + "- type: ${1:" + childType + "}",
				InsertTextFormat: InsertTextFormatSnippet,
			})
		}
		return items
	}
	if descriptor.schema.ChildModel == schema.ChildModelTree {
		items = append(items, CompletionItem{
			Label: "child", Kind: CompletionItemKindField, Detail: "HyperBricks tree child",
			InsertText:       "${1:child}:\n" + strings.Repeat(" ", indent+4) + "- type: ${2:html}",
			InsertTextFormat: InsertTextFormatSnippet,
		})
	}
	return items
}

func diagnosticFromError(text string, err error, code string) Diagnostic {
	line, column := errorPosition(err)
	message := err.Error()
	return Diagnostic{
		Range:    rangeAtLineColumn(text, line, column, 1),
		Severity: DiagnosticSeverityError,
		Code:     code,
		Source:   staticDiagnosticSource,
		Message:  message,
	}
}

var genericYAMLPosition = regexp.MustCompile(`line ([0-9]+)(?:(?::|, column )([0-9]+))?`)

func errorPosition(err error) (int, int) {
	var sourceErr *yamlparser.SourceError
	if errors.As(err, &sourceErr) && sourceErr.Line > 0 {
		return sourceErr.Line, maxInt(1, sourceErr.Column)
	}
	if match := genericYAMLPosition.FindStringSubmatch(err.Error()); len(match) > 1 {
		line, _ := strconv.Atoi(match[1])
		column := 1
		if len(match) > 2 && match[2] != "" {
			column, _ = strconv.Atoi(match[2])
		}
		return maxInt(1, line), maxInt(1, column)
	}
	return 1, 1
}

func rangeAtLineColumn(text string, line, column, length int) Range {
	lines := splitLines(text)
	lineIndex := maxInt(0, line-1)
	if len(lines) > 0 && lineIndex >= len(lines) {
		lineIndex = len(lines) - 1
	}
	character := maxInt(0, column-1)
	if len(lines) > 0 {
		character = utf16ColumnForRuneColumn(lines[lineIndex], character)
	}
	return Range{Start: Position{Line: lineIndex, Character: character}, End: Position{Line: lineIndex, Character: character + maxInt(1, length)}}
}

func analysisRange(text string, sourceRange analysis.Range) Range {
	start := rangeAtLineColumn(text, sourceRange.Start.Line, sourceRange.Start.Column, 1).Start
	end := rangeAtLineColumn(text, sourceRange.End.Line, sourceRange.End.Column, 1).Start
	if end.Line == start.Line && end.Character <= start.Character {
		end.Character = start.Character + 1
	}
	return Range{Start: start, End: end}
}

func yamlNodeRange(text string, node *yaml.Node) Range {
	if node == nil {
		return Range{}
	}
	lineIndex := maxInt(0, node.Line-1)
	runeColumn := maxInt(0, node.Column-1)
	character := runeColumn
	lines := splitLines(text)
	if lineIndex < len(lines) {
		character = utf16ColumnForRuneColumn(lines[lineIndex], runeColumn)
	}
	length := utf16Length(node.Value)
	if length == 0 {
		length = 1
	}
	return Range{
		Start: Position{Line: lineIndex, Character: character},
		End:   Position{Line: lineIndex, Character: character + length},
	}
}

func containsPosition(text string, node *yaml.Node, position Position) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}
	rangeValue := yamlNodeRange(text, node)
	return position.Line == rangeValue.Start.Line && position.Character >= rangeValue.Start.Character && position.Character <= rangeValue.End.Character
}

func normalizeTypeName(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(unquoteYAMLScalar(value)), "<>"))
	return strings.ReplaceAll(value, "-", "_")
}

func unquoteYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func rangePointer(value Range) *Range { return &value }

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(text, "\n")
}

func leadingSpaces(value string) int {
	count := 0
	for _, char := range value {
		if char != ' ' {
			break
		}
		count++
	}
	return count
}

func utf16Length(value string) int { return len(utf16.Encode([]rune(value))) }

func utf16ColumnForRuneColumn(line string, runeColumn int) int {
	runes := []rune(line)
	if runeColumn > len(runes) {
		runeColumn = len(runes)
	}
	return len(utf16.Encode(runes[:runeColumn]))
}

func byteIndexForUTF16(value string, column int) int {
	if column <= 0 {
		return 0
	}
	units := 0
	for index, char := range value {
		width := 1
		if char > 0xffff {
			width = 2
		}
		if units+width > column {
			return index
		}
		units += width
		if units == column {
			return index + utf8.RuneLen(char)
		}
	}
	return len(value)
}

func canonicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absolute)
}

func canonicalURIPath(uri string) string {
	path, err := uriToPath(uri)
	if err != nil {
		return ""
	}
	return canonicalPath(path)
}

func uriToPath(value string) (string, error) {
	return uriToPathForOS(value, runtime.GOOS)
}

func uriToPathForOS(value, goos string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" {
		if goos == "windows" {
			return strings.ReplaceAll(value, "/", `\`), nil
		}
		return filepath.Clean(value), nil
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported document URI scheme %q", parsed.Scheme)
	}
	if parsed.Opaque != "" {
		return "", fmt.Errorf("opaque file URI is not supported")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("document file URI must not contain a query or fragment")
	}
	uriPath := parsed.Path // net/url has already percent-decoded Path.
	localHost := parsed.Host == "" || strings.EqualFold(parsed.Host, "localhost")
	if goos == "windows" {
		if !localHost {
			return `\\` + parsed.Host + strings.ReplaceAll(uriPath, "/", `\`), nil
		}
		if len(uriPath) >= 3 && uriPath[0] == '/' && isASCIILetter(uriPath[1]) && uriPath[2] == ':' {
			uriPath = uriPath[1:]
		}
		return strings.ReplaceAll(uriPath, "/", `\`), nil
	}
	if !localHost {
		uriPath = "//" + parsed.Host + uriPath
	}
	return filepath.FromSlash(uriPath), nil
}

func pathToURI(path string) string {
	absolute := canonicalPath(path)
	return pathToURIForOS(absolute, runtime.GOOS)
}

func pathToURIForOS(path, goos string) string {
	uriPath := strings.ReplaceAll(path, `\`, "/")
	if goos == "windows" {
		if strings.HasPrefix(uriPath, "//") {
			withoutPrefix := strings.TrimPrefix(uriPath, "//")
			host, sharePath, found := strings.Cut(withoutPrefix, "/")
			if found && host != "" {
				return (&url.URL{Scheme: "file", Host: host, Path: "/" + sharePath}).String()
			}
		}
		if len(uriPath) >= 2 && isASCIILetter(uriPath[0]) && uriPath[1] == ':' {
			uriPath = "/" + uriPath
		}
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String()
}

func isASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
