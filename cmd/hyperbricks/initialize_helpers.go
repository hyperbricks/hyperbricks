package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/fsnotify/fsnotify"
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

	fmt.Printf("Are you sure you want to delete directory %q? (y/n): ", dir)

	if os.Getenv("HB_NO_KEYBOARD") != "" {
		fmt.Println("non-interactive mode: deletion aborted.")
		return false
	}

	if err := keyboard.Open(); err != nil {
		fmt.Println("Failed to open keyboard input, aborting deletion.")
		return false
	}
	defer keyboard.Close()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			fmt.Printf("\nError reading input: %v\n", err)
			return false
		}

		if char == 'y' || char == 'Y' {
			fmt.Println("yes")
			return true
		}
		if char == 'n' || char == 'N' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("no")
			return false
		}
	}
}

func ensureDirectoriesExist(directories map[string]string) {
	count := 0
	//log.Printf("Checking hyperbricks.conf directories...")
	for _, dir := range directories {

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Printf("missing directory: ==>%s \n", dir)
			count++

		} else {
			// log.Printf("directory %s exists\n", dir)
		}

	}

	if count > 0 {
		log.Fatalf("Exiting, please type \"hyperbricks init\" to create required files and directories\n")
	}

	logger := logging.GetLogger()
	for key, dir := range directories {
		logger.Debugw("Checking directory", "key", key, "directory", dir)

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				logger.Fatalw("Failed to create directory", "directory", dir, "error", err)
			}
			log.Printf("Created directory ==>%s", dir)
		} else if err != nil {
			logger.Fatalw("Error checking directory", "directory", dir, "error", err)
		} else {
			logger.Debugw("Directory already exists", "directory", dir)
		}
	}

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

func watchDirectories(directories []string, reloadFunc func()) error {
	logger := logging.GetLogger()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Function to add directories recursively
	addRecursive := func(dir string) error {
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				logger.Errorw("Error accessing path during recursion", "path", path, "error", err)
				return nil
			}
			if info.IsDir() {
				err = watcher.Add(path)
				if err != nil {
					logger.Errorw("Error watching directory", "directory", path, "error", err)
					return nil
				}
				logger.Infow("Watching directory", "directory", path)
			}
			return nil
		})
	}

	for _, dir := range directories {
		err = addRecursive(dir)
		if err != nil {
			return fmt.Errorf("failed to recursively watch directory %s: %w", dir, err)
		}
	}

	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond
	debounceChan := make(chan struct{})

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				logger.Debugw("File system event detected", "event", event)
				// If a new directory is created, add it to the watcher
				if event.Op&fsnotify.Create == fsnotify.Create {
					fileInfo, err := os.Stat(event.Name)
					if err == nil && fileInfo.IsDir() {
						err = addRecursive(event.Name)
						if err != nil {
							logger.Errorw("Error adding new directory to watcher", "directory", event.Name, "error", err)
						}
					}
				}

				// Debounce configuration reload
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(debounceDuration, func() {
						debounceChan <- struct{}{}
					})
				}
			case <-debounceChan:
				logger.Infow("Debounced changes detected. Reloading configurations...")
				reloadFunc()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Errorw("File watcher error", "error", err)
			}
		}
	}()

	<-make(chan struct{}) // Keep the function running
	return nil
}

func watchSourceDirectories() {
	logger := logging.GetLogger()
	hbConfig := getHyperBricksConfiguration()
	if hbConfig.Development.Watch {
		directoriesToWatch := resolveDevelopmentWatchDirectories(hbConfig)
		go func() {
			err := watchDirectories(directoriesToWatch, PreProcessAndPopulateHyperbricksConfigurations)
			if err != nil {
				logger.Fatalw("Error setting up directory watcher", "error", err)
			}
		}()
	}
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

func applyHyperBricksConfigurations() {
	hbConfig := getHyperBricksConfiguration()
	ensureDirectoriesExist(hbConfig.Directories)
}
func setWorkingDirectory() {
	logger := logging.GetLogger()
	exeDir, err := os.Getwd()
	if err != nil {
		logger.Fatalw("Failed to evaluate os.Getwd", "error", err)
	}
	logger.Debugw("Working directory set", "directory", exeDir)
}
func PreProcessAndPopulateHyperbricksConfigurations() {
	logger := logging.GetLogger()
	err := PreProcessAndPopulateConfigs()
	if err != nil {
		logger.Errorw("Error preprocessing HyperBricks", "error", err)
		recordConfigDiagnostics([]error{preprocessErrorToComponentError(err)})
		if commands.RenderStatic {
			logger.Fatalw("Static rendering failed", "error", err)
		}
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
