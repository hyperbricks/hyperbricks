package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

const (
	defaultResponseCacheMaxBytes   int64 = 256 << 20
	defaultResponseCacheMaxEntries       = 10000
)

// htmlCache is the sole response index. Disk entries retain response metadata
// there, but their bodies exist only in private, immutable files.
var responseCache = struct {
	generation      uint64
	routes          map[string]uint64
	root            string
	directory       string
	maxBytes        int64
	maxEntries      int
	diskBytes       int64
	diskEntries     int
	pendingRemovals map[string]bool
}{routes: make(map[string]uint64), pendingRemovals: make(map[string]bool), maxBytes: defaultResponseCacheMaxBytes, maxEntries: defaultResponseCacheMaxEntries}

// Serializes disk publication, maintenance, and configuration, never response
// streaming. Index access and generation counters use htmlCacheMutex only.
var responseCacheDiskMutex sync.Mutex

var responseCacheReadBuffers = sync.Pool{New: func() any {
	buffer := make([]byte, 32*1024)
	return &buffer
}}

// Reuse the same bounded scratch buffers for verification and HTTP delivery.
// Hiding File.WriteTo avoids its allocating fallback for generic writers.
func copyResponseCacheBody(writer io.Writer, reader io.Reader) (int64, error) {
	buffer := responseCacheReadBuffers.Get().(*[]byte)
	defer responseCacheReadBuffers.Put(buffer)
	return io.CopyBuffer(writer, struct{ io.Reader }{reader}, *buffer)
}

type cacheWriteToken struct {
	generation uint64
	route      string
	routeEpoch uint64
}

type CachePurgeResult struct {
	MemoryEntries int   `json:"memory_entries"`
	DiskEntries   int   `json:"disk_entries"`
	Bytes         int64 `json:"bytes"`
}

type responseCacheOwner struct {
	Version int    `json:"version"`
	PID     int    `json:"pid"`
	Host    string `json:"host"`
}

func beginResponseCache(route string) cacheWriteToken {
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	return cacheWriteToken{responseCache.generation, route, responseCache.routes[route]}
}

func validResponseCacheTokenLocked(route string, token cacheWriteToken) bool {
	return token.route == route && token.generation == responseCache.generation && token.routeEpoch == responseCache.routes[route]
}

// A hit owns Body until the caller closes it. Verifying a disk body's digest
// before committing HTTP headers makes damaged files a recoverable cache miss.
// Both verification and serving stream the file without retaining a RAM copy.
func lookupResponseCache(route, key string, ttl time.Duration) (CacheEntry, bool) {
	if ttl <= 0 {
		return CacheEntry{}, false
	}
	htmlCacheMutex.RLock()
	entry, ok := htmlCache[key]
	htmlCacheMutex.RUnlock()
	if !ok || (entry.cacheRoute != "" && entry.cacheRoute != route) {
		return CacheEntry{}, false
	}
	if time.Since(entry.Timestamp) > ttl {
		discardResponseCacheEntry(key, entry)
		return CacheEntry{}, false
	}
	if entry.cacheDiskPath == "" {
		return entry, true
	}

	file, err := os.Open(entry.cacheDiskPath)
	if err == nil {
		var info os.FileInfo
		info, err = file.Stat()
		if err == nil && (!info.Mode().IsRegular() || info.Size() != entry.cacheDiskSize) {
			err = errors.New("cached response length or file type changed")
		}
		if err == nil {
			hash := sha256.New()
			_, err = copyResponseCacheBody(hash, file)
			if err == nil && !equalResponseCacheDigest(hash.Sum(nil), entry.cacheDiskDigest) {
				err = errors.New("cached response checksum changed")
			}
			if err == nil {
				_, err = file.Seek(0, io.SeekStart)
			}
		}
	}
	if err != nil {
		if file != nil {
			_ = file.Close()
		}
		if discardResponseCacheEntry(key, entry) {
			logging.GetLogger().Warnw("Response disk cache miss", "route", route, "error", err)
		}
		return CacheEntry{}, false
	}

	// Purge may have completed while opening/verifying this file. A reader
	// already returned before purge may finish, but do not start another hit.
	htmlCacheMutex.RLock()
	current, present := htmlCache[key]
	valid := present && current.cacheDiskPath == entry.cacheDiskPath
	htmlCacheMutex.RUnlock()
	if !valid || time.Since(entry.Timestamp) > ttl {
		_ = file.Close()
		return CacheEntry{}, false
	}
	entry.Body = file
	return entry, true
}

func equalResponseCacheDigest(actual []byte, expected [32]byte) bool {
	return bytes.Equal(actual, expected[:])
}

func storeResponseCache(route, key, storage string, entry CacheEntry, ttl time.Duration, token cacheWriteToken) error {
	if ttl <= 0 || time.Since(entry.Timestamp) > ttl {
		return nil
	}
	if storage != "mem" && storage != "disk" {
		return fmt.Errorf("unsupported response cache storage %q", storage)
	}
	entry.cacheRoute = route
	entry.cacheTTL = ttl
	entry.Body = nil
	entry.Headers = cloneResponseCacheHeaders(entry.Headers)
	entry.Cookies = append([]string(nil), entry.Cookies...)
	entry.cacheDiskPath = ""
	entry.cacheDiskSize = 0

	if storage == "mem" {
		htmlCacheMutex.Lock()
		if !validResponseCacheTokenLocked(route, token) {
			htmlCacheMutex.Unlock()
			return nil
		}
		oldPath := removeResponseCacheEntryLocked(key)
		htmlCache[key] = entry
		htmlCacheMutex.Unlock()
		removeResponseCacheFiles([]string{oldPath})
		return nil
	}

	responseCacheDiskMutex.Lock()
	defer responseCacheDiskMutex.Unlock()
	htmlCacheMutex.RLock()
	valid := validResponseCacheTokenLocked(route, token)
	maxBytes := responseCache.maxBytes
	htmlCacheMutex.RUnlock()
	if !valid {
		return nil
	}
	if int64(len(entry.Content)) > maxBytes {
		return fmt.Errorf("response body exceeds disk cache limit of %d bytes", maxBytes)
	}
	if err := retryResponseCacheRemovals(); err != nil {
		return err
	}
	directory, err := ensureResponseCacheDirectory()
	if err != nil {
		return err
	}
	// Reclaim space before writing, including temporary bytes. If a failed
	// unlink left an orphan, fresh output remains available but disk writes
	// stop until cleanup succeeds; logical eviction alone cannot bound disk.
	htmlCacheMutex.Lock()
	if !validResponseCacheTokenLocked(route, token) {
		htmlCacheMutex.Unlock()
		return nil
	}
	paths := []string{removeResponseCacheEntryLocked(key)}
	paths = append(paths, enforceResponseCacheBudgetLocked(time.Now(), int64(len(entry.Content)), 1)...)
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles(paths)
	if err := retryResponseCacheRemovals(); err != nil {
		return err
	}
	keyHash := sha256.Sum256([]byte(key))
	file, err := os.CreateTemp(directory, hex.EncodeToString(keyHash[:])+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create response cache entry: %w", err)
	}
	temporaryPath := file.Name()
	defer removeResponseCacheFiles([]string{temporaryPath})
	_, writeErr := io.Copy(file, strings.NewReader(entry.Content))
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("write response cache entry: %w", err)
	}
	path := strings.TrimSuffix(temporaryPath, ".tmp") + ".entry"
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish response cache file: %w", err)
	}
	entry.cacheDiskPath = path
	entry.cacheDiskSize = int64(len(entry.Content))
	entry.cacheDiskDigest = sha256.Sum256([]byte(entry.Content))
	entry.Content = ""

	htmlCacheMutex.Lock()
	if !validResponseCacheTokenLocked(route, token) || time.Since(entry.Timestamp) > ttl {
		htmlCacheMutex.Unlock()
		removeResponseCacheFiles([]string{path})
		return nil
	}
	paths = []string{removeResponseCacheEntryLocked(key)}
	htmlCache[key] = entry
	responseCache.diskBytes += entry.cacheDiskSize
	responseCache.diskEntries++
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles(paths)
	return nil
}

func cloneResponseCacheHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

// Caller holds htmlCacheMutex. Removing an index entry immediately invalidates
// it; immutable file names let cleanup happen afterward without deleting a new
// response that replaced the same request key.
func removeResponseCacheEntryLocked(key string) string {
	entry, found := htmlCache[key]
	if !found {
		return ""
	}
	delete(htmlCache, key)
	if entry.cacheDiskPath != "" {
		responseCache.diskBytes -= entry.cacheDiskSize
		responseCache.diskEntries--
		responseCache.pendingRemovals[entry.cacheDiskPath] = true
	}
	return entry.cacheDiskPath
}

func discardResponseCacheEntry(key string, observed CacheEntry) bool {
	htmlCacheMutex.Lock()
	entry, ok := htmlCache[key]
	var path string
	removed := ok && entry.cacheDiskPath == observed.cacheDiskPath && entry.Timestamp == observed.Timestamp
	if removed {
		path = removeResponseCacheEntryLocked(key)
	}
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles([]string{path})
	return removed
}

func purgeResponseCache(route string) CachePurgeResult {
	htmlCacheMutex.Lock()
	if route == "" {
		responseCache.generation++
		responseCache.routes = make(map[string]uint64)
	} else {
		responseCache.routes[route]++
	}
	result, paths := purgeResponseCacheEntriesLocked(route)
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles(paths)
	return result
}

func purgeResponseCacheEntriesLocked(route string) (CachePurgeResult, []string) {
	var result CachePurgeResult
	var paths []string
	for key, entry := range htmlCache {
		if route != "" && entry.cacheRoute != route {
			continue
		}
		if entry.cacheDiskPath == "" {
			result.MemoryEntries++
			result.Bytes += int64(len(entry.Content))
		} else {
			result.DiskEntries++
			result.Bytes += entry.cacheDiskSize
		}
		paths = append(paths, removeResponseCacheEntryLocked(key))
	}
	return result, paths
}

func configureResponseCache(moduleRoot, cacheRoot string, maxBytes int64, maxEntries int) error {
	if cacheRoot == "" {
		cacheRoot = filepath.Join(moduleRoot, ".cache")
	} else if !filepath.IsAbs(cacheRoot) {
		cacheRoot = filepath.Join(moduleRoot, cacheRoot)
	}
	root, err := filepath.Abs(cacheRoot)
	if err != nil {
		return fmt.Errorf("resolve response cache directory: %w", err)
	}
	if maxBytes <= 0 {
		maxBytes = defaultResponseCacheMaxBytes
	}
	if maxEntries <= 0 {
		maxEntries = defaultResponseCacheMaxEntries
	}
	responseCacheDiskMutex.Lock()
	defer responseCacheDiskMutex.Unlock()
	htmlCacheMutex.Lock()
	oldDirectory := responseCache.directory
	responseCache.generation++
	responseCache.routes = make(map[string]uint64)
	_, paths := purgeResponseCacheEntriesLocked("")
	responseCache.root = root
	responseCache.directory = ""
	responseCache.maxBytes = maxBytes
	responseCache.maxEntries = maxEntries
	responseCache.diskBytes = 0
	responseCache.diskEntries = 0
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles(paths)
	if oldDirectory != "" {
		if err := os.RemoveAll(oldDirectory); err != nil {
			logging.GetLogger().Warnw("Unable to clean previous response cache namespace", "error", err)
		}
	}
	// No directory is created until a disk write is actually needed.
	return nil
}

// Caller holds responseCacheDiskMutex. Owner records are created before any
// entry. Cleanup is conservative: another host, a reused PID, a permissions
// error, or an unknown owner record can never authorize deleting a namespace.
func ensureResponseCacheDirectory() (string, error) {
	htmlCacheMutex.RLock()
	directory, root := responseCache.directory, responseCache.root
	htmlCacheMutex.RUnlock()
	if directory != "" {
		return directory, nil
	}
	if root == "" {
		return "", errors.New("response disk cache is not configured")
	}
	parent := filepath.Join(root, "responses")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", fmt.Errorf("create response cache directory: %w", err)
	}
	// The configured root is resolved/validated by the runtime. Its private
	// subtree must not redirect into static or other source directories.
	info, err := os.Lstat(parent)
	if err != nil {
		return "", fmt.Errorf("inspect response cache directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("response cache responses directory must be a real directory, not a symlink")
	}
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("identify response cache owner: %w", err)
	}
	reapAbandonedResponseCaches(parent, host)
	directory, err = os.MkdirTemp(parent, "runtime-")
	if err != nil {
		return "", fmt.Errorf("create response cache namespace: %w", err)
	}
	owner, err := json.Marshal(responseCacheOwner{Version: 1, PID: os.Getpid(), Host: host})
	if err == nil {
		err = os.WriteFile(filepath.Join(directory, "owner.json"), owner, 0600)
	}
	if err != nil {
		_ = os.RemoveAll(directory)
		return "", fmt.Errorf("record response cache owner: %w", err)
	}
	htmlCacheMutex.Lock()
	responseCache.directory = directory
	htmlCacheMutex.Unlock()
	return directory, nil
}

func reapAbandonedResponseCaches(parent, host string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "runtime-") {
			continue
		}
		directory := filepath.Join(parent, entry.Name())
		data, err := os.ReadFile(filepath.Join(directory, "owner.json"))
		var owner responseCacheOwner
		if err != nil || json.Unmarshal(data, &owner) != nil || owner.Version != 1 || owner.Host != host || owner.PID <= 0 {
			continue
		}
		// Only ESRCH proves that no such process exists. EPERM can mean an
		// active runtime owned by another user, so leave its files alone.
		if !errors.Is(syscall.Kill(owner.PID, 0), syscall.ESRCH) {
			continue
		}
		if err := os.RemoveAll(directory); err != nil {
			logging.GetLogger().Warnw("Unable to remove abandoned response cache", "error", err)
		}
	}
}

func enforceResponseCacheBudgetLocked(now time.Time, reservedBytes int64, reservedEntries int) []string {
	maxBytes, maxEntries := responseCache.maxBytes-reservedBytes, responseCache.maxEntries-reservedEntries
	if responseCache.diskBytes <= maxBytes && responseCache.diskEntries <= maxEntries {
		return nil
	}
	var paths []string
	type candidate struct {
		key       string
		timestamp time.Time
	}
	var candidates []candidate
	for key, entry := range htmlCache {
		if entry.cacheDiskPath == "" {
			continue
		}
		if now.Sub(entry.Timestamp) > entry.cacheTTL {
			paths = append(paths, removeResponseCacheEntryLocked(key))
			continue
		}
		candidates = append(candidates, candidate{key, entry.Timestamp})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].timestamp.Before(candidates[j].timestamp) })
	for _, candidate := range candidates {
		if responseCache.diskBytes <= maxBytes && responseCache.diskEntries <= maxEntries {
			break
		}
		paths = append(paths, removeResponseCacheEntryLocked(candidate.key))
	}
	return paths
}

func removeResponseCacheFiles(paths []string) error {
	var failures error
	for _, path := range paths {
		if path == "" {
			continue
		}
		htmlCacheMutex.Lock()
		responseCache.pendingRemovals[path] = true
		htmlCacheMutex.Unlock()
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			logging.GetLogger().Warnw("Unable to remove response cache file; maintenance will retry", "error", err)
			failures = errors.Join(failures, err)
			continue
		}
		htmlCacheMutex.Lock()
		delete(responseCache.pendingRemovals, path)
		htmlCacheMutex.Unlock()
	}
	return failures
}

func retryResponseCacheRemovals() error {
	for {
		htmlCacheMutex.RLock()
		paths := make([]string, 0, len(responseCache.pendingRemovals))
		for path := range responseCache.pendingRemovals {
			paths = append(paths, path)
		}
		htmlCacheMutex.RUnlock()
		if len(paths) == 0 {
			return nil
		}
		if err := removeResponseCacheFiles(paths); err != nil {
			return fmt.Errorf("response disk cache has files awaiting cleanup; disk writes suspended: %w", err)
		}
		// A concurrent purge can queue another removal while this batch is
		// being deleted. Drain that batch too: pending work alone is not a
		// storage failure. Our disk mutex excludes new disk publications, so
		// only the finite set of existing files can join the queue here.
	}
}

func runResponseCacheCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupResponseCache(time.Now())
		}
	}
}

func cleanupResponseCache(now time.Time) {
	responseCacheDiskMutex.Lock()
	defer responseCacheDiskMutex.Unlock()
	_ = retryResponseCacheRemovals()
	htmlCacheMutex.Lock()
	var paths []string
	indexed := make(map[string]bool)
	for key, entry := range htmlCache {
		if entry.cacheTTL > 0 && now.Sub(entry.Timestamp) > entry.cacheTTL {
			paths = append(paths, removeResponseCacheEntryLocked(key))
		} else if entry.cacheDiskPath != "" {
			indexed[entry.cacheDiskPath] = true
		}
	}
	directory, root := responseCache.directory, responseCache.root
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles(paths)
	if directory != "" {
		entries, err := os.ReadDir(directory)
		if err == nil {
			for _, entry := range entries {
				path := filepath.Join(directory, entry.Name())
				if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".entry") || strings.HasSuffix(entry.Name(), ".tmp")) && !indexed[path] {
					removeResponseCacheFiles([]string{path})
				}
			}
		}
	}
	if root != "" {
		if host, err := os.Hostname(); err == nil {
			reapAbandonedResponseCaches(filepath.Join(root, "responses"), host)
		}
	}
}

func closeResponseCache() error {
	responseCacheDiskMutex.Lock()
	defer responseCacheDiskMutex.Unlock()
	htmlCacheMutex.Lock()
	responseCache.generation++
	responseCache.routes = make(map[string]uint64)
	_, paths := purgeResponseCacheEntriesLocked("")
	directory := responseCache.directory
	responseCache.directory = ""
	responseCache.root = ""
	htmlCacheMutex.Unlock()
	removeResponseCacheFiles(paths)
	if directory != "" {
		return os.RemoveAll(directory)
	}
	return nil
}
