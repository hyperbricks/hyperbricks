package commands

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetDeployCommandState(t *testing.T) {
	t.Helper()
	previousMode, previousPath := DeployServiceMode, DeployConfigPath
	previousModule, previousDir, previousBuild, previousRuntimeMode := DeployModule, DeployDir, DeployBuildID, DeployRuntimeMode
	previousStartMode, previousExit, previousExitCode := StartMode, Exit, ExitCode
	previousStartModule, previousModuleRoot, previousConfigPath := StartModule, ModuleRoot, ModuleConfigPath
	DeployServiceMode, DeployConfigPath = DeployServiceNone, ""
	DeployModule, DeployDir, DeployBuildID, DeployRuntimeMode = "", "", "", ""
	StartMode, Exit, ExitCode = false, false, 0
	StartModule, ModuleRoot, ModuleConfigPath = "", "", ""
	t.Cleanup(func() {
		DeployServiceMode, DeployConfigPath = previousMode, previousPath
		DeployModule, DeployDir, DeployBuildID, DeployRuntimeMode = previousModule, previousDir, previousBuild, previousRuntimeMode
		StartMode, Exit, ExitCode = previousStartMode, previousExit, previousExitCode
		StartModule, ModuleRoot, ModuleConfigPath = previousStartModule, previousModuleRoot, previousConfigPath
	})
}

func TestDeployRunUsesModulePathBaseName(t *testing.T) {
	resetDeployCommandState(t)
	workingDirectory := t.TempDir()
	changeStartWorkingDirectory(t, workingDirectory)
	deployRoot := filepath.Join(workingDirectory, "artifacts")
	moduleDeployDir := filepath.Join(deployRoot, "demo")
	if err := os.MkdirAll(moduleDeployDir, 0o755); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(moduleDeployDir, "demo-1.0-build-1.hra")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create(PackageConfigFileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("hyperbricks:\n  mode: live\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := updateBuildIndex(
		filepath.Join(moduleDeployDir, versionIndexFile),
		"build-1", "1.0", "hra", archivePath, "", "", "", "v1.2.5-beta", "",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := updateBuildIndex(
		filepath.Join(moduleDeployDir, versionIndexFile),
		"build-2", "1.0", "hra", archivePath, "", "", "", "v1.2.5-beta", "",
	); err != nil {
		t.Fatal(err)
	}

	cmd := NewDeployCommand()
	cmd.SetArgs([]string{"run", "-m", "./modules/demo", "--build", "build-1", "--deploy-dir", deployRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute deploy run: %v", err)
	}
	if !StartMode || Exit || StartModule != "demo" {
		t.Fatalf("run state = (start=%t, exit=%t, module=%q)", StartMode, Exit, StartModule)
	}
	wantRoot := filepath.Join(deployRoot, "demo", runtimeDirName, "build-1")
	if ModuleRoot != wantRoot || ModuleConfigPath != filepath.Join(wantRoot, PackageConfigFileName) {
		t.Fatalf("runtime selection = (%q, %q), want root %q", ModuleRoot, ModuleConfigPath, wantRoot)
	}
}

func TestDeployServiceCommandsSelectExplicitModeAndConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode DeployService
	}{
		{name: "local", mode: DeployServiceLocal},
		{name: "remote", mode: DeployServiceRemote},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetDeployCommandState(t)
			cmd := NewDeployCommand()
			cmd.SetArgs([]string{tt.name, "--config", filepath.Join("configs", "deploy.yaml")})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute deploy command: %v", err)
			}
			if DeployServiceMode != tt.mode || !StartMode || Exit {
				t.Fatalf("state = (%q, start=%t, exit=%t)", DeployServiceMode, StartMode, Exit)
			}
			if got := GetDeployConfigPath(); got != filepath.Join("configs", "deploy.yaml") {
				t.Fatalf("config = %q", got)
			}
		})
	}
}

func TestDeployWithoutSubcommandOnlyPrintsHelp(t *testing.T) {
	resetDeployCommandState(t)
	cmd := NewDeployCommand()
	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute deploy command: %v", err)
	}
	if StartMode || DeployServiceMode != DeployServiceNone {
		t.Fatalf("deploy unexpectedly selected a mode: %q", DeployServiceMode)
	}
	if !strings.Contains(output.String(), "Available Commands") {
		t.Fatalf("help output = %q", output.String())
	}
}

func TestDeployInitWritesNeutralConfigAndRefusesOverwrite(t *testing.T) {
	resetDeployCommandState(t)
	path := filepath.Join(t.TempDir(), "deploy.yaml")
	cmd := NewDeployCommand()
	cmd.SetArgs([]string{"init", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute deploy init: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, expected := range []string{"local:", "client:", "remote:", "credentials:", "hmac_secret:"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated config is missing %q", expected)
		}
	}
	if strings.Contains(content, "api_enabled:") || strings.Contains(content, "api_port:") {
		t.Fatalf("generated config contains obsolete fields:\n%s", content)
	}

	resetDeployCommandState(t)
	cmd = NewDeployCommand()
	cmd.SetArgs([]string{"init", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute second deploy init: %v", err)
	}
	if !Exit || ExitCode != 1 {
		t.Fatalf("overwrite exit state = (%t, %d)", Exit, ExitCode)
	}
}
