//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package component

import (
	"errors"
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
	file, err := openAssetLockFile(path)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("%s outputs are in use by another operation: %w", kind, err)
	}
	return func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }, nil
}

func assetLockDirectory() (string, error) { return "/tmp", nil }

func openAssetLockFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		if err == nil {
			err = fmt.Errorf("asset lock is not a regular file: %s", path)
		}
		return nil, err
	}
	return file, nil
}

func tryLockAssetTree(path string, shared bool) (func(), bool, error) {
	file, err := openAssetLockFile(path)
	if err != nil {
		return nil, false, err
	}
	mode := unix.LOCK_EX
	if shared {
		mode = unix.LOCK_SH
	}
	err = unix.Flock(int(file.Fd()), mode|unix.LOCK_NB)
	if err != nil {
		file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }, false, nil
}
