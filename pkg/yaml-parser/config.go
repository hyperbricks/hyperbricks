package yamlparser

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v4"
)

// ConfigResult is the runtime-facing result for generic YAML configuration
// files such as package.hyperbricks.yaml. It uses YAML maps directly rather
// than the ordered component-tree profile.
type ConfigResult struct {
	Preprocessed string
	Materialized map[string]interface{}
	Diagnostics  []Diagnostic
}

// ProcessConfigFile loads and materializes a generic HyperBricks YAML
// configuration file.
func ProcessConfigFile(path string, opts Options) (*ConfigResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result, err := ProcessConfigBytes(raw, opts)
	if err != nil {
		return nil, err
	}
	applyDiagnosticSource(result.Diagnostics, path)
	return result, nil
}

// ProcessConfigBytes validates, parses, and resolves a generic HyperBricks YAML
// configuration document.
func ProcessConfigBytes(input []byte, opts Options) (*ConfigResult, error) {
	preprocessed, err := PreprocessBytes(input, opts)
	if err != nil {
		return nil, err
	}
	values, vars, err := parseConfigBytes(preprocessed)
	if err != nil {
		return nil, err
	}
	doc := &Document{Vars: vars}
	ctx := newValueResolverContext(doc, opts)
	resolved := materializeValueWithResolver(values, ctx, "")
	materialized, ok := resolved.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("generic YAML configuration did not materialize to a map")
	}
	delete(materialized, "vars")
	return &ConfigResult{
		Preprocessed: string(preprocessed),
		Materialized: materialized,
		Diagnostics:  ctx.diagnostics,
	}, nil
}

func parseConfigBytes(input []byte) (map[string]interface{}, map[string]interface{}, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(input))
	decoder.KnownFields(false)
	if err := decoder.Decode(&root); err != nil {
		return nil, nil, err
	}
	if len(root.Content) == 0 {
		return map[string]interface{}{}, nil, nil
	}
	body := root.Content[0]
	if body.Kind != yaml.MappingNode {
		return nil, nil, nodeError(body, "generic YAML configuration root must be a mapping")
	}
	values, err := parsePlainGenericMap(body)
	if err != nil {
		return nil, nil, err
	}
	var vars map[string]interface{}
	if rawVars, ok := values["vars"]; ok {
		vars, ok = rawVars.(map[string]interface{})
		if !ok {
			return nil, nil, nodeError(body, "vars must be a mapping")
		}
	}
	return values, vars, nil
}

func parsePlainGenericValue(node *yaml.Node) (interface{}, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return parseScalar(node)
	case yaml.MappingNode:
		return parsePlainGenericMap(node)
	case yaml.SequenceNode:
		values := make([]interface{}, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := parsePlainGenericValue(item)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.AliasNode:
		return nil, nodeError(node, "YAML aliases are not supported in HyperBricks configuration")
	default:
		return nil, nodeError(node, "unsupported YAML node kind %d", node.Kind)
	}
}

func parsePlainGenericMap(node *yaml.Node) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			return nil, nodeError(keyNode, "map key cannot be empty")
		}
		if _, exists := out[key]; exists {
			return nil, nodeError(keyNode, "duplicate map key %q", key)
		}
		value, err := parsePlainGenericValue(valueNode)
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, nil
}
