//go:build !windows

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureRuntimeProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, errors.New("invalid pid")
	}
	err := syscall.Kill(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func signalProcess(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return errors.New("invalid pid")
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
