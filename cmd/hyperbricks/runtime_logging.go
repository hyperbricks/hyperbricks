package main

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func runtimeLogPath(file string) string {
	return logging.ModulePath(commands.GetModuleRoot(), file)
}

func runtimeSourcePath(file string) string {
	if filepath.IsAbs(file) {
		return runtimeLogPath(file)
	}
	return logging.ModuleText(commands.GetModuleRoot(), filepath.ToSlash(file))
}

func runtimeRoutePath(route string) string {
	if route == "index" || route == "" {
		return "/"
	}
	return "/" + strings.TrimPrefix(route, "/")
}

func runtimeRouteURL(route string) string {
	path := runtimeRoutePath(route)
	if shared.Location == "" {
		return path
	}
	return "http://" + strings.TrimSuffix(shared.Location, "/") + path
}

func runtimeLogFile(config *shared.Config) string {
	if file := strings.TrimSpace(config.Logger.Path); file != "" {
		return file
	}
	if config.Mode == shared.DEVELOPMENT_MODE || config.Mode == shared.DEBUG_MODE {
		if dir := strings.TrimSpace(config.Directories["logs"]); dir != "" {
			return filepath.Join(dir, "hyperbricks.log")
		}
	}
	return ""
}

func logRuntimeSummary(config *shared.Config) {
	logger := logging.GetLogger().Named("runtime")
	if logging.VerboseEnabled() {
		logger.Debugw("Module configured", "module", filepath.Base(commands.GetModuleRoot()),
			"config", runtimeLogPath(commands.GetModuleConfigPath()), "mode", config.Mode,
			"gomaxprocs", runtime.GOMAXPROCS(0))
	} else {
		logger.Infof("Module configured  module=%s mode=%s", filepath.Base(commands.GetModuleRoot()), config.Mode)
	}
	file := "disabled"
	if name := runtimeLogFile(config); name != "" {
		file = runtimeLogPath(name)
	}
	if logging.VerboseEnabled() {
		logger.Debugw("Logging configured", "level", logger.Level().String(), "file", file)
	}
	if commands.RenderStatic || shared.GetRuntimeOptions().Production || (config.Mode != shared.DEVELOPMENT_MODE && config.Mode != shared.DEBUG_MODE) {
		return
	}
	dashboard, errors, spaces := "disabled", "disabled", "disabled"
	if config.Development.Dashboard.Enabled {
		dashboard, errors = developerDashboardPath, errorsViewPath
	}
	spacesEnabled := shared.SpacesAvailable(config, shared.GetRuntimeOptions())
	if spacesEnabled {
		spaces = runtimeRoutePath(config.Development.FrontendEditing.Spaces.Route)
	}
	if logging.VerboseEnabled() {
		logger.Debugw("Developer tools", "dashboard", dashboard, "errors", errors, "spaces", spaces,
			"spaces_write", spaces != "disabled" && config.Development.FrontendEditing.Spaces.Write)
	} else if dashboard != "disabled" || spaces != "disabled" {
		logger.Infof("Developer tools  dashboard=%s spaces=%s write=%t", dashboard, spaces, spaces != "disabled" && config.Development.FrontendEditing.Spaces.Write)
	}
	credentials := config.Development.Dashboard.Credentials
	editorsEnabled := config.Mode == shared.DEVELOPMENT_MODE && config.Development.FrontendEditing.Enabled &&
		len(config.Development.FrontendEditing.Editors) > 0
	if config.Development.Dashboard.Enabled && credentials.Empty() {
		logger.Warn("Dashboard, Errors and diagnostics are accessible without login to anyone who can reach this server; configure hyperbricks.development.dashboard.credentials.user and .password to require login")
	}
	if credentials.Empty() && spacesEnabled {
		logger.Warnf("Spaces and contextual editing are accessible without login through allowed hosts; write=%t; configure hyperbricks.development.dashboard.credentials.user and .password to require login", config.Development.FrontendEditing.Spaces.Write)
	}
	if credentials.Empty() && editorsEnabled {
		logger.Warn("Frontend editor plugins locked: hyperbricks.development.dashboard.credentials is not configured")
	} else if !credentials.Empty() && !credentials.Complete() && (spacesEnabled || editorsEnabled || config.Development.Dashboard.Enabled) {
		logger.Warn("Developer interface locked: hyperbricks.development.dashboard.credentials is not fully configured")
	}
	watching := config.Mode == shared.DEVELOPMENT_MODE && config.Development.Watch
	directories := []string{}
	if watching {
		for _, dir := range resolveDevelopmentWatchDirectories(config) {
			directories = append(directories, runtimeLogPath(dir))
		}
	}
	if logging.VerboseEnabled() {
		logger.Debugw("Development refresh", "watch", watching, "directories", directories, "reload_configured", config.Development.Reload)
	} else if watching {
		logger.Infof("Watching directories  %s", strings.Join(directories, ", "))
	}
}

func printFilenameToRoutesMapping(filenameToRoutes map[string][]string, configs map[string]map[string]interface{}) {
	files := make([]string, 0, len(filenameToRoutes))
	for file := range filenameToRoutes {
		files = append(files, file)
	}
	sort.Strings(files)
	bySource := make(map[string][]map[string]string)
	for _, source := range files {
		routes := append([]string(nil), filenameToRoutes[source]...)
		sort.Strings(routes)
		for _, route := range routes {
			config := configs[route]
			meta := shared.MetaFromConfig(config)
			file := runtimeSourcePath(meta.HyperBricksFile)
			if file == "" || !strings.HasSuffix(file, ".hyperbricks.yaml") {
				file = runtimeLogPath(filepath.Join(core.ModuleDirectories.HyperbricksDir, source+".hyperbricks.yaml"))
			}
			row := map[string]string{"route": runtimeRoutePath(route), "type": meta.ConfigType}
			if target, ok := config["static"].(string); ok && target != "" {
				row["static"] = runtimeSourcePath(target)
			}
			bySource[file] = append(bySource[file], row)
		}
	}
	for _, rows := range bySource {
		sort.Slice(rows, func(i, j int) bool { return rows[i]["route"] < rows[j]["route"] })
	}
	if len(bySource) > 0 {
		logger := logging.GetLogger().Named("routes")
		count := 0
		for _, rows := range bySource {
			count += len(rows)
		}
		logger.Infof("Routes registered  count=%d", count)
		sources := make([]string, 0, len(bySource))
		for source := range bySource {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		for _, source := range sources {
			for _, row := range bySource[source] {
				logger.Infow("Route available", "route", row["route"], "url", runtimeRouteURL(row["route"]))
				if logging.VerboseEnabled() {
					fields := []interface{}{"route", row["route"], "type", strings.Trim(row["type"], "<>"), "source", filepath.Base(source)}
					if row["static"] != "" {
						fields = append(fields, "static", row["static"])
					}
					logger.Debugw("Route registered", fields...)
				}
			}
		}
	}
}
