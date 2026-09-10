package renderplan

import (
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

// NeedsAPIRequestContext preserves the server's raw-config detection, including
// API markers inside data maps and slices outside the rendered component graph.
func NeedsAPIRequestContext(node interface{}) bool {
	switch typed := node.(type) {
	case map[string]interface{}:
		if configType, ok := typed["@type"].(string); ok {
			if configType == component.APIConfigGetName() || configType == composite.ApiFragmentRenderConfigGetName() {
				return true
			}
		}
		for _, value := range typed {
			if NeedsAPIRequestContext(value) {
				return true
			}
		}
	case []interface{}:
		for _, value := range typed {
			if NeedsAPIRequestContext(value) {
				return true
			}
		}
	}
	return false
}

// NeedsAPIRequestContext reports the requirement captured at compile time.
// Publishing a replacement plan refreshes it with the replacement raw config.
func (p *Plan) NeedsAPIRequestContext() bool {
	return p.needsAPIRequestContext
}
