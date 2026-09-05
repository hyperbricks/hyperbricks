package main

import (
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

func TestStaticPathsDemo(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	moduleDir := filepath.Join(repoRoot, "modules", "static-paths-demo")
	otherRoot := t.TempDir()
	moduleMarker := "Served from the module-relative static directory."
	rootMarker := "Served from the root-relative static directory."
	otherRootMarker := "Served from the alternate CLI working directory."
	otherAssets := filepath.Join(otherRoot, "modules", "static-paths-demo", "root-assets")
	if err := os.CopyFS(otherAssets, os.DirFS(filepath.Join(moduleDir, "root-assets"))); err != nil {
		t.Fatalf("copy root assets into another working directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherAssets, "thefile.txt"), []byte(otherRootMarker+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		workingDir string
		configFile string
		staticDir  string
		marker     string
	}{
		{"module_base", repoRoot, "package.hyperbricks.yaml", filepath.Join(moduleDir, "static"), moduleMarker},
		{"root_base", repoRoot, "package.root.hyperbricks.yaml", filepath.Join("modules", "static-paths-demo", "root-assets"), rootMarker},
		{"module_base_other_cwd", otherRoot, "package.hyperbricks.yaml", filepath.Join(moduleDir, "static"), moduleMarker},
		{"root_base_other_cwd", otherRoot, "package.root.hyperbricks.yaml", filepath.Join("modules", "static-paths-demo", "root-assets"), otherRootMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(tc.workingDir)
			oldDiagnosticsSeq := renderDiagnosticsSeq
			setupDevelopmentModeServeContentTest(t, false)
			hbConfig := shared.GetHyperBricksConfiguration()
			oldConfig := *hbConfig
			oldModuleRoot := commands.ModuleRoot
			oldRenderStatic := commands.RenderStatic
			oldModuleDirectories := core.ModuleDirectories
			oldParserConfig := parser.HbConfig
			oldHypermedias := hypermediasBySection
			oldSourceErrors := routeSourceErrors
			parser.ClearTemplateStore()
			t.Cleanup(func() {
				*hbConfig = oldConfig
				commands.ModuleRoot = oldModuleRoot
				commands.RenderStatic = oldRenderStatic
				core.ModuleDirectories = oldModuleDirectories
				parser.HbConfig = oldParserConfig
				parser.ClearTemplateStore()
				renderDiagnosticsSeq = oldDiagnosticsSeq
				hypermediasMutex.Lock()
				hypermediasBySection = oldHypermedias
				hypermediasMutex.Unlock()
				routeSourceErrorsMutex.Lock()
				routeSourceErrors = oldSourceErrors
				routeSourceErrorsMutex.Unlock()
			})

			packageConfig, err := shared.LoadPackageConfigMap(filepath.Join(moduleDir, tc.configFile), moduleDir)
			if err != nil {
				t.Fatalf("load example package: %v", err)
			}
			hbConfig.Directories = map[string]string{}
			hbConfig.Plugins.Enabled = nil
			if err := mapstructure.WeakDecode(packageConfig["hyperbricks"], hbConfig); err != nil {
				t.Fatalf("decode example package: %v", err)
			}
			if got := hbConfig.Directories["static"]; got != tc.staticDir {
				t.Fatalf("resolved static directory = %q, want %q", got, tc.staticDir)
			}
			commands.ModuleRoot = moduleDir
			commands.RenderStatic = false
			parser.HbConfig = packageConfig
			initializeComponents()
			if err := PreProcessAndPopulateConfigs(); err != nil {
				t.Fatalf("load example routes: %v", err)
			}
			app := buildRuntimeHandler(nil)
			request := func(path string) *httptest.ResponseRecorder {
				response := httptest.NewRecorder()
				app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				return response
			}

			for _, path := range []string{"/", "/nested/demo"} {
				response := request(path)
				if response.Code != http.StatusOK {
					t.Fatalf("GET %s = %d: %s", path, response.Code, response.Body.String())
				}
				if count := response.Header().Get(renderErrorCountHeader); count != "" && count != "0" {
					t.Fatalf("GET %s has %s render errors", path, count)
				}
				for _, want := range []string{
					`href="/static/thefile.txt"`,
					`src="/static/thefile.txt"`,
					`href="/static/demo.css"`,
					`<code id="resolved-static-file">` + html.EscapeString(filepath.Join(tc.staticDir, "thefile.txt")) + `</code>`,
				} {
					if !strings.Contains(response.Body.String(), want) {
						t.Fatalf("GET %s missing %q: %s", path, want, response.Body.String())
					}
				}
			}

			for _, path := range []string{"/static/thefile.txt", "/static/demo.css"} {
				response := request(path)
				if response.Code != http.StatusOK {
					t.Fatalf("GET %s = %d: %s", path, response.Code, response.Body.String())
				}
				if path == "/static/thefile.txt" && strings.TrimSpace(response.Body.String()) != tc.marker {
					t.Fatalf("GET %s = %q, want %q", path, response.Body.String(), tc.marker)
				}
				if path == "/static/demo.css" && !strings.HasPrefix(response.Header().Get("Content-Type"), "text/css") {
					t.Fatalf("GET %s content type = %q, want text/css", path, response.Header().Get("Content-Type"))
				}
			}

			for _, path := range []string{
				"/nested/static/thefile.txt",
				"/modules/static-paths-demo/static/thefile.txt",
				"/modules/static-paths-demo/root-assets/thefile.txt",
				"/root-assets/thefile.txt",
				"/" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(tc.staticDir, "thefile.txt")), "/"),
			} {
				response := request(path)
				if response.Code != http.StatusNotFound {
					t.Fatalf("GET disk/relative-looking URL %s = %d, want 404", path, response.Code)
				}
				if strings.Contains(response.Body.String(), moduleMarker) || strings.Contains(response.Body.String(), rootMarker) || strings.Contains(response.Body.String(), otherRootMarker) {
					t.Fatalf("GET %s exposed an asset outside /static/", path)
				}
			}
		})
	}
}
