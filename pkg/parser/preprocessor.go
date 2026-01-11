package parser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// GetHyperScriptFiles returns a sorted list of .hyperbricks files in the specified directory.
func GetHyperScriptFiles(baseUrl string) ([]string, error) {
	files, err := filepath.Glob(baseUrl + "/*.hyperbricks")
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	if len(files) == 0 {
		log.Println("No .hyperbricks files found in the directory.")
		return nil, nil
	}

	sort.Strings(files) // Apply strict (lexicographical) order

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
	OrderedHyperBricksRoutes          []string
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

func (t *HyperScriptStringArray) PreProcessHyperBricksFromFiles() error {
	tempHyperBricks := make(map[string]string)
	orderedRoutes := []string{} // <-- stores order

	orangeTrueColor := "\033[38;2;255;165;0m"
	reset := "\033[0m"

	logging.GetLogger().Info(orangeTrueColor, "Loading hyperbricks files in ", core.ModuleDirectories.HyperbricksDir, "...", reset)
	files, err := GetHyperScriptFiles(core.ModuleDirectories.HyperbricksDir)
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
		uncommented := StripComments(string(data))
		ts, err := PreprocessHyperScript(uncommented)
		if err != nil {
			logging.GetLogger().Error("Error preprocessing")
			return fmt.Errorf("preprocessing error: %s", err)
		}

		tempHyperBricks[route] = ts
		orderedRoutes = append(orderedRoutes, route) // <--- record the order
		logging.GetLogger().Debug("Loaded configuration for route: ", route)
	}

	// store both the map and the ordered slice
	t.PreProcessedHyperScriptStoreMutex.Lock()
	t.HyperBricksStore = tempHyperBricks
	t.OrderedHyperBricksRoutes = orderedRoutes // <--- store order!
	t.PreProcessedHyperScriptStoreMutex.Unlock()

	logging.GetLogger().Debug("Total configurations loaded: ", len(tempHyperBricks))

	// Example: process in strict order
	for _, route := range orderedRoutes {
		ts := tempHyperBricks[route]
		logging.GetLogger().Info("Processing: ", route, ".hyperbricks")
		// Do your real processing here
		_ = ts // replace with real usage
	}

	return nil
}

// PreprocessHyperScript processes @import directives and replaces TEMPLATE tokens.
func PreprocessHyperScript(hyperBricks string) (string, error) {
	hyperbricksDir := core.ModuleDirectories.HyperbricksDir
	templateDir := core.ModuleDirectories.TemplateDir
	workingDir := core.ModuleDirectories.Root

	processed, err := processImports(hyperBricks, fmt.Sprintf("%s%s", workingDir, hyperbricksDir), make(map[string]bool))
	if err != nil {
		logging.GetLogger().Error("failed to process imports: %v", err)
		return "", fmt.Errorf("failed to process imports: %w", err)
	}

	// ===============
	// CACHE MARKERS
	// ===============
	templateRegex := regexp.MustCompile(`\{\{TEMPLATE:(.*?)\}\}`)
	processed = templateRegex.ReplaceAllStringFunc(processed, func(token string) string {
		matches := templateRegex.FindStringSubmatch(token)

		if len(matches) != 2 {
			return token
		}

		templateName := matches[1]
		templatePath := filepath.Join(fmt.Sprintf("%s%s", workingDir, templateDir), templateName)
		logging.GetLogger().Debug("process import: ", templatePath)
		content, err := os.ReadFile(templatePath)
		if err != nil {
			logging.GetLogger().Error("failed to process imports: ", err)
			return token
		}
		return fmt.Sprintf("<![%s[%s]]>", string(templateName), string(content))
	})

	processed = replaceFileTokens(processed)

	// ===============
	// PATH MARKERS
	// ===============
	processed = applyPathMarkers(processed)

	processed, err = processMacroBlocks(processed)
	if err != nil {
		logging.GetLogger().Error("failed to process @macro blocks: %v", err)
		return "", fmt.Errorf("failed to process macro blocks: %w", err)
	}

	return processed, nil
}

func replaceFileTokens(input string) string {
	const marker = "{{FILE:"
	var builder strings.Builder
	pos := 0

	for {
		idx := strings.Index(input[pos:], marker)
		if idx == -1 {
			builder.WriteString(input[pos:])
			break
		}
		idx += pos
		builder.WriteString(input[pos:idx])

		end, path, token, ok := parseFileToken(input, idx)
		if !ok {
			builder.WriteString(input[idx:])
			break
		}

		templatePath := applyPathMarkers(strings.TrimSpace(path))
		logging.GetLogger().Debug("process import: ", templatePath)
		content, err := os.ReadFile(templatePath)
		if err != nil {
			logging.GetLogger().Error("failed to process imports: ", err)
			builder.WriteString(token)
		} else {
			builder.WriteString(" <<[" + string(content) + " ]>>")
		}

		pos = end
	}

	return builder.String()
}

func parseFileToken(input string, start int) (int, string, string, bool) {
	const marker = "{{FILE:"
	if start < 0 || start >= len(input) || !strings.HasPrefix(input[start:], marker) {
		return 0, "", "", false
	}

	i := start + len(marker)
	depth := 1
	for i < len(input) {
		if strings.HasPrefix(input[i:], "{{") {
			depth++
			i += 2
			continue
		}
		if strings.HasPrefix(input[i:], "}}") {
			depth--
			i += 2
			if depth == 0 {
				path := input[start+len(marker) : i-2]
				token := input[start:i]
				return i, path, token, true
			}
			continue
		}
		i++
	}

	return 0, "", "", false
}

func applyPathMarkers(input string) string {
	moduleRootDirPattern := regexp.MustCompile(`{{MODULE_ROOT}}`)
	input = moduleRootDirPattern.ReplaceAllString(input, core.ModuleDirectories.ModulesRoot)

	rootDirPattern := regexp.MustCompile(`{{ROOT}}`)
	input = rootDirPattern.ReplaceAllString(input, core.ModuleDirectories.Root)

	moduleDirPattern := regexp.MustCompile(`{{MODULE}}`)
	input = moduleDirPattern.ReplaceAllString(input, core.ModuleDirectories.ModuleDir)

	rootPattern := regexp.MustCompile(`{{RESOURCES}}`)
	input = rootPattern.ReplaceAllString(input, core.ModuleDirectories.ResourcesDir)

	templatePattern := regexp.MustCompile(`{{TEMPLATES}}`)
	input = templatePattern.ReplaceAllString(input, core.ModuleDirectories.TemplateDir)

	staticPattern := regexp.MustCompile(`{{STATIC}}`)
	input = staticPattern.ReplaceAllString(input, core.ModuleDirectories.StaticDir)

	hyperBricksPattern := regexp.MustCompile(`{{HYPERBRICKS}}`)
	input = hyperBricksPattern.ReplaceAllString(input, core.ModuleDirectories.HyperbricksDir)

	return input
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

// processMacroBlocks replaces all @macro blocks in the HyperBricks config with the expanded output.
// Now supports <<<[ ... ]>>> blocks and {{{.var}}} macro replacements.
func processMacroBlocks(input string) (string, error) {
	// Regex for @macro as (..){..} = <<<[ ... ]>>>
	macroPattern := regexp.MustCompile(`(?s)@macro\s+as\s*\(([^)]+)\)\s*{([\s\S]+?)}\s*=\s*<<<\[(.*?)\]>>>`)
	matches := macroPattern.FindAllStringSubmatch(input, -1)

	if len(matches) == 0 {
		return input, nil
	}

	result := input
	for _, match := range matches {
		fullMatch := match[0]
		varNames := strings.Split(match[1], ",")
		for i := range varNames {
			varNames[i] = strings.TrimSpace(varNames[i])
		}
		dataLines := strings.Split(strings.TrimSpace(match[2]), "\n")
		tmplBlock := match[3]
		fmt.Printf("Template block: >>>\n%s\n<<<\n", tmplBlock)
		var buf strings.Builder

		// For each data line
		for _, line := range dataLines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue // Skip empty or comment lines!
			}
			fields := strings.Split(line, "|")
			row := map[string]string{}
			for i, varName := range varNames {
				if i < len(fields) {
					row[varName] = strings.TrimSpace(fields[i])
				} else {
					row[varName] = ""
				}
			}
			// Replace {{{.var}}} with row[var]
			rendered := replaceTripleBraces(tmplBlock, row)
			buf.WriteString(rendered)
			if !strings.HasSuffix(rendered, "\n") {
				buf.WriteString("\n")
			}
		}

		// Replace the whole macro block with the expanded output
		result = strings.Replace(result, fullMatch, buf.String(), 1)
		fmt.Printf("result block:%s", buf.String())
	}

	return result, nil
}

// replaceTripleBraces replaces all {{{.var}}} in the template with their values from row.
func replaceTripleBraces(template string, row map[string]string) string {
	re := regexp.MustCompile(`\{\{\{\.([a-zA-Z0-9_]+)\}\}\}`)
	return re.ReplaceAllStringFunc(template, func(m string) string {
		key := re.FindStringSubmatch(m)[1]
		if val, ok := row[key]; ok {
			return val
		}
		return ""
	})
}
