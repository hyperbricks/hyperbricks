package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"

	"github.com/mitchellh/mapstructure"

	"go.uber.org/zap"
)

func defaultInitMode() bool {
	// Logic to determine default value for InitMode
	// For example, it could be false if not explicitly set
	return false
}

func envTrue(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// GetLogger returns the singleton SugaredLogger instance
func GetLogger() *zap.SugaredLogger {
	return logging.GetLogger().Named("config")
}

func Init_configuration() {
	logging.GetInstance()
}

// CacheTime manages cache duration.
type CacheTime struct {
	Duration time.Duration
}

// Parse converts string to CacheTime.
func (ct *CacheTime) Parse(value string) error {
	d, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	ct.Duration = d
	return nil
}

// String returns CacheTime as string.
func (ct CacheTime) String() string {
	return ct.Duration.String()
}

// UnmarshalText allows mapstructure to decode CacheTime.
func (ct *CacheTime) UnmarshalText(text []byte) error {
	err := ct.Parse(string(text))
	if err != nil {
		GetLogger().Info("Setting cachetime to 24h")
		return ct.Parse(string("24h"))
	}
	return ct.Parse(string(text))
}

const (
	LIVE_MODE        string = "live"
	DEBUG_MODE       string = "debug"
	DEVELOPMENT_MODE string = "development"
)

const PackageConfigFileName = "package.hyperbricks.yaml"

// Config owns runtime settings under the package's hyperbricks mapping.
// Field descriptions/examples follow the component metadata convention. Defaults
// remain in defaultPackageConfig and section decoders, not in a second schema.
// Package metadata and static-export settings have separate consumers.
type Config struct {
	Mode                 string            `mapstructure:"mode" description:"Runtime mode: development, live, or debug. Startup options can override this value; a profile filename does not select a mode." example:"development"`
	Logger               LoggerConfig      `mapstructure:"logger" description:"Runtime logging level, output format, and optional log file. Logging CLI options can override the configured level and format." example:"{level: info, format: console}"`
	Server               ServerConfig      `mapstructure:"server" description:"HTTP listener, connection handling, rendering defaults, routing, runtime gateway, and process-wide CPU parallelism." example:"{port: 8080, gomaxprocs: auto}"`
	RateLimit            RateLimitConfig   `mapstructure:"rate_limit" description:"Process-level token-bucket request limiter, independent of rendered-output caching." example:"{enabled: true, requests_per_second: 100, burst: 500}"`
	Development          DevelopmentConfig `mapstructure:"development" description:"Source watching and developer interfaces. Interface enablement, authentication, and write permissions are separate settings." example:"{watch: true, watch_dirs: [hyperbricks, templates, resources]}"`
	Debug                DebugConfig       `mapstructure:"debug" description:"Legacy debug configuration container with no exported configurable fields. Use mode: debug and logger settings instead." example:"{}"`
	Live                 LiveConfig        `mapstructure:"live" description:"Live-mode rendered-output caching. Browser and reverse-proxy caching are configured separately through HTTP headers." example:"{cache: 10m}"`
	Directories          map[string]string `mapstructure:"directories" description:"Named filesystem directories including hyperbricks, templates, resources, static, render, and plugins. Bare relative paths use the invocation directory; use a module-based path resolver for module-owned files." example:"{resources: {path: {base: module, path: resources}}}"`
	Plugins              PluginsConfig     `mapstructure:"plugins" description:"Compiled plugin names to preload and optional plugin-specific configuration. Built-in Spaces does not require a plugin." example:"{enabled: [ExamplePlugin@1.0.0]}"`
	System               SystemConfig      `mapstructure:"system" description:"Internal runtime service settings, including the developer metrics sampling interval." example:"{metrics_watch_interval: 10s}"`
	frontendEditingError error
	dashboardConfigError error
	processesConfigError error
}

// Frontend editors are development-only; Spaces is built in, other editors are plugins.
type FrontendEditingConfig struct {
	Enabled bool                            `mapstructure:"enabled" description:"Development-only master switch for Spaces and configured frontend editors. Disabling it hides all frontend editors; enabling it does not grant write access." example:"true"`
	Spaces  SpacesConfig                    `mapstructure:"spaces" description:"Built-in Spaces editor settings. Its enabled switch and the parent frontend_editing.enabled switch must both be enabled." example:"{enabled: true, write: false}"`
	Editors map[string]FrontendEditorConfig `mapstructure:"editors" description:"Named frontend-editor plugin mounts. Each entry supplies plugin, route, and optional plugin-owned data; use spaces for the built-in editor." example:"{custom: {plugin: CustomEditor@1.0.0, route: /__hyperbricks/custom}}"`
}

type FrontendEditorConfig struct {
	Plugin string                 `mapstructure:"plugin" description:"Frontend-editor plugin name. The compiled plugin must also be enabled under hyperbricks.plugins.enabled; Spaces is not a plugin." example:"CustomEditor@1.0.0"`
	Route  string                 `mapstructure:"route" description:"Clean, non-reserved /__hyperbricks/ mount path. Editor paths must not overlap each other or Spaces." example:"/__hyperbricks/custom"`
	Data   map[string]interface{} `mapstructure:"data" description:"Open-ended configuration passed to this editor plugin. Its keys and values are owned by the plugin, not the package runtime schema." example:"{}"`
}
type SystemConfig struct {
	MetricsWatchInterval time.Duration `mapstructure:"metrics_watch_interval" description:"Positive Go duration between developer metrics samples. Zero and negative durations are rejected by runtime validation." example:"10s"`
}

type LiveConfig struct {
	CacheTime CacheTime `mapstructure:"cache" description:"Process-wide lifetime of reusable rendered output in live mode, expressed as a Go duration. Use 0s to disable internal output caching; negative durations are invalid. Does not set browser cache policy or bound cache memory." example:"10m"`
}

type DebugConfig struct {
	level string `mapstructure:"level"`
}

type PluginsConfig struct {
	Enabled []string          `mapstructure:"enabled" description:"Compiled plugin artifact names without .so or .wasm, loaded from directories.plugins. A name resolving to both formats is rejected. Native artifacts must match the runtime platform and build dependencies." example:"[ExamplePlugin@1.0.0]"`
	Config  map[string]string `mapstructure:"config" description:"Optional string-valued plugin configuration map. Interpretation belongs to its consumer; these entries do not enable plugins." example:"{}"`
}

type DevelopmentConfig struct {
	Hooks           DevelopmentHooksConfig     `mapstructure:"hooks" description:"Optional finite before_start and after_start tasks. Executed only by a direct development/debug start with --with-processes; each task must finish successfully." example:"{before_start: [{name: prepare, command: [sh, prepare.sh]}]}"`
	Services        []DevelopmentServiceConfig `mapstructure:"services" description:"Optional foreground local HTTP services owned by an opted-in development session. Services start in order, must become ready, and stop with HyperBricks in reverse order." example:"[{name: demo-api, command: [python3, server.py], ready: {http: 'http://127.0.0.1:4319/health'}}]"`
	FrontendEditing FrontendEditingConfig      `mapstructure:"frontend_editing" description:"Development-only Spaces and frontend-editor mounts, including independent write controls. Uses the shared dashboard credentials even when the dashboard is disabled." example:"{enabled: true, spaces: {enabled: true, write: false}}"`
	Dashboard       DevelopmentDashboardConfig `mapstructure:"dashboard" description:"Dashboard Overview and Errors enablement plus shared developer-interface credentials. Must be a mapping; the former Boolean form is invalid." example:"{enabled: false}"`
	FrontendErrors  bool                       `mapstructure:"frontend_errors" description:"Permit frontend error panels when the component enables debugpanel. Panels are restricted to requests authenticated with the shared developer credentials." example:"false"`
	Watch           bool                       `mapstructure:"watch" description:"Enable source-directory watching in development mode. Package configuration changes still require a process restart." example:"true"`
	WatchDirs       []string                   `mapstructure:"watch_dirs" description:"Directories to watch when development watching is enabled. Entries matching directories keys use those configured paths; other entries are filesystem paths. An omitted or empty list watches hyperbricks and templates." example:"[hyperbricks, templates, resources]"`
	Reload          bool                       `mapstructure:"reload" description:"Configured development reload flag. Currently retained and logged; the source watcher is controlled by watch, and this flag alone does not enable browser refresh." example:"false"`
}

type DevelopmentDashboardConfig struct {
	Enabled     bool              `mapstructure:"enabled" description:"Enable Dashboard Overview and Errors. This switch does not disable independently enabled Spaces or other developer interfaces." example:"false"`
	Credentials CredentialsConfig `mapstructure:"credentials" description:"Shared developer-interface username and password, separate from deployment-service credentials. There is no default account. Missing or empty values leave enabled interfaces locked with HTTP 503; use environment resolvers and protect non-loopback access with encrypted transport." example:"{user: {env: HB_DEVELOPER_USER}, password: {env: HB_DEVELOPER_PASSWORD}}"`
}

// LoggerConfig with defaults.
type LoggerConfig struct {
	Level  string `mapstructure:"level" description:"Runtime log threshold. When omitted, startup selects debug in debug mode and info otherwise; CLI logging options can override it." example:"info"`
	Path   string `mapstructure:"path" description:"Explicit runtime log-file path, not a directory. When empty, development/debug uses directories.logs/hyperbricks.log if logs is configured; otherwise no file output is added." example:"logs/hyperbricks.log"`
	Format string `mapstructure:"format" description:"Log encoding: console or json. Startup uses console when omitted; the CLI log-format option can override it." example:"console"`
}

type RoutingConfig struct {
	CleanURLs  bool     `mapstructure:"clean_urls" description:"Enable internal clean-URL matching, not redirects. Exact routes are tried before extension-based alternatives." example:"true"`
	IndexFiles []string `mapstructure:"index_files" description:"Index filenames considered for root and directory requests. Omitted or empty lists use the runtime routing defaults." example:"[index.html, index.htm]"`
	Extensions []string `mapstructure:"extensions" description:"Filename extensions used for clean-URL matching, without leading dots. Omitted or empty lists use the runtime routing defaults." example:"[html, htm]"`
}

type RuntimeGatewayConfig struct {
	Enabled      bool     `mapstructure:"enabled" description:"Enable runtime host routing through the configured resolver. Requires at least one domain or host suffix and an HTTP(S) resolver URL." example:"false"`
	Domain       string   `mapstructure:"domain" description:"Gateway domain whose subhosts identify runtimes. The apex domain itself does not match. Combined with domains; comma-separated values are accepted." example:"runtime.local"`
	Domains      []string `mapstructure:"domains" description:"Additional gateway domains for dotted runtime hostnames, combined with domain." example:"[runtime.local, live.local]"`
	HostSuffix   string   `mapstructure:"host_suffix" description:"Suffix for flat runtime hostnames, combined with host_suffixes. Comma-separated values are accepted." example:"-runtime.example.test"`
	HostSuffixes []string `mapstructure:"host_suffixes" description:"Additional suffixes for flat runtime hostnames, combined with host_suffix." example:"[-runtime.example.test, -live.example.test]"`
	Resolver     string   `mapstructure:"resolver" description:"HTTP(S) endpoint used to resolve matching runtime hosts. Required when the gateway is enabled; static configuration validation does not contact it." example:"http://127.0.0.1:8080/resolve-runtime"`
}

// ServerConfig with defaults.
type ServerConfig struct {
	// GoMaxProcs retains auto/integer input for strict startup validation.
	GoMaxProcs        any                  `mapstructure:"gomaxprocs" description:"Process-wide Go execution parallelism: auto or an integer from 1 through the detected logical CPU count. Omitted means auto. Numeric strings are accepted for environment resolvers; package configuration overrides standalone GOMAXPROCS." example:"auto"`
	Port              int                  `mapstructure:"port" description:"HTTP listener port, subject to startup CLI overrides. Choosing a value does not prove the port is available." example:"8080"`
	Beautify          bool                 `mapstructure:"beautify" description:"Global rendered-HTML beautification switch used by renderers that support beautification. Does not format YAML source." example:"false"`
	SelfClosingTags   bool                 `mapstructure:"self_closing_tags" description:"Use XHTML-style self-closing tags where supported by the HTML rendering helpers." example:"true"`
	ReadTimeout       time.Duration        `mapstructure:"read_timeout" description:"Go duration limiting how long the HTTP server may read a request." example:"5s"`
	WriteTimeout      time.Duration        `mapstructure:"write_timeout" description:"Go duration limiting how long the HTTP server may write a response." example:"10s"`
	IdleTimeout       time.Duration        `mapstructure:"idle_timeout" description:"Go duration limiting how long a keep-alive connection may remain idle." example:"20s"`
	KeepAlivesEnabled bool                 `mapstructure:"keep_alives_enabled" description:"Allow reuse of HTTP connections across requests. Disabling this increases connection churn." example:"true"`
	Routing           RoutingConfig        `mapstructure:"routing" description:"Internal URL matching defaults for clean URLs, index filenames, and allowed extensions." example:"{clean_urls: true, index_files: [index.html, index.htm], extensions: [html, htm]}"`
	RuntimeGateway    RuntimeGatewayConfig `mapstructure:"runtime_gateway" description:"Package-owned runtime host gateway. These settings do not configure the separate deployment service." example:"{enabled: false}"`
}

type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled" description:"Enable the process-level request limiter. Disable explicitly when another trusted layer owns limiting or for controlled benchmarks." example:"true"`
	RequestsPerSecond int  `mapstructure:"requests_per_second" description:"Token refill rate for the enabled limiter. Zero is not a disable switch; use enabled: false to disable limiting." example:"100"`
	Burst             int  `mapstructure:"burst" description:"Maximum token-bucket burst capacity for the enabled request limiter." example:"500"`
}

var (
	instance *Config
	once     sync.Once
	Module   string
)

// GetHyperBricksConfiguration returns the singleton instance of the Config.
func GetHyperBricksConfiguration() *Config {
	once.Do(func() {
		instance = loadHyperBricksConfiguration()
	})
	return instance
}

// loadHyperBricksConfiguration initializes the Config object with defaults and decodes the config file.
func loadHyperBricksConfiguration() *Config {
	dir, err := os.Getwd()
	if err != nil {
		GetLogger().Errorw("Failed to get working directory", "error", err)
	}

	configFilePath := Module
	if !filepath.IsAbs(configFilePath) {
		configFilePath = filepath.Join(dir, configFilePath)
	}

	runtimeOptions := GetRuntimeOptions()
	moduleDir := runtimeModuleRoot(runtimeOptions)
	parsedConfig, err := LoadPackageConfigMap(configFilePath, moduleDir)
	var processesSourceError error
	if _, ok := err.(*developmentProcessesSourceError); ok {
		processesSourceError = err
	}
	if err != nil {
		GetLogger().Warnw("Failed to load package configuration; using defaults", "file", logging.ModulePath(moduleDir, configFilePath), "error", logging.ModuleText(moduleDir, err.Error()))
		parsedConfig = map[string]interface{}{}
	}
	parser.HbConfig = parsedConfig

	config, decodeErr := decodePackageConfig(parsedConfig, moduleDir)
	if processesSourceError != nil {
		config.processesConfigError = processesSourceError
	}
	if decodeErr != nil {
		GetLogger().Errorw("Failed to decode configuration", "file", logging.ModulePath(moduleDir, configFilePath), "error", logging.ModuleText(moduleDir, decodeErr.Error()))
	}
	applyRuntimeOptions(config, runtimeOptions)
	normalizePackageMode(config)
	GetLogger().Debugw("Package configuration loaded", "file", logging.ModulePath(moduleDir, Module), "mode", config.Mode)
	return config
}

// LoadPackageConfigStrict loads and validates one module package configuration
// without changing the runtime singleton or parser configuration globals. It
// uses the same defaults, typed decoding, and runtime overrides as normal
// startup, while returning errors that startup may log or recover from.
func LoadPackageConfigStrict(configFilePath string, moduleDir string) (*Config, error) {
	result, err := yamlparser.ProcessConfigFile(configFilePath, packageConfigYAMLOptions(moduleDir))
	if err != nil {
		return nil, fmt.Errorf("load package configuration: %w", err)
	}
	return validatePackageConfigResult(result, moduleDir, GetRuntimeOptions(), true)
}

// ValidatePackageConfigBytes validates a proposed package configuration in the
// selected module context without applying command-line or process overrides.
// It is intended for editors that validate what will be persisted rather than
// the effective configuration of the process performing the validation.
func ValidatePackageConfigBytes(input []byte, moduleDir string) (*Config, error) {
	result, err := yamlparser.ProcessConfigBytes(input, packageConfigYAMLOptions(moduleDir))
	if err != nil {
		return nil, fmt.Errorf("load package configuration: %w", err)
	}
	return validatePackageConfigResult(result, moduleDir, RuntimeOptions{}, false)
}

// ValidatePackageConfigBytesWithResourceReader applies the same configuration
// contract as ValidatePackageConfigBytes, with resource access controlled by
// the caller. Editor checks must not bypass their filesystem boundary through
// a file resolver or update the runtime's shared template registry.
func ValidatePackageConfigBytesWithResourceReader(input []byte, moduleDir string, readFile func(string) ([]byte, error)) (*Config, error) {
	if readFile == nil {
		return nil, fmt.Errorf("resource reader is required")
	}
	options := packageConfigYAMLOptions(moduleDir)
	options.ResourceReadFile = readFile
	options.SkipTemplateRegistration = true
	result, err := yamlparser.ProcessConfigBytes(input, options)
	if err != nil {
		return nil, fmt.Errorf("load package configuration: %w", err)
	}
	return validatePackageConfigResult(result, moduleDir, RuntimeOptions{}, false)
}

func validatePackageConfigResult(result *yamlparser.ConfigResult, moduleDir string, runtimeOptions RuntimeOptions, applyOverrides bool) (*Config, error) {
	for _, diagnostic := range result.Diagnostics {
		if strings.EqualFold(diagnostic.Level, "error") {
			return nil, fmt.Errorf("load package configuration: %s at %s: %s", diagnostic.Code, diagnostic.Path, diagnostic.Message)
		}
	}
	if err := validateDevelopmentProcessSourceTypes(result.Preprocessed); err != nil {
		return nil, err
	}

	config, decodeErr := decodePackageConfigStrict(result.Materialized, moduleDir)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode package configuration: %w", decodeErr)
	}
	if err := validatePackageMode(config.Mode); err != nil {
		return nil, err
	}
	if err := config.ValidateDevelopmentDashboard(); err != nil {
		return nil, fmt.Errorf("validate development dashboard: %w", err)
	}
	if err := config.ValidateFrontendEditing(); err != nil {
		return nil, fmt.Errorf("validate frontend editing: %w", err)
	}
	if err := config.ValidateDevelopmentProcesses(); err != nil {
		return nil, err
	}
	if applyOverrides {
		applyRuntimeOptions(config, runtimeOptions)
	}
	if err := validatePackageMode(config.Mode); err != nil {
		return nil, err
	}
	if err := config.ValidateRuntimeSettings(); err != nil {
		return nil, fmt.Errorf("validate runtime settings: %w", err)
	}
	return config, nil
}

func defaultPackageConfig(moduleDir string) *Config {
	return &Config{

		Mode: LIVE_MODE, // Default mode

		Logger: LoggerConfig{},

		Server: ServerConfig{
			Port: 8080, // Default port
			// Default to XHTML-friendly tags when not configured.
			SelfClosingTags: true,

			// Default Low traffic (~50-500 daily visitors).
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       20 * time.Second,
			KeepAlivesEnabled: true,

			Routing: RoutingConfig{
				CleanURLs:  true,
				IndexFiles: []string{"index.html", "index.htm"},
				Extensions: []string{"html", "htm"},
			},
		},

		System: SystemConfig{
			MetricsWatchInterval: 10 * time.Second,
		},
		Plugins: PluginsConfig{
			Enabled: []string{},
			Config:  map[string]string{},
		},
		RateLimit: RateLimitConfig{
			// Default Low traffic (~50-500 daily visitors).
			Enabled:           true,
			Burst:             10,
			RequestsPerSecond: 5,
		},

		Directories: map[string]string{
			"render":      fmt.Sprintf("%s/rendered", moduleDir),
			"static":      fmt.Sprintf("%s/static", moduleDir),
			"plugins":     "bin/plugins",
			"resources":   fmt.Sprintf("%s/resources", moduleDir),
			"templates":   fmt.Sprintf("%s/templates", moduleDir),
			"hyperbricks": fmt.Sprintf("%s/hyperbricks", moduleDir),
		},

		Development: DevelopmentConfig{
			Watch:          false,
			Reload:         false,
			FrontendErrors: false,
		},

		Live: LiveConfig{
			CacheTime: CacheTime{
				Duration: 10 * time.Minute, // Default cache duration
			},
		},
	}
}

func decodePackageConfig(parsedConfig map[string]interface{}, moduleDir string) (*Config, error) {
	return decodePackageConfigWithPolicy(parsedConfig, moduleDir, false)
}

func decodePackageConfigStrict(parsedConfig map[string]interface{}, moduleDir string) (*Config, error) {
	return decodePackageConfigWithPolicy(parsedConfig, moduleDir, true)
}

func decodePackageConfigWithPolicy(parsedConfig map[string]interface{}, moduleDir string, strict bool) (*Config, error) {
	config := defaultPackageConfig(moduleDir)
	if err := decodeConfigWithPolicy(parsedConfig["hyperbricks"], config, strict); err != nil {
		return config, err
	}
	return config, nil
}

func applyRuntimeOptions(config *Config, runtimeOptions RuntimeOptions) {
	if runtimeOptions.PortOverride {
		config.Server.Port = runtimeOptions.Port
	}
	if runtimeOptions.RuntimeGatewayEnabled {
		config.Server.RuntimeGateway.Enabled = true
	}
	if strings.TrimSpace(runtimeOptions.RuntimeGatewayDomain) != "" {
		config.Server.RuntimeGateway.Domain = strings.TrimSpace(runtimeOptions.RuntimeGatewayDomain)
	}
	if strings.TrimSpace(runtimeOptions.RuntimeGatewayHostSuffix) != "" {
		config.Server.RuntimeGateway.HostSuffix = strings.TrimSpace(runtimeOptions.RuntimeGatewayHostSuffix)
	}
	if strings.TrimSpace(runtimeOptions.RuntimeGatewayResolver) != "" {
		config.Server.RuntimeGateway.Resolver = strings.TrimSpace(runtimeOptions.RuntimeGatewayResolver)
	}
	modeOverride := strings.ToLower(strings.TrimSpace(runtimeOptions.ModeOverride))
	if modeOverride == "" {
		modeOverride = strings.ToLower(strings.TrimSpace(os.Getenv("HB_DEPLOY_RUNTIME_MODE")))
	}
	if modeOverride != "" {
		config.Mode = modeOverride
	} else if runtimeOptions.Production || envTrue("HB_DEPLOY_PRODUCTION") || envTrue("HB_PRODUCTION") {
		config.Mode = LIVE_MODE
	}
}

func validatePackageMode(mode string) error {
	switch mode {
	case LIVE_MODE, DEVELOPMENT_MODE, DEBUG_MODE:
		return nil
	default:
		return fmt.Errorf("invalid hyperbricks.mode %q: expected %q, %q, or %q", mode, LIVE_MODE, DEVELOPMENT_MODE, DEBUG_MODE)
	}
}

func normalizePackageMode(config *Config) {
	if config.Mode == LIVE_MODE {
		GetLogger().Debug("Setting mode to live (production) mode")
	} else if config.Mode == DEVELOPMENT_MODE {
		GetLogger().Debug("Setting mode to development mode")
	} else if config.Mode == DEBUG_MODE {
		GetLogger().Debug("Setting mode to debug mode")
	} else {
		GetLogger().Debugf("Invalid mode set in package config %v", config.Mode)

		GetLogger().Warn("Setting mode not recognised, setting to live (production) mode")
		config.Mode = LIVE_MODE
	}
}

func LoadPackageConfigMap(configFilePath string, moduleDir string) (map[string]interface{}, error) {
	result, err := yamlparser.ProcessConfigFile(configFilePath, packageConfigYAMLOptions(moduleDir))
	if err != nil {
		return nil, err
	}
	if err := validateDevelopmentProcessSourceTypes(result.Preprocessed); err != nil {
		return nil, err
	}
	if err := validateDevelopmentProcessResolverDiagnostics(result); err != nil {
		return nil, err
	}
	return result.Materialized, nil
}

func packageConfigYAMLOptions(moduleDir string) yamlparser.Options {
	modulesRoot := filepath.Dir(moduleDir)
	return yamlparser.Options{
		Variables: map[string]string{
			"module": moduleDir,
		},
		Paths: yamlparser.PathMarkers{
			ModuleRoot:  modulesRoot,
			Root:        ".",
			Module:      moduleDir,
			Resources:   filepath.Join(moduleDir, "resources"),
			Templates:   filepath.Join(moduleDir, "templates"),
			Static:      filepath.Join(moduleDir, "static"),
			HyperBricks: filepath.Join(moduleDir, "hyperbricks"),
			Render:      filepath.Join(moduleDir, "rendered"),
		},
	}
}

// decodeConfig decodes map to struct with defaults using mapstructure.
func decodeConfig(input interface{}, output interface{}) error {
	return decodeConfigWithPolicy(input, output, false)
}

func decodeConfigWithPolicy(input interface{}, output interface{}, strict bool) error {
	if config, ok := output.(*Config); ok {
		hooks, services, err := decodeDevelopmentProcesses(input)
		config.processesConfigError = err
		config.Development.Hooks, config.Development.Services = hooks, services
		if err != nil {
			return err
		}
		// These sections deliberately bypass the legacy weakly typed decoder.
		// Restore their validated defaults even when decoding a reused config.
		defer func() { config.Development.Hooks, config.Development.Services = hooks, services }()

		dashboard, err := decodeDevelopmentDashboard(input)
		config.dashboardConfigError = err
		config.Development.Dashboard = dashboard
		if err != nil {
			return err
		}
		// Use the separately validated value even when decoding into a reused config.
		defer func() { config.Development.Dashboard = dashboard }()

		editing, err := decodeFrontendEditing(input)
		config.frontendEditingError = err
		config.Development.FrontendEditing = editing
		if err != nil {
			return err
		}
		// Use the separately validated value even when decoding into a reused config.
		defer func() { config.Development.FrontendEditing = editing }()
	}
	var fallback CacheTime
	err := fallback.Parse("24h") // Fallback duration
	if err != nil {
		GetLogger().Errorw("Failed to set fallback cache duration", "error", err)
	}

	decodeHook := mapstructure.ComposeDecodeHookFunc(
		// Decode CacheTime
		func(srcType reflect.Type, destType reflect.Type, value interface{}) (interface{}, error) {
			if srcType.Kind() == reflect.String && destType == reflect.TypeOf(CacheTime{}) {
				var ct CacheTime
				err := ct.Parse(value.(string))
				if err != nil {
					if strict {
						return CacheTime{}, fmt.Errorf("invalid cache duration %q: %w", value, err)
					}
					GetLogger().Errorw("Failed to parse cache duration", "value", value, "error", err)
					return fallback, nil // Use fallback value on error
				}
				return ct, nil
			}
			return value, nil
		},
		// Decode time.Duration
		func(srcType reflect.Type, destType reflect.Type, value interface{}) (interface{}, error) {
			if srcType.Kind() == reflect.String && destType == reflect.TypeOf(time.Duration(0)) {
				duration, err := time.ParseDuration(value.(string))
				if err != nil {
					if strict {
						return time.Duration(0), fmt.Errorf("invalid duration %q: %w", value, err)
					}
					GetLogger().Errorw("Failed to parse duration", "value", value, "error", err)
					return time.Duration(0), nil // Default to zero if parsing fails
				}
				return duration, nil
			}
			return value, nil
		},
	)

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       decodeHook,
		WeaklyTypedInput: true,
		Result:           output,
		TagName:          "mapstructure",
	})
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}
