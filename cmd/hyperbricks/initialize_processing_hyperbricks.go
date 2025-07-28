package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// PreProcessAndPopulateConfigs orchestrates the preprocessing and population of configurations.
func PreProcessAndPopulateConfigs() error {
	hbConfig, logger := retrieveConfigAndLogger()
	determineDirectories(hbConfig)

	if err := loadHyperBricks(); err != nil {
		return fmt.Errorf("error loading HyperBricks: %w", err)
	}

	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	filenameToRoutes := make(map[string][]string)

	// Acquire lock if necessary for thread safety
	hyperBricksArray.PreProcessedHyperScriptStoreMutex.Lock()
	allScripts := hyperBricksArray.HyperBricksStore
	orderedRoutes := hyperBricksArray.OrderedHyperBricksRoutes
	hyperBricksArray.PreProcessedHyperScriptStoreMutex.Unlock()

	// ---- Process configs in strict order! ----
	sort.Strings(orderedRoutes)
	for _, filename := range orderedRoutes {
		content := allScripts[filename]
		config := parser.ParseHyperScript(content)
		if err := processScript(filename, config, tempConfigs, tempHyperMediasBySection, logger, filenameToRoutes); err != nil {
			logger.Warnw("Error processing script", "file", filename, "error", err)
		}
	}

	// populate configurations
	updateGlobalConfigs(tempConfigs)
	updateGlobalHyperMediasBySection(tempHyperMediasBySection)

	// linking resources to the renderers
	linkRendererResources()

	// clear cache
	clearHTMLCache()

	logger.Infow("Hyperbricks configurations loaded", "count", len(tempConfigs))

	// prepare for static rendering
	if commands.RenderStatic {
		PrepareForStaticRendering(tempConfigs)
	} else {
		// Print mapping from filename to routes
		printFilenameToRoutesMapping(filenameToRoutes)
	}

	return nil
}

// retrieveConfigAndLogger fetches the HyperBricks configuration and initializes the logger.
func retrieveConfigAndLogger() (*shared.Config, *zap.SugaredLogger) {
	hbConfig := shared.GetHyperBricksConfiguration()
	logger := logging.GetLogger()
	return hbConfig, logger
}

// determineDirectories resolves the directories for templates and HyperBricks.
func determineDirectories(hbConfig *shared.Config) core.ModuleConfiguredDirectories {
	getDirectory := func(key, defaultDir string) string {
		if dir, exists := hbConfig.Directories[key]; exists && strings.TrimSpace(dir) != "" {
			return fmt.Sprintf("./%s", dir)
		}
		return defaultDir
	}

	core.ModuleDirectories.ModulesRoot = "./modules"
	core.ModuleDirectories.ModuleDir = "modules/" + commands.StartModule

	core.ModuleDirectories.Root = "./"
	core.ModuleDirectories.RenderedDir = getDirectory("rendered", fmt.Sprintf("%s/rendered", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.TemplateDir = getDirectory("templates", fmt.Sprintf("%s/templates", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.HyperbricksDir = getDirectory("hyperbricks", fmt.Sprintf("%s/hyperbricks", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.StaticDir = getDirectory("static", fmt.Sprintf("%s/static", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.ResourcesDir = getDirectory("resources", fmt.Sprintf("%s/resources", core.ModuleDirectories.ModuleDir))

	return core.ModuleDirectories
}

// loadHyperBricks preprocesses HyperBricks from the specified directories.
func loadHyperBricks() error {
	return hyperBricksArray.PreProcessHyperBricksFromFiles()
}

// processScript parses and processes a single HyperBricks file.
// For each config object, it decodes and checks its type, ensures that the associated route is unique
// (using ensureUniqueRoute to prevent collisions with other routes), updates metadata, and
// organizes configs by section. This centralizes route management and avoids accidental overwrites
// when loading multiple configs. Additional logging and metadata assignment help with debugging
// and traceability.
func processScript(
	filename string,
	config map[string]interface{},
	tempConfigs map[string]map[string]interface{},
	tempHyperMediasBySection map[string][]composite.HyperMediaConfig,
	logger *zap.SugaredLogger,
	filenameToRoutes map[string][]string, // <-- add this
) error {
	hbConfig := getHyperBricksConfiguration()

	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		v := config[key]
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		typeValue, hasType := obj["@type"]
		if !hasType {
			continue
		}
		switch typeValue {
		case composite.FragmentConfigGetName(), composite.ApiFragmentRenderConfigGetName():
			fragmentConfig, err := decodeFragmentConfig(obj)
			if err != nil {
				logger.Warnw("Error decoding HyperMediaConfig", "error", err)
				continue
			}
			if fragmentConfig.Route == "" {
				continue
			}
			fragmentConfig.Route = ensureUniqueRoute(fragmentConfig.Route, filename, tempConfigs)
			obj["route"] = fragmentConfig.Route
			handleStaticRoute(obj, &fragmentConfig)
			hyperMediaConfig := composite.HyperMediaConfig{
				Section: fragmentConfig.Section,
				Title:   fragmentConfig.Title,
				Route:   fragmentConfig.Route,
			}
			tempHyperMediasBySection[hyperMediaConfig.Section] = append(
				tempHyperMediasBySection[hyperMediaConfig.Section],
				hyperMediaConfig,
			)
			if hyperMediaConfig.Static == "" {
				logger.Info(fmt.Sprintf("fragment (%s): [http://%s/%s] initialized", filename, shared.Location, hyperMediaConfig.Route))
			} else {
				logger.Info(fmt.Sprintf("static file: %s", hyperMediaConfig.Static))
			}
			obj["hyperbricksfile"] = filename
			obj["hyperbrickskey"] = key
			tempConfigs[fragmentConfig.Route] = obj

			// --- Map filename to route here
			filenameToRoutes[filename] = append(filenameToRoutes[filename], fragmentConfig.Route)

		case composite.HyperMediaConfigGetName():
			hyperMediaConfig, err := decodeHyperMediaConfig(obj)
			if err != nil {
				logger.Warnw("Error decoding HyperMediaConfig", "error", err)
				continue
			}
			if hyperMediaConfig.Route == "" {
				continue
			}
			hyperMediaConfig.Route = ensureUniqueRoute(hyperMediaConfig.Route, filename, tempConfigs)
			obj["route"] = hyperMediaConfig.Route
			handleStaticRoute(obj, &hyperMediaConfig)
			tempHyperMediasBySection[hyperMediaConfig.Section] = append(
				tempHyperMediasBySection[hyperMediaConfig.Section],
				hyperMediaConfig,
			)
			ips, err := getHostIPv4s()
			if err != nil {
				logging.GetLogger().Errorw("Error retrieving host IPs", "error", err)
			}
			if len(ips) == 0 {
				logging.GetLogger().Errorw("No IPv4 addresses found for the host")
			}
			shared.Location = fmt.Sprintf("%s:%d", ips[0], hbConfig.Server.Port)
			if hyperMediaConfig.Static == "" {
				logger.Info(fmt.Sprintf("route  (%s): [http://%s/%s] initialized", filename, shared.Location, hyperMediaConfig.Route))
			} else {
				logger.Info(fmt.Sprintf("static file: %s", hyperMediaConfig.Static))
			}
			obj["hyperbricksfile"] = filename
			obj["hyperbrickskey"] = key
			tempConfigs[hyperMediaConfig.Route] = obj

			// --- Map filename to route here
			filenameToRoutes[filename] = append(filenameToRoutes[filename], hyperMediaConfig.Route)

		default:
			continue
		}
	}
	return nil
}

func printFilenameToRoutesMapping(filenameToRoutes map[string][]string) {
	filenames := make([]string, 0, len(filenameToRoutes))
	for fname := range filenameToRoutes {
		filenames = append(filenames, fname)
	}
	logging.GetLogger().Info("====== route/config map ======")

	sort.Strings(filenames)
	for _, fname := range filenames {
		routes := filenameToRoutes[fname]
		logging.GetLogger().Info(fmt.Sprintf("%-24s -> %s", fname, strings.Join(routes, ", ")))

	}
	logging.GetLogger().Info("==============================")
}

func decodeHyperMediaConfig(v map[string]interface{}) (composite.HyperMediaConfig, error) {
	var hypermediaInfo composite.HyperMediaConfig
	decoder, err := createDecoder(&hypermediaInfo)
	if err != nil {
		return hypermediaInfo, err
	}
	err = decoder.Decode(v)
	return hypermediaInfo, err
}

func decodeFragmentConfig(v map[string]interface{}) (composite.FragmentConfig, error) {
	var fragmentConfig composite.FragmentConfig
	decoder, err := createDecoder(&fragmentConfig)
	if err != nil {
		return fragmentConfig, err
	}
	err = decoder.Decode(v)
	return fragmentConfig, err
}

// createDecoder creates a mapstructure decoder with the necessary hooks.
func createDecoder(result interface{}) (*mapstructure.Decoder, error) {
	combinedHook := mapstructure.ComposeDecodeHookFunc(
		typefactory.StringToSliceHookFunc(),
		typefactory.StringToIntHookFunc(),
		typefactory.StringToMapStringHookFunc(),
	)

	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		DecodeHook:       combinedHook,
		Result:           result,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	}

	return mapstructure.NewDecoder(decoderConfig)
}

// ensureUniqueRoute returns a unique route string that does not collide with any key in tempConfigs.
// If the provided original route is empty, it generates one from the filename (basename without extension).
// If the route already exists, it appends _1, _2, etc., until a unique name is found.
func ensureUniqueRoute(original, filename string, tempConfigs map[string]map[string]interface{}) string {
	route := strings.TrimSpace(original)
	if route == "" {
		route = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}

	original = route
	counter := 1
	for {
		if _, exists := tempConfigs[route]; !exists {
			break
		}
		route = fmt.Sprintf("%s_%d", original, counter)
		counter++
	}
	return route
}

// ensureUniqueEndPoint ensures that the HxEndpoint is unique within tempConfigs.
// func ensureUniqueEndPoint(originalEndpoint, filename string, tempConfigs map[string]map[string]interface{}) string {
// 	endpoint := strings.TrimSpace(originalEndpoint)
// 	if endpoint == "" {
// 		endpoint = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
// 	}

// 	originalEndpoint = endpoint
// 	counter := 1
// 	for {
// 		if _, exists := tempConfigs[endpoint]; !exists {
// 			break
// 		}
// 		endpoint = fmt.Sprintf("%s_%d", originalEndpoint, counter)
// 		counter++
// 	}
// 	return endpoint
// }

// handleStaticSlug updates the route and marks the config as static if a static route is provided.
func handleStaticRoute(obj map[string]interface{}, config interface{}) {
	if routeObj, hasStatic := obj["static"].(string); hasStatic && strings.TrimSpace(routeObj) != "" {
		route := strings.TrimSpace(routeObj)
		switch cfg := config.(type) {
		case *composite.HyperMediaConfig:
			cfg.Route = route
			cfg.IsStatic = true
		case *composite.FragmentConfig:
			cfg.Route = route
			cfg.IsStatic = true
		}
	}
}

// updateGlobalConfigs safely updates the global configs map.
func updateGlobalConfigs(tempConfigs map[string]map[string]interface{}) {
	configMutex.Lock()
	defer configMutex.Unlock()
	configs = tempConfigs
}

// updateGlobalHyperMediasBySection safely updates the global hypermediasBySection map.
func updateGlobalHyperMediasBySection(tempHyperMediasBySection map[string][]composite.HyperMediaConfig) {
	hypermediasMutex.Lock()
	defer hypermediasMutex.Unlock()
	hypermediasBySection = tempHyperMediasBySection
}

func GetGlobalHyperMediasBySection() map[string][]composite.HyperMediaConfig {
	hypermediasMutex.Lock()
	temp := hypermediasBySection // Copy the map for use outside the lock
	hypermediasMutex.Unlock()
	return temp
}

// resetHTMLCache clears the HTML cache.
func clearHTMLCache() {
	htmlCacheMutex.Lock()
	defer htmlCacheMutex.Unlock()
	htmlCache = make(map[string]CacheEntry)
}
