package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestStaticSnapshotRejectsMalformedPackageBlocks(t *testing.T) {
	oldConfig := parser.HbConfig
	t.Cleanup(func() { parser.HbConfig = oldConfig })

	for _, tc := range []struct {
		name string
		raw  interface{}
		want string
	}{
		{"scalar", "products", "hyperbricks.static must be a mapping"},
		{"list", []interface{}{map[string]interface{}{"path": "/products"}}, "hyperbricks.static must be a mapping"},
		{"null", nil, "hyperbricks.static must be a mapping"},
		{"crawl scalar", map[string]interface{}{"crawl": true}, "hyperbricks.static.crawl must be a mapping"},
		{"crawl list", map[string]interface{}{"crawl": []interface{}{}}, "hyperbricks.static.crawl must be a mapping"},
		{"crawl null", map[string]interface{}{"crawl": nil}, "hyperbricks.static.crawl must be a mapping"},
		{"routes mapping", map[string]interface{}{"routes": map[string]interface{}{"path": "/products"}}, "static.routes must be a list"},
		{"variant scalar", map[string]interface{}{"variants": []interface{}{"/products"}}, "static.variants[0] must be an object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser.HbConfig = map[string]interface{}{"hyperbricks": map[string]interface{}{"static": tc.raw}}
			renderDir := t.TempDir()
			err := snapshotStaticRoutes(map[string]map[string]interface{}{
				"products": {"route": "products"},
			}, renderDir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			files, err := os.ReadDir(renderDir)
			if err != nil || len(files) != 0 {
				t.Fatalf("malformed package produced output: files=%v error=%v", files, err)
			}
		})
	}
}

func TestStaticSnapshotOptionalPackageBlockPreservesDiscovery(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	oldConfig := parser.HbConfig
	t.Cleanup(func() { parser.HbConfig = oldConfig })

	for _, tc := range []struct {
		name string
		root map[string]interface{}
	}{
		{"absent package", nil},
		{"absent static", map[string]interface{}{"hyperbricks": map[string]interface{}{}}},
		{"empty static", map[string]interface{}{"hyperbricks": map[string]interface{}{"static": map[string]interface{}{}}}},
		{"empty crawl", map[string]interface{}{"hyperbricks": map[string]interface{}{"static": map[string]interface{}{"crawl": map[string]interface{}{}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser.HbConfig = tc.root
			targets, err := collectStaticSnapshotTargets(map[string]map[string]interface{}{
				"products": {"route": "products"},
			})
			if err != nil || len(targets) != 1 || targets[0].OutputPath != "products.html" {
				t.Fatalf("automatic discovery = %v, %v; want products.html", targets, err)
			}
		})
	}
}

func TestStaticSnapshotCrawlAliasesRemainSupported(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	oldConfig := parser.HbConfig
	t.Cleanup(func() { parser.HbConfig = oldConfig })
	parser.HbConfig = map[string]interface{}{
		"hyperbricks": map[string]interface{}{
			"static": map[string]interface{}{
				"crawl": map[string]interface{}{
					"routes": []interface{}{map[string]interface{}{"path": "/products"}},
					"variants": []interface{}{map[string]interface{}{
						"path": "/products?category=shoes", "output": "shoes.html",
					}},
				},
			},
		},
	}
	targets, err := collectStaticSnapshotTargets(nil)
	if err != nil || len(targets) != 2 {
		t.Fatalf("crawl targets = %v, %v; want both legacy lists", targets, err)
	}
	if targets[0].OutputPath != "products.html" || targets[1].Query.Get("category") != "shoes" || targets[1].OutputPath != "shoes.html" {
		t.Fatalf("crawl targets lost route or variant settings: %v", targets)
	}
}

type staticPackageExamplePlugin struct{}

func (staticPackageExamplePlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	req := ctx.Value(shared.Request).(*http.Request)
	return fmt.Sprintf("<main>path=%s category=%s tags=%s host=%s language=%s</main>",
		req.URL.Path, req.URL.Query().Get("category"), strings.Join(req.URL.Query()["tag"], ","),
		req.Host, req.Header.Get("Accept-Language")), nil
}

func TestStaticPackageDocumentationExportsAndServesStandaloneZip(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	oldConfig := parser.HbConfig
	t.Cleanup(func() { parser.HbConfig = oldConfig })

	// Load the actual documentation example through the package loader so the
	// documented configuration is exercised, including its resolved directories.
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "HYPERBRICKS_CLI.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(doc), "### Package configuration\n")
	if !ok {
		t.Fatal("missing static package configuration section")
	}
	_, example, ok := strings.Cut(section, "```yaml\n")
	if !ok {
		t.Fatal("missing package YAML example")
	}
	example, _, ok = strings.Cut(example, "\n```")
	if !ok {
		t.Fatal("unterminated package YAML example")
	}
	moduleDir := t.TempDir()
	configFile := filepath.Join(moduleDir, "package.hyperbricks.yaml")
	writeTestFile(t, configFile, example)
	parser.HbConfig, err = shared.LoadPackageConfigMap(configFile, moduleDir)
	if err != nil {
		t.Fatalf("load documented package: %v", err)
	}

	rm.SetPlugin("static_package_example_test", staticPackageExamplePlugin{})
	routes := map[string]map[string]interface{}{}
	for _, route := range []string{"index", "products"} {
		config := map[string]interface{}{
			"@type":  component.PluginRenderGetName(),
			"route":  route,
			"plugin": "static_package_example_test",
		}
		routes[route] = config
		setTestRouteConfig(route, config)
	}
	renderDir := filepath.Join(moduleDir, "rendered")
	if err := snapshotStaticRoutes(routes, renderDir); err != nil {
		t.Fatalf("snapshot documented targets: %v", err)
	}
	zipPath, err := exportStaticZip(renderDir, "documentation-test", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	extracted := t.TempDir()
	if err := os.CopyFS(extracted, archive); err != nil {
		t.Fatalf("extract static ZIP: %v", err)
	}
	if err := os.RemoveAll(moduleDir); err != nil {
		t.Fatal(err)
	}

	// Serving the extracted archive works after the source/render directories
	// have gone, without HyperBricks or the rendering plugin handling requests.
	server := httptest.NewServer(http.FileServer(http.Dir(extracted)))
	defer server.Close()
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/", "path=/index category= tags="},
		{"/products.html", "path=/products category= tags= host=catalog.example.test language=en"},
		{"/products/shoes.html", "path=/products category=shoes tags=sale,summer host=catalog.example.test language=en"},
		{"/products/hats.html", "path=/products category=hats tags= host=catalog.example.test language=nl"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response, err := server.Client().Get(server.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), tc.want) {
				t.Fatalf("static response status=%d body=%q error=%v; want %q", response.StatusCode, body, err, tc.want)
			}
		})
	}
}
