package schema

import "testing"

func TestGojaSchemaIncludesPublicFieldsOnly(t *testing.T) {
	schema := ExtractRegistry(Definitions())
	typ := findType(schema, "<GOJA_RENDER>")
	if typ == nil {
		t.Fatal("goja_render missing from component schema")
	}
	for _, name := range []string{"script", "timeout", "values", "querykeys", "inline", "template", "enclose"} {
		if findField(*typ, name) == nil {
			t.Errorf("public field %s missing", name)
		}
	}
	if findField(*typ, "@goja_prepared") != nil {
		t.Fatal("schema exposes runtime-only prepared metadata")
	}
}
