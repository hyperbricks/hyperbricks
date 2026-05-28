package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageConfigYAMLFixturesLoadThroughRuntimePath(t *testing.T) {
	root := findRepoRoot(t)
	roots := []string{
		filepath.Join(root, "modules"),
		filepath.Join(root, "test", "dedicated", "modules"),
		filepath.Join(root, "test", "docs", "modules"),
	}

	for _, fixturesRoot := range roots {
		if _, err := os.Stat(fixturesRoot); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(fixturesRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || entry.Name() != PackageConfigFileName {
				return nil
			}

			rel, _ := filepath.Rel(root, path)
			t.Run(filepath.ToSlash(rel), func(t *testing.T) {
				moduleDir := filepath.Dir(path)
				parsed, err := LoadPackageConfigMap(path, moduleDir)
				if err != nil {
					t.Fatalf("LoadPackageConfigMap() error = %v", err)
				}
				if _, ok := parsed["hyperbricks"].(map[string]interface{}); !ok {
					t.Fatalf("parsed hyperbricks = %T, want map", parsed["hyperbricks"])
				}
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk package config fixtures in %s: %v", fixturesRoot, err)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if strings.TrimSpace(parent) == "" || parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}
