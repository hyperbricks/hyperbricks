package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"golang.org/x/net/html"
)

func TestEsbuildYAMLInheritedHeadAndReload(t *testing.T) {
	t.Run("fixed", func(t *testing.T) { testEsbuildYAMLInheritedHeadAndReload(t, false) })
	t.Run("versioned", func(t *testing.T) { testEsbuildYAMLInheritedHeadAndReload(t, true) })
}

func testEsbuildYAMLInheritedHeadAndReload(t *testing.T, fingerprint bool) {
	setupLiveModeServeContentTest(t)
	config := shared.GetHyperBricksConfiguration()
	oldConfig := *config
	oldRoot, oldDirectories, oldParser := commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig
	oldSections, oldErrors := hypermediasBySection, routeSourceErrors
	oldDiagnostics, oldOrder, oldSequence := renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq
	t.Cleanup(func() {
		*config = oldConfig
		commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig = oldRoot, oldDirectories, oldParser
		hypermediasBySection, routeSourceErrors = oldSections, oldErrors
		renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq = oldDiagnostics, oldOrder, oldSequence
		parser.ClearTemplateStore()
	})
	root := t.TempDir()
	commands.ModuleRoot = root
	config.Directories = make(map[string]string)
	for _, name := range []string{"resources", "static", "templates", "hyperbricks", "render", "plugins"} {
		config.Directories[name] = filepath.Join(root, "custom-"+name)
		if err := os.MkdirAll(config.Directories[name], 0755); err != nil {
			t.Fatal(err)
		}
	}
	config.Development.FrontendErrors = false
	config.Server.Beautify = false
	config.Development.WatchDirs = []string{"hyperbricks", "templates", "resources"}
	parser.HbConfig = map[string]interface{}{}
	initializeComponents()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	entry := filepath.Join(config.Directories["resources"], "js", "main.js")
	output := filepath.Join(config.Directories["static"], "js", "bundle.min.main.js")
	write(entry, `import {label} from "./label.js"; console.log(label);`)
	dep := filepath.Join(filepath.Dir(entry), "label.js")
	write(dep, `export const label = "initial";`)
	source := `esbuild:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/bundle.min.main.js}
  - minify: "true"
  - minifyident: "true"
  - mangle: "false"
  - sourcemap: "true"
  - debug: "false"
  - cache: "true"
  - enclose: '<script src="|" defer></script>'
page:
  - type: hypermedia
  - route: index
  - nocache: true
  - head:
      - type: head
      - first:
          - type: html
          - value: '<meta name="before">'
      - any_name:
          - inherit: esbuild
      - last:
          - type: html
          - value: '<meta name="after">'
`
	if fingerprint {
		source = strings.Replace(source, "  - cache: \"true\"\n", "  - cache: \"true\"\n  - fingerprint: true\n", 1)
	}
	write(filepath.Join(config.Directories["hyperbricks"], "page.hyperbricks.yaml"), source)
	load := func() {
		t.Helper()
		if err := PreProcessAndPopulateConfigs(); err != nil {
			t.Fatal(err)
		}
	}
	load()
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("module loading eagerly built the asset")
	}
	check := func(want string) {
		t.Helper()
		writer := httptest.NewRecorder()
		ServeContent(writer, httptest.NewRequest("GET", "/", nil))
		body := writer.Body.String()
		assetURL := esbuildScriptURL(t, body)
		if writer.Code != 200 || !strings.HasPrefix(assetURL, "/static/js/bundle.min.main.") || !strings.HasSuffix(assetURL, ".js") {
			t.Fatalf("status=%d body=%s", writer.Code, body)
		}
		if fingerprint == (assetURL == "/static/js/bundle.min.main.js") {
			t.Fatalf("wrong output naming policy: %s", assetURL)
		}
		output = filepath.Join(config.Directories["static"], strings.TrimPrefix(assetURL, "/static/"))
		if count := writer.Header().Get(renderErrorCountHeader); count != "" && count != "0" {
			t.Fatalf("render errors=%s body=%s", count, body)
		}
		if strings.Index(body, `name="before"`) > strings.Index(body, "<script") || strings.Index(body, "<script") > strings.Index(body, `name="after"`) {
			t.Fatal("inherited component position changed")
		}
		data, err := os.ReadFile(output)
		if err != nil || !strings.Contains(string(data), want) {
			t.Fatalf("asset=%s err=%v", data, err)
		}
	}
	check("initial")
	if !isEsbuildOutput(output) {
		t.Fatal("watcher would reload on generated output")
	}
	if isEsbuildOutput(entry) {
		t.Fatal("watcher ignores the source")
	}
	write(dep, `export const label = "updated";`)
	config.Mode = shared.DEVELOPMENT_MODE
	load()
	check("updated")
	watch := resolveDevelopmentWatchDirectories(config)
	if !strings.Contains(strings.Join(watch, "\n"), config.Directories["resources"]) {
		t.Fatal("configured resources missing from development watch")
	}
	write(filepath.Join(config.Directories["hyperbricks"], "second.hyperbricks.yaml"), `second:
  - type: hypermedia
  - route: second
  - script:
      - type: esbuild
      - entry:
          path: {base: resources, path: js/main.js}
      - outfile:
          path: {base: static, path: js/bundle.min.main.js}
      - minify: false
`)
	load()
	if len(getRouteSourceErrors("index")) == 0 || len(getRouteSourceErrors("second")) == 0 {
		t.Fatal("conflicting output configs not diagnosed for both routes")
	}
	if r, ok := rm.GetRenderComponent(component.EsbuildConfigGetName()).(*component.EsbuildRenderer); !ok || r == nil {
		t.Fatal("native renderer missing")
	}
}

func esbuildScriptURL(t *testing.T, body string) string {
	t.Helper()
	tokens := html.NewTokenizer(strings.NewReader(body))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			if err := tokens.Err(); err != io.EOF {
				t.Fatal(err)
			}
			t.Fatalf("no script URL in %s", body)
		case html.StartTagToken:
			token := tokens.Token()
			if token.Data == "script" {
				for _, attr := range token.Attr {
					if attr.Key == "src" {
						return attr.Val
					}
				}
			}
		}
	}
}
