package commands

import (
	"github.com/hyperbricks/hyperbricks/pkg/packagemetadata"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPackageImportInputsParticipateInHash(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, PackageConfigFileName)
	fragment := filepath.Join(root, "config", "runtime.yaml")
	if err := os.WriteFile(entry, []byte("imports: [config/runtime.yaml]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fragment, []byte("hyperbricks: {mode: live}"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := collectModuleFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := computeSourceHash(files)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fragment, []byte("hyperbricks: {mode: debug}"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err = collectModuleFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := computeSourceHash(files)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("import edit did not affect source hash")
	}
	if err := os.WriteFile(entry, []byte("imports: [.cache/settings.yaml]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".cache"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cache/settings.yaml"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := collectModuleFiles(root); err == nil || !strings.Contains(err.Error(), "excluded from the archive") {
		t.Fatalf("filtered import: %v", err)
	}
}

func TestArtifactUsesImportedVersionWithoutFlattening(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, PackageConfigFileName)
	content := []byte("imports: [metadata.yaml]\n")
	if err := os.WriteFile(filepath.Join(root, "metadata.yaml"), []byte("hyperbricks: {metadata: {moduleversion: '2.3.4'}, mode: development}"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := renderPackageArtifact(content, entry, root, packagemetadata.ArtifactOptions{Module: "demo", Format: "hra", FormatVersion: "1", Commit: "unknown", BuiltAt: "2026-10-07T00:00:00Z", HyperBricks: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Metadata.ModuleVersion != "2.3.4" || !strings.Contains(string(result.Content), "metadata.yaml") || strings.Contains(string(result.Content), "mode:") {
		t.Fatalf("artifact flattened or wrong version: %s", result.Content)
	}
}
