package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// resolveGoMaxProcs returns zero for Go-managed automatic parallelism.
// Validate before calling the runtime: GOMAXPROCS silently ignores nonpositive
// values, and mapstructure's weak integer conversion would accept booleans or
// truncate fractions. Keep the YAML value intact until this boundary.
func resolveGoMaxProcs(value any, logicalCPUs int) (int, error) {
	invalid := func() (int, error) {
		return 0, fmt.Errorf("hyperbricks.server.gomaxprocs must be auto or an integer between 1 and %d; got %v", logicalCPUs, value)
	}
	if value == nil {
		return 0, nil
	}
	var n int
	switch v := value.(type) {
	case int:
		n = v
	case string:
		v = strings.TrimSpace(v)
		if v == "auto" {
			return 0, nil
		}
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return invalid()
		}
		n = parsed
	default:
		return invalid()
	}
	if n < 1 || n > logicalCPUs {
		return invalid()
	}
	return n, nil
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
