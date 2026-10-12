package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWindowsProcessTreeHelper(t *testing.T) {
	pidFile := os.Getenv("HB_WINDOWS_TREE_PID_FILE")
	if pidFile == "" {
		return
	}
	binary, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	child := exec.Command(binary, "-test.run=^TestRuntimeProcessHelper$")
	child.Env = append(os.Environ(), "HB_PROCESS_TEST_HELPER=1")
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
		_ = child.Process.Kill()
		os.Exit(4)
	}
	_ = child.Wait()
}

func TestWindowsRuntimeTerminatesChildProcesses(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	parent := exec.Command(binary, "-test.run=^TestWindowsProcessTreeHelper$")
	parent.Env = append(os.Environ(), "HB_WINDOWS_TREE_PID_FILE="+pidFile)
	configureRuntimeProcessGroup(parent)
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = terminateProcess(parent.Process.Pid)
		_ = parent.Wait()
	})
	childPID := 0
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if childPID > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if childPID <= 0 {
		t.Fatal("runtime child did not start")
	}
	t.Cleanup(func() {
		if process, err := os.FindProcess(childPID); err == nil {
			defer process.Release()
			_ = process.Kill()
		}
	})
	if running, err := processRunning(childPID); err != nil || !running {
		t.Fatalf("runtime child: running=%v err=%v", running, err)
	}
	if err := signalProcess(parent.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		parentRunning, parentErr := processRunning(parent.Process.Pid)
		childRunning, childErr := processRunning(childPID)
		if parentErr == nil && childErr == nil && !parentRunning && !childRunning {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("runtime parent or child survived process-tree termination")
}
