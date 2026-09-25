package yamlparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeSourceMetadataPreservesImportedFieldsAndNestedComponents(t *testing.T) {
	module := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(module, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("resources/calculate.js", "function main() { return {}; }")
	write("templates/page.html", "{{.content}}")
	write("hyperbricks/partials/base.hyperbricks.yaml", `base:
  - type: hypermedia
  - template:
      template:
        file: page.html
      values:
        content:
          - type: goja_render
          - script:
              file:
                base: resources
                path: calculate.js
          - inline: '{{.Data}}'
`)
	write("hyperbricks/page.hyperbricks.yaml", `imports: [partials/base.hyperbricks.yaml]
page:
  - inherit: base
  - route: index
`)
	opts := Options{IncludeSourceMetadata: true, TemplateDir: filepath.Join(module, "templates"),
		Paths: PathMarkers{Module: module, Resources: filepath.Join(module, "resources")}}
	result, err := ProcessFile(filepath.Join(module, "hyperbricks/page.hyperbricks.yaml"), opts)
	if err != nil {
		t.Fatal(err)
	}
	page := result.Materialized["page"].(map[string]interface{})
	if page["hyperbricksfile"] != "hyperbricks/page.hyperbricks.yaml" || page["hyperbrickspath"] != "page" {
		t.Fatalf("page context = %#v", page)
	}
	template := page["template"].(map[string]interface{})
	source := template["@source"].(map[string]interface{})
	if source["file"] != "hyperbricks/partials/base.hyperbricks.yaml" {
		t.Fatalf("template origin = %#v", source)
	}
	if source["resources"].(map[string]interface{})["template"] != "templates/page.html" {
		t.Fatalf("template resource = %#v", source)
	}
	child := template["values"].(map[string]interface{})["content"].(map[string]interface{})
	if child["hyperbricksfile"] != "hyperbricks/partials/base.hyperbricks.yaml" || child["hyperbrickspath"] != "page.template.values.content" {
		t.Fatalf("child origin = %#v", child)
	}
	source = child["@source"].(map[string]interface{})
	if source["line"].(int) == 0 || source["resources"].(map[string]interface{})["script"] != "resources/calculate.js" {
		t.Fatalf("script source = %#v", source)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	opts.IncludeSourceMetadata = false
	plain, err := ProcessFile(filepath.Join(module, "hyperbricks/page.hyperbricks.yaml"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Materialized["page"].(map[string]interface{})["@source"] != nil {
		t.Fatal("metadata leaked into ordinary materialization")
	}
}
