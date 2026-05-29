package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	hbschema "github.com/hyperbricks/hyperbricks/pkg/schema"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

var updateYAMLReadableFlag = flag.Bool("update-yaml-readable", false, "rewrite HyperBricks YAML readable expected JSON sections")

const (
	yamlProfileFixtureDir = "hyperbricks-yaml-test-files"
)

type yamlReadableCase struct {
	Path                     string
	Scope                    string
	Source                   string
	Explainer                string
	ExpectedPreprocessedYAML string
	ExpectedMaterializedJSON string
	ExpectedRuntimeJSON      string
	ExpectedJSON             string
	ExpectedDiagnostics      string
	ExpectedOutput           string
}

type expectedYAMLDiagnostic struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Key      string `json:"key"`
	Level    string `json:"level"`
	Contains string `json:"contains"`
}

var yamlCoreCorpus = []string{
	"text-html-tree.hyperbricks.yaml.test",
	"template-values-data.hyperbricks.yaml.test",
	"inheritance-override.hyperbricks.yaml.test",
	"ordered-children.hyperbricks.yaml.test",
	"nested-tree-3-level.hyperbricks.yaml.test",
	"head-assets.hyperbricks.yaml.test",
	"head-generated-items.hyperbricks.yaml.test",
	"hypermedia-route.hyperbricks.yaml.test",
	"fragment-response.hyperbricks.yaml.test",
	"menu-items.hyperbricks.yaml.test",
	"api-render-request.hyperbricks.yaml.test",
	"reserved-name-collision.hyperbricks.yaml.test",
	"duplicate-item-names.hyperbricks.yaml.test",
	"duplicate-item-names-deep.hyperbricks.yaml.test",
	"duplicate-item-name-collision.hyperbricks.yaml.test",
	"duplicate-inheritance-path.hyperbricks.yaml.test",
}

func TestYAMLProfileFixturesInstantiateRuntimeConfigs(t *testing.T) {
	factory := typefactory.NewTypeFactory()
	for _, def := range hbschema.Definitions() {
		factory.RegisterType(def.Token, def.ConfigType)
	}

	for _, name := range yamlCoreCorpus {
		t.Run(name, func(t *testing.T) {
			materialized := materializeYAMLFixture(t, name)
			for rootName, raw := range materialized {
				scope := asMap(t, raw, rootName)
				typeName, ok := scope["@type"].(string)
				if !ok || typeName == "" {
					t.Fatalf("%s @type = %#v, want runtime type string", rootName, scope["@type"])
				}
				if _, err := factory.CreateInstance(typefactory.TypeRequest{
					TypeName: typeName,
					Data:     scope,
				}); err != nil {
					t.Fatalf("instantiate %s as %s: %v", rootName, typeName, err)
				}
			}
		})
	}
}

func TestYAMLProfileReadableCases(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join(yamlProfileFixtureDir, "*.hyperbricks.yaml.test"))
	if err != nil {
		t.Fatalf("glob readable YAML cases: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no readable YAML cases found")
	}

	rm := newYAMLProfileRenderManager(t)
	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			testCase := parseYAMLReadableCase(t, path)
			result, err := yamlparser.ProcessBytes([]byte(testCase.Source), yamlProfileOptionsForCase(t, path))
			if err != nil {
				t.Fatalf("process YAML case: %v", err)
			}
			doc := result.Document
			materialized := result.Materialized
			configureYAMLProfileMenus(t, rm, doc, materialized)

			scope := asMap(t, materialized[testCase.Scope], testCase.Scope)
			assertCaseExpectedPreprocessedYAML(t, testCase, result.Preprocessed)
			assertCaseExpectedMaterializedJSON(t, testCase, scope)
			assertCaseExpectedRuntimeJSON(t, rm, testCase, scope)
			assertCaseExpectedJSON(t, rm, testCase, scope)
			assertCaseExpectedDiagnostics(t, testCase, result.Diagnostics)

			if strings.TrimSpace(testCase.ExpectedOutput) == "" {
				return
			}
			typeName, ok := scope["@type"].(string)
			if !ok || typeName == "" {
				t.Fatalf("%s @type = %#v, want runtime type string", testCase.Scope, scope["@type"])
			}
			output, renderErrors := rm.Render(typeName, scope, createMockContext())
			if len(renderErrors) > 0 {
				t.Fatalf("render %s returned errors: %v", testCase.Scope, renderErrors)
			}
			if stripAllWhitespace(output) != stripAllWhitespace(testCase.ExpectedOutput) {
				t.Fatalf("rendered output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(path), output, testCase.ExpectedOutput)
			}
		})
	}
}

func TestYAMLProfileFixtureDirectoryContainsOnlyReadableYAMLFixtures(t *testing.T) {
	entries, err := os.ReadDir(yamlProfileFixtureDir)
	if err != nil {
		t.Fatalf("read YAML fixture directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".hyperbricks.yaml.test") {
			t.Fatalf("unexpected file in YAML fixture directory: %s", entry.Name())
		}
	}
}

func TestYAMLProfileReadableCasesUseValueResolversInsteadOfLegacyMarkers(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join(yamlProfileFixtureDir, "*.hyperbricks.yaml.test"))
	if err != nil {
		t.Fatalf("glob readable YAML cases: %v", err)
	}
	legacyMarkerPattern := regexp.MustCompile(`\{\{(?:VAR|ENV|CONF|FILE|TEMPLATE|RESOURCES|TEMPLATES|STATIC|HYPERBRICKS|MODULE|ROOT|MODULE_ROOT|MODULE_PATH)(?::|\}\})`)
	for _, path := range matches {
		testCase := parseYAMLReadableCase(t, path)
		if legacyMarkerPattern.MatchString(testCase.Source) {
			t.Fatalf("%s uses legacy {{...}} marker syntax in YAML source", filepath.Base(path))
		}
	}
}

func TestYAMLProfileReadableCasesCoverCoreCorpus(t *testing.T) {
	for _, name := range yamlCoreCorpus {
		readablePath := filepath.Join(yamlProfileFixtureDir, name)
		if _, err := os.Stat(readablePath); err != nil {
			t.Fatalf("readable YAML test case %s is missing: %v", name, err)
		}
	}
}

func TestYAMLProfileFixturePreservesSourceOrder(t *testing.T) {
	materialized := materializeYAMLFixture(t, "ordered-children.hyperbricks.yaml.test")
	page := asMap(t, materialized["page"], "page")
	want := []string{"zeta", "alpha", "middle"}
	if got := page["@order"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page @order = %#v, want %#v", got, want)
	}
}

func TestYAMLProfileFixturePreservesThreeLevelNestedTree(t *testing.T) {
	materialized := materializeYAMLFixture(t, "nested-tree-3-level.hyperbricks.yaml.test")
	page := asMap(t, materialized["page"], "page")
	content := asMap(t, page["content"], "page.content")
	section := asMap(t, content["section_block"], "page.content.section_block")
	article := asMap(t, section["article_block"], "page.content.section_block.article_block")

	if content["@type"] != "<TREE>" {
		t.Fatalf("content @type = %v", content["@type"])
	}
	if section["@type"] != "<TREE>" {
		t.Fatalf("section_block @type = %v", section["@type"])
	}
	if article["@type"] != "<TREE>" {
		t.Fatalf("article_block @type = %v", article["@type"])
	}
	if got := content["@order"]; !reflect.DeepEqual(got, []string{"section_block"}) {
		t.Fatalf("content @order = %#v", got)
	}
	if got := section["@order"]; !reflect.DeepEqual(got, []string{"article_block"}) {
		t.Fatalf("section_block @order = %#v", got)
	}
	if got := article["@order"]; !reflect.DeepEqual(got, []string{"title", "body"}) {
		t.Fatalf("article_block @order = %#v", got)
	}
}

func TestYAMLProfileFixtureResolvesInheritanceOverride(t *testing.T) {
	materialized := materializeYAMLFixture(t, "inheritance-override.hyperbricks.yaml.test")
	page := asMap(t, materialized["page"], "page")
	card := asMap(t, page["featured_card"], "featured_card")
	values := asMap(t, card["values"], "featured_card.values")
	theme := asMap(t, values["theme"], "featured_card.values.theme")

	if card["@type"] != "<TEMPLATE>" {
		t.Fatalf("featured_card @type = %v", card["@type"])
	}
	if values["title"] != "Overridden title" {
		t.Fatalf("title = %v", values["title"])
	}
	if values["body"] != "Base body" {
		t.Fatalf("body = %v", values["body"])
	}
	if values["width"] != "320" {
		t.Fatalf("width = %#v", values["width"])
	}
	if theme["tone"] != "strong" || theme["density"] != "compact" {
		t.Fatalf("theme = %#v", theme)
	}
}

func TestYAMLProfileFixtureKeepsDataArrays(t *testing.T) {
	materialized := materializeYAMLFixture(t, "template-values-data.hyperbricks.yaml.test")
	card := asMap(t, materialized["card"], "card")
	values := asMap(t, card["values"], "card.values")

	cards, ok := values["cards"].([]interface{})
	if !ok {
		t.Fatalf("cards type = %T, want []interface{}", values["cards"])
	}
	if len(cards) != 2 {
		t.Fatalf("cards len = %d, want 2", len(cards))
	}
	first := asMap(t, cards[0], "cards[0]")
	if first["title"] != "First card" {
		t.Fatalf("first card title = %v", first["title"])
	}

	cta := asMap(t, values["cta"], "card.values.cta")
	if cta["@type"] != "<HTML>" {
		t.Fatalf("cta @type = %v", cta["@type"])
	}
}

func TestYAMLProfileFixtureAvoidsReservedNameChildCollision(t *testing.T) {
	materialized := materializeYAMLFixture(t, "reserved-name-collision.hyperbricks.yaml.test")
	page := asMap(t, materialized["page"], "page")
	head := asMap(t, page["head"], "page.head")

	css, ok := head["css"].([]interface{})
	if !ok {
		t.Fatalf("head.css type = %T, want []interface{}", head["css"])
	}
	if len(css) != 1 {
		t.Fatalf("head.css len = %d, want 1", len(css))
	}
	wantOrder := []string{"css_component", "js_component"}
	if got := head["@order"]; !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("head @order = %#v, want %#v", got, wantOrder)
	}
	if _, exists := head["css"].(map[string]interface{}); exists {
		t.Fatal("head.css became a child object; reserved property name collided")
	}
}

func materializeYAMLFixture(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(yamlProfileFixtureDir, name)
	testCase := parseYAMLReadableCase(t, path)
	result, err := yamlparser.ProcessBytes([]byte(testCase.Source), yamlProfileOptionsForCase(t, path))
	if err != nil {
		t.Fatalf("process YAML fixture %s: %v", name, err)
	}
	return result.Materialized
}

func parseYAMLReadableCase(t *testing.T, path string) yamlReadableCase {
	t.Helper()
	raw := string(readFixtureFile(t, path))
	sections, scopes := parseReadableCaseSections(t, raw)

	testCase := yamlReadableCase{
		Path:                     path,
		Scope:                    scopes["hyperbricks yaml"],
		Source:                   sections["hyperbricks yaml"],
		Explainer:                strings.TrimSpace(sections["explainer"]),
		ExpectedPreprocessedYAML: sections["expected preprocessed yaml"],
		ExpectedMaterializedJSON: sections["expected materialized json"],
		ExpectedRuntimeJSON:      sections["expected runtime json"],
		ExpectedJSON:             sections["expected json"],
		ExpectedDiagnostics:      sections["expected diagnostics"],
		ExpectedOutput:           sections["expected output"],
	}
	if strings.TrimSpace(testCase.Source) == "" {
		t.Fatalf("%s missing ==== hyperbricks yaml ==== section", path)
	}
	if strings.TrimSpace(testCase.Scope) == "" {
		t.Fatalf("%s missing scope reference, expected ==== hyperbricks yaml {!{scope}} ====", path)
	}
	if strings.TrimSpace(testCase.ExpectedJSON) == "" &&
		strings.TrimSpace(testCase.ExpectedMaterializedJSON) == "" &&
		strings.TrimSpace(testCase.ExpectedRuntimeJSON) == "" {
		t.Fatalf("%s missing expected JSON section", path)
	}
	return testCase
}

func parseReadableCaseSections(t *testing.T, content string) (map[string]string, map[string]string) {
	t.Helper()
	headerPattern := regexp.MustCompile(`^====\s*([^!]+?)(?:\s*\{\!\{([^}]+)\}\})?\s*====\s*$`)
	sections := make(map[string]string)
	scopes := make(map[string]string)
	var current string
	var builder strings.Builder

	flush := func() {
		if current == "" {
			return
		}
		sections[current] = builder.String()
		builder.Reset()
	}

	for _, line := range strings.Split(content, "\n") {
		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			flush()
			current = strings.ToLower(strings.TrimSpace(matches[1]))
			if len(matches) > 2 {
				scopes[current] = strings.TrimSpace(matches[2])
			}
			continue
		}
		if current == "" {
			continue
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	flush()
	return sections, scopes
}

func assertCaseExpectedJSON(t *testing.T, rm *render.RenderManager, testCase yamlReadableCase, scope map[string]interface{}) {
	t.Helper()
	if strings.TrimSpace(testCase.ExpectedJSON) == "" {
		return
	}
	var expected interface{}
	if err := json.Unmarshal([]byte(testCase.ExpectedJSON), &expected); err != nil {
		t.Fatalf("parse expected JSON in %s: %v", testCase.Path, err)
	}
	gotValue := interface{}(scope)
	if expectedMap, ok := expected.(map[string]interface{}); ok {
		if _, isRuntimeConfig := expectedMap["ConfigType"]; isRuntimeConfig {
			typeName, ok := scope["@type"].(string)
			if !ok || typeName == "" {
				t.Fatalf("%s @type = %#v, want runtime type string", testCase.Scope, scope["@type"])
			}
			response, err := rm.MakeInstance(typefactory.TypeRequest{
				TypeName: typeName,
				Data:     scope,
			})
			if err != nil {
				t.Fatalf("instantiate %s as %s: %v", testCase.Scope, typeName, err)
			}
			gotValue = response.Instance
		}
	}
	got := canonicalJSON(t, gotValue)
	want := canonicalJSON(t, expected)
	if *updateYAMLReadableFlag {
		rewriteYAMLReadableExpectedJSON(t, testCase.Path, got)
		return
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected JSON mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(testCase.Path), got, want)
	}
}

func assertCaseExpectedPreprocessedYAML(t *testing.T, testCase yamlReadableCase, preprocessed string) {
	t.Helper()
	if strings.TrimSpace(testCase.ExpectedPreprocessedYAML) == "" {
		return
	}
	got := normalizeFixtureText(preprocessed)
	want := normalizeFixtureText(testCase.ExpectedPreprocessedYAML)
	if got != want {
		t.Fatalf("expected preprocessed YAML mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(testCase.Path), got, want)
	}
}

func assertCaseExpectedMaterializedJSON(t *testing.T, testCase yamlReadableCase, scope map[string]interface{}) {
	t.Helper()
	if strings.TrimSpace(testCase.ExpectedMaterializedJSON) == "" {
		return
	}
	assertExpectedJSONValue(t, testCase.Path, "expected materialized json", testCase.ExpectedMaterializedJSON, scope)
}

func assertCaseExpectedRuntimeJSON(t *testing.T, rm *render.RenderManager, testCase yamlReadableCase, scope map[string]interface{}) {
	t.Helper()
	if strings.TrimSpace(testCase.ExpectedRuntimeJSON) == "" {
		return
	}
	projection := runtimeConfigProjection(t, rm, testCase, scope)
	assertExpectedJSONValue(t, testCase.Path, "expected runtime json", testCase.ExpectedRuntimeJSON, projection)
}

func assertExpectedJSONValue(t *testing.T, path string, section string, expectedJSON string, gotValue interface{}) {
	t.Helper()
	var expected interface{}
	if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil {
		t.Fatalf("parse %s in %s: %v", section, path, err)
	}
	got := canonicalJSON(t, gotValue)
	want := canonicalJSON(t, expected)
	if !bytes.Equal(got, want) {
		t.Fatalf("%s mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", section, filepath.Base(path), got, want)
	}
}

func assertCaseExpectedDiagnostics(t *testing.T, testCase yamlReadableCase, diagnostics []yamlparser.Diagnostic) {
	t.Helper()
	if strings.TrimSpace(testCase.ExpectedDiagnostics) == "" {
		return
	}
	var expected []expectedYAMLDiagnostic
	if err := json.Unmarshal([]byte(testCase.ExpectedDiagnostics), &expected); err != nil {
		t.Fatalf("parse expected diagnostics in %s: %v", testCase.Path, err)
	}
	if len(diagnostics) != len(expected) {
		t.Fatalf("diagnostics len for %s = %d, want %d: %#v", filepath.Base(testCase.Path), len(diagnostics), len(expected), diagnostics)
	}
	for index, want := range expected {
		got := diagnostics[index]
		if want.Type != "" && want.Type != "YAML" {
			t.Fatalf("expected diagnostics[%d].type = %q, only YAML diagnostics are supported in %s", index, want.Type, testCase.Path)
		}
		if want.Path != "" && got.Path != want.Path {
			t.Fatalf("diagnostics[%d].path for %s = %q, want %q", index, filepath.Base(testCase.Path), got.Path, want.Path)
		}
		if want.Key != "" && got.OriginalName != want.Key {
			t.Fatalf("diagnostics[%d].key for %s = %q, want %q", index, filepath.Base(testCase.Path), got.OriginalName, want.Key)
		}
		if want.Level != "" && !strings.EqualFold(got.Level, want.Level) {
			t.Fatalf("diagnostics[%d].level for %s = %q, want %q", index, filepath.Base(testCase.Path), got.Level, want.Level)
		}
		if want.Contains != "" && !strings.Contains(got.Message, want.Contains) {
			t.Fatalf("diagnostics[%d].message for %s = %q, want to contain %q", index, filepath.Base(testCase.Path), got.Message, want.Contains)
		}
	}
}

func runtimeConfigProjection(t *testing.T, rm *render.RenderManager, testCase yamlReadableCase, scope map[string]interface{}) interface{} {
	t.Helper()
	typeName, ok := scope["@type"].(string)
	if !ok || typeName == "" {
		t.Fatalf("%s @type = %#v, want runtime type string", testCase.Scope, scope["@type"])
	}
	response, err := rm.MakeInstance(typefactory.TypeRequest{
		TypeName: typeName,
		Data:     scope,
	})
	if err != nil {
		t.Fatalf("instantiate %s as %s: %v", testCase.Scope, typeName, err)
	}
	switch config := response.Instance.(type) {
	case composite.HyperMediaConfig:
		return map[string]interface{}{
			"ConfigType": config.Composite.Meta.ConfigType,
			"Route":      config.Route,
			"Title":      config.Title,
			"Items":      config.Composite.Items,
		}
	case composite.FragmentConfig:
		return map[string]interface{}{
			"ConfigType": config.Composite.Meta.ConfigType,
			"Route":      config.Route,
			"Response":   config.HxResponse,
			"Items":      config.Composite.Items,
		}
	case composite.TreeConfig:
		return map[string]interface{}{
			"ConfigType": config.Composite.Meta.ConfigType,
			"Items":      config.Composite.Items,
			"Enclose":    config.Enclose,
		}
	case composite.TemplateConfig:
		return map[string]interface{}{
			"ConfigType": config.Composite.Meta.ConfigType,
			"Template":   config.Template,
			"Inline":     config.Inline,
			"Values":     config.Values,
			"Items":      config.Composite.Items,
		}
	default:
		return response.Instance
	}
}

func yamlProfileOptionsForCase(t *testing.T, path string) yamlparser.Options {
	t.Helper()
	if filepath.Base(path) != "pipeline-pre-parse-post.hyperbricks.yaml.test" {
		return yamlparser.Options{
			Paths: yamlparser.PathMarkers{
				Resources: "resources",
			},
			RecoverDuplicateChildren: true,
		}
	}
	return yamlparser.Options{
		Variables: map[string]string{
			"route": "pipeline",
		},
		Env: map[string]string{
			"CTA_TEXT": "Start now",
		},
		Config: map[string]interface{}{
			"site": map[string]interface{}{
				"title": "Pipeline Page",
			},
		},
		Paths: yamlparser.PathMarkers{
			Resources: filepath.Join(yamlProfileFixtureDir, "pipeline-assets"),
		},
		RecoverDuplicateChildren: true,
	}
}

func newYAMLProfileRenderManager(t *testing.T) *render.RenderManager {
	t.Helper()
	shared.Init_configuration()
	conf := shared.GetHyperBricksConfiguration()
	testOutputRoot := t.TempDir()
	conf.Directories["static"] = filepath.Join(testOutputRoot, "static")
	conf.Directories["render"] = filepath.Join(testOutputRoot, "rendered")

	rm := render.NewRenderManager()
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"cards/card.html": `<article><h2>{{.title}}</h2>{{if .lead}}<p>{{.lead}}</p>{{end}}{{if .body}}<p>{{.body}}</p>{{end}}{{if .cta}}{{.cta}}{{end}}{{if .theme}}<span>{{.theme.tone}}</span>{{end}}</article>`,
		}
		content, exists := templates[templateName]
		return content, exists
	}
	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))
	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))
	rm.RegisterComponent(component.JSConfigGetName(), &component.JSRenderer{}, reflect.TypeOf(component.JSConfig{}))
	rm.RegisterComponent(component.SingleImageConfigGetName(), &component.SingleImageRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}, reflect.TypeOf(component.SingleImageConfig{}))
	rm.RegisterComponent(component.MultipleImagesConfigGetName(), &component.MultipleImagesRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}, reflect.TypeOf(component.MultipleImagesConfig{}))
	rm.RegisterComponent(component.LocalJSONConfigGetName(), &component.LocalJSONRenderer{
		TemplateProvider: templateProvider,
	}, reflect.TypeOf(component.LocalJSONConfig{}))

	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	menuRenderer := &component.MenuRenderer{
		TemplateProvider:     templateProvider,
		HyperMediasBySection: make(map[string][]composite.HyperMediaConfig),
	}
	rm.RegisterComponent(component.MenuConfigGetName(), menuRenderer, reflect.TypeOf(component.MenuConfig{}))
	pluginRenderer := &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	rm.SetPlugin("example", documentationExamplePlugin{})
	rm.RegisterComponent(component.PluginRenderGetName(), pluginRenderer, reflect.TypeOf(component.PluginConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	fragmentRenderer := &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	rm.RegisterComponent(composite.FragmentConfigGetName(), fragmentRenderer, reflect.TypeOf(composite.FragmentConfig{}))
	apiFragmentRenderer := &composite.ApiFragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	rm.RegisterComponent(composite.ApiFragmentRenderConfigGetName(), apiFragmentRenderer, reflect.TypeOf(composite.ApiFragmentRenderConfig{}))

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	hypermediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HyperMediaConfigGetName(), hypermediaRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))
	return rm
}

func configureYAMLProfileMenus(t *testing.T, rm *render.RenderManager, doc *yamlparser.Document, materialized map[string]interface{}) {
	t.Helper()
	rawMenuRenderer := rm.GetRenderComponent(component.MenuConfigGetName())
	menuRenderer, ok := rawMenuRenderer.(*component.MenuRenderer)
	if !ok || menuRenderer == nil {
		return
	}

	type menuCandidate struct {
		name   string
		config composite.HyperMediaConfig
	}

	candidatesBySection := make(map[string][]menuCandidate)
	for _, root := range doc.Roots {
		rootName := root.Name
		raw, exists := materialized[rootName]
		if !exists {
			continue
		}
		scope, ok := raw.(map[string]interface{})
		if !ok || scope["@type"] != composite.HyperMediaConfigGetName() {
			continue
		}
		section, _ := scope["section"].(string)
		if section == "" {
			continue
		}
		route, _ := scope["route"].(string)
		title, _ := scope["title"].(string)
		index := intFromInterface(scope["index"])
		candidatesBySection[section] = append(candidatesBySection[section], menuCandidate{
			name: rootName,
			config: composite.HyperMediaConfig{
				Title:   title,
				Route:   route,
				Section: section,
				Index:   index,
			},
		})
	}

	sections := make(map[string][]composite.HyperMediaConfig)
	for section, candidates := range candidatesBySection {
		hasConcretePages := false
		for _, candidate := range candidates {
			if candidate.name != "hypermedia" {
				hasConcretePages = true
				break
			}
		}
		for _, candidate := range candidates {
			if hasConcretePages && candidate.name == "hypermedia" {
				continue
			}
			sections[section] = append(sections[section], candidate.config)
		}
	}
	menuRenderer.HyperMediasBySection = sections
}

func rewriteYAMLReadableExpectedJSON(t *testing.T, path string, expectedJSON []byte) {
	t.Helper()
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read readable YAML case %s: %v", path, err)
	}
	raw := string(rawBytes)
	jsonHeader := "==== expected json ===="
	start := strings.Index(raw, jsonHeader)
	if start < 0 {
		t.Fatalf("%s missing expected JSON header", path)
	}
	afterHeader := start + len(jsonHeader)
	relativeEnd := strings.Index(raw[afterHeader:], "\n==== ")
	if relativeEnd < 0 {
		t.Fatalf("%s missing section after expected JSON header", path)
	}
	end := afterHeader + relativeEnd
	replacement := "\n" + strings.TrimRight(string(expectedJSON), "\n") + "\n"
	updated := raw[:afterHeader] + replacement + raw[end:]
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write readable YAML case %s: %v", path, err)
	}
}

func intFromInterface(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func canonicalJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal canonical JSON: %v", err)
	}
	var normalized interface{}
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatalf("normalize canonical JSON: %v", err)
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(normalized); err != nil {
		t.Fatalf("encode canonical JSON: %v", err)
	}
	return buf.Bytes()
}

func asMap(t *testing.T, value interface{}, label string) map[string]interface{} {
	t.Helper()
	typed, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("%s type = %T, want map[string]interface{}", label, value)
	}
	return typed
}

func readFixtureFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return raw
}

func normalizeFixtureText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n")
}
