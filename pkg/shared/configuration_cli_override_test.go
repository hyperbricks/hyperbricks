package shared

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetConfigurationForTest(t *testing.T) {
	t.Helper()

	previousInstance := instance
	previousModule := Module
	previousRuntimeOptions := GetRuntimeOptions()

	instance = nil
	once = sync.Once{}
	Module = ""
	SetRuntimeOptions(RuntimeOptions{})

	t.Cleanup(func() {
		instance = previousInstance
		once = sync.Once{}
		Module = previousModule
		SetRuntimeOptions(previousRuntimeOptions)
	})
}

func writePackageConfig(t *testing.T, root string, moduleRoot string, content string) {
	t.Helper()

	configPath := filepath.Join(root, moduleRoot, PackageConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed to create package config directory: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write package config: %v", err)
	}
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})
}

func TestLoadHyperBricksConfigurationAppliesCLIOverrides(t *testing.T) {
	Init_configuration()
	resetConfigurationForTest(t)

	root := t.TempDir()
	moduleRoot := filepath.ToSlash(filepath.Join("runtime", "current"))
	writePackageConfig(t, root, moduleRoot, `
hyperbricks:
  mode: development
  server:
    port: 7070
    runtime_gateway:
      enabled: false
      domain: config.runtime.local
      host_suffix: -config-runtime.local
      resolver: http://config-resolver.local/resolve
  directories:
    render:
      path:
        base: module
        path: rendered
    templates:
      path:
        base: module
        path: templates
    hyperbricks:
      path:
        base: module
        path: hyperbricks
    static:
      path:
        base: module
        path: static
    resources:
      path:
        base: module
        path: resources
`)
	chdirForTest(t, root)

	Module = filepath.Join(moduleRoot, PackageConfigFileName)
	SetRuntimeOptions(RuntimeOptions{
		ModuleRoot: moduleRoot,

		Port:         9099,
		PortOverride: true,

		Production: true,

		RuntimeGatewayEnabled:    true,
		RuntimeGatewayDomain:     "cli.runtime.local",
		RuntimeGatewayHostSuffix: "-cli-runtime.local",
		RuntimeGatewayResolver:   "http://cli-resolver.local/resolve",
	})

	config := GetHyperBricksConfiguration()

	if config.Mode != LIVE_MODE {
		t.Fatalf("mode = %q, want %q", config.Mode, LIVE_MODE)
	}
	if config.Server.Port != 9099 {
		t.Fatalf("server.port = %d, want CLI override 9099", config.Server.Port)
	}
	if !config.Server.RuntimeGateway.Enabled {
		t.Fatal("runtime gateway enabled = false, want CLI override true")
	}
	if config.Server.RuntimeGateway.Domain != "cli.runtime.local" {
		t.Fatalf("runtime gateway domain = %q, want CLI override", config.Server.RuntimeGateway.Domain)
	}
	if config.Server.RuntimeGateway.HostSuffix != "-cli-runtime.local" {
		t.Fatalf("runtime gateway host suffix = %q, want CLI override", config.Server.RuntimeGateway.HostSuffix)
	}
	if config.Server.RuntimeGateway.Resolver != "http://cli-resolver.local/resolve" {
		t.Fatalf("runtime gateway resolver = %q, want CLI override", config.Server.RuntimeGateway.Resolver)
	}
	if config.Directories["render"] != moduleRoot+"/rendered" {
		t.Fatalf("render directory = %q, want module root override", config.Directories["render"])
	}
}

func TestLoadHyperBricksConfigurationKeepsConfiguredPortWhenCLIPortIsDefault(t *testing.T) {
	Init_configuration()
	resetConfigurationForTest(t)

	root := t.TempDir()
	moduleRoot := filepath.ToSlash(filepath.Join("modules", "demo"))
	writePackageConfig(t, root, moduleRoot, `
hyperbricks:
  server:
    port: 7070
`)
	chdirForTest(t, root)

	Module = filepath.Join(moduleRoot, PackageConfigFileName)
	SetRuntimeOptions(RuntimeOptions{
		ModuleRoot: moduleRoot,
		Port:       8080,
	})

	config := GetHyperBricksConfiguration()

	if config.Server.Port != 7070 {
		t.Fatalf("server.port = %d, want configured port 7070 when CLI port is default", config.Server.Port)
	}
}

func TestLoadHyperBricksConfigurationIgnoresRuntimePortWithoutOverride(t *testing.T) {
	Init_configuration()
	resetConfigurationForTest(t)

	root := t.TempDir()
	moduleRoot := filepath.ToSlash(filepath.Join("modules", "demo"))
	writePackageConfig(t, root, moduleRoot, `
hyperbricks:
  server:
    port: 7070
`)
	chdirForTest(t, root)

	Module = filepath.Join(moduleRoot, PackageConfigFileName)
	SetRuntimeOptions(RuntimeOptions{
		ModuleRoot: moduleRoot,
		Port:       9099,
	})

	config := GetHyperBricksConfiguration()

	if config.Server.Port != 7070 {
		t.Fatalf("server.port = %d, want configured port 7070 when runtime PortOverride is false", config.Server.Port)
	}
}

func TestRuntimeModuleRootDefaultsToDefaultModule(t *testing.T) {
	if got := runtimeModuleRoot(RuntimeOptions{}); got != "modules/default" {
		t.Fatalf("runtime module root = %q, want modules/default", got)
	}
}
