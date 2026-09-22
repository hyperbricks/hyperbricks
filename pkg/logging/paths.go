package logging

import (
	"os"
	"path/filepath"
	"strings"
)

// ModulePath formats a filesystem path (resolved from the process working
// directory) relative to its module. External paths retain their ../ segments.
func ModulePath(module, file string) string {
	if file == "" {
		return ""
	}
	root, rootErr := filepath.Abs(module)
	absolute, err := filepath.Abs(file)
	if rootErr == nil && err == nil {
		if relative, err := filepath.Rel(root, absolute); err == nil {
			return filepath.ToSlash(relative)
		}
	}
	return filepath.ToSlash(filepath.Clean(file))
}

// ModuleText removes the module's known prefixes from embedded source locations.
// Already module-relative locations and unrelated URLs are left unchanged.
func ModuleText(module, value string) string {
	root, err := filepath.Abs(module)
	if err != nil {
		return value
	}
	prefixes := []string{root, filepath.Clean(module)}
	if cwd, err := os.Getwd(); err == nil {
		if relative, err := filepath.Rel(cwd, root); err == nil && relative != ".." && !strings.HasPrefix(relative, "../") {
			prefixes = append(prefixes, "./"+relative, relative)
		}
	}
	for _, prefix := range prefixes {
		if prefix == "." || prefix == "" {
			continue
		}
		value = strings.ReplaceAll(value, filepath.ToSlash(prefix)+"/", "")
	}
	return value
}
