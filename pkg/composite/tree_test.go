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

func TestTreeRendererUsesExplicitOrderExactly(t *testing.T) {
	rm := newTreeTestRenderManager()
	data := map[string]interface{}{
		"@type":  "<TREE>",
		"@order": []interface{}{"second"},
		"first": map[string]interface{}{
			"@type": "<HTML>",
			"value": "<p>first</p>",
		},
		"second": map[string]interface{}{
			"@type": "<HTML>",
			"value": "<p>second</p>",
		},
	}

	output, errs := rm.Render(composite.TreeRendererConfigGetName(), data, context.Background())
	if len(errs) > 0 {
		t.Fatalf("render errors: %v", errs)
	}
	if strings.Contains(output, "first") {
		t.Fatalf("unlisted item rendered despite explicit @order:\n%s", output)
	}
	if !strings.Contains(output, "second") {
		t.Fatalf("ordered item did not render:\n%s", output)
	}
}

func TestTreeRendererAppliesExplicitOrderPerNestedTree(t *testing.T) {
	rm := newTreeTestRenderManager()
	data := map[string]interface{}{
		"@type":  "<TREE>",
		"@order": []interface{}{"article"},
		"article": map[string]interface{}{
			"@type":  "<TREE>",
			"@order": []interface{}{"body"},
			"teaser": map[string]interface{}{
				"@type": "<HTML>",
				"value": "<p>teaser</p>",
			},
			"body": map[string]interface{}{
				"@type":  "<TREE>",
				"@order": []interface{}{"copy"},
				"heading": map[string]interface{}{
					"@type": "<HTML>",
					"value": "<h1>heading</h1>",
				},
				"copy": map[string]interface{}{
					"@type": "<HTML>",
					"value": "<p>copy</p>",
				},
			},
		},
	}

	output, errs := rm.Render(composite.TreeRendererConfigGetName(), data, context.Background())
	if len(errs) > 0 {
		t.Fatalf("render errors: %v", errs)
	}
	if strings.Contains(output, "teaser") || strings.Contains(output, "heading") {
		t.Fatalf("unlisted nested item rendered despite explicit nested @order:\n%s", output)
	}
	if !strings.Contains(output, "copy") {
		t.Fatalf("ordered nested item did not render:\n%s", output)
	}
}

func TestTreeRendererKeepsLegacySortedFallbackWithoutOrder(t *testing.T) {
	rm := newTreeTestRenderManager()
	data := map[string]interface{}{
		"@type": "<TREE>",
		"b": map[string]interface{}{
			"@type": "<HTML>",
			"value": "b",
		},
		"a": map[string]interface{}{
			"@type": "<HTML>",
			"value": "a",
		},
	}

	output, errs := rm.Render(composite.TreeRendererConfigGetName(), data, context.Background())
	if len(errs) > 0 {
		t.Fatalf("render errors: %v", errs)
	}
	assertContainsInOrder(t, output, "a", "b")
}

func newTreeTestRenderManager() *render.RenderManager {
	shared.Init_configuration()
	rm := render.NewRenderManager()
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}, reflect.TypeOf(composite.TreeConfig{}))
	return rm
}
