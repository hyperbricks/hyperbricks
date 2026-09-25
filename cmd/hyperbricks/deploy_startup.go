package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Use the same archive/runtime contract as deploy run, including an already
// extracted runtime and legacy ZIP builds. The editor's HRA download resolver
// is intentionally narrower and must not restrict which builds can start.
func prepareDeployStartupConfig(root, module, buildID string) (deployPackageConfigLocation, error) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) {
		return deployPackageConfigLocation{}, errors.New("invalid module or build_id")
	}
	moduleRoot, err := confinedDirectory(root, filepath.Join(root, module))
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeRoot := filepath.Join(moduleRoot, "runtime", buildID)
	for _, directory := range []string{filepath.Dir(runtimeRoot), runtimeRoot} {
		if err := validateExistingDirectory(moduleRoot, directory); err != nil {
			return deployPackageConfigLocation{}, err
		}
	}
	archive, _, err := commands.ResolveDeployArchive(module, root, buildID)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	if _, err := commands.EnsureRuntimeExtracted(archive, root, module, buildID); err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeRoot, err = confinedDirectory(moduleRoot, runtimeRoot)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	return deployPackageConfigLocation{
		moduleRoot: runtimeRoot,
		path:       filepath.Join(runtimeRoot, shared.PackageConfigFileName),
		scope:      "runtime",
	}, nil
}

const (
	deployStartupTimeout     = 10 * time.Second
	deployStartupOutputLimit = 4096
)

// Validate before stopping the currently running build. Use the editor's strict
// validator so control-plane flags and environment cannot hide invalid input.
func validateDeployStartupConfig(location deployPackageConfigLocation) error {
	content, _, err := readRegularConfinedFile(location.moduleRoot, location.path)
	if err != nil {
		return fmt.Errorf("read package configuration before start: %w", err)
	}
	if _, err := shared.ValidatePackageConfigBytes(content, location.moduleRoot); err != nil {
		return fmt.Errorf("cannot start module: %w; edit package.hyperbricks.yaml or rebuild this archive", err)
	}
	return nil
}

func readDeployStartupLog(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ""
	}
	if info.Size() > deployStartupOutputLimit {
		if _, err := file.Seek(-deployStartupOutputLimit, io.SeekEnd); err != nil {
			return ""
		}
	}
	tail, err := io.ReadAll(io.LimitReader(file, deployStartupOutputLimit))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, strings.ToValidUTF8(string(tail), "")))
}

func startDeployProcess(cmd *exec.Cmd, port int, logFile *os.File, logPath string) error {
	// Pass real file descriptors directly. A Go writer would create a pipe tied
	// to the control plane, killing the detached runtime on its next log write
	// after the deployment service exits. Nil preserves /dev/null when logs are off.
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return fmt.Errorf("start module process: %w", err)
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	err := waitDeployProcessReady(exited, port, deployStartupTimeout)
	if err == nil {
		return nil
	}
	// A timeout must not leave an untracked process (or its child processes).
	if errors.Is(err, errDeployStartupTimeout) {
		if runtime.GOOS == "windows" {
			_ = cmd.Process.Kill()
		} else {
			_ = signalProcess(cmd.Process.Pid, syscall.SIGKILL)
		}
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
		}
	}
	message := fmt.Sprintf("module failed to start on port %d: %v", port, err)
	if logPath != "" {
		if detail := readDeployStartupLog(logPath); detail != "" {
			message += "\n" + detail
		}
		message += "\nSee build logs: " + logPath
	} else {
		message += "\nEnable deployment logs to inspect runtime startup output."
	}
	return errors.New(message)
}

var errDeployStartupTimeout = errors.New("timed out waiting for the runtime listener")

func waitDeployProcessReady(exited <-chan error, port int, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(25 * time.Millisecond)
	defer poll.Stop()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for {
		select {
		case err := <-exited:
			if err == nil {
				return errors.New("process exited before the runtime listener became ready")
			}
			return fmt.Errorf("process exited before the runtime listener became ready: %w", err)
		case <-deadline.C:
			return errDeployStartupTimeout
		case <-poll.C:
			connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err != nil {
				continue
			}
			_ = connection.Close()
			select {
			case err := <-exited:
				return fmt.Errorf("process exited during startup: %v", err)
			default:
				return nil
			}
		}
	}
}
