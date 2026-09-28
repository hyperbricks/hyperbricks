package analysis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

// Resolver diagnostics come from the same materializer as runtime. Keep their
// severity and source ownership, but turn point locations into editor ranges.
// An inherited value may be evaluated once per page; show its source warning
// once, not once per inheriting page.
func resolverIssues(diagnostics []yamlparser.Diagnostic, sources []sourceDocument) []Issue {
	issues := make([]Issue, 0, len(diagnostics))
	seen := make(map[string]bool)
	for _, diagnostic := range diagnostics {
		severity := SeverityWarning
		if diagnostic.Level == "error" {
			severity = SeverityError
		} else if diagnostic.Level == "info" {
			severity = SeverityInformation
		}
		location := rangeAt(diagnostic.Line, diagnostic.Column, 1)
		file := diagnostic.Source
		variableOwned := false
		if strings.HasPrefix(diagnostic.Path, "vars.") {
			for _, source := range sources {
				if node := sourceValueAtPath(source.root, strings.Split(diagnostic.Path, ".")); node != nil {
					location, file, variableOwned = resolverNodeRange(node), source.file, true
					break
				}
			}
		}
		for _, source := range sources {
			if variableOwned {
				break
			}
			if file != "" && file != source.file {
				continue
			}
			if node := sourceNodeAtPosition(source.root, diagnostic.Line, diagnostic.Column); node != nil {
				if diagnostic.Code == "path_base_unknown" {
					node = resolverBaseNode(node)
				}
				location = resolverNodeRange(node)
				if file == "" {
					file = source.file
				}
				break
			}
		}
		key := fmt.Sprintf("%s:%d:%d:%s:%s", file, location.Start.Line, location.Start.Column, diagnostic.Code, diagnostic.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		issues = append(issues, Issue{
			Code: diagnostic.Code, Severity: severity, Message: diagnostic.Message,
			File: file, Path: diagnostic.Path, Range: location,
		})
	}
	sort.SliceStable(issues, func(left, right int) bool {
		a, b := issues[left], issues[right]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Range.Start.Line != b.Range.Start.Line {
			return a.Range.Start.Line < b.Range.Start.Line
		}
		if a.Range.Start.Column != b.Range.Start.Column {
			return a.Range.Start.Column < b.Range.Start.Column
		}
		return a.Code < b.Code
	})
	return issues
}

func resolverBaseNode(node *yaml.Node) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return node
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == "base" {
			return node.Content[index+1]
		}
	}
	if len(node.Content) == 2 && (node.Content[0].Value == "file" || node.Content[0].Value == "path") {
		return resolverBaseNode(node.Content[1])
	}
	return node
}

func sourceNodeAtPosition(node *yaml.Node, line, column int) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Line == line && node.Column == column {
		return node
	}
	for _, child := range node.Content {
		if found := sourceNodeAtPosition(child, line, column); found != nil {
			return found
		}
	}
	return nil
}

func sourceValueAtPath(node *yaml.Node, path []string) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return sourceValueAtPath(node.Content[0], path)
	}
	if len(path) == 0 {
		return node
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == path[0] {
			return sourceValueAtPath(node.Content[index+1], path[1:])
		}
	}
	return nil
}

func resolverNodeRange(node *yaml.Node) Range {
	// Select the directive in both block and flow mappings. Do not pretend a
	// decoded multiline scalar has one source-line-wide spelling.
	if node.Kind == yaml.MappingNode && len(node.Content) >= 2 {
		return nodeRange(node.Content[0])
	}
	if node.Kind == yaml.ScalarNode && !strings.ContainsAny(node.Value, "\r\n") {
		return nodeRange(node)
	}
	return rangeAt(node.Line, node.Column, 1)
}

var missingResolverColon = regexp.MustCompile(`^(\s*-\s+)([A-Za-z_][A-Za-z0-9_-]*)(\s+)(\{\s*(?:file|path|var|env|config|format)\s*:)`)

// The omitted colon before a flow resolver often makes YAML report an error at
// its closing brace. Only replace that generic error when inserting one colon
// makes valid YAML and the repaired entry belongs to a real component sequence.
func missingResolverSeparator(input []byte) (Issue, bool) {
	lines := strings.Split(string(input), "\n")
	for index, line := range lines {
		match := missingResolverColon.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		keyStart, keyEnd := match[4], match[5]
		repaired := append([]string(nil), lines...)
		repaired[index] = line[:keyEnd] + ":" + line[keyEnd:]
		root, err := decodeYAML([]byte(strings.Join(repaired, "\n")))
		if err != nil || !componentEntryAtLine(root, index+1) {
			continue
		}
		key := line[keyStart:keyEnd]
		return Issue{
			Code: "yaml.missing_separator", Severity: SeverityError,
			Message: fmt.Sprintf("missing ':' after component field %q; write '- %s: {...}' to assign a resolver", key, key),
			Range:   rangeAt(index+1, utf8.RuneCountInString(line[:keyStart])+1, utf8.RuneCountInString(key)),
		}, true
	}
	return Issue{}, false
}

func componentEntryAtLine(node *yaml.Node, line int) bool {
	if node == nil {
		return false
	}
	if isComponentSequence(node) {
		for _, item := range node.Content {
			if item.Kind == yaml.MappingNode && len(item.Content) == 2 && item.Content[0].Line == line {
				return true
			}
		}
	}
	for _, child := range node.Content {
		if componentEntryAtLine(child, line) {
			return true
		}
	}
	return false
}
