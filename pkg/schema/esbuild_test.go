package schema

import "testing"

func TestEsbuildSchemaIncludesAllOptions(t *testing.T) {
	typ := findType(ExtractRegistry(Definitions()), "<ESBUILD>")
	if typ == nil {
		t.Fatal("esbuild missing from schema")
	}
	for _, name := range []string{"entry", "outfile", "minify", "minify_identifiers", "mangle", "sourcemap", "cache", "fingerprint", "debug", "enclose", "target", "loader", "external"} {
		if findField(*typ, name) == nil {
			t.Errorf("missing field %s", name)
		}
	}
	if findField(*typ, "@esbuild_prepared") != nil {
		t.Fatal("runtime state leaked into schema")
	}
	if findField(*typ, "binary") != nil {
		t.Fatal("removed external compiler option leaked into schema")
	}
}
