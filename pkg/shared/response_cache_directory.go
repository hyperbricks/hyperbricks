package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

// LoadPackageCacheDirectories resolves the directory inputs needed by archive
// exclusion without validating unrelated runtime options or publishing template
// resources. The Boolean distinguishes a new explicit cache directory from the
// existing built-in .cache exclusion.
func LoadPackageCacheDirectories(configPath, moduleRoot string) (map[string]string, bool, error) {
	options := packageConfigYAMLOptions(moduleRoot)
	options.SkipTemplateRegistration = true
	result, err := yamlparser.ProcessPackageConfigFile(configPath, moduleRoot, options)
	if err != nil {
		return nil, false, err
	}
	hyperbricks, _ := result.Materialized["hyperbricks"].(map[string]interface{})
	rawDirectories, _ := hyperbricks["directories"].(map[string]interface{})
	rawCache, explicitCache := rawDirectories["cache"]
	directories := make(map[string]string)
	if !explicitCache {
		return directories, false, nil
	}
	cachePath, ok := rawCache.(string)
	if !ok || strings.TrimSpace(cachePath) == "" {
		return nil, false, fmt.Errorf("hyperbricks.directories.cache must resolve to a nonempty path")
	}
	for key, value := range rawDirectories {
		if path, ok := value.(string); ok {
			directories[key] = path
		}
	}
	return directories, true, nil
}

// ResolveResponseCacheDirectory applies the same private-directory boundary to
// serving and packaging. Relative paths retain their invocation-directory
// meaning; module markers are resolved by the package loader before this call.
func ResolveResponseCacheDirectory(moduleRoot string, directories map[string]string) (string, error) {
	cacheRoot := directories["cache"]
	if strings.TrimSpace(cacheRoot) == "" {
		cacheRoot = filepath.Join(moduleRoot, ".cache")
	}
	cacheRoot, err := CanonicalResponseCachePath(cacheRoot)
	if err != nil {
		return "", fmt.Errorf("resolve response cache directory: %w", err)
	}
	if info, err := os.Stat(cacheRoot); err == nil && !info.IsDir() {
		return "", fmt.Errorf("directories.cache must be a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect response cache directory: %w", err)
	}
	moduleRoot, err = CanonicalResponseCachePath(moduleRoot)
	if err != nil {
		return "", err
	}
	if ResponseCachePathContains(cacheRoot, moduleRoot) {
		return "", fmt.Errorf("directories.cache must not be the module directory or its ancestor")
	}
	defaults := defaultPackageConfig(moduleRoot).Directories
	for _, key := range []string{"static", "render", "resources", "templates", "hyperbricks", "plugins"} {
		configured := directories[key]
		if configured == "" {
			configured = defaults[key]
		}
		protected, err := CanonicalResponseCachePath(configured)
		if err != nil {
			return "", err
		}
		if ResponseCachePathContains(protected, cacheRoot) || ResponseCachePathContains(cacheRoot, protected) {
			return "", fmt.Errorf("directories.cache must not overlap directories.%s", key)
		}
	}
	return cacheRoot, nil
}

// CanonicalResponseCachePath resolves existing symlink ancestors while allowing
// the runtime-owned cache directory itself to be created lazily.
func CanonicalResponseCachePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(absolute)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(absolute) == absolute {
			return "", err
		}
		missing = append(missing, filepath.Base(absolute))
		absolute = filepath.Dir(absolute)
	}
}

func ResponseCachePathContains(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
