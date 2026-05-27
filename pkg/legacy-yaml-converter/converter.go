package legacyyamlconverter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Options struct {
	SourcePath string
}

type Result struct {
	YAML     string
	Warnings []string
}

type document struct {
	imports []string
	roots   []*node
	rootBy  map[string]*node
	pathMap map[string][]string
	warn    []string
}

type node struct {
	name     string
	typ      string
	inherit  string
	props    orderedMap
	children []*node
	childBy  map[string]*node
}

type orderedMap struct {
	keys []string
	vals map[string]value
}

type value struct {
	scalar *scalarValue
	array  []value
	object *orderedMap
	node   *node
}

type scalarValue struct {
	text  string
	block bool
}

type opKind int

const (
	opSet opKind = iota
	opInherit
	opOpenMap
)

type op struct {
	kind  opKind
	path  []string
	value string
	block bool
	line  int
}

var (
	importLinePattern = regexp.MustCompile(`^\s*@import\s+["']([^"']+)["']\s*$`)
	typeTokenPattern  = regexp.MustCompile(`^<([A-Za-z0-9_]+)>$`)
)

func ConvertBytes(input []byte, opts Options) (*Result, error) {
	ops, imports, warnings, err := parseLegacyOps(string(input))
	if err != nil {
		return nil, err
	}
	doc := newDocument()
	doc.imports = convertImports(imports)
	doc.warn = append(doc.warn, warnings...)
	for _, operation := range ops {
		if err := doc.apply(operation); err != nil {
			return nil, err
		}
	}
	doc.resolveInheritReferences()

	var builder strings.Builder
	writeDocument(&builder, doc)
	return &Result{
		YAML:     builder.String(),
		Warnings: doc.warn,
	}, nil
}

func ConvertFile(path string, opts Options) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if opts.SourcePath == "" {
		opts.SourcePath = path
	}
	return ConvertBytes(data, opts)
}

func ConvertDirectory(sourceDir string, targetDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(sourceDir, "*.hyperbricks"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}

	written := make([]string, 0, len(files))
	for _, sourceFile := range files {
		result, err := ConvertFile(sourceFile, Options{SourcePath: sourceFile})
		if err != nil {
			return written, fmt.Errorf("convert %s: %w", sourceFile, err)
		}
		targetName := strings.TrimSuffix(filepath.Base(sourceFile), ".hyperbricks") + ".hyperbricks.yaml"
		targetFile := filepath.Join(targetDir, targetName)
		if err := os.WriteFile(targetFile, []byte(result.YAML), 0o644); err != nil {
			return written, err
		}
		written = append(written, targetFile)
	}
	return written, nil
}

func newDocument() *document {
	return &document{
		rootBy:  make(map[string]*node),
		pathMap: make(map[string][]string),
	}
}

func parseLegacyOps(input string) ([]op, []string, []string, error) {
	lines := strings.Split(input, "\n")
	var ops []op
	var imports []string
	var warnings []string
	var prefixStack [][]string

	for index := 0; index < len(lines); index++ {
		lineNumber := index + 1
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@macro") {
			warnings = append(warnings, fmt.Sprintf("line %d: macros are not converted by the source converter", lineNumber))
			continue
		}
		if match := importLinePattern.FindStringSubmatch(line); len(match) == 2 {
			imports = append(imports, match[1])
			continue
		}
		if strings.HasPrefix(line, "$") {
			continue
		}
		if line == "}" {
			if len(prefixStack) == 0 {
				return nil, nil, nil, fmt.Errorf("line %d: unmatched closing brace", lineNumber)
			}
			prefixStack = prefixStack[:len(prefixStack)-1]
			continue
		}

		prefix := currentPrefix(prefixStack)
		if strings.HasSuffix(line, "{") {
			beforeBrace := strings.TrimSpace(strings.TrimSuffix(line, "{"))
			if strings.HasSuffix(beforeBrace, "=") {
				beforeBrace = strings.TrimSpace(strings.TrimSuffix(beforeBrace, "="))
			}
			path := appendPath(prefix, splitPath(beforeBrace)...)
			ops = append(ops, op{kind: opOpenMap, path: path, line: lineNumber})
			prefixStack = append(prefixStack, path)
			continue
		}

		if key, ref, ok := splitInherit(line); ok {
			ops = append(ops, op{
				kind:  opInherit,
				path:  appendPath(prefix, splitPath(key)...),
				value: ref,
				line:  lineNumber,
			})
			continue
		}

		eq := strings.Index(line, "=")
		if eq == -1 {
			warnings = append(warnings, fmt.Sprintf("line %d: skipped unrecognized line %q", lineNumber, line))
			continue
		}
		key := strings.TrimSpace(line[:eq])
		rawValue := strings.TrimSpace(line[eq+1:])
		valueText, block, consumed, err := collectValue(rawValue, lines[index+1:])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		index += consumed
		ops = append(ops, op{
			kind:  opSet,
			path:  appendPath(prefix, splitPath(key)...),
			value: valueText,
			block: block,
			line:  lineNumber,
		})
	}
	if len(prefixStack) > 0 {
		return nil, nil, nil, fmt.Errorf("unterminated block at end of input")
	}
	return ops, imports, warnings, nil
}

func collectValue(raw string, remaining []string) (string, bool, int, error) {
	if !strings.HasPrefix(raw, "<<[") {
		return raw, false, 0, nil
	}
	value := strings.TrimPrefix(raw, "<<[")
	if strings.HasSuffix(value, "]>>") {
		return strings.TrimSuffix(value, "]>>"), true, 0, nil
	}

	var builder strings.Builder
	builder.WriteString(value)
	if value != "" {
		builder.WriteString("\n")
	}
	for index, line := range remaining {
		trimmedRight := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmedRight, "]>>") {
			builder.WriteString(strings.TrimSuffix(trimmedRight, "]>>"))
			return stripCommonLeadingSpaces(builder.String()), true, index + 1, nil
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return "", false, 0, fmt.Errorf("unterminated multiline value")
}

func splitInherit(line string) (string, string, bool) {
	if strings.Contains(line, "=") || strings.Count(line, "<<<") != 1 {
		return "", "", false
	}
	parts := strings.SplitN(line, "<<<", 2)
	key := strings.TrimSpace(parts[0])
	ref := strings.TrimSpace(parts[1])
	return key, ref, key != "" && ref != ""
}

func splitPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func appendPath(base []string, rest ...string) []string {
	out := append([]string(nil), base...)
	out = append(out, rest...)
	return out
}

func currentPrefix(stack [][]string) []string {
	if len(stack) == 0 {
		return nil
	}
	return stack[len(stack)-1]
}

func convertImports(imports []string) []string {
	out := make([]string, 0, len(imports))
	for _, item := range imports {
		item = strings.TrimSpace(item)
		if strings.HasSuffix(item, ".hyperbricks") {
			item = strings.TrimSuffix(item, ".hyperbricks") + ".hyperbricks.yaml"
		}
		out = append(out, item)
	}
	return out
}

func (d *document) apply(operation op) error {
	if len(operation.path) == 0 {
		return nil
	}
	switch operation.kind {
	case opOpenMap:
		d.ensureMap(operation.path)
	case opInherit:
		target := d.ensureNode(operation.path, "", operation.value)
		target.inherit = operation.value
	case opSet:
		if typ, ok := parseType(operation.value); ok {
			target := d.ensureNode(operation.path, typ, "")
			target.typ = typ
			return nil
		}
		if strings.TrimSpace(operation.value) == "{" {
			d.ensureMap(operation.path)
			return nil
		}
		d.setValue(operation.path, parseLegacyValue(operation.value, operation.block))
	}
	return nil
}

func (d *document) ensureRoot(name string) *node {
	if existing := d.rootBy[name]; existing != nil {
		return existing
	}
	root := &node{
		name:    name,
		props:   newOrderedMap(),
		childBy: make(map[string]*node),
	}
	d.rootBy[name] = root
	d.roots = append(d.roots, root)
	d.pathMap[name] = []string{name}
	return root
}

func (d *document) ensureNode(path []string, hintType string, inherit string) *node {
	if len(path) == 1 {
		root := d.ensureRoot(path[0])
		if hintType != "" {
			root.typ = hintType
		}
		return root
	}

	parentNode, parentLegacyPath, propPath := d.locateParentNode(path[:len(path)-1])
	targetLegacyName := path[len(path)-1]
	targetYAMLName := d.yamlNameFor(parentLegacyPath, targetLegacyName, hintType, inherit)

	if len(propPath) == 0 {
		if existing := parentNode.childBy[targetLegacyName]; existing != nil {
			if hintType != "" {
				existing.typ = hintType
			}
			return existing
		}
		child := &node{
			name:    targetYAMLName,
			props:   newOrderedMap(),
			childBy: make(map[string]*node),
		}
		if hintType != "" {
			child.typ = hintType
		}
		parentNode.childBy[targetLegacyName] = child
		parentNode.children = append(parentNode.children, child)
		d.pathMap[joinLegacyPath(path)] = append(d.pathMap[joinLegacyPath(parentLegacyPath)], targetYAMLName)
		return child
	}

	propMap := ensurePropMap(parentNode, propPath)
	if existing := propMap.vals[targetLegacyName]; existing.node != nil {
		if hintType != "" {
			existing.node.typ = hintType
		}
		return existing.node
	}
	child := &node{
		name:    targetYAMLName,
		props:   newOrderedMap(),
		childBy: make(map[string]*node),
	}
	if hintType != "" {
		child.typ = hintType
	}
	propMap.set(targetLegacyName, value{node: child})
	d.pathMap[joinLegacyPath(path)] = append(d.pathMap[joinLegacyPath(parentLegacyPath)], append(propPath, targetYAMLName)...)
	return child
}

func (d *document) ensureMap(path []string) *orderedMap {
	if len(path) == 1 {
		return &d.ensureRoot(path[0]).props
	}
	parentNode, _, propPath := d.locateParentNode(path)
	return ensurePropMap(parentNode, propPath)
}

func (d *document) setValue(path []string, val value) {
	if len(path) == 1 {
		root := d.ensureRoot(path[0])
		root.props.set(path[0], val)
		return
	}
	parentNode, _, propPath := d.locateParentNode(path[:len(path)-1])
	if len(propPath) == 0 {
		parentNode.props.set(path[len(path)-1], val)
		return
	}
	propMap := ensurePropMap(parentNode, propPath)
	propMap.set(path[len(path)-1], val)
}

func (d *document) locateParentNode(path []string) (*node, []string, []string) {
	root := d.ensureRoot(path[0])
	best := []string{path[0]}
	for end := len(path); end >= 1; end-- {
		d.ensureInheritedAlias(path[:end])
		candidate := joinLegacyPath(path[:end])
		if _, ok := d.pathMap[candidate]; ok {
			best = path[:end]
			break
		}
	}
	if len(best) == 1 && len(path) > 1 && path[1] == "head" {
		head := d.ensureNode([]string{path[0], "head"}, "head", "")
		return head, []string{path[0], "head"}, path[2:]
	}
	node := d.nodeByYAMLPath(d.pathMap[joinLegacyPath(best)])
	if node == nil {
		node = root
	}
	return node, best, path[len(best):]
}

func (d *document) ensureInheritedAlias(path []string) {
	if len(path) < 2 {
		return
	}
	root := d.rootBy[path[0]]
	if root == nil || root.inherit == "" {
		return
	}
	legacyKey := joinLegacyPath(path)
	if _, exists := d.pathMap[legacyKey]; exists {
		return
	}
	inheritedPath := root.inherit + "." + strings.Join(path[1:], ".")
	inheritedYAMLPath, ok := d.pathMap[inheritedPath]
	if !ok || len(inheritedYAMLPath) < 2 {
		return
	}
	aliasYAMLPath := append([]string{root.name}, inheritedYAMLPath[1:]...)
	d.pathMap[legacyKey] = aliasYAMLPath
	d.ensureOverlayNodeByYAMLPath(root, path[1:], inheritedYAMLPath[1:])
}

func (d *document) ensureOverlayNodeByYAMLPath(root *node, legacySegments []string, yamlSegments []string) {
	if len(yamlSegments) == 0 {
		return
	}
	current := root
	for index, yamlName := range yamlSegments {
		if index == 0 {
			legacyName := legacySegments[0]
			if existing := current.childBy[legacyName]; existing != nil {
				current = existing
				continue
			}
			child := &node{
				name:    yamlName,
				props:   newOrderedMap(),
				childBy: make(map[string]*node),
			}
			current.childBy[legacyName] = child
			current.children = append(current.children, child)
			current = child
			continue
		}
		// The current converter only needs to create the inherited child node
		// overlay. Deeper inherited paths are reached through normal property
		// traversal after the child exists.
		return
	}
}

func (d *document) nodeByYAMLPath(path []string) *node {
	if len(path) == 0 {
		return nil
	}
	current := d.rootBy[path[0]]
	if current == nil {
		return nil
	}
	var currentMap *orderedMap
	for _, segment := range path[1:] {
		if currentMap != nil {
			val := currentMap.vals[segment]
			switch {
			case val.node != nil:
				current = val.node
				currentMap = nil
			case val.object != nil:
				currentMap = val.object
			default:
				return nil
			}
			continue
		}

		foundChild := false
		for _, child := range current.children {
			if child.name == segment {
				current = child
				foundChild = true
				break
			}
		}
		if foundChild {
			continue
		}
		val := current.props.vals[segment]
		switch {
		case val.node != nil:
			current = val.node
		case val.object != nil:
			currentMap = val.object
		default:
			return nil
		}
	}
	return current
}

func ensurePropMap(parent *node, path []string) *orderedMap {
	current := &parent.props
	for _, part := range path {
		val := current.vals[part]
		if val.object == nil {
			m := newOrderedMap()
			val = value{object: &m}
			current.set(part, val)
		}
		current = val.object
	}
	return current
}

func (d *document) yamlNameFor(parentLegacyPath []string, legacyName string, hintType string, inherit string) string {
	if !isNumericKey(legacyName) {
		return legacyName
	}
	base := "item"
	if hintType != "" {
		base = normalizeIdentifier(hintType)
	}
	if hintType == "" && inherit != "" {
		base = normalizeIdentifier(lastPathSegment(inherit))
	}
	if base == "" {
		base = "item"
	}
	return base + "_" + legacyName
}

func parseLegacyValue(raw string, block bool) value {
	raw = strings.TrimSpace(raw)
	if block {
		return value{scalar: &scalarValue{text: raw, block: true}}
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
		if inner == "" {
			return value{array: []value{}}
		}
		parts := splitCommaList(inner)
		items := make([]value, 0, len(parts))
		for _, part := range parts {
			items = append(items, value{scalar: &scalarValue{text: strings.TrimSpace(part)}})
		}
		return value{array: items}
	}
	return value{scalar: &scalarValue{text: raw, block: strings.Contains(raw, "\n") || isFileMarker(raw)}}
}

func splitCommaList(input string) []string {
	var out []string
	var current strings.Builder
	for _, r := range input {
		if r == ',' {
			out = append(out, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if strings.TrimSpace(current.String()) != "" {
		out = append(out, strings.TrimSpace(current.String()))
	}
	return out
}

func parseType(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	match := typeTokenPattern.FindStringSubmatch(raw)
	if len(match) != 2 {
		return "", false
	}
	return strings.ToLower(match[1]), true
}

func newOrderedMap() orderedMap {
	return orderedMap{vals: make(map[string]value)}
}

func (m *orderedMap) set(key string, val value) {
	if m.vals == nil {
		m.vals = make(map[string]value)
	}
	if _, exists := m.vals[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = val
}

func (d *document) resolveInheritReferences() {
	for _, root := range d.roots {
		d.resolveNodeInherits(root)
	}
}

func (d *document) resolveNodeInherits(n *node) {
	if n == nil {
		return
	}
	if n.inherit != "" {
		if mapped, ok := d.pathMap[n.inherit]; ok {
			n.inherit = strings.Join(mapped, ".")
		}
	}
	for _, child := range n.children {
		d.resolveNodeInherits(child)
	}
	resolveMapNodeInherits(&n.props, d)
}

func resolveMapNodeInherits(m *orderedMap, d *document) {
	if m == nil {
		return
	}
	for _, key := range m.keys {
		val := m.vals[key]
		if val.node != nil {
			d.resolveNodeInherits(val.node)
		}
		if val.object != nil {
			resolveMapNodeInherits(val.object, d)
		}
	}
}

func writeDocument(builder *strings.Builder, doc *document) {
	if len(doc.imports) > 0 {
		builder.WriteString("imports:\n")
		for _, item := range doc.imports {
			writeIndent(builder, 1)
			builder.WriteString("- ")
			builder.WriteString(quoteString(item))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	for index, root := range doc.roots {
		if index > 0 {
			builder.WriteString("\n")
		}
		writeKey(builder, root.name)
		builder.WriteString(":\n")
		writeNodeBody(builder, root, 1)
	}
}

func writeNodeBody(builder *strings.Builder, n *node, indent int) {
	if n.typ != "" {
		writeIndent(builder, indent)
		builder.WriteString("- type: ")
		builder.WriteString(n.typ)
		builder.WriteString("\n")
	}
	if n.inherit != "" {
		writeIndent(builder, indent)
		builder.WriteString("- inherit: ")
		builder.WriteString(quoteString(n.inherit))
		builder.WriteString("\n")
	}
	for _, key := range n.props.keys {
		writeIndent(builder, indent)
		builder.WriteString("- ")
		writeKey(builder, key)
		builder.WriteString(":")
		writeValue(builder, n.props.vals[key], indent+2)
	}
	for _, child := range n.children {
		writeIndent(builder, indent)
		builder.WriteString("- ")
		writeKey(builder, child.name)
		builder.WriteString(":\n")
		writeNodeBody(builder, child, indent+2)
	}
}

func writeValue(builder *strings.Builder, val value, indent int) {
	switch {
	case val.node != nil:
		builder.WriteString("\n")
		writeNodeBody(builder, val.node, indent)
	case val.object != nil:
		builder.WriteString("\n")
		writeMap(builder, val.object, indent)
	case val.array != nil:
		if len(val.array) == 0 {
			builder.WriteString(" []\n")
			return
		}
		builder.WriteString("\n")
		for _, item := range val.array {
			writeIndent(builder, indent)
			builder.WriteString("-")
			writeValue(builder, item, indent+1)
		}
	case val.scalar != nil:
		if val.scalar.block {
			builder.WriteString(" |\n")
			writeBlockScalar(builder, val.scalar.text, indent)
		} else {
			builder.WriteString(" ")
			builder.WriteString(formatScalar(val.scalar.text))
			builder.WriteString("\n")
		}
	default:
		builder.WriteString(" \"\"\n")
	}
}

func writeMap(builder *strings.Builder, m *orderedMap, indent int) {
	for _, key := range m.keys {
		writeIndent(builder, indent)
		writeKey(builder, key)
		builder.WriteString(":")
		writeValue(builder, m.vals[key], indent+1)
	}
}

func writeBlockScalar(builder *strings.Builder, text string, indent int) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 {
		writeIndent(builder, indent)
		builder.WriteString("\n")
		return
	}
	for _, line := range lines {
		writeIndent(builder, indent)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}

func writeIndent(builder *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		builder.WriteString("  ")
	}
}

func writeKey(builder *strings.Builder, key string) {
	if isPlainKey(key) {
		builder.WriteString(key)
		return
	}
	builder.WriteString(quoteString(key))
}

func formatScalar(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "\"\""
	}
	if isPlainScalar(text) {
		return text
	}
	return quoteString(text)
}

func quoteString(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, `"`, `\"`)
	return `"` + text + `"`
}

func isPlainKey(text string) bool {
	if text == "" {
		return false
	}
	for index, r := range text {
		if index == 0 && !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
			return false
		}
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func isPlainScalar(text string) bool {
	if strings.ContainsAny(text, "{}[],:#&*!|>'\"%@`") {
		return false
	}
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "?") {
		return false
	}
	switch strings.ToLower(text) {
	case "true", "false", "null", "~":
		return false
	}
	return true
}

func isNumericKey(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isFileMarker(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "{{FILE:") && strings.HasSuffix(strings.TrimSpace(text), "}}")
}

func joinLegacyPath(path []string) string {
	return strings.Join(path, ".")
}

func normalizeIdentifier(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "<")
	text = strings.TrimSuffix(text, ">")
	text = strings.ReplaceAll(text, "-", "_")
	text = strings.ToLower(text)
	if text == "api_fragment_render" {
		return "api_fragment"
	}
	if text == "api_render" {
		return "api"
	}
	return text
}

func lastPathSegment(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func stripCommonLeadingSpaces(input string) string {
	lines := strings.Split(input, "\n")
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent <= 0 {
		return input
	}
	for i, line := range lines {
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}
	return strings.Join(lines, "\n")
}
