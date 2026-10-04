package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eiannone/keyboard"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func validatePath(renderDir string) error {
	if strings.TrimSpace(renderDir) == "" {
		return fmt.Errorf("the path is empty")
	}

	// Absolute path to the module directory
	moduleBasePath, err := filepath.Abs(commands.GetModuleRoot())
	if err != nil {
		return fmt.Errorf("failed to resolve base path for ./modules: %w", err)
	}

	// Absolute path to the renderDir
	absRenderDir, err := filepath.Abs(renderDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for renderDir: %w", err)
	}

	// Normalize paths
	moduleBasePath = filepath.Clean(moduleBasePath)
	absRenderDir = filepath.Clean(absRenderDir)

	// Compute the relative path
	rel, err := filepath.Rel(moduleBasePath, absRenderDir)
	if err != nil {
		return fmt.Errorf("failed to determine relative path: %w", err)
	}

	// Check that it's inside the module path and not equal to it
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("renderDir must be a subdirectory of %s", commands.GetModuleRoot())
	}

	return nil
}

func confirmDeletion(dir string) bool {

	fmt.Fprintf(os.Stderr, "Delete directory %q? (y/n): ", dir)

	if os.Getenv("HB_NO_KEYBOARD") != "" {
		fmt.Fprintln(os.Stderr, "Non-interactive mode: deletion canceled.")
		return false
	}

	if err := keyboard.Open(); err != nil {
		logging.GetLogger().Warnw("Keyboard unavailable; deletion canceled", "error", err)
		return false
	}
	defer keyboard.Close()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			logging.GetLogger().Warnw("Input unavailable; deletion canceled", "error", err)
			return false
		}

		if char == 'y' || char == 'Y' {
			fmt.Fprintln(os.Stderr, "yes")
			return true
		}
		if char == 'n' || char == 'N' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Fprintln(os.Stderr, "no")
			return false
		}
	}
}

func ensureDirectoriesExist(directories map[string]string) error {
	for name, dir := range directories {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("required directory %s (%s): %w; run hyperbricks init to create missing directories", name, runtimeLogPath(dir), err)
		}
		if !info.IsDir() {
			return fmt.Errorf("required directory %s (%s) is not a directory", name, runtimeLogPath(dir))
		}
	}
	return nil
}

func makeStatic(config map[string]map[string]interface{}, renderDir string) error {

	logger := logging.GetLogger()
	for _, v := range config {
		obj := v

		renderPath := ""
		if staticPath, ok := obj["static"].(string); ok && strings.TrimSpace(staticPath) != "" {
			renderPath = strings.TrimSpace(staticPath)
		} else if routePath, ok := obj["route"].(string); ok && strings.TrimSpace(routePath) != "" {
			renderPath = strings.TrimSpace(routePath)
		}

		if renderPath != "" {
			htmlContent := renderStaticContentFromConfig(obj, renderPath, nil)

			renderPath = fmt.Sprintf("%s/%s", renderDir, renderPath)
			dir := filepath.Dir(renderPath)
			logger.Debugw("Static file path", "directory", dir, "path", renderPath)
			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				logger.Errorw("Error creating directories for path", "path", renderPath, "error", err)
				continue
			}

			err = os.WriteFile(renderPath, []byte(htmlContent), 0644)
			if err != nil {
				logger.Errorw("Error writing static file", "path", renderPath, "error", err)
				continue
			}

			logger.Infow("Rendered and saved static file", "path", renderPath)
		}
	}
	return nil
}

func resolveDevelopmentWatchDirectories(hbConfig *shared.Config) []string {
	watchDirs := hbConfig.Development.WatchDirs
	if len(watchDirs) == 0 {
		watchDirs = []string{"hyperbricks", "templates"}
	}

	seen := map[string]bool{}
	directories := make([]string, 0, len(watchDirs))
	for _, name := range watchDirs {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		dir := name
		if configured, ok := hbConfig.Directories[name]; ok && strings.TrimSpace(configured) != "" {
			dir = configured
		}
		dir = cleanWatchDirectory(dir)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		directories = append(directories, dir)
	}
	return directories
}

func cleanWatchDirectory(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	if strings.HasPrefix(dir, "./") || strings.HasPrefix(dir, "../") {
		return filepath.Clean(dir)
	}
	return filepath.Clean("./" + dir)
}

func getHyperBricksConfiguration() *shared.Config {
	return shared.GetHyperBricksConfiguration()
}

func applyHyperBricksConfigurations() error {
	return ensureDirectoriesExist(getHyperBricksConfiguration().Directories)
}
func setWorkingDirectory() error {
	logger := logging.GetLogger()
	exeDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("read working directory: %w", err)
	}
	logger.Debugw("Working directory set", "directory", runtimeLogPath(exeDir))
	return nil
}
func PreProcessAndPopulateHyperbricksConfigurations() {
	renderDiagnosticsMutex.RLock()
	generation := diagnosticsGeneration
	renderDiagnosticsMutex.RUnlock()
	logger := logging.GetLogger()
	err := PreProcessAndPopulateConfigs()
	if err != nil {
		if commands.RenderStatic {
			logger.Named("static").Fatalw("Static rendering failed", "error", err)
		}
		recordConfigDiagnosticsAtGeneration([]error{preprocessErrorToComponentError(err)}, generation)
	}
}

func preprocessErrorToComponentError(err error) shared.ComponentError {
	message := "error preprocessing HyperBricks"
	if err != nil {
		message = err.Error()
	}
	return shared.ComponentError{
		Hash:     shared.HyperScriptErrorHash(message),
		File:     "__config",
		Type:     "CONFIG",
		Path:     "__config",
		Err:      message,
		Level:    "ERROR",
		Rejected: true,
	}
}
