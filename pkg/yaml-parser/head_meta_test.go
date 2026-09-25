package yamlparser

import "testing"

func TestHeadMetadataNullOverridesSurviveInheritance(t *testing.T) {
	result, err := ProcessBytes([]byte(`base_head:
  - type: head
  - meta:
      author: Original
      empty: ''
      og:title: Original
child_head:
  - inherit: base_head
  - meta:
      author: null
      og:title: Updated
grandchild:
  - inherit: child_head
  - meta:
      author: Restored
ordinary:
  - type: template
  - values:
      meta:
        author: null
  - editable: [name]
`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	child := result.Materialized["child_head"].(map[string]interface{})["meta"].(map[string]interface{})
	if v, ok := child["author"]; !ok || v != nil {
		t.Fatalf("null removal lost: %#v", child)
	}
	if child["empty"] != "" || child["og:title"] != "Updated" {
		t.Fatalf("metadata changed: %#v", child)
	}
	base := result.Materialized["base_head"].(map[string]interface{})["meta"].(map[string]interface{})
	if base["author"] != "Original" {
		t.Fatal("base mutated")
	}
	grand := result.Materialized["grandchild"].(map[string]interface{})["meta"].(map[string]interface{})
	if grand["author"] != "Restored" {
		t.Fatal("re-add failed")
	}
	ordinary := result.Materialized["ordinary"].(map[string]interface{})["values"].(map[string]interface{})["meta"].(map[string]interface{})
	if ordinary["author"] != "" {
		t.Fatal("unrelated null semantics changed")
	}
}
