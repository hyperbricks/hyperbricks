package component

import (
	"context"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func cachedImageFixture(t *testing.T) (*ImageProcessor, string, string) {
	t.Helper()
	root := imageTestDirectories(t)
	src := filepath.Join(root, "photo.png")
	writeTestImage(t, src, color.RGBA{R: 200, A: 255})
	p := NewImageProcessor()
	p.cacheDir = filepath.Join(root, "private")
	t.Cleanup(p.CloseAssets)
	return p, src, filepath.Join(root, "static", "images")
}
func imageGenerate(t *testing.T, p *ImageProcessor, src, dest string, cfg SingleImageConfig) string {
	t.Helper()
	name, err := p.processImage(src, dest, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return name
}
func imageInfo(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
func TestImageCacheReuseAndRestart(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncached", true: "cached"}[cached], func(t *testing.T) {
			p, src, dest := cachedImageFixture(t)
			cfg := SingleImageConfig{Width: 4, Cache: cached}
			name := imageGenerate(t, p, src, dest, cfg)
			before := imageInfo(t, filepath.Join(dest, name))
			s, _ := p.storeFor(dest)
			history := imageInfo(t, s.historyPath)
			again := imageGenerate(t, p, src, dest, cfg)
			after := imageInfo(t, filepath.Join(dest, again))
			if name != again || os.SameFile(before, after) != cached {
				t.Fatal("cache policy not followed")
			}
			if !os.SameFile(history, imageInfo(t, s.historyPath)) {
				t.Fatal("identical generation rewrote ownership")
			}
			p.CloseAssets()
			restart := NewImageProcessor()
			restart.cacheDir = p.cacheDir
			t.Cleanup(restart.CloseAssets)
			again = imageGenerate(t, restart, src, dest, cfg)
			if os.SameFile(after, imageInfo(t, filepath.Join(dest, again))) != cached {
				t.Fatal("restart reuse not followed")
			}
		})
	}
}
func TestImageCacheSourceBytesAndSettings(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true}
	name := imageGenerate(t, p, src, dest, cfg)
	info := imageInfo(t, src)
	writeTestImage(t, src, color.RGBA{B: 200, A: 255})
	if err := os.Chtimes(src, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed := imageGenerate(t, p, src, dest, cfg)
	if name == changed {
		t.Fatal("changed source reused URL")
	}
	if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
		t.Fatal("default grace did not preserve old URL")
	}
	cfg.Width = 8
	if imageGenerate(t, p, src, dest, cfg) == changed {
		t.Fatal("changed dimensions reused URL")
	}
}
func TestImageCacheMissingAndCorruptOutputRebuild(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "corrupt", true: "missing"}[missing], func(t *testing.T) {
			p, src, dest := cachedImageFixture(t)
			cfg := SingleImageConfig{Width: 4, Cache: true}
			name := imageGenerate(t, p, src, dest, cfg)
			path := filepath.Join(dest, name)
			original, _ := os.ReadFile(path)
			if missing {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(path, []byte("corrupt"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if imageGenerate(t, p, src, dest, cfg) != name {
				t.Fatal("repair changed URL")
			}
			actual, _ := os.ReadFile(path)
			if string(actual) != string(original) {
				t.Fatal("corrupt cache served")
			}
		})
	}
}
func TestImageCacheTagAttributesRemainCurrent(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Src: src, Width: 4, Cache: true, Alt: "first"}
	first, err := p.ProcessSingleImage(cfg)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(parsedImageAttributes(t, first)["src"])
	before := imageInfo(t, filepath.Join(dest, name))
	cfg.Alt = `second " & <tag>`
	cfg.Class = "new"
	second, err := p.ProcessSingleImage(cfg)
	if err != nil {
		t.Fatal(err)
	}
	attrs := parsedImageAttributes(t, second)
	if attrs["alt"] != cfg.Alt || attrs["class"] != "new" || attrs["src"] != parsedImageAttributes(t, first)["src"] || !os.SameFile(before, imageInfo(t, filepath.Join(dest, name))) {
		t.Fatal("tag or cache stale")
	}
}
func TestImageCacheFailurePreservesPreviousGeneration(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true, CacheGrace: "0s"}
	name := imageGenerate(t, p, src, dest, cfg)
	s, _ := p.storeFor(dest)
	history, _ := os.ReadFile(s.historyPath)
	os.WriteFile(src, []byte("invalid source"), 0644)
	if _, err := p.processImage(src, dest, cfg); err == nil {
		t.Fatal("invalid source served cache")
	}
	if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
		t.Fatal("failed build lost old file")
	}
	after, _ := os.ReadFile(s.historyPath)
	if string(history) != string(after) {
		t.Fatal("failed generation changed ownership")
	}
	writeTestImage(t, src, color.RGBA{G: 200, A: 255})
	imageGenerate(t, p, src, dest, cfg)
}
func TestImageCacheRetentionGraceProtectionAndOrphans(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true, CacheGrace: "1h"}
	s, _ := p.storeFor(dest)
	clock := time.Now()
	s.now = func() time.Time { return clock }
	first := imageGenerate(t, p, src, dest, cfg)
	writeTestImage(t, src, color.RGBA{B: 200, A: 255})
	second := imageGenerate(t, p, src, dest, cfg)
	if !p.HasPendingAssetCleanup() {
		t.Fatal("retired URL not pending")
	}
	clock = clock.Add(time.Hour)
	protected := true
	p.SetAssetProtection(func(path string) bool { return protected && filepath.Base(path) == first })
	if err := p.ReconcileAssets(dest, nil); err != nil {
		t.Fatal(err)
	}
	imageInfo(t, filepath.Join(dest, first))
	protected = false
	if err := p.ReconcileAssets(dest, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, first)); !os.IsNotExist(err) {
		t.Fatal("expired unprotected generation not removed")
	}
	if !p.OwnsOutput(filepath.Join(dest, first)) {
		t.Fatal("queued watcher removal forgotten")
	}
	manual := filepath.Join(dest, "manual.png")
	os.WriteFile(manual, []byte("not owned"), 0644)
	if err := p.ReconcileAssets(dest, map[string]ImageAssetPolicy{}); err != nil {
		t.Fatal(err)
	}
	imageInfo(t, filepath.Join(dest, second))
	clock = clock.Add(time.Hour)
	if err := p.ReconcileAssets(dest, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, second)); !os.IsNotExist(err) {
		t.Fatal("orphan not removed")
	}
	imageInfo(t, manual)
}
func TestImageCacheKeepAndExternallyModifiedOutputs(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true, CacheGrace: "0s", CacheKeep: 1}
	first := imageGenerate(t, p, src, dest, cfg)
	writeTestImage(t, src, color.RGBA{B: 200, A: 255})
	second := imageGenerate(t, p, src, dest, cfg)
	imageInfo(t, filepath.Join(dest, first))
	os.WriteFile(filepath.Join(dest, first), []byte("manual edit"), 0644)
	writeTestImage(t, src, color.RGBA{G: 200, A: 255})
	imageGenerate(t, p, src, dest, cfg)
	data, _ := os.ReadFile(filepath.Join(dest, first))
	if string(data) != "manual edit" {
		t.Fatal("modified file deleted")
	}
	imageInfo(t, filepath.Join(dest, second))
}
func TestImageCacheConcurrentReuseAndCancellation(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true}
	var wg sync.WaitGroup
	names := make(chan string, 12)
	for range 12 {
		wg.Go(func() {
			name, err := p.processImage(src, dest, cfg)
			if err != nil {
				t.Error(err)
			}
			names <- name
		})
	}
	wg.Wait()
	close(names)
	var want string
	for name := range names {
		if want != "" && name != want {
			t.Fatal("inconsistent concurrent URL")
		}
		want = name
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 1 {
		t.Fatal("duplicate generations or staging leak")
	}
	before := imageInfo(t, filepath.Join(dest, want))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg.renderContext = ctx
	if _, err := p.processImage(src, dest, cfg); err != context.Canceled {
		t.Fatalf("cancellation lost: %v", err)
	}
	if !os.SameFile(before, imageInfo(t, filepath.Join(dest, want))) {
		t.Fatal("cancelled request republished output")
	}
}
func TestImageCacheHistoryValidationAndSymlinkSafety(t *testing.T) {
	for _, kind := range []string{"malformed", "identity", "traversal", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			p, src, dest := cachedImageFixture(t)
			cfg := SingleImageConfig{Width: 4, Cache: true, CacheGrace: "0s"}
			name := imageGenerate(t, p, src, dest, cfg)
			s, _ := p.storeFor(dest)
			if kind == "symlink" {
				outside := filepath.Join(t.TempDir(), "outside.png")
				os.WriteFile(outside, []byte("outside"), 0644)
				os.Remove(filepath.Join(dest, name))
				if err := os.Symlink(outside, filepath.Join(dest, name)); err != nil {
					t.Fatal(err)
				}
				if _, err := p.processImage(src, dest, cfg); err == nil {
					t.Fatal("followed output symlink")
				}
				data, _ := os.ReadFile(outside)
				if string(data) != "outside" {
					t.Fatal("outside file changed")
				}
				return
			}
			data := []byte("{")
			if kind != "malformed" {
				h := s.history
				if kind == "identity" {
					h.Root = "wrong"
				} else {
					for _, r := range h.Owners {
						r.Generations[0].Name = "../outside.png"
					}
				}
				data, _ = json.Marshal(h)
			}
			os.WriteFile(s.historyPath, data, 0600)
			restart := NewImageProcessor()
			restart.cacheDir = p.cacheDir
			t.Cleanup(restart.CloseAssets)
			if _, err := restart.processImage(src, dest, cfg); err == nil {
				t.Fatal("invalid ownership accepted")
			}
			imageInfo(t, filepath.Join(dest, name))
		})
	}
}
func TestImagesCacheDirectoryChangesAndSharedOutputs(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	dir := filepath.Join(filepath.Dir(src), "gallery")
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	writeTestImage(t, a, color.RGBA{R: 200, A: 255})
	cfg := MultipleImagesConfig{Directory: dir, Width: 4, Cache: true, CacheGrace: "0s"}
	first, err := p.ProcessMultipleImages(cfg)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(parsedImageAttributes(t, first)["src"])
	before := imageInfo(t, filepath.Join(dest, name))
	if _, err = p.ProcessMultipleImages(cfg); err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, imageInfo(t, filepath.Join(dest, name))) {
		t.Fatal("gallery did not reuse")
	}
	// A single component shares the same output but retains separate ownership.
	imageGenerate(t, p, a, dest, SingleImageConfig{Width: 4, Cache: true, CacheGrace: "0s"})
	writeTestImage(t, b, color.RGBA{B: 200, A: 255})
	os.Remove(a)
	gallery, err := p.ProcessMultipleImages(cfg)
	if err != nil || strings.Count(gallery, "<img ") != 1 {
		t.Fatalf("changed directory: %s %v", gallery, err)
	}
	imageInfo(t, filepath.Join(dest, name))
	owners, err := p.DiscoverImages(nil, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.ReconcileAssets(dest, owners); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, name)); !os.IsNotExist(err) {
		t.Fatal("removed image not pruned after all owners retired")
	}
	os.Remove(b)
	if _, err = p.ProcessMultipleImages(cfg); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Fatal("empty gallery did not retire output")
	}
}
func TestImageCacheSessionExcludesOtherProcess(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	if err := p.BeginAssetSession(dest); err != nil {
		t.Fatal(err)
	}
	other := NewImageProcessor()
	other.cacheDir = p.cacheDir
	t.Cleanup(other.CloseAssets)
	if _, err := other.processImage(src, dest, SingleImageConfig{Cache: true}); err == nil {
		t.Fatal("second session published")
	}
	p.CloseAssets()
	imageGenerate(t, other, src, dest, SingleImageConfig{Cache: true})
}
func TestImageCacheInvalidPolicy(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	for _, cfg := range []SingleImageConfig{{CacheKeep: -1}, {CacheGrace: "-1s"}, {CacheGrace: "bad"}} {
		if _, err := p.processImage(src, dest, cfg); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}

func TestImageCacheCleanupFailurePreservesHistoryAndRetries(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true, CacheGrace: "0s"}
	first := imageGenerate(t, p, src, dest, cfg)
	path := filepath.Join(dest, first)
	original, _ := os.ReadFile(path)
	protect := true
	p.SetAssetProtection(func(path string) bool { return protect && filepath.Base(path) == first })
	writeTestImage(t, src, color.RGBA{B: 200, A: 255})
	second := imageGenerate(t, p, src, dest, cfg)
	s, _ := p.storeFor(dest)
	history, _ := os.ReadFile(s.historyPath)
	os.Remove(path)
	os.Mkdir(path, 0755)
	protect = false
	if err := p.ReconcileAssets(dest, nil); err == nil {
		t.Fatal("unsafe cleanup succeeded")
	}
	after, _ := os.ReadFile(s.historyPath)
	if string(after) != string(history) || !p.HasPendingAssetCleanup() {
		t.Fatal("failed cleanup lost history or retry")
	}
	imageInfo(t, filepath.Join(dest, second))
	os.Remove(path)
	os.WriteFile(path, original, 0644)
	if err := p.ReconcileAssets(dest, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("retry did not remove obsolete output")
	}
	if p.HasPendingAssetCleanup() {
		t.Fatal("successful cleanup still pending")
	}
}
func TestImageCacheMaximumKeepCannotDeleteCurrent(t *testing.T) {
	p, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true, CacheKeep: int(^uint(0) >> 1), CacheGrace: "0s"}
	first := imageGenerate(t, p, src, dest, cfg)
	writeTestImage(t, src, color.RGBA{B: 200, A: 255})
	second := imageGenerate(t, p, src, dest, cfg)
	imageInfo(t, filepath.Join(dest, first))
	imageInfo(t, filepath.Join(dest, second))
}
