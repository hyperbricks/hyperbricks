//go:build !darwin && !linux

package main

import (
	"errors"
	"os/exec"
)

func developmentProcessesSupported() bool        { return false }
func developmentConfigureProcessGroup(*exec.Cmd) {}
func developmentSignalProcessGroup(int, bool) error {
	return errors.New("development processes are unsupported on this platform")
}
func developmentProcessGroupAlive(int) bool { return false }
