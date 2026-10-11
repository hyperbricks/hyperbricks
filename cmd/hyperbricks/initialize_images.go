package main

import (
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// Discover complete effective IMAGE/IMAGES ownership without generating assets.
func prepareImageRouteConfigs(routes map[string]map[string]interface{}, diagnostics map[string][]error) map[string]component.ImageAssetPolicy {
	p := runtimeImageProcessor()
	if p == nil {
		return nil
	}
	p.SetAssetProtection(protectRuntimeAsset)
	active := map[string]component.ImageAssetPolicy{}
	valid := true
	var walk func(string, interface{})
	walk = func(route string, node interface{}) {
		switch raw := node.(type) {
		case map[string]interface{}:
			kind, _ := raw["@type"].(string)
			if kind == component.SingleImageConfigGetName() || kind == component.MultipleImagesConfigGetName() {
				response, err := rm.MakeInstance(typefactory.TypeRequest{TypeName: kind, Data: raw})
				var owners map[string]component.ImageAssetPolicy
				if err == nil {
					switch cfg := response.Instance.(type) {
					case component.SingleImageConfig:
						owners, err = p.DiscoverImages(&cfg, nil)
					case component.MultipleImagesConfig:
						owners, err = p.DiscoverImages(nil, &cfg)
					}
				}
				if err != nil {
					valid = false
					addRouteSourceErrors(diagnostics, route, []error{shared.Diagnostic(err, shared.MetaFromConfig(raw), "prepare")})
					return
				}
				for id, policy := range owners {
					old := active[id]
					policy.Keep = max(old.Keep, policy.Keep)
					policy.Grace = max(old.Grace, policy.Grace)
					active[id] = policy
				}
				return
			}
			for _, value := range raw {
				walk(route, value)
			}
		case []interface{}:
			for _, value := range raw {
				walk(route, value)
			}
		}
	}
	for route, raw := range routes {
		walk(route, raw)
	}
	if !valid {
		return nil
	}
	return active
}
func isImageOutput(path string) bool {
	p := runtimeImageProcessor()
	return p != nil && p.OwnsOutput(path)
}
