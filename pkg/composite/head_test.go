package composite_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestHeadRendererAppendsGeneratedItemsToYAMLOrder(t *testing.T) {
	rm := newHeadTestRenderManager()
	data := map[string]interface{}{
		"@type": "<HEAD>",
		"@order": []interface{}{
			"inline_styles",
		},
		"title": "Generated title",
		"inline_styles": map[string]interface{}{
			"@type":  "<CSS>",
			"inline": ".fixture { color: green; }",
		},
	}

	output, errs := rm.Render(composite.HeadConfigGetName(), data, context.Background())
	if len(errs) > 0 {
		t.Fatalf("render errors: %v", errs)
	}
	assertContainsInOrder(t, output,
		"<style>",
		`<meta name="generator" content="HyperBricks">`,
		"<title>Generated title</title>",
	)

	order, ok := data["@order"].([]interface{})
	if !ok {
		t.Fatalf("source data @order mutated; got %T", data["@order"])
	}
	if !reflect.DeepEqual(order, []interface{}{"inline_styles"}) {
		t.Fatalf("source data @order = %#v, want unchanged source order", order)
	}
}

func TestHeadRendererAllowsNamedGeneratedItemsToBeOverridden(t *testing.T) {
	rm := newHeadTestRenderManager()
	data := map[string]interface{}{
		"@type": "<HEAD>",
		"@order": []interface{}{
			"payload",
			"custom_head",
			"generator",
		},
		"title": "Generated title should not render",
		"payload": map[string]interface{}{
			"@type": "<HTML>",
			"value": "<title>Manual payload</title>",
		},
		"custom_head": map[string]interface{}{
			"@type": "<HTML>",
			"value": `<meta name="custom" content="yes">`,
		},
		"generator": map[string]interface{}{
			"@type": "<HTML>",
			"value": `<meta name="generator" content="manual">`,
		},
	}

	output, errs := rm.Render(composite.HeadConfigGetName(), data, context.Background())
	if len(errs) > 0 {
		t.Fatalf("render errors: %v", errs)
	}
	assertContainsInOrder(t, output,
		"<title>Manual payload</title>",
		`<meta name="custom" content="yes">`,
		`<meta name="generator" content="manual">`,
	)
	if strings.Contains(output, `<meta name="generator" content="HyperBricks">`) {
		t.Fatalf("default generator rendered despite explicit generator item:\n%s", output)
	}
	if strings.Contains(output, "Generated title should not render") {
		t.Fatalf("generated head payload rendered despite explicit payload item:\n%s", output)
	}
}

func newHeadTestRenderManager() *render.RenderManager {
	shared.Init_configuration()
	rm := render.NewRenderManager()
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}, reflect.TypeOf(composite.TreeConfig{}))
	rm.RegisterComponent(composite.HeadConfigGetName(), &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}, reflect.TypeOf(composite.HeadConfig{}))
	return rm
}

func assertContainsInOrder(t *testing.T, output string, parts ...string) {
	t.Helper()
	offset := 0
	for _, part := range parts {
		index := strings.Index(output[offset:], part)
		if index < 0 {
			t.Fatalf("output does not contain %q after byte %d:\n%s", part, offset, output)
		}
		offset += index + len(part)
	}
}
