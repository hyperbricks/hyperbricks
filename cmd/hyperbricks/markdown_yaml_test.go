package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestMarkdownYAMLPageFragmentLiveAndReload(t *testing.T) {
	setupLiveModeServeContentTest(t)
	config := shared.GetHyperBricksConfiguration()
	oldConfig := *config
	oldRoot, oldDirs, oldParser := commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig
	oldSections, oldErrors := hypermediasBySection, routeSourceErrors
	oldDiagnostics, oldOrder, oldSeq := renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq
	t.Cleanup(func() {
		*config = oldConfig
		commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig = oldRoot, oldDirs, oldParser
		hypermediasBySection, routeSourceErrors = oldSections, oldErrors
		renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq = oldDiagnostics, oldOrder, oldSeq
		parser.ClearTemplateStore()
	})
	root := t.TempDir()
	commands.ModuleRoot = root
	config.Directories = map[string]string{}
	for _, name := range []string{"resources", "hyperbricks", "templates", "static", "render", "plugins"} {
		config.Directories[name] = filepath.Join(root, "custom-"+name)
		if err := os.MkdirAll(config.Directories[name], 0755); err != nil {
			t.Fatal(err)
		}
	}
	config.Server.Beautify = false
	config.Development.FrontendErrors = false
	parser.HbConfig = map[string]interface{}{}
	initializeComponents()
	write := func(p, value string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	doc := filepath.Join(config.Directories["resources"], "read.md")
	write(doc, "# Declared\n")
	write(filepath.Join(config.Directories["resources"], "private.md"), "# Not selected\n")
	write(filepath.Join(config.Directories["hyperbricks"], "page.hyperbricks.yaml"), `document:
  - type: markdown
  - file: read.md
page:
  - type: hypermedia
  - route: index
  - nocache: true
  - template:
      - inline: '<!doctype html><html><body><main>{{.document}}</main></body></html>'
      - values:
          document:
            - inherit: document
fragment:
  - type: fragment
  - route: perspective
  - nocache: true
  - content:
      - inherit: document
inline:
  - type: fragment
  - route: inline
  - content:
      - type: markdown
      - content: {file: {base: resources, path: read.md}}
`)
	load := func() {
		t.Helper()
		if err := PreProcessAndPopulateConfigs(); err != nil {
			t.Fatal(err)
		}
	}
	check := func(route, want string, full bool) {
		t.Helper()
		w := httptest.NewRecorder()
		ServeContent(w, httptest.NewRequest("GET", route+"?file=private.md", nil))
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, want) || strings.Contains(body, "Not selected") || strings.Contains(body, "<html>") != full {
			t.Fatalf("status=%d body=%s", w.Code, body)
		}
		if n := w.Header().Get(renderErrorCountHeader); n != "" && n != "0" {
			t.Fatalf("errors=%s body=%s", n, body)
		}
	}
	load()
	if _, plan, found := getConfigAndPlan("index"); !found || plan == nil {
		t.Fatal("Markdown page was not compiled in live mode")
	}
	check("/", "Declared", true)
	check("/perspective", "Declared", false)
	check("/inline", "Declared", false)
	write(doc, "# Updated\n")
	check("/", "Updated", true)
	load()
	check("/inline", "Updated", false)
	_, plan, _ := getConfigAndPlan("index")
	out, errs := plan.Render(context.Background())
	if len(errs) != 0 || !strings.Contains(out, "Updated") {
		t.Fatalf("requestless render: %s %v", out, errs)
	}
	config.Mode = shared.DEVELOPMENT_MODE
	load()
	check("/", "Updated", true)
	if err := os.Remove(doc); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	ServeContent(w, httptest.NewRequest("GET", "/perspective", nil))
	if count := w.Header().Get(renderErrorCountHeader); count == "" || count == "0" {
		t.Fatal("missing file had no render diagnostic")
	}
}
