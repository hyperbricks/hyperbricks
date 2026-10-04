package main

import (
	"fmt"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Validate policies only at executable component positions. An API's values,
// request body, headers and other arbitrary maps are application data.
func validateResponseStatusPolicies(config map[string]interface{}) error {
	kind, _ := config["@type"].(string)
	if kind == component.APIConfigGetName() || kind == composite.ApiFragmentRenderConfigGetName() {
		if err := shared.ValidateResponseStatusFieldRaw(config); err != nil {
			return err
		}
	}
	if raw, exists := config["response_status"]; exists {
		if kind != component.APIConfigGetName() && kind != composite.ApiFragmentRenderConfigGetName() {
			return fmt.Errorf("response_status is supported only on api_render and api_fragment_render")
		}
		if err := shared.ValidateRawResponseStatus(raw); err != nil {
			return err
		}
	}
	visit := func(value interface{}) error {
		child, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		if _, executable := child["@type"].(string); executable {
			return validateResponseStatusPolicies(child)
		}
		return nil
	}
	switch kind {
	case composite.TemplateConfigGetName():
		values, _ := config["values"].(map[string]interface{})
		for _, value := range values {
			if err := visit(value); err != nil {
				return err
			}
		}
	case composite.HyperMediaConfigGetName(), composite.FragmentConfigGetName(), composite.TreeRendererConfigGetName(), composite.HeadConfigGetName():
		for key, value := range config {
			if key == "head" && kind == composite.HyperMediaConfigGetName() {
				// Hypermedia promotes its head options to an executable HEAD.
				if options, ok := value.(map[string]interface{}); ok {
					head := shared.CloneMapDeep(options)
					head["@type"] = composite.HeadConfigGetName()
					if err := validateResponseStatusPolicies(head); err != nil {
						return err
					}
				}
			}
			if key == "template" {
				// Route template options are a template component without @type.
				if options, ok := value.(map[string]interface{}); ok {
					values, _ := options["values"].(map[string]interface{})
					for _, child := range values {
						if err := visit(child); err != nil {
							return err
						}
					}
				}
			}
			if err := visit(value); err != nil {
				return err
			}
		}
	}
	return nil
}
