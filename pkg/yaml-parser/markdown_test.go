package yamlparser

import "testing"

func TestMarkdownAndEditableFieldNames(t *testing.T) {
	result, err := ProcessBytes([]byte(`vars:
  label: Document
base:
  - type: markdown
  - file: uploads/documents/base.md
  - editable:
      file:
        type: asset
        label: {var: label}
copy:
  - inherit: base
  - file: uploads/documents/copy.md
template:
  - type: template
  - inline: '{{.file}}'
  - values: {file: label}
  - editable:
      file: text
`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"base", "copy"} {
		node := result.Materialized[name].(map[string]interface{})
		if node["@type"] != "<MARKDOWN>" || node["file"] != "uploads/documents/"+name+".md" {
			t.Fatalf("bad node: %+v", node)
		}
		fields, ok := node["editable"].(map[string]interface{})
		if !ok {
			t.Fatalf("metadata resolved as file: %+v", node)
		}
		field := fields["file"].(map[string]interface{})
		if field["label"] != "Document" {
			t.Fatalf("metadata resolver lost: %+v", field)
		}
	}
	node := result.Materialized["template"].(map[string]interface{})
	if _, ok := node["editable"].(map[string]interface{}); !ok {
		t.Fatal("template editable file was resolved")
	}
}
