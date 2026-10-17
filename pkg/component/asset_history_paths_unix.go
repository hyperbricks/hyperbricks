//go:build !windows

package component

import (
	"os"
	"path/filepath"
	"runtime"
)

func assetOwnershipBaseDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		return filepath.Join(home, "Library", "Caches"), nil
	}
	return filepath.Join(home, ".cache"), nil
}
