//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package component

import "fmt"

func lockEsbuildFile(path string) (func(), error) {
	return lockAssetFile(path, "esbuild")
}

func lockAssetFile(path, kind string) (func(), error) {
	return nil, fmt.Errorf("%s asset ownership locking is unsupported on this platform", kind)
}
