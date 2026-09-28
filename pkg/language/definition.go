package language

import (
	"os"
	"path/filepath"
	"strings"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type definitionSource struct {
	path string
	text string
}

type definitionReference struct {
	value       string
	originRange Range
}

type resourceDefinitionReference struct {
	base        string
	path        string
	originRange Range
}

// Definitions resolves the concrete source or resource referenced at position.
// Open HyperBricks documents are used as an overlay while the reachable import
// graph is traversed, matching diagnostics, completion, and hover behavior.
func (a *Analyzer) Definitions(uri, text string, position Position, documentOverlays ...map[string]string) []Location {
	links := a.DefinitionLinks(uri, text, position, documentOverlays...)
	locations := make([]Location, 0, len(links))
	for _, link := range links {
		locations = append(locations, Location{URI: link.TargetURI, Range: link.TargetRange})
	}
	return locations
}

// DefinitionLinks resolves definitions with the complete source scalar as the
// origin selection. LSP clients use that range for Cmd/Ctrl-click highlighting.
func (a *Analyzer) DefinitionLinks(uri, text string, position Position, documentOverlays ...map[string]string) []LocationLink {
	documents := map[string]string(nil)
	if len(documentOverlays) > 0 {
		documents = documentOverlays[0]
	}
	currentPath, readFile, confined := a.sourceReader(uri, text, documents)
	if !confined {
		return []LocationLink{}
	}
	document, err := decodeYAMLDocument(text)
	if err != nil {
		return []LocationLink{}
	}

	if imported, ok := importDefinitionReference(text, document, position); ok {
		importPath, resolveErr := a.resolveDefinitionImport(currentPath, imported.value)
		if resolveErr != nil {
			return []LocationLink{}
		}
		if _, readErr := readFile(importPath); readErr != nil {
			return []LocationLink{}
		}
		return []LocationLink{definitionLocationLink(imported.originRange, Location{URI: pathToURI(importPath), Range: Range{}})}
	}

	if inherited, ok := inheritDefinitionReference(text, position); ok {
		if location, resolved := a.resolveInheritanceDefinition(currentPath, inherited.value, readFile); resolved {
			return []LocationLink{definitionLocationLink(inherited.originRange, location)}
		}
		return []LocationLink{}
	}

	if reference, ok := a.resourceDefinitionAtPosition(uri, text, document, position, documents); ok {
		if location, resolved := a.resolveResourceDefinition(reference); resolved {
			return []LocationLink{definitionLocationLink(reference.originRange, location)}
		}
	}
	return []LocationLink{}
}

func definitionLocationLink(originRange Range, target Location) LocationLink {
	return LocationLink{
		OriginSelectionRange: &originRange,
		TargetURI:            target.URI,
		TargetRange:          target.Range,
		TargetSelectionRange: target.Range,
	}
}

func importDefinitionReference(text string, document *yaml.Node, position Position) (definitionReference, bool) {
	body := yamlDocumentBody(document)
	if body == nil || body.Kind != yaml.MappingNode {
		return definitionReference{}, false
	}
	for index := 0; index+1 < len(body.Content); index += 2 {
		if strings.TrimSpace(body.Content[index].Value) != "imports" {
			continue
		}
		value := body.Content[index+1]
		switch value.Kind {
		case yaml.ScalarNode:
			if definitionContainsPosition(text, value, position) {
				return definitionReference{value: strings.TrimSpace(value.Value), originRange: definitionSourceRange(text, value)}, true
			}
		case yaml.SequenceNode:
			for _, item := range value.Content {
				if item.Kind == yaml.ScalarNode && definitionContainsPosition(text, item, position) {
					return definitionReference{value: strings.TrimSpace(item.Value), originRange: definitionSourceRange(text, item)}, true
				}
			}
		}
		return definitionReference{}, false
	}
	return definitionReference{}, false
}

func inheritDefinitionReference(text string, position Position) (definitionReference, bool) {
	target := definitionReference{}
	visitSourceComponents(text, func(_ string, sequence *yaml.Node) {
		if target.value != "" {
			return
		}
		for _, item := range sequence.Content {
			if item.Kind != yaml.MappingNode || len(item.Content) != 2 || strings.TrimSpace(item.Content[0].Value) != "inherit" {
				continue
			}
			value := item.Content[1]
			if value.Kind == yaml.ScalarNode && definitionContainsPosition(text, value, position) {
				target = definitionReference{value: strings.TrimSpace(value.Value), originRange: definitionSourceRange(text, value)}
				return
			}
		}
	})
	return target, target.value != ""
}

func (a *Analyzer) resolveDefinitionImport(ownerPath, imported string) (string, error) {
	imported = strings.TrimSpace(imported)
	if imported == "" {
		return "", os.ErrNotExist
	}
	path := filepath.FromSlash(imported)
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(ownerPath), path)
	}
	return a.confinedSourcePath(path)
}

func (a *Analyzer) resolveInheritanceDefinition(currentPath, targetPath string, readFile func(string) ([]byte, error)) (Location, bool) {
	document := &yamlparser.Document{}
	for _, source := range a.definitionSources(currentPath, readFile) {
		options := yamlparser.ParseOptions{
			AllowUnknownTypes:        true,
			RecoverDuplicateChildren: true,
			Source:                   source.path,
		}
		sourceDocument, err := yamlparser.ParseBytesWithOptions([]byte(source.text), options)
		if err != nil {
			continue
		}
		document.Roots = append(document.Roots, sourceDocument.Roots...)
	}
	for _, target := range document.InheritanceTargets() {
		if target.Path != targetPath || target.Source == "" || target.Line <= 0 || target.Column <= 0 {
			continue
		}
		raw, readErr := readFile(target.Source)
		if readErr != nil {
			return Location{}, false
		}
		nameNode := definitionDeclarationNode(string(raw), target.Line, target.Column)
		if nameNode == nil {
			return Location{}, false
		}
		return Location{URI: pathToURI(target.Source), Range: definitionSourceRange(string(raw), nameNode)}, true
	}
	return Location{}, false
}

// definitionSources follows imports relative to their declaring source while
// tolerating an unreadable sibling import. Every successful source keeps its
// path so the parser can retain declaration provenance through inheritance.
func (a *Analyzer) definitionSources(currentPath string, readFile func(string) ([]byte, error)) []definitionSource {
	sources := make([]definitionSource, 0, 4)
	visited := make(map[string]bool)
	var walk func(string)
	walk = func(ownerPath string) {
		ownerKey := canonicalPath(ownerPath)
		if visited[ownerKey] {
			return
		}
		visited[ownerKey] = true
		raw, err := readFile(ownerPath)
		if err != nil {
			return
		}
		ownerText := string(raw)
		for _, imported := range sourceImports(ownerText) {
			importPath, resolveErr := a.resolveDefinitionImport(ownerPath, imported)
			if resolveErr == nil {
				walk(importPath)
			}
		}
		sources = append(sources, definitionSource{path: ownerPath, text: ownerText})
	}
	walk(currentPath)
	return sources
}

// definitionDeclarationNode maps the parser's sequence position back to the
// YAML key that declares that node. The parser decides which paths are valid;
// this source walk only recovers the destination selection range.
func definitionDeclarationNode(text string, line, column int) *yaml.Node {
	document, err := decodeYAMLDocument(text)
	if err != nil {
		return nil
	}
	var result *yaml.Node
	var walk func(*yaml.Node)
	walk = func(node *yaml.Node) {
		if node == nil || result != nil {
			return
		}
		switch node.Kind {
		case yaml.MappingNode:
			for index := 0; index+1 < len(node.Content); index += 2 {
				key, value := node.Content[index], node.Content[index+1]
				if value.Kind == yaml.SequenceNode && value.Line == line && value.Column == column {
					result = key
					return
				}
				walk(value)
			}
		case yaml.DocumentNode, yaml.SequenceNode:
			for _, child := range node.Content {
				walk(child)
			}
		}
	}
	walk(document)
	return result
}

func (a *Analyzer) resourceDefinitionAtPosition(uri, text string, document *yaml.Node, position Position, documents map[string]string) (resourceDefinitionReference, bool) {
	body := yamlDocumentBody(document)
	if body == nil {
		return resourceDefinitionReference{}, false
	}
	var found resourceDefinitionReference
	var resolved bool
	var walk func(*yaml.Node, string)
	walk = func(node *yaml.Node, parentKey string) {
		if node == nil || resolved {
			return
		}
		if node.Kind == yaml.SequenceNode && isComponentSequence(node) {
			if reference, ok := a.directResourceDefinition(uri, text, node, position, documents); ok {
				found, resolved = reference, true
				return
			}
		}
		if node.Kind == yaml.MappingNode {
			baseNode := definitionMappingValue(node, "base")
			pathNode := definitionMappingValue(node, "path")
			partsNode := definitionMappingValue(node, "parts")
			if baseNode != nil && baseNode.Kind == yaml.ScalarNode {
				if pathNode != nil && pathNode.Kind == yaml.ScalarNode && definitionContainsPosition(text, pathNode, position) {
					found = resourceDefinitionReference{
						base:        strings.TrimSpace(baseNode.Value),
						path:        strings.TrimSpace(pathNode.Value),
						originRange: definitionSourceRange(text, pathNode),
					}
					resolved = found.path != ""
					return
				}
				if partsNode != nil && partsNode.Kind == yaml.SequenceNode {
					parts := make([]string, 0, len(partsNode.Content))
					clicked := false
					clickedRange := Range{}
					for _, item := range partsNode.Content {
						if item.Kind != yaml.ScalarNode {
							continue
						}
						parts = append(parts, filepath.FromSlash(strings.TrimSpace(item.Value)))
						if definitionContainsPosition(text, item, position) {
							clicked = true
							clickedRange = definitionSourceRange(text, item)
						}
					}
					if clicked && len(parts) > 0 {
						found = resourceDefinitionReference{base: strings.TrimSpace(baseNode.Value), path: filepath.Join(parts...), originRange: clickedRange}
						resolved = true
						return
					}
				}
			}
			if parentKey == "template" {
				fileNode := definitionMappingValue(node, "file")
				if fileNode != nil && fileNode.Kind == yaml.ScalarNode && definitionContainsPosition(text, fileNode, position) {
					found = resourceDefinitionReference{
						base:        "templates",
						path:        strings.TrimSpace(fileNode.Value),
						originRange: definitionSourceRange(text, fileNode),
					}
					resolved = found.path != ""
					return
				}
			}
			for index := 0; index+1 < len(node.Content); index += 2 {
				key := strings.TrimSpace(node.Content[index].Value)
				walk(node.Content[index+1], key)
			}
			return
		}
		for _, child := range node.Content {
			walk(child, parentKey)
		}
	}
	walk(body, "")
	return found, resolved
}

func (a *Analyzer) directResourceDefinition(uri, text string, sequence *yaml.Node, position Position, documents map[string]string) (resourceDefinitionReference, bool) {
	key := ""
	path := ""
	originRange := Range{}
	for _, item := range sequence.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			continue
		}
		value := item.Content[1]
		if value.Kind == yaml.ScalarNode && definitionContainsPosition(text, value, position) {
			key = strings.TrimSpace(item.Content[0].Value)
			path = strings.TrimSpace(value.Value)
			originRange = definitionSourceRange(text, value)
			break
		}
	}
	if key == "" || path == "" {
		return resourceDefinitionReference{}, false
	}
	entries := sequenceEntries(sequence)
	typeName := ""
	if typeNode := entries["type"]; typeNode != nil && typeNode.Kind == yaml.ScalarNode {
		typeName = typeNode.Value
	} else if inheritNode := entries["inherit"]; inheritNode != nil && inheritNode.Kind == yaml.ScalarNode {
		typeName = a.inheritedType(uri, text, inheritNode.Value, documents)
	}
	descriptor := a.types[normalizeTypeName(typeName)]
	if descriptor == nil {
		return resourceDefinitionReference{}, false
	}
	spec, ok := pathCompletionFor(descriptor, key)
	if ok && spec.resolver == "relative" {
		return resourceDefinitionReference{base: spec.directoryKey, path: path, originRange: originRange}, true
	}
	return resourceDefinitionReference{}, false
}

func definitionMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if strings.TrimSpace(mapping.Content[index].Value) == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

// definitionContainsPosition uses the scalar's source spelling for quoted
// values. yaml.Node.Value is decoded, so escape sequences such as \u002E are
// shorter than the text under the cursor and cannot safely define a hit range.
func definitionContainsPosition(text string, node *yaml.Node, position Position) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}
	rangeValue := definitionSourceRange(text, node)
	if position.Line < rangeValue.Start.Line || position.Line > rangeValue.End.Line {
		return false
	}
	if position.Line == rangeValue.Start.Line && position.Character < rangeValue.Start.Character {
		return false
	}
	// LSP ranges are end-exclusive. Keeping hit testing aligned guarantees that
	// every successful definition request lies inside OriginSelectionRange.
	if position.Line == rangeValue.End.Line && position.Character >= rangeValue.End.Character {
		return false
	}
	return true
}

func definitionSourceRange(text string, node *yaml.Node) Range {
	if node == nil {
		return Range{}
	}
	style := node.Style &^ yaml.TaggedStyle
	if style == yaml.DoubleQuotedStyle || style == yaml.SingleQuotedStyle {
		if sourceRange, ok := quotedScalarSourceRange(text, node, style); ok {
			return sourceRange
		}
	}
	return yamlNodeRange(text, node)
}

func quotedScalarSourceRange(text string, node *yaml.Node, style yaml.Style) (Range, bool) {
	lines := splitLines(text)
	startLine := node.Line - 1
	startRune := node.Column - 1
	if startLine < 0 || startLine >= len(lines) || startRune < 0 {
		return Range{}, false
	}
	startRunes := []rune(lines[startLine])
	if startRune >= len(startRunes) {
		return Range{}, false
	}
	quote := rune('"')
	if style == yaml.SingleQuotedStyle {
		quote = '\''
	}
	if startRunes[startRune] != quote {
		return Range{}, false
	}
	start := Position{Line: startLine, Character: utf16Length(string(startRunes[:startRune]))}
	escaped := false
	for lineIndex := startLine; lineIndex < len(lines); lineIndex++ {
		runes := []rune(lines[lineIndex])
		index := 0
		if lineIndex == startLine {
			index = startRune + 1
		}
		for index < len(runes) {
			character := runes[index]
			if style == yaml.DoubleQuotedStyle {
				if escaped {
					escaped = false
					index++
					continue
				}
				if character == '\\' {
					escaped = true
					index++
					continue
				}
			} else if character == quote && index+1 < len(runes) && runes[index+1] == quote {
				index += 2
				continue
			}
			if character == quote {
				end := Position{Line: lineIndex, Character: utf16Length(string(runes[:index+1]))}
				return Range{Start: start, End: end}, true
			}
			index++
		}
		escaped = false
	}
	return Range{}, false
}

func (a *Analyzer) resolveResourceDefinition(reference resourceDefinitionReference) (Location, bool) {
	base, ok := a.resourceDefinitionBase(reference.base)
	if !ok || strings.TrimSpace(reference.path) == "" {
		return Location{}, false
	}
	path := filepath.FromSlash(strings.TrimSpace(reference.path))
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	// Definitions intentionally navigate only to files owned by the selected
	// module. The wider root and module_root markers remain useful when their
	// final path points back into that module, but they cannot expose sibling
	// modules or arbitrary workspace files through editor navigation.
	confined, err := a.confinedPath(path)
	if err != nil {
		return Location{}, false
	}
	info, err := os.Stat(confined)
	if err != nil || !info.Mode().IsRegular() {
		return Location{}, false
	}
	return Location{URI: pathToURI(confined), Range: Range{}}, true
}

func (a *Analyzer) resourceDefinitionBase(base string) (string, bool) {
	switch strings.TrimSpace(base) {
	case "module":
		return a.moduleRoot(), true
	case "module_root":
		return filepath.Dir(a.moduleRoot()), true
	case "root":
		return a.workspaceRoot, true
	case "resources":
		return a.directory("resources", "resources"), true
	case "templates":
		return a.directory("templates", "templates"), true
	case "static":
		return a.directory("static", "static"), true
	case "hyperbricks":
		return a.directory("hyperbricks", "hyperbricks"), true
	case "render":
		return a.directory("render", "rendered"), true
	default:
		return "", false
	}
}
