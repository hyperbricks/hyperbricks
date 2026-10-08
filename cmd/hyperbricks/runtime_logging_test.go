package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"go.uber.org/zap/zapcore"
)

func setupModuleLogTest(t *testing.T) string {
	t.Helper()
	setupDevelopmentModeServeContentTest(t, false)
	t.Chdir(t.TempDir())
	root, err := filepath.Abs("modules/demo")
	if err != nil {
		t.Fatal(err)
	}
	oldRoot, oldConfig, oldStatic := commands.ModuleRoot, commands.ModuleConfigPath, commands.RenderStatic
	oldDirectories, oldLocation := core.ModuleDirectories, shared.Location
	oldLevel := logging.GetLogger().Level()
	commands.ModuleRoot = root
	commands.ModuleConfigPath = filepath.Join(root, "profiles/development.hyperbricks.yaml")
	commands.RenderStatic = false
	core.ModuleDirectories.ModuleDir = root
	core.ModuleDirectories.HyperbricksDir = filepath.Join(root, "content")
	shared.Location = "127.0.0.1:8080"
	logging.ChangeLevel(zapcore.InfoLevel)
	t.Cleanup(func() {
		commands.ModuleRoot, commands.ModuleConfigPath, commands.RenderStatic = oldRoot, oldConfig, oldStatic
		core.ModuleDirectories = oldDirectories
		shared.Location = oldLocation
		logging.ChangeLevel(oldLevel)
	})
	return root
}

func TestRuntimeSummaryAndRoutesStayCompactAtInfo(t *testing.T) {
	root := setupModuleLogTest(t)
	config := &shared.Config{Mode: shared.DEVELOPMENT_MODE, Logger: shared.LoggerConfig{Path: filepath.Join(root, "logs/runtime.jsonl")},
		Development: shared.DevelopmentConfig{Watch: true, Reload: true, Dashboard: shared.DevelopmentDashboardConfig{Enabled: true, Credentials: developerTestCredentials}, WatchDirs: []string{"hyperbricks", "templates"}},
		Directories: map[string]string{"hyperbricks": filepath.Join(root, "content"), "templates": filepath.Join(root, "views")}}
	config.Development.FrontendEditing.Enabled = true
	config.Development.FrontendEditing.Spaces.Enabled = true
	config.Development.FrontendEditing.Spaces.Route = "/__hyperbricks/spaces"
	started := time.Now()
	logRuntimeSummary(config)
	printFilenameToRoutesMapping(map[string][]string{"page": {"index", "status"}}, map[string]map[string]interface{}{
		"index":  {"@type": "<HYPERMEDIA>", "hyperbricksfile": "content/page.hyperbricks.yaml"},
		"status": {"@type": "<FRAGMENT>", "hyperbricksfile": "content/parts/status.hyperbricks.yaml"},
	})
	logging.GetLogger().Debug("internal trace should stay hidden")
	events := map[string]logging.LogMessage{}
	for _, event := range logging.GetLogs() {
		if event.Time.Before(started) {
			continue
		}
		encoded, _ := json.Marshal(event)
		if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "modules/demo") || event.Level != zapcore.InfoLevel {
			t.Fatalf("unexpected context/level: %s", encoded)
		}
		events[event.Message] = event
	}
	for _, message := range []string{"Module configured  module=demo mode=development", "Developer tools  dashboard=/__hyperbricks/dashboard spaces=/__hyperbricks/spaces write=false", "Watching directories  content, views", "Routes registered  count=2"} {
		if _, ok := events[message]; !ok {
			t.Fatalf("missing compact event %q: %#v", message, events)
		}
	}
	if events["Route available"].Fields["route"] != "/status" || events["Route available"].Fields["url"] != "http://127.0.0.1:8080/status" {
		t.Fatalf("missing usable route URL: %#v", events["Route available"])
	}
	if _, ok := events["internal trace should stay hidden"]; ok {
		t.Fatal("DEBUG visible at INFO")
	}
}

func TestRuntimeSummarySpacesAccessWarnings(t *testing.T) {
	setupModuleLogTest(t)
	oldRuntime := shared.GetRuntimeOptions()
	t.Cleanup(func() { shared.SetRuntimeOptions(oldRuntime) })
	for _, scenario := range []struct {
		name, mode                   string
		credentials                  shared.CredentialsConfig
		disableParent, disableSpaces bool
		readOnly, externalEditor     bool
		production, static           bool
		wantTools                    bool
		warnings                     []string
	}{
		{name: "open development", mode: shared.DEVELOPMENT_MODE, wantTools: true, warnings: []string{"Spaces and contextual editing are accessible without login through allowed hosts; write=true"}},
		{name: "open debug", mode: shared.DEBUG_MODE, wantTools: true, warnings: []string{"Spaces and contextual editing are accessible without login through allowed hosts; write=true"}},
		{name: "read only", mode: shared.DEVELOPMENT_MODE, readOnly: true, wantTools: true, warnings: []string{"Spaces and contextual editing are accessible without login through allowed hosts; write=false"}},
		{name: "Spaces disabled", mode: shared.DEVELOPMENT_MODE, disableSpaces: true},
		{name: "parent disabled", mode: shared.DEVELOPMENT_MODE, disableParent: true, externalEditor: true},
		{name: "configured", mode: shared.DEVELOPMENT_MODE, credentials: developerTestCredentials, wantTools: true},
		{name: "user only", mode: shared.DEVELOPMENT_MODE, credentials: shared.CredentialsConfig{User: "developer"}, wantTools: true, warnings: []string{"Developer interface locked"}},
		{name: "password only", mode: shared.DEBUG_MODE, credentials: shared.CredentialsConfig{Password: "secret"}, wantTools: true, warnings: []string{"Developer interface locked"}},
		{name: "live", mode: shared.LIVE_MODE, externalEditor: true},
		{name: "production", mode: shared.DEVELOPMENT_MODE, production: true, externalEditor: true},
		{name: "static", mode: shared.DEVELOPMENT_MODE, static: true, externalEditor: true},
		{name: "external plugin locked", mode: shared.DEVELOPMENT_MODE, disableSpaces: true, externalEditor: true, warnings: []string{"Frontend editor plugins locked"}},
		{name: "external plugin unavailable in debug", mode: shared.DEBUG_MODE, disableSpaces: true, externalEditor: true},
		{name: "Spaces open and external plugin locked", mode: shared.DEVELOPMENT_MODE, externalEditor: true, wantTools: true, warnings: []string{"Spaces and contextual editing are accessible without login through allowed hosts; write=true", "Frontend editor plugins locked"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			config := &shared.Config{Mode: scenario.mode}
			config.Development.Dashboard.Credentials = scenario.credentials
			config.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
			config.Development.FrontendEditing.Enabled = !scenario.disableParent
			config.Development.FrontendEditing.Spaces.Enabled = !scenario.disableSpaces
			config.Development.FrontendEditing.Spaces.Write = !scenario.readOnly
			if scenario.externalEditor {
				config.Development.FrontendEditing.Editors = map[string]shared.FrontendEditorConfig{"other": {Plugin: "Other@1", Route: "/__hyperbricks/other"}}
			}
			runtime := oldRuntime
			runtime.Production, commands.RenderStatic = scenario.production, scenario.static
			shared.SetRuntimeOptions(runtime)
			started := time.Now()
			logRuntimeSummary(config)
			var warnings []string
			var toolsMessage string
			for _, event := range logging.GetLogs() {
				if event.Time.Before(started) {
					continue
				}
				if event.Level == zapcore.WarnLevel {
					warnings = append(warnings, event.Message)
				}
				if strings.HasPrefix(event.Message, "Developer tools  ") {
					toolsMessage = event.Message
				}
			}
			if len(warnings) != len(scenario.warnings) {
				t.Fatalf("warnings = %v, want %v", warnings, scenario.warnings)
			}
			for index, want := range scenario.warnings {
				if !strings.Contains(warnings[index], want) {
					t.Fatalf("warning = %q, want %q", warnings[index], want)
				}
			}
			if scenario.wantTools {
				want := "Developer tools  dashboard=disabled spaces=/__hyperbricks/spaces write=true"
				if scenario.readOnly {
					want = "Developer tools  dashboard=disabled spaces=/__hyperbricks/spaces write=false"
				}
				if toolsMessage != want {
					t.Fatalf("tools summary = %q, want %q", toolsMessage, want)
				}
			} else if toolsMessage != "" {
				t.Fatalf("disabled Spaces advertised in startup summary: %q", toolsMessage)
			}
		})
	}
}

func TestRuntimeRoutesGroupedBySourceAndSorted(t *testing.T) {
	root := setupModuleLogTest(t)
	logging.ChangeLevel(zapcore.DebugLevel)
	started := time.Now()
	printFilenameToRoutesMapping(map[string][]string{"landing": {"nl", "index", "de", "status"}}, map[string]map[string]interface{}{
		"index":  {"@type": "<HYPERMEDIA>", "hyperbricksfile": filepath.Join(root, "content/landing.hyperbricks.yaml"), "static": "index.html"},
		"de":     {"@type": "<HYPERMEDIA>", "hyperbricksfile": "content/landing.hyperbricks.yaml", "static": "de/index.html"},
		"nl":     {"@type": "<HYPERMEDIA>"},
		"status": {"@type": "<FRAGMENT>", "hyperbricksfile": "content/parts/status.hyperbricks.yaml"},
	})
	var events []logging.LogMessage
	for _, event := range logging.GetLogs() {
		if !event.Time.Before(started) && event.Message == "Route registered" {
			events = append(events, event)
		}
	}
	if len(events) != 4 {
		t.Fatalf("source grouping: %#v", events)
	}
	for i, route := range []string{"/", "/de", "/nl", "/status"} {
		if events[i].Fields["route"] != route {
			t.Fatalf("route order=%#v", events)
		}
	}
	if events[0].Fields["static"] != "index.html" || events[1].Fields["static"] != "de/index.html" || events[3].Fields["source"] != "status.hyperbricks.yaml" {
		t.Fatalf("incomplete detail events: %#v", events)
	}
}

func TestRenderLogUsesModuleRelativeLocations(t *testing.T) {
	root := setupModuleLogTest(t)
	request := httptest.NewRequest("GET", "https://example.test/page?token=private", nil)
	logRenderDiagnostics(request, "hb-12345", "index", []error{shared.ComponentError{
		Type: "<TEMPLATE>", File: filepath.Join(root, "content/page.hyperbricks.yaml"),
		Path: "page.content", Line: 24, Column: 7, Resource: filepath.Join(root, "views/hero.html"), ResourceLine: 12,
		Err: "template: " + filepath.Join(root, "views/hero.html") + ":12:7: failed",
	}})
	entries := logging.GetLogs()
	entry := entries[len(entries)-1]
	for key, want := range map[string]string{"file": "content/page.hyperbricks.yaml", "resource": "views/hero.html", "path": "page.content", "route": "/", "diagnostics_url": "/__hyperbricks/render-diagnostics?request_id=hb-12345"} {
		if entry.Fields[key] != want {
			t.Fatalf("%s=%v want %s", key, entry.Fields[key], want)
		}
	}
	encoded, _ := json.Marshal(entry)
	if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "https://") {
		t.Fatalf("non-relative event: %s", encoded)
	}
}

func TestRuntimeSummaryDashboardCredentialWarnings(t *testing.T) {
	setupModuleLogTest(t)
	oldRuntime := shared.GetRuntimeOptions()
	t.Cleanup(func() { shared.SetRuntimeOptions(oldRuntime) })
	for _, scenario := range []struct {
		name, mode, warning string
		credentials         shared.CredentialsConfig
		production, static  bool
	}{
		{"open development", shared.DEVELOPMENT_MODE, "accessible without login to anyone who can reach this server", shared.CredentialsConfig{}, false, false},
		{"open debug", shared.DEBUG_MODE, "accessible without login to anyone who can reach this server", shared.CredentialsConfig{}, false, false},
		{"partial account", shared.DEVELOPMENT_MODE, "Developer interface locked", shared.CredentialsConfig{User: "developer"}, false, false},
		{"configured", shared.DEVELOPMENT_MODE, "", developerTestCredentials, false, false},
		{"live", shared.LIVE_MODE, "", shared.CredentialsConfig{}, false, false},
		{"production", shared.DEVELOPMENT_MODE, "", shared.CredentialsConfig{}, true, false},
		{"static", shared.DEVELOPMENT_MODE, "", shared.CredentialsConfig{}, false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			config := &shared.Config{Mode: scenario.mode}
			config.Development.Dashboard = shared.DevelopmentDashboardConfig{Enabled: true, Credentials: scenario.credentials}
			runtime := oldRuntime
			runtime.Production, commands.RenderStatic = scenario.production, scenario.static
			shared.SetRuntimeOptions(runtime)
			started := time.Now()
			logRuntimeSummary(config)
			var warnings []string
			for _, event := range logging.GetLogs() {
				if !event.Time.Before(started) && event.Level == zapcore.WarnLevel {
					warnings = append(warnings, event.Message)
				}
			}
			if scenario.warning == "" {
				if len(warnings) != 0 {
					t.Fatalf("unexpected warnings: %v", warnings)
				}
			} else if len(warnings) != 1 || !strings.Contains(warnings[0], scenario.warning) {
				t.Fatalf("warnings = %v, want one containing %q", warnings, scenario.warning)
			}
		})
	}
}
