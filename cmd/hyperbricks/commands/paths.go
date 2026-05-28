package commands

import (
	"path/filepath"
	"strings"
)

var (
	ModuleRoot       string
	ModuleConfigPath string
)

const PackageConfigFileName = "package.hyperbricks.yaml"

func GetModuleRoot() string {
	if strings.TrimSpace(ModuleRoot) != "" {
		return filepath.Clean(ModuleRoot)
	}
	if strings.TrimSpace(StartModule) == "" {
		return filepath.Join("modules", "default")
	}
	return filepath.Join("modules", StartModule)
}

func GetModuleConfigPath() string {
	if strings.TrimSpace(ModuleConfigPath) != "" {
		return filepath.Clean(ModuleConfigPath)
	}
	return filepath.Join(GetModuleRoot(), PackageConfigFileName)
}

func GetModulesRoot() string {
	return filepath.Dir(GetModuleRoot())
}
