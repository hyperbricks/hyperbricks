package commands

import (
	"fmt"
	"os"
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

func resolveDirectStartModuleRoot(module, workingDirectory string) (string, error) {
	module = strings.TrimSpace(module)
	if module == "" {
		return "", fmt.Errorf("module name or directory path is empty")
	}

	if !isModulePath(module) {
		return filepath.Join("modules", module), nil
	}

	if filepath.IsAbs(module) {
		return filepath.Clean(module), nil
	}
	if strings.TrimSpace(workingDirectory) == "" {
		return "", fmt.Errorf("working directory is empty")
	}
	if !filepath.IsAbs(workingDirectory) {
		return "", fmt.Errorf("working directory must be absolute")
	}
	return filepath.Clean(filepath.Join(workingDirectory, module)), nil
}

func isModulePath(module string) bool {
	if module == "." || module == ".." || filepath.IsAbs(module) {
		return true
	}
	for index := 0; index < len(module); index++ {
		if os.IsPathSeparator(module[index]) {
			return true
		}
	}
	return false
}

func resolveModuleConfigPath(moduleRoot, configPath string) (string, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(configPath))
	if cleanPath == "." || cleanPath == "" {
		return "", fmt.Errorf("config path is empty")
	}
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("config path must be relative to the selected module")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("config path must stay inside the selected module")
	}
	return filepath.Join(moduleRoot, cleanPath), nil
}

func GetModulesRoot() string {
	return filepath.Dir(GetModuleRoot())
}
