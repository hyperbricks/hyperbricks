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

func TestDefaultInitAssetsWriteThreePageStarter(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HYPERBRICKS_INIT_FIXTURE", "starter test value")
	if err := initializeModule("demo"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("modules", "demo")
	config, err := yamlparser.ProcessConfigFile(filepath.Join(root, PackageConfigFileName), yamlparser.Options{
		Paths: yamlparser.PathMarkers{Module: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	hb := config.Materialized["hyperbricks"].(map[string]interface{})
	if got := hb["metadata"].(map[string]interface{})["module"]; got != "demo" {
		t.Fatalf("module = %v", got)
	}
	if got := hb["directories"].(map[string]interface{})["render"]; got != filepath.Join(root, "rendered") {
		t.Fatalf("render = %v", got)
	}
	parser.ClearTemplateStore()
	result, err := yamlparser.ProcessFile(filepath.Join(root, "hyperbricks", "hello-world.hyperbricks.yaml"), yamlparser.Options{
		TemplateDir: filepath.Join(root, "templates"), Config: config.Materialized,
		Paths: yamlparser.PathMarkers{Module: root, Resources: filepath.Join(root, "resources"), Static: filepath.Join(root, "static")},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"overview_page", "templates_page", "fragments_page"} {
		page := result.Materialized[name].(map[string]interface{})
		if page["route"] != []string{"index", "templates", "fragments"}[i] || page["section"] != "scaffold_navigation" {
			t.Fatalf("incorrect route or section for %s: %#v", name, page)
		}
		if page["title"] != []string{"Overview", "Templates", "Fragments"}[i] {
			t.Fatalf("title = %v", page["title"])
		}
		content := page["content"].(map[string]interface{})
		if content["template"] != "shell.html" {
			t.Fatalf("missing shared shell: %v", content)
		}
	}
	menu := result.Materialized["scaffold_navigation"].(map[string]interface{})
	if menu["section"] != "scaffold_navigation" || menu["sort"] != "index" {
		t.Fatalf("menu = %#v", menu)
	}
	if !strings.Contains(menu["item"].(string), "hx-select-oob") || !strings.Contains(menu["active"].(string), `aria-current="page"`) {
		t.Fatal("missing HTMX menu or active state")
	}
	templates := result.Materialized["templates_content"].(map[string]interface{})
	values := templates["inline_template"].(map[string]interface{})["values"].(map[string]interface{})
	if values["environment"] != "starter test value" || !strings.Contains(values["resource_copy"].(string), "YAML file resolver") {
		t.Fatalf("resolver values = %#v", values)
	}
	article := templates["nested_tree"].(map[string]interface{})["article"].(map[string]interface{})
	if !reflect.DeepEqual(article["@order"], []string{"heading", "copy"}) {
		t.Fatalf("nested order = %#v", article["@order"])
	}
	fragments := result.Materialized["fragments_content"].(map[string]interface{})
	if _, present := fragments["path_probe"]; present {
		t.Fatal("filesystem path probe must not ship")
	}
	cta := fragments["template_file"].(map[string]interface{})["values"].(map[string]interface{})["cta"].(map[string]interface{})["value"].(string)
	for _, attr := range []string{`hx-get="/hello-status"`, `hx-target="#hello-status"`, `hx-swap="outerHTML"`} {
		if !strings.Contains(cta, attr) {
			t.Fatalf("missing %s in CTA", attr)
		}
	}
	status := result.Materialized["status_fragment"].(map[string]interface{})
	if status["route"] != "hello-status" {
		t.Fatalf("fragment route = %v", status["route"])
	}
	for _, file := range []string{"README.md", "VENDOR.md", ".gitignore", "resources/vendor/htmx-4.0.0.js", "resources/css/app.css", "resources/js/app.js", "templates/overview.html", "templates/shell.html"} {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Fatal(err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "hyperbricks start -m 'demo'") || strings.Contains(string(readme), "__MODULE") || strings.Contains(string(readme), "scaffold-upgrade") {
		t.Fatalf("unpersonalized README: %s", readme)
	}
	for _, dir := range []string{"rendered", "static"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("generated files embedded in %s", dir)
		}
	}
}

func TestInitPersonalizesQuotedModuleNames(t *testing.T) {
	t.Chdir(t.TempDir())
	name := "demo's project"
	if err := initializeModule(name); err != nil {
		t.Fatal(err)
	}
	result, err := yamlparser.ProcessConfigFile(filepath.Join("modules", name, PackageConfigFileName), yamlparser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	metadata := result.Materialized["hyperbricks"].(map[string]interface{})["metadata"].(map[string]interface{})
	if metadata["module"] != name {
		t.Fatalf("module = %v", metadata["module"])
	}
	readme, err := os.ReadFile(filepath.Join("modules", name, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), `-m 'demo'"'"'s project'`) {
		t.Fatalf("README module argument is not shell quoted: %s", readme)
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
