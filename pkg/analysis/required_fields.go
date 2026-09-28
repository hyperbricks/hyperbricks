package analysis

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Requiredness is checked against materialized values, but feedback must
// distinguish an absent field from a present expression that resolves empty.
// Do not repeat evaluation here: it could read resources twice or expose an
// environment/configuration value in a diagnostic.
func requiredFieldIssue(typeName, componentPath, field string, sequence *yaml.Node, value interface{}, present bool) Issue {
	entries := sequenceEntries(sequence)
	anchor := entries["type"]
	if anchor == nil {
		anchor = entries["inherit"]
	}
	if anchor == nil && len(sequence.Content) > 0 {
		anchor = sequence.Content[0]
	}
	issue := Issue{
		Code: "component.missing_required_field", Severity: SeverityError,
		Message: fmt.Sprintf("%s requires field %q", typeName, field), Path: componentPath + "." + field,
		Range: nodeRange(anchor),
	}
	if !present {
		return issue
	}

	issue.Code = "component.empty_required_field"
	what := "an empty value"
	if _, ok := value.(string); ok {
		what = "an empty string"
	}
	issue.Message = fmt.Sprintf("%s.%s resolves to %s; a non-empty value is required", typeName, field, what)
	for _, item := range sequence.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 || strings.TrimSpace(item.Content[0].Value) != field {
			continue
		}
		// The authored field owns this finding. A resolver mapping has no
		// scalar spelling, so its field key is a stable, useful edit target in
		// both block and flow YAML instead of squiggling an unrelated type.
		issue.Range = nodeRange(item.Content[0])
		if reference := requiredFileReference(item.Content[1]); reference != "" {
			issue.Message += fmt.Sprintf(" (file: %s)", reference)
		}
		return issue
	}
	// No local field exists: the empty value comes from inheritance. Anchor
	// the reference rather than pretending the author omitted the field.
	if inherited := entries["inherit"]; inherited != nil {
		issue.Range = nodeRange(inherited)
		issue.Message += " (inherited)"
	}
	return issue
}

// Show only the literal file reference already written by the user. Dynamic
// parts are intentionally not evaluated or guessed. A missing/unreadable file
// has its own resolver diagnostic; this message says only that its resulting
// field is empty, not that a read succeeded or that the file itself was empty.
func requiredFileReference(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.MappingNode || len(node.Content) != 2 || node.Content[0].Value != "file" {
		return ""
	}
	spec := node.Content[1]
	if spec.Kind == yaml.ScalarNode {
		return spec.Value
	}
	if spec.Kind != yaml.MappingNode {
		return ""
	}
	var base, path string
	var parts *yaml.Node
	pathPresent := false
	for index := 0; index+1 < len(spec.Content); index += 2 {
		key, value := spec.Content[index], spec.Content[index+1]
		switch key.Value {
		case "base":
			if value.Kind != yaml.ScalarNode {
				return ""
			}
			base = value.Value
		case "path":
			if value.Kind != yaml.ScalarNode {
				return ""
			}
			path, pathPresent = value.Value, true
		case "parts":
			parts = value
		}
	}
	if !pathPresent && parts != nil {
		if parts.Kind != yaml.SequenceNode {
			return ""
		}
		segments := make([]string, 0, len(parts.Content))
		for _, part := range parts.Content {
			if part.Kind != yaml.ScalarNode {
				return ""
			}
			segments = append(segments, part.Value)
		}
		path = filepath.ToSlash(filepath.Join(segments...))
	}
	if path == "" {
		return ""
	}
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + "/" + path
}
