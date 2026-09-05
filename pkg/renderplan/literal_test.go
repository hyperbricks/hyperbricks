package renderplan_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestLiteralTemplateLegacyParity(t *testing.T) {
	manager := newTestRenderManager()
	for _, tc := range []struct {
		name, source, enclose, want string
		errors                      int
	}{
		{"constant", " <b>raw & text</b>\n", "", " <b>raw & text</b>\n", 0},
		{"empty", "", "", "", 0},
		{"template comment", `{{/* removed */}}`, "", "", 0},
		{"trimmed comment", "left \n{{- /* removed */ -}}\n right", "", "leftright", 0},
		{"HTML comment", `before<!-- removed -->after`, "", "beforeafter", 0},
		{"HTML comment only", `<!-- removed -->`, "", "", 0},
		{"doctype", `<!DOCTYPE html><p>A &amp; B</p>`, "", `<!DOCTYPE html><p>A &amp; B</p>`, 0},
		{"script", `<script>const x = "<&>";</script>`, "", `<script>const x = "<&>";</script>`, 0},
		{"style", `<style>.x{color:red}</style>`, "", `<style>.x{color:red}</style>`, 0},
		{"open attribute", `<a title="unfinished`, "", "", 1},
		{"open script", `<script>const x = "`, "", "", 1},
		{"open style", `<style>.x{color:`, "", "", 1},
		{"empty enclosed", "", "section", "<section></section>", 0},
		{"pair enclosed", "constant", " <main> | </main> ", "<main>constant</main>", 0},
		{"failed enclosed", `<a title="unfinished`, "section", "<section></section>", 1},
		{"named root", `{{define "hyperbricks-generic-template"}}defined{{end}}`, "", "defined", 0},
		{"other root", `{{define "literal-check"}}unused{{end}}root`, "", "root", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A provider permits genuinely empty source; empty inline means missing.
			provider := func(name string) (string, bool) { return tc.source, name == "literal-fixture" }
			manager.GetRenderComponent(composite.TemplateConfigGetName()).(*composite.TemplateRenderer).TemplateProvider = provider
			raw := headlessTemplateRoute(nil)
			raw["template"] = map[string]interface{}{"template": "literal-fixture", "enclose": tc.enclose}
			plan, err := renderplan.Compile(manager, raw, provider)
			if err != nil {
				t.Fatalf("compile must preserve request-time execution errors: %v", err)
			}
			for _, id := range []string{"first", "second"} {
				legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext(id))
				got, gotErrors := plan.Render(requestContext(id))
				if got != legacy || got != tc.want {
					t.Errorf("request %s: compiled=%q, legacy=%q, want=%q", id, got, legacy, tc.want)
				}
				if len(gotErrors) != tc.errors || len(legacyErrors) != tc.errors {
					t.Fatalf("compiled errors=%v, legacy errors=%v, want %d", gotErrors, legacyErrors, tc.errors)
				}
				for i := range gotErrors {
					if reflect.TypeOf(gotErrors[i]) != reflect.TypeOf(legacyErrors[i]) || gotErrors[i].Error() != legacyErrors[i].Error() {
						t.Errorf("compiled error=%v, legacy error=%v", gotErrors[i], legacyErrors[i])
					}
				}
			}
		})
	}
}

func TestLiteralFoldPreservesDynamicTemplates(t *testing.T) {
	manager := newTestRenderManager()
	for _, tc := range []struct {
		name, source, prefix string
		values               map[string]interface{}
	}{
		{"helper", `{{env "HYPERBRICKS_LITERAL_REQUEST"}}`, "", nil},
		{"values", `{{.content}}/{{.Params.rid}}`, "&lt;b&gt;&amp;&lt;/b&gt;/", map[string]interface{}{"content": "<b>&</b>"}},
		{"named root", `{{define "hyperbricks-generic-template"}}{{.Params.rid}}{{end}}`, "", nil},
		{"named call", `{{define "literal-check"}}{{.Params.rid}}{{end}}{{template "literal-check" .}}`, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HYPERBRICKS_LITERAL_REQUEST", "compile-time")
			raw := headlessTemplateRoute(tc.values)
			template := raw["template"].(map[string]interface{})
			template["inline"], template["querykeys"] = tc.source, []string{"rid"}
			plan, err := renderplan.Compile(manager, raw, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"first", "second", "first"} {
				t.Setenv("HYPERBRICKS_LITERAL_REQUEST", id)
				legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext(id))
				got, gotErrors := plan.Render(requestContext(id))
				if got != legacy || got != tc.prefix+id || len(gotErrors)+len(legacyErrors) != 0 {
					t.Fatalf("request %s: compiled=%q (%v), legacy=%q (%v)", id, got, gotErrors, legacy, legacyErrors)
				}
			}
		})
	}
}

func TestLiteralTemplateStillRendersIgnoredChild(t *testing.T) {
	manager := newTestRenderManager()
	var calls []string
	manager.RegisterComponent(component.TextConfigGetName(), forwardChildRenderer(func(_ interface{}, ctx context.Context) (string, []error) {
		id := ctx.Value(shared.Request).(*http.Request).URL.Query().Get("rid")
		calls = append(calls, id)
		return id, []error{fmt.Errorf("ignored child failed: %s", id)}
	}), reflect.TypeOf(component.TextConfig{}))
	raw := headlessTemplateRoute(map[string]interface{}{
		"unused": map[string]interface{}{"@type": component.TextConfigGetName(), "value": "unused"},
	})
	raw["template"].(map[string]interface{})["inline"] = "constant"
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("child executed during compilation: %v", calls)
	}
	for _, id := range []string{"first", "second"} {
		calls = nil
		legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext(id))
		got, gotErrors := plan.Render(requestContext(id))
		if got != legacy || got != "constant" || !reflect.DeepEqual(calls, []string{id, id}) {
			t.Fatalf("request %s: compiled=%q, legacy=%q, child calls=%v", id, got, legacy, calls)
		}
		if len(gotErrors) != 1 || len(legacyErrors) != 1 || gotErrors[0].Error() != legacyErrors[0].Error() || gotErrors[0].Error() != "ignored child failed: "+id {
			t.Fatalf("request %s: compiled errors=%v, legacy errors=%v", id, gotErrors, legacyErrors)
		}
	}
}

func TestLiteralFoldSSRProofRequestIDs(t *testing.T) {
	manager := newTestRenderManager()
	raw := loadSSRProofRoute(t)
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"literal-first", "literal-second", "literal-first"} {
		legacy, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext(id))
		got, gotErrors := plan.Render(requestContext(id))
		if got != legacy || strings.Count(got, id) != 3 || len(gotErrors)+len(legacyErrors) != 0 {
			t.Fatalf("request %s: byte parity=%t, request ID count=%d, compiled errors=%v, legacy errors=%v", id, got == legacy, strings.Count(got, id), gotErrors, legacyErrors)
		}
	}
}
