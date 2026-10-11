package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
)

func TestRuntimeAssetCleanupWaitsForRequestsAndCachedHTML(t *testing.T) {
	setupLiveModeServeContentTest(t)
	root := t.TempDir()
	static := filepath.Join(root, "static")
	if err := os.MkdirAll(static, 0755); err != nil {
		t.Fatal(err)
	}
	config := getHyperBricksConfiguration()
	oldDirs := config.Directories
	config.Directories = map[string]string{"static": static}
	t.Cleanup(func() { config.Directories = oldDirs })
	renderer := component.NewEsbuildRenderer(static)
	renderer.SetAssetProtection(protectRuntimeAsset)
	rm.RegisterComponent(component.EsbuildConfigGetName(), renderer, reflect.TypeOf(component.EsbuildConfig{}))
	cfg := component.EsbuildConfig{Entry: filepath.Join(root, "main.js"), Outfile: filepath.Join(static, "js", "app.js"), Cache: true, Fingerprint: true}
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(cfg.Entry, []byte("console.log("+value+");"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	build := func() string {
		t.Helper()
		url, failures := renderer.Prepare(cfg).Render(context.Background())
		if len(failures) > 0 {
			t.Fatal(failures)
		}
		return url
	}
	write("1")
	oldURL := build()
	oldPath := filepath.Join(static, filepath.FromSlash(strings.TrimPrefix(oldURL, "/static/")))
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	handler := assetRequestMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; w.Write([]byte("ok")) }))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	write("2")
	var newURL string
	assetRequestMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { newURL = build(); w.Write([]byte("ok")) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !renderer.HasPendingAssetCleanup() {
		t.Fatal("protected obsolete asset not pending")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("active request lost its asset", err)
	}
	htmlCacheMutex.Lock()
	htmlCache["protected"] = CacheEntry{Content: `<script src="` + oldURL + `"></script>`}
	htmlCacheMutex.Unlock()
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request cleanup blocked")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("cached HTML lost its asset", err)
	}
	if !renderer.HasPendingAssetCleanup() {
		t.Fatal("cached reference stopped deferred cleanup")
	}
	htmlCacheMutex.Lock()
	delete(htmlCache, "protected")
	htmlCacheMutex.Unlock()
	ordinary := assetRequestMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	ordinary.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("obsolete asset remains: %v", err)
	}
	newPath := filepath.Join(static, filepath.FromSlash(strings.TrimPrefix(newURL, "/static/")))
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("current asset removed", err)
	}
	if renderer.HasPendingAssetCleanup() {
		t.Fatal("completed cleanup still pending")
	}
	// Subsequent ordinary requests must not rewrite the record.
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(static)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cache, "hyperbricks", "esbuild", fmt.Sprintf("%x.generations.json", sha256.Sum256([]byte(resolved))))
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".lock") })
	stamp := time.Unix(946684800, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	ordinary.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("ordinary request rewrote history")
	}
}
