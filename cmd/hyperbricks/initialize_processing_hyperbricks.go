package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/internal/typefactory"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// PreProcessAndPopulateConfigs orchestrates the preprocessing and population of configurations.
func PreProcessAndPopulateConfigs() error {
	hbConfig, logger := retrieveConfigAndLogger()
	templateDir, hyperBricksDir := determineDirectories(hbConfig)

	if err := loadHyperBricks(hyperBricksDir, templateDir); err != nil {
		return fmt.Errorf("error loading HyperBricks: %w", err)
	}

	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	allScripts := hyperBricksArray.GetAllHyperBricks()

	for filename, content := range allScripts {
		config := parser.ParseHyperScript(content)
		if err := processScript(filename, config, tempConfigs, tempHyperMediasBySection, logger); err != nil {
			logger.Warnw("Error processing script", "file", filename, "error", err)
		}
	}

	updateGlobalConfigs(tempConfigs)
	updateGlobalHyperMediasBySection(tempHyperMediasBySection)

	PrepareForStaticRendering()
	resetHTMLCache()

	logger.Infow("Hyperbricks configurations loaded", "count", len(configs))
	return nil
}

// retrieveConfigAndLogger fetches the HyperBricks configuration and initializes the logger.
func retrieveConfigAndLogger() (*shared.Config, *zap.SugaredLogger) {
	hbConfig := shared.GetHyperBricksConfiguration()
	logger := logging.GetLogger()
	return hbConfig, logger
}

// determineDirectories resolves the directories for templates and HyperBricks.
func determineDirectories(hbConfig *shared.Config) (string, string) {
	getDirectory := func(key, defaultDir string) string {
		if dir, exists := hbConfig.Directories[key]; exists && strings.TrimSpace(dir) != "" {
			return fmt.Sprintf("./%s", dir)
		}
		return defaultDir
	}

	templateDir := getDirectory("templates", "./templates")
	hyperBricksDir := getDirectory("hyperbricks", "./hyperbricks")
	return templateDir, hyperBricksDir
}

// loadHyperBricks preprocesses HyperBricks from the specified directories.
func loadHyperBricks(hyperBricksDir, templateDir string) error {
	return hyperBricksArray.PreProcessHyperBricksFromFiles(hyperBricksDir, templateDir)
}

// processScript parses and processes a single HyperBricks file.
func processScript(filename string, config map[string]interface{},
	tempConfigs map[string]map[string]interface{},
	tempHyperMediasBySection map[string][]composite.HyperMediaConfig,
	logger *zap.SugaredLogger) error {

	hbConfig := getHyperBricksConfiguration()
	for key, v := range config {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue // Skip if the structure is unexpected
		}

		typeValue, hasType := obj["@type"]
		if !hasType {
			continue // Skip configurations without a type
		}

		switch typeValue {

		case composite.FragmentConfigGetName():
			fragmentConfig, err := decodeFragmentConfig(v.(map[string]interface{}))
			if err != nil {
				logger.Warnw("Error decoding HyperMediaConfig", "error", err)
				continue
			}

			fragmentConfig.Route = ensureUniqueRoute(fragmentConfig.Route, filename, tempConfigs)
			handleStaticRoute(obj, &fragmentConfig)

			// Add metadata and store in tempConfigs
			obj["file"] = filename
			obj["key"] = key
			tempConfigs[fragmentConfig.Route] = obj

		case composite.HyperMediaConfigGetName():
			hypermediaConfig, err := decodeHyperMediaConfig(v.(map[string]interface{}))
			if err != nil {
				logger.Warnw("Error decoding HyperMediaConfig", "error", err)
				continue
			}

			hypermediaConfig.Route = ensureUniqueRoute(hypermediaConfig.Route, filename, tempConfigs)
			handleStaticRoute(obj, &hypermediaConfig)
			blueTrueColor := "\033[38;2;0;0;255m"
			//orangeTrueColor := "\033[38;2;255;165;0m"
			reset := "\033[0m"
			tempHyperMediasBySection[hypermediaConfig.Section] = append(tempHyperMediasBySection[hypermediaConfig.Section], hypermediaConfig)

			ips, err := getHostIPv4s()
			if err != nil {
				logging.GetLogger().Errorw("Error retrieving host IPs", "error", err)

			}
			if len(ips) == 0 {
				logging.GetLogger().Errorw("No IPv4 addresses found for the host")

			}
			location := fmt.Sprintf("%s:%d", ips[0], hbConfig.Server.Port)
			logger.Info(blueTrueColor, fmt.Sprintf("route: [http://%s/%s] initialized:", location, hypermediaConfig.Route), reset)
			//
			// Add metadata and store in tempConfigs
			obj["file"] = filename
			obj["key"] = key
			tempConfigs[hypermediaConfig.Route] = obj

		case composite.HxApiConfigGetName():
			hxRouteConfig, err := decodeHxApiConfig(v.(map[string]interface{}))
			if err != nil {
				logger.Warnw("Error decoding HxEndPointConfig", "error", err)
				continue
			}

			// this does not HAVE to be unique because it overlaps with POST/GET etc ...
			hxRouteConfig.HxRoute = ensureUniqueEndPoint(hxRouteConfig.HxRoute, filename, tempConfigs)
			handleStaticRoute(obj, &hxRouteConfig)

			// Add metadata and store in tempConfigs
			obj["@file"] = filename
			obj["@key"] = key
			tempConfigs[hxRouteConfig.HxRoute] = obj

		default:
			continue // Handle other types if necessary
		}
	}
	return nil
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

func decodeHxApiConfig(v map[string]interface{}) (composite.HxApiConfig, error) {
	var hxApiConfig composite.HxApiConfig
	decoder, err := createDecoder(&hxApiConfig)
	if err != nil {
		return hxApiConfig, err
	}
	err = decoder.Decode(v)
	return hxApiConfig, err
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

// ensureUniqueSlug ensures that the route is unique within tempConfigs.
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
func ensureUniqueEndPoint(originalEndpoint, filename string, tempConfigs map[string]map[string]interface{}) string {
	endpoint := strings.TrimSpace(originalEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}

	originalEndpoint = endpoint
	counter := 1
	for {
		if _, exists := tempConfigs[endpoint]; !exists {
			break
		}
		endpoint = fmt.Sprintf("%s_%d", originalEndpoint, counter)
		counter++
	}
	return endpoint
}

// handleStaticSlug updates the route and marks the config as static if a static route is provided.
func handleStaticRoute(obj map[string]interface{}, config interface{}) {
	if routeObj, hasStatic := obj["static"].(string); hasStatic && strings.TrimSpace(routeObj) != "" {
		route := strings.TrimSpace(routeObj)
		switch cfg := config.(type) {
		case *composite.HyperMediaConfig:
			cfg.Route = route
			cfg.IsStatic = true
		case *composite.HxApiConfig:
			cfg.HxRoute = route
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
func resetHTMLCache() {
	htmlCacheMutex.Lock()
	defer htmlCacheMutex.Unlock()
	htmlCache = make(map[string]CacheEntry)
}
