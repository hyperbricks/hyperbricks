package language

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinitionsResolveImportsAndInheritanceThroughOverlayGraph(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	cardDir := filepath.Join(sourceDir, "partials", "cards")
	if err := os.MkdirAll(cardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pagePath := filepath.Join(sourceDir, "main.hyperbricks.yaml")
	sitePath := filepath.Join(sourceDir, "partials", "site.hyperbricks.yaml")
	cardPath := filepath.Join(cardDir, "card.hyperbricks.yaml")
	if err := os.WriteFile(sitePath, []byte("disk_site:\n  - type: tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cardPath, []byte("disk_card:\n  - type: html\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	page := `imports: [partials/site.hyperbricks.yaml]
page:
  - type: hypermedia
  - main:
      - inherit: site_layout
  - featured:
      - inherit: shared_card
`
	site := `imports: [cards/card.hyperbricks.yaml]
site_layout:
  - type: tree
`
	card := `# unsaved imported overlay
shared_card:
  - type: html
`
	pageURI, siteURI, cardURI := pathToURI(pagePath), pathToURI(sitePath), pathToURI(cardPath)
	documents := map[string]string{pageURI: page, siteURI: site, cardURI: card}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})

	assertDefinitionLocation(t,
		analyzer.Definitions(pageURI, page, definitionTestPosition(t, page, "partials/site.hyperbricks.yaml"), documents),
		siteURI, Position{})
	assertDefinitionLocation(t,
		analyzer.Definitions(pageURI, page, definitionTestPosition(t, page, "site_layout"), documents),
		siteURI, Position{Line: 1, Character: 0})
	assertDefinitionLocation(t,
		analyzer.Definitions(pageURI, page, definitionTestPosition(t, page, "shared_card"), documents),
		cardURI, Position{Line: 1, Character: 0})
	assertDefinitionLocation(t,
		analyzer.Definitions(siteURI, site, definitionTestPosition(t, site, "cards/card.hyperbricks.yaml"), documents),
		cardURI, Position{})

	unreachablePath := filepath.Join(sourceDir, "unreachable.hyperbricks.yaml")
	unreachableURI := pathToURI(unreachablePath)
	documents[unreachableURI] = "hidden:\n  - type: html\n"
	unresolved := strings.Replace(page, "site_layout", "hidden", 1)
	if locations := analyzer.Definitions(pageURI, unresolved, definitionTestPosition(t, unresolved, "hidden"), documents); len(locations) != 0 {
		t.Fatalf("unreachable definition leaked into result: %#v", locations)
	}
}

func TestDefinitionsResolveConfiguredTemplateAndResourceFiles(t *testing.T) {
	workspace := t.TempDir()
	module := filepath.Join(workspace, "modules", "demo")
	for _, directory := range []string{"source", "views/cards", "assets/docs", "assets/scripts", "assets/images", "public/css"} {
		if err := os.MkdirAll(filepath.Join(module, filepath.FromSlash(directory)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"views/cards/hero.html": "<article>Hero</article>\n",
		"assets/docs/intro.md":  "# Intro\n",
		"assets/scripts/app.js": "export const app = true;\n",
		"assets/images/😀.txt":   "image placeholder\n",
		"public/css/app.css":    "body {}\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(module, filepath.FromSlash(name)), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	config := `hyperbricks:
  directories:
    hyperbricks:
      path: {base: module, path: source}
    templates:
      path: {base: module, path: views}
    resources:
      path: {base: module, path: assets}
    static:
      path: {base: module, path: public}
`
	if err := os.WriteFile(filepath.Join(module, "package.hyperbricks.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `card:
  - type: template
  - template: {file: "cards/hero.html"}
article:
  - type: markdown
  - file: docs/intro.md
script:
  - type: goja_render
  - script:
      file: {base: resources, path: scripts/app.js}
asset:
  - type: html
  - value:
      file: {base: resources, parts: [images, "😀.txt"]}
styles:
  - type: esbuild
  - outfile:
      path: {base: static, path: css/app.css}
`
	sourcePath := filepath.Join(module, "source", "page hyperbricks %.hyperbricks.yaml")
	uri := pathToURI(sourcePath)
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: workspace, Module: "demo"})
	tests := []struct {
		needle string
		file   string
	}{
		{"cards/hero.html", "views/cards/hero.html"},
		{"docs/intro.md", "assets/docs/intro.md"},
		{"scripts/app.js", "assets/scripts/app.js"},
		{"😀.txt", "assets/images/😀.txt"},
		{"css/app.css", "public/css/app.css"},
	}
	for _, test := range tests {
		t.Run(definitionTestName(test.needle), func(t *testing.T) {
			locations := analyzer.Definitions(uri, source, definitionTestPosition(t, source, test.needle), map[string]string{uri: source})
			assertDefinitionLocation(t, locations, pathToURI(filepath.Join(module, filepath.FromSlash(test.file))), Position{})
		})
	}
	missing := strings.Replace(source, "cards/hero.html", "cards/missing.html", 1)
	if locations := analyzer.Definitions(uri, missing, definitionTestPosition(t, missing, "cards/missing.html")); len(locations) != 0 {
		t.Fatalf("missing template produced a definition: %#v", locations)
	}
}

func TestDefinitionLinksUseCompleteRelationSourceRanges(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	partialDir := filepath.Join(sourceDir, "partials")
	templateDir := filepath.Join(root, "templates")
	resourceDir := filepath.Join(root, "resources", "docs")
	for _, directory := range []string{sourceDir, partialDir, templateDir, resourceDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	pagePath := filepath.Join(sourceDir, "page.hyperbricks.yaml")
	sitePath := filepath.Join(partialDir, "site.hyperbricks.yaml")
	templatePath := filepath.Join(templateDir, "dashboard.html")
	resourcePath := filepath.Join(resourceDir, "manual.md")
	site := "todo_page:\n  - type: hypermedia\n"
	source := `imports:
  - partials/site.hyperbricks.yaml
about_page:
  - inherit: todo_page
dashboard:
  - type: template
  - template:
      file: dashboard.html
manual:
  - type: html
  - value:
      file: {base: resources, path: docs/manual.md}
`
	for path, contents := range map[string]string{
		pagePath:     source,
		sitePath:     site,
		templatePath: "<main>Dashboard</main>\n",
		resourcePath: "# Manual\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	uri := pathToURI(pagePath)
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	tests := []struct {
		name            string
		relation        string
		wantURI         string
		wantTargetStart Position
	}{
		{name: "import", relation: "partials/site.hyperbricks.yaml", wantURI: pathToURI(sitePath)},
		{name: "inherit", relation: "todo_page", wantURI: pathToURI(sitePath), wantTargetStart: Position{}},
		{name: "template", relation: "dashboard.html", wantURI: pathToURI(templatePath)},
		{name: "path", relation: "docs/manual.md", wantURI: pathToURI(resourcePath)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantOrigin := definitionTestRange(t, source, test.relation)
			for offset := 0; offset < len(test.relation); offset++ {
				position := wantOrigin.Start
				position.Character += utf16Length(test.relation[:offset])
				links := analyzer.DefinitionLinks(uri, source, position)
				assertDefinitionLink(t, links, test.wantURI, test.wantTargetStart, wantOrigin)
			}
			if links := analyzer.DefinitionLinks(uri, source, wantOrigin.End); len(links) != 0 {
				t.Fatalf("position after relation produced definition links: %#v", links)
			}
		})
	}
}

func TestDefinitionLinksUseEffectiveInheritanceTargetsAndProvenance(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	partialsDir := filepath.Join(sourceDir, "partials")
	componentsDir := filepath.Join(partialsDir, "components")
	if err := os.MkdirAll(componentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pagePath := filepath.Join(sourceDir, "app.hyperbricks.yaml")
	sitePath := filepath.Join(partialsDir, "site.hyperbricks.yaml")
	layoutPath := filepath.Join(componentsDir, "layout.hyperbricks.yaml")
	source := `imports:
  - partials/site.hyperbricks.yaml
about_page:
  - inherit: todo_page
  - body:
      - values:
          title: Local override
fallback_page:
  - inherit: todo_page
card:
  - type: template
  - values:
      button:
        - type: html
        - value: Not an inheritance child
local_probe:
  - inherit: about_page.body
fallback_probe:
  - inherit: fallback_page.body
invalid_probe:
  - inherit: card.values.button
`
	siteOverlay := "imports: [components/layout.hyperbricks.yaml]\n"
	layoutOverlay := `todo_page:
  - type: hypermedia
  - body:
      - type: template
      - template: shell.html
`
	for path, contents := range map[string]string{
		pagePath:   "disk_page:\n  - type: html\n",
		sitePath:   "disk_site:\n  - type: html\n",
		layoutPath: "disk_layout:\n  - type: html\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	uri := pathToURI(pagePath)
	siteURI := pathToURI(sitePath)
	layoutURI := pathToURI(layoutPath)
	documents := map[string]string{
		uri:       source,
		siteURI:   siteOverlay,
		layoutURI: layoutOverlay,
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	tests := []struct {
		name       string
		relation   string
		wantURI    string
		wantTarget Range
	}{
		{
			name:       "local type-less child overlay",
			relation:   "about_page.body",
			wantURI:    uri,
			wantTarget: definitionTestRange(t, source, "body"),
		},
		{
			name:       "inherited base child fallback",
			relation:   "fallback_page.body",
			wantURI:    layoutURI,
			wantTarget: definitionTestRange(t, layoutOverlay, "body"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantOrigin := definitionTestRange(t, source, test.relation)
			for offset := 0; offset < len(test.relation); offset++ {
				position := wantOrigin.Start
				position.Character += utf16Length(test.relation[:offset])
				links := analyzer.DefinitionLinks(uri, source, position, documents)
				assertDefinitionLink(t, links, test.wantURI, test.wantTarget.Start, wantOrigin)
				if links[0].TargetSelectionRange != test.wantTarget {
					t.Fatalf("definition target range = %#v, want %#v", links[0].TargetSelectionRange, test.wantTarget)
				}
			}
		})
	}

	invalidOrigin := definitionTestRange(t, source, "card.values.button")
	for offset := 0; offset < len("card.values.button"); offset++ {
		position := invalidOrigin.Start
		position.Character += offset
		if links := analyzer.DefinitionLinks(uri, source, position, documents); len(links) != 0 {
			t.Fatalf("ordinary mapping pseudo-path produced definition links: %#v", links)
		}
	}
}

func TestDefinitionsKeepRootMarkersInsideSelectedNamedModule(t *testing.T) {
	workspace := t.TempDir()
	module := filepath.Join(workspace, "modules", "todo-demo-unpoly")
	for _, directory := range []string{"hyperbricks", "resources"} {
		if err := os.MkdirAll(filepath.Join(module, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	owned := filepath.Join(module, "resources", "owned.txt")
	shared := filepath.Join(workspace, "modules", "shared.txt")
	if err := os.WriteFile(owned, []byte("owned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shared, []byte("outside selected module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: workspace, Module: "todo-demo-unpoly"})
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	source := `module_root_owned:
  - type: html
  - value: {file: {base: module_root, path: todo-demo-unpoly/resources/owned.txt}}
root_owned:
  - type: html
  - value: {file: {base: root, path: modules/todo-demo-unpoly/resources/owned.txt}}
module_root_escape:
  - type: html
  - value: {file: {base: module_root, path: shared.txt}}
`
	positions := []Position{
		definitionTestPosition(t, source, "todo-demo-unpoly/resources/owned.txt"),
		definitionTestPosition(t, source, "modules/todo-demo-unpoly/resources/owned.txt"),
	}
	for _, position := range positions {
		assertDefinitionLocation(t, analyzer.Definitions(uri, source, position), pathToURI(owned), Position{})
	}
	if locations := analyzer.Definitions(uri, source, definitionTestPosition(t, source, "shared.txt")); len(locations) != 0 {
		t.Fatalf("module_root escape produced a definition: %#v", locations)
	}
}

func TestDefinitionsRejectTraversalAndSymlinkEscapes(t *testing.T) {
	parent := t.TempDir()
	module := filepath.Join(parent, "module")
	sourceDir := filepath.Join(module, "hyperbricks")
	templateDir := filepath.Join(module, "templates")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideTemplate := filepath.Join(parent, "outside.html")
	outsideSource := filepath.Join(parent, "outside.hyperbricks.yaml")
	if err := os.WriteFile(outsideTemplate, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideSource, []byte("outside:\n  - type: html\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))

	traversal := "card:\n  - type: template\n  - template: {file: ../../outside.html}\n"
	if locations := analyzer.Definitions(uri, traversal, definitionTestPosition(t, traversal, "../../outside.html")); len(locations) != 0 {
		t.Fatalf("module traversal produced a definition: %#v", locations)
	}
	importTraversal := "imports: [../../outside.hyperbricks.yaml]\npage:\n  - type: html\n"
	if locations := analyzer.Definitions(uri, importTraversal, definitionTestPosition(t, importTraversal, "../../outside.hyperbricks.yaml")); len(locations) != 0 {
		t.Fatalf("source traversal produced a definition: %#v", locations)
	}

	templateLink := filepath.Join(templateDir, "linked.html")
	sourceLink := filepath.Join(sourceDir, "linked.hyperbricks.yaml")
	if err := os.Symlink(outsideTemplate, templateLink); err != nil {
		t.Logf("template symlink unavailable: %v", err)
	} else {
		symlinkSource := "card:\n  - type: template\n  - template: {file: linked.html}\n"
		if locations := analyzer.Definitions(uri, symlinkSource, definitionTestPosition(t, symlinkSource, "linked.html")); len(locations) != 0 {
			t.Fatalf("template symlink escape produced a definition: %#v", locations)
		}
	}
	if err := os.Symlink(outsideSource, sourceLink); err != nil {
		t.Logf("source symlink unavailable: %v", err)
	} else {
		symlinkImport := "imports: [linked.hyperbricks.yaml]\npage:\n  - type: html\n"
		if locations := analyzer.Definitions(uri, symlinkImport, definitionTestPosition(t, symlinkImport, "linked.hyperbricks.yaml")); len(locations) != 0 {
			t.Fatalf("source symlink escape produced a definition: %#v", locations)
		}
	}
}

func TestDefinitionsUseUTF16RangesAndEscapedFileURIs(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pagePath := filepath.Join(sourceDir, "page %.hyperbricks.yaml")
	childPath := filepath.Join(sourceDir, "child file.hyperbricks.yaml")
	pageURI, childURI := pathToURI(pagePath), pathToURI(childPath)
	page := "imports: [\"😀.hyperbricks.yaml\", child file.hyperbricks.yaml]\npage:\n  - inherit: shared_😀card\n"
	child := "shared_😀card:\n  - type: html\n"
	documents := map[string]string{pageURI: page, childURI: child}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})

	assertDefinitionLocation(t,
		analyzer.Definitions(pageURI, page, definitionTestPosition(t, page, "child file.hyperbricks.yaml"), documents),
		childURI, Position{})
	wantCharacter := utf16Length("shared_😀card")
	locations := analyzer.Definitions(pageURI, page, definitionTestPosition(t, page, "shared_😀card"), documents)
	assertDefinitionLocation(t, locations, childURI, Position{Line: 0, Character: 0})
	if locations[0].Range.End.Character != wantCharacter {
		t.Fatalf("definition UTF-16 end = %d, want %d", locations[0].Range.End.Character, wantCharacter)
	}
	if !strings.Contains(pageURI, "%25") || !strings.Contains(pageURI, "%20") {
		t.Fatalf("source URI was not escaped: %q", pageURI)
	}
}

func TestDefinitionsUseQuotedScalarSourceWidthForEscapedReferences(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	templateDir := filepath.Join(root, "templates", "cards")
	resourceDir := filepath.Join(root, "resources")
	for _, directory := range []string{sourceDir, templateDir, resourceDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	basePath := filepath.Join(sourceDir, "base file.hyperbricks.yaml")
	baseSource := `"shared_\u0063ard":
  - type: html
`
	if err := os.WriteFile(basePath, []byte(baseSource), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		filepath.Join(templateDir, "hero.html"): "hero\n",
		filepath.Join(templateDir, "it's.html"): "quoted\n",
		filepath.Join(resourceDir, "guide.md"):  "# Guide\n",
	} {
		if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	source := `imports: ["base\u0020file.hyperbricks.yaml"]
page:
  - type: hypermedia
  - content:
      - inherit: "shared_\u0063ard"
template_card:
  - type: template
  - template: {file: "cards/hero\u002Ehtml"}
single_quoted_template:
  - type: template
  - template: {file: 'cards/it''s.html'}
article:
  - type: markdown
  - file: "guide\u002Emd"
`
	pagePath := filepath.Join(sourceDir, "page.hyperbricks.yaml")
	uri := pathToURI(pagePath)
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	tests := []struct {
		needle string
		uri    string
		start  Position
	}{
		{needle: "file.hyperbricks.yaml", uri: pathToURI(basePath)},
		{needle: "ard\"", uri: pathToURI(basePath), start: Position{Line: 0, Character: 0}},
		{needle: "html\"", uri: pathToURI(filepath.Join(templateDir, "hero.html"))},
		{needle: "s.html'", uri: pathToURI(filepath.Join(templateDir, "it's.html"))},
		{needle: "md\"", uri: pathToURI(filepath.Join(resourceDir, "guide.md"))},
	}
	for _, test := range tests {
		t.Run(definitionTestName(test.needle), func(t *testing.T) {
			locations := analyzer.Definitions(uri, source, definitionTestPosition(t, source, test.needle))
			assertDefinitionLocation(t, locations, test.uri, test.start)
		})
	}
	locations := analyzer.Definitions(uri, source, definitionTestPosition(t, source, "ard\""))
	wantEnd := utf16Length(`"shared_\u0063ard"`)
	if locations[0].Range.End.Character != wantEnd {
		t.Fatalf("escaped definition source end = %d, want %d", locations[0].Range.End.Character, wantEnd)
	}
}

func TestDefinitionsResolveVariableAndConfigDeclarationsWithoutEvaluatingValues(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uri := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))
	sharedURI := pathToURI(filepath.Join(sourceDir, "shared.hyperbricks.yaml"))
	configURI := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	source := `imports: [shared.hyperbricks.yaml]
vars:
  local:
    title: Local
page:
  - type: text
  - value:
      format: '%s %s %s'
      args:
        - var: local.title
        - var: {name: 'shared.heading'}
        - config: {path: myconf.site.title}
`
	shared := "vars:\n  shared:\n    heading: {env: PRIVATE_HEADING}\n"
	config := "myconf:\n  site:\n    title: {env: PRIVATE_SITE_TITLE}\n"
	documents := map[string]string{uri: source, sharedURI: shared, configURI: config}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	for _, test := range []struct{ needle, targetURI, targetSource, targetKey string }{
		{"local.title", uri, source, "title:"},
		{"'shared.heading'", sharedURI, shared, "heading:"},
		{"myconf.site.title", configURI, config, "title:"},
	} {
		t.Run(test.needle, func(t *testing.T) {
			origin := definitionTestRange(t, source, test.needle)
			for character := origin.Start.Character; character < origin.End.Character; character++ {
				position := Position{Line: origin.Start.Line, Character: character}
				links := analyzer.DefinitionLinks(uri, source, position, documents)
				assertDefinitionLink(t, links, test.targetURI, definitionTestPosition(t, test.targetSource, test.targetKey), origin)
			}
		})
	}
	// Unreachable open buffers and duplicate declarations do not win by chance.
	delete(documents, sharedURI)
	documents[pathToURI(filepath.Join(sourceDir, "unreachable.hyperbricks.yaml"))] = shared
	if links := analyzer.DefinitionLinks(uri, source, definitionTestPosition(t, source, "shared.heading"), documents); len(links) != 0 {
		t.Fatalf("unreachable var definition: %#v", links)
	}
	documents[sharedURI] = "vars:\n  local:\n    title: Duplicate\n"
	if links := analyzer.DefinitionLinks(uri, source, definitionTestPosition(t, source, "local.title"), documents); len(links) != 0 {
		t.Fatalf("duplicate var definition: %#v", links)
	}
}

func TestResolverResourceDefinitionsFollowOnlyStaticRuntimePaths(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"hyperbricks", "resources/docs", "templates/cards"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"resources/docs/guide.md", "templates/cards/card.html"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	for _, test := range []struct{ name, value, needle, target string }{
		{"flow file", "{file: {base: resources, path: docs/guide.md}}", "docs/guide.md", "resources/docs/guide.md"},
		{"block file", "\n      file:\n        base: resources\n        path: docs/guide.md", "docs/guide.md", "resources/docs/guide.md"},
		{"literal parts", "{file: {base: resources, parts: [docs, guide.md]}}", "guide.md", "resources/docs/guide.md"},
		{"unresolved parts", "{file: {base: resources, parts: [docs, {var: subdir}, guide.md]}}", "guide.md", ""},
		{"path wins", "{path: {base: resources, path: missing.md, parts: [docs, guide.md]}}", "guide.md", ""},
		{"ordinary map", "{base: resources, path: docs/guide.md}", "docs/guide.md", ""},
		{"file ordinary map", "{file: {base: resources, path: docs/guide.md}, caption: ordinary}", "docs/guide.md", ""},
		{"template path", "{template: {file: {path: cards/card.html}}}", "cards/card.html", "templates/cards/card.html"},
		{"template parts", "{template: {file: {parts: [cards, card.html]}}}", "card.html", "templates/cards/card.html"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "page:\n  - type: plugin\n  - data: " + test.value + "\n"
			links := analyzer.DefinitionLinks(uri, source, definitionTestPosition(t, source, test.needle))
			if test.target == "" {
				if len(links) != 0 {
					t.Fatalf("unexpected definition: %#v", links)
				}
				return
			}
			assertDefinitionLink(t, links, pathToURI(filepath.Join(root, test.target)), Position{}, definitionTestRange(t, source, test.needle))
		})
	}
}

func TestDefinitionsResolveUnsavedPackageVariablesAndConfineConfigLinks(t *testing.T) {
	parent := t.TempDir()
	module := filepath.Join(parent, "module")
	if err := os.MkdirAll(filepath.Join(module, "hyperbricks"), 0o755); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	configURI := pathToURI(filepath.Join(module, "package.hyperbricks.yaml"))
	config := "vars:\n  app:\n    label: Local\nmyconf:\n  label: {var: app.label}\n"
	origin := definitionTestRange(t, config, "app.label")
	assertDefinitionLink(t, analyzer.DefinitionLinks(configURI, config, origin.Start), configURI, definitionTestPosition(t, config, "label:"), origin)

	outside := filepath.Join(parent, "outside.yaml")
	if err := os.WriteFile(outside, []byte("myconf: {title: private}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(module, "package.hyperbricks.yaml")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	source := "page:\n  - type: text\n  - value: {config: myconf.title}\n"
	documents := map[string]string{configURI: "myconf: {title: private}"}
	if links := analyzer.DefinitionLinks(uri, source, definitionTestPosition(t, source, "myconf.title"), documents); len(links) != 0 {
		t.Fatalf("symlink config escaped module: %#v", links)
	}
}

func definitionTestPosition(t *testing.T, text, needle string) Position {
	t.Helper()
	index := strings.Index(text, needle)
	if index < 0 {
		t.Fatalf("needle %q not found", needle)
	}
	line := strings.Count(text[:index], "\n")
	lineStart := strings.LastIndex(text[:index], "\n") + 1
	return Position{Line: line, Character: utf16Length(text[lineStart:index])}
}

func definitionTestRange(t *testing.T, text, needle string) Range {
	t.Helper()
	start := definitionTestPosition(t, text, needle)
	return Range{
		Start: start,
		End:   Position{Line: start.Line, Character: start.Character + utf16Length(needle)},
	}
}

func assertDefinitionLocation(t *testing.T, locations []Location, wantURI string, wantStart Position) {
	t.Helper()
	if len(locations) != 1 {
		t.Fatalf("definition locations = %#v, want one", locations)
	}
	if locations[0].URI != wantURI || locations[0].Range.Start != wantStart {
		t.Fatalf("definition location = %#v, want URI %q start %#v", locations[0], wantURI, wantStart)
	}
}

func assertDefinitionLink(t *testing.T, links []LocationLink, wantURI string, wantTargetStart Position, wantOrigin Range) {
	t.Helper()
	if len(links) != 1 {
		t.Fatalf("definition links = %#v, want one", links)
	}
	link := links[0]
	if link.OriginSelectionRange == nil || *link.OriginSelectionRange != wantOrigin {
		t.Fatalf("definition origin = %#v, want %#v", link.OriginSelectionRange, wantOrigin)
	}
	if link.TargetURI != wantURI || link.TargetSelectionRange.Start != wantTargetStart {
		t.Fatalf("definition target = %#v, want URI %q start %#v", link, wantURI, wantTargetStart)
	}
	if link.TargetRange != link.TargetSelectionRange {
		t.Fatalf("definition target range = %#v, selection range = %#v", link.TargetRange, link.TargetSelectionRange)
	}
}

func definitionTestName(value string) string {
	return strings.NewReplacer("/", "_", ".", "_", "😀", "unicode").Replace(value)
}
