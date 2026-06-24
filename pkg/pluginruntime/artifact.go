package pluginruntime

import (
	"fmt"
	"os"
	"path/filepath"
)

type Runtime string

const (
	RuntimeNative Runtime = "native"
	RuntimeWasm   Runtime = "wasm"
)

type Artifact struct {
	Path    string
	Runtime Runtime
}

func ResolveArtifact(pluginDir string, name string) (Artifact, error) {
	nativePath := filepath.Join(pluginDir, name+".so")
	wasmPath := filepath.Join(pluginDir, name+".wasm")

	nativeExists, err := fileExists(nativePath)
	if err != nil {
		return Artifact{}, err
	}
	wasmExists, err := fileExists(wasmPath)
	if err != nil {
		return Artifact{}, err
	}

	switch {
	case nativeExists && wasmExists:
		return Artifact{}, fmt.Errorf("plugin %s has both native and wasm artifacts in %s", name, pluginDir)
	case wasmExists:
		return Artifact{Path: wasmPath, Runtime: RuntimeWasm}, nil
	case nativeExists:
		return Artifact{Path: nativePath, Runtime: RuntimeNative}, nil
	default:
		return Artifact{}, fmt.Errorf("plugin artifact %s.so or %s.wasm not found in %s", name, name, pluginDir)
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to inspect plugin artifact %s: %v", path, err)
}
