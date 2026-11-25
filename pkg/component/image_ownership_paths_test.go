package component

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetOwnershipDirectoryIgnoresBuildCacheEnvironment(t *testing.T) {
	before, err := assetOwnershipDirectory("images")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"XDG_CACHE_HOME", "LOCALAPPDATA", "TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, t.TempDir())
	}
	after, err := assetOwnershipDirectory("images")
	if err != nil || after != before {
		t.Fatalf("ownership moved with cache environment: %s -> %s (%v)", before, after, err)
	}
}

func TestImageOwnershipLegacyReadAndMigration(t *testing.T) {
	previous, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true}
	name := imageGenerate(t, previous, src, dest, cfg)
	old, err := previous.storeFor(dest)
	if err != nil {
		t.Fatal(err)
	}
	before := imageInfo(t, filepath.Join(dest, name))
	legacyBytes := esbuildRead(t, old.historyPath)
	unrelated := filepath.Join(previous.cacheDir, "other-root.json")
	esbuildWrite(t, unrelated, "leave this history alone")
	canonical := t.TempDir()
	reader := NewImageProcessor()
	reader.cacheDir, reader.ownershipDir = previous.cacheDir, canonical
	store, err := reader.storeFor(dest)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.load(); err != nil {
		t.Fatal(err)
	}
	if len(store.history.Owners) != 1 || esbuildRead(t, old.historyPath) != legacyBytes {
		t.Fatal("read-only history load changed or lost legacy ownership")
	}
	if _, err := os.Stat(store.historyPath); !os.IsNotExist(err) {
		t.Fatalf("read-only load migrated ownership: %v", err)
	}
	if got := imageGenerate(t, reader, src, dest, cfg); got != name {
		t.Fatal("migration changed the public image URL")
	}
	if !os.SameFile(before, imageInfo(t, filepath.Join(dest, name))) {
		t.Fatal("migration rebuilt the existing generated image")
	}
	if _, err := os.Stat(old.historyPath); !os.IsNotExist(err) {
		t.Fatalf("exact legacy record was not retired after migration: %v", err)
	}
	if esbuildRead(t, store.historyPath) != legacyBytes || esbuildRead(t, unrelated) != "leave this history alone" {
		t.Fatal("migration changed records outside its exact history")
	}
	// A process with a different build cache sees the same ownership record.
	restart := NewImageProcessor()
	restart.cacheDir, restart.ownershipDir = t.TempDir(), canonical
	if got := imageGenerate(t, restart, src, dest, cfg); got != name || !os.SameFile(before, imageInfo(t, filepath.Join(dest, name))) {
		t.Fatal("different build cache lost canonical ownership")
	}
}

func TestImageOwnershipMigrationFailuresPreserveLegacy(t *testing.T) {
	for _, fault := range []string{"legacy-invalid", "canonical-invalid", "canonical-unwritable"} {
		t.Run(fault, func(t *testing.T) {
			previous, src, dest := cachedImageFixture(t)
			cfg := SingleImageConfig{Width: 4, Cache: true}
			name := imageGenerate(t, previous, src, dest, cfg)
			old, err := previous.storeFor(dest)
			if err != nil {
				t.Fatal(err)
			}
			before := imageInfo(t, filepath.Join(dest, name))
			canonical := t.TempDir()
			if fault == "canonical-unwritable" {
				file := filepath.Join(canonical, "file")
				esbuildWrite(t, file, "not a directory")
				canonical = file
			}
			next := NewImageProcessor()
			next.cacheDir, next.ownershipDir = previous.cacheDir, canonical
			store, err := next.storeFor(dest)
			if err != nil {
				t.Fatal(err)
			}
			if fault == "legacy-invalid" {
				esbuildWrite(t, old.historyPath, "{")
			} else if fault == "canonical-invalid" {
				esbuildWrite(t, store.historyPath, "{")
			}
			legacy := esbuildRead(t, old.historyPath)
			if _, err := next.processImage(src, dest, cfg); err == nil {
				t.Fatal("accepted invalid or unpublishable ownership")
			}
			if esbuildRead(t, old.historyPath) != legacy || !os.SameFile(before, imageInfo(t, filepath.Join(dest, name))) {
				t.Fatal("failed migration changed legacy evidence or public output")
			}
		})
	}
}

func TestImageFreshExportResetsCanonicalAndExactLegacyOwnership(t *testing.T) {
	previous, src, dest := cachedImageFixture(t)
	cfg := SingleImageConfig{Width: 4, Cache: true, CacheGrace: "0s"}
	name := imageGenerate(t, previous, src, dest, cfg)
	old, err := previous.storeFor(dest)
	if err != nil {
		t.Fatal(err)
	}
	canonical := t.TempDir()
	fresh := NewImageProcessor()
	fresh.cacheDir, fresh.ownershipDir = previous.cacheDir, canonical
	if err := fresh.SetOutputRoot(t.TempDir(), filepath.Dir(dest)); err != nil {
		t.Fatal(err)
	}
	store, err := fresh.storeFor(dest)
	if err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, store.historyPath, esbuildRead(t, old.historyPath))
	if err := fresh.ResetAssetsForFreshExport(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	if err := fresh.ReconcileAssets(dest, map[string]imagePolicy{}); err != nil {
		t.Fatal(err)
	}
	imageInfo(t, filepath.Join(dest, name))
	for _, history := range []string{store.historyPath, old.historyPath} {
		if _, err := os.Stat(history); !os.IsNotExist(err) {
			t.Fatalf("fresh export retained old ownership %s: %v", history, err)
		}
	}
}
