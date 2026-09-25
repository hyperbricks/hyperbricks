package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const deployDashboardFixture = `hyperbricks:
  mode: live
  development:
    dashboard:
      enabled: true
      credentials:
        user: fixture-developer
        password: fixture-dashboard-secret
`

func TestDeployStatusDashboardFollowsRunningProcess(t *testing.T) {
	for _, scope := range []string{"local", "remote", "source-dev"} {
		for _, scenario := range []struct {
			name       string
			mode       string
			savedMode  string
			production bool
			running    bool
			config     string
			wantPath   bool
		}{
			{"development", shared.DEVELOPMENT_MODE, shared.DEVELOPMENT_MODE, false, true, deployDashboardFixture, true},
			{"live", shared.LIVE_MODE, shared.LIVE_MODE, false, true, deployDashboardFixture, false},
			{"production", shared.DEVELOPMENT_MODE, shared.LIVE_MODE, true, true, deployDashboardFixture, false},
			{"stopped", shared.DEVELOPMENT_MODE, shared.DEVELOPMENT_MODE, false, false, deployDashboardFixture, false},
			{"disabled", shared.DEVELOPMENT_MODE, shared.DEVELOPMENT_MODE, false, true, strings.Replace(deployDashboardFixture, "enabled: true", "enabled: false", 1), false},
			{"invalid config", shared.DEVELOPMENT_MODE, shared.DEVELOPMENT_MODE, false, true, "hyperbricks: {development: {dashboard: true}}", false},
			{"unknown process mode", "", shared.DEVELOPMENT_MODE, false, true, deployDashboardFixture, false},
			{"unrecognized process mode", "invalid", shared.DEVELOPMENT_MODE, false, true, deployDashboardFixture, false},
			{"live saved but development still running", shared.DEVELOPMENT_MODE, shared.LIVE_MODE, false, true, deployDashboardFixture, true},
			{"development saved but live still running", shared.LIVE_MODE, shared.DEVELOPMENT_MODE, false, true, deployDashboardFixture, false},
		} {
			t.Run(scope+"/"+scenario.name, func(t *testing.T) {
				root := t.TempDir()
				local := &deployLocalServer{buildRoot: filepath.Join(root, "build"), modulesDir: filepath.Join(root, "modules")}
				remote := &deployAPI{root: local.buildRoot}
				buildID := "build-1"
				moduleRoot := filepath.Join(local.buildRoot, "demo", "runtime", buildID)
				if scope == "source-dev" {
					buildID = localDevBuildID
					moduleRoot = filepath.Join(local.modulesDir, "demo")
				}
				if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(moduleRoot, shared.PackageConfigFileName), []byte(scenario.config), 0o600); err != nil {
					t.Fatal(err)
				}
				index := localBuildIndex{Current: buildID, Port: 7000, Versions: []localBuildRow{{BuildID: buildID, RuntimeMode: scenario.savedMode}}}
				if err := saveLocalBuildIndex(local.indexPath("demo"), index); err != nil {
					t.Fatal(err)
				}
				if scenario.running {
					// The read-only status handlers can safely observe this test process.
					proc := deployProcess{Module: "demo", PID: os.Getpid(), BuildID: buildID, Port: 7001, RuntimeMode: scenario.mode, Production: scenario.production}
					if err := local.writeProcess("demo", proc); err != nil {
						t.Fatal(err)
					}
				}
				for _, level := range []string{"module", "build"} {
					response := httptest.NewRecorder()
					if scope == "remote" {
						if level == "module" {
							remote.handleModuleStatus(response, "demo")
						} else {
							remote.handleBuildStatus(response, "demo", buildID)
						}
					} else if level == "module" {
						local.handleModuleStatus(response, "demo")
					} else {
						local.handleBuildStatus(response, "demo", buildID)
					}
					if response.Code != http.StatusOK {
						t.Fatalf("%s status = %d: %s", level, response.Code, response.Body.String())
					}
					payload := decodeResponseMap(t, response)
					wantPath := ""
					if scenario.wantPath {
						wantPath = developerDashboardPath
					}
					if payload["dashboard_path"] != wantPath {
						t.Errorf("%s dashboard_path = %#v, want %q", level, payload["dashboard_path"], wantPath)
					}
					wantRunningMode := ""
					if scenario.running && (scenario.mode == shared.DEVELOPMENT_MODE || scenario.mode == shared.LIVE_MODE) {
						wantRunningMode = scenario.mode
					}
					if payload["running_mode"] != wantRunningMode {
						t.Errorf("%s running_mode = %#v, want actual process mode %q", level, payload["running_mode"], wantRunningMode)
					}
					if level == "build" {
						wantSavedMode := scenario.savedMode
						if scope == "source-dev" {
							wantSavedMode = shared.DEVELOPMENT_MODE
						}
						if payload["runtime_mode"] != wantSavedMode {
							t.Errorf("saved runtime_mode = %#v, want %q", payload["runtime_mode"], wantSavedMode)
						}
					}
					if scenario.running && payload["port"] != float64(7001) {
						t.Errorf("%s port = %#v, want running process port", level, payload["port"])
					}
					if strings.Contains(response.Body.String(), "fixture-dashboard-secret") {
						t.Fatal("status leaked developer credentials")
					}
				}
			})
		}
	}
}

func TestDeployDashboardConfigReadIsStrictAndConfined(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "runtime")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	proc := deployProcess{RuntimeMode: shared.DEVELOPMENT_MODE}
	if got := deployDashboardPath(root, moduleRoot, proc); got != "" {
		t.Fatalf("missing config dashboard path = %q", got)
	}
	configPath := filepath.Join(moduleRoot, shared.PackageConfigFileName)
	if err := os.WriteFile(configPath, []byte(deployDashboardFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := shared.GetRuntimeOptions()
	t.Cleanup(func() { shared.SetRuntimeOptions(previous) })
	shared.SetRuntimeOptions(shared.RuntimeOptions{Production: true, ModeOverride: shared.LIVE_MODE})
	t.Setenv("HB_PRODUCTION", "true")
	t.Setenv("HB_DEPLOY_PRODUCTION", "true")
	if got := deployDashboardPath(root, moduleRoot, proc); got != developerDashboardPath {
		t.Fatalf("control-plane overrides changed dashboard path: %q", got)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, shared.PackageConfigFileName), []byte(deployDashboardFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	escapingRoot := filepath.Join(root, "escaping-runtime")
	if err := os.Symlink(outside, escapingRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if got := deployDashboardPath(root, escapingRoot, proc); got != "" {
		t.Fatalf("outside runtime config dashboard path = %q", got)
	}
}
