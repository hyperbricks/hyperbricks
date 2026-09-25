package main

import (
	"runtime"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// resolveGoMaxProcs returns zero for Go-managed automatic parallelism.
// Validate before calling the runtime: GOMAXPROCS silently ignores nonpositive
// values, and mapstructure's weak integer conversion would accept booleans or
// truncate fractions. Keep the YAML value intact until this boundary.
func resolveGoMaxProcs(value any, logicalCPUs int) (int, error) {
	return shared.ResolveGoMaxProcs(value, logicalCPUs)
}

func configureGoMaxProcs(value any) error {
	n, err := resolveGoMaxProcs(value, runtime.NumCPU())
	if err != nil {
		return err
	}
	if n == 0 {
		// Explicitly restore Go's container/CPU-aware default and periodic updates,
		// including when GOMAXPROCS or GODEBUG disabled those defaults at launch.
		runtime.SetDefaultGOMAXPROCS()
	} else {
		runtime.GOMAXPROCS(n)
	}
	return nil
}
