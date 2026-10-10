package language

import (
	"errors"
	"io"
	"sort"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v4"
)

const lintDiagnosticSource = "hyperbricks-lint"

// LintDocument checks YAML syntax, literal metadata types, and source style without
// resolving imports, executing components, or changing the document. Ranges use
// the same zero-based UTF-16 coordinates as the language server.
func LintDocument(source string) []Diagnostic {
	root, err := decodeYAMLDocument(source)
	if errors.Is(err, io.EOF) {
		return nil // A document containing only comments has no scalars to lint.
	}
	if err != nil {
		diagnostic := diagnosticFromError(source, err, "yaml.syntax")
		diagnostic.Source = lintDiagnosticSource
		return []Diagnostic{diagnostic}
	}
	diagnostics := moduleVersionTypeDiagnostics(source, root)
	diagnostics = append(diagnostics, unnecessaryQuoteDiagnostics(source, root)...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i].Range.Start, diagnostics[j].Range.Start
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Character < right.Character
	})
	return diagnostics
}

func moduleVersionTypeDiagnostics(source string, root *yaml.Node) []Diagnostic {
	metadata := definitionMappingValue(definitionMappingValue(yamlDocumentBody(root), "hyperbricks"), "metadata")
	if metadata == nil || metadata.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(metadata.Content); i += 2 {
		key, value := metadata.Content[i], metadata.Content[i+1]
		if key.Value != "moduleversion" {
			continue
		}
		if value.Kind == yaml.ScalarNode && value.Tag == "!!str" {
			return nil
		}
		// Lint does not materialize resolvers or follow aliases. Package
		// validation remains responsible for those source expressions.
		if kind, _, _ := sourceResolverKind(value, "moduleversion"); kind != "" || value.Kind == yaml.AliasNode {
			return nil
		}
		return []Diagnostic{{
			Range: yamlNodeRange(source, key), Severity: DiagnosticSeverityError,
			Code: "metadata.moduleversion_type", Source: lintDiagnosticSource,
			Message: `hyperbricks.metadata.moduleversion must be a YAML string. Use a full version such as 1.0.0, or quote a legacy version such as "1.0".`,
		}}
	}
	return nil
}

type quoteCandidate struct {
	start, end int
	value      string
	rangeValue Range
}

func unnecessaryQuoteDiagnostics(source string, root *yaml.Node) []Diagnostic {
	lines := strings.Split(source, "\n")
	offsets := make([]int, len(lines))
	for i := 1; i < len(lines); i++ {
		offsets[i] = offsets[i-1] + len(lines[i-1]) + 1
	}
	var candidates []quoteCandidate
	var visit func(*yaml.Node, bool)
	visit = func(node *yaml.Node, mappingKey bool) {
		if node.Kind == yaml.ScalarNode && node.Tag == "!!str" && node.Anchor == "" &&
			(node.Style == yaml.SingleQuotedStyle || node.Style == yaml.DoubleQuotedStyle) &&
			simplePlainString(node.Value) {
			line := node.Line - 1
			if line < 0 || line >= len(lines) || node.Column < 1 {
				return
			}
			runes := []rune(lines[line])
			if node.Column-1 >= len(runes) {
				return
			}
			prefix := string(runes[:node.Column-1])
			start := offsets[line] + len(prefix)
			end := start + len(node.Value) + 2
			quote := "\""
			if node.Style == yaml.SingleQuotedStyle {
				quote = "'"
			}
			// Escape spellings and multiline scalars deliberately remain untouched.
			if end > len(source) || source[start:end] != quote+node.Value+quote {
				return
			}
			// JSON-style flow mappings allow a quoted key immediately followed by
			// :value. Removing its quotes can change the mapping's interpretation.
			if mappingKey && end+1 < len(source) && source[end] == ':' &&
				!strings.ContainsRune(" \t\r\n", rune(source[end+1])) {
				return
			}
			character := utf16Length(prefix)
			candidates = append(candidates, quoteCandidate{
				start: start, end: end, value: node.Value,
				rangeValue: Range{
					Start: Position{Line: line, Character: character},
					End:   Position{Line: line, Character: character + utf16Length(node.Value) + 2},
				},
			})
		}
		for i, child := range node.Content {
			visit(child, node.Kind == yaml.MappingNode && i%2 == 0)
		}
	}
	visit(root, false)
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].start < candidates[j].start })
	// Validate all proposed removals together in their actual syntax context.
	// This requires one additional document parse, rather than one per scalar.
	var proposed strings.Builder
	previous := 0
	for _, candidate := range candidates {
		proposed.WriteString(source[previous:candidate.start])
		proposed.WriteString(candidate.value)
		previous = candidate.end
	}
	proposed.WriteString(source[previous:])
	reparsed, err := decodeYAMLDocument(proposed.String())
	if err != nil || !sameYAMLSemantics(root, reparsed) ||
		!equalStrings(yamlComments(root, nil), yamlComments(reparsed, nil)) {
		return nil
	}
	diagnostics := make([]Diagnostic, 0, len(candidates))
	for _, candidate := range candidates {
		diagnostics = append(diagnostics, Diagnostic{
			Range: candidate.rangeValue, Severity: DiagnosticSeverityWarning,
			Code: "yaml.unnecessary_quotes", Source: lintDiagnosticSource,
			Message: "Unnecessary quotes around a simple string; remove the surrounding quotes and keep the enclosed text unchanged.",
		})
	}
	return diagnostics
}

func simplePlainString(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return false
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !strings.ContainsRune(" _./-", r) {
			return false
		}
	}
	// Preserve spellings also interpreted as booleans by YAML 1.1 consumers.
	switch strings.ToLower(value) {
	case "y", "n", "yes", "no", "on", "off":
		return false
	}
	plain, err := decodeYAMLDocument(value)
	if err != nil {
		return false
	}
	body := yamlDocumentBody(plain)
	return body != nil && body.Kind == yaml.ScalarNode && body.Tag == "!!str" && body.Value == value
}
