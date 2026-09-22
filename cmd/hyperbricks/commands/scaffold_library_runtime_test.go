package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestScaffoldLibraryStartersRenderInInitializedModule(t *testing.T) {
	t.Chdir(t.TempDir())
	const moduleName = "scaffold-library-runtime"
	if err := initializeModule(moduleName); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("modules", moduleName))
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"title":"Runtime fixture item"}]}`))
	}))
	t.Cleanup(api.Close)

	starters, err := scaffoldLibraryTypes("")
	if err != nil {
		t.Fatal(err)
	}
	for _, starter := range starters {
		t.Run(starter, func(t *testing.T) {
			templates, err := scaffoldLibraryTemplates()
			if err != nil {
				t.Fatal(err)
			}
			spec := scaffoldSpec{
				Module:  root,
				Starter: starter,
				Type:    templates[starter].Type,
				Name:    "runtime_" + starter,
				File:    "runtime-" + starter + ".hyperbricks.yaml",
				Title:   "Runtime " + starter,
			}
			if scaffoldLibraryStarterOwnsRoute(starter) {
				spec.Route = "runtime-" + strings.ReplaceAll(starter, "_", "-")
			}
			plan, err := prepareScaffoldLibrary(spec)
			if err != nil {
				t.Fatal(err)
			}
			if starter == "api_render" || starter == "api_fragment_render" {
				scaffoldUseRuntimeAPI(t, plan, api.URL)
			}
			if err := plan.apply(); err != nil {
				t.Fatal(err)
			}

			module, err := loadAuthoringModule(root, "")
			if err != nil {
				t.Fatal(err)
			}
			result, err := yamlparser.ProcessFile(filepath.Join(module.Directories["hyperbricks"], spec.File), module.options())
			if err != nil {
				t.Fatal(err)
			}
			value, ok := result.Materialized[spec.Name].(map[string]interface{})
			if !ok {
				t.Fatalf("missing materialized %s root: %#v", spec.Name, result.Materialized[spec.Name])
			}
			typeName, _ := value["@type"].(string)
			manager := scaffoldLibraryRuntimeManager(t, module)
			scaffoldPrepareRuntimeStarter(t, manager, value, typeName)
			request := httptest.NewRequest(http.MethodGet, "http://runtime.local/", nil)
			ctx := context.WithValue(request.Context(), shared.Request, request)
			output, diagnostics := manager.Render(typeName, value, ctx)
			if len(diagnostics) != 0 {
				t.Fatalf("runtime diagnostics: %v\n%s", diagnostics, output)
			}
			if strings.TrimSpace(output) == "" {
				t.Fatal("starter rendered no output")
			}
		})
	}
}

func scaffoldLibraryStarterOwnsRoute(starter string) bool {
	templates, err := scaffoldLibraryTemplates()
	if err != nil {
		return false
	}
	return scaffoldRouteOwner(templates[starter].Type)
}

func scaffoldPrepareRuntimeStarter(t *testing.T, manager *render.RenderManager, value map[string]interface{}, typeName string) {
	t.Helper()
	response, err := manager.MakeInstance(renderTypeRequest(typeName, value))
	if err != nil {
		t.Fatalf("create runtime instance: %v", err)
	}
	switch typeName {
	case component.GojaRenderConfigGetName():
		value[component.GojaPreparedKey] = component.PrepareGojaRender(response.Instance.(component.GojaRenderConfig), nil)
	case component.EsbuildConfigGetName():
		esbuild := manager.GetRenderComponent(component.EsbuildConfigGetName()).(*component.EsbuildRenderer)
		value[component.EsbuildPreparedKey] = esbuild.Prepare(response.Instance.(component.EsbuildConfig))
	}
}

func scaffoldUseRuntimeAPI(t *testing.T, plan *scaffoldPlan, endpoint string) {
	t.Helper()
	for i := range plan.Files {
		if strings.HasSuffix(plan.Files[i].Path, ".hyperbricks.yaml") {
			plan.Files[i].After = strings.ReplaceAll(plan.Files[i].After, "https://api.example.com/items", endpoint)
			return
		}
	}
	t.Fatal("API starter plan has no YAML output")
}

func scaffoldLibraryRuntimeManager(t *testing.T, module *authoringModule) *render.RenderManager {
	t.Helper()
	shared.Init_configuration()
	config := shared.GetHyperBricksConfiguration()
	config.Directories["resources"] = module.Directories["resources"]
	config.Directories["static"] = module.Directories["static"]
	config.Directories["render"] = filepath.Join(module.Root, "rendered")

	rm := render.NewRenderManager()
	templateProvider := func(name string) (string, bool) {
		body, err := os.ReadFile(filepath.Join(module.Directories["templates"], name))
		return string(body), err == nil
	}
	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(component.MarkdownConfigGetName(), component.NewMarkdownRenderer(module.Directories["resources"]), reflect.TypeOf(component.MarkdownConfig{}))
	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))
	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JSConfigGetName(), &component.JSRenderer{}, reflect.TypeOf(component.JSConfig{}))
	rm.RegisterComponent(component.SingleImageConfigGetName(), &component.SingleImageRenderer{ImageProcessorInstance: &component.ImageProcessor{}}, reflect.TypeOf(component.SingleImageConfig{}))
	rm.RegisterComponent(component.MultipleImagesConfigGetName(), &component.MultipleImagesRenderer{ImageProcessorInstance: &component.ImageProcessor{}}, reflect.TypeOf(component.MultipleImagesConfig{}))
	rm.RegisterComponent(component.LocalJSONConfigGetName(), &component.LocalJSONRenderer{TemplateProvider: templateProvider}, reflect.TypeOf(component.LocalJSONConfig{}))
	rm.RegisterComponent(component.GojaRenderConfigGetName(), &component.GojaRenderer{}, reflect.TypeOf(component.GojaRenderConfig{}))
	rm.RegisterComponent(component.APIConfigGetName(), &component.APIRenderer{ComponentRenderer: renderer.ComponentRenderer{TemplateProvider: templateProvider}}, reflect.TypeOf(component.APIConfig{}))
	rm.RegisterComponent(component.MenuConfigGetName(), &component.MenuRenderer{HyperMediasBySection: map[string][]composite.HyperMediaConfig{
		"pages": {{Title: "Runtime page", Route: "runtime-page", Section: "pages"}},
	}}, reflect.TypeOf(component.MenuConfig{}))
	rm.RegisterComponent(component.EsbuildConfigGetName(), component.NewEsbuildRenderer(module.Directories["static"]), reflect.TypeOf(component.EsbuildConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), &composite.TreeRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm}}, reflect.TypeOf(composite.TreeConfig{}))
	rm.RegisterComponent(composite.TemplateConfigGetName(), &composite.TemplateRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm, TemplateProvider: templateProvider}}, reflect.TypeOf(composite.TemplateConfig{}))
	rm.RegisterComponent(composite.FragmentConfigGetName(), &composite.FragmentRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm, TemplateProvider: templateProvider}}, reflect.TypeOf(composite.FragmentConfig{}))
	rm.RegisterComponent(composite.ApiFragmentRenderConfigGetName(), &composite.ApiFragmentRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm, TemplateProvider: templateProvider}}, reflect.TypeOf(composite.ApiFragmentRenderConfig{}))
	rm.RegisterComponent(composite.HeadConfigGetName(), &composite.HeadRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm}}, reflect.TypeOf(composite.HeadConfig{}))
	rm.RegisterComponent(composite.HyperMediaConfigGetName(), &composite.HyperMediaRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm}}, reflect.TypeOf(composite.HyperMediaConfig{}))
	return rm
}

func renderTypeRequest(typeName string, value map[string]interface{}) typefactory.TypeRequest {
	return typefactory.TypeRequest{TypeName: typeName, Data: value}
}
