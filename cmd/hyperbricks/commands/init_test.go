package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
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
	metadata := hb["metadata"].(map[string]interface{})
	if got := metadata["module"]; got != "demo" {
		t.Fatalf("module = %v", got)
	}
	if got := metadata["moduleversion"]; got != "1.0.0" {
		t.Fatalf("moduleversion = %v", got)
	}
	if got := metadata["hyperbricks"]; got != strings.TrimSpace(assets.VersionMD) {
		t.Fatalf("hyperbricks = %v", got)
	}
	for _, artifactField := range []string{"format", "format_version", "commit", "built_at", "source_hash"} {
		if _, present := metadata[artifactField]; present {
			t.Fatalf("source package contains artifact-only metadata %q", artifactField)
		}
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

func TestInitCommandUpdatesMetadataByNameRelativeAndAbsolutePath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	tests := []struct {
		name       string
		moduleRoot string
		selection  func(string) string
	}{
		{
			name:       "module name",
			moduleRoot: filepath.Join(root, "modules", "named-module"),
			selection:  func(string) string { return "named-module" },
		},
		{
			name:       "relative path",
			moduleRoot: filepath.Join(root, "custom", "relative-module"),
			selection: func(moduleRoot string) string {
				relative, err := filepath.Rel(root, moduleRoot)
				if err != nil {
					t.Fatal(err)
				}
				return "." + string(filepath.Separator) + relative
			},
		},
		{
			name:       "absolute path",
			moduleRoot: filepath.Join(root, "external", "absolute-module"),
			selection:  func(moduleRoot string) string { return moduleRoot },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := writeInitMetadataFixture(t, test.moduleRoot, `# keep this comment
hyperbricks:
  metadata:
    module: stale
    moduleversion: "1.0"
    format: zip
    format_version: "0"
    commit: stale
    built_at: "1970-01-01T00:00:00Z"
    source_hash: stale
    hyperbricks: v0.0.0
    owner: platform
  mode: live
`)
			output, err := executeInitCommand(t, "--module", test.selection(test.moduleRoot), "--update-metadata")
			if err != nil {
				t.Fatalf("metadata update failed: %v\n%s", err, output)
			}
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			metadata := decodeInitMetadata(t, content)
			if got := metadata["module"]; got != filepath.Base(test.moduleRoot) {
				t.Fatalf("module = %v, want %s", got, filepath.Base(test.moduleRoot))
			}
			if got := metadata["moduleversion"]; got != "1.0.0" {
				t.Fatalf("moduleversion = %v, want 1.0.0", got)
			}
			if got := metadata["hyperbricks"]; got != strings.TrimSpace(assets.VersionMD) {
				t.Fatalf("hyperbricks = %v", got)
			}
			if got := metadata["owner"]; got != "platform" {
				t.Fatalf("unrelated metadata = %v", got)
			}
			for _, field := range []string{"format", "format_version", "commit", "built_at", "source_hash"} {
				if _, present := metadata[field]; present {
					t.Fatalf("artifact-only field %q survived refresh", field)
				}
			}
			text := string(content)
			if !strings.Contains(text, "# keep this comment") || !strings.Contains(text, "mode: live") {
				t.Fatalf("unrelated YAML/comment was not preserved:\n%s", text)
			}
		})
	}
}

func TestInitCommandMetadataNoOpDoesNotRewrite(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	configPath := writeInitMetadataFixture(t, filepath.Join(root, "modules", "demo"), currentInitMetadataFixture("1.0.0"))
	oldTime := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(configPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	output, err := executeInitCommand(t, "--module", "demo", "--update-metadata")
	if err != nil {
		t.Fatalf("metadata update failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Metadata already current") {
		t.Fatalf("no-op output = %q", output)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(oldTime) {
		t.Fatalf("no-op modified file at %v, want %v", info.ModTime(), oldTime)
	}
}

func TestInitCommandMetadataFailuresDoNotCreateScaffold(t *testing.T) {
	tests := []struct {
		name         string
		prepare      func(*testing.T, string)
		selection    string
		missingPaths []string
	}{
		{
			name:      "missing module",
			prepare:   func(*testing.T, string) {},
			selection: "missing",
			missingPaths: []string{
				filepath.Join("modules", "missing"),
			},
		},
		{
			name: "missing package",
			prepare: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "modules", "demo"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			selection: "demo",
			missingPaths: []string{
				filepath.Join("modules", "demo", "hyperbricks"),
				filepath.Join("modules", "demo", "templates"),
				filepath.Join("modules", "demo", "resources"),
				filepath.Join("modules", "demo", "rendered"),
				filepath.Join("modules", "demo", "static"),
				filepath.Join("modules", "demo", "logs"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			test.prepare(t, root)
			if output, err := executeInitCommand(t, "--module", test.selection, "--update-metadata"); err == nil {
				t.Fatalf("metadata update succeeded unexpectedly:\n%s", output)
			}
			for _, path := range append(test.missingPaths, filepath.Join("bin", "plugins")) {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("metadata-only failure created %s: %v", path, err)
				}
			}
		})
	}
}

func TestInitCommandBumpVersionModes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "bare patch", args: []string{"--bump-version"}, want: "1.2.4"},
		{name: "explicit patch", args: []string{"--bump-version=patch"}, want: "1.2.4"},
		{name: "explicit minor", args: []string{"--bump-version=minor"}, want: "1.3.0"},
		{name: "explicit major", args: []string{"--bump-version=major"}, want: "2.0.0"},
		{name: "update plus bump once", args: []string{"--update-metadata", "--bump-version=patch"}, want: "1.2.4"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			configPath := writeInitMetadataFixture(t, filepath.Join(root, "modules", "demo"), currentInitMetadataFixture("1.2.3"))
			args := append([]string{"--module", "demo"}, test.args...)
			output, err := executeInitCommand(t, args...)
			if err != nil {
				t.Fatalf("version bump failed: %v\n%s", err, output)
			}
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeInitMetadata(t, content)["moduleversion"]; got != test.want {
				t.Fatalf("moduleversion = %v, want %s", got, test.want)
			}
		})
	}
}

func TestInitCommandInvalidBumpLeavesPackageUnchanged(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unsupported value", args: []string{"--bump-version=build"}},
		{name: "explicit empty value", args: []string{"--bump-version="}},
		{name: "positional value", args: []string{"--bump-version", "minor"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			configPath := writeInitMetadataFixture(t, filepath.Join(root, "modules", "demo"), currentInitMetadataFixture("1.2.3"))
			original, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			args := append([]string{"--module", "demo"}, test.args...)
			if output, err := executeInitCommand(t, args...); err == nil {
				t.Fatalf("invalid bump succeeded unexpectedly:\n%s", output)
			}
			current, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(current, original) {
				t.Fatalf("invalid bump changed package:\n%s", current)
			}
		})
	}
}

func executeInitCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	previousModule, previousExit, previousExitCode := module, Exit, ExitCode
	t.Cleanup(func() {
		module, Exit, ExitCode = previousModule, previousExit, previousExitCode
	})
	Exit = false
	ExitCode = 0
	command := NewInitCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}

func writeInitMetadataFixture(t *testing.T, moduleRoot, content string) string {
	t.Helper()
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(moduleRoot, PackageConfigFileName)
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func currentInitMetadataFixture(version string) string {
	return "hyperbricks:\n  metadata:\n    module: demo\n    moduleversion: \"" + version + "\"\n    hyperbricks: " + strings.TrimSpace(assets.VersionMD) + "\n"
}

func decodeInitMetadata(t *testing.T, content []byte) map[string]interface{} {
	t.Helper()
	var document struct {
		HyperBricks struct {
			Metadata map[string]interface{} `yaml:"metadata"`
		} `yaml:"hyperbricks"`
	}
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	return document.HyperBricks.Metadata
}
