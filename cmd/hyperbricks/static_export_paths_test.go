package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestStaticExportRejectsOverlappingRootsBeforeDeletion(t *testing.T) {
	root := t.TempDir()
	previousRoot, force := commands.ModuleRoot, commands.ForceStatic
	commands.ModuleRoot, commands.ForceStatic = root, true
	t.Cleanup(func() { commands.ModuleRoot, commands.ForceStatic = previousRoot, force })
	for _, name := range []string{"static", "hyperbricks", "templates", "resources"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	sentinel := filepath.Join(root, "static", "keep.js")
	if err := os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"static", "static/export", "resources", "resources/export", "."} {
		t.Run(relative, func(t *testing.T) {
			config := &shared.Config{Directories: map[string]string{
				"render": filepath.Join(root, relative), "static": filepath.Join(root, "static"),
				"resources": filepath.Join(root, "resources"),
			}}
			if _, err := planStaticExport(config); err == nil {
				t.Fatal("overlapping export was accepted")
			}
			if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
				t.Fatalf("source changed during rejected planning: %q %v", data, err)
			}
		})
	}
	config := &shared.Config{Directories: map[string]string{
		"render": filepath.Join(root, "rendered"), "static": filepath.Join(root, "static"),
		"resources": filepath.Join(root, "resources"),
	}}
	if _, err := planStaticExport(config); err != nil {
		t.Fatalf("separate output was rejected: %v", err)
	}
	config.Directories["resources"] = filepath.Join(root, "rendered", "inputs")
	if _, err := planStaticExport(config); err == nil {
		t.Fatal("export containing its inputs was accepted")
	}
}

func TestStaticExportRejectsCacheOverlapBeforeHooksAndDeletion(t *testing.T) {
	for _, paths := range []struct{ name, cache, render string }{
		{"same directory", "cache", "cache"},
		{"export inside cache", "cache", "cache/export"},
		{"cache inside export", "rendered/cache", "rendered"},
	} {
		t.Run(paths.name, func(t *testing.T) {
			root := t.TempDir()
			previousRoot := commands.ModuleRoot
			commands.ModuleRoot = root
			t.Cleanup(func() { commands.ModuleRoot = previousRoot })
			configureStaticExportTest(t)
			commands.StaticWithProcesses = true
			config := &shared.Config{Directories: map[string]string{
				"static": filepath.Join(root, "static"), "cache": filepath.Join(root, paths.cache), "render": filepath.Join(root, paths.render),
			}}
			config.Hooks.BeforeStatic = []shared.DevelopmentTaskConfig{{Name: "unexpected", Command: []string{"sh", "-c", "touch hook-ran"}, Timeout: time.Second}}
			sentinel := filepath.Join(config.Directories["cache"], "live-response.cache")
			writeTestFile(t, sentinel, "active response cache")
			if err := runStaticOperation(config); err == nil || !strings.Contains(err.Error(), "overlaps cache") {
				t.Fatalf("cache overlap was accepted: %v", err)
			}
			if data, err := os.ReadFile(sentinel); err != nil || string(data) != "active response cache" {
				t.Fatalf("live response cache was changed: %q %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(root, "hook-ran")); !os.IsNotExist(err) {
				t.Fatalf("hook ran before cache-path validation: %v", err)
			}
		})
	}
}

func TestStaticExportRejectsSymlinkEquivalentRoots(t *testing.T) {
	root := t.TempDir()
	previousRoot, force := commands.ModuleRoot, commands.ForceStatic
	commands.ModuleRoot, commands.ForceStatic = root, true
	t.Cleanup(func() { commands.ModuleRoot, commands.ForceStatic = previousRoot, force })
	static := filepath.Join(root, "static")
	if err := os.MkdirAll(static, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(static, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, output := range []string{alias, filepath.Join(alias, "new-output")} {
		config := &shared.Config{Directories: map[string]string{"render": output, "static": static}}
		if _, err := planStaticExport(config); err == nil {
			t.Fatalf("symlink-equivalent output accepted: %s", output)
		}
	}
	outside := t.TempDir()
	externalAlias := filepath.Join(root, "outside")
	if err := os.Symlink(outside, externalAlias); err != nil {
		t.Fatal(err)
	}
	config := &shared.Config{Directories: map[string]string{"render": filepath.Join(externalAlias, "new-output"), "static": static}}
	if _, err := planStaticExport(config); err == nil {
		t.Fatal("symlink escaping the module was accepted")
	}
}

func TestStaticExportRejectsReplaceableOutputSymlinksBeforeHooks(t *testing.T) {
	for _, target := range []string{"rendered", "rendered/static", "rendered/static/images"} {
		t.Run(target, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			previousRoot, force, processes := commands.ModuleRoot, commands.ForceStatic, commands.StaticWithProcesses
			commands.ModuleRoot, commands.ForceStatic, commands.StaticWithProcesses = root, true, true
			t.Cleanup(func() {
				commands.ModuleRoot, commands.ForceStatic, commands.StaticWithProcesses = previousRoot, force, processes
			})
			link := filepath.Join(root, filepath.FromSlash(target))
			if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			sentinel := filepath.Join(outside, "keep")
			if err := os.WriteFile(sentinel, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			config := &shared.Config{Directories: map[string]string{"render": filepath.Join(root, "rendered"), "static": filepath.Join(root, "static")},
				Hooks: shared.LifecycleHooksConfig{BeforeStatic: []shared.DevelopmentTaskConfig{{Name: "unexpected", Command: []string{"sh", "-c", "touch hook-ran"}, Timeout: time.Second}}}}
			if err := runStaticOperation(config); err == nil {
				t.Fatal("replaceable output symlink accepted")
			}
			if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
				t.Fatalf("symlink target changed: %q %v", data, err)
			}
			if _, err := os.Lstat(link); err != nil {
				t.Fatalf("output symlink was changed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "hook-ran")); !os.IsNotExist(err) {
				t.Fatalf("hook ran before symlink validation: %v", err)
			}
		})
	}
}
