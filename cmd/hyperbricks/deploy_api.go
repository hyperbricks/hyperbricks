package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

const (
	deployIndexFile       = "hyperbricks.versions.json"
	hmacTimeWindow        = 60 * time.Second
	maxPortCandidate      = 65535
	processStartTolerance = 10 * time.Second
	maxLogLines           = 1000
	defaultLogLines       = 200
	maxLogBytes           = 256 * 1024
)

type deployIndex struct {
	Current  string           `json:"current"`
	Port     int              `json:"port,omitempty"`
	Versions []deployIndexRow `json:"versions"`
}

type deployIndexRow struct {
	BuildID       string `json:"build_id"`
	ModuleVersion string `json:"moduleversion"`
	Format        string `json:"format"`
	File          string `json:"file"`
	BuiltAt       string `json:"built_at"`
	Commit        string `json:"commit"`
	SourceHash    string `json:"source_hash"`
	Production    bool   `json:"production,omitempty"`
}

type deployActivateRequest struct {
	BuildID string `json:"build_id"`
}

type deployBuildProductionRequest struct {
	Production bool `json:"production"`
}

type pluginGlobalRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type pluginCustomCompileRequest struct {
	Module  string `json:"module"`
	BuildID string `json:"build_id"`
	Plugin  string `json:"plugin"`
}

type pluginCustomRemoveRequest struct {
	Module  string `json:"module"`
	BuildID string `json:"build_id"`
	Plugin  string `json:"plugin"`
}

type deployAPI struct {
	root        string
	secret      string
	apiPort     int
	portStart   int
	logsEnabled bool
	binaryPath  string
	workingDir  string
	nonceStore  *deployNonceStore
	pluginTasks *pluginTaskStore
}

type deployProcess struct {
	Module      string `json:"module"`
	PID         int    `json:"pid"`
	BuildID     string `json:"build_id"`
	Port        int    `json:"port"`
	StartedAt   string `json:"started_at"`
	StartedUnix int64  `json:"started_unix"`
	Binary      string `json:"binary"`
	Command     string `json:"command"`
	Production  bool   `json:"production,omitempty"`
}

type deployNonceStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

func newDeployNonceStore() *deployNonceStore {
	return &deployNonceStore{entries: make(map[string]time.Time)}
}

func (s *deployNonceStore) seen(nonce string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, ts := range s.entries {
		if now.Sub(ts) > hmacTimeWindow {
			delete(s.entries, key)
		}
	}

	if _, ok := s.entries[nonce]; ok {
		return true
	}
	s.entries[nonce] = now
	return false
}

func startDeployAPIServer() error {
	configPath := "deploy.hyperbricks"
	if envPath := strings.TrimSpace(os.Getenv("HB_DEPLOY_CONFIG")); envPath != "" {
		configPath = envPath
	}

	deployCfg, err := loadDeployConfig(configPath)
	if err != nil {
		return err
	}
	if !deployCfg.Remote.APIEnabled {
		return fmt.Errorf("deploy api disabled in %s", configPath)
	}

	root := strings.TrimSpace(deployCfg.Remote.Root)
	if root == "" {
		root = "deploy"
	}
	if envRoot := strings.TrimSpace(os.Getenv("HB_DEPLOY_ROOT")); envRoot != "" {
		root = envRoot
	}

	bind := strings.TrimSpace(deployCfg.Remote.APIBind)
	if bind == "" {
		bind = "127.0.0.1"
	}
	if envBind := strings.TrimSpace(os.Getenv("HB_DEPLOY_BIND")); envBind != "" {
		bind = envBind
	}

	port := deployCfg.Remote.APIPort
	if port == 0 {
		port = 9090
	}
	if envPort := strings.TrimSpace(os.Getenv("HB_DEPLOY_PORT")); envPort != "" {
		if parsed, err := strconv.Atoi(envPort); err == nil && parsed > 0 {
			port = parsed
		}
	}

	secret := strings.TrimSpace(deployCfg.HMACSecret)
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("HB_DEPLOY_SECRET"))
	}
	if secret == "" {
		return fmt.Errorf("deploy api requires deploy.hmac_secret or HB_DEPLOY_SECRET")
	}

	portStart := deployCfg.Remote.PortStart
	if portStart == 0 {
		portStart = 8080
	}
	if envStart := strings.TrimSpace(os.Getenv("HB_DEPLOY_PORT_START")); envStart != "" {
		if parsed, err := strconv.Atoi(envStart); err == nil && parsed > 0 {
			portStart = parsed
		}
	}

	logsEnabled := deployCfg.Remote.LogsEnabled
	if envLogs := strings.TrimSpace(os.Getenv("HB_DEPLOY_LOGS")); envLogs != "" {
		logsEnabled = envLogs == "1" || strings.EqualFold(envLogs, "true")
	}

	binaryPath := strings.TrimSpace(deployCfg.Remote.Binary)
	if envBin := strings.TrimSpace(os.Getenv("HB_DEPLOY_BIN")); envBin != "" {
		binaryPath = envBin
	}
	if binaryPath == "" {
		if exe, err := os.Executable(); err == nil {
			binaryPath = exe
		}
	}

	workingDir, _ := os.Getwd()

	api := &deployAPI{
		root:        root,
		secret:      secret,
		apiPort:     port,
		portStart:   portStart,
		logsEnabled: logsEnabled,
		binaryPath:  binaryPath,
		workingDir:  workingDir,
		nonceStore:  newDeployNonceStore(),
		pluginTasks: newPluginTaskStore(),
	}

	mux := http.NewServeMux()
	mux.Handle("/deploy/", api.wrapAuth(api.handleDeploy))
	mux.HandleFunc("/assets/dashboard.css", serveDashboardCSS)
	mux.HandleFunc("/assets/logo.png", serveDashboardLogo)
	mux.HandleFunc("/assets/logo_blue.png", serveDashboardLogoBlue)
	mux.HandleFunc("/assets/logo_black.png", serveDashboardLogoBlack)
	mux.HandleFunc("/", serveDeployDashboard)

	addr := fmt.Sprintf("%s:%d", bind, port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("Deploy API listening on http://%s\n", addr)
	return server.ListenAndServe()
}

func serveDeployDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/deploy-ui" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, assets.DeployDashboard)
}

func serveDashboardCSS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(".css"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, assets.DashboardCSS)
}

func serveDashboardLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(".png"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(assets.Logo)
}

func serveDashboardLogoBlue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(".png"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(assets.Logo_Blue)
}

func serveDashboardLogoBlack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(".png"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(assets.Logo_Black)
}

func loadDeployConfig(path string) (shared.DeployConfig, error) {
	cfg := shared.DeployConfig{
		Remote: shared.DeployRemoteConfig{
			APIEnabled:  true,
			APIBind:     "127.0.0.1",
			APIPort:     9090,
			Root:        "deploy",
			PortStart:   8080,
			LogsEnabled: true,
		},
		Local: shared.DeployLocalConfig{
			Bind:       "127.0.0.1",
			Port:       9091,
			ModulesDir: "modules",
			BuildRoot:  "deploy",
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
	if _, ok := deployRaw["remote"].(map[string]interface{}); !ok {
		return cfg, fmt.Errorf("missing deploy.remote block in %s", path)
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

func (api *deployAPI) wrapAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := readBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := api.verifyRequest(r, body); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		next(w, r)
	}
}

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func (api *deployAPI) verifyRequest(r *http.Request, body []byte) error {
	tsHeader := strings.TrimSpace(r.Header.Get("X-HB-Timestamp"))
	nonce := strings.TrimSpace(r.Header.Get("X-HB-Nonce"))
	signature := strings.TrimSpace(r.Header.Get("X-HB-Signature"))

	if tsHeader == "" || nonce == "" || signature == "" {
		return errors.New("missing HMAC headers")
	}

	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	now := time.Now().UTC()
	tsTime := time.Unix(ts, 0).UTC()
	diff := now.Sub(tsTime)
	if diff < -hmacTimeWindow || diff > hmacTimeWindow {
		return errors.New("timestamp outside allowed window")
	}

	if api.nonceStore.seen(nonce, now) {
		return errors.New("nonce already used")
	}

	hash := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(hash[:])

	canonical := strings.Join([]string{
		r.Method,
		r.URL.Path,
		bodyHash,
		tsHeader,
		nonce,
	}, "\n")

	mac := hmac.New(sha256.New, []byte(api.secret))
	_, _ = mac.Write([]byte(canonical))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid signature")
	}
	return nil
}

func (api *deployAPI) handleDeploy(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) < 2 || segments[0] != "deploy" {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}

	if segments[1] == "admin" {
		if len(segments) < 3 {
			writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
			return
		}
		action := segments[2]
		switch {
		case r.Method == http.MethodPost && action == "kill-all":
			api.handleKillAll(w, r)
		default:
			writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		}
		return
	}

	if segments[1] == "status" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		api.handleAPIStatus(w)
		return
	}

	if segments[1] == "plugins" {
		api.handlePluginRoutes(w, r, segments[2:])
		return
	}

	if segments[1] != "modules" {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}

	if len(segments) == 2 && r.Method == http.MethodGet {
		api.handleListModules(w)
		return
	}

	if len(segments) < 4 {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}

	module := segments[2]
	action := segments[3]

	if action == "builds" && len(segments) >= 6 && r.Method == http.MethodPost && segments[5] == "production" {
		api.handleBuildProduction(w, r, module, segments[4])
		return
	}
	if action == "builds" && len(segments) >= 6 && r.Method == http.MethodGet && segments[5] == "status" {
		api.handleBuildStatus(w, module, segments[4])
		return
	}
	if action == "builds" && len(segments) >= 6 && r.Method == http.MethodGet && segments[5] == "logs" {
		api.handleBuildLogs(w, r, module, segments[4])
		return
	}
	if action == "builds" && len(segments) >= 6 && r.Method == http.MethodPost && segments[5] == "delete" {
		api.handleBuildDelete(w, module, segments[4])
		return
	}

	switch {
	case r.Method == http.MethodGet && action == "builds":
		api.handleModuleBuilds(w, module)
	case r.Method == http.MethodGet && action == "status":
		api.handleModuleStatus(w, module)
	case r.Method == http.MethodPost && action == "activate":
		api.handleModuleActivate(w, r, module)
	case r.Method == http.MethodPost && action == "rollback":
		api.handleModuleRollback(w, module)
	case r.Method == http.MethodPost && action == "restart":
		api.handleModuleRestart(w, module)
	case r.Method == http.MethodPost && action == "stop":
		api.handleModuleStop(w, module)
	default:
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
	}
}

func (api *deployAPI) handleListModules(w http.ResponseWriter) {
	entries, err := os.ReadDir(api.root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	modules := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			modules = append(modules, entry.Name())
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modules": modules,
	})
}

func (api *deployAPI) handleAPIStatus(w http.ResponseWriter) {
	version := strings.TrimSpace(assets.VersionMD)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":      version,
		"logs_enabled": api.logsEnabled,
	})
}

func (api *deployAPI) handlePluginRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}
	switch segments[0] {
	case "global":
		api.handleGlobalPluginRoutes(w, r, segments[1:])
	case "custom":
		api.handleCustomPluginRoutes(w, r, segments[1:])
	case "tasks":
		api.handlePluginTaskRoutes(w, r, segments[1:])
	default:
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
	}
}

func (api *deployAPI) handleGlobalPluginRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 && r.Method == http.MethodGet {
		api.handleGlobalPluginsList(w)
		return
	}
	if len(segments) == 1 && segments[0] == "index" && r.Method == http.MethodGet {
		api.handleGlobalPluginsIndex(w)
		return
	}
	if len(segments) == 1 && r.Method == http.MethodPost {
		switch segments[0] {
		case "install", "rebuild", "remove":
			api.handleGlobalPluginAction(w, r, segments[0])
			return
		}
	}
	writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
}

func (api *deployAPI) handleCustomPluginRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 && r.Method == http.MethodGet {
		api.handleCustomPluginsList(w, r)
		return
	}
	if len(segments) == 1 && segments[0] == "compile" && r.Method == http.MethodPost {
		api.handleCustomPluginCompile(w, r)
		return
	}
	if len(segments) == 1 && segments[0] == "remove" && r.Method == http.MethodPost {
		api.handleCustomPluginRemove(w, r)
		return
	}
	writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
}

func (api *deployAPI) handlePluginTaskRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) < 1 {
		writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
		return
	}
	taskID := segments[0]
	if len(segments) == 2 && segments[1] == "logs" && r.Method == http.MethodGet {
		api.handlePluginTaskLogs(w, taskID)
		return
	}
	if len(segments) == 1 && r.Method == http.MethodGet {
		api.handlePluginTaskStatus(w, taskID)
		return
	}
	writeError(w, http.StatusNotFound, errors.New("unknown endpoint"))
}

func (api *deployAPI) handleGlobalPluginsIndex(w http.ResponseWriter) {
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

func (api *deployAPI) handleGlobalPluginsList(w http.ResponseWriter) {
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

func (api *deployAPI) handleGlobalPluginAction(w http.ResponseWriter, r *http.Request, action string) {
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

func (api *deployAPI) handleCustomPluginsList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	module := strings.TrimSpace(query.Get("module"))
	buildID := strings.TrimSpace(query.Get("build_id"))
	if module == "" || buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("module and build_id are required"))
		return
	}

	runtimeDir := filepath.Join(api.root, module, "runtime", buildID)
	configPath := filepath.Join(runtimeDir, "package.hyperbricks")
	pluginRoot := filepath.Join(runtimeDir, "plugins")
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
			BuildID:    buildID,
			SourcePath: sourcePath,
			BinaryPath: filepath.Join("bin", "plugins", outputName),
			Status:     status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plugins": plugins,
	})
}

func (api *deployAPI) handleCustomPluginCompile(w http.ResponseWriter, r *http.Request) {
	var req pluginCustomCompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.BuildID = strings.TrimSpace(req.BuildID)
	req.Plugin = strings.TrimSpace(req.Plugin)
	if req.Module == "" || req.BuildID == "" || req.Plugin == "" {
		writeError(w, http.StatusBadRequest, errors.New("module, build_id, and plugin are required"))
		return
	}

	runtimeDir := filepath.Join(api.root, req.Module, "runtime", req.BuildID)
	pluginRoot := filepath.Join(runtimeDir, "plugins")
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

func (api *deployAPI) handleCustomPluginRemove(w http.ResponseWriter, r *http.Request) {
	var req pluginCustomRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.BuildID = strings.TrimSpace(req.BuildID)
	req.Plugin = strings.TrimSpace(req.Plugin)
	if req.Module == "" || req.BuildID == "" || req.Plugin == "" {
		writeError(w, http.StatusBadRequest, errors.New("module, build_id, and plugin are required"))
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

func (api *deployAPI) handlePluginTaskStatus(w http.ResponseWriter, taskID string) {
	task, ok := api.pluginTasks.get(taskID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("task not found"))
		return
	}
	writeJSON(w, http.StatusOK, task.snapshot())
}

func (api *deployAPI) handlePluginTaskLogs(w http.ResponseWriter, taskID string) {
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

func (api *deployAPI) runPluginCommandTask(args []string) *pluginTask {
	task := api.pluginTasks.newTask()
	go func() {
		task.start()
		output, err := runCommand(api.binaryPath, api.workingDir, args)
		task.finish(err, output)
	}()
	return task
}

func (api *deployAPI) handleModuleBuilds(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, index)
}

func (api *deployAPI) handleBuildProduction(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}

	var req deployBuildProductionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	row, ok := findDeployRow(index, buildID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}

	row.Production = req.Production
	index = upsertDeployRow(index, row)
	if err := saveDeployIndex(indexPath, index); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	restarted := false
	if proc, ok := api.readProcess(module); ok && proc.BuildID == buildID {
		if err := api.restartModule(module, buildID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		restarted = true
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":     module,
		"build_id":   buildID,
		"production": req.Production,
		"restarted":  restarted,
	})
}

func (api *deployAPI) handleBuildStatus(w http.ResponseWriter, module string, buildID string) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}

	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	row, ok := findDeployRow(index, buildID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}

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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":        module,
		"build_id":      buildID,
		"running":       running,
		"port":          port,
		"moduleversion": row.ModuleVersion,
		"commit":        row.Commit,
		"built_at":      row.BuiltAt,
		"source_hash":   row.SourceHash,
		"format":        row.Format,
		"production":    row.Production,
	})
}

func (api *deployAPI) handleBuildLogs(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	if !api.logsEnabled {
		writeError(w, http.StatusForbidden, errors.New("logs disabled"))
		return
	}
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}
	if strings.Contains(buildID, "..") || strings.Contains(buildID, "/") || strings.Contains(buildID, "\\") {
		writeError(w, http.StatusBadRequest, errors.New("invalid build_id"))
		return
	}

	lines := parseLogLines(r, defaultLogLines)
	if lines > maxLogLines {
		lines = maxLogLines
	}

	logPath := api.buildLogPath(module, buildID)
	if _, err := os.Stat(logPath); err != nil {
		fallback := api.moduleLogPath(module)
		if _, err := os.Stat(fallback); err != nil {
			writeError(w, http.StatusNotFound, errors.New("log file not found"))
			return
		}
		logPath = fallback
	}

	content, truncated, err := readLogTail(logPath, lines, maxLogBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":     module,
		"build_id":   buildID,
		"lines":      lines,
		"truncated":  truncated,
		"log_path":   api.relativePath(logPath),
		"log_output": content,
	})
}

func (api *deployAPI) handleBuildDelete(w http.ResponseWriter, module string, buildID string) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}
	if strings.Contains(buildID, "..") || strings.Contains(buildID, "/") || strings.Contains(buildID, "\\") {
		writeError(w, http.StatusBadRequest, errors.New("invalid build_id"))
		return
	}

	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if _, ok := findDeployRow(index, buildID); !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}

	isCurrent := index.Current == buildID
	rollbackTarget := ""
	if isCurrent {
		for i := len(index.Versions) - 1; i >= 0; i-- {
			if index.Versions[i].BuildID != buildID {
				rollbackTarget = index.Versions[i].BuildID
				break
			}
		}
		if err := api.stopModule(module, buildID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		if _, _, err := api.stopManagedProcessByBuild(module, buildID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	archivePath, _ := api.resolveArchivePath(module, buildID, index)
	runtimeDir := filepath.Join(api.root, module, "runtime", buildID)
	logPath := api.buildLogPath(module, buildID)

	_ = os.RemoveAll(runtimeDir)
	_ = os.Remove(logPath)
	api.clearBuildProcess(module, buildID)
	if isCurrent {
		api.clearProcess(module)
	}
	if strings.TrimSpace(archivePath) != "" {
		if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	index = removeDeployRow(index, buildID)
	if isCurrent {
		index.Current = rollbackTarget
		if rollbackTarget == "" {
			index.Port = 0
		}
	}
	if index.Current != "" {
		if _, ok := findDeployRow(index, index.Current); !ok {
			index.Current = ""
			index.Port = 0
		}
	}
	moduleDeleted := false
	if len(index.Versions) == 0 {
		moduleDir := filepath.Join(api.root, module)
		if err := os.RemoveAll(moduleDir); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		moduleDeleted = true
	} else {
		if err := saveDeployIndex(indexPath, index); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	rolledBack := false
	rollbackErr := ""
	if isCurrent && rollbackTarget != "" && !moduleDeleted {
		if err := api.restartModule(module, rollbackTarget); err != nil {
			rollbackErr = err.Error()
		} else {
			rolledBack = true
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":         module,
		"build_id":       buildID,
		"deleted":        true,
		"current":        index.Current,
		"rollback":       rolledBack,
		"rollback_build": rollbackTarget,
		"rollback_error": rollbackErr,
		"module_deleted": moduleDeleted,
	})
}

func (api *deployAPI) handleModuleStatus(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	port := index.Port
	if port == 0 && index.Current != "" {
		runtimeDir := filepath.Join(api.root, module, "runtime", index.Current)
		configPath := filepath.Join(runtimeDir, "package.hyperbricks")
		if _, err := os.Stat(configPath); err == nil {
			packagePort, _ := readServerPort(configPath)
			if packagePort > 0 {
				port = packagePort
			}
		}
	}

	running := false
	runningBuild := ""
	if proc, ok := api.readProcess(module); ok {
		if isProcessRunning(proc.PID) {
			running = true
			runningBuild = proc.BuildID
		} else {
			api.clearProcess(module)
		}
	}
	if running && runningBuild == "" {
		runningBuild = index.Current
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

func (api *deployAPI) handleModuleActivate(w http.ResponseWriter, r *http.Request, module string) {
	var req deployActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	buildID := strings.TrimSpace(req.BuildID)
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("build_id is required"))
		return
	}

	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	archivePath, err := api.resolveArchivePath(module, buildID, index)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	archivePath, err = api.moveToArchives(module, archivePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	runtimeDir, err := commands.EnsureRuntimeExtracted(archivePath, api.root, module, buildID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	configPath := filepath.Join(runtimeDir, "package.hyperbricks")
	metadata, packagePort, err := readMetadataAndPort(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if packagePort > 0 {
		index.Port = packagePort
	}

	production := false
	if existing, ok := findDeployRow(index, buildID); ok {
		production = existing.Production
	}

	row := deployIndexRow{
		BuildID:       buildID,
		ModuleVersion: metadata["moduleversion"],
		Format:        metadata["format"],
		File:          api.relativePath(archivePath),
		BuiltAt:       metadata["built_at"],
		Commit:        metadata["commit"],
		SourceHash:    metadata["source_hash"],
		Production:    production,
	}
	index = upsertDeployRow(index, row)
	index.Current = buildID

	if err := saveDeployIndex(indexPath, index); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := api.restartModule(module, buildID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	assignedPort := index.Port
	if refreshed, err := loadDeployIndex(indexPath); err == nil {
		assignedPort = refreshed.Port
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":    module,
		"build_id":  buildID,
		"port":      assignedPort,
		"runtime":   runtimeDir,
		"archived":  api.relativePath(archivePath),
		"activated": true,
	})
}

func (api *deployAPI) handleModuleRollback(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if len(index.Versions) < 2 {
		writeError(w, http.StatusBadRequest, errors.New("no previous build available"))
		return
	}

	var previous string
	for i := len(index.Versions) - 1; i >= 0; i-- {
		if index.Versions[i].BuildID != index.Current {
			previous = index.Versions[i].BuildID
			break
		}
	}
	if previous == "" {
		writeError(w, http.StatusBadRequest, errors.New("no previous build available"))
		return
	}

	index.Current = previous
	if err := saveDeployIndex(indexPath, index); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := api.restartModule(module, previous); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":    module,
		"build_id":  previous,
		"rollback":  true,
		"activated": true,
	})
}

func (api *deployAPI) handleModuleRestart(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	buildID := index.Current
	if err := api.restartModule(module, buildID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":   module,
		"build_id": buildID,
		"restart":  true,
	})
}

func (api *deployAPI) handleModuleStop(w http.ResponseWriter, module string) {
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	buildID := index.Current
	if err := api.stopModule(module, buildID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":   module,
		"build_id": buildID,
		"stop":     true,
	})
}

type killAllRequest struct {
	KeepPort int `json:"keep_port"`
}

func (api *deployAPI) handleKillAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keepPort := api.apiPort
	if r.Body != nil {
		var req killAllRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.KeepPort > 0 {
				keepPort = req.KeepPort
			}
		}
	}

	exclude := map[int]struct{}{
		os.Getpid(): {},
	}
	for pid := range pidsListeningOnPort(keepPort) {
		exclude[pid] = struct{}{}
	}

	candidates, err := listHyperbricksProcesses()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var killed []int
	var skipped []int
	var failed []string
	for _, proc := range candidates {
		if _, ok := exclude[proc.PID]; ok {
			skipped = append(skipped, proc.PID)
			continue
		}
		if strings.Contains(proc.Command, "--deploy-remote") {
			skipped = append(skipped, proc.PID)
			continue
		}
		if err := terminateProcess(proc.PID); err != nil {
			failed = append(failed, fmt.Sprintf("%d: %v", proc.PID, err))
			continue
		}
		killed = append(killed, proc.PID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"killed":    killed,
		"skipped":   skipped,
		"failed":    failed,
		"keep_port": keepPort,
	})
}

func (api *deployAPI) restartModule(module string, buildID string) error {
	return api.startManaged(module, buildID)
}

func (api *deployAPI) stopModule(module string, buildID string) error {
	return api.stopManaged(module)
}

func (api *deployAPI) pidPath(module string) string {
	return filepath.Join(api.root, module, "hyperbricks.pid")
}

func (api *deployAPI) processDir(module string) string {
	return filepath.Join(api.root, module, "processes")
}

func (api *deployAPI) buildPidPath(module string, buildID string) string {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return ""
	}
	return filepath.Join(api.processDir(module), buildID+".json")
}

func (api *deployAPI) logDir(module string) string {
	return filepath.Join(api.root, module, "logs")
}

func (api *deployAPI) buildLogPath(module string, buildID string) string {
	return filepath.Join(api.logDir(module), buildID+".log")
}

func (api *deployAPI) moduleLogPath(module string) string {
	return filepath.Join(api.root, module, "hyperbricks.log")
}

func (api *deployAPI) readProcessFile(path string) (deployProcess, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return deployProcess{}, false
	}
	var proc deployProcess
	if err := json.Unmarshal(data, &proc); err != nil {
		return deployProcess{}, false
	}
	if proc.PID <= 0 {
		return deployProcess{}, false
	}
	return proc, true
}

func (api *deployAPI) readProcess(module string) (deployProcess, bool) {
	path := api.pidPath(module)
	proc, ok := api.readProcessFile(path)
	if !ok {
		return deployProcess{}, false
	}
	if !isProcessRunning(proc.PID) {
		return deployProcess{}, false
	}
	return proc, true
}

func (api *deployAPI) readBuildProcessFile(module string, buildID string) (deployProcess, bool) {
	path := api.buildPidPath(module, buildID)
	if path == "" {
		return deployProcess{}, false
	}
	return api.readProcessFile(path)
}

func (api *deployAPI) readBuildProcess(module string, buildID string) (deployProcess, bool) {
	proc, ok := api.readBuildProcessFile(module, buildID)
	if !ok {
		return deployProcess{}, false
	}
	if !isProcessRunning(proc.PID) {
		return deployProcess{}, false
	}
	return proc, true
}

func (api *deployAPI) writeProcessFile(path string, proc deployProcess) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(proc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (api *deployAPI) writeProcess(module string, proc deployProcess) error {
	path := api.pidPath(module)
	if err := api.writeProcessFile(path, proc); err != nil {
		return err
	}
	buildPath := api.buildPidPath(module, proc.BuildID)
	if buildPath == "" {
		return nil
	}
	return api.writeProcessFile(buildPath, proc)
}

func (api *deployAPI) clearProcess(module string) {
	path := api.pidPath(module)
	_ = os.Remove(path)
}

func (api *deployAPI) clearBuildProcess(module string, buildID string) {
	path := api.buildPidPath(module, buildID)
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

func (api *deployAPI) findProcessesByPort(module string, port int) []deployProcess {
	if port <= 0 {
		return nil
	}
	dir := api.processDir(module)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var matches []deployProcess
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		proc, ok := api.readProcessFile(path)
		if !ok {
			continue
		}
		if proc.Port != port {
			continue
		}
		if !isProcessRunning(proc.PID) {
			_ = os.Remove(path)
			continue
		}
		matches = append(matches, proc)
	}
	return matches
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
	return syscall.Kill(pid, 0) == nil
}

func signalProcess(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	if runtime.GOOS == "windows" {
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return process.Signal(sig)
	}
	if err := syscall.Kill(-pid, sig); err == nil {
		return nil
	} else if errors.Is(err, syscall.ESRCH) {
		// Try direct PID signal when process group doesn't exist.
	} else if errors.Is(err, syscall.EPERM) {
		return err
	}
	if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	if runtime.GOOS == "windows" {
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return process.Kill()
	}

	if err := signalProcess(pid, syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		if !isProcessRunning(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := signalProcess(pid, syscall.SIGKILL); err != nil {
		return err
	}
	return nil
}

func getProcessDetails(pid int) (time.Time, string, error) {
	if pid <= 0 {
		return time.Time{}, "", errors.New("invalid pid")
	}
	if startedAt, command, err := getProcessDetailsFromPS(pid); err == nil {
		return startedAt, command, nil
	} else if runtime.GOOS == "linux" {
		if startedAt, command, err := getProcessDetailsFromProc(pid); err == nil {
			return startedAt, command, nil
		} else {
			return time.Time{}, "", err
		}
	} else {
		return time.Time{}, "", err
	}
}

func getProcessDetailsFromPS(pid int) (time.Time, string, error) {
	cmd := exec.Command("ps", "-ww", "-o", "lstart=", "-o", "command=", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, "", err
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return time.Time{}, "", errors.New("process info not found")
	}
	fields := strings.Fields(text)
	if len(fields) < 6 {
		return time.Time{}, "", fmt.Errorf("unexpected ps output: %q", text)
	}
	startStr := strings.Join(fields[:5], " ")
	startedAt, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", startStr, time.Local)
	if err != nil {
		return time.Time{}, "", err
	}
	command := strings.Join(fields[5:], " ")
	return startedAt, command, nil
}

func getProcessDetailsFromProc(pid int) (time.Time, string, error) {
	cmdlinePath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	cmdlineRaw, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return time.Time{}, "", err
	}
	cmdline := strings.TrimSpace(strings.ReplaceAll(string(cmdlineRaw), "\x00", " "))
	if cmdline == "" {
		return time.Time{}, "", errors.New("empty cmdline")
	}

	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	statRaw, err := os.ReadFile(statPath)
	if err != nil {
		return time.Time{}, "", err
	}
	statText := strings.TrimSpace(string(statRaw))
	closeIdx := strings.LastIndex(statText, ")")
	if closeIdx == -1 {
		return time.Time{}, "", errors.New("invalid stat format")
	}
	fields := strings.Fields(statText[closeIdx+1:])
	if len(fields) < 20 {
		return time.Time{}, "", errors.New("stat missing starttime")
	}
	startTicks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return time.Time{}, "", err
	}

	startedAt, err := startTimeFromProc(startTicks)
	if err != nil {
		return time.Time{}, cmdline, nil
	}
	return startedAt, cmdline, nil
}

type processCandidate struct {
	PID     int
	Command string
}

func isHyperbricksCommand(command string) bool {
	return strings.Contains(command, "hyperbricks")
}

func listHyperbricksProcesses() ([]processCandidate, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("process listing not supported on windows")
	}
	cmd := exec.Command("ps", "-A", "-o", "pid=", "-o", "command=")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var processes []processCandidate
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		command := strings.Join(fields[1:], " ")
		if !isHyperbricksCommand(command) {
			continue
		}
		processes = append(processes, processCandidate{PID: pid, Command: command})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return processes, nil
}

func pidsListeningOnPort(port int) map[int]struct{} {
	pids := make(map[int]struct{})
	if port <= 0 || runtime.GOOS == "windows" {
		return pids
	}
	if _, err := exec.LookPath("lsof"); err != nil {
		return pids
	}
	cmd := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-Fp")
	output, err := cmd.Output()
	if err != nil {
		return pids
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "p") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimPrefix(line, "p"))
		if err != nil || pid <= 0 {
			continue
		}
		pids[pid] = struct{}{}
	}
	return pids
}

func startTimeFromProc(startTicks int64) (time.Time, error) {
	statRaw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	var bootTime int64
	for _, line := range strings.Split(string(statRaw), "\n") {
		if strings.HasPrefix(line, "btime ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				bootTime, err = strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					return time.Time{}, err
				}
				break
			}
		}
	}
	if bootTime == 0 {
		return time.Time{}, errors.New("boot time not found")
	}

	clockTicks, err := readClockTicks()
	if err != nil {
		return time.Time{}, err
	}
	seconds := startTicks / clockTicks
	return time.Unix(bootTime+seconds, 0), nil
}

func readClockTicks() (int64, error) {
	cmd := exec.Command("getconf", "CLK_TCK")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return 0, errors.New("clk_tck empty")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid clk_tck")
	}
	return parsed, nil
}

func hasArg(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}

func flagValue(args []string, names ...string) string {
	for i, arg := range args {
		for _, name := range names {
			if arg == name && i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

func commandMatchesDeploy(command string, module string, buildID string, port int) bool {
	args := strings.Fields(command)
	if len(args) == 0 || !hasArg(args, "--deploy") {
		return false
	}
	if module != "" {
		value := flagValue(args, "-m", "--module")
		if value == "" || value != module {
			return false
		}
	}
	if buildID != "" {
		value := flagValue(args, "--build")
		if value == "" || value != buildID {
			return false
		}
	}
	if port > 0 {
		value := flagValue(args, "-p", "--port")
		if value == "" || value != strconv.Itoa(port) {
			return false
		}
	}
	return true
}

func verifyProcess(proc deployProcess) error {
	if proc.PID <= 0 {
		return errors.New("invalid pid")
	}
	if proc.Command == "" || proc.StartedUnix == 0 {
		return errors.New("missing process metadata")
	}
	startedAt, command, err := getProcessDetails(proc.PID)
	if err != nil {
		return err
	}
	if command != "" && proc.Command != command {
		if !commandMatchesDeploy(command, proc.Module, proc.BuildID, proc.Port) {
			return fmt.Errorf("process command mismatch")
		}
	}
	if !startedAt.IsZero() {
		expected := time.Unix(proc.StartedUnix, 0)
		diff := startedAt.Sub(expected)
		if diff < 0 {
			diff = -diff
		}
		if diff > processStartTolerance {
			return fmt.Errorf("process start mismatch")
		}
	}
	return nil
}

func stopManagedProcess(proc deployProcess) error {
	if err := verifyProcess(proc); err != nil {
		return err
	}
	if err := terminateProcess(proc.PID); err != nil {
		return err
	}
	return nil
}

func (api *deployAPI) stopManagedProcessByBuild(module string, buildID string) (deployProcess, bool, error) {
	proc, ok := api.readBuildProcessFile(module, buildID)
	if !ok {
		return deployProcess{}, false, nil
	}
	if !isProcessRunning(proc.PID) {
		api.clearBuildProcess(module, buildID)
		return proc, false, nil
	}
	if err := stopManagedProcess(proc); err != nil {
		return proc, true, err
	}
	api.clearBuildProcess(module, buildID)
	return proc, true, nil
}

func (api *deployAPI) stopManagedProcessesByPort(module string, port int, excludeBuild string) error {
	if port <= 0 {
		return nil
	}
	procs := api.findProcessesByPort(module, port)
	for _, proc := range procs {
		if excludeBuild != "" && proc.BuildID == excludeBuild {
			continue
		}
		if err := stopManagedProcess(proc); err != nil {
			return err
		}
		api.clearBuildProcess(module, proc.BuildID)
	}
	return nil
}

func (api *deployAPI) startManaged(module string, buildID string) error {
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		return err
	}
	if buildID == "" {
		buildID = index.Current
	}
	if buildID == "" {
		return errors.New("no build id available for start")
	}

	production := false
	if row, ok := findDeployRow(index, buildID); ok {
		production = row.Production
	}

	prevProc, hasPrev := api.readBuildProcessFile(module, buildID)
	previousPort := 0
	if hasPrev {
		previousPort = prevProc.Port
	}

	preferredPort := api.readRuntimePort(module, buildID)
	if preferredPort == 0 && previousPort > 0 {
		preferredPort = previousPort
	}
	if preferredPort == 0 && index.Port > 0 {
		preferredPort = index.Port
	}

	if _, _, err := api.stopManagedProcessByBuild(module, buildID); err != nil {
		return err
	}
	if err := api.stopManagedProcessesByPort(module, preferredPort, buildID); err != nil {
		return err
	}

	port, err := api.selectPort(module, index, preferredPort)
	if err != nil {
		return err
	}
	if port != index.Port {
		index.Port = port
		if err := saveDeployIndex(indexPath, index); err != nil {
			return err
		}
	}

	if err := api.stopManaged(module); err != nil {
		return err
	}

	binary := strings.TrimSpace(api.binaryPath)
	if binary == "" {
		return errors.New("deploy binary path is not configured")
	}

	args := []string{"start", "--deploy"}
	if production {
		args = append(args, "--production")
	}
	args = append(args,
		"-m", module,
		"--build", buildID,
		"--deploy-dir", api.root,
		"-p", strconv.Itoa(port),
	)

	cmd := exec.Command(binary, args...)
	productionValue := "0"
	if production {
		productionValue = "1"
	}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HB_DEPLOY_MODULE=%s", module),
		fmt.Sprintf("HB_DEPLOY_BUILD_ID=%s", buildID),
		fmt.Sprintf("HB_DEPLOY_PORT=%d", port),
		fmt.Sprintf("HB_DEPLOY_ROOT=%s", api.root),
		fmt.Sprintf("HB_DEPLOY_PRODUCTION=%s", productionValue),
		"HB_NO_KEYBOARD=1",
	)
	if api.workingDir != "" {
		cmd.Dir = api.workingDir
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err == nil {
		cmd.Stdin = devNull
	}

	var logFile *os.File
	if api.logsEnabled {
		logPath := api.buildLogPath(module, buildID)
		if err := os.MkdirAll(api.logDir(module), 0755); err != nil {
			if devNull != nil {
				devNull.Close()
			}
			return err
		}
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			if devNull != nil {
				devNull.Close()
			}
			return err
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	startedAt := time.Now().UTC()
	if err := cmd.Start(); err != nil {
		if devNull != nil {
			devNull.Close()
		}
		if logFile != nil {
			logFile.Close()
		}
		return err
	}
	if logFile != nil {
		logFile.Close()
	}
	if devNull != nil {
		devNull.Close()
	}
	go func() {
		_ = cmd.Wait()
	}()

	commandLine := strings.Join(append([]string{binary}, args...), " ")
	proc := deployProcess{
		Module:      module,
		PID:         cmd.Process.Pid,
		BuildID:     buildID,
		Port:        port,
		StartedAt:   startedAt.Format(time.RFC3339),
		StartedUnix: startedAt.Unix(),
		Binary:      binary,
		Command:     commandLine,
		Production:  production,
	}
	if err := api.writeProcess(module, proc); err != nil {
		_ = cmd.Process.Kill()
		logFile.Close()
		return err
	}
	return nil
}

func (api *deployAPI) stopManaged(module string) error {
	proc, ok := api.readProcess(module)
	if !ok {
		api.clearProcess(module)
		return nil
	}
	if !isProcessRunning(proc.PID) {
		api.clearProcess(module)
		api.clearBuildProcess(module, proc.BuildID)
		return nil
	}
	if err := verifyProcess(proc); err != nil {
		return err
	}
	if err := terminateProcess(proc.PID); err != nil {
		return err
	}
	api.clearProcess(module)
	api.clearBuildProcess(module, proc.BuildID)
	return nil
}

func (api *deployAPI) resolveArchivePath(module string, buildID string, index deployIndex) (string, error) {
	if row, ok := findDeployRow(index, buildID); ok {
		if row.File != "" {
			path := filepath.Clean(filepath.FromSlash(row.File))
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	incomingDir := filepath.Join(api.root, module, "incoming")
	archivesDir := filepath.Join(api.root, module, "archives")

	if path, err := findArchiveByBuildID(incomingDir, buildID); err == nil {
		return path, nil
	}
	if path, err := findArchiveByBuildID(archivesDir, buildID); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("archive not found for build id %s", buildID)
}

func (api *deployAPI) moveToArchives(module string, archivePath string) (string, error) {
	incomingDir := filepath.Clean(filepath.Join(api.root, module, "incoming"))
	cleanPath := filepath.Clean(archivePath)
	if !strings.HasPrefix(cleanPath, incomingDir+string(os.PathSeparator)) {
		return archivePath, nil
	}

	archivesDir := filepath.Join(api.root, module, "archives")
	if err := os.MkdirAll(archivesDir, 0755); err != nil {
		return "", err
	}
	destPath := filepath.Join(archivesDir, filepath.Base(cleanPath))
	if err := moveFile(cleanPath, destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func (api *deployAPI) indexPath(module string) string {
	return filepath.Join(api.root, module, deployIndexFile)
}

func (api *deployAPI) relativePath(path string) string {
	rel, err := filepath.Rel(".", path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func loadDeployIndex(path string) (deployIndex, error) {
	var index deployIndex
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index, nil
		}
		return index, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return index, nil
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return index, err
	}
	return index, nil
}

func saveDeployIndex(path string, index deployIndex) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}

func upsertDeployRow(index deployIndex, row deployIndexRow) deployIndex {
	updated := false
	for i, existing := range index.Versions {
		if existing.BuildID == row.BuildID {
			index.Versions[i] = row
			updated = true
			break
		}
	}
	if !updated {
		index.Versions = append(index.Versions, row)
	}
	return index
}

func removeDeployRow(index deployIndex, buildID string) deployIndex {
	if buildID == "" {
		return index
	}
	updated := make([]deployIndexRow, 0, len(index.Versions))
	for _, row := range index.Versions {
		if row.BuildID == buildID {
			continue
		}
		updated = append(updated, row)
	}
	index.Versions = updated
	return index
}

func findDeployRow(index deployIndex, buildID string) (deployIndexRow, bool) {
	for _, row := range index.Versions {
		if row.BuildID == buildID {
			return row, true
		}
	}
	return deployIndexRow{}, false
}

func findArchiveByBuildID(dir string, buildID string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".hra" && ext != ".zip" {
			continue
		}
		if strings.Contains(name, buildID) {
			matches = append(matches, filepath.Join(dir, name))
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no archive found in %s", dir)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple archives match build id %s", buildID)
	}
	return matches[0], nil
}

func parseLogLines(r *http.Request, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("lines"))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func readLogTail(path string, maxLines int, maxBytes int64) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", false, err
	}

	start := int64(0)
	truncated := false
	if info.Size() > maxBytes {
		start = info.Size() - maxBytes
		truncated = true
	}
	if start > 0 {
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return "", false, err
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", false, err
	}
	if start > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}

	content := string(data)
	if maxLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
			truncated = true
		}
		content = strings.Join(lines, "\n")
	}
	return strings.TrimRight(content, "\n"), truncated, nil
}

func readMetadataAndPort(path string) (map[string]string, int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	parsed := parser.ParseHyperScript(string(content))
	hyper, ok := parsed["hyperbricks"].(map[string]interface{})
	if !ok {
		return nil, 0, errors.New("missing hyperbricks config")
	}

	meta := map[string]string{
		"moduleversion": "unknown",
		"format":        "unknown",
		"built_at":      "",
		"commit":        "unknown",
		"source_hash":   "",
	}
	if rawMeta, ok := hyper["metadata"].(map[string]interface{}); ok {
		meta["moduleversion"] = getString(rawMeta, "moduleversion", meta["moduleversion"])
		meta["format"] = getString(rawMeta, "format", meta["format"])
		meta["built_at"] = getString(rawMeta, "built_at", meta["built_at"])
		meta["commit"] = getString(rawMeta, "commit", meta["commit"])
		meta["source_hash"] = getString(rawMeta, "source_hash", meta["source_hash"])
	}

	port, _ := readServerPortFromMap(hyper)

	return meta, port, nil
}

func readServerPort(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	parsed := parser.ParseHyperScript(string(content))
	hyper, ok := parsed["hyperbricks"].(map[string]interface{})
	if !ok {
		return 0, errors.New("missing hyperbricks config")
	}
	return readServerPortFromMap(hyper)
}

func readServerPortFromMap(hyper map[string]interface{}) (int, error) {
	server, ok := hyper["server"].(map[string]interface{})
	if !ok {
		return 0, nil
	}
	portVal, ok := server["port"]
	if !ok {
		return 0, nil
	}
	return asInt(portVal)
}

func (api *deployAPI) readRuntimePort(module string, buildID string) int {
	if buildID == "" {
		return 0
	}
	configPath := filepath.Join(api.root, module, "runtime", buildID, "package.hyperbricks")
	if _, err := os.Stat(configPath); err != nil {
		return 0
	}
	port, _ := readServerPort(configPath)
	return port
}

func (api *deployAPI) portReusable(module string, port int) bool {
	proc, ok := api.readProcess(module)
	if !ok {
		return false
	}
	return proc.Port == port
}

func (api *deployAPI) selectPort(module string, index deployIndex, preferredPort int) (int, error) {
	if preferredPort > 0 {
		if api.portReusable(module, preferredPort) || isPortAvailable(preferredPort) {
			return preferredPort, nil
		}
		if index.Port > 0 && index.Port != preferredPort {
			if api.portReusable(module, index.Port) || isPortAvailable(index.Port) {
				return index.Port, nil
			}
		}
		return findFreePort(preferredPort)
	}

	if index.Port > 0 {
		if api.portReusable(module, index.Port) || isPortAvailable(index.Port) {
			return index.Port, nil
		}
		return findFreePort(index.Port)
	}

	return findFreePort(api.portStart)
}

func getString(values map[string]interface{}, key string, fallback string) string {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func asInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		val := strings.TrimSpace(v)
		if val == "" {
			return 0, nil
		}
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("unsupported int type: %T", value)
	}
}

func findFreePort(start int) (int, error) {
	if start <= 0 {
		start = 8080
	}
	for port := start; port <= maxPortCandidate; port++ {
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found starting at %d", start)
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func moveFile(src string, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]interface{}{
		"error": err.Error(),
	})
}
