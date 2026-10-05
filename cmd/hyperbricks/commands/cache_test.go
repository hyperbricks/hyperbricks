package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func privateCacheControlTestHome(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("Unix cache control requires macOS or Linux")
	}
	// Darwin limits Unix socket paths to 104 bytes; Go's ordinary test temp
	// directory can exceed that before the socket name is appended.
	home, err := os.MkdirTemp("/tmp", "hbcc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
}

func TestCacheControlPurgeRoundTripAndShutdown(t *testing.T) {
	privateCacheControlTestHome(t)
	module := t.TempDir()
	routes := make(chan string, 4)
	want := CachePurgeResult{MemoryEntries: 2, DiskEntries: 3, Bytes: 4096}
	stop, err := StartCacheControl(module, func(route string) (CachePurgeResult, error) {
		routes <- route
		return want, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(context.Background()) })
	got, instance, err := purgeRunningCache(context.Background(), module, "", cachePurgeRequest{Route: "/products/"})
	if err != nil || got != want || instance == "" {
		t.Fatalf("route purge = %#v, %q, %v", got, instance, err)
	}
	if route := <-routes; route != "products" {
		t.Fatalf("route = %q, want products", route)
	}
	got, _, err = purgeRunningCache(context.Background(), module, instance, cachePurgeRequest{All: true})
	if err != nil || got != want {
		t.Fatalf("all purge = %#v, %v", got, err)
	}
	if route := <-routes; route != "" {
		t.Fatalf("all purge route = %q", route)
	}
	_, directory, err := cacheControlDirectory(module, false)
	if err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{directory: 0o700, filepath.Join(directory, instance+".sock"): 0o600} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("permissions for %s = %v, %v", path, info, err)
		}
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(directory, instance+".sock")); !os.IsNotExist(err) {
		t.Fatalf("shutdown did not remove socket: %v", err)
	}
	if _, _, err := purgeRunningCache(context.Background(), module, "", cachePurgeRequest{All: true}); err == nil || !strings.Contains(err.Error(), "no running") {
		t.Fatalf("purge after shutdown = %v", err)
	}
}

func TestCacheControlSelectsModuleAndInstance(t *testing.T) {
	privateCacheControlTestHome(t)
	module := t.TempDir()
	var selected []string
	var selectedMutex sync.Mutex
	start := func(root, label string) {
		t.Helper()
		stop, err := StartCacheControl(root, func(route string) (CachePurgeResult, error) {
			selectedMutex.Lock()
			selected = append(selected, label)
			selectedMutex.Unlock()
			return CachePurgeResult{}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = stop(context.Background()) })
	}
	start(module, "first")
	_, first, err := purgeRunningCache(context.Background(), module, "", cachePurgeRequest{All: true})
	if err != nil {
		t.Fatal(err)
	}
	start(module, "second")
	start(t.TempDir(), "different-module")
	if _, _, err := purgeRunningCache(context.Background(), module, "", cachePurgeRequest{All: true}); err == nil || !strings.Contains(err.Error(), "multiple running") || !strings.Contains(err.Error(), first) {
		t.Fatalf("ambiguous selection = %v", err)
	}
	if _, _, err := purgeRunningCache(context.Background(), module, "missing", cachePurgeRequest{All: true}); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("missing instance = %v", err)
	}
	if _, _, err := purgeRunningCache(context.Background(), module, first, cachePurgeRequest{All: true}); err != nil {
		t.Fatal(err)
	}
	selectedMutex.Lock()
	defer selectedMutex.Unlock()
	if got := strings.Join(selected, ","); got != "first,first" {
		t.Fatalf("purged instances = %q", got)
	}
}

func TestCacheCommandValidatesScopeAndReportsResult(t *testing.T) {
	privateCacheControlTestHome(t)
	module := t.TempDir()
	stop, err := StartCacheControl(module, func(route string) (CachePurgeResult, error) {
		if route == "missing" {
			return CachePurgeResult{}, fmt.Errorf("route %q is not configured", route)
		}
		return CachePurgeResult{MemoryEntries: 4, DiskEntries: 2, Bytes: 500}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(context.Background()) })
	for _, args := range [][]string{
		{}, {"--all", "--route", "products"}, {"--route", ""}, {"--route", "../products"},
		{"--route", "products?lang=nl"}, {"--route", "https://example.com/products"}, {"--route", "missing"},
	} {
		cmd := NewCacheCommand()
		cmd.SetArgs(append([]string{"purge", "--module", module}, args...))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("args %v unexpectedly succeeded", args)
		}
	}
	cmd := NewCacheCommand()
	cmd.SetArgs([]string{"purge", "--module", module, "--all"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "4 memory entries and 2 disk entries (500 bytes)") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestCacheControlRejectsMalformedRequestsAndUnsafeDirectory(t *testing.T) {
	privateCacheControlTestHome(t)
	module := t.TempDir()
	stop, err := StartCacheControl(module, func(route string) (CachePurgeResult, error) {
		t.Error("invalid request reached purge callback")
		return CachePurgeResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(context.Background()) })
	_, directory, _ := cacheControlDirectory(module, false)
	sockets, _ := filepath.Glob(filepath.Join(directory, "*.sock"))
	client := cacheControlClient(sockets[0])
	for _, body := range []string{`{}`, `{"all":true,"route":"products"}`, `{"all":true,"extra":true}`, `{"all":true} {"all":true}`, strings.Repeat("x", 5000)} {
		response, err := client.Post("http://local/v1/purge", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("body %q status = %d", body, response.StatusCode)
		}
	}
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := StartCacheControl(module, func(string) (CachePurgeResult, error) { return CachePurgeResult{}, nil }); err == nil || !strings.Contains(err.Error(), "private permissions") {
		t.Fatalf("unsafe directory = %v", err)
	}
}
