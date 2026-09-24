package commands

import (
	"os"
	"path/filepath"
	"strings"
)

func deployConfigPath() string {
	return resolveDeployConfigPath("", os.Getenv("HB_DEPLOY_CONFIG"))
}

func resolveDeployConfigPath(explicitPath, environmentPath string) string {
	if path := strings.TrimSpace(explicitPath); path != "" {
		return filepath.Clean(path)
	}
	if path := strings.TrimSpace(environmentPath); path != "" {
		return filepath.Clean(path)
	}
	return DeployConfigFileName
}

// GetStartDeployConfigPath returns the deployment config selected by the start
// flag, environment, or conventional default, in that order.
func GetStartDeployConfigPath() string {
	return resolveDeployConfigPath(StartDeployConfig, os.Getenv("HB_DEPLOY_CONFIG"))
}
