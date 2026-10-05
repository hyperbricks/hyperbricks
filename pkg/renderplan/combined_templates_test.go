package renderplan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestSSRProofSelectsCombinedTemplates(t *testing.T) {
	manager := combinationRenderManager()
	result, err := yamlparser.ProcessFile("../../modules/ssr-proof-hyperbricks/hyperbricks/landing.hyperbricks.yaml", yamlparser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	raw := result.Materialized["ssr_proof_page"].(map[string]interface{})
	plan, err := Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	combined, ok := plan.root.(*hyperMediaNode).template.(*combinedTemplateNode)
	if !ok {
		t.Fatal("SSR proof must exercise combined template execution")
	}
	for _, query := range []string{"rid=original", "rid=%3Cscript%3E%26%22%27", "rid=a&rid=b", "rid=%E2%82%AC", ""} {
		ctx := combinationContext(query)
		got, gotErrors := plan.Render(ctx)
		want, wantErrors := combined.original.Render(newRenderState(ctx, plan.querySets))
		legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, ctx)
		if got != want || got != legacy || len(gotErrors)+len(wantErrors)+len(legacyErrors) != 0 {
			t.Fatalf("query=%q\ngot=%q\noriginal=%q\nlegacy=%q\nerrors=%v/%v/%v", query, got, want, legacy, gotErrors, wantErrors, legacyErrors)
		}
	}
}

func TestCombinedTemplatesPreserveScopesAndEscaping(t *testing.T) {
	child := combinationTemplate(`<b title="{{.Params.id}}">{{.Params.child}}{{.Params.id}}/{{.label}}</b>`, map[string]interface{}{"label": "child & <label>"})
	child["querykeys"] = []string{"id", "child"}
	sibling := combinationTemplate(`<p>{{.Params.child}}/{{.Params.sibling}}/{{.label}}</p>`, map[string]interface{}{"label": "sibling {{.Params.id}}"})
	sibling["querykeys"] = []string{"sibling"}
	tree := map[string]interface{}{
		"@type": composite.TreeRendererConfigGetName(), "@order": []string{"child", "sibling"},
		"child": child, "sibling": sibling,
	}
	root := combinationTemplate(`<main title="{{.Params.id}}">{{.content}}<i>{{.label}}</i></main><a href="{{.Params.id}}" data-id={{.Params.id}}>link</a><script>const id={{.Params.id}};</script><style>p{color:{{.Params.id}}}</style>`,
		map[string]interface{}{"content": tree, "label": "parent & <label>"})
	root["querykeys"] = []string{"id"}
	c, original := compileCombination(t, root)
	before := make(map[*templateNode]string)
	for node := range c.templateSources {
		before[node] = node.template.Tree.Root.String()
	}
	combined := combineTemplates(original, c.templateSources)
	if combined == nil {
		t.Fatal("pure direct fields with distinct query scopes must combine")
	}
	for node, tree := range before {
		if got := node.template.Tree.Root.String(); got != tree {
			t.Fatal("combination mutated the shared cached template")
		}
	}
	queries := []string{
		"id=red&child=child-only&sibling=sibling-only",
		"id=%3Cscript%3E%26%22%27&child=%3Cb%3E&sibling=%E2%82%AC",
		"id=javascript%3Aalert%281%29&child=x&sibling=y",
		"id=one&id=two&child=x&child=y&sibling=z",
		"", "unallowed=secret",
	}
	for _, query := range queries {
		assertCombinedParity(t, combined, original, newRenderState(combinationContext(query), c.querySets))
	}
	assertCombinedParity(t, combined, original, newRenderState(context.Background(), c.querySets))
	// A request with no URL must retain the original absent-Params behavior.
	ctx := context.WithValue(context.Background(), shared.Request, &http.Request{})
	assertCombinedParity(t, combined, original, newRenderState(ctx, c.querySets))

	const requests = 128
	var wait sync.WaitGroup
	failures := make(chan string, requests)
	for i := 0; i < requests; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id := fmt.Sprintf("unique-%d", i)
			state := newRenderState(combinationContext("id="+url.QueryEscape(id)), c.querySets)
			got, gotErrors := combined.Render(state)
			want, wantErrors := original.Render(state)
			if got != want || len(gotErrors)+len(wantErrors) != 0 || !strings.Contains(got, id) {
				failures <- id
			}
		}()
	}
	wait.Wait()
	close(failures)
	for id := range failures {
		t.Errorf("concurrent output differs for %s", id)
	}
}

func TestCombinedTemplatesPreserveLiteralBoundaries(t *testing.T) {
	root := combinationTemplate(`{{.first}}{{.second}}{{.third}}`, map[string]interface{}{
		"first":  combinationTemplate(`<`, nil),
		"second": combinationTemplate(`b><!--remove--><script>const x="<x>";</script>`, nil),
		"third":  combinationTemplate(`<p>{{.Params.id}}</p>`, nil),
	})
	c, original := compileCombination(t, root)
	combined := combineTemplates(original, c.templateSources)
	if combined == nil {
		t.Fatal("standalone escaped literals must be eligible")
	}
	assertCombinedParity(t, combined, original, newRenderState(combinationContext("id=%3Cb%3E"), c.querySets))
	output, _ := combined.Render(newRenderState(context.Background(), c.querySets))
	if !strings.HasPrefix(output, "&lt;b>") || strings.Contains(output, "remove") {
		t.Fatalf("standalone literal boundary or comment removal changed: %q", output)
	}
}

func TestCombinedTemplatesRejectObservableOrContextualChanges(t *testing.T) {
	for _, test := range []struct {
		name, source string
		change       func(map[string]interface{})
	}{
		{name: "attribute child", source: `<a title="{{.child}}">x</a>`},
		{name: "unquoted attribute child", source: `<a title={{.child}}>x</a>`},
		{name: "URL child", source: `<a href="{{.child}}">x</a>`},
		{name: "JS child", source: `<script>const x={{.child}};</script>`},
		{name: "CSS child", source: `<style>p{color:{{.child}}}</style>`},
		{name: "RCDATA child", source: `<textarea>{{.child}}</textarea>`},
		{name: "comment child", source: `<!--{{.child}}-->`},
		{name: "repeated child", source: `{{.child}}{{.child}}`},
		{name: "unused child", source: `no interpolation`},
		{name: "function", source: `{{printf "%s" .child}}`},
		{name: "custom function", source: `{{.child | safe}}`},
		{name: "branch", source: `{{if .child}}{{.child}}{{end}}`},
		{name: "declaration", source: `{{$x := .child}}{{$x}}`},
		{name: "associated template", source: `{{template "extra" .}}{{define "extra"}}{{.child}}{{end}}`},
		{name: "missing selector", source: `{{.missing}}{{.child}}`},
		{name: "invalid child context", source: `{{.child}}`, change: func(raw map[string]interface{}) {
			raw["values"].(map[string]interface{})["child"].(map[string]interface{})["inline"] = `<script>`
		}},
		{name: "wrapper", source: `{{.child}}`, change: func(raw map[string]interface{}) { raw["enclose"] = "main" }},
		{name: "configured Params", source: `{{.child}}`, change: func(raw map[string]interface{}) {
			raw["values"].(map[string]interface{})["Params"] = "configured"
		}},
		{name: "static map", source: `{{.child}}`, change: func(raw map[string]interface{}) {
			raw["values"].(map[string]interface{})["data"] = map[string]interface{}{"key": "value"}
		}},
		{name: "XML", source: `{{.child}}`, change: func(raw map[string]interface{}) { raw["@output_content_type"] = "application/xml" }},
		{name: "custom renderer", source: `{{.child}}`, change: func(raw map[string]interface{}) {
			raw["values"].(map[string]interface{})["child"] = map[string]interface{}{"@type": component.TextConfigGetName(), "value": "custom"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := combinationTemplate(test.source, map[string]interface{}{"child": combinationTemplate(`<b>{{.Params.id}}</b>`, nil)})
			if test.change != nil {
				test.change(raw)
			}
			c, original := compileCombination(t, raw)
			if combineTemplates(original, c.templateSources) != nil {
				t.Fatal("graph with observable/contextual differences must retain original execution")
			}
		})
	}
}

func TestCombinedTemplatesKeepWarningsAndUnusedChildCalls(t *testing.T) {
	raw := combinationTemplate(`{{.child}}`, map[string]interface{}{"child": combinationTemplate(`<b>child</b>`, nil)})
	c, original := compileCombination(t, raw)
	child := original.(*templateNode).values[0].child.(*templateNode)
	child.warnings = []string{"source warning"}
	if combineTemplates(original, c.templateSources) != nil {
		t.Fatal("source warnings must retain their original execution and diagnostics")
	}
	state := newRenderState(context.Background(), nil)
	_, first := original.Render(state)
	_, second := original.Render(state)
	if len(first) != 1 || len(second) != 1 || first[0].(shared.ComponentError).Hash == second[0].(shared.ComponentError).Hash {
		t.Fatal("warnings must remain request-local")
	}
	child.warnings = nil
	calls := 0
	original.(*templateNode).values = append(original.(*templateNode).values, templateValue{
		key: "unused", child: forwardChildNodeFunc(func(*renderState) (string, []error) {
			calls++
			return "unused", nil
		}),
	})
	// The sole-child forwarding shortcut was prepared before this test changed
	// the graph; disable it so original eager evaluation is exercised.
	original.(*templateNode).forwardChild = nil
	if combineTemplates(original, c.templateSources) != nil || calls != 0 {
		t.Fatal("preparation must not execute or discard unused custom children")
	}
	original.Render(state)
	if calls != 1 {
		t.Fatalf("original eager child execution called %d times, want once", calls)
	}
}

func combinationRenderManager() *render.RenderManager {
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration().Mode = shared.LIVE_MODE
	manager := render.NewRenderManager()
	base := renderer.CompositeRenderer{RenderManager: manager}
	manager.RegisterComponent(composite.TemplateConfigGetName(), &composite.TemplateRenderer{CompositeRenderer: base}, reflect.TypeOf(composite.TemplateConfig{}))
	manager.RegisterComponent(composite.TreeRendererConfigGetName(), &composite.TreeRenderer{CompositeRenderer: base}, reflect.TypeOf(composite.TreeConfig{}))
	manager.RegisterComponent(composite.HyperMediaConfigGetName(), &composite.HyperMediaRenderer{CompositeRenderer: base}, reflect.TypeOf(composite.HyperMediaConfig{}))
	manager.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	return manager
}

func compileCombination(t *testing.T, raw map[string]interface{}) (*compiler, renderNode) {
	t.Helper()
	c := &compiler{manager: combinationRenderManager(), templateSources: make(map[*templateNode]string)}
	node, err := c.compileTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c, node
}

func combinationTemplate(source string, values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = make(map[string]interface{})
	}
	return map[string]interface{}{"@type": composite.TemplateConfigGetName(), "inline": source, "values": values}
}

func combinationContext(query string) context.Context {
	request := httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return context.WithValue(request.Context(), shared.Request, request)
}

func assertCombinedParity(t *testing.T, combined, original renderNode, state *renderState) {
	t.Helper()
	got, gotErrors := combined.Render(state)
	want, wantErrors := original.Render(state)
	if got != want || len(gotErrors)+len(wantErrors) != 0 {
		t.Fatalf("combined=%q\noriginal=%q\nerrors=%v/%v", got, want, gotErrors, wantErrors)
	}
}
