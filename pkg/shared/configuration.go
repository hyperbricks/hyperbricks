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

// Config structure with default values.
type Config struct {
	Mode                 string            `mapstructure:"mode"`
	Logger               LoggerConfig      `mapstructure:"logger"`
	Server               ServerConfig      `mapstructure:"server"`
	RateLimit            RateLimitConfig   `mapstructure:"rate_limit"`
	Development          DevelopmentConfig `mapstructure:"development"`
	Debug                DebugConfig       `mapstructure:"debug"`
	Live                 LiveConfig        `mapstructure:"live"`
	Directories          map[string]string `mapstructure:"directories"`
	Plugins              PluginsConfig     `mapstructure:"plugins"`
	System               SystemConfig      `mapstructure:"system"`
	frontendEditingError error
	dashboardConfigError error
}

// Frontend editors are development-only; Spaces is built in, other editors are plugins.
type FrontendEditingConfig struct {
	Enabled bool                            `mapstructure:"enabled"`
	Spaces  SpacesConfig                    `mapstructure:"spaces"`
	Editors map[string]FrontendEditorConfig `mapstructure:"editors"`
}

type FrontendEditorConfig struct {
	Plugin string                 `mapstructure:"plugin"`
	Route  string                 `mapstructure:"route"`
	Data   map[string]interface{} `mapstructure:"data"`
}
type SystemConfig struct {
	MetricsWatchInterval time.Duration `mapstructure:"metrics_watch_interval"`
}

type LiveConfig struct {
	CacheTime CacheTime `mapstructure:"cache"`
}

type DebugConfig struct {
	level string `mapstructure:"level"`
}

type PluginsConfig struct {
	Enabled []string          `mapstructure:"enabled"`
	Config  map[string]string `mapstructure:"config"`
}

type DevelopmentConfig struct {
	FrontendEditing FrontendEditingConfig      `mapstructure:"frontend_editing"`
	Dashboard       DevelopmentDashboardConfig `mapstructure:"dashboard"`
	FrontendErrors  bool                       `mapstructure:"frontend_errors"`
	Watch           bool                       `mapstructure:"watch"`
	WatchDirs       []string                   `mapstructure:"watch_dirs"`
	Reload          bool                       `mapstructure:"reload"`
}

type DevelopmentDashboardConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
}

// LoggerConfig with defaults.
type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Path   string `mapstructure:"path"`
	Format string `mapstructure:"format"`
}

type RoutingConfig struct {
	CleanURLs  bool     `mapstructure:"clean_urls"`
	IndexFiles []string `mapstructure:"index_files"`
	Extensions []string `mapstructure:"extensions"`
}

type RuntimeGatewayConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	Domain       string   `mapstructure:"domain"`
	Domains      []string `mapstructure:"domains"`
	HostSuffix   string   `mapstructure:"host_suffix"`
	HostSuffixes []string `mapstructure:"host_suffixes"`
	Resolver     string   `mapstructure:"resolver"`
}

// ServerConfig with defaults.
type ServerConfig struct {
	// GoMaxProcs retains auto/integer input for strict startup validation.
	GoMaxProcs        any                  `mapstructure:"gomaxprocs"`
	Port              int                  `mapstructure:"port"`
	Beautify          bool                 `mapstructure:"beautify"`
	SelfClosingTags   bool                 `mapstructure:"self_closing_tags"`
	ReadTimeout       time.Duration        `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration        `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration        `mapstructure:"idle_timeout"`
	KeepAlivesEnabled bool                 `mapstructure:"keep_alives_enabled"`
	Routing           RoutingConfig        `mapstructure:"routing"`
	RuntimeGateway    RuntimeGatewayConfig `mapstructure:"runtime_gateway"`
}

type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerSecond int  `mapstructure:"requests_per_second"`
	Burst             int  `mapstructure:"burst"`
}

type DeployConfig struct {
	Remote DeployRemoteConfig `mapstructure:"remote"`
	Local  DeployLocalConfig  `mapstructure:"local"`
	Client DeployClientConfig `mapstructure:"client"`
}

type DeployRemoteConfig struct {
	Bind        string                 `mapstructure:"bind"`
	Port        int                    `mapstructure:"port"`
	Root        string                 `mapstructure:"root"`
	PortStart   int                    `mapstructure:"port_start"`
	LogsEnabled bool                   `mapstructure:"logs_enabled"`
	Binary      string                 `mapstructure:"binary"`
	Credentials CredentialsConfig      `mapstructure:"credentials"`
	HMACSecret  string                 `mapstructure:"hmac_secret"`
	Auth        DeployRemoteAuthConfig `mapstructure:"auth"`
}

type DeployRemoteAuthConfig struct {
	EnvPrefix string `mapstructure:"env_prefix"`
}

type DeployLocalConfig struct {
	Bind        string            `mapstructure:"bind"`
	Port        int               `mapstructure:"port"`
	ModulesDir  string            `mapstructure:"modules_dir"`
	BuildRoot   string            `mapstructure:"build_root"`
	PortStart   int               `mapstructure:"port_start"`
	LogsEnabled bool              `mapstructure:"logs_enabled"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
}

type DeployClientConfig struct {
	Target  string                        `mapstructure:"target"`
	Targets map[string]DeployClientTarget `mapstructure:"targets"`
}

type DeployClientTarget struct {
	API         string            `mapstructure:"api"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
	HMACSecret  string            `mapstructure:"hmac_secret"`
	KeyID       string            `mapstructure:"key_id"`
}

type CredentialsConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
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
	if err != nil {
		GetLogger().Warnw("Failed to load package configuration; using defaults", "file", logging.ModulePath(moduleDir, configFilePath), "error", logging.ModuleText(moduleDir, err.Error()))
		parsedConfig = map[string]interface{}{}
	}
	parser.HbConfig = parsedConfig

	config, decodeErr := decodePackageConfig(parsedConfig, moduleDir)
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

func validatePackageConfigResult(result *yamlparser.ConfigResult, moduleDir string, runtimeOptions RuntimeOptions, applyOverrides bool) (*Config, error) {
	for _, diagnostic := range result.Diagnostics {
		if strings.EqualFold(diagnostic.Level, "error") {
			return nil, fmt.Errorf("load package configuration: %s at %s: %s", diagnostic.Code, diagnostic.Path, diagnostic.Message)
		}
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
