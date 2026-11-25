package component

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestEsbuildWarmCacheUsesPublishedResultUntilInvalidation(t *testing.T) {
	for _, fingerprint := range []bool{false, true} {
		for _, session := range []bool{false, true} {
			t.Run(fmt.Sprintf("fingerprint=%t/session=%t", fingerprint, session), func(t *testing.T) {
				r, cfg := esbuildFixture(t)
				cfg.Fingerprint, cfg.Sourcemap = fingerprint, true
				cfg.Loader = map[string]string{".bin": "file"}
				esbuildWrite(t, cfg.Entry, `import payload from "./payload.bin"; window.payloadURL = payload;`)
				esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "payload.bin"), strings.Repeat("asset", 262144))
				if session {
					if err := r.BeginAssetSession(); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(r.CloseAssets)
				}
				p := r.Prepare(cfg)
				first := esbuildRender(t, p)
				// Simulate unavailable external files after the successful build.
				// The component still returns its admitted URL until invalidation;
				// serving the asset itself remains a separate HTTP/file operation.
				if err := os.Remove(cfg.Entry); err != nil {
					t.Fatal(err)
				}
				for output := range r.store.results[p.key].outputs {
					if err := os.Remove(output); err != nil {
						t.Fatal(err)
					}
				}
				r.store.cacheDirErr = errors.New("ownership storage unavailable")
				r.store.ownershipDirErr = r.store.cacheDirErr
				var wg sync.WaitGroup
				for i := 0; i < 24; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if got, errs := p.Render(context.Background()); got != first || len(errs) != 0 {
							t.Errorf("warm result = %q, %v", got, errs)
						}
					}()
				}
				wg.Wait()
				if r.store.buildCount != 1 {
					t.Fatal("warm requests compiled again")
				}
				r.Invalidate()
				if got, errs := p.Render(nil); got != "" || len(errs) == 0 {
					t.Fatal("invalidation continued returning unavailable output")
				}
			})
		}
	}
}

func TestEsbuildRemovedOwnerCannotPublishFromOldRoute(t *testing.T) {
	r, cfg := esbuildFixture(t)
	p := r.Prepare(cfg)
	esbuildRender(t, p)
	if err := r.ReconcileAssets(map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if got, errs := p.Render(nil); got != "" || len(errs) == 0 {
		t.Fatal("removed route republished its assets")
	}
	if _, err := os.Stat(cfg.Outfile); !os.IsNotExist(err) {
		t.Fatal("orphan output remains", err)
	}
}

func TestEsbuildInheritedRetentionUsesMaximumCurrentPolicy(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.CacheKeep, cfg.Fingerprint = 2, true
	cfg.Enclose = ""
	high := r.Prepare(cfg)
	cfg.CacheKeep = 0
	low := r.Prepare(cfg)
	first := esbuildRender(t, low)
	for _, value := range []string{"second", "third"} {
		esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), fmt.Sprintf("export const message=%q;", value))
		r.Invalidate()
		// Re-prepare both inherited uses; a smaller later declaration must not
		// lower the larger active policy.
		cfg.CacheKeep = 2
		high = r.Prepare(cfg)
		cfg.CacheKeep = 0
		low = r.Prepare(cfg)
		esbuildRender(t, low)
	}
	h, err := r.store.loadHistory()
	if err != nil {
		t.Fatal(err)
	}
	if owner := h.Owners[cfg.Outfile]; owner.Keep != 2 || len(owner.Generations) != 3 {
		t.Fatalf("shared retention = %#v", owner)
	}
	builds := r.store.buildCount
	if err := r.ReconcileAssets(map[string]int{cfg.Outfile: 0}); err != nil {
		t.Fatal(err)
	}
	esbuildRender(t, high)
	if r.store.buildCount != builds {
		t.Fatal("retention policy rebuilt output")
	}
	if _, err := os.Stat(esbuildURLPath(t, r, first)); !os.IsNotExist(err) {
		t.Fatal("old policy restored a retired generation", err)
	}
}

func TestEsbuildPublicationInvalidatesDependentBuild(t *testing.T) {
	r, producer := esbuildFixture(t)
	producer.Cache, producer.Enclose = false, ""
	esbuildWrite(t, producer.Entry, `window.generated = "first";`)
	a := r.Prepare(producer)
	esbuildRender(t, a)
	consumer := producer
	consumer.Cache, consumer.Entry, consumer.Outfile = true, producer.Outfile, filepath.Join(r.store.staticDir, "js", "consumer.js")
	b := r.Prepare(consumer)
	esbuildRender(t, b)
	esbuildWrite(t, producer.Entry, `window.generated = "second";`)
	esbuildRender(t, a)
	esbuildRender(t, b)
	if !strings.Contains(esbuildRead(t, consumer.Outfile), "second") {
		t.Fatal("generated input remained stale in consumer cache")
	}
}

func TestEsbuildExportOutputAndFreshOwnership(t *testing.T) {
	source, cfg := esbuildFixture(t)
	cfg.Enclose = ""
	sourceURL := esbuildRender(t, source.Prepare(cfg))
	sourceBytes := esbuildRead(t, cfg.Outfile)
	output := filepath.Join(t.TempDir(), "rendered", "static")
	newExport := func() *EsbuildRenderer {
		r := NewEsbuildRenderer(source.store.staticDir)
		r.store.cacheDir = source.store.cacheDir
		r.store.ownershipDir = source.store.ownershipDir
		if err := r.SetOutputRoot(output); err != nil {
			t.Fatal(err)
		}
		if err := r.BeginAssetSession(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(r.CloseAssets)
		if err := r.ResetAssetsForFreshExport(context.Background()); err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := newExport()
	if got := esbuildRender(t, first.Prepare(cfg)); got != sourceURL {
		t.Fatal("output root changed public URL", got)
	}
	first.CloseAssets()
	exported := filepath.Join(output, "js", "app.js")
	second := newExport()
	// A new copy can contain the same physical name as an earlier export's
	// generated output. Previous ownership must not adopt/delete that copy.
	esbuildWrite(t, exported, "copied static file")
	if err := second.ReconcileAssets(map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if got := esbuildRead(t, exported); got != "copied static file" {
		t.Fatal("previous export adopted copied static file")
	}
	p := second.Prepare(cfg)
	if got := esbuildRender(t, p); got != sourceURL || second.store.buildCount != 1 {
		t.Fatal("fresh export reused a prior build", got)
	}
	if got := esbuildRead(t, cfg.Outfile); got != sourceBytes {
		t.Fatal("export modified source static output")
	}
	if got := esbuildRead(t, exported); got != sourceBytes {
		t.Fatal("exported output differs from source build")
	}
	if err := second.SetOutputRoot(t.TempDir()); err == nil {
		t.Fatal("active renderer changed output root")
	}
	if err := second.ResetAssetsForFreshExport(context.Background()); err == nil {
		t.Fatal("prepared renderer discarded ownership")
	}
}

func TestEsbuildFailedOwnershipIsNotAdmittedAsWarm(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(fmt.Sprintf("restore=%t", restore), func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			cfg.Enclose = ""
			esbuildRender(t, r.Prepare(cfg))
			history, err := r.store.historyPath()
			if err != nil {
				t.Fatal(err)
			}
			previous := esbuildRead(t, history)
			if restore {
				r = esbuildRestart(r)
			} else {
				r.Invalidate()
			}
			p := r.Prepare(cfg)
			esbuildWrite(t, history, "broken ownership history")
			got, errs := p.Render(context.Background())
			if got != "" || len(errs) == 0 {
				t.Fatalf("ownership failure returned success: %q %v", got, errs)
			}
			if r.store.results[p.key] != nil {
				t.Error("failed ownership operation left a warm success entry")
			}
			got, errs = p.Render(context.Background())
			if got != "" || len(errs) == 0 {
				t.Error("retry hid the unresolved ownership failure")
			}
			esbuildWrite(t, history, previous)
			if got := esbuildRender(t, p); got != "/static/js/app.js" {
				t.Errorf("recovery URL = %q", got)
			}
			if !strings.Contains(esbuildRead(t, cfg.Outfile), "first") {
				t.Error("recovery lost asset content")
			}
		})
	}
}

func TestEsbuildOldPreparedConfigCannotReplaceCurrentAssets(t *testing.T) {
	for _, fingerprint := range []bool{false, true} {
		t.Run(fmt.Sprintf("fingerprint=%t", fingerprint), func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			cfg.Enclose, cfg.Fingerprint = "", fingerprint
			old := r.Prepare(cfg)
			esbuildRender(t, old)
			r.Invalidate()
			cfg.Minify = true
			current := r.Prepare(cfg)
			currentURL := esbuildRender(t, current)
			currentPath := esbuildURLPath(t, r, currentURL)
			currentBytes := esbuildRead(t, currentPath)
			currentFingerprint := r.store.results[current.key].outputs[currentPath]
			// Deterministic stale-request schedule: an old snapshot resumes only
			// after replacement configuration has successfully published assets.
			if got, errs := old.Render(context.Background()); got != "" || len(errs) == 0 {
				t.Fatal("superseded configuration was accepted")
			}
			got, err := os.ReadFile(currentPath)
			if err != nil || string(got) != currentBytes {
				t.Errorf("stale prepared config replaced/removed current output: %v", err)
			}
			h, err := r.store.loadHistory()
			if err != nil {
				t.Fatal(err)
			}
			owner := h.Owners[cfg.Outfile]
			if owner == nil || len(owner.Generations) == 0 || owner.Generations[0].Outputs[currentPath] != currentFingerprint {
				t.Error("stale prepared config changed current ownership generation")
			}
		})
	}
}

func TestEsbuildFixedOutputHistoryAllowsReplacedOlderBytes(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose, cfg.CacheKeep = "", 1
	p := r.Prepare(cfg)
	esbuildRender(t, p)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message = "replacement";`)
	r.Invalidate()
	esbuildRender(t, p)
	h, err := r.store.loadHistory()
	if err != nil {
		t.Fatal(err)
	}
	owner := h.Owners[cfg.Outfile]
	if owner == nil || len(owner.Generations) != 2 {
		t.Fatal("expected current and retained historical generation")
	}
	newer, older := owner.Generations[0], owner.Generations[1]
	if newer.Entry != older.Entry || newer.Entry != cfg.Outfile {
		t.Fatal("fixture must use one fixed output path")
	}
	if newer.Outputs[cfg.Outfile] == older.Outputs[cfg.Outfile] {
		t.Fatal("fixture did not replace the old bytes")
	}
	actual, err := esbuildFingerprints(map[string]bool{cfg.Outfile: true})
	if err != nil {
		t.Fatal(err)
	}
	if actual[cfg.Outfile] != newer.Outputs[cfg.Outfile] {
		t.Fatal("current published bytes differ from current ownership")
	}
	// An export validator must not reject this valid tree merely because an
	// older record legitimately has a different hash for the overwritten path.
	if actual[cfg.Outfile] == older.Outputs[cfg.Outfile] {
		t.Fatal("expected obsolete history bytes to differ")
	}
}

// Invalidating a prepared configuration permits one shared fresh build.
func TestEsbuildExplicitInvalidationConcurrentReuse(t *testing.T) {
	for _, fingerprint := range []bool{false, true} {
		t.Run(fmt.Sprintf("fingerprint=%t", fingerprint), func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			cfg.Enclose, cfg.Fingerprint, cfg.Sourcemap = "", fingerprint, true
			p := r.Prepare(cfg)
			first := esbuildRender(t, p)
			esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message = "after invalidation";`)
			r.Invalidate()
			urls := make(chan string, 24)
			var wg sync.WaitGroup
			for i := 0; i < cap(urls); i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					got, errs := p.Render(context.Background())
					if len(errs) != 0 {
						t.Errorf("render: %v", errs)
					}
					urls <- got
				}()
			}
			wg.Wait()
			close(urls)
			current := ""
			for got := range urls {
				if current != "" && got != current {
					t.Errorf("concurrent URLs differ: %q / %q", current, got)
				}
				current = got
			}
			if r.store.buildCount != 2 {
				t.Errorf("expected one compile per generation, got %d", r.store.buildCount)
			}
			if fingerprint == (current == first) {
				t.Error("URL did not follow fingerprint policy")
			}
			path := esbuildURLPath(t, r, current)
			if !strings.Contains(esbuildRead(t, path), "after invalidation") {
				t.Error("stale generated content")
			}
			if _, err := os.Stat(path + ".map"); err != nil {
				t.Fatal("missing sourcemap", err)
			}
			if esbuildRender(t, p) != current || r.store.buildCount != 2 {
				t.Error("new generation not reused")
			}
		})
	}
}
