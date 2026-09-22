package schema

import "testing"

func TestMarkdownSchema(t *testing.T) {
	typ := findType(ExtractRegistry(Definitions()), "<MARKDOWN>")
	if typ == nil {
		t.Fatal("markdown missing from schema")
	}
	for _, key := range []string{"content", "file", "class", "max_bytes", "enclose", "editable"} {
		if findField(*typ, key) == nil {
			t.Errorf("missing %s", key)
		}
	}
	for _, key := range []string{"plugin", "route", "querykeys", "file_query"} {
		if findField(*typ, key) != nil {
			t.Errorf("unexpected %s", key)
		}
	}
}
