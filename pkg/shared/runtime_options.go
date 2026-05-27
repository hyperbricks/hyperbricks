package shared

import (
	"path/filepath"
	"strings"
	"sync"
)

type RuntimeOptions struct {
	ModuleRoot string

	Port         int
	PortOverride bool

	Production bool

	RuntimeGatewayEnabled    bool
	RuntimeGatewayDomain     string
	RuntimeGatewayHostSuffix string
	RuntimeGatewayResolver   string
}

var runtimeOptionsState struct {
	sync.RWMutex
	options RuntimeOptions
}

func SetRuntimeOptions(options RuntimeOptions) {
	runtimeOptionsState.Lock()
	runtimeOptionsState.options = options
	runtimeOptionsState.Unlock()
}

func GetRuntimeOptions() RuntimeOptions {
	runtimeOptionsState.RLock()
	defer runtimeOptionsState.RUnlock()
	return runtimeOptionsState.options
}

func runtimeModuleRoot(options RuntimeOptions) string {
	moduleRoot := strings.TrimSpace(options.ModuleRoot)
	if moduleRoot == "" {
		moduleRoot = filepath.Join("modules", "default")
	}
	return filepath.ToSlash(filepath.Clean(moduleRoot))
}
