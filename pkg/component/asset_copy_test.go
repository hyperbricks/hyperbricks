package component

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStaticAssetCopyMaterializesLinksAndExcludesStaging(t *testing.T) {
	base := t.TempDir()
	src, dst := filepath.Join(base, "static"), filepath.Join(base, "copy")
	esbuildWrite(t, filepath.Join(src, "nested", "authored.txt"), "authored")
	esbuildWrite(t, filepath.Join(src, ".well-known", "asset"), "hidden authored")
	esbuildWrite(t, filepath.Join(base, "external.txt"), "external authored")
	for _, name := range []string{".hb-esbuild-stage", ".image-stage", ".spaces-stage", ".scaffold-stage"} {
		esbuildWrite(t, filepath.Join(src, name), "private")
	}
	for name, target := range map[string]string{"alias": "nested", "external.txt": "../external.txt"} {
		if err := os.Symlink(target, filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyStaticAssets(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"nested/authored.txt": "authored", "alias/authored.txt": "authored", ".well-known/asset": "hidden authored", "external.txt": "external authored"} {
		if got := esbuildRead(t, filepath.Join(dst, path)); got != want {
			t.Fatalf("%s = %q", path, got)
		}
	}
	if err := filepath.WalkDir(dst, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 || privateStaticAsset(entry.Name()) {
			t.Errorf("nonportable/private output: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, filepath.Join(base, "external.txt"), "changed after copy")
	if esbuildRead(t, filepath.Join(dst, "external.txt")) != "external authored" {
		t.Fatal("copy still depends on source target")
	}
}

func TestStaticAssetCopyRejectsCyclesAndOverlap(t *testing.T) {
	root := t.TempDir()
	esbuildWrite(t, filepath.Join(root, "asset.txt"), "kept")
	if err := ResetAndCopyStaticAssets(context.Background(), root, root); err == nil {
		t.Fatal("equal export accepted")
	}
	if err := CopyStaticAssets(context.Background(), root, root); err == nil {
		t.Fatal("equal copy accepted")
	}
	if err := ResetAndCopyStaticAssets(context.Background(), root, filepath.Join(root, "export")); err == nil {
		t.Fatal("overlapping export accepted")
	}
	if esbuildRead(t, filepath.Join(root, "asset.txt")) != "kept" {
		t.Fatal("source changed")
	}
	if err := os.Symlink(".", filepath.Join(root, "cycle")); err != nil {
		t.Fatal(err)
	}
	if err := CopyStaticAssets(context.Background(), root, filepath.Join(t.TempDir(), "copy")); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle: %v", err)
	}
}

func TestStaticAssetCopyWaitsForCompletePublication(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "static"), filepath.Join(root, "copy")
	esbuildWrite(t, filepath.Join(src, "a.txt"), "old")
	unlock, err := LockAssetTree(context.Background(), src, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err = CopyStaticAssets(ctx, src, dst)
	if !errors.Is(err, context.DeadlineExceeded) {
		unlock()
		t.Fatalf("copy crossed active publication: %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		unlock()
		t.Fatalf("copy changed destination while writer owns tree: %v", err)
	}
	esbuildWrite(t, filepath.Join(src, "a.txt"), "new")
	esbuildWrite(t, filepath.Join(src, "b.txt"), "new")
	unlock()
	if err := CopyStaticAssets(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if esbuildRead(t, filepath.Join(dst, name)) != "new" {
			t.Fatalf("mixed generation at %s", name)
		}
	}
}

func TestStaticAssetCopyValidatesLatestFixedOutput(t *testing.T) {
	r, cfg := esbuildFixture(t)
	// Copy discovers persisted ownership through the same normal namespace as
	// a second CLI process. This root is unique to this test.
	r.store.cacheDir = NewEsbuildRenderer(r.store.staticDir).store.cacheDir
	r.store.ownershipDir = NewEsbuildRenderer(r.store.staticDir).store.ownershipDir
	cfg.CacheKeep = 1
	cfg.Enclose = ""
	esbuildRender(t, r.Prepare(cfg))
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "message.js"), `export const message="second";`)
	r.Invalidate()
	esbuildRender(t, r.Prepare(cfg))
	path, err := r.store.historyPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	history, err := r.store.loadHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Owners[cfg.Outfile].Generations) != 2 {
		t.Fatal("fixture lacks differing historical hashes")
	}
	dst := filepath.Join(t.TempDir(), "copy")
	if err := CopyStaticAssets(context.Background(), r.store.staticDir, dst); err != nil {
		t.Fatal(err)
	}
	if esbuildRead(t, filepath.Join(dst, "js", "app.js")) != esbuildRead(t, cfg.Outfile) {
		t.Fatal("wrong generation copied")
	}
	esbuildWrite(t, cfg.Outfile, "incomplete publication")
	export := filepath.Join(t.TempDir(), "export")
	esbuildWrite(t, filepath.Join(export, "sentinel"), "keep before validation")
	if err := ResetAndCopyStaticAssets(context.Background(), r.store.staticDir, export); err == nil {
		t.Fatal("corrupt known generation copied")
	}
	if esbuildRead(t, filepath.Join(export, "sentinel")) != "keep before validation" {
		t.Fatal("failed validation erased destination")
	}
}
