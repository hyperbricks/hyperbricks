package language

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func reviewCompletionSource(t *testing.T, marked string) (string, Position) {
	t.Helper()
	position := definitionTestPosition(t, marked, "§")
	return strings.Replace(marked, "§", "", 1), position
}

func applyReviewCompletion(t *testing.T, text string, item CompletionItem) string {
	t.Helper()
	if item.TextEdit == nil {
		t.Fatalf("completion %q lacks an explicit edit", item.Label)
	}
	lines := splitLines(text)
	edit := item.TextEdit
	if edit.Range.Start.Line != edit.Range.End.Line {
		t.Fatalf("unexpected multiline replacement: %#v", edit.Range)
	}
	line := lines[edit.Range.Start.Line]
	start := byteIndexForUTF16(line, edit.Range.Start.Character)
	end := byteIndexForUTF16(line, edit.Range.End.Character)
	lines[edit.Range.Start.Line] = line[:start] + edit.NewText + line[end:]
	return strings.Join(lines, "\n")
}

func TestCompletionReviewQuotedDottedInheritanceEdits(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	for _, quoted := range []string{`"base.ch§"`, `'base.ch§'`, `"b😀se.ch§"`} {
		t.Run(quoted, func(t *testing.T) {
			baseName := "base"
			if strings.Contains(quoted, "😀") {
				baseName = "b😀se"
			}
			source, position := reviewCompletionSource(t, baseName+":\n  - type: tree\n  - child:\n      - type: html\ncopy:\n  - inherit: "+quoted+"\n")
			items := analyzer.Completions(uri, source, position, nil)
			item, ok := completionByLabel(items, "child")
			if !ok {
				t.Fatalf("child completion absent: %#v", items)
			}
			completed := applyReviewCompletion(t, source, item)
			document, err := yamlparser.ParseBytesWithOptions([]byte(completed), yamlparser.ParseOptions{AllowUnknownTypes: true})
			if err != nil {
				t.Fatalf("accepted completion corrupted YAML: %v\n%s\nedit: %#v", err, completed, item.TextEdit)
			}
			if got := document.Roots[1].Inherit; got != baseName+".child" {
				t.Fatalf("inherit target = %q, want %q; edit: %#v", got, baseName+".child", item.TextEdit)
			}
		})
	}
}

func TestCompletionReviewResolverPartsAreValuesNotOptionKeys(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, marked := range []string{
		"page:\n  - type: text\n  - value: {file: {base: resources, parts: [docs, §]}}\n",
		"page:\n  - type: text\n  - value:\n      file:\n        base: resources\n        parts:\n          - docs\n          - §\n",
		"page:\n  - type: text\n  - value: {file: {parts: [docs, §]}}\n",
		"page:\n  - type: text\n  - value:\n      file:\n        parts:\n          - docs\n          - §\n",
	} {
		source, position := reviewCompletionSource(t, marked)
		items := analyzer.Completions("untitled:page", source, position, nil)
		for _, key := range []string{"base", "path", "parts", "title", "route"} {
			if hasCompletion(items, key) {
				t.Errorf("path segment received option/component key %q: %#v", key, items)
			}
		}
	}
}

func TestCompletionReviewConfigPathsExcludeResolverInternalsAndVars(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hyperbricks"), 0o755); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	configURI := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	config := "vars:\n  secret_name: PRIVATE_TITLE\nmyconf:\n  title: {env: PRIVATE_TITLE}\n  literal: Welcome\n"
	source, position := reviewCompletionSource(t, "page:\n  - type: text\n  - value: {config: §}\n")
	items := analyzer.Completions(uri, source, position, map[string]string{configURI: config})
	if !hasCompletion(items, "myconf.title") || !hasCompletion(items, "myconf.literal") {
		t.Fatalf("real config paths missing: %#v", items)
	}
	for _, key := range []string{"vars", "vars.secret_name", "myconf.title.env"} {
		if hasCompletion(items, key) {
			t.Errorf("non-runtime configuration path %q offered: %#v", key, items)
		}
	}
}

func TestCompletionReviewInheritedOverlayAndSiblingOwnership(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct {
		name, marked string
		want         []string
		absent       []string
	}{
		{
			name:   "inherited type-less child overlay",
			marked: "base:\n  - type: hypermedia\n  - body:\n      - type: template\npage:\n  - inherit: base\n  - body:\n      - §\n",
			want:   []string{"values", "template"}, absent: []string{"route", "title"},
		},
		{
			name:   "new untyped child does not borrow sibling",
			marked: "page:\n  - type: tree\n  - previous:\n      - type: hypermedia\n  - next:\n      - §\n",
			want:   []string{"type", "inherit"}, absent: []string{"route", "title", "beautify"},
		},
		{
			name:   "ordinary string list does not borrow parent fields",
			marked: "page:\n  - type: hypermedia\n  - querykeys:\n      - q\n      - §\n",
			absent: []string{"route", "title", "querykeys", "body", "type", "inherit"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, position := reviewCompletionSource(t, test.marked)
			items := analyzer.Completions("untitled:page", source, position, nil)
			for _, key := range test.want {
				if !hasCompletion(items, key) {
					t.Errorf("missing %q in %#v", key, items)
				}
			}
			for _, key := range test.absent {
				if hasCompletion(items, key) {
					t.Errorf("unexpected %q in %#v", key, items)
				}
			}
		})
	}
}

func TestCompletionReviewFileEditKeepsFlowSiblingsCommentsAndUTF16(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "resources", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "docs", "😀guide.md"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	for _, marked := range []string{
		"page:\n  - type: text\n  - value: {file: {base: resources, path: 'docs/😀g§uide.md'}} # keep\n",
		"page:\n  - type: text\n  - value:\n      file:\n        base: resources\n        path: \"docs/😀g§uide.md\" # keep\n",
	} {
		source, position := reviewCompletionSource(t, marked)
		items := analyzer.Completions("untitled:page", source, position, nil)
		item, ok := completionByLabel(items, "docs/😀guide.md")
		if !ok {
			t.Fatalf("file completion missing: %#v", items)
		}
		completed := applyReviewCompletion(t, source, item)
		if !strings.Contains(completed, "# keep") || !strings.Contains(completed, "base: resources") {
			t.Fatalf("completion removed unrelated YAML: %s", completed)
		}
		if _, err := decodeYAMLDocument(completed); err != nil {
			t.Fatalf("file completion corrupted YAML: %v\n%s", err, completed)
		}
	}
}

func TestCompletionReviewFlowSchemaMapSnippetProducesValidYAML(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	source, position := reviewCompletionSource(t, "page:\n  - type: hypermedia\n  - response: {he§}\n")
	items := analyzer.Completions("untitled:page", source, position, nil)
	item, ok := completionByLabel(items, "headers")
	if !ok {
		t.Fatalf("headers completion missing: %#v", items)
	}
	if item.TextEdit == nil {
		t.Fatal("headers completion lacks text edit")
	}
	item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+:([^}]*)\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
	item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+\|([^,|]+)[^}]*\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
	completed := applyReviewCompletion(t, source, item)
	if _, err := decodeYAMLDocument(completed); err != nil {
		t.Fatalf("flow schema completion produces invalid YAML: %v\n%s", err, completed)
	}
}

func TestCompletionReviewExistingDataMappingIsNotResolverWrapper(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	source, position := reviewCompletionSource(t, "page:\n  - type: plugin\n  - data: {caption: ordinary, §}\n")
	items := analyzer.Completions("untitled:page", source, position, nil)
	for _, key := range []string{"file", "path", "env", "var", "config", "format"} {
		if hasCompletion(items, key) {
			t.Errorf("offered %q as a resolver alongside ordinary data keys; accepting it cannot resolve that mapping", key)
		}
	}
}

func TestCompletionReviewMultilineFlowMappings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "resources", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "docs", "guide.md"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	for _, test := range []struct{ name, marked, label string }{
		{"schema nested map", "page:\n  - type: hypermedia\n  - response: {\n      he§\n    }\n", "headers"},
		{"resolver wrapper", "page:\n  - type: text\n  - value: {\n      fi§\n    }\n", "file"},
		{"resolver option", "page:\n  - type: text\n  - value: {file: {\n      base: resources,\n      pa§\n    }}\n", "path"},
		{"path before closers", "page:\n  - type: text\n  - value: {file: {\n      base: resources,\n      path: docs/g§}} # keep\n", "docs/guide.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, position := reviewCompletionSource(t, test.marked)
			items := analyzer.Completions("untitled:page", source, position, nil)
			item, ok := completionByLabel(items, test.label)
			if !ok {
				t.Fatalf("missing %q completion: %#v", test.label, items)
			}
			if item.TextEdit == nil {
				t.Fatal("completion lacks text edit")
			}
			item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+:([^}]*)\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
			item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+\|([^,|]+)[^}]*\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
			completed := applyReviewCompletion(t, source, item)
			if _, err := decodeYAMLDocument(completed); err != nil {
				t.Fatalf("multiline flow completion corrupted YAML: %v\n%s", err, completed)
			}
			if strings.Contains(source, "# keep") && !strings.Contains(completed, "# keep") {
				t.Fatalf("completion discarded trailing comment:\n%s", completed)
			}
		})
	}
}

func TestCompletionReviewAuthoredScalarsDoNotReceiveStructuralEdits(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, marked := range []string{
		"page:\n  - type: text\n  - value: ordinary {fi§} text\n",
		"page:\n  - type: text\n  - value: 'ordinary {fi§} text'\n",
		"page:\n  - type: text\n  - value: ordinary\n      fi§rst continuation\n      last continuation\n",
		"page:\n  - type: text\n  - value: or§dinary\n      continuation\n",
		"page:\n  - type: text\n  - value: \"ordinary\n      fi§rst continuation\n      last continuation\"\n",
		"page:\n  - type: text\n  - value: |\n      fi§rst block line\n",
		"page:\n  - type: text\n  - value: >-\n      fi§rst block line\n",
	} {
		source, position := reviewCompletionSource(t, marked)
		if _, err := decodeYAMLDocument(source); err != nil {
			t.Fatalf("invalid scalar test input: %v\n%s", err, source)
		}
		if items := analyzer.Completions("untitled:page", source, position, nil); len(items) != 0 {
			t.Errorf("authored scalar received structural suggestions: %#v\n%s", items, source)
		}
	}
}

func TestCompletionReviewNestedDefaultAndArgumentResolvers(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct{ name, value, label string }{
		{"block argument", "\n      format: '%s'\n      args:\n        - var: local.t§", "local.title"},
		{"flow argument", "{format: '%s', args: [{var: local.t§}]}", "local.title"},
		{"block fallback", "\n      env:\n        name: EDITOR_TITLE\n        default:\n          var: local.t§", "local.title"},
		{"flow fallback", "{env: {name: EDITOR_TITLE, default: {var: local.t§}}}", "local.title"},
		{"empty scalar fallback", "{env: {name: EDITOR_TITLE, default: §}}", "var"},
		{"empty flow argument", "{format: '%s', args: [§]}", "var"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, position := reviewCompletionSource(t, "vars:\n  local:\n    title: Title\npage:\n  - type: text\n  - value: "+test.value+"\n")
			items := analyzer.Completions("untitled:page", source, position, nil)
			item, ok := completionByLabel(items, test.label)
			if !ok {
				t.Fatalf("missing %q in nested resolver context: %#v", test.label, items)
			}
			if item.TextEdit == nil {
				t.Fatal("nested resolver completion lacks text edit")
			}
			item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+:([^}]*)\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
			completed := applyReviewCompletion(t, source, item)
			document, err := decodeYAMLDocument(completed)
			if err != nil {
				t.Fatalf("nested resolver acceptance corrupted YAML: %v\n%s", err, completed)
			}
			foundVar := false
			visitSourceResolvers(completed, document, false, func(resolver sourceResolver) {
				if resolver.kind == "var" {
					foundVar = true
				}
			})
			if !foundVar {
				t.Fatalf("accepted completion did not create a var resolver:\n%s", completed)
			}
		})
	}
}

func TestCompletionReviewPackageVariablesKeepOrdinaryMappingShape(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	source, position := reviewCompletionSource(t, "vars:\n  app:\n    title: Site\nmyconf:\n  title: {var: app.t§}\n")
	items := analyzer.Completions(uri, source, position, nil)
	if !hasCompletion(items, "app.title") {
		t.Fatalf("package variable missing: %#v", items)
	}
	source, position = reviewCompletionSource(t, "vars:\n  §\nmyconf:\n  title: Site\n")
	items = analyzer.Completions(uri, source, position, nil)
	for _, key := range []string{"type", "inherit", "file", "env", "path"} {
		if hasCompletion(items, key) {
			t.Errorf("vars declaration offered reserved instruction %q: %#v", key, items)
		}
	}
}
