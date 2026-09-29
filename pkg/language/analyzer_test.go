package language

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestAnalyzerCompletesTypesAndTypeScopedFields(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	typeSource := "page:\n  - type: \n"
	types := analyzer.Completions("untitled:page", typeSource, Position{Line: 1, Character: 10}, nil)
	if !hasCompletion(types, "html") || !hasCompletion(types, "hypermedia") || !hasCompletion(types, "javascript") || !hasCompletion(types, "json") || !hasCompletion(types, "styles") {
		t.Fatalf("type completions = %#v", types)
	}
	if hasCompletion(types, "style") {
		t.Fatalf("display-only schema name leaked into type completions: %#v", types)
	}
	javascript, _ := completionByLabel(types, "javascript")
	if !strings.Contains(javascript.Detail, "Alias of") {
		t.Fatalf("javascript alias completion = %#v", javascript)
	}

	fieldSource := "page:\n  - type: html\n  - "
	fields := analyzer.Completions("untitled:page", fieldSource, Position{Line: 2, Character: 4}, nil)
	if !hasCompletion(fields, "value") || !hasCompletion(fields, "trimspace") || hasCompletion(fields, "route") {
		t.Fatalf("HTML field completions = %#v", fields)
	}
	quotedFields := analyzer.Completions("untitled:page", "page:\n  - type: \"html\"\n  - ", Position{Line: 2, Character: 4}, nil)
	if !hasCompletion(quotedFields, "value") || hasCompletion(quotedFields, "route") {
		t.Fatalf("quoted type field completions = %#v", quotedFields)
	}
	quotedInherited := "base:\n  - type: html\n  - value: base\npage:\n  - inherit: 'base'\n  - "
	quotedInheritedFields := analyzer.Completions("untitled:page", quotedInherited, Position{Line: 5, Character: 4}, nil)
	if !hasCompletion(quotedInheritedFields, "trimspace") || hasCompletion(quotedInheritedFields, "route") {
		t.Fatalf("quoted inherit field completions = %#v", quotedInheritedFields)
	}
	styleDiagnostics := analyzer.Diagnostics("untitled:page", "page:\n  - type: style\n", nil)
	if len(styleDiagnostics) != 1 || styleDiagnostics[0].Code != "component.unknown_type" {
		t.Fatalf("display-only style type diagnostics = %#v", styleDiagnostics)
	}
}

func TestAnalyzerValueCompletionsRequireYAMLSeparationSpace(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, source := range []string{
		"page:\n  - type:",
		"page:\n  - type:hy",
	} {
		items := analyzer.Completions("untitled:page", source, Position{Line: 1, Character: utf16Length(strings.TrimPrefix(source, "page:\n"))}, nil)
		if len(items) != 0 {
			t.Fatalf("malformed no-space value %q received completions: %#v", source, items)
		}
	}

	for _, source := range []string{
		"page:\n  - type: ",
		"page:\n  - type: hy",
	} {
		items := analyzer.Completions("untitled:page", source, Position{Line: 1, Character: utf16Length(strings.TrimPrefix(source, "page:\n"))}, nil)
		if !hasCompletion(items, "hypermedia") {
			t.Fatalf("space-separated value %q did not receive type completions: %#v", source, items)
		}
	}

	inheritSource := "base:\n  - type: html\n  - value: base\npage:\n  - inherit:ba"
	if items := analyzer.Completions("untitled:page", inheritSource, Position{Line: 4, Character: utf16Length("  - inherit:ba")}, nil); len(items) != 0 {
		t.Fatalf("no-space inherit value received completions: %#v", items)
	}
	inheritSource = strings.Replace(inheritSource, "inherit:ba", "inherit: ba", 1)
	if items := analyzer.Completions("untitled:page", inheritSource, Position{Line: 4, Character: utf16Length("  - inherit: ba")}, nil); !hasCompletion(items, "base") {
		t.Fatalf("space-separated inherit value did not receive completions: %#v", items)
	}
}

func TestAutomaticCompletionBoundaryRejectsUnrelatedSpaces(t *testing.T) {
	for _, test := range []struct {
		line string
		want bool
	}{
		{line: "  - type: ", want: true},
		{line: "  - type:   ", want: true},
		{line: "  - type:\t ", want: true},
		{line: "  - type:", want: false},
		{line: "  - type:hy ", want: false},
		{line: "  - type: html # note ", want: false},
		{line: "      # note: ", want: false},
		{line: "  - ", want: true},
	} {
		position := Position{Line: 0, Character: utf16Length(test.line)}
		if got := automaticCompletionBoundary(test.line, position); got != test.want {
			t.Errorf("automaticCompletionBoundary(%q) = %t, want %t", test.line, got, test.want)
		}
	}
}

func TestComponentEntryCompletionUsesYAMLOwner(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct {
		name   string
		source string
		line   int
		want   bool
	}{
		{
			name: "typed component",
			source: "page:\n" +
				"  - type: hypermedia\n" +
				"  - ",
			line: 2, want: true,
		},
		{
			name: "inherited component",
			source: "base:\n" +
				"  - type: hypermedia\n" +
				"page:\n" +
				"  - inherit: base\n" +
				"  - ",
			line: 4, want: true,
		},
		{
			name: "typed child component",
			source: "page:\n" +
				"  - type: hypermedia\n" +
				"  - body:\n" +
				"      - type: template\n" +
				"      - ",
			line: 4, want: true,
		},
		{
			name: "comments and blanks within component",
			source: "page:\n" +
				"  - type: hypermedia\n" +
				"\n" +
				"  # next field\n" +
				"  - ",
			line: 4, want: true,
		},
		{
			name: "ordinary list below dynamic values",
			source: "view:\n" +
				"  - type: template\n" +
				"  - values:\n" +
				"      items:\n" +
				"        - ",
			line: 4, want: false,
		},
		{
			name: "new child without a declared type",
			source: "page:\n" +
				"  - type: hypermedia\n" +
				"  - body:\n" +
				"      - ",
			line: 3, want: true,
		},
		{
			name: "untyped sibling after typed child",
			source: "page:\n" +
				"  - type: hypermedia\n" +
				"  - body:\n" +
				"      - type: html\n" +
				"      - value: Body\n" +
				"  - head:\n" +
				"      - ",
			line: 6, want: true,
		},
		{
			name: "separate root boundary",
			source: "first:\n" +
				"  - type: html\n" +
				"  - value: First\n" +
				"second:\n" +
				"  - ",
			line: 4, want: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			line := splitLines(test.source)[test.line]
			position := Position{Line: test.line, Character: utf16Length(line)}
			if got := len(analyzer.Completions("untitled:source", test.source, position, nil)) > 0; got != test.want {
				t.Fatalf("component completion = %t, want %t\n%s", got, test.want, test.source)
			}
		})
	}
}

func TestDocumentURIConversionSupportsWindowsDriveUNCAndEscapes(t *testing.T) {
	driveURI := "file:///C:/Users/A%20B/100%25.hyperbricks.yaml"
	drivePath := `C:\Users\A B\100%.hyperbricks.yaml`
	converted, err := uriToPathForOS(driveURI, "windows")
	if err != nil || converted != drivePath {
		t.Fatalf("Windows drive URI = %q, %v", converted, err)
	}
	if convertedURI := pathToURIForOS(drivePath, "windows"); convertedURI != driveURI {
		t.Fatalf("Windows drive path URI = %q", convertedURI)
	}

	uncURI := "file://server/share/HyperBricks/page.hyperbricks.yaml"
	uncPath := `\\server\share\HyperBricks\page.hyperbricks.yaml`
	converted, err = uriToPathForOS(uncURI, "windows")
	if err != nil || converted != uncPath {
		t.Fatalf("Windows UNC URI = %q, %v", converted, err)
	}
	if convertedURI := pathToURIForOS(uncPath, "windows"); convertedURI != uncURI {
		t.Fatalf("Windows UNC path URI = %q", convertedURI)
	}

	posixURI := "file:///tmp/A%20B/100%25.hyperbricks.yaml"
	converted, err = uriToPathForOS(posixURI, "linux")
	if err != nil || converted != "/tmp/A B/100%.hyperbricks.yaml" {
		t.Fatalf("POSIX URI = %q, %v", converted, err)
	}
	if _, err := uriToPathForOS(posixURI+"?token=secret", "linux"); err == nil {
		t.Fatal("file URI query was accepted")
	}
}

func TestAnalyzerCompletesOnlyReachableInheritanceTargetsAndInheritedFields(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	currentURI := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))
	baseURI := pathToURI(filepath.Join(sourceDir, "base.hyperbricks.yaml"))
	unreachableURI := pathToURI(filepath.Join(sourceDir, "unreachable.hyperbricks.yaml"))
	current := "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: \n"
	documents := map[string]string{
		currentURI:     current,
		baseURI:        "base:\n  - type: hypermedia\n  - card:\n      - type: html\n      - value: card\n",
		unreachableURI: "hidden:\n  - type: html\n  - value: hidden\n",
	}
	items := analyzer.Completions(currentURI, current, Position{Line: 2, Character: 13}, documents)
	if !hasCompletion(items, "base") || !hasCompletion(items, "base.card") {
		t.Fatalf("inherit completions = %#v", items)
	}
	if hasCompletion(items, "hidden") || hasCompletion(items, "page") {
		t.Fatalf("unreachable or self-cycling target leaked into inherit completions: %#v", items)
	}

	current = "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\n  - "
	documents[currentURI] = current
	fields := analyzer.Completions(currentURI, current, Position{Line: 3, Character: 4}, documents)
	if !hasCompletion(fields, "route") || hasCompletion(fields, "value") {
		t.Fatalf("inherited hypermedia field completions = %#v", fields)
	}
}

func TestAnalyzerCompletesOnlyParserValidInheritanceObjectPaths(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	source := `card:
  - type: template
  - values:
      button:
        - type: html
        - value: click
      groups:
        - item:
            - type: text
            - value: row
tree:
  - type: tree
  - branch:
      - type: tree
      - leaf:
          - type: html
          - value: nested
page:
  - type: tree
  - child:
      - inherit:` + " \n"
	items := analyzer.Completions("untitled:page", source, Position{Line: 20, Character: 17}, nil)
	for _, target := range []string{"card", "tree", "tree.branch", "tree.branch.leaf"} {
		if !hasCompletion(items, target) {
			t.Fatalf("inheritance target %q missing from %#v", target, items)
		}
	}
	for _, invalid := range []string{"card.values.button", "card.values.groups[0].item"} {
		if hasCompletion(items, invalid) {
			t.Fatalf("non-child object path %q leaked into %#v", invalid, items)
		}
	}
	for _, cycle := range []string{"page", "page.child"} {
		if hasCompletion(items, cycle) {
			t.Fatalf("cycle-forming target %q present in %#v", cycle, items)
		}
	}

	inherited := `card:
  - type: tree
  - button:
      - type: html
      - value: click
page:
  - inherit: card.button
  - `
	fields := analyzer.Completions("untitled:page", inherited, Position{Line: 7, Character: 4}, nil)
	if !hasCompletion(fields, "trimspace") || hasCompletion(fields, "route") {
		t.Fatalf("child-path inherited type fields = %#v", fields)
	}
}

func TestAnalyzerCompletesInheritedChildPathSegmentsWithoutPrefixDuplication(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	partialsDir := filepath.Join(sourceDir, "partials")
	if err := os.MkdirAll(partialsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	currentURI := pathToURI(filepath.Join(sourceDir, "app.hyperbricks.yaml"))
	siteURI := pathToURI(filepath.Join(partialsDir, "site.hyperbricks.yaml"))
	viewsURI := pathToURI(filepath.Join(partialsDir, "views.hyperbricks.yaml"))
	source := `imports:
  - partials/site.hyperbricks.yaml
  - partials/views.hyperbricks.yaml
about_page:
  - inherit: todo_page
  - body:
      - values:
          content:
            - inherit: todo_about
probe:
  - inherit: about_page.b
`
	documents := map[string]string{
		currentURI: source,
		siteURI: `todo_page:
  - type: hypermedia
  - body:
      - type: template
      - values:
          content:
            - type: tree
`,
		viewsURI: `todo_about:
  - type: html
  - value: About
`,
	}
	items := analyzer.Completions(currentURI, source, Position{Line: 10, Character: utf16Length("  - inherit: about_page.b")}, documents)
	body, ok := completionByLabel(items, "body")
	if !ok || body.InsertText != "body" || !strings.Contains(body.Detail, "about_page.body") {
		t.Fatalf("dotted child segment completion = %#v in %#v", body, items)
	}
	if hasCompletion(items, "about_page.body") {
		t.Fatalf("full path would duplicate typed prefix: %#v", items)
	}

	invalid := strings.Replace(source, "inherit: about_page.b", "inherit: about_page.body.values.c", 1)
	items = analyzer.Completions(currentURI, invalid, Position{Line: 10, Character: utf16Length("  - inherit: about_page.body.values.c")}, documents)
	if len(items) != 0 {
		t.Fatalf("template values leaked into inheritance object paths: %#v", items)
	}

	bare := source + "about_page.body.values.c\n"
	items = analyzer.Completions(currentURI, bare, Position{Line: 11, Character: utf16Length("about_page.body.values.c")}, documents)
	if len(items) != 0 {
		t.Fatalf("bare YAML text received reference completions: %#v", items)
	}
}

func TestAnalyzerValidatesPackageConfigWithRuntimeRules(t *testing.T) {
	module := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	for _, filename := range []string{"package.hyperbricks.yaml", "package.development.hyperbricks.yaml"} {
		t.Run(filename, func(t *testing.T) {
			uri := pathToURI(filepath.Join(module, filename))
			diagnostics := analyzer.Diagnostics(uri, "hyperbricks:\n  mode: live\n  live:\n    cache: -1s\n", nil)
			if len(diagnostics) != 1 || diagnostics[0].Code != "yaml.configuration" || !strings.Contains(diagnostics[0].Message, "live.cache") {
				t.Fatalf("strict package diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestAnalyzerRecognizesPackageConfigFilenameMarker(t *testing.T) {
	module := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{name: "default package", filename: "package.hyperbricks.yaml", want: true},
		{name: "named package profile", filename: "package.development.hyperbricks.yaml", want: true},
		{name: "embedded marker", filename: "local.package.development.hyperbricks.yaml", want: true},
		{name: "package without dot", filename: "package-development.hyperbricks.yaml", want: false},
		{name: "package directory only", filename: filepath.Join("package.profiles", "development.hyperbricks.yaml"), want: false},
		{name: "component source", filename: "development.hyperbricks.yaml", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uri := pathToURI(filepath.Join(module, test.filename))
			if got := analyzer.isPackageConfig(uri); got != test.want {
				t.Fatalf("isPackageConfig(%q) = %t, want %t", test.filename, got, test.want)
			}
		})
	}
}

func TestAnalyzerRejectsImportAndSymlinkEscapesButUsesOverlays(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.hyperbricks.yaml")
	if err := os.WriteFile(outside, []byte("outside:\n  - type: html\n  - value: no\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	currentPath := filepath.Join(sourceDir, "page.hyperbricks.yaml")
	currentURI := pathToURI(currentPath)
	escape := "imports: [../outside.hyperbricks.yaml]\npage:\n  - inherit: outside\n"
	diagnostics := analyzer.Diagnostics(currentURI, escape, map[string]string{currentURI: escape})
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "configured HyperBricks source directory") {
		t.Fatalf("relative import escape diagnostics = %#v", diagnostics)
	}

	link := filepath.Join(sourceDir, "linked.hyperbricks.yaml")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	symlinkSource := "imports: [linked.hyperbricks.yaml]\npage:\n  - inherit: outside\n"
	diagnostics = analyzer.Diagnostics(currentURI, symlinkSource, map[string]string{currentURI: symlinkSource})
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "resolves outside") {
		t.Fatalf("symlink import escape diagnostics = %#v", diagnostics)
	}

	baseURI := pathToURI(filepath.Join(sourceDir, "base.hyperbricks.yaml"))
	overlaySource := "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: \n"
	items := analyzer.Completions(currentURI, overlaySource, Position{Line: 2, Character: 13}, map[string]string{
		currentURI: overlaySource,
		baseURI:    "overlay_base:\n  - type: html\n  - value: pending\n",
	})
	if !hasCompletion(items, "overlay_base") {
		t.Fatalf("unsaved imported overlay was not used: %#v", items)
	}
}

func TestAnalyzerUsesConfiguredHyperBricksSourceDirectoryForImports(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "custom-source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := `hyperbricks:
  directories:
    hyperbricks:
      path:
        base: module
        path: custom-source
`
	if err := os.WriteFile(filepath.Join(root, "package.hyperbricks.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	currentURI := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))
	baseURI := pathToURI(filepath.Join(sourceDir, "base.hyperbricks.yaml"))
	current := "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: \n"
	items := analyzer.Completions(currentURI, current, Position{Line: 2, Character: 13}, map[string]string{
		currentURI: current,
		baseURI:    "custom_base:\n  - type: html\n  - value: pending\n",
	})
	if !hasCompletion(items, "custom_base") {
		t.Fatalf("configured source directory completions = %#v", items)
	}
}

func TestAnalyzerAnchorsImportedIssuesAtImportReference(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	currentPath := filepath.Join(sourceDir, "page.hyperbricks.yaml")
	basePath := filepath.Join(sourceDir, "base.hyperbricks.yaml")
	current := "imports: [base.hyperbricks.yaml]\npage:\n  - type: html\n  - value: ok\n"
	diagnostics := analyzer.Diagnostics(pathToURI(currentPath), current, map[string]string{
		pathToURI(currentPath): current,
		pathToURI(basePath):    "base:\n  - type: [\n",
	})
	if len(diagnostics) != 1 || diagnostics[0].Range.Start.Line != 0 || !strings.Contains(diagnostics[0].Message, "import base.hyperbricks.yaml") {
		t.Fatalf("import-anchored diagnostics = %#v", diagnostics)
	}
}

func TestAnalyzerUsesOwningTopLevelGraphForNestedSourceDiagnostics(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	spaceDir := filepath.Join(sourceDir, "spaces", "landing_source")
	if err := os.MkdirAll(spaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entryPath := filepath.Join(sourceDir, "02-landing.hyperbricks.yaml")
	indexPath := filepath.Join(spaceDir, "index.hyperbricks.yaml")
	spacePath := filepath.Join(spaceDir, "landing_en.hyperbricks.yaml")
	entry := "imports: [spaces/landing_source/index.hyperbricks.yaml]\nlanding_source:\n  - type: hypermedia\n"
	index := "imports: [landing_en.hyperbricks.yaml]\n"
	space := "landing_en:\n  - inherit: landing_source\n  - route: index\n"
	for path, content := range map[string]string{entryPath: entry, indexPath: index, spacePath: space} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	for name, path := range map[string]string{"nested index": indexPath, "nested Space": spacePath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if diagnostics := analyzer.Diagnostics(pathToURI(path), string(raw), nil); len(diagnostics) != 0 {
			t.Fatalf("%s diagnostics = %#v", name, diagnostics)
		}
	}

	spaceURI := pathToURI(spacePath)
	draft := space + "  - "
	fields := analyzer.Completions(spaceURI, draft, Position{Line: 3, Character: 4}, map[string]string{spaceURI: draft})
	if !hasCompletion(fields, "title") || hasCompletion(fields, "value") {
		t.Fatalf("nested inherited field completions = %#v", fields)
	}
	if hover := analyzer.Hover(spaceURI, space, Position{Line: 2, Character: 5}); hover == nil || !strings.Contains(hover.Contents.Value, "**route**") {
		t.Fatalf("nested inherited field hover = %#v", hover)
	}
	locations := analyzer.Definitions(spaceURI, space, Position{Line: 1, Character: 15})
	if len(locations) != 1 || locations[0].URI != pathToURI(entryPath) {
		t.Fatalf("nested inheritance definition = %#v", locations)
	}

	orphanPath := filepath.Join(sourceDir, "orphan", "landing_orphan.hyperbricks.yaml")
	orphan := "landing_orphan:\n  - inherit: landing_source\n"
	if err := os.MkdirAll(filepath.Dir(orphanPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanPath, []byte(orphan), 0o644); err != nil {
		t.Fatal(err)
	}
	orphanDiagnostics := analyzer.Diagnostics(pathToURI(orphanPath), orphan, nil)
	if len(orphanDiagnostics) != 1 || orphanDiagnostics[0].Code != "component.inheritance" {
		t.Fatalf("orphan nested source diagnostics = %#v", orphanDiagnostics)
	}
}

func TestAnalyzerCompletesResolversPathBasesAndTreeChildren(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	resolverSource := "page:\n  - type: html\n  - value:\n      "
	resolvers := analyzer.Completions("untitled:page", resolverSource, Position{Line: 3, Character: 6}, nil)
	for _, label := range []string{"var", "env", "config", "path", "file", "format"} {
		if !hasCompletion(resolvers, label) {
			t.Fatalf("missing %q resolver completion in %#v", label, resolvers)
		}
	}

	baseSource := "page:\n  - type: html\n  - value:\n      path:\n        base: "
	bases := analyzer.Completions("untitled:page", baseSource, Position{Line: 4, Character: 14}, nil)
	for _, label := range []string{"module", "resources", "templates", "hyperbricks", "render", "module_root", "root", "static"} {
		if !hasCompletion(bases, label) {
			t.Fatalf("missing %q path base completion in %#v", label, bases)
		}
	}

	childSource := "page:\n  - type: tree\n  - "
	children := analyzer.Completions("untitled:page", childSource, Position{Line: 2, Character: 4}, nil)
	if !hasCompletion(children, "body") {
		t.Fatalf("tree child completion = %#v", children)
	}
}

func TestAnalyzerAuthoringSlotsOverrideSameKeyFields(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	hypermedia := "page:\n  - type: hypermedia\n  - "
	items := analyzer.Completions("untitled:page", hypermedia, Position{Line: 2, Character: 4}, nil)
	headCount := 0
	var head CompletionItem
	for _, item := range items {
		if item.Label == "head" {
			headCount++
			head = item
		}
	}
	if headCount != 1 || !strings.Contains(head.InsertText, "- type: ${1:head}") {
		t.Fatalf("hypermedia head slot completion = %#v (all %#v)", head, items)
	}
	if !hasCompletion(items, "body") {
		t.Fatalf("hypermedia default body slot missing from %#v", items)
	}

	authoredHead := "page:\n  - type: hypermedia\n  - head:\n      - type: head\n  - "
	items = analyzer.Completions("untitled:page", authoredHead, Position{Line: 4, Character: 4}, nil)
	if hasCompletion(items, "head") {
		t.Fatalf("authored head slot was offered again: %#v", items)
	}
	if !hasCompletion(items, "body") {
		t.Fatalf("authored head removed the default body slot: %#v", items)
	}

	template := "view:\n  - type: template\n  - "
	items = analyzer.Completions("untitled:view", template, Position{Line: 2, Character: 4}, nil)
	values, ok := completionByLabel(items, "values")
	if !ok || values.Kind != CompletionItemKindField {
		t.Fatalf("template dynamic values field completion = %#v (all %#v)", values, items)
	}
}

func TestAnalyzerBuildsNestedFieldSnippetsFromSchemaPaths(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	source := "page:\n  - type: hypermedia\n  - "
	items := analyzer.Completions("untitled:page", source, Position{Line: 2, Character: 4}, nil)
	response, ok := completionByLabel(items, "response")
	if !ok {
		t.Fatalf("response completion missing from %#v", items)
	}
	want := "response:\n      headers:\n        ${1:key}: ${2:value}\n      status: ${3:0}"
	if response.InsertText != want {
		t.Fatalf("response snippet:\n%s\nwant:\n%s", response.InsertText, want)
	}
	if strings.Contains(response.InsertText, "\n      ${1:key}:") {
		t.Fatalf("response snippet inserted an unsupported generic response key: %q", response.InsertText)
	}

	guard, ok := completionByLabel(items, "guard")
	if !ok || !strings.Contains(guard.InsertText, "\n      auth:\n        cookie:") || !strings.Contains(guard.InsertText, "\n      on_forbidden:\n") {
		t.Fatalf("deep guard snippet is not schema-backed: %#v", guard)
	}
}

func TestAnalyzerMaterializesResolversWithSelectedModulePathMarkers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hyperbricks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "greeting.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	source := "page:\n  - type: html\n  - value:\n      file:\n        base: resources\n        path: greeting.txt\n"
	if diagnostics := analyzer.Diagnostics(uri, source, map[string]string{uri: source}); len(diagnostics) != 0 {
		t.Fatalf("module path marker diagnostics = %#v", diagnostics)
	}
}

func TestAnalyzerResolvesNamedModuleAndPackageDirectoryPaths(t *testing.T) {
	workspace := t.TempDir()
	module := filepath.Join(workspace, "modules", "demo")
	if err := os.MkdirAll(filepath.Join(module, "custom-views", "cards"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(module, "assets", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "custom-views", "cards", "hero.html"), []byte("hero"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "assets", "data", "items.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := `hyperbricks:
  directories:
    templates:
      path:
        base: module
        path: custom-views
    resources:
      path:
        base: module
        path: assets
`
	if err := os.WriteFile(filepath.Join(module, "package.hyperbricks.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: workspace, Module: "demo"})
	if analyzer.moduleRoot() != module {
		t.Fatalf("module root = %q, want %q", analyzer.moduleRoot(), module)
	}
	templateItems := analyzer.Completions("untitled:page", "page:\n  - type: template\n  - template: ", Position{Line: 2, Character: 14}, nil)
	if !hasCompletion(templateItems, "cards/hero.html") {
		t.Fatalf("template completions = %#v", templateItems)
	}
	resourceItems := analyzer.Completions("untitled:page", "page:\n  - type: json_render\n  - file: ", Position{Line: 2, Character: 10}, nil)
	if !hasCompletion(resourceItems, "data/items.json") {
		t.Fatalf("resource completions = %#v", resourceItems)
	}
}

func TestAnalyzerPathSuggestionsMaterializeForNativeFieldShapes(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"hyperbricks", "resources/gallery", "resources/docs", "templates", "static/css"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"resources/data.json":     `{}`,
		"resources/styles.css":    `body {}`,
		"resources/hero.jpg":      `image`,
		"resources/gallery/a.jpg": `gallery image`,
		"resources/docs/read.md":  `# Read me`,
		"resources/app.ts":        `export const app = true`,
		"resources/script.js":     `function main() { return "ok"; }`,
		"templates/view.html":     `<p>{{.Data}}</p>`,
		"static/css/site.css":     `body { color: green; }`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	tests := []struct {
		typeName string
		key      string
		label    string
		want     string
	}{
		{typeName: "json_render", key: "file", label: "data.json", want: filepath.Join(root, "resources", "data.json")},
		{typeName: "styles", key: "file", label: "styles.css", want: filepath.Join(root, "resources", "styles.css")},
		{typeName: "image", key: "src", label: "hero.jpg", want: filepath.Join(root, "resources", "hero.jpg")},
		{typeName: "images", key: "directory", label: "gallery", want: filepath.Join(root, "resources", "gallery")},
		{typeName: "esbuild", key: "entry", label: "app.ts", want: filepath.Join(root, "resources", "app.ts")},
		{typeName: "goja_render", key: "script", label: "script.js", want: files["resources/script.js"]},
		{typeName: "markdown", key: "file", label: "docs/read.md", want: "docs/read.md"},
		{typeName: "template", key: "template", label: "view.html", want: "view.html"},
	}
	for _, test := range tests {
		t.Run(test.typeName+"_"+test.key, func(t *testing.T) {
			source := fmt.Sprintf("item:\n  - type: %s\n  - %s: ", test.typeName, test.key)
			items := analyzer.Completions(uri, source, Position{Line: 2, Character: utf16Length("  - " + test.key + ": ")}, nil)
			item, ok := completionByLabel(items, test.label)
			if !ok || item.InsertText == "" || item.InsertText == test.label {
				t.Fatalf("path completion %q = %#v in %#v", test.label, item, items)
			}
			materializedSource := []byte(source + item.InsertText + "\n")
			result, err := yamlparser.ProcessBytes(materializedSource, analyzer.sourceParserOptions())
			if err != nil {
				t.Fatalf("materialize suggestion %q: %v\n%s", item.InsertText, err, materializedSource)
			}
			object, _ := result.Materialized["item"].(map[string]interface{})
			if got := fmt.Sprint(object[test.key]); got != test.want {
				t.Fatalf("materialized %s.%s = %q, want %q (insert %q)", test.typeName, test.key, got, test.want, item.InsertText)
			}
		})
	}
	markdownSource := "item:\n  - type: markdown\n  - file: "
	markdownItems := analyzer.Completions(uri, markdownSource, Position{Line: 2, Character: utf16Length("  - file: ")}, nil)
	markdown, ok := completionByLabel(markdownItems, "docs/read.md")
	if !ok || markdown.InsertText != `"docs/read.md"` || hasCompletion(markdownItems, "data.json") {
		t.Fatalf("markdown resource completions = %#v", markdownItems)
	}

	// URL-valued fields have no schema-owned filesystem base. Static files must
	// not be suggested as raw or absolute values for them.
	for _, test := range []struct{ typeName, key string }{{"css", "link"}, {"hypermedia", "favicon"}} {
		source := fmt.Sprintf("item:\n  - type: %s\n  - %s: ", test.typeName, test.key)
		for _, item := range analyzer.Completions(uri, source, Position{Line: 2, Character: utf16Length("  - " + test.key + ": ")}, nil) {
			if item.Kind == CompletionItemKindFile {
				t.Fatalf("URL field %s.%s received filesystem completion: %#v", test.typeName, test.key, item)
			}
		}
	}
}

func TestAnalyzerPluginBoundaryAndTypeAliasNormalization(t *testing.T) {
	workspace := t.TempDir()
	module := filepath.Join(workspace, "modules", "default")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "hyperbricks:\n  plugins:\n    enabled: [CustomPlugin@1.0.0]\n"
	if err := os.WriteFile(filepath.Join(module, "package.hyperbricks.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: workspace, Module: "default"})
	if normalizeTypeName("<API-RENDER>") != "api_render" {
		t.Fatal("type aliases do not use parser-compatible dash normalization")
	}
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	diagnostics := analyzer.Diagnostics(uri, "widget:\n  - type: custom-widget\n", map[string]string{})
	if len(diagnostics) != 1 || diagnostics[0].Code != "component.unknown_type" || diagnostics[0].Severity != DiagnosticSeverityWarning {
		t.Fatalf("plugin-owned diagnostic boundary = %#v", diagnostics)
	}
}

func TestAnalyzerHoverUsesInheritedOverlayTypeAndNestedSchemaPaths(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	pageURI := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))
	baseURI := pathToURI(filepath.Join(sourceDir, "base.hyperbricks.yaml"))
	page := "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\n  - route: inherited\n"
	hover := analyzer.Hover(pageURI, page, Position{Line: 3, Character: 5}, map[string]string{
		pageURI: page,
		baseURI: "base:\n  - type: hypermedia\n",
	})
	if hover == nil || !strings.Contains(hover.Contents.Value, "**route**") {
		t.Fatalf("inherited overlay hover = %#v", hover)
	}

	nested := `page:
  - type: hypermedia
  - response:
      status: 201
  - guard:
      auth:
        cookie: session
`
	hover = analyzer.Hover("untitled:page", nested, Position{Line: 3, Character: 8})
	if hover == nil || !strings.Contains(hover.Contents.Value, "**response.status**") || !strings.Contains(hover.Contents.Value, "Browser HTTP status") {
		t.Fatalf("nested response hover = %#v", hover)
	}
	hover = analyzer.Hover("untitled:page", nested, Position{Line: 6, Character: 10})
	if hover == nil || !strings.Contains(hover.Contents.Value, "**guard.auth.cookie**") {
		t.Fatalf("deep guard hover = %#v", hover)
	}
}

func TestAnalyzerHoverSuppressesDocumentationIncludeMarkers(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	source := "page:\n  - type: html\n  - value: hello\n"
	hover := analyzer.Hover("untitled:page", source, Position{Line: 2, Character: 5})
	if hover == nil || !strings.Contains(hover.Contents.Value, "**value**") {
		t.Fatalf("HTML value hover = %#v", hover)
	}
	if strings.Contains(hover.Contents.Value, "{!{") || strings.Contains(hover.Contents.Value, "Example:") {
		t.Fatalf("HTML value hover exposed an internal documentation marker: %#v", hover)
	}
	if got := editorHoverExample("literal authoring example"); got != "literal authoring example" {
		t.Fatalf("plain editor-ready example = %q", got)
	}
}

func TestAnalyzerYAMLNodeRangesUseUTF16Columns(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	source := `page: [{value: "😀"}, {type: html}, {trimspace: true}]`
	typeByte := strings.Index(source, "html")
	typeStart := utf16Length(source[:typeByte])
	hover := analyzer.Hover("untitled:page", source, Position{Line: 0, Character: typeStart})
	if hover == nil || hover.Range == nil || hover.Range.Start.Character != typeStart {
		t.Fatalf("type hover UTF-16 range = %#v, want start %d", hover, typeStart)
	}

	keyByte := strings.Index(source, "trimspace")
	keyStart := utf16Length(source[:keyByte])
	hover = analyzer.Hover("untitled:page", source, Position{Line: 0, Character: keyStart})
	if hover == nil || hover.Range == nil || hover.Range.Start.Character != keyStart {
		t.Fatalf("field hover UTF-16 range = %#v, want start %d", hover, keyStart)
	}

	imports := `imports: ["😀.hyperbricks.yaml", base.hyperbricks.yaml]`
	importByte := strings.Index(imports, "base.hyperbricks.yaml")
	importStart := utf16Length(imports[:importByte])
	got := importReferenceRange(imports, filepath.Join(t.TempDir(), "base.hyperbricks.yaml"))
	if got.Start.Character != importStart {
		t.Fatalf("import UTF-16 range = %#v, want start %d", got, importStart)
	}
}

func hasCompletion(items []CompletionItem, label string) bool {
	_, ok := completionByLabel(items, label)
	return ok
}

func completionByLabel(items []CompletionItem, label string) (CompletionItem, bool) {
	for _, item := range items {
		if item.Label == label {
			return item, true
		}
	}
	return CompletionItem{}, false
}
