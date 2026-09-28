package language

import (
	"bytes"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// FormatDocument formats YAML presentation without reordering nodes or
// changing scalar styles. The encoded document is reparsed and compared with
// the original syntax tree before an edit is returned.
func FormatDocument(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	original, err := decodeYAMLDocument(source)
	if err != nil {
		return "", err
	}
	if containsAlias(original) {
		return "", fmt.Errorf("formatting YAML aliases is not supported in HyperBricks source")
	}
	originalComments := yamlComments(original, nil)

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(original); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	formatted := output.String()
	if strings.Contains(source, "\r\n") {
		formatted = strings.ReplaceAll(formatted, "\n", "\r\n")
	}

	reparsed, err := decodeYAMLDocument(formatted)
	if err != nil {
		return "", fmt.Errorf("formatter produced invalid YAML: %w", err)
	}
	if !sameYAMLSemantics(original, reparsed) {
		return "", fmt.Errorf("formatter refused an edit that changes YAML meaning or entry order")
	}
	if !sameYAMLStyles(original, reparsed) {
		return "", fmt.Errorf("formatter refused an edit that changes YAML scalar or collection style")
	}
	if !equalStrings(originalComments, yamlComments(reparsed, nil)) {
		return "", fmt.Errorf("formatter refused an edit that does not preserve comments")
	}
	return formatted, nil
}

func sameYAMLSemantics(left, right *yaml.Node) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Kind != right.Kind || left.Tag != right.Tag || left.Value != right.Value || left.Anchor != right.Anchor || len(left.Content) != len(right.Content) {
		return false
	}
	for index := range left.Content {
		if !sameYAMLSemantics(left.Content[index], right.Content[index]) {
			return false
		}
	}
	return true
}

func sameYAMLStyles(left, right *yaml.Node) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Style != right.Style || len(left.Content) != len(right.Content) {
		return false
	}
	for index := range left.Content {
		if !sameYAMLStyles(left.Content[index], right.Content[index]) {
			return false
		}
	}
	return true
}

func containsAlias(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return true
	}
	for _, child := range node.Content {
		if containsAlias(child) {
			return true
		}
	}
	return false
}

type yamlComment struct {
	Head string
	Line string
	Foot string
}

func yamlComments(node *yaml.Node, comments []yamlComment) []yamlComment {
	if node == nil {
		return comments
	}
	if node.HeadComment != "" || node.LineComment != "" || node.FootComment != "" {
		comments = append(comments, yamlComment{Head: node.HeadComment, Line: node.LineComment, Foot: node.FootComment})
	}
	for _, child := range node.Content {
		comments = yamlComments(child, comments)
	}
	return comments
}

func equalStrings(left, right []yamlComment) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func documentEndPosition(text string) Position {
	lines := splitLines(text)
	if len(lines) == 0 {
		return Position{}
	}
	last := len(lines) - 1
	return Position{Line: last, Character: utf16Length(lines[last])}
}
