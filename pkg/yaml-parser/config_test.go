package yamlparser

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestProcessConfigBytesResolvesPackageConfigShape(t *testing.T) {
	result, err := ProcessConfigBytes([]byte(`
myconf:
  demo:
    title: Package YAML

hyperbricks:
  mode: development
  live:
    cache: 30s
  server:
    port: 8080
    routing:
      clean_urls: true
      index_files:
        - index.html
        - index.htm
  directories:
    render:
      path:
        base: module
        path: rendered
    templates:
      path:
        base: module
        path: templates
    plugins: ./bin/plugins
  plugins:
    enabled:
      - MarkdownPlugin@2.0.0
`), Options{
		Paths: PathMarkers{
			Module: "modules/demo",
		},
	})
	if err != nil {
		t.Fatalf("ProcessConfigBytes() error = %v", err)
	}

	hyperbricks := result.Materialized["hyperbricks"].(map[string]interface{})
	directories := hyperbricks["directories"].(map[string]interface{})
	if directories["render"] != filepath.Join("modules/demo", "rendered") {
		t.Fatalf("render directory = %#v", directories["render"])
	}
	if directories["templates"] != filepath.Join("modules/demo", "templates") {
		t.Fatalf("templates directory = %#v", directories["templates"])
	}
	plugins := hyperbricks["plugins"].(map[string]interface{})
	if !reflect.DeepEqual(plugins["enabled"], []interface{}{"MarkdownPlugin@2.0.0"}) {
		t.Fatalf("plugins.enabled = %#v", plugins["enabled"])
	}
	if result.Materialized["vars"] != nil {
		t.Fatalf("vars leaked into materialized config: %#v", result.Materialized["vars"])
	}
}

func TestProcessConfigBytesKeepsTemplateSyntaxLiteral(t *testing.T) {
	result, err := ProcessConfigBytes([]byte(`
myconf:
  template: "{{ .Title | upper }}"
`), Options{})
	if err != nil {
		t.Fatalf("ProcessConfigBytes() error = %v", err)
	}
	myconf := result.Materialized["myconf"].(map[string]interface{})
	if myconf["template"] != "{{ .Title | upper }}" {
		t.Fatalf("template = %#v", myconf["template"])
	}
}
