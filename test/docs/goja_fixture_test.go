package main

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// Standalone doc fixtures bypass module loading, so prepare their script leaf
// explicitly before exercising the ordinary renderer.
func prepareYAMLProfileGoja(t *testing.T, rm *render.RenderManager, scope map[string]interface{}) {
	t.Helper()
	if scope["@type"] != component.GojaRenderConfigGetName() {
		return
	}
	response, err := rm.MakeInstance(typefactory.TypeRequest{TypeName: component.GojaRenderConfigGetName(), Data: scope})
	if err != nil {
		t.Fatal(err)
	}
	p := component.PrepareGojaRender(response.Instance.(component.GojaRenderConfig), nil)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	scope[component.GojaPreparedKey] = p
}
