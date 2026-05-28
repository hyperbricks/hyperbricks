package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestDefaultInitAssetsWriteYAMLHelloWorld(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prevWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	createModuleDirectories("demo")
	createHbConfig("demo")
	extractEmbeddedFiles("demo")

	packagePath := filepath.Join("modules", "demo", "package.hyperbricks.yaml")
	packageContent, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatalf("read package.hyperbricks.yaml: %v", err)
	}
	for _, want := range []string{
		"hyperbricks:",
		"directories:",
		"base: module",
		"path: hyperbricks",
	} {
		if !strings.Contains(string(packageContent), want) {
			t.Fatalf("package.hyperbricks.yaml missing %q:\n%s", want, packageContent)
		}
	}
	packageResult, err := yamlparser.ProcessConfigFile(packagePath, yamlparser.Options{
		Paths: yamlparser.PathMarkers{
			Module: filepath.Join("modules", "demo"),
		},
	})
	if err != nil {
		t.Fatalf("process package.hyperbricks.yaml: %v", err)
	}
	hyperbricks, ok := packageResult.Materialized["hyperbricks"].(map[string]interface{})
	if !ok {
		t.Fatalf("package hyperbricks = %T, want map", packageResult.Materialized["hyperbricks"])
	}
	directories, ok := hyperbricks["directories"].(map[string]interface{})
	if !ok {
		t.Fatalf("package directories = %T, want map", hyperbricks["directories"])
	}
	if directories["render"] != filepath.Join("modules", "demo", "rendered") {
		t.Fatalf("package render directory = %#v", directories["render"])
	}

	yamlPath := filepath.Join("modules", "demo", "hyperbricks", "hello-world.hyperbricks.yaml")
	yamlContent, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read YAML hello world fixture: %v", err)
	}
	for _, want := range []string{
		"page:",
		"- type: hypermedia",
		"- route: index",
		"<h1>Hello World</h1>",
		"template:",
		"file: hello-card.html",
		"Inline template",
	} {
		if !strings.Contains(string(yamlContent), want) {
			t.Fatalf("YAML hello world missing %q:\n%s", want, yamlContent)
		}
	}

	templatePath := filepath.Join("modules", "demo", "templates", "hello-card.html")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read extracted template file: %v", err)
	}
	if !strings.Contains(string(templateContent), "{{.heading}}") {
		t.Fatalf("template file does not contain value placeholders:\n%s", templateContent)
	}

	parser.ClearTemplateStore()
	result, err := yamlparser.ProcessFile(yamlPath, yamlparser.Options{
		TemplateDir: filepath.Join("modules", "demo", "templates"),
	})
	if err != nil {
		t.Fatalf("process generated YAML hello world: %v", err)
	}
	page, ok := result.Materialized["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized page = %T, want map", result.Materialized["page"])
	}
	main, ok := page["main"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized page.main = %T, want map", page["main"])
	}
	if got := main["@order"]; !reflect.DeepEqual(got, []string{"intro", "template_file", "inline_template"}) {
		t.Fatalf("page.main @order = %#v, want intro/template_file/inline_template", got)
	}
	templateFile, ok := main["template_file"].(map[string]interface{})
	if !ok {
		t.Fatalf("template_file = %T, want map", main["template_file"])
	}
	if templateFile["template"] != "hello-card.html" {
		t.Fatalf("template.file resolver was not resolved, got %#v", templateFile["template"])
	}
	if storedTemplate, found := parser.GetTemplate("hello-card.html"); !found || !strings.Contains(storedTemplate, "{{.body}}") {
		t.Fatalf("template.file resolver did not cache hello-card.html, found=%v content=%q", found, storedTemplate)
	}

	legacyPath := filepath.Join("modules", "demo", "hyperbricks", "hello-world.hyperbricks")
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy hello-world.hyperbricks should not be created, stat err: %v", err)
	}
}
