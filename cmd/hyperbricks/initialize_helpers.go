package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// validatePath ensures that the given path is within the ./modules directory.
func validatePath(renderDir string) error {
	if strings.TrimSpace(renderDir) == "" {
		return fmt.Errorf("the path is empty")
	}

	// Get the absolute path of ./modules
	allowedBasePath, err := filepath.Abs("./modules")
	if err != nil {
		return errors.New("failed to resolve base path for ./modules")
	}

	// Get the absolute path of the renderDir
	absRenderDir, err := filepath.Abs(renderDir)
	if err != nil {
		return errors.New("failed to resolve absolute path for renderDir")
	}

	// Ensure the renderDir is within the allowed base path
	if !strings.HasPrefix(absRenderDir, allowedBasePath) {
		return errors.New("renderDir is outside the allowed ./modules directory")
	}

	return nil
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
	defaultHyperbricks := `# index page
page = <HYPERMEDIA>
page.route = index
page.10 = TEXT
page.10.value = HELLO WORLD!
`
	for key, dir := range directories {
		logger.Debugw("Checking directory", "key", key, "directory", dir)

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				logger.Fatalw("Failed to create directory", "directory", dir, "error", err)
			}
			log.Printf("Created directory ==>%s", dir)

			if key == "hyperbricks" {
				configFilePath := filepath.Join(dir, "test_index.hyperbricks")
				if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
					err := os.WriteFile(configFilePath, []byte(defaultHyperbricks), 0644)
					if err != nil {
						logger.Fatalw("Failed to create configuration file", "file", configFilePath, "error", err)
					}
					logger.Infow("Configuration file created", "file", configFilePath)
				}
			}
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

		renderPath, hasStatic := obj["route"].(string)
		if hasStatic && strings.TrimSpace(renderPath) != "" {
			htmlContent := fmt.Sprintf("%v", v)
			if v["route"] != "" {
				htmlContent = renderStaticContent(v["route"].(string), nil)
			}

			renderPath = fmt.Sprintf("%s/%s", renderDir, renderPath)
			dir := filepath.Dir(renderPath)
			logger.Debugw("Static file path", "directory", dir, "path", renderPath)
			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				logger.Errorw("Error creating directories for path", "path", renderPath, "error", err)
				continue
			}

			err = os.WriteFile(renderPath+".html", []byte(htmlContent), 0644)
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
		templateDir := "./templates"
		if tbtemplates, ok := hbConfig.Directories["templates"]; ok {
			templateDir = fmt.Sprintf("./%s", tbtemplates)
		}

		hyperbricksDir := "./hyperbricks"
		if tbhyperbricksDir, ok := hbConfig.Directories["hyperbricks"]; ok {
			hyperbricksDir = fmt.Sprintf("./%s", tbhyperbricksDir)
		}

		directoriesToWatch := []string{hyperbricksDir, templateDir}
		go func() {
			err := watchDirectories(directoriesToWatch, PreProcessAndPopulateHyperbricksConfigurations)
			if err != nil {
				logger.Fatalw("Error setting up directory watcher", "error", err)
			}
		}()
	}
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
		logger.Fatalw("Error preprocessing Hyperbrickss", "error", err)
	}
}
