package main

import (
	"bytes"
	"os/exec"
)

func runCommand(binary string, workdir string, args []string) (string, error) {
	cmd := exec.Command(binary, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Stderr = &buffer
	err := cmd.Run()
	return buffer.String(), err
}
