package commands

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/assets"
)

func TestPackageRuntimeSnapshotIncludesCurrentFilesAndProvenance(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	archiveDir := filepath.Join(root, "archives")
	if err := os.MkdirAll(filepath.Join(runtimeDir, "hyperbricks", "spaces", "page"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "hyperbricks:\n  metadata:\n    module: demo\n    moduleversion: \"1.0.1\"\n    format: hra\n    commit: old\n    built_at: \"2026-01-01T00:00:00Z\"\n    hyperbricks: " + strings.TrimSpace(assets.VersionMD) + "\n  mode: development\n"
	if err := os.WriteFile(filepath.Join(runtimeDir, PackageConfigFileName), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	spacePath := filepath.Join(runtimeDir, "hyperbricks", "spaces", "page", "nl.hyperbricks.yaml")
	if err := os.WriteFile(spacePath, []byte("title: Current Spaces content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".spaces-writing", ".hb-esbuild-writing", ".hyperbricks-deploy-edit-writing"} {
		if err := os.WriteFile(filepath.Join(runtimeDir, name), []byte("transient"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	opts := RuntimeSnapshotOptions{Module: "demo", RuntimeDir: runtimeDir, ArchiveDir: archiveDir,
		OriginBuildID: "parent-id", OriginCommit: "source7", HyperBricks: strings.TrimSpace(assets.VersionMD)}
	first, err := PackageRuntimeSnapshot(opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PackageRuntimeSnapshot(opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.BuildID == second.BuildID {
		t.Fatal("repeated duplication reused a build ID")
	}
	if first.ModuleVersion != "1.0.1" || first.Commit != "source7" {
		t.Fatalf("snapshot metadata = %#v", first)
	}
	if got := archiveContentBuildID(t, first.ArchivePath); got != first.BuildID {
		t.Fatalf("archive content ID = %q, filename ID = %q", got, first.BuildID)
	}
	archive, err := zip.OpenReader(first.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	entries := make(map[string]string)
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[entry.Name] = string(content)
	}
	if !strings.Contains(entries[PackageConfigFileName], "origin_build_id: parent-id") ||
		!strings.Contains(entries[PackageConfigFileName], "commit: source7") ||
		!strings.Contains(entries[PackageConfigFileName], "moduleversion: \"1.0.1\"") {
		t.Fatalf("derived metadata missing: %s", entries[PackageConfigFileName])
	}
	if entries["hyperbricks/spaces/page/nl.hyperbricks.yaml"] != "title: Current Spaces content\n" {
		t.Fatal("new Spaces content was not captured")
	}
	for name := range entries {
		if isRuntimeStagingFile(filepath.Base(name)) {
			t.Fatalf("staging file entered archive: %s", name)
		}
	}
}

func TestVerifyRuntimeSnapshotRejectsChangedSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	snapshot := filepath.Join(root, "snapshot")
	for _, dir := range []string{source, snapshot} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(source, "content.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyRuntimeSnapshot(context.Background(), source, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyRuntimeSnapshot(context.Background(), source, snapshot); !errors.Is(err, ErrRuntimeChanged) {
		t.Fatalf("verify changed source = %v, want ErrRuntimeChanged", err)
	}
}

func TestPackageRuntimeSnapshotRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	archiveDir := filepath.Join(root, "archives")
	if err := os.Mkdir(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), filepath.Join(runtimeDir, "linked")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := PackageRuntimeSnapshot(RuntimeSnapshotOptions{Module: "demo", RuntimeDir: runtimeDir,
		ArchiveDir: archiveDir, OriginBuildID: "parent", HyperBricks: "v1"})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("symlink snapshot error = %v", err)
	}
}

func TestPackageRuntimeSnapshotRejectsOversizedRuntime(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	archiveDir := filepath.Join(root, "archives")
	if err := os.Mkdir(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	large, err := os.Create(filepath.Join(runtimeDir, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(MaxRuntimeSnapshotBytes + 1); err != nil {
		large.Close()
		t.Skipf("sparse files unsupported: %v", err)
	}
	large.Close()
	_, err = PackageRuntimeSnapshot(RuntimeSnapshotOptions{Module: "demo", RuntimeDir: runtimeDir,
		ArchiveDir: archiveDir, OriginBuildID: "parent", HyperBricks: "v1"})
	if !errors.Is(err, ErrRuntimeSnapshotTooLarge) {
		t.Fatalf("oversized runtime error = %v", err)
	}
}

func TestPackageRuntimeSnapshotHonorsCanceledRequest(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	archiveDir := filepath.Join(root, "archives")
	if err := os.Mkdir(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PackageRuntimeSnapshot(RuntimeSnapshotOptions{Context: ctx, Module: "demo", RuntimeDir: runtimeDir,
		ArchiveDir: archiveDir, OriginBuildID: "parent", HyperBricks: "v1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot error = %v", err)
	}
	entries, err := os.ReadDir(archiveDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("canceled snapshot left files: %#v, %v", entries, err)
	}
}

func archiveContentBuildID(t *testing.T, archivePath string) string {
	t.Helper()
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	hasher := sha256.New()
	for _, entry := range archive.File {
		if _, err := io.WriteString(hasher, entry.Name+"\n"); err != nil {
			t.Fatal(err)
		}
		if entry.FileInfo().IsDir() {
			if _, err := io.WriteString(hasher, "dir\n"); err != nil {
				t.Fatal(err)
			}
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(hasher, reader); err != nil {
			reader.Close()
			t.Fatal(err)
		}
		reader.Close()
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
