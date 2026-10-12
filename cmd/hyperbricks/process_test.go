package main

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRuntimeProcessHelper(t *testing.T) {
	if os.Getenv("HB_PROCESS_TEST_HELPER") != "1" {
		return
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestRuntimeProcessLifecycle(t *testing.T) {
	running, err := processRunning(os.Getpid())
	if err != nil || !running {
		t.Fatalf("current process: running=%v err=%v", running, err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestRuntimeProcessHelper$")
	cmd.Env = append(os.Environ(), "HB_PROCESS_TEST_HELPER=1")
	configureRuntimeProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	if running, err := processRunning(cmd.Process.Pid); err != nil || !running {
		t.Fatalf("started child: running=%v err=%v", running, err)
	}
	if err := terminateProcess(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("terminated child did not exit")
	}
	if running, err := processRunning(cmd.Process.Pid); err != nil || running {
		t.Fatalf("exited child: running=%v err=%v", running, err)
	}
	if isProcessRunning(cmd.Process.Pid) {
		t.Fatal("exited child reported as running")
	}
}

func TestRuntimeProcessRejectsInvalidPID(t *testing.T) {
	for _, pid := range []int{-1, 0} {
		if _, err := processRunning(pid); err == nil {
			t.Fatalf("processRunning(%d) accepted invalid PID", pid)
		}
		if err := terminateProcess(pid); err == nil {
			t.Fatalf("terminateProcess(%d) accepted invalid PID", pid)
		}
		if isProcessRunning(pid) {
			t.Fatalf("isProcessRunning(%d) = true", pid)
		}
	}
}
