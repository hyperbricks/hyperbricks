package language

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func completionTestPosition(t *testing.T, marked string) (string, Position) {
	t.Helper()
	before, after, ok := strings.Cut(marked, "«cursor»")
	if !ok {
		t.Fatal("missing cursor marker")
	}
	lines := splitLines(before)
	return before + after, Position{Line: len(lines) - 1, Character: utf16Length(lines[len(lines)-1])}
}

func TestYAMLCompletionOwnsComponentAndResolverContext(t *testing.T) {
	a := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct {
		name, source string
		want, absent []string
	}{
		{"nested schema", "page:\n  - type: hypermedia\n  - response:\n      «cursor»", []string{"status", "headers"}, []string{"body", "file", "value"}},
		{"nested existing schema", "page:\n  - type: hypermedia\n  - response:\n      status: 201\n      «cursor»", []string{"headers"}, []string{"status", "body"}},
		{"flow resolver options", "page:\n  - type: text\n  - value: {file: {«cursor»}}", []string{"base", "path", "parts"}, []string{"value", "route", "file"}},
		{"unfinished flow options", "page:\n  - type: text\n  - value: {file: {base: resources, p«cursor»", []string{"path", "parts"}, []string{"base", "value"}},
		{"flow base", "page:\n  - type: text\n  - value: {file: {base: «cursor»}}", []string{"resources", "templates"}, []string{"value"}},
		{"block resolver options", "page:\n  - type: text\n  - value:\n      file:\n        «cursor»", []string{"base", "path", "parts"}, []string{"value", "route"}},
		{"scalar snippets", "page:\n  - type: text\n  - value: «cursor»", []string{"file", "path", "env", "var", "format", "config"}, nil},
		{"flow snippets", "page:\n  - type: text\n  - value: {«cursor»}", []string{"file", "path", "env", "var", "format", "config"}, nil},
		{"template options", "page:\n  - type: template\n  - template:\n      «cursor»", []string{"file"}, []string{"env", "path"}},
		{"bool value", "page:\n  - type: hypermedia\n  - nocache: «cursor»", []string{"true", "false"}, []string{"route"}},
		{"new component", "page:\n  - «cursor»", []string{"type", "inherit"}, []string{"value", "route"}},
		{"inherited child", "base:\n  - type: hypermedia\n  - body:\n      - type: template\npage:\n  - inherit: base\n  - body:\n      - «cursor»", []string{"template", "values"}, []string{"route", "doctype"}},
		{"comment", "page:\n  - type: html # «cursor»", nil, []string{"html", "value", "file"}},
		{"literal", "page:\n  - type: html\n  - value: |\n      type: «cursor»", nil, []string{"html", "value", "file"}},
		{"ordinary list", "page:\n  - type: template\n  - values:\n      items:\n        - «cursor»", nil, []string{"route", "template", "value", "inherit"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, pos := completionTestPosition(t, test.source)
			items := a.Completions("untitled:page", source, pos, nil)
			for _, label := range test.want {
				if !hasCompletion(items, label) {
					t.Errorf("missing %q from %#v", label, items)
				}
			}
			for _, label := range test.absent {
				if hasCompletion(items, label) {
					t.Errorf("unexpected %q in %#v", label, items)
				}
			}
		})
	}
}

func TestYAMLCompletionResourcePathsReplaceWholeScalar(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"resources/docs/llms.md", "templates/cards/card.html", "hyperbricks/partials/site.hyperbricks.yaml"} {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("example"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	a := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks/app.hyperbricks.yaml"))
	for _, test := range []struct{ name, source, label, want string }{
		{"flow file", "page:\n  - type: text\n  - value: {file: {base: resources, path: docs/ll«cursor»}}", "docs/llms.md", "page:\n  - type: text\n  - value: {file: {base: resources, path: \"docs/llms.md\"}}"},
		{"quoted file", "page:\n  - type: text\n  - value: {file: {base: resources, path: 'docs/ll«cursor»'}}", "docs/llms.md", "page:\n  - type: text\n  - value: {file: {base: resources, path: \"docs/llms.md\"}}"},
		{"template", "page:\n  - type: template\n  - template:\n      file: cards/ca«cursor»", "cards/card.html", "page:\n  - type: template\n  - template:\n      file: \"cards/card.html\""},
		{"import", "imports:\n  - partials/si«cursor»", "partials/site.hyperbricks.yaml", "imports:\n  - \"partials/site.hyperbricks.yaml\""},
		{"unicode path prefix", "😀page:\n  - type: text\n  - value: {file: {base: resources, path: docs/ll«cursor»}}", "docs/llms.md", "😀page:\n  - type: text\n  - value: {file: {base: resources, path: \"docs/llms.md\"}}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, pos := completionTestPosition(t, test.source)
			item, ok := completionByLabel(a.Completions(uri, source, pos, nil), test.label)
			if !ok || item.TextEdit == nil {
				t.Fatalf("missing %s: %#v", test.label, item)
			}
			lines := splitLines(source)
			line := lines[item.TextEdit.Range.Start.Line]
			lines[item.TextEdit.Range.Start.Line] = line[:byteIndexForUTF16(line, item.TextEdit.Range.Start.Character)] + item.TextEdit.NewText + line[byteIndexForUTF16(line, item.TextEdit.Range.End.Character):]
			if actual := strings.Join(lines, "\n"); actual != test.want {
				t.Fatalf("applied edit:\n%s\nwant:\n%s", actual, test.want)
			}
			if _, err := decodeYAMLDocument(strings.Join(lines, "\n")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestYAMLCompletionVariableAndConfigPaths(t *testing.T) {
	root := t.TempDir()
	a := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks/app.hyperbricks.yaml"))
	configURI := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	for _, test := range []struct{ source, label string }{
		{"vars:\n  site:\n    title: hello\npage:\n  - type: text\n  - value: {var: site.«cursor»}", "site.title"},
		{"page:\n  - type: text\n  - value: {config: myconf.site.«cursor»}", "myconf.site.title"},
	} {
		source, pos := completionTestPosition(t, test.source)
		items := a.Completions(uri, source, pos, map[string]string{configURI: "myconf:\n  site:\n    title: PRIVATE_TEST_VALUE\n"})
		item, ok := completionByLabel(items, test.label)
		if !ok {
			t.Fatalf("missing %q: %#v", test.label, items)
		}
		if strings.Contains(item.Detail, "PRIVATE_TEST_VALUE") {
			t.Fatal("leaked config value")
		}
	}
}
