package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestAnalyzeSourceReportsNativeSchemaIssuesAtAuthoringKeys(t *testing.T) {
	issues := AnalyzeSource([]byte(`
page:
  - type: html
  - made_up: true
`), SourceOptions{})
	byCode := issuesByCode(issues)
	unsupported := byCode["component.unsupported_field"]
	if unsupported.Message == "" || unsupported.Range.Start.Line != 4 || unsupported.Range.Start.Column != 5 {
		t.Fatalf("unsupported field issue = %#v", unsupported)
	}
	missing := byCode["component.missing_required_field"]
	if missing.Message == "" || !strings.Contains(missing.Message, `requires field "value"`) {
		t.Fatalf("missing required issue = %#v", missing)
	}
}

func TestAnalyzeSourceValidatesDeclaredNestedSchemaFields(t *testing.T) {
	issues := AnalyzeSource([]byte(`
page:
  - type: hypermedia
  - response:
      made_up: true
  - guard:
      auth:
        made_up: true
`), SourceOptions{})
	unsupported := issuesByCodeList(issues, "component.unsupported_field")
	if len(unsupported) != 2 {
		t.Fatalf("nested unsupported issues = %#v (all %#v)", unsupported, issues)
	}
	if unsupported[0].Path != "page.response.made_up" || unsupported[0].Range.Start.Line != 5 || unsupported[0].Range.Start.Column != 7 {
		t.Fatalf("response nested issue = %#v", unsupported[0])
	}
	if unsupported[1].Path != "page.guard.auth.made_up" || unsupported[1].Range.Start.Line != 8 || unsupported[1].Range.Start.Column != 9 {
		t.Fatalf("guard nested issue = %#v", unsupported[1])
	}
}

func TestAnalyzeSourceLeavesDynamicMapsAndResolverValuesOpen(t *testing.T) {
	issues := AnalyzeSource([]byte(`
page:
  - type: hypermedia
  - response:
      headers:
        X-Trace: enabled
  - guard:
      require:
        query:
          preview: true
  - template:
      values:
        heading: Welcome
        payload:
          cards:
            - title: First
            - title: Second
view:
  - type: template
  - values:
      arbitrary: content
leaf:
  - type: html
  - attributes:
      data-role: banner
  - value:
      format: '<h1>%s</h1>'
      args: [Welcome]
`), SourceOptions{})
	if unsupported := issuesByCodeList(issues, "component.unsupported_field"); len(unsupported) != 0 {
		t.Fatalf("dynamic/resolver mappings were treated as schema fields: %#v (all %#v)", unsupported, issues)
	}
	if placements := issuesByCodeList(issues, "component.invalid_child_placement"); len(placements) != 0 {
		t.Fatalf("dynamic list data was treated as component children: %#v (all %#v)", placements, issues)
	}
}

func TestAnalyzeSourceLeavesEditableDefinitionsOpen(t *testing.T) {
	issues := AnalyzeSource([]byte(`
view:
  - type: template
  - inline: '{{.skip_link}}'
  - values:
      skip_link: Skip to content
  - editable:
      skip_link: {type: text, label: "Skip-to-content label", group: "01 Navigation", max: 2000, order: 1}
document:
  - type: markdown
  - content: '# Welcome'
  - editable:
      content: {type: textarea, label: "Document", rows: 12, max: 2000}
`), SourceOptions{})
	if unsupported := issuesByCodeList(issues, "component.unsupported_field"); len(unsupported) != 0 {
		t.Fatalf("editable definitions were treated as schema fields: %#v (all %#v)", unsupported, issues)
	}
}

func TestAnalyzeSourceReportsUnknownTypeAndYAMLLocations(t *testing.T) {
	issues := AnalyzeSource([]byte("page:\n  - type: made-up\n"), SourceOptions{})
	if len(issues) != 1 || issues[0].Code != "component.unknown_type" || issues[0].Range.Start.Line != 2 || issues[0].Range.Start.Column != 11 {
		t.Fatalf("unknown type issues = %#v", issues)
	}
	issues = AnalyzeSource([]byte("page:\n  - type: [\n"), SourceOptions{})
	if len(issues) != 1 || issues[0].Code != "yaml.syntax" || issues[0].Range.Start.Line < 2 {
		t.Fatalf("YAML issues = %#v", issues)
	}
	issues = AnalyzeSource([]byte("page:\n  - type: style\n"), SourceOptions{})
	if len(issues) != 1 || issues[0].Code != "component.unknown_type" {
		t.Fatalf("display-only type name was accepted: %#v", issues)
	}
	issues = AnalyzeSource([]byte("page:\n  - type: styles\n  - file: site.css\n"), SourceOptions{})
	if len(issues) != 0 {
		t.Fatalf("canonical styles token was rejected: %#v", issues)
	}
}

func TestAnalyzeSourceReportsDuplicateAndBrokenInheritance(t *testing.T) {
	duplicate := AnalyzeSource([]byte(`
page:
  - type: tree
  - item:
      - type: text
      - value: first
  - item:
      - type: text
      - value: second
`), SourceOptions{})
	if issue := issuesByCode(duplicate)["duplicate_child_name"]; issue.Message == "" || issue.Severity != SeverityError || issue.Range.Start.Line != 7 {
		t.Fatalf("duplicate issue = %#v (all %#v)", issue, duplicate)
	}

	broken := AnalyzeSource([]byte("page:\n  - inherit: missing\n"), SourceOptions{})
	if issue := issuesByCode(broken)["component.inheritance"]; !strings.Contains(issue.Message, `inherit reference "missing"`) {
		t.Fatalf("inheritance issues = %#v", broken)
	}
}

func TestAnalyzeSourceEnforcesSchemaChildPlacement(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantIssues int
		wantLine   int
	}{
		{
			name: "leaf rejects arbitrary child",
			source: `leaf:
  - type: html
  - value: parent
  - child:
      - type: text
      - value: nested
`,
			wantIssues: 1,
			wantLine:   4,
		},
		{
			name: "values model rejects body child",
			source: `view:
  - type: template
  - inline: '{{.button}}'
  - body:
      - type: html
      - value: misplaced
`,
			wantIssues: 1,
			wantLine:   4,
		},
		{
			name: "values model rejects mounts below the declared path key",
			source: `view:
  - type: template
  - inline: '{{.group}}'
  - values:
      group:
        nested:
          - type: html
          - value: misplaced
`,
			wantIssues: 1,
			wantLine:   6,
		},
		{
			name: "values model accepts named values mount",
			source: `view:
  - type: template
  - inline: '{{.button}}'
  - values:
      button:
        - type: html
        - value: placed
`,
		},
		{
			name: "tree and head slots accept their placements",
			source: `page:
  - type: hypermedia
  - content:
      - type: html
      - value: body
  - head:
      - type: head
      - styles:
          - type: css
          - inline: 'body {}'
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := AnalyzeSource([]byte(test.source), SourceOptions{})
			placementIssues := issuesByCodeList(issues, "component.invalid_child_placement")
			if len(placementIssues) != test.wantIssues {
				t.Fatalf("placement issues = %#v (all %#v), want %d", placementIssues, issues, test.wantIssues)
			}
			if test.wantIssues > 0 && placementIssues[0].Range.Start.Line != test.wantLine {
				t.Fatalf("placement range = %#v, want child key on line %d", placementIssues[0].Range, test.wantLine)
			}
		})
	}
}

func TestAnalyzeSourceUsesUnsavedImportOverlayAndInheritedRequiredFields(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "page.hyperbricks.yaml")
	basePath := filepath.Join(root, "base.hyperbricks.yaml")
	main := []byte("imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\n")
	base := []byte("base:\n  - type: html\n  - value: inherited\n")
	read := func(path string) ([]byte, error) {
		switch filepath.Clean(path) {
		case mainPath:
			return main, nil
		case basePath:
			return base, nil
		default:
			return nil, os.ErrNotExist
		}
	}
	issues := AnalyzeSource(main, SourceOptions{Filename: mainPath, ReadFile: read})
	if len(issues) != 0 {
		t.Fatalf("inherited source issues = %#v", issues)
	}
}

func TestAnalyzeSourceAcceptsInheritedChildOverlaysAcrossImports(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "app.hyperbricks.yaml")
	sitePath := filepath.Join(root, "partials", "site.hyperbricks.yaml")
	viewsPath := filepath.Join(root, "partials", "views.hyperbricks.yaml")
	main := []byte(`imports: [partials/site.hyperbricks.yaml]
about_page:
  - inherit: todo_page
  - body:
      - values:
          content:
            - inherit: todo_about
broken_page:
  - inherit: todo_page
  - not_a_parent_field: true
  - body:
      - not_a_template_field: true
`)
	sources := map[string][]byte{
		mainPath: main,
		sitePath: []byte(`imports: [views.hyperbricks.yaml]
todo_page:
  - type: hypermedia
  - body:
      - type: template
      - inline: '{{.content}}'
      - values:
          content:
            - type: tree
`),
		viewsPath: []byte(`todo_about:
  - type: html
  - value: About
`),
	}
	read := func(path string) ([]byte, error) {
		content, ok := sources[filepath.Clean(path)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return content, nil
	}

	issues := AnalyzeSource(main, SourceOptions{Filename: mainPath, ReadFile: read})
	unsupported := issuesByCodeList(issues, "component.unsupported_field")
	unsupportedPaths := make(map[string]bool, len(unsupported))
	for _, issue := range unsupported {
		unsupportedPaths[issue.Path] = true
	}
	wantUnsupported := []string{
		"broken_page.not_a_parent_field",
		"broken_page.body.not_a_template_field",
	}
	if len(unsupported) != len(wantUnsupported) {
		t.Fatalf("unsupported fields = %#v (all %#v)", unsupported, issues)
	}
	for _, path := range wantUnsupported {
		if !unsupportedPaths[path] {
			t.Fatalf("unsupported fields missing %q: %#v (all %#v)", path, unsupported, issues)
		}
	}
	if placements := issuesByCodeList(issues, "component.invalid_child_placement"); len(placements) != 0 {
		t.Fatalf("inherited child overlay placements = %#v (all %#v)", placements, issues)
	}
	if len(issues) != len(wantUnsupported) {
		t.Fatalf("inherited child overlay issues = %#v", issues)
	}
}

func TestAnalyzeMountedUsesEffectiveTypeForInheritedChildOverlay(t *testing.T) {
	root, err := decodeYAML([]byte(`overlay:
  - made_up: true
`))
	if err != nil {
		t.Fatal(err)
	}
	body := documentBody(root)
	if body == nil || len(body.Content) != 2 {
		t.Fatalf("decoded overlay = %#v", body)
	}

	issues := analyzeMounted(sourceSchemaRegistry(), "page.body", body.Content[1], map[string]interface{}{
		"@type": "<HTML>",
		"value": "inherited",
	}, SeverityError)
	unsupported := issuesByCodeList(issues, "component.unsupported_field")
	if len(issues) != 1 || len(unsupported) != 1 || unsupported[0].Path != "page.body.made_up" || unsupported[0].Message != `html does not support field "made_up"` {
		t.Fatalf("effective inherited child issues = %#v", issues)
	}
}

func TestAnalyzeSourceReportsNativeSchemaIssuesFromReachableImports(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "page.hyperbricks.yaml")
	middlePath := filepath.Join(root, "middle.hyperbricks.yaml")
	basePath := filepath.Join(root, "base.hyperbricks.yaml")
	main := []byte("imports: [middle.hyperbricks.yaml]\npage:\n  - inherit: base\n")
	sources := map[string][]byte{
		mainPath:   main,
		middlePath: []byte("imports: [base.hyperbricks.yaml]\n"),
		basePath:   []byte("base:\n  - type: made-up\n"),
	}
	read := func(path string) ([]byte, error) {
		content, ok := sources[filepath.Clean(path)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return content, nil
	}
	issues := AnalyzeSource(main, SourceOptions{Filename: mainPath, ReadFile: read})
	if len(issues) != 1 || issues[0].Code != "component.unknown_type" || issues[0].File != basePath || issues[0].Range.Start.Line != 2 || issues[0].Range.Start.Column != 11 {
		t.Fatalf("imported native-schema issues = %#v", issues)
	}

	// The same reader is also the unsaved-buffer boundary. Replacing the disk
	// shape with a valid pending source must immediately clear the importer.
	sources[basePath] = []byte("base:\n  - type: html\n  - value: valid overlay\n")
	if issues = AnalyzeSource(main, SourceOptions{Filename: mainPath, ReadFile: read}); len(issues) != 0 {
		t.Fatalf("valid imported overlay issues = %#v", issues)
	}
}

func TestAnalyzeSourceCanDowngradeUnverifiedPluginTypes(t *testing.T) {
	issues := AnalyzeSource([]byte("widget:\n  - type: custom-widget\n"), SourceOptions{UnknownTypeSeverity: SeverityWarning})
	if len(issues) != 1 || issues[0].Code != "component.unknown_type" || issues[0].Severity != SeverityWarning || !strings.Contains(issues[0].Message, "enabled plugin") {
		t.Fatalf("plugin-owned boundary = %#v", issues)
	}
}

func TestAnalyzeSourceUsesExplicitParserPathMarkers(t *testing.T) {
	resources := t.TempDir()
	if err := os.WriteFile(filepath.Join(resources, "greeting.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues := AnalyzeSource([]byte(`
page:
  - type: html
  - value:
      file:
        base: resources
        path: greeting.txt
`), SourceOptions{ParserOptions: yamlparser.Options{Paths: yamlparser.PathMarkers{Resources: resources}}})
	if len(issues) != 0 {
		t.Fatalf("path marker issues = %#v", issues)
	}
}

func issuesByCode(issues []Issue) map[string]Issue {
	result := make(map[string]Issue, len(issues))
	for _, issue := range issues {
		result[issue.Code] = issue
	}
	return result
}

func issuesByCodeList(issues []Issue, code string) []Issue {
	result := make([]Issue, 0)
	for _, issue := range issues {
		if issue.Code == code {
			result = append(result, issue)
		}
	}
	return result
}
