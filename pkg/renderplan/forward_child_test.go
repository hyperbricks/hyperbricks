package renderplan_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestForwardChildLegacyHTMLParity(t *testing.T) {
	manager := newTestRenderManager()
	const trusted = `<b>raw & "quoted"</b><!--kept-->`
	child := map[string]interface{}{"@type": component.TextConfigGetName(), "value": trusted}
	for _, test := range []struct {
		name, source, enclose string
		value                 interface{}
		want                  string
	}{
		{"trusted component", `{{.content}}`, "", child, trusted},
		{"escaped scalar", `{{.content}}`, "", trusted, `&lt;b&gt;raw &amp; &#34;quoted&#34;&lt;/b&gt;&lt;!--kept--&gt;`},
		{"trim markers", " \n{{- .content -}}\n ", "", child, trusted},
		{"literal whitespace", " {{.content}}\n", "", child, " " + trusted + "\n"},
		{"pair wrapper", `{{.content}}`, " <section> | </section> ", child, "<section>" + trusted + "</section>"},
		{"tag wrapper", `{{.content}}`, " article ", child, "<article>" + trusted + "</article>"},
		{"attribute context", `<a title="{{.content}}">x</a>`, "", child, `<a title="raw & &#34;quoted&#34;">x</a>`},
		{"script context", `<script>let x = {{.content}};</script>`, "", child, `<script>let x = "\u003cb\u003eraw \u0026 \"quoted\"\u003c/b\u003e\u003c!--kept--\u003e";</script>`},
		{"repeated child", `{{.content}}{{.content}}`, "", child, trusted + trusted},
		{"named root", `{{define "hyperbricks-generic-template"}}{{.content}}{{end}}`, "", child, trusted},
		{"other named root", `{{define "forward-check"}}{{.content}}{{end}}`, "", child, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := headlessTemplateRoute(map[string]interface{}{"content": test.value})
			template := raw["template"].(map[string]interface{})
			template["inline"], template["enclose"] = test.source, test.enclose
			assertPlanParity(t, manager, raw, requestContext("unused"), test.want)
		})
	}
}

func TestForwardChildRendersCustomChildOncePerRequest(t *testing.T) {
	manager := newTestRenderManager()
	childError := errors.New("child failed")
	calls := 0
	manager.RegisterComponent(component.TextConfigGetName(), forwardChildRenderer(func(_ interface{}, ctx context.Context) (string, []error) {
		calls++
		request := ctx.Value(shared.Request).(*http.Request)
		return "<b>" + request.URL.Query().Get("rid") + "</b><!--kept-->", []error{childError}
	}), reflect.TypeOf(component.TextConfig{}))
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{"@type": component.TextConfigGetName(), "value": "unused"},
	})
	raw["enclose"] = "main"
	raw["template"].(map[string]interface{})["enclose"] = " <section> | </section> "
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("child rendered %d times during compilation", calls)
	}
	var retained []string
	var expected []string
	for _, id := range []string{"first & raw", strings.Repeat("long", 1024), "last"} {
		ctx := requestContext(id)
		before := calls
		legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, ctx)
		if calls != before+1 || len(legacyErrors) != 1 || legacyErrors[0] != childError {
			t.Fatalf("legacy child calls = %d, errors = %v", calls-before, legacyErrors)
		}
		before = calls
		got, renderErrors := plan.Render(ctx)
		want := "<main><section><b>" + id + "</b><!--kept--></section></main>"
		if calls != before+1 || got != legacy || got != want {
			t.Fatalf("compiled child calls = %d, output differs = %t", calls-before, got != want)
		}
		if len(renderErrors) != 1 || renderErrors[0] != childError {
			t.Fatalf("errors = %v, want original child error", renderErrors)
		}
		retained = append(retained, got)
		expected = append(expected, want)
	}
	if !reflect.DeepEqual(retained, expected) {
		t.Fatal("later requests changed retained output strings")
	}
}

func TestForwardChildMultipleValuesStillRenderUnusedChildren(t *testing.T) {
	manager := newTestRenderManager()
	unusedError := errors.New("unused child failed")
	var calls []string
	manager.RegisterComponent(component.TextConfigGetName(), forwardChildRenderer(func(instance interface{}, _ context.Context) (string, []error) {
		value := instance.(component.TextConfig).Value
		calls = append(calls, value)
		if value == "unused" {
			return "discarded", []error{unusedError}
		}
		return "<b>visible</b>", nil
	}), reflect.TypeOf(component.TextConfig{}))
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{"@type": component.TextConfigGetName(), "value": "visible"},
		"unused":  map[string]interface{}{"@type": component.TextConfigGetName(), "value": "unused"},
	})
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("children rendered during compilation: %v", calls)
	}
	for _, compiled := range []bool{false, true, true} {
		calls = nil
		var output string
		var renderErrors []error
		if compiled {
			output, renderErrors = plan.Render(requestContext("unused"))
		} else {
			output, renderErrors = manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext("unused"))
		}
		if output != "<b>visible</b>" || !reflect.DeepEqual(calls, []string{"visible", "unused"}) {
			t.Fatalf("compiled=%t: output=%q, child calls=%v", compiled, output, calls)
		}
		if len(renderErrors) != 1 || renderErrors[0] != unusedError {
			t.Fatalf("compiled=%t: errors=%v, want unused child error", compiled, renderErrors)
		}
	}
}

func TestForwardChildConfiguredParamsRequestCollision(t *testing.T) {
	manager := newTestRenderManager()
	for _, test := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{"scalar", "<b>configured</b>", "&lt;b&gt;configured&lt;/b&gt;"},
		{"component", map[string]interface{}{"@type": component.TextConfigGetName(), "value": "<b>configured</b>"}, "<b>configured</b>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := headlessTemplateRoute(map[string]interface{}{"Params": test.value})
			template := raw["template"].(map[string]interface{})
			template["inline"] = `{{.Params}}`
			template["querykeys"] = []string{"rid"}
			assertPlanParity(t, manager, raw, context.Background(), test.want)
			assertPlanParity(t, manager, raw, requestContext("request"), "map[rid:request]")
			assertPlanParity(t, manager, raw, context.Background(), test.want)
		})
	}
}

func TestForwardChildPreservesChildValidationErrors(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{"@type": component.TextConfigGetName(), "value": ""},
	})
	raw["template"].(map[string]interface{})["enclose"] = "section"
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext("unused"))
	got, renderErrors := plan.Render(requestContext("unused"))
	if got != "<section></section>" || got != legacy || len(legacyErrors) != 1 || len(renderErrors) != 1 {
		t.Fatalf("output=%q, legacy=%q, errors=%v, legacy errors=%v", got, legacy, renderErrors, legacyErrors)
	}
	assertComponentErrorParity(t, renderErrors[0], legacyErrors[0])
}

type forwardChildRenderer func(interface{}, context.Context) (string, []error)

func (f forwardChildRenderer) Types() []string { return []string{component.TextConfigGetName()} }

func (f forwardChildRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	return f(instance, ctx)
}
