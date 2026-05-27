package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	hbschema "github.com/hyperbricks/hyperbricks/pkg/schema"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

var updateYAMLGoldensFlag = flag.Bool("update-yaml-golden", false, "rewrite HyperBricks YAML materialized JSON golden fixtures")
var updateYAMLReadableFlag = flag.Bool("update-yaml-readable", false, "rewrite migrated HyperBricks YAML readable expected JSON sections")

type yamlReadableCase struct {
	Path                     string
	Scope                    string
	Source                   string
	Explainer                string
	ExpectedPreprocessedYAML string
	ExpectedMaterializedJSON string
	ExpectedRuntimeJSON      string
	ExpectedJSON             string
	ExpectedOutput           string
}

var yamlCoreCorpus = []string{
	"text-html-tree.hyperbricks.yaml",
	"template-values.hyperbricks.yaml",
	"inheritance-override.hyperbricks.yaml",
	"ordered-children.hyperbricks.yaml",
	"nested-tree-3-level.hyperbricks.yaml",
	"head-assets.hyperbricks.yaml",
	"hypermedia-route.hyperbricks.yaml",
	"fragment-response.hyperbricks.yaml",
	"menu-items.hyperbricks.yaml",
	"api-render-request.hyperbricks.yaml",
	"reserved-name-collision.hyperbricks.yaml",
}

var yamlLegacyParityCorpus = map[string]string{
	"text-html-tree.hyperbricks.yaml":          "text-html-tree.legacy.hyperbricks",
	"inheritance-override.hyperbricks.yaml":    "inheritance-override.legacy.hyperbricks",
	"ordered-children.hyperbricks.yaml":        "ordered-children.legacy.hyperbricks",
	"nested-tree-3-level.hyperbricks.yaml":     "nested-tree-3-level.legacy.hyperbricks",
	"head-assets.hyperbricks.yaml":             "head-assets.legacy.hyperbricks",
	"hypermedia-route.hyperbricks.yaml":        "hypermedia-route.legacy.hyperbricks",
	"fragment-response.hyperbricks.yaml":       "fragment-response.legacy.hyperbricks",
	"menu-items.hyperbricks.yaml":              "menu-items.legacy.hyperbricks",
	"reserved-name-collision.hyperbricks.yaml": "reserved-name-collision.legacy.hyperbricks",
}

func TestYAMLProfileFixturesParseAndMaterialize(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("hyperbricks-yaml-test-files", "*.hyperbricks.yaml"))
	if err != nil {
		t.Fatalf("glob YAML fixtures: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no YAML profile fixtures found")
	}

	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw := readFixtureFile(t, path)
			doc, err := yamlparser.ParseBytes(raw)
			if err != nil {
				t.Fatalf("parse YAML fixture: %v", err)
			}
			materialized, err := doc.Materialize()
			if err != nil {
				t.Fatalf("materialize YAML fixture: %v", err)
			}
			if len(doc.Roots) > 0 && len(materialized) == 0 {
				t.Fatal("materialized map is empty for non-empty fixture")
			}
			assertMaterializedGolden(t, path, materialized)
		})
	}
}

func TestYAMLProfileLegacyParity(t *testing.T) {
	ensureLegacyParserTypes()
	for yamlName, legacyName := range yamlLegacyParityCorpus {
		t.Run(yamlName, func(t *testing.T) {
			yamlMaterialized := materializeYAMLFixture(t, yamlName)
			legacyPath := filepath.Join("hyperbricks-yaml-test-files", legacyName)
			legacyRaw := string(readFixtureFile(t, legacyPath))
			legacyMaterialized := parser.ParseHyperScript(legacyRaw)

			got := canonicalJSON(t, normalizeForLegacyParity(yamlMaterialized))
			want := canonicalJSON(t, normalizeForLegacyParity(legacyMaterialized))
			if !bytes.Equal(got, want) {
				t.Fatalf("legacy parity mismatch for %s\n--- yaml ---\n%s\n--- legacy ---\n%s", yamlName, got, want)
			}
		})
	}
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
	matches, err := filepath.Glob(filepath.Join("hyperbricks-yaml-test-files", "*.hyperbricks.yaml.test"))
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

func TestYAMLProfileReadableCasesCoverCoreCorpus(t *testing.T) {
	for _, name := range yamlCoreCorpus {
		readablePath := filepath.Join("hyperbricks-yaml-test-files", name+".test")
		if _, err := os.Stat(readablePath); err != nil {
			t.Fatalf("readable YAML test case %s is missing: %v", name+".test", err)
		}
	}
}

func TestYAMLProfileReadableCasesCoverMigratedDocumentationFixtures(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("hyperbricks-yaml-test-files", "*.hyperbricks"))
	if err != nil {
		t.Fatalf("glob migrated documentation fixtures: %v", err)
	}
	for _, path := range matches {
		name := filepath.Base(path)
		if strings.HasSuffix(name, ".legacy.hyperbricks") {
			continue
		}
		readablePath := strings.TrimSuffix(path, ".hyperbricks") + ".hyperbricks.yaml.test"
		if _, err := os.Stat(readablePath); err != nil {
			t.Fatalf("readable YAML test case %s is missing for %s: %v", filepath.Base(readablePath), name, err)
		}
	}
}

func TestYAMLProfileCoreCorpusExists(t *testing.T) {
	for _, name := range yamlCoreCorpus {
		path := filepath.Join("hyperbricks-yaml-test-files", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("core corpus fixture %s is missing: %v", name, err)
		}
	}
}

func TestYAMLProfileLegacyParityFixturesExist(t *testing.T) {
	for yamlName, legacyName := range yamlLegacyParityCorpus {
		if _, err := os.Stat(filepath.Join("hyperbricks-yaml-test-files", yamlName)); err != nil {
			t.Fatalf("YAML parity fixture %s is missing: %v", yamlName, err)
		}
		if _, err := os.Stat(filepath.Join("hyperbricks-yaml-test-files", legacyName)); err != nil {
			t.Fatalf("legacy parity fixture %s is missing: %v", legacyName, err)
		}
	}
}

func TestYAMLProfileFixturePreservesSourceOrder(t *testing.T) {
	materialized := materializeYAMLFixture(t, "ordered-children.hyperbricks.yaml")
	page := asMap(t, materialized["page"], "page")
	want := []string{"zeta", "alpha", "middle"}
	if got := page["@order"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page @order = %#v, want %#v", got, want)
	}
}

func TestYAMLProfileFixturePreservesThreeLevelNestedTree(t *testing.T) {
	materialized := materializeYAMLFixture(t, "nested-tree-3-level.hyperbricks.yaml")
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
	materialized := materializeYAMLFixture(t, "inheritance-override.hyperbricks.yaml")
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
	materialized := materializeYAMLFixture(t, "template-values.hyperbricks.yaml")
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
	materialized := materializeYAMLFixture(t, "reserved-name-collision.hyperbricks.yaml")
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
	path := filepath.Join("hyperbricks-yaml-test-files", name)
	raw := readFixtureFile(t, path)
	doc, err := yamlparser.ParseBytes(raw)
	if err != nil {
		t.Fatalf("parse YAML fixture %s: %v", name, err)
	}
	materialized, err := doc.Materialize()
	if err != nil {
		t.Fatalf("materialize YAML fixture %s: %v", name, err)
	}
	return materialized
}

func assertMaterializedGolden(t *testing.T, fixturePath string, materialized map[string]interface{}) {
	t.Helper()
	goldenPath := yamlGoldenPath(fixturePath)
	got := canonicalJSON(t, materialized)
	if *updateYAMLGoldensFlag {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write YAML golden %s: %v", goldenPath, err)
		}
		return
	}

	rawExpected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read YAML golden %s: %v; run go test ./test/docs -run TestYAMLProfileFixturesParseAndMaterialize -update-yaml-golden", goldenPath, err)
	}
	var expected interface{}
	if err := json.Unmarshal(rawExpected, &expected); err != nil {
		t.Fatalf("parse YAML golden %s: %v", goldenPath, err)
	}
	want := canonicalJSON(t, expected)
	if !bytes.Equal(got, want) {
		t.Fatalf("materialized JSON mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(fixturePath), got, want)
	}
}

func yamlGoldenPath(fixturePath string) string {
	return strings.TrimSuffix(fixturePath, ".hyperbricks.yaml") + ".materialized.json"
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
	if *updateYAMLReadableFlag && isMigratedDocumentationYAMLReadableCase(testCase.Path) {
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
		return yamlparser.Options{}
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
			Resources: filepath.Join("hyperbricks-yaml-test-files", "pipeline-assets"),
		},
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
			"cards/card.html":              `<article><h2>{{.title}}</h2>{{if .lead}}<p>{{.lead}}</p>{{end}}{{if .body}}<p>{{.body}}</p>{{end}}{{if .cta}}{{.cta}}{{end}}{{if .theme}}<span>{{.theme.tone}}</span>{{end}}</article>`,
			"{{TEMPLATE:cards/card.html}}": `<article><h2>{{.title}}</h2>{{if .lead}}<p>{{.lead}}</p>{{end}}{{if .body}}<p>{{.body}}</p>{{end}}{{if .cta}}{{.cta}}{{end}}{{if .theme}}<span>{{.theme.tone}}</span>{{end}}</article>`,
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

func isMigratedDocumentationYAMLReadableCase(path string) bool {
	sourcePath := strings.TrimSuffix(path, ".yaml.test")
	if sourcePath == path {
		return false
	}
	_, err := os.Stat(sourcePath)
	return err == nil
}

func rewriteYAMLReadableExpectedJSON(t *testing.T, path string, expectedJSON []byte) {
	t.Helper()
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read readable YAML case %s: %v", path, err)
	}
	raw := string(rawBytes)
	jsonHeader := "==== expected json ===="
	outputHeader := "\n==== expected output ===="
	start := strings.Index(raw, jsonHeader)
	if start < 0 {
		t.Fatalf("%s missing expected JSON header", path)
	}
	afterHeader := start + len(jsonHeader)
	relativeEnd := strings.Index(raw[afterHeader:], outputHeader)
	if relativeEnd < 0 {
		t.Fatalf("%s missing expected output header", path)
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

func normalizeForLegacyParity(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if key == "@order" {
				continue
			}
			out[key] = normalizeForLegacyParity(nested)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			out = append(out, normalizeForLegacyParity(nested))
		}
		return out
	case string:
		return normalizeLegacyParityString(typed)
	case nil:
		return nil
	default:
		return normalizeLegacyParityString(fmt.Sprint(typed))
	}
}

func normalizeLegacyParityString(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.TrimPrefix(value, "\n")
	return strings.TrimRight(value, "\n")
}

func ensureLegacyParserTypes() {
	for _, token := range []string{
		"<HYPERMEDIA>",
		"<FRAGMENT>",
		"<API_RENDER>",
		"<API_FRAGMENT_RENDER>",
		"<TREE>",
		"<TEMPLATE>",
		"<HEAD>",
		"<TEXT>",
		"<HTML>",
		"<IMAGE>",
		"<IMAGES>",
		"<MENU>",
		"<CSS>",
		"<STYLES>",
		"<JAVASCRIPT>",
		"<JS>",
		"<JSON>",
		"<JSON_RENDER>",
		"<PLUGIN>",
	} {
		parser.KnownTypes[token] = true
	}
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
