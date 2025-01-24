package parser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// GetHyperScriptFiles returns a list of .hyperbricks files in the specified directory.
func GetHyperScriptFiles(baseUrl string) ([]string, error) {
	files, err := filepath.Glob(baseUrl + "/*.hyperbricks")
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	if len(files) == 0 {
		log.Println("No .hyperbricks files found in the directory.")
		return nil, nil
	}
	return files, nil
}

// GetHyperScriptContents retrieves the content of a Hyperbricks by its route (metadata key).
func (t *HyperScriptStringArray) GetHyperScriptContents(route string) (string, bool) {
	t.PreProcessedHyperScriptStoreMutex.RLock()
	defer t.PreProcessedHyperScriptStoreMutex.RUnlock()

	content, found := t.HyperBricksStore[route]
	return content, found
}

// HyperScriptStringArray is a struct that holds a map of loaded HyperBricks strings
// and provides thread-safe access to the data.
type HyperScriptStringArray struct {
	HyperBricksStore                  map[string]string
	PreProcessedHyperScriptStoreMutex sync.RWMutex
}

// GetAllHyperBricks returns a copy of all loaded Hyperbricks contents.
// This method is exported (starts with an uppercase letter) to be accessible from other packages.
func (tsa *HyperScriptStringArray) GetAllHyperBricks() map[string]string {
	tsa.PreProcessedHyperScriptStoreMutex.RLock()
	defer tsa.PreProcessedHyperScriptStoreMutex.RUnlock()

	// Create a copy of the HyperBricksStore to prevent external modifications.
	copyMap := make(map[string]string, len(tsa.HyperBricksStore))
	for key, value := range tsa.HyperBricksStore {
		copyMap[key] = value
	}

	return copyMap
}

// PreProcessHyperScriptFromFile loads a HyperBricks file's content from the specified file path
// and stores it in the HyperScriptStringArray instance.
func (t *HyperScriptStringArray) PreProcessHyperScriptFromFile(hyperbricksfile string, hyperbricksDir string, templateDir string) error {
	tempHyperBricks := make(map[string]string)

	// Read the content of the file
	data, err := os.ReadFile(hyperbricksfile)
	if err != nil {
		logging.GetLogger().Error("Error reading file ", hyperbricksfile, ":", err)
		return fmt.Errorf("read file error: %w", err)
	}

	// Extract route from the file path (filename without extension)
	route := filepath.Base(hyperbricksfile)
	route = route[:len(route)-len(filepath.Ext(route))]

	// Preprocess the HyperBricks content
	ts, err := PreprocessHyperScript(string(data), hyperbricksDir, templateDir)
	if err != nil {
		logging.GetLogger().Error("Error preprocessing HyperBricks")
		return fmt.Errorf("preprocessing error: %w", err)
	}

	// Store the preprocessed HyperBricks using the route as the key
	tempHyperBricks[route] = ts
	logging.GetLogger().Info("Loaded configuration for route: ", route)

	// Update the HyperBricksStore with the new data in a thread-safe manner
	t.PreProcessedHyperScriptStoreMutex.Lock()
	t.HyperBricksStore = tempHyperBricks
	t.PreProcessedHyperScriptStoreMutex.Unlock()
	logging.GetLogger().Info("Total configurations loaded:", len(tempHyperBricks))

	return nil
}

// PreProcessHyperBricksFromFiles loads Hyperbricks files' contents from the specified directory
// and stores them in the HyperScriptStringArray instance.
func (t *HyperScriptStringArray) PreProcessHyperBricksFromFiles(hyperbricksDir string, templateDir string) error {
	tempHyperBricks := make(map[string]string)

	orangeTrueColor := "\033[38;2;255;165;0m"
	reset := "\033[0m"

	logging.GetLogger().Info(orangeTrueColor, "Loading hyperbricks files in ", hyperbricksDir, "...", reset)
	files, err := GetHyperScriptFiles(hyperbricksDir)
	if err != nil {
		return fmt.Errorf("glob error: %v", err)
	}

	if len(files) == 0 {
		logging.GetLogger().Error("No .hyperbricks files found in the 'hyperbricks' directory.")
		return fmt.Errorf("no .hyperbricks files found in the 'hyperbricks' directory")
	}
	logging.GetLogger().Info(orangeTrueColor, "Preprocessing hyperbricks configurations...", reset)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			logging.GetLogger().Error("Error reading file:", file, "", err)
			return fmt.Errorf("read file error: %v", err)
		}

		route := filepath.Base(file)
		route = route[:len(route)-len(filepath.Ext(route))]

		ts, err := PreprocessHyperScript(string(data), hyperbricksDir, templateDir)
		if err != nil {
			logging.GetLogger().Error("Error preprocessing")
			return fmt.Errorf("preprocessing error: %s", err)
		}

		tempHyperBricks[route] = ts
		//log.Printf("Parsed ConfigObject for route '%s': %+v", route, string(data))
		logging.GetLogger().Debug("Loaded configuration for route: ", route)
	}

	t.PreProcessedHyperScriptStoreMutex.Lock()
	t.HyperBricksStore = tempHyperBricks
	t.PreProcessedHyperScriptStoreMutex.Unlock()

	logging.GetLogger().Debug("Total configurations loaded: ", len(tempHyperBricks))

	return nil
}

// func (t *HyperScriptStringArray) PreProcessAllLoadedTemplates() {
//     t.PreProcessedHyperScriptStoreMutex.RLock()
//     for route, content := range t.HyperBricksStore {
//         fmt.Printf("Slug: %s, Content: %s\n", route, content)
//     }
//     t.PreProcessedHyperScriptStoreMutex.RUnlock()
// }

// PreprocessHyperScript processes @import directives and replaces TEMPLATE tokens.
func PreprocessHyperScript(hyperBricks string, hyperbricksDir string, templateDir string) (string, error) {

	// 1. Serve static files from the ./static directory at the /static/ URL path
	processed, err := processImports(hyperBricks, fmt.Sprintf("./%s", hyperbricksDir), make(map[string]bool))
	if err != nil {
		logging.GetLogger().Error("failed to process imports: %v", err)
		return "", fmt.Errorf("failed to process imports: %w", err)
	}

	templateRegex := regexp.MustCompile(`\{\{TEMPLATE:(.*?)\}\}`)
	//templateRegex := regexp.MustCompile(`{{TEMPLATE:(\w+)}}`)
	processed = templateRegex.ReplaceAllStringFunc(processed, func(token string) string {
		matches := templateRegex.FindStringSubmatch(token)
		if len(matches) != 2 {
			return token
		}
		templateName := matches[1]
		templatePath := filepath.Join(fmt.Sprintf("./%s", templateDir), templateName)
		logging.GetLogger().Debug("process import: ", templatePath)
		content, err := os.ReadFile(templatePath)
		if err != nil {
			logging.GetLogger().Error("failed to process imports: ", err)
			return token
		}

		return fmt.Sprintf("<![%s[%s]]>", string(templateName), string(content))
	})

	rootPattern := regexp.MustCompile(`{{MODULE_PATH}}`)
	processed = rootPattern.ReplaceAllString(processed, "modules/"+commands.StartModule)

	fileRegex := regexp.MustCompile(`\{\{FILE:(.*?)\}\}`)
	//templateRegex := regexp.MustCompile(`{{TEMPLATE:(\w+)}}`)
	processed = fileRegex.ReplaceAllStringFunc(processed, func(token string) string {
		matches := fileRegex.FindStringSubmatch(token)
		if len(matches) != 2 {
			return token
		}
		templateName := matches[1]
		templatePath := templateName
		logging.GetLogger().Debug("process import: ", templatePath)
		content, err := os.ReadFile(templatePath)
		if err != nil {
			logging.GetLogger().Error("failed to process imports: ", err)
			return token
		}

		return string(content)
	})

	return processed, nil
}

// processImports recursively processes @import directives to include external HyperBricks files.
func processImports(hyperBricks, baseDir string, importedFiles map[string]bool) (string, error) {
	importRegex := regexp.MustCompile(`@import\s+['"]([^'"]+)['"]`)
	matches := importRegex.FindAllStringSubmatch(hyperBricks, -1)

	for _, match := range matches {
		if len(match) != 2 {
			logging.GetLogger().Error("Invalid @import directive: ", match[0])
			continue
		}

		importPath := match[1]
		fullImportPath := filepath.Join(baseDir, importPath)

		if importedFiles[fullImportPath] {
			logging.GetLogger().Error("Circular import detected: ", fullImportPath)
			continue
		}

		importedFiles[fullImportPath] = true

		logging.GetLogger().Debug("Importing HyperBricks file: ", fullImportPath)

		importContent, err := os.ReadFile(fullImportPath)
		if err != nil {
			logging.GetLogger().Error("Error reading imported file: ", fullImportPath, " Error: ", err)

			continue
		}

		processedImport, err := processImports(string(importContent), filepath.Dir(fullImportPath), importedFiles)
		if err != nil {
			return "", err
		}

		hyperBricks = strings.Replace(hyperBricks, match[0], processedImport, 1)
	}

	return hyperBricks, nil
}
