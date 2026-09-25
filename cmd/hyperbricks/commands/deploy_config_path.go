package commands

import (
	"os"
	"path/filepath"
	"strings"
)

const DeployConfigFileName = "deploy.hyperbricks.yaml"

func deployConfigPath() string {
	return resolveDeployConfigPath(DeployConfigPath, os.Getenv("HB_DEPLOY_CONFIG"))
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

// GetDeployConfigPath returns the deployment config selected by the deploy
// command, environment, or conventional default, in that order.
func GetDeployConfigPath() string {
	return resolveDeployConfigPath(DeployConfigPath, os.Getenv("HB_DEPLOY_CONFIG"))
}
