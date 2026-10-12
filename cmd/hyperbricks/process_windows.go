package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func configureRuntimeProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

func processRunning(pid int) (bool, error) {
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) {
		return false, errors.New("invalid pid")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false, err
	}
	switch state {
	case windows.WAIT_OBJECT_0:
		return false, nil
	case uint32(windows.WAIT_TIMEOUT):
		return true, nil
	default:
		return false, fmt.Errorf("unexpected process wait result: %d", state)
	}
}

func signalProcess(pid int, sig syscall.Signal) error {
	if sig != syscall.SIGKILL && sig != syscall.SIGTERM {
		return fmt.Errorf("unsupported Windows process signal: %v", sig)
	}
	return terminateProcess(pid)
}

func terminateProcess(pid int) error {
	running, err := processRunning(pid)
	if err != nil || !running {
		return err
	}
	// Go cannot deliver Unix termination signals on Windows. Use Windows'
	// process-tree termination instead of killing only the parent executable.
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, filepath.Join(systemDirectory, "taskkill.exe"),
		"/PID", strconv.Itoa(pid), "/T", "/F").CombinedOutput()
	if err != nil {
		if running, probeErr := processRunning(pid); probeErr == nil && !running {
			return nil // The runtime may have exited before taskkill reached it.
		}
		return fmt.Errorf("terminate Windows runtime tree: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
