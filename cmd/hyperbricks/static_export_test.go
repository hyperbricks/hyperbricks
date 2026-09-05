package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/otiai10/copy"
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

func TestExportStaticZipEsbuildMapsEmbedSources(t *testing.T) {
	type engineCase struct {
		name, binary string
		fingerprint  bool
	}
	engines := []engineCase{{name: "embedded"}, {name: "embedded-versioned", fingerprint: true}}
	if binary := os.Getenv("ESBUILD_TEST_BINARY"); binary != "" {
		engines = append(engines, engineCase{name: "external", binary: binary}, engineCase{name: "external-versioned", binary: binary, fingerprint: true})
	}
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			moduleDir, renderDir := t.TempDir(), t.TempDir()
			staticDir := filepath.Join(moduleDir, "static")
			sources := map[string]string{
				"css/tokens.css": ":root { --ink: #123456; }",
				"css/site.css":   `@import "./tokens.css"; body { color: var(--ink); }`,
				"js/value.ts":    `export const label: string = "standalone export";`,
				"js/main.ts":     `import {label} from "./value"; document.body.dataset.label = label;`,
			}
			for path, contents := range sources {
				writeTestFile(t, filepath.Join(moduleDir, "resources", path), contents)
			}
			writeTestFile(t, filepath.Join(moduleDir, "resources", "server-only.json"), `{"private":"not a browser asset"}`)
			renderer := component.NewEsbuildRenderer(staticDir)
			urls := make(map[string]string)
			for _, asset := range []struct{ entry, outfile string }{{"css/site.css", "css/site.css"}, {"js/main.ts", "js/app.js"}} {
				prepared := renderer.Prepare(component.EsbuildConfig{
					Entry: filepath.Join(moduleDir, "resources", asset.entry), Outfile: filepath.Join(staticDir, asset.outfile),
					Sourcemap: true, Cache: true, Binary: engine.binary, Fingerprint: engine.fingerprint,
				})
				publicURL, errs := prepared.Render(context.Background())
				if len(errs) != 0 {
					t.Fatalf("build %s: %v", asset.entry, errs)
				}
				urls[asset.outfile] = publicURL
			}
			writeTestFile(t, filepath.Join(renderDir, "index.html"), fmt.Sprintf(`<link rel="stylesheet" href="%s"><script src="%s" defer></script>`, urls["css/site.css"], urls["js/app.js"]))
			if err := copy.Copy(staticDir, filepath.Join(renderDir, "static")); err != nil {
				t.Fatal(err)
			}
			zipPath, err := exportStaticZip(renderDir, "esbuild", t.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}
			files := zipFileNames(t, zipPath)
			for original, publicURL := range urls {
				asset := strings.TrimPrefix(publicURL, "/")
				if !files[asset] || !files[asset+".map"] {
					t.Fatalf("HTML references missing exported assets: %s", publicURL)
				}
				if engine.fingerprint && asset == "static/"+original {
					t.Fatal("versioned export still references an unversioned filename")
				}
				if engine.fingerprint {
					base, ext := filepath.Base(asset), filepath.Ext(asset)
					stem := strings.TrimSuffix(base, ext)
					prefix := strings.TrimSuffix(filepath.Base(original), filepath.Ext(original)) + "."
					hash := strings.TrimPrefix(stem, prefix)
					if !strings.HasPrefix(stem, prefix) || hash == "" || hash != strings.ToLower(hash) {
						t.Fatalf("exported entry hash is not lowercase: %s", asset)
					}
				}
			}
			// Inspect only the artifact after deleting the entire temporary source module.
			if err := os.RemoveAll(moduleDir); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.OpenReader(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			foundSources := make(map[string]bool)
			mapCount := 0
			for _, file := range archive.File {
				if strings.HasPrefix(file.Name, "resources/") || strings.HasSuffix(file.Name, ".json") || strings.HasPrefix(filepath.Base(file.Name), ".hb-esbuild-") {
					t.Fatalf("private build input exported: %s", file.Name)
				}
				if !strings.HasSuffix(file.Name, ".map") {
					continue
				}
				mapCount++
				reader, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				var sourceMap struct {
					Sources        []string  `json:"sources"`
					SourcesContent []*string `json:"sourcesContent"`
				}
				if err := json.Unmarshal(data, &sourceMap); err != nil {
					t.Fatal(err)
				}
				if len(sourceMap.Sources) != 2 || len(sourceMap.SourcesContent) != 2 {
					t.Fatalf("%s does not embed both sources: %+v", file.Name, sourceMap)
				}
				for i, name := range sourceMap.Sources {
					// esbuild may resolve the temporary directory's symlink in source labels.
					key := filepath.ToSlash(filepath.Join(filepath.Base(filepath.Dir(name)), filepath.Base(name)))
					want, ok := sources[key]
					if !ok || sourceMap.SourcesContent[i] == nil || *sourceMap.SourcesContent[i] != want {
						t.Fatalf("%s lacks original content for %s", file.Name, name)
					}
					foundSources[key] = true
				}
			}
			if mapCount != 2 || len(foundSources) != len(sources) {
				t.Fatalf("incomplete exported source maps: maps=%d sources=%v", mapCount, foundSources)
			}
		})
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
