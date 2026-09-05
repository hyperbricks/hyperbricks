package yamlparser

import (
	"strings"
	"testing"
)

func TestEsbuildAliasInheritanceAndExplicitPaths(t *testing.T) {
	source := `bundle:
  - type: esbuild
  - minifyident: "true"
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/app.js}
page:
  - type: hypermedia
  - head:
      - type: head
      - arbitrary_name:
          - inherit: bundle
          - minify_identifiers: false
other:
  - inherit: bundle
  - minifyident: false
`
	result, err := ProcessBytes([]byte(source), Options{Paths: PathMarkers{Resources: "/site/custom-source", Static: "/site/custom-public"}})
	if err != nil {
		t.Fatal(err)
	}
	root := result.Materialized["bundle"].(map[string]interface{})
	if root["@type"] != "<ESBUILD>" || root["entry"] != "/site/custom-source/js/main.js" || root["outfile"] != "/site/custom-public/js/app.js" {
		t.Fatalf("wrong materialization: %#v", root)
	}
	if _, ok := root["minifyident"]; ok {
		t.Fatal("alias was not normalized")
	}
	page := result.Materialized["page"].(map[string]interface{})
	child := page["head"].(map[string]interface{})["arbitrary_name"].(map[string]interface{})
	if child["minify_identifiers"] != "false" || result.Materialized["other"].(map[string]interface{})["minify_identifiers"] != "false" {
		t.Fatalf("inherited alias could not be overridden: child=%#v other=%#v", child, result.Materialized["other"])
	}
	_, err = ProcessBytes([]byte("bad:\n  - type: esbuild\n  - minifyident: true\n  - minify_identifiers: false\n"), Options{})
	if err == nil || !strings.Contains(err.Error(), "only one") {
		t.Fatalf("ambiguous aliases accepted: %v", err)
	}
}
