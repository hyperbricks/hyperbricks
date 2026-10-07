package component

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEsbuildRetentionDefaultAndRestart(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint = true
	cfg.Enclose = ""
	first := esbuildURLPath(t, r, esbuildRender(t, r.Prepare(cfg)))
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="second";`)
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
	esbuildRender(t, r.Prepare(cfg))
	cfg.CacheKeep = 0
	builds := r.store.buildCount
	esbuildRender(t, r.Prepare(cfg))
	if r.store.buildCount != builds {
		t.Fatal("retention change rebuilt valid assets")
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("cache hit skipped cleanup")
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
