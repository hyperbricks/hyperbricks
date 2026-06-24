package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExportStaticZipContainsRenderedFilesOnly(t *testing.T) {
	renderDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, filepath.Join(renderDir, "index.html"), "<main>static-home</main>")
	writeTestFile(t, filepath.Join(renderDir, "static", "coffee.css"), "body{color:#111}")

	zipPath, err := exportStaticZip(renderDir, "demo", outDir, "")
	if err != nil {
		t.Fatalf("exportStaticZip returned error: %v", err)
	}

	files := zipFileNames(t, zipPath)
	for _, want := range []string{"index.html", "static/", "static/coffee.css"} {
		if !files[want] {
			t.Fatalf("expected %s in static zip, got %#v", want, files)
		}
	}
	for _, sourceOnly := range []string{"package.hyperbricks.yaml", "hyperbricks/", "templates/"} {
		if files[sourceOnly] {
			t.Fatalf("did not expect source-only module file %s in static zip", sourceOnly)
		}
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func zipFileNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer reader.Close()

	files := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = true
	}
	return files
}
