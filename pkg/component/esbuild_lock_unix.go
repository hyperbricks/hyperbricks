//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package component

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func lockEsbuildFile(path string) (func(), error) {
	return lockAssetFile(path, "esbuild")
}

func lockAssetFile(path, kind string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("%s outputs are in use by another operation: %w", kind, err)
	}
	return func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }, nil
}
