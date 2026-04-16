package commands

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveStarterVersionPrefersLatestCompatible(t *testing.T) {
	starters := map[string]map[string]StarterMeta{
		"hello-world": {
			"1.0.0": {
				Name:                  "hello-world",
				Version:               "1.0.0",
				CompatibleHyperbricks: []string{">=0.1.0-alpha"},
			},
			"1.1.0": {
				Name:                  "hello-world",
				Version:               "1.1.0",
				CompatibleHyperbricks: []string{">=0.1.0-alpha"},
			},
			"2.0.0": {
				Name:                  "hello-world",
				Version:               "2.0.0",
				CompatibleHyperbricks: []string{">=999.0.0-alpha"},
			},
		},
	}

	meta, err := resolveStarterVersion(starters, "hello-world", "")
	if err != nil {
		t.Fatalf("resolveStarterVersion returned error: %v", err)
	}
	if meta.Version != "1.1.0" {
		t.Fatalf("expected latest compatible version 1.1.0, got %s", meta.Version)
	}
}

func TestRunInitStarterGetDownloadsAndExtractsStarter(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer os.Chdir(prevWD)

	indexPayload := map[string]map[string]StarterMeta{
		"hello-world": {
			"1.0.0": {
				Name:                  "hello-world",
				Version:               "1.0.0",
				Path:                  "starters/hello-world/1.0.0",
				Entrypoint:            "package.hyperbricks",
				Description:           "Minimal starter",
				CompatibleHyperbricks: []string{">=0.8.0-alpha"},
			},
		},
	}

	archiveBytes := createTestStarterArchive(t, map[string]string{
		"hyperbricks-starters-main/starters/hello-world/1.0.0/package.hyperbricks":                 "$module = {{MODULE_PATH}}\n",
		"hyperbricks-starters-main/starters/hello-world/1.0.0/hyperbricks/hello-world.hyperbricks": "page = <TEXT>\npage.value = HELLO WORLD!\n",
		"hyperbricks-starters-main/starters/hello-world/1.0.0/templates/.gitkeep":                  "",
		"hyperbricks-starters-main/starters/hello-world/1.0.0/static/.gitkeep":                     "",
		"hyperbricks-starters-main/starters/hello-world/1.0.0/resources/.gitkeep":                  "",
		"hyperbricks-starters-main/starters/hello-world/1.0.0/rendered/.gitkeep":                   "",
		"hyperbricks-starters-main/starters/hello-world/1.0.0/logs/.gitkeep":                       "",
		"hyperbricks-starters-main/starters/other-starter/1.0.0/package.hyperbricks":               "ignored\n",
		"hyperbricks-starters-main/README.md":                                                      "ignored\n",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/starters.index.json":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(indexPayload); err != nil {
				t.Fatalf("encode index payload: %v", err)
			}
		case "/archive.zip":
			w.Header().Set("Content-Type", "application/zip")
			if _, err := w.Write(archiveBytes); err != nil {
				t.Fatalf("write archive: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	prevIndexURL := starterIndexURL
	prevArchiveURL := starterArchiveURL
	prevArchiveRoot := starterArchiveRoot
	starterIndexURL = server.URL + "/starters.index.json"
	starterArchiveURL = server.URL + "/archive.zip"
	starterArchiveRoot = "hyperbricks-starters-main"
	defer func() {
		starterIndexURL = prevIndexURL
		starterArchiveURL = prevArchiveURL
		starterArchiveRoot = prevArchiveRoot
	}()

	moduleName, meta, err := runInitStarterGet("hello-world", "example-site")
	if err != nil {
		t.Fatalf("runInitStarterGet returned error: %v", err)
	}
	if moduleName != "example-site" {
		t.Fatalf("expected module name example-site, got %s", moduleName)
	}
	if meta.Version != "1.0.0" {
		t.Fatalf("expected starter version 1.0.0, got %s", meta.Version)
	}

	expectedFiles := []string{
		filepath.Join("modules", "example-site", "package.hyperbricks"),
		filepath.Join("modules", "example-site", "hyperbricks", "hello-world.hyperbricks"),
		filepath.Join("modules", "example-site", "templates"),
		filepath.Join("modules", "example-site", "static"),
		filepath.Join("modules", "example-site", "resources"),
		filepath.Join("modules", "example-site", "rendered"),
		filepath.Join("modules", "example-site", "logs"),
		filepath.Join("bin", "plugins"),
	}
	for _, path := range expectedFiles {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join("modules", "example-site", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("expected starter manifest.json to be excluded from installed module")
	}
}

func TestRunInitStarterGetRejectsNonEmptyModuleDir(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer os.Chdir(prevWD)

	indexPayload := map[string]map[string]StarterMeta{
		"hello-world": {
			"1.0.0": {
				Name:                  "hello-world",
				Version:               "1.0.0",
				Path:                  "starters/hello-world/1.0.0",
				Entrypoint:            "package.hyperbricks",
				CompatibleHyperbricks: []string{">=0.8.0-alpha"},
			},
		},
	}

	archiveBytes := createTestStarterArchive(t, map[string]string{
		"hyperbricks-starters-main/starters/hello-world/1.0.0/package.hyperbricks": "$module = {{MODULE_PATH}}\n",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/starters.index.json":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(indexPayload); err != nil {
				t.Fatalf("encode index payload: %v", err)
			}
		case "/archive.zip":
			w.Header().Set("Content-Type", "application/zip")
			if _, err := w.Write(archiveBytes); err != nil {
				t.Fatalf("write archive: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	prevIndexURL := starterIndexURL
	prevArchiveURL := starterArchiveURL
	prevArchiveRoot := starterArchiveRoot
	starterIndexURL = server.URL + "/starters.index.json"
	starterArchiveURL = server.URL + "/archive.zip"
	starterArchiveRoot = "hyperbricks-starters-main"
	defer func() {
		starterIndexURL = prevIndexURL
		starterArchiveURL = prevArchiveURL
		starterArchiveRoot = prevArchiveRoot
	}()

	moduleDir := filepath.Join("modules", "hello-world")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("mkdir module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "existing.txt"), []byte("occupied"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	_, _, err = runInitStarterGet("hello-world", "")
	if err == nil {
		t.Fatalf("expected error for non-empty module dir")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("expected non-empty directory error, got: %v", err)
	}
}

func createTestStarterArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for name, contents := range files {
		fileWriter, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := fileWriter.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return buf.Bytes()
}
