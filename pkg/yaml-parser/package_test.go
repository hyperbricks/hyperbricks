package yamlparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageCompositionPrecedenceAndOrigins(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"common.yaml":  "vars: {port: 8000}\nhyperbricks: {server: {port: {var: port}}, mode: live}\n",
		"left.yaml":    "imports: [common.yaml]\nhyperbricks: {mode: debug, list: [old]}\n",
		"right.yaml":   "imports: [common.yaml]\nhyperbricks: {list: []}\n",
		"package.yaml": "imports: [left.yaml, right.yaml]\nvars: {port: 9090}\nhyperbricks: {local: true}\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ProcessPackageConfigFile(filepath.Join(root, "package.yaml"), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	hb := result.Materialized["hyperbricks"].(map[string]interface{})
	if hb["mode"] != "live" {
		t.Fatalf("shared import must reapply in declared order: %#v", hb)
	}
	if hb["server"].(map[string]interface{})["port"] != "9090" {
		t.Fatalf("entry vars must resolve after merge: %#v", hb)
	}
	if len(hb["list"].([]interface{})) != 0 {
		t.Fatal("empty list did not replace")
	}
	if _, ok := result.Materialized["imports"]; ok {
		t.Fatal("imports leaked to runtime")
	}
	origin := result.Origins["hyperbricks.server.port"]
	if origin.File != filepath.Join(root, "common.yaml") || origin.Line != 2 {
		t.Fatalf("origin: %#v", origin)
	}
	if len(result.Sources) != 4 {
		t.Fatalf("source graph: %v", result.Dependencies)
	}
}

func TestPackageCompositionAtomicResolversAndNull(t *testing.T) {
	root := t.TempDir()
	read := func(path string) ([]byte, error) {
		return []byte("vars: {value: old}\nhyperbricks: {value: {var: value}, list: [one], gone: before}"), nil
	}
	result, err := ProcessPackageConfigBytes([]byte("imports: [base.yaml]\nhyperbricks: {value: {env: VALUE}, list: null, gone: null}"), filepath.Join(root, "entry.yaml"), root, Options{ResourceReadFile: read, Env: map[string]string{"VALUE": "new"}})
	if err != nil {
		t.Fatal(err)
	}
	hb := result.Materialized["hyperbricks"].(map[string]interface{})
	if hb["value"] != "new" || hb["list"] != "" || hb["gone"] != "" {
		t.Fatalf("atomic replacement: %#v", hb)
	}
}

func TestPackageCompositionRejectsInvalidGraph(t *testing.T) {
	for _, tc := range []struct{ name, entry, imported, want string }{
		{"cycle", "imports: [child.yaml]", "imports: [entry.yaml]", "circular"},
		{"duplicate", "imports: [child.yaml]", "value: 1\nvalue: 2", "duplicate"},
		{"nonmapping", "imports: [child.yaml]", "- item", "mapping"},
		{"badentry", "imports: [{env: FILE}]", "{}", "literal"},
		{"escape", "imports: [../outside.yaml]", "{}", "boundary"},
		{"multidocument", "imports: [child.yaml]", "{}\n---\n{}", "one YAML"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			reader := func(path string) ([]byte, error) { return []byte(tc.imported), nil }
			_, err := ProcessPackageConfigBytes([]byte(tc.entry), filepath.Join(root, "entry.yaml"), root, Options{ResourceReadFile: reader})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %s", err, tc.want)
			}
		})
	}
}

func TestPackageCompositionConfinesSymlinksAndUsesReader(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err := ProcessPackageConfigBytes([]byte("imports: [escape/secret.yaml]"), filepath.Join(root, "entry.yaml"), root, Options{ResourceReadFile: func(string) ([]byte, error) { called = true; return []byte("{}"), nil }})
	if err == nil || !strings.Contains(err.Error(), "boundary") || called {
		t.Fatalf("escape error=%v readerCalled=%v", err, called)
	}
}

func TestPackageCompositionUnsavedProfileAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	var readPath string
	result, err := ProcessPackageConfigBytes([]byte("imports: [child.yaml]"), filepath.Join(root, "profiles", "live.yaml"), root, Options{ResourceReadFile: func(path string) ([]byte, error) {
		readPath = path
		return []byte("hyperbricks:\n  value: {var: missing}\n"), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if readPath != filepath.Join(root, "profiles", "child.yaml") {
		t.Fatal(readPath)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Source != readPath || result.Diagnostics[0].Line != 2 {
		t.Fatalf("diagnostics=%#v", result.Diagnostics)
	}
	generic, err := ProcessConfigBytes([]byte("imports: [missing.yaml]"), Options{})
	if err != nil || generic.Materialized["imports"] == nil {
		t.Fatalf("generic parser changed: %v", err)
	}
}
