package component

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEsbuildRetentionDefaultAndRestart(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint = true
	cfg.Enclose = ""
	first := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="second";`)
	r.Invalidate()
	second := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	if first == second {
		t.Fatal("no new generation")
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("obsolete output remains: %v", err)
	}
	next := esbuildRestart(r)
	handwritten := filepath.Join(next.store.staticDir, "js", "manual.js")
	esbuildWrite(t, handwritten, "manual")
	if err := next.ReconcileAssets(map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(second); !os.IsNotExist(err) {
		t.Fatalf("orphan remains after restart: %v", err)
	}
	if got := esbuildRead(t, handwritten); got != "manual" {
		t.Fatal(got)
	}
}

func TestEsbuildRetentionProtectionAndKeep(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint = true
	cfg.Enclose = ""
	cfg.CacheKeep = 1
	var paths []string
	for _, value := range []string{"one", "two", "three"} {
		esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="`+value+`";`)
		r.Invalidate()
		paths = append(paths, esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg))))
		if len(paths) == 1 {
			r.SetAssetProtection(func(path string) bool { return path == paths[0] })
		}
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	r.SetAssetProtection(nil)
	if err := r.ReconcileAssets(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("unprotected old output retained: %v", err)
	}
	for _, path := range paths[1:] {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEsbuildRetentionCacheHitsAndSessionExclusion(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint = true
	cfg.Enclose = ""
	cfg.CacheKeep = 2
	first := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="new";`)
	r.Invalidate()
	esbuildRender(t, r.Prepare(cfg))
	cfg.CacheKeep = 0
	if err := r.ReconcileAssets(map[string]int{cfg.Outfile: 0}); err != nil {
		t.Fatal(err)
	}
	builds := r.store.buildCount
	esbuildRender(t, r.Prepare(cfg))
	if r.store.buildCount != builds {
		t.Fatal("retention change rebuilt valid assets")
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("retention reconciliation skipped cleanup")
	}
	if err := r.BeginAssetSession(); err != nil {
		t.Fatal(err)
	}
	other := esbuildRestart(r)
	if err := other.BeginAssetSession(); err == nil {
		other.CloseAssets()
		t.Fatal("competing session acquired outputs")
	}
	r.CloseAssets()
	if err := other.BeginAssetSession(); err != nil {
		t.Fatal(err)
	}
	other.CloseAssets()
}

func TestEsbuildRetentionPreservesModifiedFiles(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint = true
	cfg.Enclose = ""
	old := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	esbuildWrite(t, old, "manually replaced")
	if err := r.ReconcileAssets(map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if got := esbuildRead(t, old); got != "manually replaced" {
		t.Fatal(got)
	}
}

func TestEsbuildRetentionTwoPreviousAndFailedUncachedBuild(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint = true
	cfg.Enclose = ""
	cfg.Cache = false
	cfg.CacheKeep = 2
	var paths []string
	for _, value := range []string{"one", "two", "three", "four"} {
		esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="`+value+`";`)
		r.Invalidate()
		paths = append(paths, esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg))))
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatal("fourth generation did not prune oldest")
	}
	esbuildWrite(t, cfg.Entry, `import "./missing-retention.js";`)
	if _, failures := r.Prepare(cfg).Render(nil); len(failures) == 0 {
		t.Fatal("broken build succeeded")
	}
	for _, path := range paths[1:] {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("failed build pruned successful generation", err)
		}
	}
}

// Fixing the timestamp makes an unnecessary rewrite observable without sleeps.
func esbuildHistoryUnchanged(t *testing.T, r *EsbuildRenderer) func() {
	t.Helper()
	path, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(946684800, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
			t.Fatal("unchanged asset history was rewritten")
		}
	}
}

func TestEsbuildRetentionInitNoAssetsAndUnchangedOwners(t *testing.T) {
	r, cfg := esbuildFixture(t)
	if err := r.ReconcileAssets(map[string]int{}); err != nil {
		t.Fatal(err)
	}
	path, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty init wrote history: %v", err)
	}
	if r.HasPendingAssetCleanup() {
		t.Fatal("empty module has pending cleanup")
	}
	// A real init discovers the owner before its first asset build.
	if err := r.ReconcileAssets(map[string]int{cfg.Outfile: 0}); err != nil {
		t.Fatal(err)
	}
	esbuildRender(t, r.Prepare(cfg))
	next := esbuildRestart(r)
	unchanged := esbuildHistoryUnchanged(t, next)
	if err := next.ReconcileAssets(map[string]int{cfg.Outfile: 0}); err != nil {
		t.Fatal(err)
	}
	unchanged()
	if next.HasPendingAssetCleanup() {
		t.Fatal("current generation needs no cleanup")
	}
	if err := next.ReconcileAssets(map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.Outfile); !os.IsNotExist(err) {
		t.Fatalf("init failed to remove orphan: %v", err)
	}
}

func TestEsbuildRetentionCacheHitAndIdenticalUncachedOutputNoWrite(t *testing.T) {
	for _, cached := range []bool{true, false} {
		t.Run(fmt.Sprintf("cached=%t", cached), func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			cfg.Cache = cached
			esbuildRender(t, r.Prepare(cfg))
			unchanged := esbuildHistoryUnchanged(t, r)
			builds := r.store.buildCount
			esbuildRender(t, r.Prepare(cfg))
			unchanged()
			if cached && r.store.buildCount != builds {
				t.Fatal("cache hit rebuilt assets")
			}
			if !cached && r.store.buildCount != builds+1 {
				t.Fatal("uncached build was skipped")
			}
			esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="changed";`)
			r.Invalidate()
			esbuildRender(t, r.Prepare(cfg))
			if r.store.buildCount != builds+1 && cached {
				t.Fatal("invalidated cache did not rebuild")
			}
			h, err := r.store.loadHistory()
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(cfg.Outfile)
			if err != nil {
				t.Fatal(err)
			}
			if h.Owners[cfg.Outfile].Generations[0].Outputs[cfg.Outfile].Hash != sha256.Sum256(data) {
				t.Fatal("changed build was not registered")
			}
		})
	}
}

func TestEsbuildRetentionDeferredCleanupRetriesWithoutRewriting(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint, cfg.Enclose = true, ""
	old := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	protected := true
	r.SetAssetProtection(func(path string) bool { return protected && path == old })
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="changed";`)
	r.Invalidate()
	current := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	if !r.HasPendingAssetCleanup() {
		t.Fatal("protected obsolete generation not pending")
	}
	unchanged := esbuildHistoryUnchanged(t, r)
	if err := r.ReconcileAssets(nil); err != nil {
		t.Fatal(err)
	}
	unchanged()
	if _, err := os.Stat(old); err != nil {
		t.Fatal("protected output was removed", err)
	}
	if !r.HasPendingAssetCleanup() {
		t.Fatal("cleanup stopped waiting for protected output")
	}
	protected = false
	if err := r.ReconcileAssets(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("unprotected obsolete output remains: %v", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatal("current output was removed", err)
	}
	if r.HasPendingAssetCleanup() {
		t.Fatal("completed cleanup is still pending")
	}
}

func TestEsbuildRetentionCleanupFailurePreservesHistoryAndRetries(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint, cfg.Enclose = true, ""
	old := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	original := esbuildRead(t, old)
	protected := true
	r.SetAssetProtection(func(path string) bool { return protected && path == old })
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="replacement";`)
	r.Invalidate()
	current := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	historyPath, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	before := esbuildRead(t, historyPath)
	protected = false
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(old, 0755); err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileAssets(nil); err == nil {
		t.Fatal("cleanup silently accepted a directory replacing an owned file")
	}
	if !r.HasPendingAssetCleanup() {
		t.Fatal("failed cleanup was not retained for retry")
	}
	if got := esbuildRead(t, historyPath); got != before {
		t.Fatal("failed cleanup changed ownership history")
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatal("failed cleanup lost current output", err)
	}
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, old, original)
	if err := r.ReconcileAssets(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("successful retry did not remove obsolete asset", err)
	}
	if r.HasPendingAssetCleanup() {
		t.Fatal("successful retry remains pending")
	}
}

func TestEsbuildRetentionCorruptHistoryFailsWithoutDeletingAssets(t *testing.T) {
	for _, kind := range []string{"malformed", "version", "root", "nil-owner", "outside-owner"} {
		t.Run(kind, func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			esbuildRender(t, r.Prepare(cfg))
			path, err := r.store.historyPath()
			if err != nil {
				t.Fatal(err)
			}
			h, err := r.store.loadHistory()
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "version":
				h.Version++
			case "root":
				h.Root = t.TempDir()
			case "nil-owner":
				h.Owners[cfg.Outfile] = nil
			case "outside-owner":
				h.Owners[filepath.Join(t.TempDir(), "outside.js")] = h.Owners[cfg.Outfile]
				delete(h.Owners, cfg.Outfile)
			}
			bytes, err := json.Marshal(h)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "malformed" {
				bytes = []byte("not JSON")
			}
			esbuildWrite(t, path, string(bytes))
			output := esbuildRead(t, cfg.Outfile)
			if err := r.ReconcileAssets(map[string]int{}); err == nil {
				t.Fatal("invalid ownership was accepted for deletion")
			}
			if esbuildRead(t, cfg.Outfile) != output {
				t.Fatal("invalid history changed a successful output")
			}
			if esbuildRead(t, path) != string(bytes) {
				t.Fatal("invalid history was silently replaced")
			}
		})
	}
}

func TestEsbuildOwnershipMigrationIsReadOnlyUntilMutation(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose = ""
	p := r.Prepare(cfg)
	esbuildRender(t, p)
	legacy, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	original := esbuildRead(t, legacy)
	r.store.ownershipDir = t.TempDir()
	canonical, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.store.loadHistory()
	if err != nil || h.Owners[cfg.Outfile] == nil {
		t.Fatal("legacy ownership was lost", err)
	}
	if _, err := os.Stat(canonical); !os.IsNotExist(err) {
		t.Fatal("read-only discovery migrated ownership", err)
	}
	if got := esbuildRead(t, legacy); got != original {
		t.Fatal("read-only discovery changed legacy ownership")
	}
	if err := r.ReconcileAssets(map[string]int{cfg.Outfile: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("mutation left legacy ownership behind", err)
	}
	if got := esbuildRead(t, canonical); got != original {
		t.Fatal("migration changed valid ownership")
	}
	// A later reader with a different build cache still sees this same history.
	next := esbuildRestart(r)
	next.store.ownershipDir = r.store.ownershipDir
	next.store.cacheDir = t.TempDir()
	nextHistory, err := next.store.loadHistory()
	if err != nil || nextHistory.Owners[cfg.Outfile] == nil {
		t.Fatal("ownership depends on build cache environment", err)
	}
}

func TestEsbuildOwnershipMigrationNeverLosesInvalidOrConflictingHistory(t *testing.T) {
	for _, mode := range []string{"invalid", "unwritable", "canonical wins"} {
		t.Run(mode, func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			esbuildRender(t, r.Prepare(cfg))
			legacy, err := r.store.historyPath()
			if err != nil {
				t.Fatal(err)
			}
			original := esbuildRead(t, legacy)
			r.store.ownershipDir = t.TempDir()
			canonical, err := r.store.historyPath()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "invalid" {
				esbuildWrite(t, legacy, "invalid legacy record")
				original = "invalid legacy record"
			}
			if mode == "unwritable" {
				// A file as parent produces a deterministic storage failure under root too.
				blocker := filepath.Join(r.store.ownershipDir, "file")
				esbuildWrite(t, blocker, "not a directory")
				r.store.ownershipDir = filepath.Join(blocker, "ownership")
			}
			if mode == "canonical wins" {
				esbuildWrite(t, canonical, original)
				esbuildWrite(t, legacy, "invalid legacy record")
				original = "invalid legacy record"
			}
			err = r.ReconcileAssets(map[string]int{cfg.Outfile: 0})
			if (mode == "canonical wins") != (err == nil) {
				t.Fatalf("unexpected migration result: %v", err)
			}
			if got := esbuildRead(t, legacy); got != original {
				t.Fatal("migration damaged legacy ownership")
			}
			if mode != "canonical wins" {
				if _, err := os.Stat(canonical); !os.IsNotExist(err) {
					t.Fatal("failed migration published canonical ownership", err)
				}
			}
			esbuildRead(t, cfg.Outfile)
		})
	}
}

func TestEsbuildEmptyLegacyOwnershipRemainsAuthoritative(t *testing.T) {
	r, cfg := esbuildFixture(t)
	esbuildRender(t, r.Prepare(cfg)) // Leave a valid build manifest available.
	legacy, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	empty := esbuildHistory{Version: 1, Root: r.store.staticDir, Owners: map[string]*esbuildOwnerHistory{}}
	data, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, legacy, string(data))
	r.store.ownershipDir = t.TempDir()
	if err := r.ReconcileAssets(nil); err != nil {
		t.Fatal(err)
	}
	canonical, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(canonical); err != nil {
		t.Fatal("migration failed to publish explicit empty ownership", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy record was not migrated", err)
	}
	h, err := r.store.loadHistory()
	if err != nil || len(h.Owners) != 0 {
		t.Fatal("old manifests reclaimed intentionally empty ownership", err)
	}
}
