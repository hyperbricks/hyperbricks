package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestMain(m *testing.M) {
	moduleRoot, err := os.MkdirTemp("", "hyperbricks-dedicated-tests-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create dedicated test module: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(moduleRoot, shared.PackageConfigFileName)
	config := []byte(`hyperbricks:
  mode: development
  development:
    watch: false
    reload: false
    frontend_errors: false
    dashboard:
      enabled: false
    frontend_editing:
      enabled: false
      spaces:
        enabled: false
  plugins:
    enabled: []
`)
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write dedicated test package configuration: %v\n", err)
		_ = os.RemoveAll(moduleRoot)
		os.Exit(1)
	}

	shared.Module = configPath
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: moduleRoot})

	exitCode := m.Run()
	if err := os.RemoveAll(moduleRoot); err != nil {
		fmt.Fprintf(os.Stderr, "remove dedicated test module: %v\n", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
