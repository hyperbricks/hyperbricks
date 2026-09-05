package main

import (
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// Preparation attaches immutable resources to the unpublished route snapshot,
// so legacy renders and compiled plans use the same resource generation.
func prepareGojaRouteConfigs(routes map[string]map[string]interface{}, diagnostics map[string][]error) {
	for route, config := range routes {
		var errs []error
		if !prepareGojaNodes(config, &errs) {
			continue
		}
		config["nocache"] = true
		headers := make(map[string]interface{})
		for key, value := range extractResponseHeaders(config) {
			if strings.EqualFold(key, "Cache-Control") {
				continue
			}
			headers[key] = value
		}
		headers["Cache-Control"] = "no-store"
		config["headers"] = headers
		addRouteSourceErrors(diagnostics, route, errs)
	}
}

func prepareGojaNodes(node interface{}, errs *[]error) bool {
	found := false
	switch raw := node.(type) {
	case map[string]interface{}:
		if raw["@type"] == component.GojaRenderConfigGetName() {
			// Never trust a source-authored value under this runtime-only key.
			delete(raw, component.GojaPreparedKey)
			response, err := rm.MakeInstance(typefactory.TypeRequest{TypeName: component.GojaRenderConfigGetName(), Data: raw})
			if err != nil {
				*errs = append(*errs, err)
				return true
			}
			prepared := component.PrepareGojaRender(response.Instance.(component.GojaRenderConfig), parser.GetTemplate)
			raw[component.GojaPreparedKey] = prepared
			if err := prepared.Err(); err != nil {
				_, componentErrors := prepared.Render(nil, nil)
				*errs = append(*errs, componentErrors...)
			}
			return true
		}
		for _, value := range raw {
			if prepareGojaNodes(value, errs) {
				found = true
			}
		}
	case []interface{}:
		for _, value := range raw {
			if prepareGojaNodes(value, errs) {
				found = true
			}
		}
	}
	return found
}
