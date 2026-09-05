package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestGojaYAMLResourceLoadingReloadAndDevelopment(t *testing.T) {
	setupLiveModeServeContentTest(t)
	hbConfig := shared.GetHyperBricksConfiguration()
	oldConfig := *hbConfig
	oldRoot, oldDirectories, oldParserConfig := commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig
	oldSections, oldSourceErrors := hypermediasBySection, routeSourceErrors
	oldDiagnostics, oldOrder, oldSequence := renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq
	t.Cleanup(func() {
		*hbConfig = oldConfig
		commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig = oldRoot, oldDirectories, oldParserConfig
		hypermediasBySection, routeSourceErrors = oldSections, oldSourceErrors
		renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq = oldDiagnostics, oldOrder, oldSequence
		parser.ClearTemplateStore()
	})
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	root := t.TempDir()
	hbConfig.Directories = make(map[string]string)
	for _, name := range []string{"hyperbricks", "resources", "templates", "static", "render", "plugins"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		hbConfig.Directories[name] = path
	}
	if err := os.MkdirAll(filepath.Join(root, "resources", "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	fixtureRoot := filepath.Join("..", "..", "modules", "goja-render-demo")
	for _, name := range []string{
		"hyperbricks/page.hyperbricks.yaml", "resources/scripts/availability.js",
		"hyperbricks/materials.hyperbricks.yaml", "resources/scripts/materials.js",
	} {
		content, err := os.ReadFile(filepath.Join(fixtureRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	commands.ModuleRoot = root
	hbConfig.Server.Beautify = false
	hbConfig.Development.FrontendErrors = false
	parser.HbConfig = map[string]interface{}{}
	parser.ClearTemplateStore()
	load := func() {
		t.Helper()
		if err := PreProcessAndPopulateConfigs(); err != nil {
			t.Fatal(err)
		}
	}
	check := func(quantity, want string) {
		t.Helper()
		writer := httptest.NewRecorder()
		ServeContent(writer, httptest.NewRequest("GET", "/?quantity="+quantity+"&secret=hidden", nil))
		if writer.Code != 200 || !strings.Contains(writer.Body.String(), `<p role="status">`+want+`</p>`) || writer.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("quantity=%s status=%d headers=%v body=%s", quantity, writer.Code, writer.Header(), writer.Body.String())
		}
		if count := writer.Header().Get(renderErrorCountHeader); count != "" && count != "0" {
			t.Fatalf("render errors: %s", count)
		}
	}
	load()
	checkGojaMaterialsPage(t, true)
	_, oldPlan, found := getConfigAndPlan("index")
	if !found || oldPlan == nil {
		t.Fatal("resource-backed YAML page must compile")
	}
	check("2", "Op voorraad")
	check("20", "Onvoldoende voorraad")
	check("-1", "Kies een geheel aantal tussen 1 en 10000.")
	path := filepath.Join(root, "resources", "scripts", "availability.js")
	if err := os.WriteFile(path, []byte(`function main(input) { return {message: "Bijgewerkt"}; }`), 0644); err != nil {
		t.Fatal(err)
	}
	load()
	check("2", "Bijgewerkt")
	req := httptest.NewRequest("GET", "/?quantity=2", nil)
	oldOutput, errs := oldPlan.Render(context.WithValue(req.Context(), shared.Request, req))
	if len(errs) != 0 || !strings.Contains(oldOutput, "Op voorraad") || strings.Contains(oldOutput, "Bijgewerkt") {
		t.Fatalf("old plan lost its resource snapshot: %q %v", oldOutput, errs)
	}
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	load()
	checkGojaMaterialsPage(t, false)
	if _, plan, _ := getConfigAndPlan("index"); plan != nil {
		t.Fatal("development route unexpectedly compiled")
	}
	check("2", "Bijgewerkt")
	if err := os.WriteFile(path, []byte("function main("), 0644); err != nil {
		t.Fatal(err)
	}
	load()
	if len(getRouteSourceErrors("index")) == 0 {
		t.Fatal("invalid script missing from load diagnostics")
	}
	writer := httptest.NewRecorder()
	ServeContent(writer, httptest.NewRequest("GET", "/?quantity=2", nil))
	if strings.Contains(writer.Body.String(), "Bijgewerkt") || writer.Header().Get(renderErrorCountHeader) == "0" || writer.Header().Get(renderErrorCountHeader) == "" {
		t.Fatalf("invalid replacement silently kept old output: %s headers=%v", writer.Body.String(), writer.Header())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	load()
	if len(getRouteSourceErrors("index")) == 0 {
		t.Fatal("missing resource did not produce a component load error")
	}
}

func checkGojaMaterialsPage(t *testing.T, compiled bool) {
	t.Helper()
	if _, plan, found := getConfigAndPlan("materialenlijst"); !found || (plan != nil) != compiled {
		t.Fatalf("materials route found=%v compiled=%v, want compiled=%v", found, plan != nil, compiled)
	}
	invalid := []string{`<p role="alert">Geef breedte en lengte als gehele centimeters van 1 tot 2000, elk eenmaal.</p>`}
	for _, tc := range []struct {
		name  string
		query string
		want  []string
	}{
		{"documented", "width_cm=400&length_cm=500", []string{
			"<dd>400 x 500 cm</dd>", "<dd>20.00 m2</dd>", "<dd>22.00 m2</dd>", "<dd>18</dd>", "<dd>22.50 m2</dd>",
			`value="400"`, `value="500"`,
		}},
		{"different input", "width_cm=100&length_cm=100", []string{
			"<dd>1.00 m2</dd>", "<dd>1.10 m2</dd>", "<dd>1</dd>", "<dd>1.25 m2</dd>",
			`value="100"`,
		}},
		{"exact box boundary", "width_cm=500&length_cm=500", []string{"<dd>22</dd>", "<dd>27.50 m2</dd>"}},
		{"unlisted rule override", "width_cm=400&length_cm=500&area_per_box_cm2=1&waste_percent=0", []string{"<dd>18</dd>", "<dd>22.00 m2</dd>"}},
		{"initial form", "", []string{`value="400"`, `value="500"`}},
		{"missing length", "width_cm=400", invalid},
		{"empty", "width_cm=&length_cm=", invalid},
		{"negative", "width_cm=-1&length_cm=500", invalid},
		{"zero", "width_cm=0&length_cm=500", invalid},
		{"fraction", "width_cm=1.5&length_cm=500", invalid},
		{"too large", "width_cm=2001&length_cm=500", invalid},
		{"not numeric", "width_cm=abc&length_cm=500", invalid},
		{"repeated", "width_cm=400&width_cm=500&length_cm=500", invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			ServeContent(writer, httptest.NewRequest("GET", "/materialenlijst?"+tc.query, nil))
			body := writer.Body.String()
			if writer.Code != 200 || writer.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v body=%s", writer.Code, writer.Header(), body)
			}
			if count := writer.Header().Get(renderErrorCountHeader); count != "" && count != "0" {
				t.Fatalf("render errors: %s; body=%s", count, body)
			}
			if !strings.Contains(body, "<!DOCTYPE html>") || strings.Contains(strings.ToLower(body), "<script") {
				t.Fatalf("expected complete HTML without browser scripts: %s", body)
			}
			if tc.query == "" && (strings.Contains(body, `<p role="alert">`) || strings.Contains(body, "<dl>")) {
				t.Fatalf("initial page should show an input form, not an error or calculation: %s", body)
			}
			for _, control := range []string{`<form method="get">`, `name="width_cm"`, `name="length_cm"`, `<button type="submit">Bereken</button>`} {
				if !strings.Contains(body, control) {
					t.Errorf("missing form control %q in %s", control, body)
				}
			}
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q in %s", want, body)
				}
			}
		})
	}
}
