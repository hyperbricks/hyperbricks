package main

import (
	"sort"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// Preparation binds effective inherited configs without compiling any assets.
func prepareEsbuildRouteConfigs(routes map[string]map[string]interface{}, diagnostics map[string][]error) {
	renderer, ok := rm.GetRenderComponent(component.EsbuildConfigGetName()).(*component.EsbuildRenderer)
	if !ok {
		return
	}
	renderer.Invalidate()
	type preparedRoute struct {
		route    string
		prepared *component.PreparedEsbuild
	}
	byOutput := make(map[string][]preparedRoute)
	var prepared []preparedRoute
	var walk func(string, interface{})
	walk = func(route string, node interface{}) {
		switch raw := node.(type) {
		case map[string]interface{}:
			if raw["@type"] == component.EsbuildConfigGetName() {
				delete(raw, component.EsbuildPreparedKey)
				response, err := rm.MakeInstance(typefactory.TypeRequest{TypeName: component.EsbuildConfigGetName(), Data: raw})
				if err != nil {
					addRouteSourceErrors(diagnostics, route, []error{err})
					return
				}
				p := renderer.Prepare(response.Instance.(component.EsbuildConfig))
				raw[component.EsbuildPreparedKey] = p
				item := preparedRoute{route: route, prepared: p}
				prepared = append(prepared, item)
				if p.Err() == nil {
					output, _ := p.BuildIdentity()
					byOutput[output] = append(byOutput[output], item)
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
	names := make([]string, 0, len(routes))
	for name := range routes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		walk(name, routes[name])
	}
	for _, items := range byOutput {
		_, first := items[0].prepared.BuildIdentity()
		conflict := false
		for _, item := range items {
			_, key := item.prepared.BuildIdentity()
			conflict = conflict || key != first
		}
		if conflict {
			for _, item := range items {
				item.prepared.RejectOutputConflict()
			}
		}
	}
	for _, item := range prepared {
		if err := item.prepared.Err(); err != nil {
			addRouteSourceErrors(diagnostics, item.route, []error{err})
		}
	}
}

func isEsbuildOutput(path string) bool {
	if rm == nil {
		return false
	}
	renderer, ok := rm.GetRenderComponent(component.EsbuildConfigGetName()).(*component.EsbuildRenderer)
	return ok && renderer.OwnsOutput(path)
}
