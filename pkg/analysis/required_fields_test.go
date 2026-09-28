package analysis

import (
	"os"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestRequiredFieldsDistinguishMissingAndEmpty(t *testing.T) {
	for _, test := range []struct {
		name, source, code, message string
		line, column                int
	}{
		{"missing", "doc:\n  - type: text\n", "component.missing_required_field", `requires field "value"`, 2, 11},
		{"literal empty", "doc:\n  - type: text\n  - value: ''\n", "component.empty_required_field", "text.value resolves to an empty string", 3, 5},
		{"normalized key", "doc:\n  - type: text\n  - ' value ': ''\n", "component.empty_required_field", "text.value resolves to an empty string", 3, 5},
		{"empty resolver", "doc:\n  - type: text\n  - value: {file: {base: resources, path: docs/test.md}}\n", "component.empty_required_field", "file: resources/docs/test.md", 3, 5},
		{"empty var", "vars:\n  copy: ''\ndoc:\n  - type: text\n  - value: {var: copy}\n", "component.empty_required_field", "text.value resolves to an empty string", 5, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads := 0
			issues := AnalyzeSource([]byte(test.source), SourceOptions{ParserOptions: yamlparser.Options{ResourceReadFile: func(string) ([]byte, error) { reads++; return []byte{}, nil }}})
			if len(issues) != 1 {
				t.Fatalf("issues=%#v", issues)
			}
			got := issues[0]
			if got.Code != test.code || !strings.Contains(got.Message, test.message) || got.Range.Start.Line != test.line || got.Range.Start.Column != test.column {
				t.Fatalf("issue=%#v", got)
			}
			if got.Path != "doc.value" {
				t.Fatalf("path=%q", got.Path)
			}
			if reads > 1 {
				t.Fatalf("required-value diagnostics reread resource %d times", reads)
			}
		})
	}
}

func TestRequiredFieldsPreserveWhitespaceAndMarkdownEmpty(t *testing.T) {
	for _, source := range []string{
		"doc:\n  - type: text\n  - value: '   '\n",
		"doc:\n  - type: text\n  - value: {file: whitespace.txt}\n",
		"doc:\n  - type: markdown\n  - content: ''\n",
	} {
		issues := AnalyzeSource([]byte(source), SourceOptions{ParserOptions: yamlparser.Options{ResourceReadFile: func(string) ([]byte, error) { return []byte(" \n\t"), nil }}})
		if len(issues) != 0 {
			t.Fatalf("unexpected issues for %s: %#v", source, issues)
		}
	}
}

func TestRequiredFileFailureIsNotDescribedAsEmptyFile(t *testing.T) {
	source := []byte("doc:\n  - type: text\n  - value: {file: missing.txt}\n")
	issues := AnalyzeSource(source, SourceOptions{ParserOptions: yamlparser.Options{ResourceReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist }}})
	if len(issues) != 2 {
		t.Fatalf("issues=%#v", issues)
	}
	for _, issue := range issues {
		if strings.Contains(issue.Message, "file is empty") {
			t.Fatalf("guessed successful read: %#v", issue)
		}
	}
	if issuesByCode(issues)["component.empty_required_field"].Range.Start.Line != 3 || issuesByCode(issues)["file_missing"].Message == "" {
		t.Fatalf("issues=%#v", issues)
	}
}

func TestRequiredFileReferenceOnlyUsesLiteralSource(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{"file: {base: resources, path: docs/test.md}", "resources/docs/test.md"},
		{"file: {base: resources, parts: [docs, test.md]}", "resources/docs/test.md"},
		{"file: {base: resources, path: docs/test.md, parts: [ignored.md]}", "resources/docs/test.md"},
		{"file: {base: resources, parts: [docs, {env: SECRET_FILE}]}", ""},
		{"var: {name: copy, default: {file: fallback.md}}", ""},
	} {
		root, err := decodeYAML([]byte(test.source))
		if err != nil {
			t.Fatal(err)
		}
		if got := requiredFileReference(documentBody(root)); got != test.want {
			t.Fatalf("reference(%s)=%q want %q", test.source, got, test.want)
		}
	}
}
