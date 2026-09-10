package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Package YAML materializes scalar text. Preserve that text for strict startup validation.
func TestPackageGoMaxProcsPreservesValidationInput(t *testing.T) {
	Init_configuration()
	for _, tc := range []struct {
		yaml string
		want any
	}{
		{"auto", "auto"}, {"4", "4"}, {"true", "true"}, {"1.5", "1.5"}, {"null", ""}, {"\"4\"", "4"},
	} {
		t.Run(tc.yaml, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, PackageConfigFileName)
			if err := os.WriteFile(path, []byte("hyperbricks:\n  server:\n    gomaxprocs: "+tc.yaml+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			parsed, err := LoadPackageConfigMap(path, dir)
			if err != nil {
				t.Fatal(err)
			}
			var config Config
			if err := decodeConfig(parsed["hyperbricks"], &config); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(config.Server.GoMaxProcs, tc.want) {
				t.Fatalf("got %#v (%T), want %#v (%T)", config.Server.GoMaxProcs, config.Server.GoMaxProcs, tc.want, tc.want)
			}
		})
	}
}
