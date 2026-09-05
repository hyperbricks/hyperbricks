package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func resetStartCommandState(t *testing.T) {
	t.Helper()

	previousStartMode := StartMode
	previousStartModule := StartModule
	previousStartConfigPath := StartConfigPath
	previousStartDeploy := StartDeploy
	previousStartDeployDir := StartDeployDir
	previousStartBuildID := StartBuildID
	previousStartDeployRemote := StartDeployRemote
	previousStartDeployLocal := StartDeployLocal
	previousStartDeployInit := StartDeployInit
	previousStartRuntimeGateway := StartRuntimeGateway
	previousStartRuntimeDomain := StartRuntimeDomain
	previousStartRuntimeHostSuffix := StartRuntimeHostSuffix
	previousStartRuntimeResolver := StartRuntimeResolver
	previousPort := Port
	previousProduction := Production
	previousDebug := Debug
	previousModuleRoot := ModuleRoot
	previousModuleConfigPath := ModuleConfigPath
	previousExit := Exit
	previousExitCode := ExitCode

	StartMode = false
	StartModule = ""
	StartConfigPath = ""
	StartDeploy = false
	StartDeployDir = ""
	StartBuildID = ""
	StartDeployRemote = false
	StartDeployLocal = false
	StartDeployInit = ""
	StartRuntimeGateway = false
	StartRuntimeDomain = ""
	StartRuntimeHostSuffix = ""
	StartRuntimeResolver = ""
	Port = 0
	Production = false
	Debug = false
	ModuleRoot = ""
	ModuleConfigPath = ""
	Exit = false
	ExitCode = 0

	t.Cleanup(func() {
		StartMode = previousStartMode
		StartModule = previousStartModule
		StartConfigPath = previousStartConfigPath
		StartDeploy = previousStartDeploy
		StartDeployDir = previousStartDeployDir
		StartBuildID = previousStartBuildID
		StartDeployRemote = previousStartDeployRemote
		StartDeployLocal = previousStartDeployLocal
		StartDeployInit = previousStartDeployInit
		StartRuntimeGateway = previousStartRuntimeGateway
		StartRuntimeDomain = previousStartRuntimeDomain
		StartRuntimeHostSuffix = previousStartRuntimeHostSuffix
		StartRuntimeResolver = previousStartRuntimeResolver
		Port = previousPort
		Production = previousProduction
		Debug = previousDebug
		ModuleRoot = previousModuleRoot
		ModuleConfigPath = previousModuleConfigPath
		Exit = previousExit
		ExitCode = previousExitCode
	})
}

func changeStartWorkingDirectory(t *testing.T, directory string) {
	t.Helper()

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}

func TestStartCommandResolvesExplicitModuleAndConfigPaths(t *testing.T) {
	resetStartCommandState(t)
	workingDirectory := t.TempDir()
	changeStartWorkingDirectory(t, workingDirectory)
	resolvedWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}

	moduleDirectory := filepath.Join(resolvedWorkingDirectory, "modules", "demo")
	configPath := filepath.Join(moduleDirectory, "profiles", "development.hyperbricks.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("hyperbricks:\n  mode: development\n"), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	command := NewStartCommand()
	command.SetArgs([]string{"--module", "./modules/demo", "--config", "profiles/development.hyperbricks.yaml"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute start command: %v", err)
	}

	if ModuleRoot != moduleDirectory {
		t.Fatalf("module root = %q, want %q", ModuleRoot, moduleDirectory)
	}
	if ModuleConfigPath != configPath {
		t.Fatalf("module config path = %q, want %q", ModuleConfigPath, configPath)
	}
	if !StartMode {
		t.Fatal("start mode = false, want true")
	}
	if Exit || ExitCode != 0 {
		t.Fatalf("exit state = (%t, %d), want successful startup", Exit, ExitCode)
	}
	if currentDirectory, err := os.Getwd(); err != nil || currentDirectory != resolvedWorkingDirectory {
		t.Fatalf("working directory = %q, %v; want %q", currentDirectory, err, resolvedWorkingDirectory)
	}
}

func TestStartCommandMissingConfigFailsAtResolvedPath(t *testing.T) {
	resetStartCommandState(t)
	workingDirectory := t.TempDir()
	changeStartWorkingDirectory(t, workingDirectory)
	resolvedWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}

	command := NewStartCommand()
	command.SetArgs([]string{"--module", "./modules/missing"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute start command: %v", err)
	}

	wantRoot := filepath.Join(resolvedWorkingDirectory, "modules", "missing")
	wantConfig := filepath.Join(wantRoot, PackageConfigFileName)
	if ModuleRoot != wantRoot || GetModuleConfigPath() != wantConfig {
		t.Fatalf("resolved selection = (%q, %q), want (%q, %q)", ModuleRoot, GetModuleConfigPath(), wantRoot, wantConfig)
	}
	if !Exit || ExitCode != 1 {
		t.Fatalf("exit state = (%t, %d), want failed startup", Exit, ExitCode)
	}
}

func TestStartCommandRejectsExplicitEmptyModuleSelection(t *testing.T) {
	resetStartCommandState(t)
	changeStartWorkingDirectory(t, t.TempDir())

	command := NewStartCommand()
	command.SetArgs([]string{"--module", ""})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute start command: %v", err)
	}

	if ModuleRoot != "" {
		t.Fatalf("module root = %q, want no selected root", ModuleRoot)
	}
	if !Exit || ExitCode != 1 {
		t.Fatalf("exit state = (%t, %d), want rejected selection", Exit, ExitCode)
	}
}
