package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestAnalyzeSourceRetainsResolverWarningsAndErrors(t *testing.T) {
	const source = `doc:
  - type: text
  - value: {file: {base: resources, path: docs/missing.md}}
required:
  - type: text
  - value:
      env: {name: HYPERBRICKS_ANALYSIS_MISSING_REQUIRED_9824, required: true}
empty:
  - type: text
  - value: {var: {name: undefined, default: fallback}}
`
	issues := AnalyzeSource([]byte(source), SourceOptions{ParserOptions: yamlparser.Options{
		ResourceReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}})
	byCode := issuesByCode(issues)
	file := byCode["file_missing"]
	if file.Severity != SeverityWarning || file.Range.Start.Line != 3 || file.Range.Start.Column != 13 || file.Range.End.Column != 17 {
		t.Fatalf("file resolver warning = %#v (all %#v)", file, issues)
	}
	env := byCode["env_missing"]
	if env.Severity != SeverityError || env.Range.Start.Line != 7 || env.Range.Start.Column != 7 {
		t.Fatalf("required env error = %#v (all %#v)", env, issues)
	}
	if byCode["var_missing"].Message != "" {
		t.Fatalf("defaulted variable reported missing: %#v", issues)
	}
}

func TestAnalyzeSourceValidBlockFlowAndNestedResolvers(t *testing.T) {
	source := `vars:
  label: Hello
block:
  - type: text
  - value:
      file:
        base: resources
        path: docs/intro.md
flow:
  - type: text
  - value: {file: {base: resources, path: docs/intro.md}}
nested:
  - type: text
  - value: {format: '%s %s', args: [{var: label}, {config: {path: site.title, default: there}}]}
`
	issues := AnalyzeSource([]byte(source), SourceOptions{ParserOptions: yamlparser.Options{
		ResourceReadFile: func(string) ([]byte, error) { return []byte("content"), nil },
	}})
	if len(issues) != 0 {
		t.Fatalf("valid resolvers rejected: %#v", issues)
	}
}

func TestAnalyzeSourceResolverDiagnosticsKeepImportedOwnership(t *testing.T) {
	dir := t.TempDir()
	mainPath, basePath := filepath.Join(dir, "app.hyperbricks.yaml"), filepath.Join(dir, "base.hyperbricks.yaml")
	main := []byte("imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\nother:\n  - inherit: base\n")
	base := []byte("base:\n  - type: text\n  - value: {file: {base: resources, path: absent.md}}\n")
	issues := AnalyzeSource(main, SourceOptions{
		Filename: mainPath,
		ReadFile: func(path string) ([]byte, error) {
			if path == mainPath {
				return main, nil
			}
			if path == basePath {
				return base, nil
			}
			return nil, os.ErrNotExist
		},
		ParserOptions: yamlparser.Options{ResourceReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist }},
	})
	missing := issuesByCodeList(issues, "file_missing")
	if len(missing) != 1 || missing[0].File != basePath || missing[0].Range.Start.Line != 3 || missing[0].Range.Start.Column != 13 {
		t.Fatalf("inherited warning ownership or deduplication: %#v", missing)
	}
}

func TestAnalyzeSourceExplainsMissingResolverSeparator(t *testing.T) {
	issues := AnalyzeSource([]byte("mydoc:\n  - type: text\n  - value: ok # accepted\n  - value {file: {base: resources, path: docs/llms.md}} # not accepted\n"), SourceOptions{})
	if len(issues) != 1 || issues[0].Code != "yaml.missing_separator" || issues[0].Range.Start.Line != 4 || issues[0].Range.Start.Column != 5 || !strings.Contains(issues[0].Message, "- value: {...}") {
		t.Fatalf("missing separator feedback = %#v", issues)
	}
	issues = AnalyzeSource([]byte("mydoc:\n  - type: text\n  - value: ok\n  - value: {file: {base: resources, path: docs/llms.md}}\n"), SourceOptions{})
	if len(issues) != 1 || issues[0].Code != "component.duplicate" || issues[0].Range.Start.Line != 4 {
		t.Fatalf("fixed syntax must reveal duplicate property: %#v", issues)
	}
}

func TestAnalyzeSourceClassifiesReservedAndMapDuplicates(t *testing.T) {
	for _, source := range []string{
		"doc:\n  - type: text\n  - type: html\n",
		"doc:\n  - type: text\n  - value: {var: {name: a, name: b}}\n",
	} {
		issues := AnalyzeSource([]byte(source), SourceOptions{})
		if len(issues) != 1 || issues[0].Code != "component.duplicate" {
			t.Fatalf("duplicate classification = %#v", issues)
		}
	}
}

func TestAnalyzeSourceResolverFailuresAnchorTheActualResolver(t *testing.T) {
	issues := AnalyzeSource([]byte(`vars:
  shared:
    env: HYPERBRICKS_ANALYSIS_MISSING_OPTIONAL_6820
doc:
  - type: text
  - value: {var: shared}
bad_path:
  - type: text
  - value: {path: {base: typo, path: file.txt}}
bad_file:
  - type: text
  - value: {file: [not, a, path]}
bad_format:
  - type: text
  - value: {format: '%s', args: invalid}
`), SourceOptions{ParserOptions: yamlparser.Options{
		ResourceReadFile: func(string) ([]byte, error) { t.Fatal("invalid resolver must not read files"); return nil, nil },
	}})
	byCode := issuesByCode(issues)
	if issue := byCode["env_missing"]; issue.Range.Start.Line != 3 || issue.Range.Start.Column != 5 {
		t.Fatalf("variable warning must point to declaration: %#v", issue)
	}
	if issue := byCode["path_base_unknown"]; issue.Range.Start.Line != 9 || issue.Range.Start.Column != 26 || issue.Range.End.Column != 30 {
		t.Fatalf("unknown base range = %#v", issue)
	}
	for _, code := range []string{"file_invalid", "format_args_invalid"} {
		if byCode[code].Message == "" || byCode[code].Severity != SeverityWarning {
			t.Fatalf("missing materializer diagnostic %s: %#v", code, issues)
		}
	}
}
