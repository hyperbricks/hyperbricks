package legacyyamlconverter

import (
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestConvertBytesPreservesLegacySourceIntent(t *testing.T) {
	source := []byte(`@import "partials/site.hyperbricks"

shell = <HYPERMEDIA>
shell.route = index
shell.10 = <TEMPLATE>
shell.10.template = {{TEMPLATE:patterns/shell.html}}
shell.10.values {
    title = Base title
    content = <TREE>
    content.10 = <HTML>
    content.10.value = <<[
        <p>Base content</p>
    ]>>
}

page <<< shell
page.route = converted
page.10.values {
    title = Converted title
    content <<< replacement
}

replacement = <TREE>
replacement.10 = <TEXT>
replacement.10.value = Replacement content
`)

	result, err := ConvertBytes(source, Options{})
	if err != nil {
		t.Fatalf("ConvertBytes returned error: %v", err)
	}

	for _, want := range []string{
		`imports:`,
		`- "partials/site.hyperbricks.yaml"`,
		`shell:`,
		`- type: hypermedia`,
		`template_10:`,
		`html_10:`,
		`page:`,
		`- inherit: "shell"`,
		`content:`,
		`- inherit: "replacement"`,
		`value: |`,
		`<p>Base content</p>`,
	} {
		if !strings.Contains(result.YAML, want) {
			t.Fatalf("converted YAML missing %q:\n%s", want, result.YAML)
		}
	}
}

func TestConvertBytesOutputMaterializesThroughYAMLParser(t *testing.T) {
	source := []byte(`shell = <HYPERMEDIA>
shell.route = index
shell.10 = <TEMPLATE>
shell.10.template = {{TEMPLATE:patterns/shell.html}}
shell.10.values {
    title = Base title
    content = <TREE>
}

panel = <TREE>
panel.10 = <HTML>
panel.10.value = <p>Panel</p>

page <<< shell
page.route = converted
page.10.values {
    content <<< panel
}
`)

	result, err := ConvertBytes(source, Options{})
	if err != nil {
		t.Fatalf("ConvertBytes returned error: %v", err)
	}
	doc, err := yamlparser.ParseBytes([]byte(result.YAML))
	if err != nil {
		t.Fatalf("converted YAML did not parse:\n%s\nerror: %v", result.YAML, err)
	}
	materialized, err := doc.Materialize()
	if err != nil {
		t.Fatalf("converted YAML did not materialize:\n%s\nerror: %v", result.YAML, err)
	}

	page := materialized["page"].(map[string]interface{})
	body := page["template_10"].(map[string]interface{})
	values := body["values"].(map[string]interface{})
	content := values["content"].(map[string]interface{})

	if page["@type"] != "<HYPERMEDIA>" {
		t.Fatalf("page @type = %#v, want <HYPERMEDIA>", page["@type"])
	}
	if page["route"] != "converted" {
		t.Fatalf("page route = %#v, want converted", page["route"])
	}
	if body["template"] != "{{TEMPLATE:patterns/shell.html}}" {
		t.Fatalf("template = %#v", body["template"])
	}
	if content["@type"] != "<TREE>" {
		t.Fatalf("content @type = %#v, want <TREE>", content["@type"])
	}
}
