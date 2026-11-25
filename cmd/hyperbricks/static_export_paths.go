package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// resolveStaticExportPath also resolves existing ancestors of a destination
// which has not been created yet. Validation must precede hooks and deletion.
func resolveStaticExportPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	resolved, err = resolveStaticExportPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(abs)), nil
}

func staticExportContains(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func validateStaticExportRoots(config *shared.Config, plan staticExportPlan) error {
	// These paths are bound into asset sessions before the export tree is
	// replaced. Following an old link here would bind locks/history to its
	// target, while deletion recreates a different physical output directory.
	for _, output := range []string{plan.RenderDir, filepath.Join(plan.RenderDir, "static"), filepath.Join(plan.RenderDir, "static", "images")} {
		info, err := os.Lstat(output)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("export output binding must not be a symlink: %s", output)
		}
	}
	render, err := resolveStaticExportPath(plan.RenderDir)
	if err != nil {
		return fmt.Errorf("resolve export destination: %w", err)
	}
	module, err := resolveStaticExportPath(commands.GetModuleRoot())
	if err != nil {
		return err
	}
	if render == module || !staticExportContains(module, render) {
		return fmt.Errorf("export destination must remain inside the module after resolving symlinks")
	}
	for _, name := range []string{"static", "hyperbricks", "templates", "resources", "plugins", "cache"} {
		source := strings.TrimSpace(config.Directories[name])
		if source == "" {
			continue
		}
		resolved, err := resolveStaticExportPath(source)
		if err != nil {
			return fmt.Errorf("resolve %s source directory: %w", name, err)
		}
		if staticExportContains(render, resolved) || staticExportContains(resolved, render) {
			return fmt.Errorf("export destination overlaps %s source directory %s", name, source)
		}
	}
	return nil
}
