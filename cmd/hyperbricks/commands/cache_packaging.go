package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// moduleCacheExclusion resolves only the directory contract. Do not introduce
// strict validation of unrelated package fields while collecting source files.
// Bare relative paths retain their existing invocation-directory meaning.
func moduleCacheExclusion(root string) (func(string) bool, error) {
	directories := make(map[string]string)
	explicitCache := false
	configPath := filepath.Join(root, PackageConfigFileName)
	if _, err := os.Stat(configPath); err == nil {
		var err error
		directories, explicitCache, err = shared.LoadPackageCacheDirectories(configPath, root)
		if err != nil {
			return nil, fmt.Errorf("resolve cache directory for archive: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// The built-in .cache name filter is sufficient for existing packages.
	// Only the new explicit directory setting opts into overlap validation;
	// legacy packages may legitimately serve their whole module as static.
	if !explicitCache {
		return func(string) bool { return false }, nil
	}
	cacheAbsolute, err := shared.ResolveResponseCacheDirectory(root, directories)
	if err != nil {
		return nil, err
	}
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	canonicalRoot, err := shared.CanonicalResponseCachePath(root)
	if err != nil {
		return nil, err
	}
	return func(path string) bool {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return false
		}
		relative, err := filepath.Rel(rootAbsolute, absolute)
		if err != nil {
			return false
		}
		return shared.ResponseCachePathContains(cacheAbsolute, filepath.Join(canonicalRoot, relative))
	}, nil
}
