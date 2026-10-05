package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupResponseCacheStoreTest(t testing.TB, maxBytes int64, maxEntries int) string {
	t.Helper()
	htmlCacheMutex.Lock()
	oldCache, oldState := htmlCache, responseCache
	htmlCache = make(map[string]CacheEntry)
	responseCache.directory = ""
	responseCache.root = ""
	responseCache.routes = make(map[string]uint64)
	responseCache.pendingRemovals = make(map[string]bool)
	responseCache.diskBytes = 0
	responseCache.diskEntries = 0
	htmlCacheMutex.Unlock()
	module := t.TempDir()
	if err := configureResponseCache(module, "", maxBytes, maxEntries); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeResponseCache(); err != nil {
			t.Error(err)
		}
		htmlCacheMutex.Lock()
		htmlCache, responseCache = oldCache, oldState
		htmlCacheMutex.Unlock()
	})
	return module
}

func responseCacheTestEntry(body string) CacheEntry {
	return CacheEntry{
		Content: body, ContentLength: strconv.Itoa(len(body)), ContentType: "application/json",
		ETag: `"example"`, Status: 202, Timestamp: time.Now(),
		Headers: map[string]string{"HX-Trigger": "updated", "Cache-Control": "no-store"},
		Cookies: []string{"test=one; HttpOnly"}, ErrorCount: 2,
	}
}

func storeResponseCacheTestEntry(t testing.TB, route, key, storage, body string) CacheEntry {
	t.Helper()
	entry := responseCacheTestEntry(body)
	if err := storeResponseCache(route, key, storage, entry, time.Hour, beginResponseCache(route)); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestResponseCacheStorageRoundTrip(t *testing.T) {
	for _, storage := range []string{"mem", "disk"} {
		t.Run(storage, func(t *testing.T) {
			module := setupResponseCacheStoreTest(t, 0, 0)
			if _, err := os.Stat(filepath.Join(module, ".cache")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("configuration eagerly created a cache directory: %v", err)
			}
			original := storeResponseCacheTestEntry(t, "page", "page?secret=confidential", storage, `{"message":"hello"}`)
			entry, hit := lookupResponseCache("page", "page?secret=confidential", time.Hour)
			if !hit {
				t.Fatal("expected cache hit")
			}
			if entry.ContentType != original.ContentType || entry.ContentLength != original.ContentLength || entry.Status != original.Status || entry.ETag != original.ETag || entry.Timestamp != original.Timestamp || entry.ErrorCount != original.ErrorCount || !reflect.DeepEqual(entry.Headers, original.Headers) || !reflect.DeepEqual(entry.Cookies, original.Cookies) {
				t.Fatalf("response metadata changed: %+v", entry)
			}
			if storage == "mem" {
				if entry.Content != original.Content || entry.Body != nil {
					t.Fatalf("unexpected memory entry: %+v", entry)
				}
				if _, err := os.Stat(filepath.Join(module, ".cache")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("memory cache created disk files")
				}
				return
			}
			defer entry.Body.Close()
			body, err := io.ReadAll(entry.Body)
			if err != nil || string(body) != original.Content {
				t.Fatalf("disk body=%q err=%v", body, err)
			}
			htmlCacheMutex.RLock()
			indexed := htmlCache["page?secret=confidential"]
			htmlCacheMutex.RUnlock()
			if indexed.Content != "" || indexed.Body != nil || entry.Content != "" {
				t.Fatal("disk body also retained in memory")
			}
			if strings.Contains(indexed.cacheDiskPath, "confidential") || !strings.HasPrefix(indexed.cacheDiskPath, filepath.Join(module, ".cache", "responses")+string(os.PathSeparator)) {
				t.Fatalf("unexpected disk location %q", indexed.cacheDiskPath)
			}
			for path, want := range map[string]os.FileMode{indexed.cacheDiskPath: 0600, filepath.Dir(indexed.cacheDiskPath): 0700} {
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != want {
					t.Fatalf("cache permissions for %s: info=%v err=%v", path, info, err)
				}
			}
		})
	}
}

func TestResponseCacheFixedExpiryAndCleanup(t *testing.T) {
	setupResponseCacheStoreTest(t, 0, 0)
	for _, storage := range []string{"mem", "disk"} {
		entry := storeResponseCacheTestEntry(t, storage, storage, storage, "response")
		hit, ok := lookupResponseCache(storage, storage, time.Hour)
		if !ok || hit.Timestamp != entry.Timestamp {
			t.Fatal("cache hit extended the lifetime")
		}
		if hit.Body != nil {
			hit.Body.Close()
		}
	}
	cleanupResponseCache(time.Now().Add(2 * time.Hour))
	htmlCacheMutex.RLock()
	entries, bytes := len(htmlCache), responseCache.diskBytes
	htmlCacheMutex.RUnlock()
	if entries != 0 || bytes != 0 {
		t.Fatalf("expired entries remain: entries=%d diskBytes=%d", entries, bytes)
	}
	for _, storage := range []string{"mem", "disk"} {
		entry := responseCacheTestEntry("stale")
		entry.Timestamp = time.Now().Add(-time.Hour)
		if err := storeResponseCache(storage, storage, storage, entry, time.Second, beginResponseCache(storage)); err != nil {
			t.Fatal(err)
		}
		if _, ok := lookupResponseCache(storage, storage, time.Hour); ok {
			t.Fatal("render that exceeded TTL was cached")
		}
		storeResponseCacheTestEntry(t, storage, storage, storage, "fresh")
		htmlCacheMutex.Lock()
		entry = htmlCache[storage]
		entry.Timestamp = time.Now().Add(-time.Minute)
		htmlCache[storage] = entry
		htmlCacheMutex.Unlock()
		if _, ok := lookupResponseCache(storage, storage, time.Second); ok {
			t.Fatal("expired entry was served before periodic cleanup")
		}
	}
}

func TestResponseCachePurgeVariantsAndInFlightWrites(t *testing.T) {
	setupResponseCacheStoreTest(t, 0, 0)
	storeResponseCacheTestEntry(t, "page", "page?one", "mem", "one")
	storeResponseCacheTestEntry(t, "page", "page?two", "disk", "two")
	storeResponseCacheTestEntry(t, "other", "other", "disk", "other")
	openEntry, ok := lookupResponseCache("page", "page?two", time.Hour)
	if !ok {
		t.Fatal("expected disk hit")
	}
	defer openEntry.Body.Close()
	oldToken := beginResponseCache("page")
	otherToken := beginResponseCache("other")
	result := purgeResponseCache("page")
	if result.MemoryEntries != 1 || result.DiskEntries != 1 || result.Bytes != 6 {
		t.Fatalf("incorrect purge result %+v", result)
	}
	body, err := io.ReadAll(openEntry.Body)
	if err != nil || string(body) != "two" {
		t.Fatalf("purge interrupted an already-open response: body=%q error=%v", body, err)
	}
	for _, storage := range []string{"mem", "disk"} {
		if err := storeResponseCache("page", "page?late", storage, responseCacheTestEntry("late"), time.Hour, oldToken); err != nil {
			t.Fatal(err)
		}
		if _, hit := lookupResponseCache("page", "page?late", time.Hour); hit {
			t.Fatal("pre-purge render repopulated cache")
		}
	}
	if err := storeResponseCache("other", "other?new", "mem", responseCacheTestEntry("valid"), time.Hour, otherToken); err != nil {
		t.Fatal(err)
	}
	if _, hit := lookupResponseCache("other", "other?new", time.Hour); !hit {
		t.Fatal("route purge invalidated a different route's render")
	}
	purgeResponseCache("")
	if err := storeResponseCache("other", "other?late", "disk", responseCacheTestEntry("late"), time.Hour, otherToken); err != nil {
		t.Fatal(err)
	}
	if _, hit := lookupResponseCache("other", "other?late", time.Hour); hit {
		t.Fatal("pre-reset render repopulated cache")
	}
}

func TestResponseCacheDiskBudget(t *testing.T) {
	for _, limit := range []struct {
		name    string
		bytes   int64
		entries int
	}{{"bytes", 6, 100}, {"entries", 100, 2}} {
		t.Run(limit.name, func(t *testing.T) {
			setupResponseCacheStoreTest(t, limit.bytes, limit.entries)
			for i, key := range []string{"oldest", "middle", "newest"} {
				entry := responseCacheTestEntry("123")
				entry.Timestamp = time.Now().Add(time.Duration(i-3) * time.Second)
				if err := storeResponseCache(key, key, "disk", entry, time.Hour, beginResponseCache(key)); err != nil {
					t.Fatal(err)
				}
			}
			if _, hit := lookupResponseCache("oldest", "oldest", time.Hour); hit {
				t.Fatal("budget did not evict oldest response")
			}
			htmlCacheMutex.RLock()
			count, bytes, directory := responseCache.diskEntries, responseCache.diskBytes, responseCache.directory
			htmlCacheMutex.RUnlock()
			files, err := filepath.Glob(filepath.Join(directory, "*.entry"))
			if err != nil || count != 2 || bytes != 6 || len(files) != 2 {
				t.Fatalf("budget index/files mismatch: entries=%d bytes=%d files=%v err=%v", count, bytes, files, err)
			}
		})
	}
	t.Run("expired first", func(t *testing.T) {
		setupResponseCacheStoreTest(t, 0, 2)
		storeResponseCacheTestEntry(t, "old", "old", "disk", "old")
		storeResponseCacheTestEntry(t, "expired", "expired", "disk", "expired")
		htmlCacheMutex.Lock()
		entry := htmlCache["expired"]
		entry.cacheTTL = time.Nanosecond
		htmlCache["expired"] = entry
		htmlCacheMutex.Unlock()
		storeResponseCacheTestEntry(t, "new", "new", "disk", "new")
		entry, hit := lookupResponseCache("old", "old", time.Hour)
		if !hit {
			t.Fatal("live oldest response evicted before expired response")
		}
		entry.Body.Close()
	})
}

func TestResponseCacheDiskFailuresRemainMisses(t *testing.T) {
	for _, failure := range []string{"missing", "truncated", "same-length corruption"} {
		t.Run(failure, func(t *testing.T) {
			setupResponseCacheStoreTest(t, 0, 0)
			storeResponseCacheTestEntry(t, "page", "page", "disk", "original")
			htmlCacheMutex.RLock()
			path := htmlCache["page"].cacheDiskPath
			htmlCacheMutex.RUnlock()
			var err error
			switch failure {
			case "missing":
				err = os.Remove(path)
			case "truncated":
				err = os.WriteFile(path, []byte("short"), 0600)
			default:
				err = os.WriteFile(path, []byte("corrupt!"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, hit := lookupResponseCache("page", "page", time.Hour); hit {
				t.Fatal("unusable disk response returned as a hit")
			}
			htmlCacheMutex.RLock()
			count := len(htmlCache)
			htmlCacheMutex.RUnlock()
			if count != 0 {
				t.Fatal("disk failure retained a memory fallback")
			}
		})
	}
	t.Run("unusable directory", func(t *testing.T) {
		module := setupResponseCacheStoreTest(t, 0, 0)
		if err := os.WriteFile(filepath.Join(module, ".cache"), []byte("not a directory"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := storeResponseCache("page", "page", "disk", responseCacheTestEntry("body"), time.Hour, beginResponseCache("page")); err == nil {
			t.Fatal("expected storage error")
		}
		if _, hit := lookupResponseCache("page", "page", time.Hour); hit {
			t.Fatal("disk failure fell back to memory")
		}
	})
	t.Run("oversized body", func(t *testing.T) {
		setupResponseCacheStoreTest(t, 3, 2)
		if err := storeResponseCache("page", "page", "disk", responseCacheTestEntry("large"), time.Hour, beginResponseCache("page")); err == nil {
			t.Fatal("expected entry size limit error")
		}
	})
	t.Run("private subtree symlink", func(t *testing.T) {
		module := setupResponseCacheStoreTest(t, 0, 0)
		public := filepath.Join(module, "static")
		cache := filepath.Join(module, ".cache")
		for _, directory := range []string{public, cache} {
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(public, filepath.Join(cache, "responses")); err != nil {
			t.Fatal(err)
		}
		if err := storeResponseCache("page", "page", "disk", responseCacheTestEntry("private"), time.Hour, beginResponseCache("page")); err == nil {
			t.Fatal("cache followed a symlink into public files")
		}
		entries, err := os.ReadDir(public)
		if err != nil || len(entries) != 0 {
			t.Fatalf("response files leaked into public directory: entries=%v err=%v", entries, err)
		}
	})
}

func TestResponseCacheNamespaceOwnershipAndShutdown(t *testing.T) {
	module := setupResponseCacheStoreTest(t, 0, 0)
	parent := filepath.Join(module, ".cache", "responses")
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for name, owner := range map[string]responseCacheOwner{
		"runtime-active":  {Version: 1, PID: os.Getpid(), Host: host},
		"runtime-dead":    {Version: 1, PID: 2147483647, Host: host},
		"runtime-foreign": {Version: 1, PID: 2147483647, Host: host + "-different"},
	} {
		directory := filepath.Join(parent, name)
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(owner)
		if err := os.WriteFile(filepath.Join(directory, "owner.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	storeResponseCacheTestEntry(t, "page", "page", "disk", "body")
	if _, err := os.Stat(filepath.Join(parent, "runtime-dead")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned namespace retained: %v", err)
	}
	for _, name := range []string{"runtime-active", "runtime-foreign"} {
		if _, err := os.Stat(filepath.Join(parent, name)); err != nil {
			t.Fatalf("namespace %s was incorrectly reaped: %v", name, err)
		}
	}
	htmlCacheMutex.RLock()
	directory := responseCache.directory
	htmlCacheMutex.RUnlock()
	if err := closeResponseCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned namespace survives shutdown: %v", err)
	}
	if err := configureResponseCache(module, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, hit := lookupResponseCache("page", "page", time.Hour); hit {
		t.Fatal("cache survived startup")
	}
}

func TestResponseCacheFailedRemovalSuspendsDiskWrites(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can unlink files despite the directory permission test")
	}
	setupResponseCacheStoreTest(t, 4, 1)
	storeResponseCacheTestEntry(t, "old", "old", "disk", "1234")
	htmlCacheMutex.RLock()
	directory := responseCache.directory
	htmlCacheMutex.RUnlock()
	if err := os.Chmod(directory, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0700) })
	purgeResponseCache("old")
	if err := storeResponseCache("new", "new", "disk", responseCacheTestEntry("5678"), time.Hour, beginResponseCache("new")); err == nil || !strings.Contains(err.Error(), "awaiting cleanup") {
		t.Fatalf("failed cleanup did not suspend new disk writes: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.entry"))
	if err != nil || len(files) != 1 {
		t.Fatalf("unlinked data was ignored for the disk budget: files=%v err=%v", files, err)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	cleanupResponseCache(time.Now())
	storeResponseCacheTestEntry(t, "new", "new", "disk", "5678")
	entry, hit := lookupResponseCache("new", "new", time.Hour)
	if !hit {
		t.Fatal("disk writes did not resume after cleanup recovered")
	}
	entry.Body.Close()
}

func TestResponseCacheConcurrentPublishPurgeAndCleanup(t *testing.T) {
	setupResponseCacheStoreTest(t, 4096, 8)
	var workers sync.WaitGroup
	for i := range 4 {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for iteration := range 20 {
				key := strconv.Itoa(worker) + "/" + strconv.Itoa(iteration)
				if err := storeResponseCache("page", key, "disk", responseCacheTestEntry("body"), time.Minute, beginResponseCache("page")); err != nil {
					t.Error(err)
				}
				if entry, hit := lookupResponseCache("page", key, time.Minute); hit {
					body, err := io.ReadAll(entry.Body)
					entry.Body.Close()
					if err != nil || string(body) != "body" {
						t.Errorf("partial response: body=%q err=%v", body, err)
					}
				}
				if iteration%3 == 0 {
					purgeResponseCache("page")
				}
				if iteration%5 == 0 {
					cleanupResponseCache(time.Now())
				}
			}
		}(i)
	}
	workers.Wait()
	purgeResponseCache("")
	htmlCacheMutex.RLock()
	count, bytes := len(htmlCache), responseCache.diskBytes
	htmlCacheMutex.RUnlock()
	if count != 0 || bytes != 0 {
		t.Fatalf("purge left entries=%d bytes=%d", count, bytes)
	}
}

func TestResponseCacheCleanupStopsWithContext(t *testing.T) {
	setupResponseCacheStoreTest(t, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runResponseCacheCleanup(ctx, time.Millisecond)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup worker ignored cancellation")
	}
}

func BenchmarkResponseCacheHit(b *testing.B) {
	for _, storage := range []string{"mem", "disk"} {
		for _, size := range []int{1024, 65536, 1048576} {
			b.Run(storage+"/"+strconv.Itoa(size), func(b *testing.B) {
				setupResponseCacheStoreTest(b, 0, 0)
				storeResponseCacheTestEntry(b, "page", "page", storage, strings.Repeat("x", size))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					entry, hit := lookupResponseCache("page", "page", time.Hour)
					if !hit {
						b.Fatal("expected a cache hit")
					}
					if entry.Body != nil {
						if _, err := io.Copy(io.Discard, entry.Body); err != nil {
							b.Fatal(err)
						}
						entry.Body.Close()
					}
				}
			})
		}
	}
}
