package analysis

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"go.yaml.in/yaml/v4"
)

// Validate authored literals through the runtime policy owner. A resolver's
// value may depend on the deployment environment, so its existing materializer
// diagnostics own that expression; analysis must not guess a runtime value.
// Inspecting local fields also anchors inherited failures at their defining
// source rather than repeating them on each page that inherits the policy.
func validateRouteCacheSource(descriptor *componentType, componentPath string, node *yaml.Node) []Issue {
	if descriptor == nil || node == nil || isResolverExpression(node) {
		return nil
	}
	switch descriptor.Schema.Token {
	case composite.HyperMediaConfigGetName(), composite.FragmentConfigGetName():
	default:
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return routeCacheLiteralIssues(componentPath, "cache", node)
	}
	var issues []Issue
	for index := 0; index+1 < len(node.Content); index += 2 {
		field := strings.TrimSpace(node.Content[index].Value)
		switch field {
		case "storage", "expire":
			value := node.Content[index+1]
			if !isResolverExpression(value) {
				issues = append(issues, routeCacheLiteralIssues(componentPath, "cache."+field, value)...)
			}
		default:
			// Unknown keys already have a precise unsupported-field diagnostic
			// from validateNestedSchemaFields. Do not report them twice.
		}
	}
	return issues
}

func routeCacheLiteralIssues(componentPath, field string, node *yaml.Node) []Issue {
	var value interface{}
	err := node.Decode(&value)
	if err == nil {
		cacheValue := value
		if field != "cache" {
			cacheValue = map[string]interface{}{strings.TrimPrefix(field, "cache."): value}
		}
		_, err = composite.ResolveRouteCache(map[string]interface{}{"cache": cacheValue}, 0)
	}
	if err == nil {
		return nil
	}
	return []Issue{{
		Code: "component.invalid_cache", Severity: SeverityError,
		Message: fmt.Sprintf("invalid route cache policy: %s", err),
		Path:    componentPath + "." + field, Range: nodeRange(node),
	}}
}
