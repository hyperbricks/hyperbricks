package main

import (
	"path/filepath"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func (api *deployLocalServer) dashboardPath(module string, proc deployProcess) string {
	if !validDeployPathPart(module) || !validDeployPathPart(proc.BuildID) {
		return ""
	}
	if api.isDevBuildID(proc.BuildID) {
		return deployDashboardPath(api.modulesDir, filepath.Join(api.modulesDir, module), proc)
	}
	return deployDashboardPath(api.buildRoot, filepath.Join(api.buildRoot, module, "runtime", proc.BuildID), proc)
}

func (api *deployAPI) dashboardPath(module string, proc deployProcess) string {
	if !validDeployPathPart(module) || !validDeployPathPart(proc.BuildID) {
		return ""
	}
	return deployDashboardPath(api.root, filepath.Join(api.root, module, "runtime", proc.BuildID), proc)
}

// Status callers supply only a running process. Its launch mode is authoritative:
// a newly saved build mode may not yet be applied when a restart has failed.
// Do not extract archives or apply the deployment service's own runtime flags.
func deployDashboardPath(root, moduleRoot string, proc deployProcess) string {
	if proc.RuntimeMode != shared.DEVELOPMENT_MODE || proc.Production {
		return ""
	}
	moduleRoot, err := confinedDirectory(root, moduleRoot)
	if err != nil {
		return ""
	}
	content, _, err := readRegularConfinedFile(moduleRoot, filepath.Join(moduleRoot, shared.PackageConfigFileName))
	if err != nil {
		return ""
	}
	config, err := shared.ValidatePackageConfigBytes(content, moduleRoot)
	if err != nil || !config.Development.Dashboard.Enabled {
		return ""
	}
	return developerDashboardPath
}
