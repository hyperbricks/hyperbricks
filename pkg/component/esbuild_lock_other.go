//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package component

import "fmt"

func lockEsbuildFile(path string) (func(), error) {
	return nil, fmt.Errorf("esbuild asset ownership locking is unsupported on this platform")
}
