package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageImportSourceTypesAndOrigin(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "hooks.yaml")
	err := os.WriteFile(file, []byte("hyperbricks:\n  development:\n    hooks:\n      before_start:\n        - name: task\n          command: [echo, 123]\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidatePackageConfigBytes([]byte("imports: [hooks.yaml]\n"), root)
	if err == nil || !strings.Contains(err.Error(), "hooks.yaml:6:") || !strings.Contains(err.Error(), "must be a string") {
		t.Fatalf("source validation: %v", err)
	}
}

func TestPackageProfileOriginAndControlledReader(t *testing.T) {
	root := t.TempDir()
	var paths []string
	config, err := ValidatePackageConfigBytesAt([]byte("imports: [runtime.yaml]\n"), filepath.Join(root, "profiles", "live.yaml"), root, func(path string) ([]byte, error) {
		paths = append(paths, path)
		return []byte("hyperbricks: {mode: live}"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Mode != LIVE_MODE || len(paths) != 1 || paths[0] != filepath.Join(root, "profiles", "runtime.yaml") {
		t.Fatalf("profile=%#v paths=%v", config, paths)
	}
}
