package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/mitchellh/mapstructure"
)

type deployLocalConfig struct {
	HMACSecret string                  `mapstructure:"hmac_secret"`
	Remote     deployLocalRemoteConfig `mapstructure:"remote"`
	Local      deployLocalSettings     `mapstructure:"local"`
	Client     deployClientConfig      `mapstructure:"client"`
}

type deployLocalRemoteConfig struct {
	Root        string `mapstructure:"root"`
	APIPort     int    `mapstructure:"api_port"`
	PortStart   int    `mapstructure:"port_start"`
	LogsEnabled bool   `mapstructure:"logs_enabled"`
}

type deployLocalSettings struct {
	Bind       string `mapstructure:"bind"`
	Port       int    `mapstructure:"port"`
	ModulesDir string `mapstructure:"modules_dir"`
	BuildRoot  string `mapstructure:"build_root"`
}

type deployClientConfig struct {
	Target  string                        `mapstructure:"target"`
	Targets map[string]deployClientTarget `mapstructure:"targets"`
}

type deployClientTarget struct {
	Host string `mapstructure:"host"`
	User string `mapstructure:"user"`
	Port int    `mapstructure:"port"`
	Root string `mapstructure:"root"`
	API  string `mapstructure:"api"`
}

type localBuildIndex struct {
	Current  string          `json:"current"`
	Port     int             `json:"port,omitempty"`
	Versions []localBuildRow `json:"versions"`
}

type localBuildRow struct {
	BuildID       string `json:"build_id"`
	ModuleVersion string `json:"moduleversion"`
	Format        string `json:"format"`
	File          string `json:"file"`
	BuiltAt       string `json:"built_at"`
	Commit        string `json:"commit"`
	SourceHash    string `json:"source_hash"`
	Production    bool   `json:"production,omitempty"`
	PushedAt      string `json:"pushed_at,omitempty"`
	RemoteTarget  string `json:"remote_target,omitempty"`
}

type localBuildRowResponse struct {
	localBuildRow
	RemoteStatus    string `json:"remote_status,omitempty"`
	RemoteCheckedAt string `json:"remote_checked_at,omitempty"`
	IsDev           bool   `json:"is_dev,omitempty"`
}

type remoteSyncState struct {
	target    string
	checkedAt time.Time
	builds    map[string]struct{}
}

type deployLocalServer struct {
	cfg         deployLocalConfig
	modulesDir  string
	buildRoot   string
	portStart   int
	logsEnabled bool
	syncMu      sync.Mutex
	syncState   map[string]remoteSyncState
	workingDir  string
	binaryPath  string
	pluginTasks *pluginTaskStore
}

type localSyncRequest struct {
	Module string `json:"module"`
	Target string `json:"target"`
}

type localPushRequest struct {
	Target string `json:"target"`
}

func startDeployLocalServer() error {
	configPath := deployLocalConfigPath()
	cfg, err := loadDeployLocalConfig(configPath)
	if err != nil {
		return err
	}
	if len(cfg.Client.Targets) == 0 {
		return fmt.Errorf("deploy.client.targets is required in %s", configPath)
	}

	bind := strings.TrimSpace(cfg.Local.Bind)
	if bind == "" {
		bind = "127.0.0.1"
	}
	port := cfg.Local.Port
	if port == 0 {
		port = 9091
	}
	modulesDir := strings.TrimSpace(cfg.Local.ModulesDir)
	if modulesDir == "" {
		modulesDir = "modules"
	}
	buildRoot := strings.TrimSpace(cfg.Local.BuildRoot)
	if buildRoot == "" {
		buildRoot = "deploy"
	}
	portStart := cfg.Remote.PortStart
	if portStart == 0 {
		portStart = 8080
	}
	logsEnabled := cfg.Remote.LogsEnabled

	api := &deployLocalServer{
		cfg:         cfg,
		modulesDir:  modulesDir,
		buildRoot:   buildRoot,
		portStart:   portStart,
		logsEnabled: logsEnabled,
		syncState:   make(map[string]remoteSyncState),
		workingDir:  "",
		binaryPath:  "",
		pluginTasks: newPluginTaskStore(),
	}
	if wd, err := os.Getwd(); err == nil {
		api.workingDir = wd
	}
	if exe, err := os.Executable(); err == nil {
		api.binaryPath = exe
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/local/status", api.handleStatus)
	mux.HandleFunc("/local/modules", api.handleModules)
	mux.HandleFunc("/local/modules/", api.handleModuleRoutes)
	mux.HandleFunc("/local/remote/sync", api.handleRemoteSync)
	mux.HandleFunc("/local/plugins", api.handlePluginRoutes)
	mux.HandleFunc("/local/plugins/", api.handlePluginRoutes)
	mux.HandleFunc("/assets/dashboard.css", serveDashboardCSS)
	mux.HandleFunc("/assets/logo.png", serveDashboardLogo)
	mux.HandleFunc("/assets/logo_blue.png", serveDashboardLogoBlue)
	mux.HandleFunc("/assets/logo_black.png", serveDashboardLogoBlack)
	mux.HandleFunc("/", api.serveLocalDashboard)

	addr := fmt.Sprintf("%s:%d", bind, port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("Deploy local listening on http://%s\n", addr)
	return server.ListenAndServe()
}

func deployLocalConfigPath() string {
	if envPath := strings.TrimSpace(os.Getenv("HB_DEPLOY_CONFIG")); envPath != "" {
		return envPath
	}
	return "deploy.hyperbricks"
}

func loadDeployLocalConfig(path string) (deployLocalConfig, error) {
	cfg := deployLocalConfig{
		Local: deployLocalSettings{
			Bind:       "127.0.0.1",
			Port:       9091,
			ModulesDir: "modules",
			BuildRoot:  "deploy",
		},
		Remote: deployLocalRemoteConfig{
			Root:        "deploy",
			APIPort:     9090,
			PortStart:   8080,
			LogsEnabled: true,
		},
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read deploy config %s: %w", path, err)
	}

	parsed := parser.ParseHyperScript(string(content))
	deployRaw, ok := parsed["deploy"].(map[string]interface{})
	if !ok {
		if hyper, ok := parsed["hyperbricks"].(map[string]interface{}); ok {
			deployRaw, _ = hyper["deploy"].(map[string]interface{})
		}
	}
	if deployRaw == nil {
		return cfg, fmt.Errorf("missing deploy block in %s", path)
	}
	if _, ok := deployRaw["local"].(map[string]interface{}); !ok {
		return cfg, fmt.Errorf("missing deploy.local block in %s", path)
	}
	if _, ok := deployRaw["client"].(map[string]interface{}); !ok {
		return cfg, fmt.Errorf("missing deploy.client block in %s", path)
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &cfg,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return cfg, err
	}
	if err := decoder.Decode(deployRaw); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (api *deployLocalServer) serveLocalDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	html := assets.DeployDashboard
	if strings.Contains(html, "data-mode=\"remote\"") {
		html = strings.Replace(html, "data-mode=\"remote\"", "data-mode=\"local\"", 1)
	} else {
		html = strings.Replace(html, "<body", "<body data-mode=\"local\"", 1)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, html)
}

func (api *deployLocalServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version := strings.TrimSpace(assets.VersionMD)
	payload := map[string]interface{}{
		"mode":         "local",
		"version":      version,
		"logs_enabled": api.logsEnabled,
		"target":       api.defaultTargetName(),
	}
	if name := api.defaultTargetName(); name != "" {
		if target, ok := api.cfg.Client.Targets[name]; ok {
			if normalized, err := api.normalizeTarget(target); err == nil {
				payload["target_api"] = normalized.API
				payload["target_root"] = normalized.Root
			}
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func (api *deployLocalServer) handleModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	modules, err := listLocalModules(api.modulesDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modules": modules,
		"target":  api.defaultTargetName(),
	})
}

func (api *deployLocalServer) handleModuleRoutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/local/")
	trimmed = strings.Trim(trimmed, "/")
	pathParts := strings.Split(trimmed, "/")
	if len(pathParts) < 2 || pathParts[0] != "modules" {
		writeError(w, http.StatusNotFound, errors.New("not found"))
		return
	}
	if len(pathParts) < 2 {
		writeError(w, http.StatusBadRequest, errors.New("module is required"))
		return
	}
	module := pathParts[1]

	if len(pathParts) == 3 && pathParts[2] == "status" && r.Method == http.MethodGet {
		api.handleModuleStatus(w, module)
		return
	}
	if len(pathParts) == 3 && pathParts[2] == "build" && r.Method == http.MethodPost {
		api.handleModuleBuild(w, module)
		return
	}
	if len(pathParts) == 3 && pathParts[2] == "activate" && r.Method == http.MethodPost {
		api.handleModuleActivate(w, r, module)
		return
	}
	if len(pathParts) == 3 && pathParts[2] == "restart" && r.Method == http.MethodPost {
		api.handleModuleRestart(w, module)
		return
	}
	if len(pathParts) == 3 && pathParts[2] == "stop" && r.Method == http.MethodPost {
		api.handleModuleStop(w, module)
		return
	}
	if len(pathParts) == 4 && pathParts[2] == "push" && r.Method == http.MethodPost {
		api.handleModulePush(w, r, module, pathParts[3])
		return
	}
	if len(pathParts) == 3 && pathParts[2] == "builds" && r.Method == http.MethodGet {
		api.handleModuleBuilds(w, module)
		return
	}
	if len(pathParts) == 5 && pathParts[2] == "builds" && pathParts[4] == "status" && r.Method == http.MethodGet {
		api.handleBuildStatus(w, module, pathParts[3])
		return
	}
	if len(pathParts) == 5 && pathParts[2] == "builds" && pathParts[4] == "logs" && r.Method == http.MethodGet {
		api.handleBuildLogs(w, r, module, pathParts[3])
		return
	}
	if len(pathParts) == 5 && pathParts[2] == "builds" && pathParts[4] == "production" && r.Method == http.MethodPost {
		api.handleBuildProduction(w, r, module, pathParts[3])
		return
	}
	if len(pathParts) == 5 && pathParts[2] == "builds" && pathParts[4] == "delete" && r.Method == http.MethodPost {
		api.handleBuildDelete(w, module, pathParts[3])
		return
	}

	writeError(w, http.StatusNotFound, errors.New("not found"))
}

func (api *deployLocalServer) handlePluginRoutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/local/")
	trimmed = strings.Trim(trimmed, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 1 || parts[0] != "plugins" {
		writeError(w, http.StatusNotFound, errors.New("not found"))
		return
	}
	segments := parts[1:]
	if len(segments) == 0 {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}

	switch segments[0] {
	case "global":
		api.handleLocalGlobalPluginRoutes(w, r, segments[1:])
	case "custom":
		api.handleLocalCustomPluginRoutes(w, r, segments[1:])
	case "tasks":
		api.handleLocalPluginTaskRoutes(w, r, segments[1:])
	default:
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
	}
}

func (api *deployLocalServer) handleLocalGlobalPluginRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 && r.Method == http.MethodGet {
		api.handleLocalGlobalPluginsList(w)
		return
	}
	if len(segments) == 1 && segments[0] == "index" && r.Method == http.MethodGet {
		api.handleLocalGlobalPluginsIndex(w)
		return
	}
	if len(segments) == 1 && r.Method == http.MethodPost {
		switch segments[0] {
		case "install", "rebuild", "remove":
			api.handleLocalGlobalPluginAction(w, r, segments[0])
			return
		}
	}
	writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
}

func (api *deployLocalServer) handleLocalCustomPluginRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 && r.Method == http.MethodGet {
		api.handleLocalCustomPluginsList(w, r)
		return
	}
	if len(segments) == 1 && segments[0] == "compile" && r.Method == http.MethodPost {
		api.handleLocalCustomPluginCompile(w, r)
		return
	}
	if len(segments) == 1 && segments[0] == "remove" && r.Method == http.MethodPost {
		api.handleLocalCustomPluginRemove(w, r)
		return
	}
	writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
}

func (api *deployLocalServer) handleLocalPluginTaskRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) < 1 {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}
	taskID := segments[0]
	if len(segments) == 2 && segments[1] == "logs" && r.Method == http.MethodGet {
		api.handleLocalPluginTaskLogs(w, taskID)
		return
	}
	if len(segments) == 1 && r.Method == http.MethodGet {
		api.handleLocalPluginTaskStatus(w, taskID)
		return
	}
	writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
}

func (api *deployLocalServer) handleLocalGlobalPluginsIndex(w http.ResponseWriter) {
	index, err := commands.FetchPluginIndex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	index = commands.FilterPluginIndexByHyperbricks(index, commands.HyperbricksSemver())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plugins": index,
	})
}

func (api *deployLocalServer) handleLocalGlobalPluginsList(w http.ResponseWriter) {
	pluginDir := filepath.Join(api.workingDir, "bin", "plugins")
	plugins, err := listPluginBinaries(pluginDir, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plugins": plugins,
	})
}

func (api *deployLocalServer) handleLocalGlobalPluginAction(w http.ResponseWriter, r *http.Request, action string) {
	var req pluginGlobalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Version = strings.TrimSpace(req.Version)
	if req.Name == "" || req.Version == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and version are required"))
		return
	}

	var cmdAction string
	switch action {
	case "install":
		cmdAction = "install"
	case "rebuild":
		cmdAction = "build"
	case "remove":
		cmdAction = "remove"
	default:
		writeError(w, http.StatusNotFound, errors.New("unknown action"))
		return
	}

	args := []string{"plugin", cmdAction, fmt.Sprintf("%s@%s", req.Name, req.Version)}
	task := api.runPluginCommandTask(args)
	writeJSON(w, http.StatusOK, task.snapshot())
}

func (api *deployLocalServer) handleLocalCustomPluginsList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	module := strings.TrimSpace(query.Get("module"))
	if module == "" {
		writeError(w, http.StatusBadRequest, errors.New("module is required"))
		return
	}

	configPath := filepath.Join(api.modulesDir, module, "package.hyperbricks")
	pluginRoot := filepath.Join(api.modulesDir, module, "plugins")
	pluginDir := filepath.Join(api.workingDir, "bin", "plugins")

	configNames, err := readPluginConfigNames(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	sourceEntries, err := scanPluginSourceEntries(pluginRoot, module)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	plugins := make([]pluginListEntry, 0, len(configNames))
	moduleSuffix := "__" + module
	for _, configName := range configNames {
		base, version := splitConfigName(configName)
		if !strings.HasSuffix(base, moduleSuffix) {
			continue
		}
		entry, ok := sourceEntries[configName]
		outputName := configName + ".so"
		sourcePath := ""
		status := "source-missing"
		if ok {
			outputName = entry.OutputName
			sourcePath = entry.SourceDir
			status = "missing"
			if pluginBinaryExists(pluginDir, entry.OutputName) {
				status = "installed"
			}
		}
		if !ok && pluginBinaryExists(pluginDir, outputName) {
			status = "installed"
		}
		plugins = append(plugins, pluginListEntry{
			Name:       base,
			Version:    version,
			BinaryName: outputName,
			ConfigName: configName,
			Kind:       "custom",
			Module:     module,
			SourcePath: sourcePath,
			BinaryPath: filepath.Join("bin", "plugins", outputName),
			Status:     status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plugins": plugins,
	})
}

func (api *deployLocalServer) handleLocalCustomPluginCompile(w http.ResponseWriter, r *http.Request) {
	var req pluginCustomCompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.Plugin = strings.TrimSpace(req.Plugin)
	if req.Module == "" || req.Plugin == "" {
		writeError(w, http.StatusBadRequest, errors.New("module and plugin are required"))
		return
	}

	pluginRoot := filepath.Join(api.modulesDir, req.Module, "plugins")
	sourceEntries, err := scanPluginSourceEntries(pluginRoot, req.Module)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	entry, ok := sourceEntries[req.Plugin]
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("plugin source not found"))
		return
	}

	task := api.pluginTasks.newTask()
	go func() {
		task.start()
		var buffer bytes.Buffer
		spec := commands.PluginBuildSpec{
			SourceDir:   entry.SourceDir,
			SourceFile:  entry.Meta.Source,
			OutputName:  entry.OutputName,
			DisplayName: entry.ConfigName,
			LogWriter:   &buffer,
		}
		err := commands.BuildPlugin(spec)
		task.finish(err, buffer.String())
	}()

	writeJSON(w, http.StatusOK, task.snapshot())
}

func (api *deployLocalServer) handleLocalCustomPluginRemove(w http.ResponseWriter, r *http.Request) {
	var req pluginCustomRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.Plugin = strings.TrimSpace(req.Plugin)
	if req.Module == "" || req.Plugin == "" {
		writeError(w, http.StatusBadRequest, errors.New("module and plugin are required"))
		return
	}

	task := api.pluginTasks.newTask()
	go func() {
		task.start()
		_, err := removePluginBinary(api.workingDir, req.Plugin)
		task.finish(err, "")
	}()

	writeJSON(w, http.StatusOK, task.snapshot())
}

func (api *deployLocalServer) handleLocalPluginTaskStatus(w http.ResponseWriter, taskID string) {
	task, ok := api.pluginTasks.get(taskID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("task not found"))
		return
	}
	writeJSON(w, http.StatusOK, task.snapshot())
}

func (api *deployLocalServer) handleLocalPluginTaskLogs(w http.ResponseWriter, taskID string) {
	task, ok := api.pluginTasks.get(taskID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("task not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"log":     task.logText(),
	})
}

func (api *deployLocalServer) runPluginCommandTask(args []string) *pluginTask {
	task := api.pluginTasks.newTask()
	go func() {
		task.start()
		output, err := runCommand(api.binaryPath, api.workingDir, args)
		task.finish(err, output)
	}()
	return task
}

func (api *deployLocalServer) handleModuleStatus(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	port := 0
	running := false
	runningBuild := ""
	if proc, ok := api.readProcess(module); ok {
		if isProcessRunning(proc.PID) {
			running = true
			runningBuild = proc.BuildID
			port = proc.Port
		} else {
			api.clearProcess(module)
		}
	}
	if port == 0 && index.Current != "" {
		port = api.readRuntimePort(module, index.Current)
		if port == 0 && index.Port > 0 {
			port = index.Port
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":        module,
		"current":       index.Current,
		"port":          port,
		"versions":      len(index.Versions),
		"running":       running,
		"running_build": runningBuild,
	})
}

func (api *deployLocalServer) handleModuleBuilds(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	annotated, checkedAt := api.annotateBuilds(module, index.Versions)
	if devRow, ok := api.devBuildRow(module); ok {
		annotated = append([]localBuildRowResponse{devRow}, annotated...)
	}
	payload := map[string]interface{}{
		"module":   module,
		"current":  index.Current,
		"versions": annotated,
	}
	if !checkedAt.IsZero() {
		payload["remote_checked_at"] = checkedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (api *deployLocalServer) handleBuildStatus(w http.ResponseWriter, module string, buildID string) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}

	if api.isDevBuildID(buildID) {
		status, err := api.devBuildStatus(module)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
		return
	}

	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	row, ok := findLocalRow(index, buildID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}
	status := api.annotateBuildRow(module, row)
	running := false
	port := 0
	if proc, ok := api.readBuildProcessFile(module, buildID); ok {
		if isProcessRunning(proc.PID) {
			running = true
			port = proc.Port
		} else {
			api.clearBuildProcess(module, buildID)
		}
	}
	if port == 0 {
		port = api.readRuntimePort(module, buildID)
		if port == 0 && buildID == index.Current && index.Port > 0 {
			port = index.Port
		}
	}

	payload := map[string]interface{}{
		"module":            module,
		"build_id":          buildID,
		"running":           running,
		"port":              port,
		"moduleversion":     status.ModuleVersion,
		"commit":            status.Commit,
		"built_at":          status.BuiltAt,
		"source_hash":       status.SourceHash,
		"format":            status.Format,
		"production":        status.Production,
		"pushed_at":         status.PushedAt,
		"remote_target":     status.RemoteTarget,
		"remote_status":     status.RemoteStatus,
		"remote_checked_at": status.RemoteCheckedAt,
	}
	writeJSON(w, http.StatusOK, payload)
}

func (api *deployLocalServer) handleBuildProduction(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	if api.isDevBuildID(buildID) {
		writeError(w, http.StatusBadRequest, errors.New("cannot set production on dev build"))
		return
	}
	body, err := readJSONBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var payload struct {
		Production bool `json:"production"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	updated := false
	for i := range index.Versions {
		if index.Versions[i].BuildID == buildID {
			index.Versions[i].Production = payload.Production
			updated = true
			break
		}
	}
	if !updated {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}
	if err := saveLocalBuildIndex(indexPath, index); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":     module,
		"build_id":   buildID,
		"production": payload.Production,
	})
}

func (api *deployLocalServer) handleBuildDelete(w http.ResponseWriter, module string, buildID string) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}
	if api.isDevBuildID(buildID) {
		writeError(w, http.StatusBadRequest, errors.New("cannot delete dev build"))
		return
	}
	if strings.Contains(buildID, "..") || strings.Contains(buildID, "/") || strings.Contains(buildID, "\\") {
		writeError(w, http.StatusBadRequest, errors.New("invalid build_id"))
		return
	}

	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	row, ok := findLocalRow(index, buildID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}

	archivePath := strings.TrimSpace(row.File)
	if archivePath != "" {
		path := filepath.FromSlash(archivePath)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	runtimeDir := filepath.Join(api.buildRoot, module, "runtime", buildID)
	_ = os.RemoveAll(runtimeDir)
	logPath := filepath.Join(api.buildRoot, module, "logs", buildID+".log")
	_ = os.Remove(logPath)

	index = removeLocalRow(index, buildID)
	if index.Current == buildID {
		index.Current = ""
	}
	if index.Current != "" {
		if _, ok := findLocalRow(index, index.Current); !ok {
			index.Current = ""
		}
	}
	if index.Current == "" && len(index.Versions) > 0 {
		index.Current = index.Versions[len(index.Versions)-1].BuildID
	}

	moduleDeleted := false
	if len(index.Versions) == 0 {
		moduleDir := filepath.Join(api.buildRoot, module)
		if err := os.RemoveAll(moduleDir); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		moduleDeleted = true
	} else {
		if err := saveLocalBuildIndex(indexPath, index); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":         module,
		"build_id":       buildID,
		"deleted":        true,
		"current":        index.Current,
		"rollback":       false,
		"rollback_build": "",
		"rollback_error": "",
		"module_deleted": moduleDeleted,
	})
}

func (api *deployLocalServer) handleModuleBuild(w http.ResponseWriter, module string) {
	result, err := commands.BuildModuleWithOptions(commands.BuildOptions{
		Module: module,
		OutDir: api.buildRoot,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":    module,
		"build_id":  result.BuildID,
		"archive":   result.ArchivePath,
		"new_build": result.Built,
	})
}

func (api *deployLocalServer) handleModulePush(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	body, err := readJSONBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var payload localPushRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	targetName := strings.TrimSpace(payload.Target)
	resolvedName, _, err := api.resolveTarget(targetName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	archivePath, resolvedID, err := commands.ResolveDeployArchive(module, api.buildRoot, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	targetName, err = commands.PushBuildToTarget(module, resolvedID, archivePath, resolvedName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	pushedAt := time.Now().UTC().Format(time.RFC3339)
	if err := api.markBuildPushed(module, resolvedID, targetName, pushedAt); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	checkedAt, err := api.syncRemoteModule(module, targetName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":            module,
		"build_id":          resolvedID,
		"target":            targetName,
		"pushed_at":         pushedAt,
		"remote_checked_at": checkedAt.Format(time.RFC3339),
	})
}

func (api *deployLocalServer) handleRemoteSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := readJSONBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var payload localSyncRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	module := strings.TrimSpace(payload.Module)
	target := strings.TrimSpace(payload.Target)

	var modules []string
	if module != "" {
		modules = []string{module}
	} else {
		modules, err = listLocalModules(api.modulesDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	results := make([]map[string]interface{}, 0, len(modules))
	for _, mod := range modules {
		checkedAt, syncErr := api.syncRemoteModule(mod, target)
		item := map[string]interface{}{
			"module": mod,
		}
		if syncErr != nil {
			item["error"] = syncErr.Error()
		} else {
			item["remote_checked_at"] = checkedAt.Format(time.RFC3339)
		}
		results = append(results, item)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

func (api *deployLocalServer) indexPath(module string) string {
	return filepath.Join(api.buildRoot, module, "hyperbricks.versions.json")
}

func (api *deployLocalServer) defaultTargetName() string {
	target := strings.TrimSpace(api.cfg.Client.Target)
	if target != "" {
		return target
	}
	if len(api.cfg.Client.Targets) == 1 {
		for name := range api.cfg.Client.Targets {
			return name
		}
	}
	return ""
}

func (api *deployLocalServer) resolveTarget(explicit string) (string, deployClientTarget, error) {
	name := strings.TrimSpace(explicit)
	if name == "" {
		name = api.defaultTargetName()
	}
	if name == "" {
		return "", deployClientTarget{}, errors.New("deploy target is required")
	}
	target, ok := api.cfg.Client.Targets[name]
	if !ok {
		return "", deployClientTarget{}, fmt.Errorf("deploy target not found: %s", name)
	}
	return name, target, nil
}

func (api *deployLocalServer) annotateBuilds(module string, rows []localBuildRow) ([]localBuildRowResponse, time.Time) {
	checkedAt, present := api.remoteSyncSnapshot(module)
	annotated := make([]localBuildRowResponse, 0, len(rows))
	for _, row := range rows {
		resp := api.annotateBuildRowWithSnapshot(row, checkedAt, present)
		annotated = append(annotated, resp)
	}
	return annotated, checkedAt
}

func (api *deployLocalServer) annotateBuildRow(module string, row localBuildRow) localBuildRowResponse {
	checkedAt, present := api.remoteSyncSnapshot(module)
	return api.annotateBuildRowWithSnapshot(row, checkedAt, present)
}

func (api *deployLocalServer) annotateBuildRowWithSnapshot(row localBuildRow, checkedAt time.Time, present map[string]struct{}) localBuildRowResponse {
	resp := localBuildRowResponse{
		localBuildRow: row,
	}
	if !checkedAt.IsZero() {
		resp.RemoteCheckedAt = checkedAt.Format(time.RFC3339)
	}
	if row.PushedAt == "" {
		return resp
	}
	if len(present) == 0 {
		resp.RemoteStatus = "unknown"
		return resp
	}
	if _, ok := present[row.BuildID]; ok {
		resp.RemoteStatus = "pushed"
	} else {
		resp.RemoteStatus = "missing"
	}
	return resp
}

func (api *deployLocalServer) remoteSyncSnapshot(module string) (time.Time, map[string]struct{}) {
	api.syncMu.Lock()
	defer api.syncMu.Unlock()
	state, ok := api.syncState[module]
	if !ok {
		return time.Time{}, map[string]struct{}{}
	}
	copyMap := make(map[string]struct{}, len(state.builds))
	for id := range state.builds {
		copyMap[id] = struct{}{}
	}
	return state.checkedAt, copyMap
}

func (api *deployLocalServer) syncRemoteModule(module string, targetName string) (time.Time, error) {
	name, target, err := api.resolveTarget(targetName)
	if err != nil {
		return time.Time{}, err
	}
	target, err = api.normalizeTarget(target)
	if err != nil {
		return time.Time{}, err
	}
	secret := strings.TrimSpace(api.cfg.HMACSecret)
	if secret == "" || strings.Contains(secret, "{{") {
		secret = strings.TrimSpace(os.Getenv("HB_DEPLOY_SECRET"))
	}
	if secret == "" {
		return time.Time{}, errors.New("deploy.hmac_secret or HB_DEPLOY_SECRET is required for sync")
	}

	buildIDs, err := fetchRemoteBuildIDs(target.API, module, secret)
	if err != nil {
		return time.Time{}, err
	}

	builds := make(map[string]struct{}, len(buildIDs))
	for _, id := range buildIDs {
		builds[id] = struct{}{}
	}
	checkedAt := time.Now().UTC()

	api.syncMu.Lock()
	api.syncState[module] = remoteSyncState{
		target:    name,
		checkedAt: checkedAt,
		builds:    builds,
	}
	api.syncMu.Unlock()

	return checkedAt, nil
}

func (api *deployLocalServer) normalizeTarget(target deployClientTarget) (deployClientTarget, error) {
	target.Host = strings.TrimSpace(target.Host)
	if target.Host == "" {
		return target, errors.New("deploy target host is required")
	}
	if target.Port == 0 {
		target.Port = 22
	}
	target.User = strings.TrimSpace(target.User)
	target.Root = strings.TrimSpace(target.Root)
	if target.Root == "" {
		target.Root = strings.TrimSpace(api.cfg.Remote.Root)
	}
	target.API = strings.TrimSpace(target.API)
	if target.API == "" {
		port := api.cfg.Remote.APIPort
		if port == 0 {
			port = 9090
		}
		target.API = fmt.Sprintf("http://%s:%d", target.Host, port)
	}
	return target, nil
}

func (api *deployLocalServer) markBuildPushed(module string, buildID string, targetName string, pushedAt string) error {
	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		return err
	}
	updated := false
	for i := range index.Versions {
		if index.Versions[i].BuildID == buildID {
			index.Versions[i].PushedAt = pushedAt
			index.Versions[i].RemoteTarget = targetName
			updated = true
			break
		}
	}
	if !updated {
		return fmt.Errorf("build id not found: %s", buildID)
	}
	return saveLocalBuildIndex(indexPath, index)
}

func loadLocalBuildIndex(path string) (localBuildIndex, error) {
	index := localBuildIndex{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index, nil
		}
		return index, err
	}
	if len(data) == 0 {
		return index, nil
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return index, err
	}
	return index, nil
}

func saveLocalBuildIndex(path string, index localBuildIndex) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func findLocalRow(index localBuildIndex, buildID string) (localBuildRow, bool) {
	for _, row := range index.Versions {
		if row.BuildID == buildID {
			return row, true
		}
	}
	return localBuildRow{}, false
}

func removeLocalRow(index localBuildIndex, buildID string) localBuildIndex {
	if buildID == "" {
		return index
	}
	updated := make([]localBuildRow, 0, len(index.Versions))
	for _, row := range index.Versions {
		if row.BuildID == buildID {
			continue
		}
		updated = append(updated, row)
	}
	index.Versions = updated
	return index
}

func listLocalModules(modulesDir string) ([]string, error) {
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	modules := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		configPath := filepath.Join(modulesDir, name, "package.hyperbricks")
		if _, err := os.Stat(configPath); err != nil {
			continue
		}
		modules = append(modules, name)
	}
	sort.Strings(modules)
	return modules, nil
}

func readJSONBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	return body, nil
}

func fetchRemoteBuildIDs(apiBase string, module string, secret string) ([]string, error) {
	endpoint, err := buildRemoteURL(apiBase, module, "builds")
	if err != nil {
		return nil, err
	}
	headers, err := signLocalDeployHeaders(http.MethodGet, endpoint.Path, nil, secret)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("remote sync failed: %s", message)
	}

	var payload struct {
		Versions []struct {
			BuildID string `json:"build_id"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(payload.Versions))
	for _, row := range payload.Versions {
		if row.BuildID != "" {
			ids = append(ids, row.BuildID)
		}
	}
	return ids, nil
}

func buildRemoteURL(base string, module string, tail string) (*url.URL, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil, errors.New("deploy target api is required")
	}
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid deploy api url: %s", base)
	}
	joined := path.Join(parsed.Path, "deploy", "modules", module, tail)
	parsed.Path = "/" + strings.TrimPrefix(joined, "/")
	parsed.RawQuery = ""
	return parsed, nil
}

func signLocalDeployHeaders(method string, requestPath string, body []byte, secret string) (map[string]string, error) {
	nonce, err := randomHexLocal(32)
	if err != nil {
		return nil, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	hash := sha256.Sum256(body)
	canonical := strings.Join([]string{
		method,
		requestPath,
		hex.EncodeToString(hash[:]),
		ts,
		nonce,
	}, "\n")

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))

	return map[string]string{
		"X-HB-Timestamp": ts,
		"X-HB-Nonce":     nonce,
		"X-HB-Signature": signature,
	}, nil
}

func randomHexLocal(bytesLen int) (string, error) {
	data := make([]byte, bytesLen)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
