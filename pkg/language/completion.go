package language

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/schema"
	"go.yaml.in/yaml/v4"
)

const completionMarker = "__hyperbricks_editor_cursor__"

func automaticCompletionBoundary(text string, position Position) bool {
	c := completionCursor(text, position)
	if c == nil || c.prefix != "" {
		return false
	}
	line := splitLines(text)[position.Line]
	prefix := strings.TrimRight(line[:byteIndexForUTF16(line, position.Character)], " \t")
	return prefix != "" && strings.ContainsRune(":-,{[", rune(prefix[len(prefix)-1]))
}

// Completion parses a disposable copy with only the active token replaced.
// The YAML tree owns context; indentation alone must never borrow a sibling's
// type or mistake an application-data key for a component field.
type yamlCompletionCursor struct {
	root            *yaml.Node
	trail           []completionEdge
	edit            Range
	prefix          string
	value           bool
	flow            bool
	indent          int
	existingColon   bool
	referenceSource string
	referencePrefix string
	quoted          bool
}

type completionEdge struct {
	node        *yaml.Node
	key         string
	keyPosition bool
}

func completionCursor(text string, position Position) *yamlCompletionCursor {
	lines := splitLines(text)
	if position.Line < 0 || position.Line >= len(lines) || completionInsideBlock(text, position) {
		return nil
	}
	line := lines[position.Line]
	column := byteIndexForUTF16(line, position.Character)
	start := leadingSpaces(line)
	if start < len(line) && line[start] == '-' && (start+1 == len(line) || line[start+1] == ' ') {
		start++
	}
	priorClosers, inQuote := completionFlowState(strings.Join(lines[:position.Line], "\n"))
	if inQuote {
		return nil
	}
	value, flow := false, priorClosers != ""
	var quote byte
	for i := start; i < column; i++ {
		ch := line[i]
		if quote != 0 {
			if ch == '\\' && quote == '"' {
				i++
				continue
			}
			if ch == quote {
				if quote == '\'' && i+1 < column && line[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			if strings.TrimSpace(line[start:i]) == "" {
				quote = ch
			}
		case '#':
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return nil
			}
		case '{':
			start, value, flow = i+1, false, true
		case '[':
			start, value, flow = i+1, true, true
		case ',':
			if flow {
				start, value = i+1, false
			}
		case ':':
			if !value {
				if i+1 >= column || (line[i+1] != ' ' && line[i+1] != '\t') {
					return nil
				}
				start, value = i+1, true
			}
		}
	}
	for start < column && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	if start > column {
		return nil
	}
	end := column
	if start < len(line) && (line[start] == '"' || line[start] == '\'') {
		q := line[start]
		end = start + 1
		for end < len(line) {
			if q == '"' && line[end] == '\\' {
				end += 2
				continue
			}
			if line[end] == q {
				end++
				if q == '\'' && end < len(line) && line[end] == q {
					end++
					continue
				}
				break
			}
			end++
		}
		if end > len(line) {
			end = len(line)
		}
	} else {
		for end < len(line) {
			ch := line[end]
			if (flow && strings.ContainsRune(",}]", rune(ch))) || (!value && ch == ':') || (ch == '#' && (end == 0 || line[end-1] == ' ')) {
				break
			}
			end++
		}
		for end > column && (line[end-1] == ' ' || line[end-1] == '\t') {
			end--
		}
	}
	cursor := &yamlCompletionCursor{
		edit:   Range{Start: Position{Line: position.Line, Character: utf16Length(line[:start])}, End: Position{Line: position.Line, Character: utf16Length(line[:end])}},
		prefix: strings.Trim(line[start:column], "\"'"), value: value, flow: flow, indent: leadingSpaces(line),
		existingColon: !value && strings.HasPrefix(strings.TrimLeft(line[end:], " \t"), ":"),
		quoted:        start < len(line) && (line[start] == '\'' || line[start] == '"'),
	}
	lines[position.Line] = line[:start] + completionMarker + line[end:]
	modified := strings.Join(lines, "\n")
	document, err := decodeYAMLDocument(modified)
	if err != nil && flow {
		closers, _ := completionFlowState(strings.Join(lines[:position.Line+1], "\n"))
		lines[position.Line] += closers
		modified = strings.Join(lines, "\n")
		document, err = decodeYAMLDocument(modified)
	}
	if err != nil && !value && !flow {
		lines[position.Line] = line[:start] + completionMarker + ": null" + line[end:]
		document, err = decodeYAMLDocument(strings.Join(lines, "\n"))
	}
	if err != nil {
		return nil
	}
	cursor.root = yamlDocumentBody(document)
	if !findCompletionTrail(cursor.root, nil, &cursor.trail) {
		return nil
	}
	lines = splitLines(text)
	lines[position.Line] = ""
	cursor.referenceSource = strings.Join(lines, "\n")
	if _, parseErr := parseInheritanceReferenceSource(text); parseErr == nil {
		cursor.referenceSource = text
	}
	return cursor
}

func completionFlowState(source string) (string, bool) {
	stack := []byte{}
	var quote byte
	blockIndent := -1
	for _, line := range splitLines(source) {
		indent := leadingSpaces(line)
		if blockIndent >= 0 {
			if strings.TrimSpace(line) == "" || indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		tokenStart := true
		for i := 0; i < len(line); i++ {
			ch := line[i]
			if quote != 0 {
				if ch == '\\' && quote == '"' {
					i++
					continue
				}
				if ch == quote {
					if quote == '\'' && i+1 < len(line) && line[i+1] == '\'' {
						i++
						continue
					}
					quote = 0
				}
				continue
			}
			switch ch {
			case '"', '\'':
				if tokenStart {
					quote = ch
				}
			case '{':
				if tokenStart {
					stack = append(stack, '}')
					continue
				}
			case '[':
				if tokenStart {
					stack = append(stack, ']')
					continue
				}
			case '}', ']':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			case '#':
				if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
					i = len(line)
					continue
				}
			case ':':
				if i+1 == len(line) || line[i+1] == ' ' || line[i+1] == '\t' {
					tokenStart = true
					continue
				}
			case ',':
				if len(stack) > 0 {
					tokenStart = true
					continue
				}
			case '-':
				if i == indent && i+1 < len(line) && line[i+1] == ' ' {
					tokenStart = true
					continue
				}
			case '|', '>':
				if tokenStart && len(stack) == 0 {
					blockIndent = indent
					i = len(line)
					continue
				}
			}
			if ch != ' ' && ch != '\t' {
				tokenStart = false
			}
		}
	}
	var out strings.Builder
	for i := len(stack) - 1; i >= 0; i-- {
		out.WriteByte(stack[i])
	}
	return out.String(), quote != 0
}

func completionInsideBlock(text string, position Position) bool {
	lines := splitLines(text)
	blockIndent := -1
	for index, line := range lines {
		if index > position.Line {
			break
		}
		trimmed := strings.TrimSpace(line)
		indent := leadingSpaces(line)
		if blockIndent >= 0 {
			if trimmed == "" || indent > blockIndent {
				if index == position.Line {
					return true
				}
				continue
			}
			blockIndent = -1
		}
		if colon := strings.Index(line, ":"); colon >= 0 {
			value := strings.TrimSpace(strings.SplitN(line[colon+1:], "#", 2)[0])
			if value != "" && (value[0] == '|' || value[0] == '>') && strings.Trim(value[1:], "+-123456789") == "" {
				blockIndent = indent
			}
		}
	}
	return false
}

func findCompletionTrail(node *yaml.Node, trail []completionEdge, result *[]completionEdge) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.ScalarNode && node.Value == completionMarker {
		*result = trail
		return true
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Value == completionMarker {
				*result = append(trail, completionEdge{node: key, keyPosition: true})
				return true
			}
			if findCompletionTrail(value, append(trail, completionEdge{node: value, key: key.Value}), result) {
				return true
			}
		}
	} else {
		for _, child := range node.Content {
			if findCompletionTrail(child, append(trail, completionEdge{node: child}), result) {
				return true
			}
		}
	}
	return false
}

func (a *Analyzer) yamlCompletions(uri, text string, position Position, documents map[string]string) []CompletionItem {
	cursor := completionCursor(text, position)
	if cursor == nil {
		return nil
	}
	items := a.completionCandidates(uri, text, position, documents, cursor)
	for i := range items {
		insert := items[i].InsertText
		if cPrefix := cursor.referencePrefix; cPrefix != "" {
			insert = cPrefix + insert
			if cursor.quoted {
				insert = strconv.Quote(insert)
			}
		}
		if cursor.existingColon && !cursor.value {
			insert = items[i].Label
		}
		items[i].TextEdit = &TextEdit{Range: cursor.edit, NewText: insert}
	}
	return items
}

func (a *Analyzer) completionCandidates(uri, text string, position Position, documents map[string]string, c *yamlCompletionCursor) []CompletionItem {
	if len(c.trail) == 0 {
		return nil
	}
	last := c.trail[len(c.trail)-1]
	keys := []string{}
	componentIndex := -1
	var descriptor *typeDescriptor
	var component *yaml.Node
	_, effectiveTypes, _ := a.inheritanceReferenceIndex(uri, c.referenceSource, documents)
	for index, edge := range c.trail {
		if edge.key != "" {
			keys = append(keys, edge.key)
		}
		path := strings.Join(keys, ".")
		directChild := componentIndex >= 0 && index == componentIndex+2 && descriptor != nil && descriptor.schema.ChildModel != schema.ChildModelNone && descriptor.schema.ChildModel != schema.ChildModelValues && (len(descriptor.topFields[edge.key]) == 0 || authoringSlotOwnsCompletion(descriptor, edge.key)) && completionOrderedEntries(edge.node)
		if edge.node.Kind == yaml.SequenceNode && (index == 0 && edge.key != "imports" || isComponentSequence(edge.node) || effectiveTypes[path] != "" || directChild) {
			componentIndex, component = index, edge.node
			typeName := effectiveTypes[path]
			entries := sequenceEntries(component)
			if n := entries["type"]; n != nil {
				typeName = n.Value
			} else if n := entries["inherit"]; n != nil {
				typeName = effectiveTypes[n.Value]
			}
			descriptor = a.types[normalizeTypeName(typeName)]
		}
	}
	if c.trail[0].key == "imports" {
		return a.importCompletions(uri)
	}
	// Resolver maps are recognized by ownership and shape, never by a free-
	// floating key named `path`, `base` or `file` in application data.
	if items, owned := a.resolverCompletionCandidates(uri, documents, c, componentIndex); owned {
		return items
	}
	if componentIndex < 0 {
		return nil
	}
	fieldKeys := []string{}
	for _, edge := range c.trail[componentIndex+1:] {
		if edge.key != "" {
			fieldKeys = append(fieldKeys, edge.key)
		}
	}
	fieldPath := strings.Join(fieldKeys, ".")
	// At an ordered component entry (including an empty dash), offer schema
	// fields and native child slots, excluding entries already present.
	if !c.value && fieldPath == "" {
		existing := map[string]bool{}
		for key := range sequenceEntries(component) {
			existing[key] = key != completionMarker
		}
		items := []CompletionItem{}
		if !existing["type"] && !existing["inherit"] {
			items = append(items, CompletionItem{Label: "type", Kind: CompletionItemKindField, Detail: "Component type", InsertText: "type: ${1:html}", InsertTextFormat: InsertTextFormatSnippet}, CompletionItem{Label: "inherit", Kind: CompletionItemKindReference, Detail: "Inherit a reachable component", InsertText: "inherit: ${1:component}", InsertTextFormat: InsertTextFormatSnippet})
		}
		items = append(items, schemaKeyCompletions(descriptor, "", existing, c.indent, true)...)
		if descriptor != nil {
			items = append(items, childCompletions(descriptor, existing, c.indent)...)
		}
		return items
	}
	if c.value && len(fieldKeys) == 1 {
		switch fieldPath {
		case "type":
			return a.typeCompletions()
		case "inherit":
			items := inheritCompletionItems(a.inheritTargets(uri, c.referenceSource, position, documents), c.prefix)
			// Segment labels remain concise, but the explicit edit owns the
			// complete reference, including any typed dotted prefix.
			if dot := strings.LastIndex(c.prefix, "."); dot >= 0 {
				c.referencePrefix = c.prefix[:dot+1]
			}
			return items
		}
		if spec, ok := pathCompletionFor(descriptor, fieldPath); ok {
			return a.pathCompletions(spec)
		}
	}
	if !c.value {
		existing := map[string]bool{}
		if last.keyPosition && len(c.trail) > 1 {
			existing = mappingKeys(c.trail[len(c.trail)-2].node)
		}
		if descriptor != nil && schemaHasPathPrefix(descriptor, fieldPath+".") {
			items := schemaKeyCompletions(descriptor, fieldPath, existing, c.indent, false)
			if c.flow {
				for i := range items {
					items[i].InsertText = flowSchemaSnippet(descriptor, fieldPath, items[i].Label)
				}
			}
			return items
		}
		delete(existing, completionMarker)
		if len(existing) > 0 || len(c.trail) > 1 && c.trail[len(c.trail)-2].node.Kind == yaml.SequenceNode {
			return nil
		}
		return resolverKeyCompletions(c.indent, c.flow)
	}
	if descriptor != nil {
		if field, ok := descriptor.fieldByPath[fieldPath]; ok {
			if values := field.AllowedValues(); len(values) > 0 {
				items := []CompletionItem{}
				for _, value := range values {
					if !strings.HasPrefix(value, c.prefix) {
						continue
					}
					item := literalCompletions(value)[0]
					if c.quoted {
						item.InsertText = strconv.Quote(value)
					}
					items = append(items, item)
				}
				if strings.TrimSpace(c.prefix) == "" && field.Kind != "list" {
					items = append(items, inlineResolverCompletions()...)
				}
				return items
			}
		}
		if field, ok := descriptor.fieldByPath[fieldPath]; ok && field.Kind == "bool" {
			return literalCompletions("true", "false")
		}
		// Do not pretend URL/path-looking fields are filesystem paths. Generic
		// resolver snippets are available at empty scalar/map values instead.
		if field, ok := descriptor.fieldByPath[fieldPath]; ok && field.Kind != "list" && strings.TrimSpace(c.prefix) == "" {
			return inlineResolverCompletions()
		}
	}
	return nil
}

func mappingKeys(node *yaml.Node) map[string]bool {
	keys := map[string]bool{}
	if node != nil && node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			keys[node.Content[i].Value] = true
		}
	}
	return keys
}

func completionOrderedEntries(node *yaml.Node) bool {
	if node.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range node.Content {
		if item.Value == completionMarker {
			continue
		}
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return false
		}
	}
	return true
}

func schemaKeyCompletions(descriptor *typeDescriptor, prefix string, existing map[string]bool, indent int, ordered bool) []CompletionItem {
	if descriptor == nil {
		return nil
	}
	groups := map[string][]schema.Field{}
	for _, field := range descriptor.schema.Fields {
		path := field.Path
		if prefix != "" {
			if !strings.HasPrefix(path, prefix+".") {
				continue
			}
			path = strings.TrimPrefix(path, prefix+".")
		}
		key := strings.Split(path, ".")[0]
		if existing[key] || (prefix == "" && authoringSlotOwnsCompletion(descriptor, key)) {
			continue
		}
		groups[key] = append(groups[key], field)
	}
	keys := []string{}
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := []CompletionItem{}
	for _, key := range keys {
		fields := groups[key]
		snippetIndent := indent
		if !ordered {
			snippetIndent -= 2
			if snippetIndent < 0 {
				snippetIndent = 0
			}
		}
		field := schemaFieldForPath(fields, joinTargetPath(prefix, key))
		detail := field.Kind
		if len(fields) > 1 {
			detail = "object"
		}
		items = append(items, CompletionItem{Label: key, Kind: CompletionItemKindField, Detail: detail, Documentation: MarkupContent{Kind: "markdown", Value: field.Description}, InsertText: fieldSnippet(key, fields, snippetIndent, prefix), InsertTextFormat: InsertTextFormatSnippet})
	}
	return items
}

// Flow collections must keep every nested object/list explicitly delimited;
// a block snippet cannot be inserted into an existing `{ ... }` mapping.
func flowSchemaSnippet(descriptor *typeDescriptor, prefix, key string) string {
	root := &snippetFieldNode{children: map[string]*snippetFieldNode{}}
	for _, field := range descriptor.schema.Fields {
		path := strings.TrimPrefix(field.Path, prefix+".")
		if path != key && !strings.HasPrefix(path, key+".") {
			continue
		}
		node := root
		for _, part := range strings.Split(path, ".") {
			if node.children[part] == nil {
				node.children[part] = &snippetFieldNode{children: map[string]*snippetFieldNode{}}
			}
			node = node.children[part]
		}
		copy := field
		node.field = &copy
	}
	placeholder := 1
	var render func(*snippetFieldNode) string
	render = func(node *snippetFieldNode) string {
		if len(node.children) > 0 {
			keys := []string{}
			for child := range node.children {
				keys = append(keys, child)
			}
			sort.Strings(keys)
			parts := []string{}
			for _, child := range keys {
				parts = append(parts, child+": "+render(node.children[child]))
			}
			return "{" + strings.Join(parts, ", ") + "}"
		}
		i := placeholder
		placeholder++
		if node.field != nil {
			if snippet, ok := schemaScalarSnippet(*node.field, i); ok {
				return snippet
			}
			switch node.field.Kind {
			case "map":
				placeholder++
				return fmt.Sprintf("{${%d:key}: ${%d:value}}", i, i+1)
			case "list":
				return fmt.Sprintf("[${%d:value}]", i)
			case "bool":
				return fmt.Sprintf("${%d|true,false|}", i)
			case "int", "uint", "float":
				return fmt.Sprintf("${%d:0}", i)
			}
		}
		return fmt.Sprintf("${%d:value}", i)
	}
	return key + ": " + render(root.children[key])
}

func literalCompletions(values ...string) []CompletionItem {
	items := []CompletionItem{}
	for _, value := range values {
		items = append(items, CompletionItem{Label: value, Kind: CompletionItemKindReference, InsertText: value})
	}
	return items
}

func inlineResolverCompletions() []CompletionItem {
	items := resolverKeyCompletions(0, true)
	for i := range items {
		items[i].InsertText = "{" + items[i].InsertText + "}"
	}
	return items
}

func resolverKeyCompletions(indent int, flow bool) []CompletionItem {
	items := resolverCompletions(indent)
	if !flow {
		return items
	}
	texts := map[string]string{
		"var": "var: ${1:name}", "env": "env: ${1:NAME}", "config": "config: ${1:server.port}",
		"file":   "file: {base: ${1|resources,templates,static,hyperbricks,render,module|}, path: ${2:relative/path}}",
		"path":   "path: {base: ${1|resources,templates,static,hyperbricks,render,module|}, path: ${2:relative/path}}",
		"format": "format: \"${1:%s}\", args: [${2:value}]",
	}
	for i := range items {
		items[i].InsertText = texts[items[i].Label]
	}
	return items
}

func (a *Analyzer) importCompletions(uri string) []CompletionItem {
	path, err := uriToPath(uri)
	if err != nil {
		return nil
	}
	if _, err = a.confinedSourcePath(path); err != nil {
		return nil
	}
	base := a.directory("hyperbricks", "hyperbricks")
	items := []CompletionItem{}
	_ = filepath.WalkDir(base, func(candidate string, entry fs.DirEntry, err error) error {
		if err != nil || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if len(items) >= 500 {
			return fs.SkipAll
		}
		if candidate == path || !strings.HasSuffix(candidate, ".hyperbricks.yaml") {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(path), candidate)
		if err != nil {
			return nil
		}
		value := filepath.ToSlash(rel)
		items = append(items, CompletionItem{Label: value, Kind: CompletionItemKindFile, InsertText: strconv.Quote(value)})
		return nil
	})
	return items
}

func (a *Analyzer) resolverCompletionCandidates(uri string, documents map[string]string, c *yamlCompletionCursor, componentIndex int) ([]CompletionItem, bool) {
	for index := len(c.trail) - 1; index >= 0; index-- {
		edge := c.trail[index]
		if index == componentIndex+1 && componentIndex >= 0 {
			continue
		}
		kind, _, options := sourceResolverKind(edge.node, edge.key)
		if kind == "" {
			continue
		}
		option := ""
		optionStart := index + 2
		if kind == "format" {
			optionStart = index + 1
		}
		for _, child := range c.trail[optionStart:] {
			if child.key != "" {
				option = child.key
			}
		}
		if option == "default" || option == "args" {
			if c.value && c.prefix == "" {
				return inlineResolverCompletions(), true
			}
			if !c.value {
				return resolverKeyCompletions(c.indent, c.flow), true
			}
			return nil, true
		}
		if option == "parts" {
			return nil, true
		}
		if !c.value {
			keys := []string{}
			switch kind {
			case "file", "path":
				keys = []string{"base", "path", "parts"}
			case "template.file":
				keys = []string{"path", "parts"}
			case "var":
				keys = []string{"name", "default"}
			case "env":
				keys = []string{"name", "default", "required"}
			case "config":
				keys = []string{"path", "default"}
			case "format":
				keys = []string{"args"}
			}
			existing := mappingKeys(options)
			items := []CompletionItem{}
			for _, key := range keys {
				if existing[key] || key == "parts" && existing["path"] || key == "path" && existing["parts"] {
					continue
				}
				insert := key + ": ${1:value}"
				switch key {
				case "base":
					insert = "base: ${1|resources,templates,static,hyperbricks,render,module|}"
				case "parts", "args":
					insert = key + ": [${1:value}]"
				case "required":
					insert = "required: ${1|true,false|}"
				}
				items = append(items, CompletionItem{Label: key, Kind: CompletionItemKindField, InsertText: insert, InsertTextFormat: InsertTextFormatSnippet, Documentation: MarkupContent{Kind: "markdown", Value: resolverOptionHelp(kind, key)}})
			}
			return items, true
		}
		if option == "base" {
			return pathBaseCompletions(), true
		}
		if option == "required" && kind == "env" {
			return literalCompletions("true", "false"), true
		}
		if kind == "var" && (option == "" || option == "name") {
			return a.variableCompletions(uri, c.referenceSource, documents), true
		}
		if kind == "config" && (option == "" || option == "path") {
			return a.configCompletions(documents), true
		}
		if kind == "template.file" && (option == "" || option == "path") {
			return a.pathCompletions(pathCompletionSpec{directoryKey: "templates", fallback: "templates", resolver: "relative"}), true
		}
		if (kind == "path" || kind == "file") && option == "path" {
			baseNode := definitionMappingValue(options, "base")
			if baseNode == nil || baseNode.Kind != yaml.ScalarNode {
				return nil, true
			}
			base := strings.TrimSpace(baseNode.Value)
			switch base {
			case "resources", "templates", "static", "hyperbricks":
				return a.pathCompletions(pathCompletionSpec{directoryKey: base, fallback: base, resolver: "relative"}), true
			case "render":
				return a.pathCompletions(pathCompletionSpec{directoryKey: base, fallback: "rendered", resolver: "relative"}), true
			// Do not walk the whole checkout, parent module collection or root.
			default:
				return nil, true
			}
		}
		return nil, true
	}
	// The template preload wrapper is also useful before its `file` key exists.
	if len(c.trail) > 0 {
		last := c.trail[len(c.trail)-1]
		for index, edge := range c.trail {
			if edge.key == "template" && index > componentIndex && !c.value && (edge.node.Kind == yaml.ScalarNode || last.keyPosition) {
				return []CompletionItem{{Label: "file", Kind: CompletionItemKindFile, Detail: "Preload a file from the templates directory", InsertText: "file: ${1:template.html}", InsertTextFormat: InsertTextFormatSnippet}}, true
			}
		}
	}
	return nil, false
}

func (a *Analyzer) variableCompletions(uri, text string, documents map[string]string) []CompletionItem {
	paths := map[string]bool{}
	for _, source := range a.reachableSources(uri, text, documents) {
		doc, err := decodeYAMLDocument(source)
		if err != nil {
			continue
		}
		collectCompletionPaths(definitionMappingValue(yamlDocumentBody(doc), "vars"), "", paths)
	}
	builtins := []string{"module"}
	if !a.isPackageConfig(uri) {
		builtins = []string{"module_root", "root", "module", "resources", "templates", "static", "hyperbricks", "render"}
	}
	for _, key := range builtins {
		paths[key] = true
	}
	return pathNameCompletions(paths, "Source variable")
}

func (a *Analyzer) configCompletions(documents map[string]string) []CompletionItem {
	path := a.config
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.moduleRoot(), path)
	}
	path, err := a.confinedPath(path)
	if err != nil {
		return nil
	}
	var content string
	if overlay, ok := documents[pathToURI(path)]; ok {
		content = overlay
	} else {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content = string(raw)
	}
	doc, err := decodeYAMLDocument(content)
	if err != nil {
		return nil
	}
	paths := map[string]bool{}
	collectCompletionPaths(yamlDocumentBody(doc), "", paths)
	for path := range paths {
		if path == "vars" || strings.HasPrefix(path, "vars.") {
			delete(paths, path)
		}
	}
	return pathNameCompletions(paths, "Package configuration path (value not disclosed)")
}

func collectCompletionPaths(node *yaml.Node, prefix string, paths map[string]bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index].Value, node.Content[index+1]
		if key == completionMarker {
			continue
		}
		path := joinTargetPath(prefix, key)
		paths[path] = true
		if kind, _, _ := sourceResolverKind(value, key); kind == "" {
			collectCompletionPaths(value, path, paths)
		}
	}
}

func pathNameCompletions(paths map[string]bool, detail string) []CompletionItem {
	keys := []string{}
	for key := range paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := []CompletionItem{}
	for _, key := range keys {
		items = append(items, CompletionItem{Label: key, Kind: CompletionItemKindReference, Detail: detail, InsertText: key})
	}
	return items
}
