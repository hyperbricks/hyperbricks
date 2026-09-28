// Package analysis provides editor-neutral HyperBricks source validation.
package analysis

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hyperbricks/hyperbricks/pkg/schema"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type Severity int

const (
	SeverityError       Severity = 1
	SeverityWarning     Severity = 2
	SeverityInformation Severity = 3
)

// Position is one-based and uses Unicode code points. Protocol adapters are
// responsible for converting it to their own position encoding.
type Position struct {
	Line   int
	Column int
}

type Range struct {
	Start Position
	End   Position
}

type Issue struct {
	Code     string
	Severity Severity
	Message  string
	File     string
	Path     string
	Range    Range
}

type SourceOptions struct {
	Filename            string
	ReadFile            func(string) ([]byte, error)
	UnknownTypeSeverity Severity
	// ParserOptions carries the selected module's resolver inputs (notably
	// PathMarkers) into materialization. AnalyzeSource still owns the strict
	// authoring flags below so callers cannot accidentally disable them.
	ParserOptions yamlparser.Options
}

type componentType struct {
	Name        string
	Schema      schema.TypeSchema
	TopFields   map[string][]schema.Field
	FieldByPath map[string]schema.Field
}

// AnalyzeSource returns all recoverable parser and native-schema issues for
// one source buffer. When Filename and ReadFile are supplied, imports are
// resolved through ReadFile so callers can overlay unsaved editor documents.
func AnalyzeSource(input []byte, opts SourceOptions) []Issue {
	root, err := decodeYAML(input)
	if err != nil {
		return []Issue{issueFromError(err, "yaml.syntax")}
	}

	parserOptions := opts.ParserOptions
	parserOptions.AllowUnknownTypes = true
	parserOptions.RecoverDuplicateChildren = true
	var document *yamlparser.Document
	if strings.TrimSpace(opts.Filename) != "" && opts.ReadFile != nil {
		document, err = yamlparser.LoadFileWithReader(opts.Filename, parserOptions, opts.ReadFile)
	} else {
		document, err = yamlparser.ParseBytesWithOptions(input, yamlparser.ParseOptions{
			AllowUnknownTypes:        true,
			RecoverDuplicateChildren: true,
		})
	}
	if err != nil {
		return []Issue{issueFromError(err, parserIssueCode(err))}
	}

	issues := make([]Issue, 0, len(document.Diagnostics))
	for _, diagnostic := range document.Diagnostics {
		severity := SeverityWarning
		if diagnostic.Code == "duplicate_child_name" {
			severity = SeverityError
		}
		issues = append(issues, Issue{
			Code: diagnostic.Code, Severity: severity, Message: diagnostic.Message,
			File: diagnostic.Source, Path: diagnostic.Path,
			Range: rangeAt(diagnostic.Line, diagnostic.Column, utf8.RuneCountInString(diagnostic.OriginalName)),
		})
	}

	materialized, _, err := document.MaterializeWithOptions(parserOptions)
	if err != nil {
		issues = append(issues, issueFromError(err, parserIssueCode(err)))
		return issues
	}
	registry := sourceSchemaRegistry()
	unknownSeverity := opts.UnknownTypeSeverity
	if unknownSeverity == 0 {
		unknownSeverity = SeverityError
	}
	sources, sourceErr := reachableSourceDocuments(input, root, opts.Filename, opts.ReadFile, parserOptions)
	if sourceErr != nil {
		issues = append(issues, issueFromError(sourceErr, parserIssueCode(sourceErr)))
		return issues
	}
	for _, source := range sources {
		body := documentBody(source.root)
		if body == nil || body.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index+1 < len(body.Content); index += 2 {
			key, value := body.Content[index], body.Content[index+1]
			if key.Value == "imports" || key.Value == "vars" || value.Kind != yaml.SequenceNode {
				continue
			}
			effective, _ := materialized[key.Value].(map[string]interface{})
			sourceIssues := analyzeComponent(registry, key.Value, value, effective, unknownSeverity)
			for issueIndex := range sourceIssues {
				if sourceIssues[issueIndex].File == "" {
					sourceIssues[issueIndex].File = source.file
				}
			}
			issues = append(issues, sourceIssues...)
		}
	}
	return issues
}

type sourceDocument struct {
	file string
	root *yaml.Node
}

// reachableSourceDocuments reuses the caller-provided reader for every import.
// The language layer owns filesystem confinement and overlays; analysis never
// bypasses that boundary with a direct disk read.
func reachableSourceDocuments(input []byte, root *yaml.Node, filename string, readFile func(string) ([]byte, error), parserOptions yamlparser.Options) ([]sourceDocument, error) {
	currentFile := ""
	if strings.TrimSpace(filename) != "" {
		absolute, err := filepath.Abs(filename)
		if err != nil {
			return nil, err
		}
		currentFile = filepath.Clean(absolute)
	}
	sources := []sourceDocument{{file: currentFile, root: root}}
	if currentFile == "" || readFile == nil {
		return sources, nil
	}
	visited := map[string]bool{currentFile: true}
	var walk func(string, *yaml.Node) error
	walk = func(sourceFile string, sourceRoot *yaml.Node) error {
		for _, importName := range documentImports(sourceRoot) {
			importFile := filepath.FromSlash(importName)
			if !filepath.IsAbs(importFile) {
				importFile = filepath.Join(filepath.Dir(sourceFile), importFile)
			}
			absolute, err := filepath.Abs(importFile)
			if err != nil {
				return err
			}
			importFile = filepath.Clean(absolute)
			if visited[importFile] {
				continue
			}
			visited[importFile] = true
			raw, err := readFile(importFile)
			if err != nil {
				return err
			}
			preprocessed, err := yamlparser.PreprocessBytes(raw, parserOptions)
			if err != nil {
				return err
			}
			importRoot, err := decodeYAML(preprocessed)
			if err != nil {
				return err
			}
			sources = append(sources, sourceDocument{file: importFile, root: importRoot})
			if err := walk(importFile, importRoot); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(currentFile, root); err != nil {
		return nil, err
	}
	return sources, nil
}

func documentImports(document *yaml.Node) []string {
	body := documentBody(document)
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
			if name := strings.TrimSpace(value.Value); name != "" {
				imports = append(imports, name)
			}
		case yaml.SequenceNode:
			for _, item := range value.Content {
				if item.Kind == yaml.ScalarNode {
					if name := strings.TrimSpace(item.Value); name != "" {
						imports = append(imports, name)
					}
				}
			}
		}
		return imports
	}
	return nil
}

func analyzeComponent(registry map[string]*componentType, path string, sequence *yaml.Node, effective map[string]interface{}, unknownSeverity Severity) []Issue {
	entries := sequenceEntries(sequence)
	typeNode := entries["type"]
	typeName := ""
	if typeNode != nil && typeNode.Kind == yaml.ScalarNode {
		typeName = normalizeType(typeNode.Value)
	} else if token, ok := effective["@type"].(string); ok {
		typeName = normalizeType(token)
	}
	descriptor := registry[typeName]
	issues := make([]Issue, 0)
	if typeNode != nil && descriptor == nil {
		message := fmt.Sprintf("unknown component type %q", typeNode.Value)
		if unknownSeverity == SeverityWarning {
			message += "; its schema may be owned by an enabled plugin"
		}
		issues = append(issues, Issue{
			Code: "component.unknown_type", Severity: unknownSeverity,
			Message: message, Path: path, Range: nodeRange(typeNode),
		})
	}
	if descriptor != nil {
		issues = append(issues, validateChildPlacements(descriptor, path, sequence, effective)...)
	}

	for _, item := range sequence.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			continue
		}
		keyNode, valueNode := item.Content[0], item.Content[1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "type" || key == "inherit" {
			continue
		}
		childValue := mapValue(effective, key)
		if isEffectiveComponentSequence(valueNode, childValue) {
			childEffective, _ := childValue.(map[string]interface{})
			issues = append(issues, analyzeComponent(registry, path+"."+key, valueNode, childEffective, unknownSeverity)...)
			continue
		}
		if descriptor == nil {
			continue
		}
		if _, ok := descriptor.TopFields[key]; !ok {
			issues = append(issues, Issue{
				Code: "component.unsupported_field", Severity: SeverityError,
				Message: fmt.Sprintf("%s does not support field %q", descriptor.Name, key), Path: path + "." + key, Range: nodeRange(keyNode),
			})
			continue
		}
		issues = append(issues, validateNestedSchemaFields(descriptor, path, key, valueNode)...)
		issues = append(issues, analyzeMounted(registry, path+"."+key, valueNode, mapValue(effective, key), unknownSeverity)...)
	}

	if descriptor != nil {
		for _, field := range descriptor.Schema.Fields {
			if !field.Required || strings.Contains(field.Path, ".") {
				continue
			}
			value, present := lookupMapPath(effective, field.Path)
			if present && !isZero(value) {
				continue
			}
			anchor := typeNode
			if anchor == nil && len(sequence.Content) > 0 {
				anchor = sequence.Content[0]
			}
			issues = append(issues, Issue{
				Code: "component.missing_required_field", Severity: SeverityError,
				Message: fmt.Sprintf("%s requires field %q", descriptor.Name, field.Path), Path: path + "." + field.Path, Range: nodeRange(anchor),
			})
		}
	}
	return issues
}

func validateChildPlacements(descriptor *componentType, componentPath string, sequence *yaml.Node, effective map[string]interface{}) []Issue {
	issues := make([]Issue, 0)
	var walk func([]string, *yaml.Node, *yaml.Node, interface{})
	walk = func(relative []string, node, anchor *yaml.Node, effectiveValue interface{}) {
		if node == nil {
			return
		}
		if isEffectiveComponentSequence(node, effectiveValue) {
			if !childPlacementAllowed(descriptor.Schema, relative) {
				placement := strings.Join(relative, ".")
				issues = append(issues, Issue{
					Code: "component.invalid_child_placement", Severity: SeverityError,
					Message: fmt.Sprintf("%s does not accept a component child at %q", descriptor.Name, placement),
					Path:    componentPath + "." + placement, Range: nodeRange(anchor),
				})
			}
			// The mounted component validates its own descendants separately.
			return
		}
		switch node.Kind {
		case yaml.MappingNode:
			effectiveMap, _ := effectiveValue.(map[string]interface{})
			for index := 0; index+1 < len(node.Content); index += 2 {
				keyNode, valueNode := node.Content[index], node.Content[index+1]
				key := strings.TrimSpace(keyNode.Value)
				walk(appendPath(relative, key), valueNode, keyNode, effectiveMap[key])
			}
		case yaml.SequenceNode:
			effectiveList, _ := effectiveValue.([]interface{})
			for index, item := range node.Content {
				var itemValue interface{}
				if index < len(effectiveList) {
					itemValue = effectiveList[index]
				}
				walk(relative, item, anchor, itemValue)
			}
		}
	}
	for _, item := range sequence.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			continue
		}
		keyNode, valueNode := item.Content[0], item.Content[1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "type" || key == "inherit" {
			continue
		}
		walk([]string{key}, valueNode, keyNode, mapValue(effective, key))
	}
	return issues
}

func childPlacementAllowed(typeSchema schema.TypeSchema, relative []string) bool {
	if len(relative) == 0 || typeSchema.ChildModel == schema.ChildModelNone || typeSchema.Authoring == nil {
		return false
	}
	slots := typeSchema.Authoring.Slots
	if typeSchema.ChildModel == schema.ChildModelValues {
		for _, slot := range slots {
			if slot.Model != string(schema.ChildModelValues) || relative[0] != slot.Key {
				continue
			}
			if slot.PathKeyRequired {
				return len(relative) == 2
			}
			return len(relative) == 1
		}
		return false
	}
	// Tree/head children are source-order entries at the component level. A
	// named non-default slot (Hypermedia's head) is also represented there.
	if len(relative) != 1 {
		return false
	}
	for _, slot := range slots {
		if slot.Key == relative[0] {
			return !slot.PathKeyRequired
		}
	}
	for _, slot := range slots {
		if slot.Default && !slot.PathKeyRequired {
			return true
		}
	}
	return false
}

func appendPath(path []string, key string) []string {
	out := make([]string, 0, len(path)+1)
	out = append(out, path...)
	if key != "" {
		out = append(out, key)
	}
	return out
}

// validateNestedSchemaFields checks only mappings whose ownership is explicit
// in the native schema. Dynamic maps remain intentionally open, and resolver
// expressions are values rather than nested component configuration.
func validateNestedSchemaFields(descriptor *componentType, componentPath, fieldPath string, node *yaml.Node) []Issue {
	if descriptor == nil || node == nil || node.Kind != yaml.MappingNode || isComponentSequence(node) || isResolverExpression(node) {
		return nil
	}
	if field, ok := descriptor.FieldByPath[fieldPath]; ok && (field.Kind == "map" || field.ValueDynamic) {
		return nil
	}
	allowed := schemaChildSegments(descriptor, fieldPath)
	issues := make([]Issue, 0)
	for index := 0; index+1 < len(node.Content); index += 2 {
		keyNode, valueNode := node.Content[index], node.Content[index+1]
		key := strings.TrimSpace(keyNode.Value)
		childPath := fieldPath + "." + key
		if !allowed[key] {
			issues = append(issues, Issue{
				Code: "component.unsupported_field", Severity: SeverityError,
				Message: fmt.Sprintf("%s does not support field %q", descriptor.Name, childPath),
				Path:    componentPath + "." + childPath, Range: nodeRange(keyNode),
			})
			continue
		}
		// A child is structured only when the registry declares deeper paths.
		// Leaf fields, including free-form maps, own their values themselves.
		if len(schemaChildSegments(descriptor, childPath)) > 0 {
			issues = append(issues, validateNestedSchemaFields(descriptor, componentPath, childPath, valueNode)...)
		}
	}
	return issues
}

func schemaChildSegments(descriptor *componentType, prefix string) map[string]bool {
	children := make(map[string]bool)
	marker := prefix + "."
	for path := range descriptor.FieldByPath {
		if !strings.HasPrefix(path, marker) {
			continue
		}
		remainder := strings.TrimPrefix(path, marker)
		if segment, _, ok := strings.Cut(remainder, "."); ok {
			children[segment] = true
		} else if remainder != "" {
			children[remainder] = true
		}
	}
	return children
}

func isResolverExpression(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	keys := make(map[string]bool)
	for index := 0; index+1 < len(node.Content); index += 2 {
		key := strings.TrimSpace(node.Content[index].Value)
		if key == "" {
			return false
		}
		keys[key] = true
	}
	if len(keys) == 1 {
		for _, key := range []string{"var", "env", "config", "path", "file", "format"} {
			if keys[key] {
				return true
			}
		}
	}
	if !keys["format"] {
		return false
	}
	for key := range keys {
		if key != "format" && key != "args" {
			return false
		}
	}
	return true
}

func analyzeMounted(registry map[string]*componentType, path string, node *yaml.Node, effective interface{}, unknownSeverity Severity) []Issue {
	if node == nil {
		return nil
	}
	if isEffectiveComponentSequence(node, effective) {
		mapping, _ := effective.(map[string]interface{})
		return analyzeComponent(registry, path, node, mapping, unknownSeverity)
	}
	issues := make([]Issue, 0)
	switch node.Kind {
	case yaml.MappingNode:
		effectiveMap, _ := effective.(map[string]interface{})
		for index := 0; index+1 < len(node.Content); index += 2 {
			key, value := node.Content[index].Value, node.Content[index+1]
			issues = append(issues, analyzeMounted(registry, path+"."+key, value, effectiveMap[key], unknownSeverity)...)
		}
	case yaml.SequenceNode:
		effectiveList, _ := effective.([]interface{})
		for index, value := range node.Content {
			var item interface{}
			if index < len(effectiveList) {
				item = effectiveList[index]
			}
			issues = append(issues, analyzeMounted(registry, path+"["+strconv.Itoa(index)+"]", value, item, unknownSeverity)...)
		}
	}
	return issues
}

func sourceSchemaRegistry() map[string]*componentType {
	registry := make(map[string]*componentType)
	for _, typeSchema := range schema.ExtractRegistry(schema.Definitions()).Types {
		descriptor := &componentType{
			Name: normalizeType(typeSchema.Token), Schema: typeSchema,
			TopFields: make(map[string][]schema.Field), FieldByPath: make(map[string]schema.Field),
		}
		for _, field := range typeSchema.Fields {
			top := strings.Split(field.Path, ".")[0]
			descriptor.TopFields[top] = append(descriptor.TopFields[top], field)
			descriptor.FieldByPath[field.Path] = field
		}
		for _, name := range append([]string{typeSchema.Token}, typeSchema.Aliases...) {
			registry[normalizeType(name)] = descriptor
		}
	}
	return registry
}

func decodeYAML(input []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(input))
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

func documentBody(document *yaml.Node) *yaml.Node {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil
	}
	return document.Content[0]
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
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			continue
		}
		switch strings.TrimSpace(item.Content[0].Value) {
		case "type", "inherit":
			return true
		}
	}
	return false
}

// isEffectiveComponentSequence also recognizes a source overlay that omits its
// inherited child's type. The YAML parser treats such single-key sequences as
// child nodes and supplies the inherited @type after materialization, so source
// validation must use that effective shape rather than reclassifying the child
// as a field on its parent.
func isEffectiveComponentSequence(node *yaml.Node, effective interface{}) bool {
	if node == nil || node.Kind != yaml.SequenceNode {
		return false
	}
	if isComponentSequence(node) {
		return true
	}
	mapping, ok := effective.(map[string]interface{})
	if !ok {
		return false
	}
	typeName, ok := mapping["@type"].(string)
	return ok && strings.TrimSpace(typeName) != ""
}

func normalizeType(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), "<>"))
	return strings.ReplaceAll(value, "-", "_")
}

func lookupMapPath(mapping map[string]interface{}, path string) (interface{}, bool) {
	if mapping == nil {
		return nil, false
	}
	current := interface{}(mapping)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func mapValue(mapping map[string]interface{}, key string) interface{} {
	if mapping == nil {
		return nil
	}
	return mapping[key]
}

func isZero(value interface{}) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	reflected := reflect.ValueOf(value)
	return reflected.IsValid() && reflected.IsZero()
}

func rangeAt(line, column, length int) Range {
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	if length < 1 {
		length = 1
	}
	return Range{Start: Position{Line: line, Column: column}, End: Position{Line: line, Column: column + length}}
}

func nodeRange(node *yaml.Node) Range {
	if node == nil {
		return rangeAt(1, 1, 1)
	}
	return rangeAt(node.Line, node.Column, utf8.RuneCountInString(node.Value))
}

func issueFromError(err error, code string) Issue {
	line, column, file := errorPosition(err)
	return Issue{Code: code, Severity: SeverityError, Message: err.Error(), File: file, Range: rangeAt(line, column, 1)}
}

func parserIssueCode(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "duplicate child") || strings.Contains(message, "duplicate property") || strings.Contains(message, "duplicate top-level"):
		return "component.duplicate"
	case strings.Contains(message, "inherit reference") || strings.Contains(message, "inheritance cycle"):
		return "component.inheritance"
	case strings.Contains(message, "unknown component type"):
		return "component.unknown_type"
	default:
		return "component.syntax"
	}
}

var yamlPosition = regexp.MustCompile(`line ([0-9]+)(?:(?::|, column )([0-9]+))?`)

func errorPosition(err error) (int, int, string) {
	var sourceErr *yamlparser.SourceError
	if errors.As(err, &sourceErr) && sourceErr.Line > 0 {
		return sourceErr.Line, max(1, sourceErr.Column), filepath.Clean(sourceErr.File)
	}
	if match := yamlPosition.FindStringSubmatch(err.Error()); len(match) > 1 {
		line, _ := strconv.Atoi(match[1])
		column := 1
		if len(match) > 2 && match[2] != "" {
			column, _ = strconv.Atoi(match[2])
		}
		return max(1, line), max(1, column), ""
	}
	return 1, 1, ""
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
