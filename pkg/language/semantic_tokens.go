package language

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

// SemanticHighlights classifies only syntax whose meaning is known from the
// HyperBricks model. Ordinary YAML map keys deliberately remain unclassified
// so the base grammar and active theme keep ownership of application data such
// as HTTP header names and template values.
func (a *Analyzer) SemanticHighlights(uri, text string, documents map[string]string) []SemanticHighlight {
	if a == nil || a.isPackageConfig(uri) {
		return []SemanticHighlight{}
	}
	document, err := decodeYAMLDocument(text)
	if err != nil {
		return []SemanticHighlight{}
	}
	body := yamlDocumentBody(document)
	if body == nil || body.Kind != yaml.MappingNode {
		return []SemanticHighlight{}
	}

	targets, effectiveTypes, _ := a.inheritanceReferenceIndex(uri, text, documents)
	collector := semanticHighlightCollector{
		analyzer:       a,
		text:           text,
		targets:        targets,
		effectiveTypes: effectiveTypes,
	}
	for index := 0; index+1 < len(body.Content); index += 2 {
		keyNode, valueNode := body.Content[index], body.Content[index+1]
		key := strings.TrimSpace(keyNode.Value)
		switch key {
		case "imports", "vars":
			collector.addNode(keyNode, SemanticTokenTypeKeyword)
			continue
		}
		collector.addNode(keyNode, SemanticTokenTypeClass, SemanticTokenModifierDeclaration)
		if valueNode.Kind == yaml.SequenceNode {
			collector.walkComponent(key, valueNode)
		}
	}
	return normalizeSemanticHighlights(collector.highlights)
}

type semanticHighlightCollector struct {
	analyzer       *Analyzer
	text           string
	targets        map[string]bool
	effectiveTypes map[string]string
	highlights     []SemanticHighlight
}

func (collector *semanticHighlightCollector) walkComponent(path string, sequence *yaml.Node) {
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return
	}
	descriptor := collector.componentDescriptor(path, sequence)
	for _, item := range sequence.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index+1 < len(item.Content); index += 2 {
			keyNode, valueNode := item.Content[index], item.Content[index+1]
			key := strings.TrimSpace(keyNode.Value)
			switch key {
			case "type":
				collector.addNode(keyNode, SemanticTokenTypeKeyword)
				if valueNode.Kind == yaml.ScalarNode && collector.analyzer.types[normalizeTypeName(valueNode.Value)] != nil {
					collector.addNode(valueNode, SemanticTokenTypeType)
				}
				continue
			case "inherit":
				collector.addNode(keyNode, SemanticTokenTypeKeyword)
				if valueNode.Kind == yaml.ScalarNode && strings.TrimSpace(valueNode.Value) != "" {
					collector.addNode(valueNode, SemanticTokenTypeClass)
				}
				continue
			}

			childPath := joinTargetPath(path, key)
			knownField, dynamicBoundary := semanticSchemaPath(descriptor, key)
			isDirectChild := valueNode.Kind == yaml.SequenceNode && collector.targets[childPath]
			if isDirectChild || (!knownField && isComponentSequence(valueNode)) {
				collector.addNode(keyNode, SemanticTokenTypeClass, SemanticTokenModifierDeclaration)
				collector.walkComponent(childPath, valueNode)
				continue
			}
			if knownField {
				collector.addNode(keyNode, SemanticTokenTypeProperty)
				if !dynamicBoundary && schemaHasPathPrefix(descriptor, key+".") {
					collector.walkSchemaValue(descriptor, key, childPath, valueNode)
				} else {
					collector.walkMounted(childPath, valueNode)
				}
				continue
			}
			collector.walkMounted(childPath, valueNode)
		}
	}
}

func (collector *semanticHighlightCollector) componentDescriptor(path string, sequence *yaml.Node) *typeDescriptor {
	typeName := strings.TrimSpace(collector.effectiveTypes[path])
	entries := sequenceEntries(sequence)
	if explicit := entries["type"]; explicit != nil && explicit.Kind == yaml.ScalarNode {
		typeName = explicit.Value
	} else if typeName == "" {
		if inherited := entries["inherit"]; inherited != nil && inherited.Kind == yaml.ScalarNode {
			typeName = collector.effectiveTypes[strings.TrimSpace(inherited.Value)]
		}
	}
	return collector.analyzer.types[normalizeTypeName(typeName)]
}

func (collector *semanticHighlightCollector) walkSchemaValue(descriptor *typeDescriptor, schemaPrefix, componentPath string, node *yaml.Node) {
	if node == nil {
		return
	}
	if isComponentSequence(node) {
		collector.walkComponent(componentPath, node)
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			keyNode, valueNode := node.Content[index], node.Content[index+1]
			key := strings.TrimSpace(keyNode.Value)
			schemaPath := joinTargetPath(schemaPrefix, key)
			fullPath := joinTargetPath(componentPath, key)
			knownField, dynamicBoundary := semanticSchemaPath(descriptor, schemaPath)
			if knownField {
				collector.addNode(keyNode, SemanticTokenTypeProperty)
				if !dynamicBoundary && schemaHasPathPrefix(descriptor, schemaPath+".") {
					collector.walkSchemaValue(descriptor, schemaPath, fullPath, valueNode)
				} else {
					collector.walkMounted(fullPath, valueNode)
				}
				continue
			}
			collector.walkMounted(fullPath, valueNode)
		}
	case yaml.SequenceNode:
		for index, item := range node.Content {
			collector.walkSchemaValue(descriptor, schemaPrefix, fmt.Sprintf("%s[%d]", componentPath, index), item)
		}
	}
}

// walkMounted finds component sequences inside ordinary maps and lists. Their
// mounting key is application data and is intentionally not reclassified as a
// component declaration, while the component's own reserved words and schema
// fields still receive semantic highlighting.
func (collector *semanticHighlightCollector) walkMounted(path string, node *yaml.Node) {
	if node == nil {
		return
	}
	if isComponentSequence(node) {
		collector.walkComponent(path, node)
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			key := strings.TrimSpace(node.Content[index].Value)
			collector.walkMounted(joinTargetPath(path, key), node.Content[index+1])
		}
	case yaml.SequenceNode:
		for index, item := range node.Content {
			collector.walkMounted(fmt.Sprintf("%s[%d]", path, index), item)
		}
	}
}

func semanticSchemaPath(descriptor *typeDescriptor, path string) (known bool, dynamicBoundary bool) {
	if descriptor == nil || strings.TrimSpace(path) == "" {
		return false, false
	}
	field, exact := descriptor.fieldByPath[path]
	known = exact || schemaHasPathPrefix(descriptor, path+".")
	if exact {
		dynamicBoundary = field.Kind == "map" || field.ValueDynamic
	}
	return known, dynamicBoundary
}

func (collector *semanticHighlightCollector) addNode(node *yaml.Node, tokenType string, modifiers ...string) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return
	}
	rangeValue := semanticNodeRange(collector.text, node)
	if rangeValue.Start.Line != rangeValue.End.Line || rangeValue.End.Character <= rangeValue.Start.Character {
		return
	}
	collector.highlights = append(collector.highlights, SemanticHighlight{
		Range: rangeValue, TokenType: tokenType, Modifiers: append([]string(nil), modifiers...),
	})
}

func semanticNodeRange(text string, node *yaml.Node) Range {
	rangeValue := definitionSourceRange(text, node)
	style := node.Style &^ yaml.TaggedStyle
	if rangeValue.Start.Line == rangeValue.End.Line &&
		(style == yaml.DoubleQuotedStyle || style == yaml.SingleQuotedStyle) &&
		rangeValue.End.Character-rangeValue.Start.Character >= 2 {
		rangeValue.Start.Character++
		rangeValue.End.Character--
	}
	return rangeValue
}

func normalizeSemanticHighlights(highlights []SemanticHighlight) []SemanticHighlight {
	if len(highlights) == 0 {
		return []SemanticHighlight{}
	}
	sorted := append([]SemanticHighlight(nil), highlights...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i].Range, sorted[j].Range
		if left.Start.Line != right.Start.Line {
			return left.Start.Line < right.Start.Line
		}
		if left.Start.Character != right.Start.Character {
			return left.Start.Character < right.Start.Character
		}
		if left.End.Character != right.End.Character {
			return left.End.Character < right.End.Character
		}
		return sorted[i].TokenType < sorted[j].TokenType
	})

	out := make([]SemanticHighlight, 0, len(sorted))
	for _, highlight := range sorted {
		if highlight.Range.Start.Line < 0 || highlight.Range.Start.Character < 0 ||
			highlight.Range.End.Line != highlight.Range.Start.Line ||
			highlight.Range.End.Character <= highlight.Range.Start.Character {
			continue
		}
		if len(out) > 0 {
			previous := out[len(out)-1].Range
			if previous.Start.Line == highlight.Range.Start.Line && previous.End.Character > highlight.Range.Start.Character {
				continue
			}
		}
		out = append(out, highlight)
	}
	return out
}

func encodeSemanticHighlights(highlights []SemanticHighlight) SemanticTokens {
	highlights = normalizeSemanticHighlights(highlights)
	typeIndex := make(map[string]uint32, len(SemanticTokenTypes))
	for index, tokenType := range SemanticTokenTypes {
		typeIndex[tokenType] = uint32(index)
	}
	modifierIndex := make(map[string]uint32, len(SemanticTokenModifiers))
	for index, modifier := range SemanticTokenModifiers {
		modifierIndex[modifier] = uint32(index)
	}

	data := make([]uint32, 0, len(highlights)*5)
	previousLine, previousCharacter := 0, 0
	for _, highlight := range highlights {
		tokenType, ok := typeIndex[highlight.TokenType]
		if !ok {
			continue
		}
		line := highlight.Range.Start.Line
		character := highlight.Range.Start.Character
		deltaLine := line - previousLine
		deltaCharacter := character
		if deltaLine == 0 {
			deltaCharacter -= previousCharacter
		}
		length := highlight.Range.End.Character - character
		if deltaLine < 0 || deltaCharacter < 0 || length <= 0 {
			continue
		}
		var modifierBits uint32
		for _, modifier := range highlight.Modifiers {
			if index, exists := modifierIndex[modifier]; exists {
				modifierBits |= 1 << index
			}
		}
		data = append(data, uint32(deltaLine), uint32(deltaCharacter), uint32(length), tokenType, modifierBits)
		previousLine, previousCharacter = line, character
	}
	return SemanticTokens{Data: data}
}
