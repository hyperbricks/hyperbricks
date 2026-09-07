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

	if err := initializeModule("demo"); err != nil {
		t.Fatalf("initialize module: %v", err)
	}

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
		"imports:",
		"partials/init-components.hyperbricks.yaml",
		"Nested TREE",
		"status_fragment:",
		"file: hello-status.html",
		"Inline template",
	} {
		if !strings.Contains(string(yamlContent), want) {
			t.Fatalf("YAML hello world missing %q:\n%s", want, yamlContent)
		}
	}
	partialPath := filepath.Join("modules", "demo", "hyperbricks", "partials", "init-components.hyperbricks.yaml")
	partialContent, err := os.ReadFile(partialPath)
	if err != nil {
		t.Fatalf("read imported YAML partial: %v", err)
	}
	for _, want := range []string{
		"init_card:",
		"template:",
		"file: hello-card.html",
	} {
		if !strings.Contains(string(partialContent), want) {
			t.Fatalf("YAML partial missing %q:\n%s", want, partialContent)
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
	statusTemplatePath := filepath.Join("modules", "demo", "templates", "hello-status.html")
	statusTemplateContent, err := os.ReadFile(statusTemplatePath)
	if err != nil {
		t.Fatalf("read extracted status template file: %v", err)
	}
	if !strings.Contains(string(statusTemplateContent), `id="hello-status"`) {
		t.Fatalf("status template does not contain fragment marker:\n%s", statusTemplateContent)
	}
	resourcePath := filepath.Join("modules", "demo", "resources", "init-copy.txt")
	resourceContent, err := os.ReadFile(resourcePath)
	if err != nil {
		t.Fatalf("read extracted resource file: %v", err)
	}
	if !strings.Contains(string(resourceContent), "YAML file resolver") {
		t.Fatalf("resource fixture does not contain resolver copy:\n%s", resourceContent)
	}

	parser.ClearTemplateStore()
	result, err := yamlparser.ProcessFile(yamlPath, yamlparser.Options{
		TemplateDir: filepath.Join("modules", "demo", "templates"),
		Config:      packageResult.Materialized,
		Paths: yamlparser.PathMarkers{
			Module:    filepath.Join("modules", "demo"),
			Resources: filepath.Join("modules", "demo", "resources"),
			Static:    filepath.Join("modules", "demo", "static"),
		},
	})
	if err != nil {
		t.Fatalf("process generated YAML hello world: %v", err)
	}
	if _, ok := result.Materialized["init_card"].(map[string]interface{}); !ok {
		t.Fatalf("imported init_card = %T, want map", result.Materialized["init_card"])
	}
	page, ok := result.Materialized["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized page = %T, want map", result.Materialized["page"])
	}
	if page["title"] != "Hello World | init-module" {
		t.Fatalf("materialized page title = %#v", page["title"])
	}
	head, ok := page["head"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized page.head = %T, want map", page["head"])
	}
	meta, ok := head["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized page.head.meta = %T, want map", head["meta"])
	}
	if meta["description"] != "YAML Runtime Fixture generated by the init module." {
		t.Fatalf("materialized meta.description = %#v", meta["description"])
	}
	main, ok := page["main"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized page.main = %T, want map", page["main"])
	}
	wantOrder := []string{"intro", "template_file", "inline_template", "nested_tree", "path_probe"}
	if got := main["@order"]; !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("page.main @order = %#v, want %#v", got, wantOrder)
	}
	templateFile, ok := main["template_file"].(map[string]interface{})
	if !ok {
		t.Fatalf("template_file = %T, want map", main["template_file"])
	}
	if templateFile["template"] != "hello-card.html" {
		t.Fatalf("template.file resolver was not resolved, got %#v", templateFile["template"])
	}
	templateValues := templateFile["values"].(map[string]interface{})
	cta := templateValues["cta"].(map[string]interface{})
	if cta["value"] != `<a href="/hello-status">Open status fragment</a>` {
		t.Fatalf("template cta value = %#v", cta["value"])
	}
	inlineTemplate := main["inline_template"].(map[string]interface{})
	inlineValues := inlineTemplate["values"].(map[string]interface{})
	if !strings.Contains(inlineValues["resource_copy"].(string), "modules/demo/resources/init-copy.txt") {
		t.Fatalf("file resolver value = %#v", inlineValues["resource_copy"])
	}
	if inlineValues["environment"] != "Environment resolver fallback is active." {
		t.Fatalf("env resolver fallback = %#v", inlineValues["environment"])
	}
	pathProbe := main["path_probe"].(map[string]interface{})
	pathValues := pathProbe["values"].(map[string]interface{})
	if pathValues["static_path"] != filepath.Join("modules", "demo", "static", "css", "app.css") {
		t.Fatalf("path resolver value = %#v", pathValues["static_path"])
	}
	nestedTree := main["nested_tree"].(map[string]interface{})
	article := nestedTree["article"].(map[string]interface{})
	if got := article["@order"]; !reflect.DeepEqual(got, []string{"heading", "copy"}) {
		t.Fatalf("nested article @order = %#v", got)
	}
	if storedTemplate, found := parser.GetTemplate("hello-card.html"); !found || !strings.Contains(storedTemplate, "{{.body}}") {
		t.Fatalf("template.file resolver did not cache hello-card.html, found=%v content=%q", found, storedTemplate)
	}
	if storedTemplate, found := parser.GetTemplate("hello-status.html"); !found || !strings.Contains(storedTemplate, "hello-status") {
		t.Fatalf("template.file resolver did not cache hello-status.html, found=%v content=%q", found, storedTemplate)
	}
	statusFragment, ok := result.Materialized["status_fragment"].(map[string]interface{})
	if !ok {
		t.Fatalf("status_fragment = %T, want map", result.Materialized["status_fragment"])
	}
	if statusFragment["route"] != "hello-status" {
		t.Fatalf("status fragment route = %#v", statusFragment["route"])
	}
	response, ok := statusFragment["response"].(map[string]interface{})
	if !ok {
		t.Fatalf("status fragment response = %T, want map", statusFragment["response"])
	}
	headers, ok := response["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("status fragment response.headers = %T, want map", response["headers"])
	}
	if headers["HX-Trigger"] != "init-fixture-status" || headers["HX-Retarget"] != "#hello-status" || headers["HX-Reswap"] != "outerHTML" {
		t.Fatalf("status fragment response.headers = %#v", headers)
	}

}

func TestInitializeModulePreservesExistingFilesAndRepairsMissingScaffold(t *testing.T) {
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

	if err := initializeModule("demo"); err != nil {
		t.Fatalf("first initialize module: %v", err)
	}

	configPath := filepath.Join("modules", "demo", PackageConfigFileName)
	yamlPath := filepath.Join("modules", "demo", "hyperbricks", "hello-world.hyperbricks.yaml")
	missingTemplatePath := filepath.Join("modules", "demo", "templates", "hello-status.html")
	missingDirectoryPath := filepath.Join("modules", "demo", "static")
	customConfig := []byte("custom: preserved\n")
	customYAML := []byte("custom_page:\n  - type: text\n  - value: preserved\n")
	if err := os.WriteFile(configPath, customConfig, 0644); err != nil {
		t.Fatalf("replace config fixture: %v", err)
	}
	if err := os.WriteFile(yamlPath, customYAML, 0644); err != nil {
		t.Fatalf("replace YAML fixture: %v", err)
	}
	if err := os.Remove(missingTemplatePath); err != nil {
		t.Fatalf("remove template fixture: %v", err)
	}
	if err := os.Remove(missingDirectoryPath); err != nil {
		t.Fatalf("remove directory fixture: %v", err)
	}

	if err := initializeModule("demo"); err != nil {
		t.Fatalf("second initialize module: %v", err)
	}

	if got, err := os.ReadFile(configPath); err != nil || !reflect.DeepEqual(got, customConfig) {
		t.Fatalf("config after second init = %q, %v; want preserved content", got, err)
	}
	if got, err := os.ReadFile(yamlPath); err != nil || !reflect.DeepEqual(got, customYAML) {
		t.Fatalf("YAML after second init = %q, %v; want preserved content", got, err)
	}
	if _, err := os.Stat(missingTemplatePath); err != nil {
		t.Fatalf("missing template was not restored: %v", err)
	}
	if info, err := os.Stat(missingDirectoryPath); err != nil || !info.IsDir() {
		t.Fatalf("missing directory was not restored: info=%v err=%v", info, err)
	}
}

func TestInitializeModulePreflightsConflictsBeforeWriting(t *testing.T) {
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

	moduleDir := filepath.Join("modules", "demo")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}
	conflictPath := filepath.Join(moduleDir, "templates")
	if err := os.WriteFile(conflictPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create conflicting file: %v", err)
	}

	err = initializeModule("demo")
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("initialize module error = %v, want directory conflict", err)
	}
	for _, path := range []string{
		filepath.Join(moduleDir, PackageConfigFileName),
		filepath.Join(moduleDir, "rendered"),
		filepath.Join("bin", "plugins"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("preflight created %s before reporting conflict: %v", path, statErr)
		}
	}
}

func TestInitializeModulePreflightsFileConflictsBeforeWriting(t *testing.T) {
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

	moduleDir := filepath.Join("modules", "demo")
	configPath := filepath.Join(moduleDir, PackageConfigFileName)
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("create conflicting config directory: %v", err)
	}

	err = initializeModule("demo")
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("initialize module error = %v, want file conflict", err)
	}
	for _, path := range []string{
		filepath.Join(moduleDir, "rendered"),
		filepath.Join("bin", "plugins"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("preflight created %s before reporting conflict: %v", path, statErr)
		}
	}
}

func TestValidateInitModuleNameRejectsPathsAndEmptyValues(t *testing.T) {
	invalidNames := []string{
		"",
		"   ",
		".",
		"..",
		filepath.Join("..", "outside"),
		filepath.Join("nested", "module"),
		filepath.Join(t.TempDir(), "absolute"),
	}
	for _, name := range invalidNames {
		if _, err := validateInitModuleName(name); err == nil {
			t.Fatalf("expected module name %q to be rejected", name)
		}
	}

	if got, err := validateInitModuleName("  demo  "); err != nil || got != "demo" {
		t.Fatalf("validated module name = %q, %v; want demo", got, err)
	}
}

func TestInitCommandUsesDefaultModuleAndReturnsWithoutProcessExit(t *testing.T) {
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

	previousModule, previousExit, previousExitCode := module, Exit, ExitCode
	t.Cleanup(func() {
		module, Exit, ExitCode = previousModule, previousExit, previousExitCode
	})
	Exit = false
	ExitCode = 0

	command := NewInitCommand()
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute init command: %v", err)
	}
	if !Exit || ExitCode != 0 {
		t.Fatalf("exit state = (%t, %d), want successful command exit", Exit, ExitCode)
	}
	if _, err := os.Stat(filepath.Join("modules", "default", PackageConfigFileName)); err != nil {
		t.Fatalf("default module config was not created: %v", err)
	}
}

func TestInitCommandRejectsInvalidModuleWithFailureStatus(t *testing.T) {
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

	previousModule, previousExit, previousExitCode := module, Exit, ExitCode
	t.Cleanup(func() {
		module, Exit, ExitCode = previousModule, previousExit, previousExitCode
	})
	Exit = false
	ExitCode = 0

	command := NewInitCommand()
	command.SetArgs([]string{"--module", "../outside"})
	if err := command.Execute(); err == nil {
		t.Fatal("execute init command returned nil, want invalid module error")
	}
	if !Exit || ExitCode != 1 {
		t.Fatalf("exit state = (%t, %d), want failed command exit", Exit, ExitCode)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "outside")); !os.IsNotExist(err) {
		t.Fatalf("invalid module selection created an outside path: %v", err)
	}
}
