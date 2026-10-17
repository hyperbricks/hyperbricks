//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package component

import "fmt"

func lockEsbuildFile(path string) (func(), error) {
	return lockAssetFile(path, "esbuild")
}

func lockAssetFile(path, kind string) (func(), error) {
	return nil, fmt.Errorf("%s asset ownership locking is unsupported on this platform", kind)
}

func assetLockDirectory() (string, error) {
	return "", fmt.Errorf("asset locking is unsupported on this platform")
}
func tryLockAssetTree(path string, shared bool) (func(), bool, error) {
	return nil, false, fmt.Errorf("asset locking is unsupported on this platform")
}
