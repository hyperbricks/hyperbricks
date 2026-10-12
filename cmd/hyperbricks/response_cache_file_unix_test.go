//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func blockResponseCacheRemoval(t *testing.T, path string) func() {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	directory := filepath.Dir(path)
	if err := os.Chmod(directory, 0500); err != nil {
		t.Fatal(err)
	}
	unblock := func() {
		if err := os.Chmod(directory, 0700); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(unblock)
	return unblock
}
