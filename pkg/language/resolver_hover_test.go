package language

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolverHoverExplainsBlockAndFlowResolvers(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	for _, test := range []struct {
		name, value, needle, want string
	}{
		{"file contents", "{file: {base: resources, path: docs/guide.md}}", "file:", "Reads file contents"},
		{"path", "{path: {base: resources, path: docs/guide.md}}", "path:", "not file contents"},
		{"path base", "{file: {base: resources, path: docs/guide.md}}", "base:", "Named directory"},
		{"path parts", "{path: {base: resources, parts: [docs, guide.md]}}", "parts:", "every segment is a literal scalar"},
		{"var", "{var: page.title}", "var:", "source `vars`"},
		{"var default", "{var: {name: page.title, default: Untitled}}", "default:", "Fallback"},
		{"env", "{env: HB_TEST_EDITOR_SECRET}", "env:", "never shown"},
		{"required", "{env: {name: SITE_TITLE, required: true}}", "required:", "error instead of a warning"},
		{"config", "{config: myconf.site.title}", "config:", "runtime configuration"},
		{"config path", "{config: {path: myconf.site.title}}", "path:", "Dotted runtime"},
		{"format", "{format: '%s', args: [{var: page.title}]}", "format:", "fmt.Sprintf"},
		{"format args", "{format: '%s', args: [{var: page.title}]}", "args:", "Ordered arguments"},
		{"nested argument", "{format: '%s', args: [{var: page.title}]}", "var:", "source `vars`"},
		{"block resolver", "\n      file:\n        base: resources\n        path: docs/guide.md", "file:", "Reads file contents"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "page:\n  - type: text\n  - value: " + test.value + "\n"
			hover := analyzer.resolverHover(uri, source, definitionTestPosition(t, source, test.needle))
			if hover == nil || !strings.Contains(hover.Contents.Value, test.want) {
				t.Fatalf("hover = %#v, want %q", hover, test.want)
			}
			wantRange := definitionTestRange(t, source, strings.TrimSuffix(test.needle, ":"))
			if hover.Range == nil || *hover.Range != wantRange {
				t.Fatalf("hover range = %#v, want %#v", hover.Range, wantRange)
			}
		})
	}
	t.Setenv("HB_TEST_EDITOR_SECRET", "must-never-be-disclosed")
	source := "page:\n  - type: text\n  - value: {env: HB_TEST_EDITOR_SECRET}\n"
	hover := analyzer.resolverHover(uri, source, definitionTestPosition(t, source, "env:"))
	if hover == nil || strings.Contains(hover.Contents.Value, "must-never-be-disclosed") {
		t.Fatalf("environment hover = %#v", hover)
	}
}

func TestResolverHoverKeepsComponentFieldsDataAndStringsSeparate(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	for _, test := range []struct{ name, source, needle string }{
		{"native file field", "page:\n  - type: markdown\n  - file: docs/guide.md\n", "file:"},
		{"plain data path", "page:\n  - type: plugin\n  - data: {base: resources, path: docs/guide.md}\n", "path:"},
		{"plain data env", "page:\n  - type: plugin\n  - data: {env: local, name: ordinary}\n", "env:"},
		{"quoted string", "page:\n  - type: text\n  - value: '{file: docs/guide.md}'\n", "file:"},
		{"block string", "page:\n  - type: text\n  - value: |\n      file: docs/guide.md\n", "file:"},
		{"comment", "page:\n  - type: text\n  # file: docs/guide.md\n", "file:"},
		{"vars declaration", "vars:\n  env: ordinary\npage:\n  - type: text\n", "env:"},
		{"inherited child field", "base:\n  - type: tree\n  - child:\n      - type: markdown\npage:\n  - inherit: base\n  - child:\n      - file: docs/guide.md\n", "file:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if hover := analyzer.resolverHover(uri, test.source, definitionTestPosition(t, test.source, test.needle)); hover != nil {
				t.Fatalf("unexpected resolver hover: %#v", hover)
			}
		})
	}
}

func TestResolverHoverTemplateDataAndPackageConfiguration(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	source := "page:\n  - type: plugin\n  - data:\n      template: {file: cards/card.html}\n"
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	hover := analyzer.resolverHover(uri, source, definitionTestPosition(t, source, "file:"))
	if hover == nil || !strings.Contains(hover.Contents.Value, "**template.file**") {
		t.Fatalf("template resolver hover = %#v", hover)
	}
	config := "myconf:\n  title:\n    env: SITE_TITLE\n"
	configURI := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	hover = analyzer.resolverHover(configURI, config, definitionTestPosition(t, config, "env:"))
	if hover == nil || !strings.Contains(hover.Contents.Value, "environment variable") {
		t.Fatalf("configuration resolver hover = %#v", hover)
	}
}
