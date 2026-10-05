package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveCollectorsExcludeResponseCache(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "custom"}[custom], func(t *testing.T) {
			root := t.TempDir()
			config := "hyperbricks:\n  mode: live\n"
			if custom {
				config += "  directories:\n    cache: {path: {base: module, path: runtime-cache}}\n"
			}
			files := map[string]string{
				PackageConfigFileName:               config,
				"hyperbricks/page.hyperbricks.yaml": "page:\n  - type: hypermedia\n",
				"resources/source.txt":              "source", ".cache/responses/body": "cached",
			}
			if custom {
				files["runtime-cache/responses/body"] = "custom cached"
				files["runtime-cache-source/keep.txt"] = "similarly named source"
			}
			for name, body := range files {
				file := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, collect := range map[string]func(string) ([]buildFile, error){"build": collectModuleFiles, "runtime": collectRuntimeSnapshotFiles} {
				got, err := collect(root)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				included := map[string]bool{}
				for _, file := range got {
					path := filepath.ToSlash(file.rel)
					included[path] = true
					if strings.HasPrefix(path, ".cache") || path == "runtime-cache" || strings.HasPrefix(path, "runtime-cache/") {
						t.Fatalf("%s included cache file %q", name, path)
					}
				}
				for _, path := range []string{PackageConfigFileName, "hyperbricks/page.hyperbricks.yaml", "resources/source.txt"} {
					if !included[path] {
						t.Fatalf("%s omitted source %q", name, path)
					}
				}
				if custom && !included["runtime-cache-source/keep.txt"] {
					t.Fatalf("%s excluded similarly named source directory", name)
				}
			}
			snapshot := t.TempDir()
			if err := copyRuntimeSnapshot(context.Background(), root, snapshot); err != nil {
				t.Fatal(err)
			}
			if err := verifyRuntimeSnapshot(context.Background(), root, snapshot); err != nil {
				t.Fatalf("snapshot with excluded cache no longer verifies: %v", err)
			}
		})
	}
}

func TestCacheExclusionPreservesRelativeDirectoryMeaning(t *testing.T) {
	working := t.TempDir()
	t.Chdir(working)
	root := filepath.Join(working, "modules", "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "hyperbricks:\n  directories:\n    cache: modules/demo/generated-cache\n"
	if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	excluded, err := moduleCacheExclusion(root)
	if err != nil {
		t.Fatal(err)
	}
	if !excluded(filepath.Join(root, "generated-cache", "body")) || excluded(filepath.Join(root, "modules", "demo", "generated-cache", "body")) {
		t.Fatal("bare relative cache path did not retain invocation-directory semantics")
	}
}

func TestArchiveCollectorsRejectUnsafeCustomCacheDirectory(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"static", "render", "resources", "templates", "hyperbricks", "plugins"} {
		t.Run(key, func(t *testing.T) {
			protected := filepath.Join(root, "protected-"+key)
			config := fmt.Sprintf("hyperbricks:\n  directories:\n    %s: %q\n    cache: %q\n", key, protected, filepath.Join(protected, "cache"))
			if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			for name, collect := range map[string]func(string) ([]buildFile, error){"build": collectModuleFiles, "snapshot": collectRuntimeSnapshotFiles} {
				if _, err := collect(root); err == nil || !strings.Contains(err.Error(), "must not overlap") {
					t.Fatalf("%s accepted overlap with %s: %v", name, key, err)
				}
			}
		})
	}
	for _, cache := range []string{root, filepath.Dir(root), filepath.Join(root, PackageConfigFileName)} {
		config := fmt.Sprintf("hyperbricks:\n  directories:\n    cache: %q\n", cache)
		if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := collectModuleFiles(root); err == nil {
			t.Fatalf("unsafe cache directory accepted: %s", cache)
		}
	}
	// Unused default disk-cache settings must not reject older packages that
	// serve their module root as the static directory.
	config := fmt.Sprintf("hyperbricks:\n  directories:\n    static: %q\n", root)
	if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := collectModuleFiles(root); err != nil {
		t.Fatalf("legacy static directory rejected without explicit cache directory: %v", err)
	}
}

func TestArchiveCacheExclusionResolvesSymlinkedParent(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "module")
	if err := os.MkdirAll(filepath.Join(root, "generated-cache", "responses"), 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "parent-alias")
	if err := os.Symlink(base, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	config := "hyperbricks:\n  directories:\n    cache: {path: {base: module, path: generated-cache}}\n"
	if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, collect := range []func(string) ([]buildFile, error){collectModuleFiles, collectRuntimeSnapshotFiles} {
		files, err := collect(filepath.Join(alias, "module"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasPrefix(file.rel, "generated-cache") {
				t.Fatalf("cache leaked through symlinked parent: %s", file.rel)
			}
		}
		if len(files) != 1 || files[0].rel != PackageConfigFileName {
			t.Fatalf("source configuration omitted: %#v", files)
		}
	}
}

func TestArchiveCacheDirectoryInspectionDoesNotValidateUnrelatedRuntimeFields(t *testing.T) {
	root := t.TempDir()
	config := "hyperbricks:\n  directories:\n    cache: {path: {base: module, path: response-cache}}\n  development:\n    services:\n      - name: true\n        command: [false]\n"
	if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := collectModuleFiles(root)
	if err != nil || len(files) != 1 || files[0].rel != PackageConfigFileName {
		t.Fatalf("cache exclusion introduced unrelated runtime validation: %#v, %v", files, err)
	}
}
