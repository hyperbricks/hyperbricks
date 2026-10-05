package main

import (
	"context"
	"fmt"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
)

func startCacheControl(moduleRoot string) (func(context.Context) error, error) {
	return commands.StartCacheControl(moduleRoot, purgeConfiguredResponseCache)
}

func purgeConfiguredResponseCache(route string) (commands.CachePurgeResult, error) {
	if route != "" {
		resolved, exists := resolveRoute(route, getHyperBricksConfiguration().Server.Routing)
		if !exists {
			return commands.CachePurgeResult{}, fmt.Errorf("route %q is not configured", route)
		}
		route = resolved
	}
	result := purgeResponseCache(route)
	return commands.CachePurgeResult{MemoryEntries: result.MemoryEntries, DiskEntries: result.DiskEntries, Bytes: result.Bytes}, nil
}
