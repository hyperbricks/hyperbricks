package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const localDevBuildID = "dev"

func (api *deployLocalServer) isDevBuildID(buildID string) bool {
	return strings.EqualFold(strings.TrimSpace(buildID), localDevBuildID)
}

func (api *deployLocalServer) devBuildRow(module string) (localBuildRowResponse, bool) {
	configPath := filepath.Join(api.modulesDir, module, "package.hyperbricks")
	if _, err := os.Stat(configPath); err != nil {
		return localBuildRowResponse{}, false
	}

	meta, _, err := readMetadataAndPort(configPath)
	if err != nil {
		return localBuildRowResponse{}, false
	}

	moduleVersion := strings.TrimSpace(meta["moduleversion"])
	if moduleVersion == "" || moduleVersion == "unknown" {
		moduleVersion = "dev"
	}
	commit := strings.TrimSpace(meta["commit"])
	if commit == "unknown" {
		commit = ""
	}

	row := localBuildRow{
		BuildID:       localDevBuildID,
		ModuleVersion: moduleVersion,
		Format:        "dev",
		File:          "",
		BuiltAt:       "",
		Commit:        commit,
		SourceHash:    "",
		Production:    false,
	}
	return localBuildRowResponse{
		localBuildRow: row,
		IsDev:         true,
	}, true
}

func (api *deployLocalServer) devBuildStatus(module string) (map[string]interface{}, error) {
	configPath := filepath.Join(api.modulesDir, module, "package.hyperbricks")
	meta, preferredPort, err := readMetadataAndPort(configPath)
	if err != nil {
		return nil, err
	}

	running := false
	port := 0
	if proc, ok := api.readBuildProcessFile(module, localDevBuildID); ok {
		if isProcessRunning(proc.PID) {
			running = true
			port = proc.Port
		} else {
			api.clearBuildProcess(module, localDevBuildID)
		}
	}
	if port == 0 && preferredPort > 0 {
		port = preferredPort
	}

	moduleVersion := strings.TrimSpace(meta["moduleversion"])
	if moduleVersion == "" || moduleVersion == "unknown" {
		moduleVersion = "dev"
	}
	commit := strings.TrimSpace(meta["commit"])
	if commit == "unknown" {
		commit = ""
	}

	return map[string]interface{}{
		"module":        module,
		"build_id":      localDevBuildID,
		"running":       running,
		"port":          port,
		"moduleversion": moduleVersion,
		"commit":        commit,
		"built_at":      "",
		"source_hash":   "",
		"format":        "dev",
		"production":    false,
		"is_dev":        true,
	}, nil
}

func (api *deployLocalServer) handleModuleActivate(w http.ResponseWriter, r *http.Request, module string) {
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

	if api.isDevBuildID(buildID) {
		if err := api.startLocalDev(module); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		if err := api.startLocalBuild(module, buildID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":    module,
		"build_id":  buildID,
		"activated": true,
	})
}

func (api *deployLocalServer) handleModuleRestart(w http.ResponseWriter, module string) {
	buildID := ""
	if proc, ok := api.readProcessFile(api.pidPath(module)); ok {
		buildID = strings.TrimSpace(proc.BuildID)
	}
	if buildID == "" {
		indexPath := api.indexPath(module)
		index, err := loadLocalBuildIndex(indexPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		buildID = strings.TrimSpace(index.Current)
	}
	if buildID == "" {
		writeError(w, http.StatusBadRequest, errors.New("no build available to restart"))
		return
	}

	if api.isDevBuildID(buildID) {
		if err := api.startLocalDev(module); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		if err := api.startLocalBuild(module, buildID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":   module,
		"build_id": buildID,
		"restart":  true,
	})
}

func (api *deployLocalServer) handleModuleStop(w http.ResponseWriter, module string) {
	buildID, err := api.stopLocalModule(module)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":   module,
		"build_id": buildID,
		"stop":     true,
	})
}

func (api *deployLocalServer) handleBuildLogs(w http.ResponseWriter, r *http.Request, module string, buildID string) {
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
		"log_path":   logPath,
		"log_output": content,
	})
}

func (api *deployLocalServer) pidPath(module string) string {
	return filepath.Join(api.buildRoot, module, "hyperbricks.pid")
}

func (api *deployLocalServer) processDir(module string) string {
	return filepath.Join(api.buildRoot, module, "processes")
}

func (api *deployLocalServer) buildPidPath(module string, buildID string) string {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return ""
	}
	return filepath.Join(api.processDir(module), buildID+".json")
}

func (api *deployLocalServer) logDir(module string) string {
	return filepath.Join(api.buildRoot, module, "logs")
}

func (api *deployLocalServer) buildLogPath(module string, buildID string) string {
	return filepath.Join(api.logDir(module), buildID+".log")
}

func (api *deployLocalServer) moduleLogPath(module string) string {
	return filepath.Join(api.buildRoot, module, "hyperbricks.log")
}

func (api *deployLocalServer) readProcessFile(path string) (deployProcess, bool) {
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

func (api *deployLocalServer) readProcess(module string) (deployProcess, bool) {
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

func (api *deployLocalServer) readBuildProcessFile(module string, buildID string) (deployProcess, bool) {
	path := api.buildPidPath(module, buildID)
	if path == "" {
		return deployProcess{}, false
	}
	return api.readProcessFile(path)
}

func (api *deployLocalServer) readBuildProcess(module string, buildID string) (deployProcess, bool) {
	proc, ok := api.readBuildProcessFile(module, buildID)
	if !ok {
		return deployProcess{}, false
	}
	if !isProcessRunning(proc.PID) {
		return deployProcess{}, false
	}
	return proc, true
}

func (api *deployLocalServer) writeProcessFile(path string, proc deployProcess) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(proc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (api *deployLocalServer) writeProcess(module string, proc deployProcess) error {
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

func (api *deployLocalServer) clearProcess(module string) {
	path := api.pidPath(module)
	_ = os.Remove(path)
}

func (api *deployLocalServer) clearBuildProcess(module string, buildID string) {
	path := api.buildPidPath(module, buildID)
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

func (api *deployLocalServer) findProcessesByPort(module string, port int) []deployProcess {
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

func commandMatchesStart(command string, module string, port int) bool {
	args := strings.Fields(command)
	if len(args) == 0 || !hasArg(args, "start") {
		return false
	}
	if hasArg(args, "--deploy") {
		return false
	}
	if module != "" {
		value := flagValue(args, "-m", "--module")
		if value == "" || value != module {
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

func verifyLocalProcess(proc deployProcess) error {
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
		if !commandMatchesDeploy(command, proc.Module, proc.BuildID, proc.Port) && !commandMatchesStart(command, proc.Module, proc.Port) {
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

func (api *deployLocalServer) stopManagedProcess(proc deployProcess) error {
	if err := verifyLocalProcess(proc); err != nil {
		return err
	}
	if err := terminateProcess(proc.PID); err != nil {
		return err
	}
	return nil
}

func (api *deployLocalServer) stopManagedProcessByBuild(module string, buildID string) (deployProcess, bool, error) {
	proc, ok := api.readBuildProcessFile(module, buildID)
	if !ok {
		return deployProcess{}, false, nil
	}
	if !isProcessRunning(proc.PID) {
		api.clearBuildProcess(module, buildID)
		return proc, false, nil
	}
	if err := api.stopManagedProcess(proc); err != nil {
		return proc, true, err
	}
	api.clearBuildProcess(module, buildID)
	return proc, true, nil
}

func (api *deployLocalServer) stopManagedProcessesByPort(module string, port int, excludeBuild string) error {
	if port <= 0 {
		return nil
	}
	procs := api.findProcessesByPort(module, port)
	for _, proc := range procs {
		if excludeBuild != "" && proc.BuildID == excludeBuild {
			continue
		}
		if err := api.stopManagedProcess(proc); err != nil {
			return err
		}
		api.clearBuildProcess(module, proc.BuildID)
	}
	return nil
}

func (api *deployLocalServer) stopLocalModule(module string) (string, error) {
	proc, ok := api.readProcessFile(api.pidPath(module))
	if !ok {
		api.clearProcess(module)
		return "", nil
	}
	buildID := strings.TrimSpace(proc.BuildID)
	if !isProcessRunning(proc.PID) {
		api.clearProcess(module)
		api.clearBuildProcess(module, buildID)
		return buildID, nil
	}
	if err := api.stopManagedProcess(proc); err != nil {
		return buildID, err
	}
	api.clearProcess(module)
	api.clearBuildProcess(module, buildID)
	return buildID, nil
}

func (api *deployLocalServer) readRuntimePort(module string, buildID string) int {
	if buildID == "" {
		return 0
	}
	configPath := filepath.Join(api.buildRoot, module, "runtime", buildID, "package.hyperbricks")
	if _, err := os.Stat(configPath); err != nil {
		return 0
	}
	port, _ := readServerPort(configPath)
	return port
}

func (api *deployLocalServer) portReusable(module string, port int) bool {
	proc, ok := api.readProcess(module)
	if !ok {
		return false
	}
	return proc.Port == port
}

func (api *deployLocalServer) selectPort(module string, index localBuildIndex, preferredPort int) (int, error) {
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

func (api *deployLocalServer) selectPortForDev(module string, preferredPort int) (int, error) {
	if preferredPort > 0 {
		if api.portReusable(module, preferredPort) || isPortAvailable(preferredPort) {
			return preferredPort, nil
		}
		return findFreePort(preferredPort)
	}
	return findFreePort(api.portStart)
}

func (api *deployLocalServer) startLocalBuild(module string, buildID string) error {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return errors.New("build_id is required")
	}
	if strings.Contains(buildID, "..") || strings.Contains(buildID, "/") || strings.Contains(buildID, "\\") {
		return errors.New("invalid build_id")
	}

	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		return err
	}

	row, ok := findLocalRow(index, buildID)
	if !ok {
		return errors.New("build_id not found")
	}

	production := row.Production

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
	if _, err := api.stopLocalModule(module); err != nil {
		return err
	}

	port, err := api.selectPort(module, index, preferredPort)
	if err != nil {
		return err
	}
	if port != index.Port || index.Current != buildID {
		index.Port = port
		index.Current = buildID
		if err := saveLocalBuildIndex(indexPath, index); err != nil {
			return err
		}
	}

	args := []string{"start", "--deploy"}
	if production {
		args = append(args, "--production")
	}
	args = append(args,
		"-m", module,
		"--build", buildID,
		"--deploy-dir", api.buildRoot,
		"-p", strconv.Itoa(port),
	)

	return api.startProcess(module, buildID, port, production, args)
}

func (api *deployLocalServer) startLocalDev(module string) error {
	configPath := filepath.Join(api.modulesDir, module, "package.hyperbricks")
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("module config not found: %s", configPath)
	}

	preferredPort, _ := readServerPort(configPath)
	prevProc, hasPrev := api.readBuildProcessFile(module, localDevBuildID)
	if preferredPort == 0 && hasPrev {
		preferredPort = prevProc.Port
	}

	if _, _, err := api.stopManagedProcessByBuild(module, localDevBuildID); err != nil {
		return err
	}
	if err := api.stopManagedProcessesByPort(module, preferredPort, localDevBuildID); err != nil {
		return err
	}
	if _, err := api.stopLocalModule(module); err != nil {
		return err
	}

	port, err := api.selectPortForDev(module, preferredPort)
	if err != nil {
		return err
	}

	args := []string{"start", "-m", module, "-p", strconv.Itoa(port)}
	return api.startProcess(module, localDevBuildID, port, false, args)
}

func (api *deployLocalServer) startProcess(module string, buildID string, port int, production bool, args []string) error {
	binary := strings.TrimSpace(api.binaryPath)
	if binary == "" {
		return errors.New("deploy binary path is not configured")
	}

	cmd := exec.Command(binary, args...)
	productionValue := "0"
	if production {
		productionValue = "1"
	}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HB_DEPLOY_MODULE=%s", module),
		fmt.Sprintf("HB_DEPLOY_BUILD_ID=%s", buildID),
		fmt.Sprintf("HB_DEPLOY_PORT=%d", port),
		fmt.Sprintf("HB_DEPLOY_ROOT=%s", api.buildRoot),
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
		if logFile != nil {
			logFile.Close()
		}
		return err
	}
	return nil
}
