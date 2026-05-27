package render_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func benchmarkRenderManager() *render.RenderManager {
	shared.Init_configuration()
	rm := render.NewRenderManager()

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm},
	}
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm},
	}

	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))
	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))
	return rm
}

func benchmarkTextData() map[string]interface{} {
	return map[string]interface{}{
		"@type": "<TEXT>",
		"value": "A fast HyperBricks text component.",
	}
}

func benchmarkTreeData(children int) map[string]interface{} {
	items := map[string]interface{}{
		"@type": "<TREE>",
	}
	for i := 0; i < children; i++ {
		key := fmt.Sprintf("%03d", i+1)
		items[key] = map[string]interface{}{
			"@type": "<TEXT>",
			"value": fmt.Sprintf("Tree child %s", key),
		}
	}
	return items
}

func benchmarkTemplateScalarData() map[string]interface{} {
	return map[string]interface{}{
		"@type":     "<TEMPLATE>",
		"inline":    `<section><h1>{{ .title }}</h1><p>{{ .summary }}</p><a href="{{ .href }}">{{ .cta }}</a></section>`,
		"enclose":   `<main>|</main>`,
		"querykeys": []interface{}{"project", "space"},
		"values": map[string]interface{}{
			"title":   "Render flow",
			"summary": "Template values rendered through RenderManager.",
			"href":    "/docs/render-flow",
			"cta":     "Read",
		},
	}
}

func benchmarkTemplateChildData(children int) map[string]interface{} {
	values := map[string]interface{}{
		"title": "Render flow",
	}
	for i := 0; i < children; i++ {
		key := fmt.Sprintf("slot_%03d", i+1)
		values[key] = map[string]interface{}{
			"@type": "<TEXT>",
			"value": fmt.Sprintf("Template child %03d", i+1),
		}
	}
	return map[string]interface{}{
		"@type":  "<TEMPLATE>",
		"inline": `{{ .title }}{{ range $key, $value := . }}{{ if hasPrefix "slot_" $key }}<span>{{ $value }}</span>{{ end }}{{ end }}`,
		"values": values,
	}
}

func BenchmarkRenderManagerText(b *testing.B) {
	rm := benchmarkRenderManager()
	data := benchmarkTextData()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, errs := rm.Render(component.TextConfigGetName(), data, ctx)
		if len(errs) > 0 {
			b.Fatalf("Render returned errors: %v", errs)
		}
		if out == "" {
			b.Fatal("Render returned empty output")
		}
	}
}

func BenchmarkRenderManagerTree32Text(b *testing.B) {
	rm := benchmarkRenderManager()
	data := benchmarkTreeData(32)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, errs := rm.Render(composite.TreeRendererConfigGetName(), data, ctx)
		if len(errs) > 0 {
			b.Fatalf("Render returned errors: %v", errs)
		}
		if out == "" {
			b.Fatal("Render returned empty output")
		}
	}
}

func BenchmarkRenderManagerTemplateScalar(b *testing.B) {
	rm := benchmarkRenderManager()
	data := benchmarkTemplateScalarData()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, errs := rm.Render(composite.TemplateConfigGetName(), data, ctx)
		if len(errs) > 0 {
			b.Fatalf("Render returned errors: %v", errs)
		}
		if out == "" {
			b.Fatal("Render returned empty output")
		}
	}
}

func BenchmarkRenderManagerTemplate16Children(b *testing.B) {
	rm := benchmarkRenderManager()
	data := benchmarkTemplateChildData(16)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, errs := rm.Render(composite.TemplateConfigGetName(), data, ctx)
		if len(errs) > 0 {
			b.Fatalf("Render returned errors: %v", errs)
		}
		if out == "" {
			b.Fatal("Render returned empty output")
		}
	}
}
