package main

import (
	"debug/buildinfo"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.uber.org/zap"
)

func TestProcessScriptIndexesAPIFragmentRenderWithScalarTemplate(t *testing.T) {
	shared.Init_configuration()

	config := map[string]interface{}{
		"api_panel": map[string]interface{}{
			"@type":    composite.ApiFragmentRenderConfigGetName(),
			"route":    "api/panel",
			"title":    "API Panel",
			"section":  "api",
			"endpoint": "http://example.invalid/api",
			"method":   "GET",
			"template": "api/panel.html",
		},
	}
	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	tempRouteSourceErrors := make(map[string][]error)
	filenameToRoutes := make(map[string][]string)

	err := processScript("test", config, nil, tempConfigs, tempHyperMediasBySection, tempRouteSourceErrors, zap.NewNop().Sugar(), filenameToRoutes)
	if err != nil {
		t.Fatalf("processScript returned error: %v", err)
	}

	if _, ok := tempConfigs["api/panel"]; !ok {
		t.Fatalf("expected API fragment route to be indexed; routes: %#v", tempConfigs)
	}
	if got := filenameToRoutes["test"]; len(got) != 1 || got[0] != "api/panel" {
		t.Fatalf("expected filename route mapping [api/panel], got %#v", got)
	}
	if got := tempHyperMediasBySection["api"]; len(got) != 1 || got[0].Route != "api/panel" {
		t.Fatalf("expected menu metadata for api/panel, got %#v", got)
	}
}

func TestProcessScriptRejectsFragmentScalarTemplate(t *testing.T) {
	shared.Init_configuration()

	config := map[string]interface{}{
		"bad_fragment": map[string]interface{}{
			"@type":    composite.FragmentConfigGetName(),
			"route":    "bad-fragment",
			"title":    "Bad Fragment",
			"section":  "api",
			"template": "fragment.html",
		},
	}
	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	tempRouteSourceErrors := make(map[string][]error)
	filenameToRoutes := make(map[string][]string)

	err := processScript("test", config, nil, tempConfigs, tempHyperMediasBySection, tempRouteSourceErrors, zap.NewNop().Sugar(), filenameToRoutes)
	if err != nil {
		t.Fatalf("processScript returned error: %v", err)
	}

	if _, ok := tempConfigs["bad-fragment"]; ok {
		t.Fatalf("expected scalar <FRAGMENT>.template to be rejected")
	}
}

func TestYAMLDiagnosticsToComponentErrorsFormatsResolverDiagnosticsWithoutZeroPosition(t *testing.T) {
	errors := yamlDiagnosticsToComponentErrors([]yamlparser.Diagnostic{{
		Level:   "warning",
		Code:    "var_missing",
		Source:  "/tmp/project/page.hyperbricks.yaml",
		Path:    "page.title",
		Message: `missing var "title"; resolved as empty string`,
	}})
	if len(errors) != 1 {
		t.Fatalf("errors len = %d, want 1", len(errors))
	}
	diagnostic, ok := errors[0].(shared.ComponentError)
	if !ok {
		t.Fatalf("diagnostic type = %T, want shared.ComponentError", errors[0])
	}
	if diagnostic.Level != "WARNING" || diagnostic.File != "page" || diagnostic.Path != "page.title" {
		t.Fatalf("diagnostic fields = %#v", diagnostic)
	}
	if strings.Contains(diagnostic.Err, ":0:0") {
		t.Fatalf("diagnostic should not expose zero YAML position: %#v", diagnostic.Err)
	}
	if !strings.Contains(diagnostic.Err, "(source: page.hyperbricks.yaml)") {
		t.Fatalf("diagnostic source = %#v", diagnostic.Err)
	}
}

func TestPreProcessAndPopulateConfigsKeepsRunningWhenYAMLSourceIsInvalid(t *testing.T) {
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldBeautify := hbConfig.Server.Beautify
	oldDirectories := hbConfig.Directories
	oldModuleRoot := commands.ModuleRoot
	oldModuleDirectories := core.ModuleDirectories
	oldConfigs := configs
	oldHypermediasBySection := hypermediasBySection
	oldRouteSourceErrors := routeSourceErrors
	oldRM := rm
	oldParserHbConfig := parser.HbConfig
	oldRenderDiagnosticsSeq := renderDiagnosticsSeq

	renderDiagnosticsMutex.Lock()
	oldRenderDiagnostics := renderDiagnostics
	oldRenderDiagnosticsOrder := renderDiagnosticsOrder
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	renderDiagnosticsMutex.Unlock()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Server.Beautify = oldBeautify
		hbConfig.Directories = oldDirectories
		commands.ModuleRoot = oldModuleRoot
		core.ModuleDirectories = oldModuleDirectories
		rm = oldRM
		parser.HbConfig = oldParserHbConfig

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermediasBySection
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldRouteSourceErrors
		routeSourceErrorsMutex.Unlock()

		renderDiagnosticsSeq = oldRenderDiagnosticsSeq
		renderDiagnosticsMutex.Lock()
		renderDiagnostics = oldRenderDiagnostics
		renderDiagnosticsOrder = oldRenderDiagnosticsOrder
		renderDiagnosticsMutex.Unlock()
	})

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	moduleDir := filepath.Join(repoRoot, "modules", "yaml-invalid-source")
	hyperbricksDir := filepath.Join(moduleDir, "hyperbricks")
	if _, err := os.Stat(filepath.Join(hyperbricksDir, "broken.hyperbricks.yaml")); err != nil {
		t.Fatalf("invalid YAML fixture module is missing: %v", err)
	}

	commands.ModuleRoot = moduleDir
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Server.Beautify = false
	hbConfig.Directories = map[string]string{
		"hyperbricks": hyperbricksDir,
		"templates":   filepath.Join(moduleDir, "templates"),
		"resources":   filepath.Join(moduleDir, "resources"),
		"static":      filepath.Join(moduleDir, "static"),
		"render":      filepath.Join(moduleDir, "rendered"),
		"plugins":     filepath.Join(moduleDir, "plugins"),
	}

	configs = make(map[string]map[string]interface{})
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)
	parser.HbConfig = map[string]interface{}{}

	initializeComponents()
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatalf("PreProcessAndPopulateConfigs returned error: %v", err)
	}
	if _, ok := getConfig("valid-route"); !ok {
		t.Fatalf("valid YAML route was not indexed; configs: %#v", configs)
	}
	if _, ok := getConfig("broken-route"); ok {
		t.Fatalf("invalid YAML route should not be indexed")
	}

	diagnostics := collectRecentRenderDiagnostics(10)
	if len(diagnostics) != 1 {
		t.Fatalf("recent diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Route != "__config" {
		t.Fatalf("diagnostics route = %q, want __config", diagnostics[0].Route)
	}
	if len(diagnostics[0].Errors) != 1 {
		t.Fatalf("diagnostics errors len = %d, want 1: %#v", len(diagnostics[0].Errors), diagnostics[0].Errors)
	}
	diagnostic := diagnostics[0].Errors[0]
	if diagnostic.Type != "YAML" || diagnostic.File != "broken" || !strings.Contains(diagnostic.Err, "YAML source broken.hyperbricks.yaml was skipped") {
		t.Fatalf("config diagnostic = %#v", diagnostic)
	}
	if strings.Contains(diagnostic.Err, repoRoot) {
		t.Fatalf("config diagnostic should not expose absolute repo path: %#v", diagnostic.Err)
	}
	if !strings.Contains(diagnostic.Err, "modules/yaml-invalid-source/hyperbricks/broken.hyperbricks.yaml") {
		t.Fatalf("config diagnostic should include relative YAML path: %#v", diagnostic.Err)
	}

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/broken-route", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusInternalServerError {
		t.Fatalf("missing broken route status = %d, want %d", writer.Code, http.StatusInternalServerError)
	}
	body := writer.Body.String()
	if !strings.Contains(body, "YAML source broken.hyperbricks.yaml was skipped") {
		t.Fatalf("missing route body should include YAML load error, got %q", body)
	}
	if strings.Contains(body, repoRoot) {
		t.Fatalf("missing route body should not expose absolute repo path: %q", body)
	}
	if !strings.Contains(body, "modules/yaml-invalid-source/hyperbricks/broken.hyperbricks.yaml") {
		t.Fatalf("missing route body should include relative YAML path: %q", body)
	}
	if got := writer.Header().Get(renderErrorCountHeader); got != "1" {
		t.Fatalf("missing route error count = %q, want 1", got)
	}
	requestID := writer.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatal("missing route response should include request id")
	}
	renderDiagnosticsMutex.RLock()
	routeDiagnostics, ok := renderDiagnostics[requestID]
	renderDiagnosticsMutex.RUnlock()
	if !ok {
		t.Fatalf("expected diagnostics for missing route request %q", requestID)
	}
	if routeDiagnostics.Route != "broken-route" {
		t.Fatalf("missing route diagnostics route = %q, want broken-route", routeDiagnostics.Route)
	}
	if len(routeDiagnostics.Errors) != 1 || routeDiagnostics.Errors[0].File != "broken" {
		t.Fatalf("missing route diagnostics errors = %#v", routeDiagnostics.Errors)
	}
}

func TestYAMLUnknownChildTypeRendersSiblingsAndRecordsDiagnostic(t *testing.T) {
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldBeautify := hbConfig.Server.Beautify
	oldDirectories := hbConfig.Directories
	oldModuleRoot := commands.ModuleRoot
	oldModuleDirectories := core.ModuleDirectories
	oldConfigs := configs
	oldHypermediasBySection := hypermediasBySection
	oldRouteSourceErrors := routeSourceErrors
	oldRM := rm
	oldParserHbConfig := parser.HbConfig
	oldRenderDiagnosticsSeq := renderDiagnosticsSeq

	renderDiagnosticsMutex.Lock()
	oldRenderDiagnostics := renderDiagnostics
	oldRenderDiagnosticsOrder := renderDiagnosticsOrder
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	renderDiagnosticsMutex.Unlock()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Server.Beautify = oldBeautify
		hbConfig.Directories = oldDirectories
		commands.ModuleRoot = oldModuleRoot
		core.ModuleDirectories = oldModuleDirectories
		rm = oldRM
		parser.HbConfig = oldParserHbConfig

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermediasBySection
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldRouteSourceErrors
		routeSourceErrorsMutex.Unlock()

		renderDiagnosticsSeq = oldRenderDiagnosticsSeq
		renderDiagnosticsMutex.Lock()
		renderDiagnostics = oldRenderDiagnostics
		renderDiagnosticsOrder = oldRenderDiagnosticsOrder
		renderDiagnosticsMutex.Unlock()
	})

	moduleDir := filepath.Join(t.TempDir(), "modules", "yaml-unknown-child")
	hyperbricksDir := filepath.Join(moduleDir, "hyperbricks")
	for _, dir := range []string{
		hyperbricksDir,
		filepath.Join(moduleDir, "templates"),
		filepath.Join(moduleDir, "resources"),
		filepath.Join(moduleDir, "static"),
		filepath.Join(moduleDir, "rendered"),
		filepath.Join(moduleDir, "plugins"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create test module dir %s: %v", dir, err)
		}
	}

	yamlSource := `page:
  - type: hypermedia
  - route: unknown-child
  - title: Unknown child
  - main:
      - type: tree
      - before:
          - type: html
          - value: <p>before unknown child</p>
      - inline_css:
          - type: XXX
          - inline: |
              body { color: red; }
      - after:
          - type: html
          - value: <p>after unknown child</p>
`
	if err := os.WriteFile(filepath.Join(hyperbricksDir, "page.hyperbricks.yaml"), []byte(yamlSource), 0644); err != nil {
		t.Fatalf("write YAML route fixture: %v", err)
	}

	commands.ModuleRoot = moduleDir
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Server.Beautify = false
	hbConfig.Directories = map[string]string{
		"hyperbricks": hyperbricksDir,
		"templates":   filepath.Join(moduleDir, "templates"),
		"resources":   filepath.Join(moduleDir, "resources"),
		"static":      filepath.Join(moduleDir, "static"),
		"render":      filepath.Join(moduleDir, "rendered"),
		"plugins":     filepath.Join(moduleDir, "plugins"),
	}

	configs = make(map[string]map[string]interface{})
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)
	parser.HbConfig = map[string]interface{}{}

	initializeComponents()
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatalf("PreProcessAndPopulateConfigs returned error: %v", err)
	}
	routeConfig, ok := getConfig("unknown-child")
	if !ok {
		t.Fatalf("YAML route was not indexed; configs: %#v", configs)
	}
	main := routeConfig["main"].(map[string]interface{})
	inlineCSS := main["inline_css"].(map[string]interface{})
	if inlineCSS["@type"] != "<XXX>" {
		t.Fatalf("inline_css @type = %#v, want <XXX>", inlineCSS["@type"])
	}

	request := httptest.NewRequest(http.MethodGet, "/unknown-child", nil)
	response := httptest.NewRecorder()
	ServeContent(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body:\n%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "before unknown child") || !strings.Contains(body, "after unknown child") {
		t.Fatalf("valid siblings were not rendered around unknown child:\n%s", body)
	}
	if got := response.Header().Get(renderErrorCountHeader); got != "1" {
		t.Fatalf("render error count = %q, want 1; body:\n%s", got, body)
	}

	requestID := response.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatal("missing render request id header")
	}
	renderDiagnosticsMutex.RLock()
	diagnostics, ok := renderDiagnostics[requestID]
	renderDiagnosticsMutex.RUnlock()
	if !ok {
		t.Fatalf("expected diagnostics for request %q", requestID)
	}
	if len(diagnostics.Errors) != 1 {
		t.Fatalf("render diagnostics len = %d, want 1: %#v", len(diagnostics.Errors), diagnostics.Errors)
	}
	diagnostic := diagnostics.Errors[0]
	if diagnostic.Type != "<XXX>" || diagnostic.File != "page" || diagnostic.Path != "page.main.inline_css" || diagnostic.Key != "inline_css" {
		t.Fatalf("render diagnostic metadata = %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.Err, "type <XXX> not registered") {
		t.Fatalf("render diagnostic error = %#v", diagnostic.Err)
	}
}

func TestPreProcessAndPopulateHyperbricksConfigurationsRecordsTopLevelErrors(t *testing.T) {
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldDirectories := hbConfig.Directories
	oldModuleRoot := commands.ModuleRoot
	oldModuleDirectories := core.ModuleDirectories
	oldConfigs := configs
	oldHypermediasBySection := hypermediasBySection
	oldRouteSourceErrors := routeSourceErrors
	oldRenderDiagnosticsSeq := renderDiagnosticsSeq

	renderDiagnosticsMutex.Lock()
	oldRenderDiagnostics := renderDiagnostics
	oldRenderDiagnosticsOrder := renderDiagnosticsOrder
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	renderDiagnosticsMutex.Unlock()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Directories = oldDirectories
		commands.ModuleRoot = oldModuleRoot
		core.ModuleDirectories = oldModuleDirectories

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermediasBySection
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldRouteSourceErrors
		routeSourceErrorsMutex.Unlock()

		renderDiagnosticsSeq = oldRenderDiagnosticsSeq
		renderDiagnosticsMutex.Lock()
		renderDiagnostics = oldRenderDiagnostics
		renderDiagnosticsOrder = oldRenderDiagnosticsOrder
		renderDiagnosticsMutex.Unlock()
	})

	moduleDir := filepath.Join(t.TempDir(), "modules", "yaml-empty-module")
	hyperbricksDir := filepath.Join(moduleDir, "hyperbricks")
	for _, dir := range []string{
		hyperbricksDir,
		filepath.Join(moduleDir, "templates"),
		filepath.Join(moduleDir, "resources"),
		filepath.Join(moduleDir, "static"),
		filepath.Join(moduleDir, "rendered"),
		filepath.Join(moduleDir, "plugins"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create test module dir %s: %v", dir, err)
		}
	}

	commands.ModuleRoot = moduleDir
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Directories = map[string]string{
		"hyperbricks": hyperbricksDir,
		"templates":   filepath.Join(moduleDir, "templates"),
		"resources":   filepath.Join(moduleDir, "resources"),
		"static":      filepath.Join(moduleDir, "static"),
		"render":      filepath.Join(moduleDir, "rendered"),
		"plugins":     filepath.Join(moduleDir, "plugins"),
	}
	configs = make(map[string]map[string]interface{})
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)

	PreProcessAndPopulateHyperbricksConfigurations()

	diagnostics := collectRecentRenderDiagnostics(10)
	if len(diagnostics) != 1 {
		t.Fatalf("recent diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Route != "__config" || len(diagnostic.Errors) != 1 {
		t.Fatalf("config diagnostics = %#v", diagnostic)
	}
	if diagnostic.Errors[0].Type != "CONFIG" || !strings.Contains(diagnostic.Errors[0].Err, "no .hyperbricks.yaml files found") {
		t.Fatalf("config diagnostic error = %#v", diagnostic.Errors[0])
	}
}

func TestPreProcessAndPopulateConfigsLoadsYAMLRouteThroughServerRenderFlow(t *testing.T) {
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldBeautify := hbConfig.Server.Beautify
	oldDirectories := hbConfig.Directories
	oldModuleRoot := commands.ModuleRoot
	oldModuleDirectories := core.ModuleDirectories
	oldConfigs := configs
	oldHypermediasBySection := hypermediasBySection
	oldRouteSourceErrors := routeSourceErrors
	oldRM := rm
	oldRenderDiagnosticsSeq := renderDiagnosticsSeq

	renderDiagnosticsMutex.Lock()
	oldRenderDiagnostics := renderDiagnostics
	oldRenderDiagnosticsOrder := renderDiagnosticsOrder
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	renderDiagnosticsMutex.Unlock()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Server.Beautify = oldBeautify
		hbConfig.Directories = oldDirectories
		commands.ModuleRoot = oldModuleRoot
		core.ModuleDirectories = oldModuleDirectories
		rm = oldRM

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermediasBySection
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldRouteSourceErrors
		routeSourceErrorsMutex.Unlock()

		renderDiagnosticsSeq = oldRenderDiagnosticsSeq
		renderDiagnosticsMutex.Lock()
		renderDiagnostics = oldRenderDiagnostics
		renderDiagnosticsOrder = oldRenderDiagnosticsOrder
		renderDiagnosticsMutex.Unlock()
	})

	moduleDir := filepath.Join(t.TempDir(), "modules", "yaml-runtime")
	hyperbricksDir := filepath.Join(moduleDir, "hyperbricks")
	for _, dir := range []string{
		hyperbricksDir,
		filepath.Join(moduleDir, "templates"),
		filepath.Join(moduleDir, "resources"),
		filepath.Join(moduleDir, "static"),
		filepath.Join(moduleDir, "rendered"),
		filepath.Join(moduleDir, "plugins"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create test module dir %s: %v", dir, err)
		}
	}

	yamlSource := `page:
  - type: hypermedia
  - route: yaml-runtime
  - title: YAML Runtime
  - main:
      - type: tree
      - intro:
          - type: html
          - value: <main><h1>YAML runtime route</h1></main>
      - intro:
          - type: html
          - value: <p>Recovered duplicate child</p>
`
	if err := os.WriteFile(filepath.Join(hyperbricksDir, "page.hyperbricks.yaml"), []byte(yamlSource), 0644); err != nil {
		t.Fatalf("write YAML route fixture: %v", err)
	}

	commands.ModuleRoot = moduleDir
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Server.Beautify = false
	hbConfig.Directories = map[string]string{
		"hyperbricks": hyperbricksDir,
		"templates":   filepath.Join(moduleDir, "templates"),
		"resources":   filepath.Join(moduleDir, "resources"),
		"static":      filepath.Join(moduleDir, "static"),
		"render":      filepath.Join(moduleDir, "rendered"),
		"plugins":     filepath.Join(moduleDir, "plugins"),
	}

	configs = make(map[string]map[string]interface{})
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)

	initializeComponents()
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatalf("PreProcessAndPopulateConfigs returned error: %v", err)
	}

	routeConfig, ok := getConfig("yaml-runtime")
	if !ok {
		t.Fatalf("YAML route was not indexed; configs: %#v", configs)
	}
	if routeConfig["@type"] != composite.HyperMediaConfigGetName() {
		t.Fatalf("route @type = %#v, want %s", routeConfig["@type"], composite.HyperMediaConfigGetName())
	}
	if order, ok := routeConfig["@order"].([]string); !ok || len(order) != 1 || order[0] != "main" {
		t.Fatalf("route @order = %#v, want [main]", routeConfig["@order"])
	}

	request := httptest.NewRequest(http.MethodGet, "/yaml-runtime", nil)
	response := httptest.NewRecorder()
	ServeContent(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body:\n%s", response.Code, response.Body.String())
	}
	t.Logf("rendered YAML route output:\n%s", response.Body.String())
	if body := response.Body.String(); !strings.Contains(body, "<h1>YAML runtime route</h1>") {
		t.Fatalf("rendered body does not contain YAML route content:\n%s", body)
	}
	if body := response.Body.String(); !strings.Contains(body, "Recovered duplicate child") {
		t.Fatalf("rendered body does not contain recovered duplicate child:\n%s", body)
	}

	if got := response.Header().Get(renderErrorCountHeader); got != "1" {
		t.Fatalf("render error count = %q, want 1", got)
	}
	requestID := response.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatal("missing render request id header")
	}
	renderDiagnosticsMutex.RLock()
	diagnostics, ok := renderDiagnostics[requestID]
	renderDiagnosticsMutex.RUnlock()
	if !ok {
		t.Fatalf("expected diagnostics for request %q", requestID)
	}
	if len(diagnostics.Errors) != 1 {
		t.Fatalf("render diagnostics len = %d, want 1: %#v", len(diagnostics.Errors), diagnostics.Errors)
	}
	diagnostic := diagnostics.Errors[0]
	if diagnostic.Type != "YAML" || diagnostic.Path != "page.main" || diagnostic.Key != "intro" || !strings.Contains(diagnostic.Err, `using "intro_2" as runtime path`) {
		t.Fatalf("render diagnostic = %#v", diagnostic)
	}
	if diagnostic.File != "page" {
		t.Fatalf("render diagnostic file = %#v", diagnostic.File)
	}
}

func TestPreProcessAndPopulateConfigsSupportsYAMLRuntimePreprocessing(t *testing.T) {
	shared.Init_configuration()
	parser.ClearTemplateStore()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldBeautify := hbConfig.Server.Beautify
	oldDirectories := hbConfig.Directories
	oldModuleRoot := commands.ModuleRoot
	oldModuleDirectories := core.ModuleDirectories
	oldConfigs := configs
	oldHypermediasBySection := hypermediasBySection
	oldRouteSourceErrors := routeSourceErrors
	oldRM := rm
	oldParserHbConfig := parser.HbConfig

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Server.Beautify = oldBeautify
		hbConfig.Directories = oldDirectories
		commands.ModuleRoot = oldModuleRoot
		core.ModuleDirectories = oldModuleDirectories
		rm = oldRM
		parser.HbConfig = oldParserHbConfig
		parser.ClearTemplateStore()

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermediasBySection
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldRouteSourceErrors
		routeSourceErrorsMutex.Unlock()
	})

	moduleDir := filepath.Join(t.TempDir(), "modules", "yaml-runtime-pipeline")
	hyperbricksDir := filepath.Join(moduleDir, "hyperbricks")
	templatesDir := filepath.Join(moduleDir, "templates")
	resourcesDir := filepath.Join(moduleDir, "resources")
	staticDir := filepath.Join(moduleDir, "static")
	renderDir := filepath.Join(moduleDir, "rendered")
	pluginsDir := filepath.Join(moduleDir, "plugins")
	for _, dir := range []string{
		filepath.Join(hyperbricksDir, "partials"),
		filepath.Join(templatesDir, "cards"),
		resourcesDir,
		staticDir,
		renderDir,
		pluginsDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create test module dir %s: %v", dir, err)
		}
	}

	t.Setenv("HB_TEST_TITLE", "Runtime Pipeline")
	parser.HbConfig = map[string]interface{}{
		"site": map[string]interface{}{
			"heading": "Configured heading",
		},
	}

	importedSource := `shared_hero:
  - type: html
  - value: <p>fallback from import</p>
`
	if err := os.WriteFile(filepath.Join(hyperbricksDir, "partials", "shared.hyperbricks.yaml"), []byte(importedSource), 0644); err != nil {
		t.Fatalf("write imported YAML fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourcesDir, "runtime-body.html"), []byte("<section class=\"from-resource\"><p>Runtime file body</p></section>\n"), 0644); err != nil {
		t.Fatalf("write resource fixture: %v", err)
	}
	templateSource := `<section class="runtime-card">
  <h2>{{.heading}}</h2>
  <p data-var="{{.module_var}}">module marker: {{.module_marker}}</p>
  <p>templates: {{.templates_marker}}</p>
  <p>static: {{.static_marker}}</p>
  <p>hyperbricks: {{.hyperbricks_marker}}</p>
</section>`
	if err := os.WriteFile(filepath.Join(templatesDir, "cards", "runtime-card.html"), []byte(templateSource), 0644); err != nil {
		t.Fatalf("write template fixture: %v", err)
	}

	yamlSource := `imports:
  - partials/shared.hyperbricks.yaml

page:
  - type: hypermedia
  - route: runtime-pipeline
  - title:
      env: HB_TEST_TITLE
  - main:
      - type: tree
      - hero:
          - inherit: shared_hero
          - value:
              file:
                base: resources
                path: runtime-body.html
      - card:
          - type: template
          - template:
              file: cards/runtime-card.html
          - values:
              heading:
                config: site.heading
              module_var:
                var: module
              module_marker:
                path:
                  base: module
              templates_marker:
                path:
                  base: templates
              static_marker:
                path:
                  base: static
              hyperbricks_marker:
                path:
                  base: hyperbricks
`
	if err := os.WriteFile(filepath.Join(hyperbricksDir, "page.hyperbricks.yaml"), []byte(yamlSource), 0644); err != nil {
		t.Fatalf("write YAML route fixture: %v", err)
	}

	commands.ModuleRoot = moduleDir
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Server.Beautify = false
	hbConfig.Directories = map[string]string{
		"hyperbricks": hyperbricksDir,
		"templates":   templatesDir,
		"resources":   resourcesDir,
		"static":      staticDir,
		"render":      renderDir,
		"plugins":     pluginsDir,
	}

	configs = make(map[string]map[string]interface{})
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)

	initializeComponents()
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatalf("PreProcessAndPopulateConfigs returned error: %v", err)
	}

	routeConfig, ok := getConfig("runtime-pipeline")
	if !ok {
		t.Fatalf("YAML route was not indexed; configs: %#v", configs)
	}
	if routeConfig["title"] != "Runtime Pipeline" {
		t.Fatalf("route title = %#v, want Runtime Pipeline", routeConfig["title"])
	}

	request := httptest.NewRequest(http.MethodGet, "/runtime-pipeline", nil)
	response := httptest.NewRecorder()
	ServeContent(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body:\n%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	t.Logf("rendered YAML preprocessing route output:\n%s", body)
	for _, want := range []string{
		"<title>Runtime Pipeline</title>",
		"<section class=\"from-resource\"><p>Runtime file body</p></section>",
		"<h2>Configured heading</h2>",
		`data-var="` + moduleDir + `"`,
		"module marker: " + moduleDir,
		"templates: " + templatesDir,
		"static: " + staticDir,
		"hyperbricks: " + hyperbricksDir,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered body missing %q:\n%s", want, body)
		}
	}
}

func TestPreProcessAndPopulateConfigsLoadsConvertedPatternsYAMLModule(t *testing.T) {
	shared.Init_configuration()
	parser.ClearTemplateStore()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldBeautify := hbConfig.Server.Beautify
	oldDirectories := hbConfig.Directories
	oldFrontendErrors := hbConfig.Development.FrontendErrors
	oldModuleRoot := commands.ModuleRoot
	oldModuleDirectories := core.ModuleDirectories
	oldConfigs := configs
	oldHypermediasBySection := hypermediasBySection
	oldRouteSourceErrors := routeSourceErrors
	oldRM := rm
	oldParserHbConfig := parser.HbConfig
	oldRenderDiagnosticsSeq := renderDiagnosticsSeq

	htmlCacheMutex.Lock()
	oldHTMLCache := htmlCache
	htmlCache = make(map[string]CacheEntry)
	htmlCacheMutex.Unlock()

	renderDiagnosticsMutex.Lock()
	oldRenderDiagnostics := renderDiagnostics
	oldRenderDiagnosticsOrder := renderDiagnosticsOrder
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	renderDiagnosticsMutex.Unlock()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Server.Beautify = oldBeautify
		hbConfig.Directories = oldDirectories
		hbConfig.Development.FrontendErrors = oldFrontendErrors
		commands.ModuleRoot = oldModuleRoot
		core.ModuleDirectories = oldModuleDirectories
		rm = oldRM
		parser.HbConfig = oldParserHbConfig
		parser.ClearTemplateStore()
		renderDiagnosticsSeq = oldRenderDiagnosticsSeq

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermediasBySection
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldRouteSourceErrors
		routeSourceErrorsMutex.Unlock()

		htmlCacheMutex.Lock()
		htmlCache = oldHTMLCache
		htmlCacheMutex.Unlock()

		renderDiagnosticsMutex.Lock()
		renderDiagnostics = oldRenderDiagnostics
		renderDiagnosticsOrder = oldRenderDiagnosticsOrder
		renderDiagnosticsMutex.Unlock()
	})

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	moduleDir := filepath.Join(repoRoot, "modules", "hyperbricks-patterns-yaml")
	if _, err := os.Stat(filepath.Join(moduleDir, "hyperbricks")); err != nil {
		t.Fatalf("converted patterns YAML module is missing: %v", err)
	}
	skipIfPluginToolchainMismatch(t, filepath.Join(repoRoot, "bin", "plugins", "MarkdownPlugin@2.0.0.so"))

	commands.ModuleRoot = moduleDir
	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Server.Beautify = false
	hbConfig.Development.FrontendErrors = false
	hbConfig.Directories = map[string]string{
		"hyperbricks": filepath.Join(moduleDir, "hyperbricks"),
		"templates":   filepath.Join(moduleDir, "templates"),
		"resources":   filepath.Join(moduleDir, "resources"),
		"static":      filepath.Join(moduleDir, "static"),
		"render":      filepath.Join(moduleDir, "rendered"),
		"plugins":     filepath.Join(repoRoot, "bin", "plugins"),
	}

	configs = make(map[string]map[string]interface{})
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)
	parser.HbConfig = map[string]interface{}{}
	renderDiagnosticsSeq = 0

	initializeComponents()
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatalf("PreProcessAndPopulateConfigs returned error: %v", err)
	}

	expectedRoutes := []string{
		"api-fragment-write-demo",
		"docs",
		"docs/api-fragment-write-demo",
		"docs/config-driven-section-rail",
		"docs/guarded-page-demo",
		"docs/htmx-canonical-fragment-demo",
		"docs/markdown-plugin",
		"docs/menu-htmx-demo",
		"docs/plugin-vs-api-route-split",
		"docs/readme",
		"docs/single-plugin-many-actions",
		"docs/template-config-plugin",
		"docs/unpoly-fragment-demo",
		"fragments/api-fragment-write-refresh-probe",
		"fragments/mock-postgrest-file-save-conflict",
		"fragments/mock-postgrest-file-save-success",
		"fragments/mock-postgrest-rename-conflict",
		"fragments/mock-postgrest-rename-success",
		"fragments/rail-assets",
		"fragments/rail-builder",
		"fragments/rail-status",
		"fragments/route-split-refresh-probe",
		"fragments/status-demo",
		"fragments/status-demo-plugin",
		"fragments/status-demo-settings",
		"fragments/status-demo-summary",
		"fragments/unpoly-demo",
		"guarded-demo",
		"guarded-demo/auth/authorize",
		"guarded-demo/auth/login",
		"guarded-demo/auth/logout",
		"guarded-demo/forbidden",
		"guarded-demo/login",
		"guarded-demo/secret",
		"index",
		"menu-demo",
		"menu-demo/doc-1",
		"menu-demo/doc-2",
		"menu-demo/doc-3",
		"patterns/assets/file-save-conflict",
		"patterns/assets/file-save-success",
		"patterns/route-split/import",
		"patterns/route-split/rename-conflict",
		"patterns/route-split/rename-success",
		"plugin-vs-api-route-split",
		"rail-assets",
		"rail-builder",
		"rail-status",
		"section-rail-demo",
		"single-plugin-actions-demo",
		"status-demo",
		"status-demo/plugin",
		"status-demo/settings",
		"status-demo/summary",
		"unpoly-demo",
		"unpoly-demo/loaded",
		"workflow-actions-demo/complete",
		"workflow-actions-demo/landing",
		"workflow-actions-demo/lookup",
		"workflow-actions-demo/signup",
	}
	if len(configs) != len(expectedRoutes) {
		t.Fatalf("route count = %d, want %d; routes: %#v", len(configs), len(expectedRoutes), configs)
	}
	for _, route := range expectedRoutes {
		if _, ok := getConfig(route); !ok {
			t.Fatalf("converted YAML module route %q was not indexed", route)
		}
	}

	menuRequest := httptest.NewRequest(http.MethodGet, "/menu-demo", nil)
	menuResponse := httptest.NewRecorder()
	ServeContent(menuResponse, menuRequest)
	if menuResponse.Code != http.StatusOK {
		t.Fatalf("menu-demo status = %d, body:\n%s", menuResponse.Code, menuResponse.Body.String())
	}
	menuBody := menuResponse.Body.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"HTMX Enabled MENU",
		"Rendered by &lt;MENU&gt;",
		"Landing page",
		`id="menu-demo-panel"`,
	} {
		if !strings.Contains(menuBody, want) {
			t.Fatalf("menu-demo output missing %q:\n%s", want, menuBody)
		}
	}

	fragmentRequest := httptest.NewRequest(http.MethodGet, "/fragments/status-demo-summary", nil)
	fragmentResponse := httptest.NewRecorder()
	ServeContent(fragmentResponse, fragmentRequest)
	if fragmentResponse.Code != http.StatusOK {
		t.Fatalf("summary fragment status = %d, body:\n%s", fragmentResponse.Code, fragmentResponse.Body.String())
	}
	fragmentBody := fragmentResponse.Body.String()
	for _, want := range []string{
		"Summary fragment",
		"/status-demo/summary",
		"/fragments/status-demo-summary",
	} {
		if !strings.Contains(fragmentBody, want) {
			t.Fatalf("summary fragment output missing %q:\n%s", want, fragmentBody)
		}
	}
	if got := fragmentResponse.Header().Get("X-Hyperbricks-Render-Error-Count"); got != "0" {
		t.Fatalf("summary fragment render error count = %q, want 0; body:\n%s", got, fragmentBody)
	}

	unpolyRequest := httptest.NewRequest(http.MethodGet, "/fragments/unpoly-demo", nil)
	unpolyResponse := httptest.NewRecorder()
	ServeContent(unpolyResponse, unpolyRequest)
	if unpolyResponse.Code != http.StatusOK {
		t.Fatalf("Unpoly fragment status = %d, body:\n%s", unpolyResponse.Code, unpolyResponse.Body.String())
	}
	if got := unpolyResponse.Header().Get("X-Demo-Frontend"); got != "unpoly" {
		t.Fatalf("Unpoly fragment X-Demo-Frontend = %q, want unpoly", got)
	}
	unpolyBody := unpolyResponse.Body.String()
	if !strings.Contains(unpolyBody, `id="unpoly-panel"`) || !strings.Contains(unpolyBody, "Fragment loaded") {
		t.Fatalf("Unpoly fragment is missing its matching target or loaded content:\n%s", unpolyBody)
	}
	if strings.Contains(strings.ToLower(unpolyBody), "<!doctype") || strings.Contains(strings.ToLower(unpolyBody), "<html") {
		t.Fatalf("Unpoly fragment contains a full-page wrapper:\n%s", unpolyBody)
	}
	if got := unpolyResponse.Header().Get("X-Hyperbricks-Render-Error-Count"); got != "0" {
		t.Fatalf("Unpoly fragment render error count = %q, want 0; body:\n%s", got, unpolyBody)
	}
}

func skipIfPluginToolchainMismatch(t *testing.T, pluginPath string) {
	t.Helper()
	if _, err := os.Stat(pluginPath); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("plugin %s is missing; run scripts/run_all_tests.sh --with-plugins to rebuild plugin-backed render test fixtures", pluginPath)
		}
		t.Fatalf("stat plugin %s: %v", pluginPath, err)
	}
	info, err := buildinfo.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read plugin build info %s: %v", pluginPath, err)
	}
	if info.GoVersion != runtime.Version() {
		t.Skipf("plugin %s was built with %s, but test host is %s; rebuild plugins with the same Go toolchain before running plugin-backed render tests", pluginPath, info.GoVersion, runtime.Version())
	}
}
