package yamlparser

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

func TestResolverResourceReaderAndReadOnlyTemplateValidation(t *testing.T) {
	root := t.TempDir()
	resource := filepath.Join(root, "resources", "copy.txt")
	template := filepath.Join(root, "templates", "static-analysis-template.html")
	parser.AddTemplate("static-analysis-template.html", "runtime content")
	var reads []string
	result, err := ProcessBytes([]byte(`copy:
  - type: text
  - value: {file: {base: resources, path: copy.txt}}
view:
  - type: template
  - template: {file: static-analysis-template.html}
`), Options{
		Paths: PathMarkers{Resources: filepath.Dir(resource)}, TemplateDir: filepath.Dir(template),
		SkipTemplateRegistration: true,
		ResourceReadFile: func(path string) ([]byte, error) {
			reads = append(reads, path)
			return []byte("analysis content"), nil
		},
	})
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("materialize: result=%#v error=%v", result, err)
	}
	if !reflect.DeepEqual(reads, []string{resource, template}) {
		t.Fatalf("resource reads = %#v", reads)
	}
	if result.Materialized["copy"].(map[string]interface{})["value"] != "analysis content" {
		t.Fatalf("callback content not materialized: %#v", result.Materialized)
	}
	if value, _ := parser.GetTemplate("static-analysis-template.html"); value != "runtime content" {
		t.Fatalf("static validation changed runtime template to %q", value)
	}
}

func TestResolverResourceReaderFailureNeverFallsBackToDisk(t *testing.T) {
	result, err := ProcessBytes([]byte(`copy:
  - type: text
  - value: {file: /outside/secret.txt}
view:
  - type: template
  - template: {file: secret.html}
`), Options{
		TemplateDir: "/outside", SkipTemplateRegistration: true,
		ResourceReadFile: func(string) ([]byte, error) { return nil, errors.New("outside analysis boundary") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if !strings.Contains(diagnostic.Message, "outside analysis boundary") {
			t.Fatalf("callback failure was lost: %#v", diagnostic)
		}
	}
}
