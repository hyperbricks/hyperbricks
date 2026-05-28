package main

import (
	"fmt"
	"os"
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
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type hyperBricksConfigSource struct {
	Filename string
	Config   map[string]interface{}
	Errors   []error
}

// PreProcessAndPopulateConfigs orchestrates the preprocessing and population of configurations.
func PreProcessAndPopulateConfigs() error {
	hbConfig, logger := retrieveConfigAndLogger()
	determineDirectories(hbConfig)

	sources, err := loadHyperBricks()
	if err != nil {
		return fmt.Errorf("error loading HyperBricks: %w", err)
	}

	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	tempRouteSourceErrors := make(map[string][]error)
	filenameToRoutes := make(map[string][]string)

	// ---- Process configs in strict order! ----
	for _, source := range sources {
		if err := processScript(source.Filename, source.Config, source.Errors, tempConfigs, tempHyperMediasBySection, tempRouteSourceErrors, logger, filenameToRoutes); err != nil {
			logger.Warnw("Error processing script", "file", source.Filename, "error", err)
		}
	}

	// populate configurations
	updateGlobalConfigs(tempConfigs)
	updateGlobalHyperMediasBySection(tempHyperMediasBySection)
	updateGlobalRouteSourceErrors(tempRouteSourceErrors)

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
			if filepath.IsAbs(dir) {
				return dir
			}
			return filepath.Clean(fmt.Sprintf("./%s", dir))
		}
		return defaultDir
	}

	moduleDir := commands.GetModuleRoot()
	modulesRoot := filepath.Dir(moduleDir)
	if !filepath.IsAbs(modulesRoot) {
		modulesRoot = filepath.Clean("./" + modulesRoot)
	}
	core.ModuleDirectories.ModulesRoot = modulesRoot
	core.ModuleDirectories.ModuleDir = moduleDir

	core.ModuleDirectories.Root = "./"
	core.ModuleDirectories.RenderedDir = getDirectory("render", fmt.Sprintf("%s/rendered", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.TemplateDir = getDirectory("templates", fmt.Sprintf("%s/templates", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.HyperbricksDir = getDirectory("hyperbricks", fmt.Sprintf("%s/hyperbricks", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.StaticDir = getDirectory("static", fmt.Sprintf("%s/static", core.ModuleDirectories.ModuleDir))
	core.ModuleDirectories.ResourcesDir = getDirectory("resources", fmt.Sprintf("%s/resources", core.ModuleDirectories.ModuleDir))

	return core.ModuleDirectories
}

// loadHyperBricks preprocesses HyperBricks sources from the configured directory.
func loadHyperBricks() ([]hyperBricksConfigSource, error) {
	legacySources, err := loadLegacyHyperBricksSources()
	if err != nil {
		return nil, err
	}
	yamlSources, err := loadYAMLHyperBricksSources()
	if err != nil {
		return nil, err
	}

	sources := append(legacySources, yamlSources...)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Filename < sources[j].Filename
	})
	if len(sources) == 0 {
		return nil, fmt.Errorf("no .hyperbricks or .hyperbricks.yaml files found in %s", core.ModuleDirectories.HyperbricksDir)
	}
	return sources, nil
}

func loadLegacyHyperBricksSources() ([]hyperBricksConfigSource, error) {
	files, err := filepath.Glob(filepath.Join(core.ModuleDirectories.HyperbricksDir, "*.hyperbricks"))
	if err != nil {
		return nil, fmt.Errorf("glob legacy hyperbricks files: %w", err)
	}
	sort.Strings(files)

	tempHyperBricks := make(map[string]string, len(files))
	orderedRoutes := make([]string, 0, len(files))
	sources := make([]hyperBricksConfigSource, 0, len(files))

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read legacy hyperbricks file %s: %w", file, err)
		}
		filename := hyperBricksSourceName(file, ".hyperbricks")
		uncommented := parser.StripComments(string(data))
		preprocessed, err := parser.PreprocessHyperScript(uncommented)
		if err != nil {
			return nil, fmt.Errorf("preprocess legacy hyperbricks file %s: %w", file, err)
		}
		tempHyperBricks[filename] = preprocessed
		orderedRoutes = append(orderedRoutes, filename)
		sources = append(sources, hyperBricksConfigSource{
			Filename: filename,
			Config:   parser.ParseHyperScript(preprocessed),
		})
		logging.GetLogger().Debug("Loaded legacy configuration for route: ", filename)
	}

	hyperBricksArray.PreProcessedHyperScriptStoreMutex.Lock()
	hyperBricksArray.HyperBricksStore = tempHyperBricks
	hyperBricksArray.OrderedHyperBricksRoutes = orderedRoutes
	hyperBricksArray.PreProcessedHyperScriptStoreMutex.Unlock()

	return sources, nil
}

func loadYAMLHyperBricksSources() ([]hyperBricksConfigSource, error) {
	files, err := filepath.Glob(filepath.Join(core.ModuleDirectories.HyperbricksDir, "*.hyperbricks.yaml"))
	if err != nil {
		return nil, fmt.Errorf("glob YAML hyperbricks files: %w", err)
	}
	sort.Strings(files)

	opts := yamlRuntimeOptions()
	sources := make([]hyperBricksConfigSource, 0, len(files))
	for _, file := range files {
		result, err := yamlparser.ProcessFile(file, opts)
		if err != nil {
			return nil, fmt.Errorf("process YAML hyperbricks file %s: %w", file, err)
		}
		sources = append(sources, hyperBricksConfigSource{
			Filename: hyperBricksSourceName(file, ".hyperbricks.yaml"),
			Config:   result.Materialized,
			Errors:   yamlDiagnosticsToComponentErrors(result.Diagnostics),
		})
		logging.GetLogger().Debug("Loaded YAML configuration for route: ", file)
	}
	return sources, nil
}

func yamlRuntimeOptions() yamlparser.Options {
	return yamlparser.Options{
		Config:      parser.HbConfig,
		Variables:   yamlRuntimeVariables(),
		TemplateDir: core.ModuleDirectories.TemplateDir,
		Paths: yamlparser.PathMarkers{
			ModuleRoot:  core.ModuleDirectories.ModulesRoot,
			Root:        core.ModuleDirectories.Root,
			Module:      core.ModuleDirectories.ModuleDir,
			Resources:   core.ModuleDirectories.ResourcesDir,
			Templates:   core.ModuleDirectories.TemplateDir,
			Static:      core.ModuleDirectories.StaticDir,
			HyperBricks: core.ModuleDirectories.HyperbricksDir,
		},
		RecoverDuplicateChildren: true,
	}
}

func yamlDiagnosticsToComponentErrors(diagnostics []yamlparser.Diagnostic) []error {
	out := make([]error, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		errorPath := fmt.Sprintf("%s:%d:%d:%s", diagnostic.Source, diagnostic.Line, diagnostic.Column, diagnostic.Code)
		fileName := hyperBricksSourceName(diagnostic.Source, ".hyperbricks.yaml")
		out = append(out, shared.ComponentError{
			Hash:  shared.HyperScriptErrorHash(errorPath),
			File:  fileName,
			Type:  "YAML",
			Path:  diagnostic.Path,
			Key:   diagnostic.OriginalName,
			Err:   fmt.Sprintf("%s (source: %s:%d:%d)", diagnostic.Message, filepath.Base(diagnostic.Source), diagnostic.Line, diagnostic.Column),
			Level: "WARNING",
		})
	}
	return out
}

func yamlRuntimeVariables() map[string]string {
	return map[string]string{
		"module_root": core.ModuleDirectories.ModulesRoot,
		"root":        core.ModuleDirectories.Root,
		"module":      core.ModuleDirectories.ModuleDir,
		"resources":   core.ModuleDirectories.ResourcesDir,
		"templates":   core.ModuleDirectories.TemplateDir,
		"static":      core.ModuleDirectories.StaticDir,
		"hyperbricks": core.ModuleDirectories.HyperbricksDir,
		"render":      core.ModuleDirectories.RenderedDir,
	}
}

func hyperBricksSourceName(path string, suffix string) string {
	name := filepath.Base(path)
	if strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(name, suffix)
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
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
	sourceErrors []error,
	tempConfigs map[string]map[string]interface{},
	tempHyperMediasBySection map[string][]composite.HyperMediaConfig,
	tempRouteSourceErrors map[string][]error,
	logger *zap.SugaredLogger,
	filenameToRoutes map[string][]string, // <-- add this
) error {
	hbConfig := getHyperBricksConfiguration()

	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ips, err := getHostIPv4s()
	if err != nil {
		logging.GetLogger().Errorw("Error retrieving host IPs", "error", err)
	}
	if len(ips) == 0 {
		logging.GetLogger().Errorw("No IPv4 addresses found for the host")
	}
	shared.Location = fmt.Sprintf("%s:%d", ips[0], hbConfig.Server.Port)

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
			fragmentRouteConfig := routeMetadataConfig{}
			if typeValue == composite.FragmentConfigGetName() {
				fragmentConfig, err := decodeFragmentConfig(obj)
				if err != nil {
					logger.Warnw("Error decoding FragmentConfig", "file", filename, "key", key, "type", typeValue, "error", err)
					continue
				}
				fragmentRouteConfig = routeMetadataConfig{
					Title:   fragmentConfig.Title,
					Route:   fragmentConfig.Route,
					Section: fragmentConfig.Section,
				}
			} else {
				var err error
				fragmentRouteConfig, err = decodeRouteMetadataConfig(obj)
				if err != nil {
					logger.Warnw("Error decoding API fragment route metadata", "file", filename, "key", key, "type", typeValue, "error", err)
					continue
				}
			}
			if fragmentRouteConfig.Route == "" {
				continue
			}
			fragmentRouteConfig.Route = ensureUniqueRoute(fragmentRouteConfig.Route, filename, tempConfigs)
			obj["route"] = fragmentRouteConfig.Route
			hyperMediaConfig := composite.HyperMediaConfig{
				Section: fragmentRouteConfig.Section,
				Title:   fragmentRouteConfig.Title,
				Route:   fragmentRouteConfig.Route,
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
			tempConfigs[fragmentRouteConfig.Route] = obj
			addRouteSourceErrors(tempRouteSourceErrors, fragmentRouteConfig.Route, sourceErrors)

			// --- Map filename to route here
			filenameToRoutes[filename] = append(filenameToRoutes[filename], fragmentRouteConfig.Route)

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
			tempHyperMediasBySection[hyperMediaConfig.Section] = append(
				tempHyperMediasBySection[hyperMediaConfig.Section],
				hyperMediaConfig,
			)

			if hyperMediaConfig.Static == "" {
				logger.Info(fmt.Sprintf("route  (%s): [http://%s/%s] initialized", filename, shared.Location, hyperMediaConfig.Route))
			} else {
				logger.Info(fmt.Sprintf("static file: %s", hyperMediaConfig.Static))
			}
			obj["hyperbricksfile"] = filename
			obj["hyperbrickskey"] = key
			tempConfigs[hyperMediaConfig.Route] = obj
			addRouteSourceErrors(tempRouteSourceErrors, hyperMediaConfig.Route, sourceErrors)

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

type routeMetadataConfig struct {
	Title   string `mapstructure:"title"`
	Route   string `mapstructure:"route"`
	Section string `mapstructure:"section"`
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

func decodeRouteMetadataConfig(v map[string]interface{}) (routeMetadataConfig, error) {
	var routeConfig routeMetadataConfig
	decoder, err := createDecoder(&routeConfig)
	if err != nil {
		return routeConfig, err
	}
	err = decoder.Decode(v)
	return routeConfig, err
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

func addRouteSourceErrors(target map[string][]error, route string, errors []error) {
	if len(errors) == 0 {
		return
	}
	target[route] = append(target[route], errors...)
}

func updateGlobalRouteSourceErrors(errorsByRoute map[string][]error) {
	routeSourceErrorsMutex.Lock()
	defer routeSourceErrorsMutex.Unlock()
	routeSourceErrors = cloneErrorsByRoute(errorsByRoute)
}

func getRouteSourceErrors(route string) []error {
	routeSourceErrorsMutex.RLock()
	defer routeSourceErrorsMutex.RUnlock()
	return append([]error(nil), routeSourceErrors[route]...)
}

func cloneErrorsByRoute(source map[string][]error) map[string][]error {
	out := make(map[string][]error, len(source))
	for route, errors := range source {
		out[route] = append([]error(nil), errors...)
	}
	return out
}

// resetHTMLCache clears the HTML cache.
func clearHTMLCache() {
	htmlCacheMutex.Lock()
	defer htmlCacheMutex.Unlock()
	htmlCache = make(map[string]CacheEntry)
}
