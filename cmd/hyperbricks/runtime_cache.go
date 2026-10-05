package main

import "github.com/hyperbricks/hyperbricks/pkg/shared"

// responseCacheDirectory follows the existing directories path convention:
// relative paths are relative to the invocation directory, and MODULE markers
// are resolved by the package loader. Validate existing symlink ancestors too.
func responseCacheDirectory(moduleRoot string, config *shared.Config) (string, error) {
	return shared.ResolveResponseCacheDirectory(moduleRoot, config.Directories)
}

func canonicalCachePath(path string) (string, error) {
	return shared.CanonicalResponseCachePath(path)
}

func cachePathContains(root, candidate string) bool {
	return shared.ResponseCachePathContains(root, candidate)
}
