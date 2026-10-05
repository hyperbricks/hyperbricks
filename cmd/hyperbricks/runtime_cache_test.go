package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestResponseCacheDirectoryIsPrivate(t *testing.T) {
	module := t.TempDir()
	config := &shared.Config{Directories: map[string]string{}}
	got, err := responseCacheDirectory(module, config)
	want, _ := canonicalCachePath(filepath.Join(module, ".cache"))
	if err != nil || got != want {
		t.Fatalf("default directory = %q, %v; want %q", got, err, want)
	}
	for _, path := range []string{module, filepath.Dir(module), filepath.Join(module, "static", "cache"), filepath.Join(module, "rendered"), filepath.Join(module, "resources", "cache"), filepath.Join(module, "templates", "cache"), filepath.Join(module, "hyperbricks", "cache")} {
		config.Directories["cache"] = path
		if _, err := responseCacheDirectory(module, config); err == nil {
			t.Errorf("unsafe directory accepted: %s", path)
		}
	}
	external := t.TempDir()
	config.Directories["cache"] = filepath.Join(external, "responses-private")
	if _, err := responseCacheDirectory(module, config); err != nil {
		t.Fatalf("dedicated external cache rejected: %v", err)
	}
	static := filepath.Join(module, "static")
	if err := os.MkdirAll(static, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(module, "private-alias")
	if err := os.Symlink(static, alias); err != nil {
		t.Fatal(err)
	}
	config.Directories["cache"] = filepath.Join(alias, "not-yet-created")
	if _, err := responseCacheDirectory(module, config); err == nil {
		t.Fatal("symlink into public static directory was accepted")
	}
}

func TestRuntimeWatcherIgnoresResponseCache(t *testing.T) {
	module := t.TempDir()
	custom := filepath.Join(module, "scratch")
	standard := filepath.Join(module, ".cache")
	for _, path := range []string{standard, custom} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	changed := make(chan struct{}, 8)
	done, err := startRuntimeWatcher(ctx, []string{module}, func() bool { changed <- struct{}{}; return true }, custom)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("cache watcher did not stop")
		}
	})
	for _, directory := range []string{standard, custom} {
		if err := os.WriteFile(filepath.Join(directory, "entry"), []byte("cached"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-changed:
		t.Fatal("cache write triggered a source reload")
	case <-time.After(700 * time.Millisecond):
	}
	if err := os.WriteFile(filepath.Join(module, "source.yaml"), []byte("page: changed"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("source write failed to trigger reload")
	}
}
