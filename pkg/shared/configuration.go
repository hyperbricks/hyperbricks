package shared

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/parser"

	"github.com/mitchellh/mapstructure"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.SugaredLogger
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
	return logger
}

func Init_configuration() {
	// Create a custom configuration for the logger
	config := zap.NewProductionConfig()

	// Set the logging level to ERROR
	config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)

	// Build the logger
	l, err := config.Build()
	if err != nil {
		panic(err)
	}

	defer l.Sync() // Ensure the logger is flushed on exit

	// Use the configured logger
	logger = l.Sugar()
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

// Config structure with default values.
type Config struct {
	Mode        string            `mapstructure:"mode"`
	Logger      LoggerConfig      `mapstructure:"logger"`
	Server      ServerConfig      `mapstructure:"server"`
	RateLimit   RateLimitConfig   `mapstructure:"rate_limit"`
	Development DevelopmentConfig `mapstructure:"development"`
	Debug       DebugConfig       `mapstructure:"debug"`
	Live        LiveConfig        `mapstructure:"live"`
	Deploy      DeployConfig      `mapstructure:"deploy"`
	Directories map[string]string `mapstructure:"directories"`
	Plugins     PluginsConfig     `mapstructure:"plugins"`
	System      SystemConfig      `mapstructure:"system"`
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
	Dashboard      bool `mapstructure:"dashboard"`
	FrontendErrors bool `mapstructure:"frontend_errors"`
	Watch          bool `mapstructure:"watch"`
	Reload         bool `mapstructure:"reload"`
}

// LoggerConfig with defaults.
type LoggerConfig struct {
	Level string `mapstructure:"level"`
	Path  string `mapstructure:"path"`
}

type RoutingConfig struct {
	CleanURLs  bool     `mapstructure:"clean_urls"`
	IndexFiles []string `mapstructure:"index_files"`
	Extensions []string `mapstructure:"extensions"`
}

// ServerConfig with defaults.
type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	Beautify        bool          `mapstructure:"beautify"`
	SelfClosingTags bool          `mapstructure:"self_closing_tags"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	Routing         RoutingConfig `mapstructure:"routing"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `mapstructure:"requests_per_second"`
	Burst             int `mapstructure:"burst"`
}

type DeployConfig struct {
	HMACSecret string             `mapstructure:"hmac_secret"`
	Remote     DeployRemoteConfig `mapstructure:"remote"`
	Local      DeployLocalConfig  `mapstructure:"local"`
	Client     DeployClientConfig `mapstructure:"client"`
}

type DeployRemoteConfig struct {
	APIEnabled  bool   `mapstructure:"api_enabled"`
	APIBind     string `mapstructure:"api_bind"`
	APIPort     int    `mapstructure:"api_port"`
	Root        string `mapstructure:"root"`
	PortStart   int    `mapstructure:"port_start"`
	LogsEnabled bool   `mapstructure:"logs_enabled"`
	Binary      string `mapstructure:"binary"`
}

type DeployLocalConfig struct {
	Bind       string `mapstructure:"bind"`
	Port       int    `mapstructure:"port"`
	ModulesDir string `mapstructure:"modules_dir"`
	BuildRoot  string `mapstructure:"build_root"`
}

type DeployClientConfig struct {
	Target  string                        `mapstructure:"target"`
	Targets map[string]DeployClientTarget `mapstructure:"targets"`
}

type DeployClientTarget struct {
	Host string `mapstructure:"host"`
	User string `mapstructure:"user"`
	Port int    `mapstructure:"port"`
	Root string `mapstructure:"root"`
	API  string `mapstructure:"api"`
}

var (
	instance *Config
	once     sync.Once
	Module   string
)

// GetHyperBricksConfiguration returns the singleton instance of the Config.
func GetHyperBricksConfiguration() *Config {
	once.Do(func() {
		flag.Parse()
		instance = loadHyperBricksConfiguration()
	})
	return instance
}

// loadHyperBricksConfiguration initializes the Config object with defaults and decodes the config file.
func loadHyperBricksConfiguration() *Config {
	dir, err := os.Getwd()
	if err != nil {
		GetLogger().Errorf("Failed to get working directory", "error", err)
	}
	fmt.Println("loading " + Module)
	configFilePath := filepath.Join(dir, Module)

	// Read the configuration file
	configContent, err := os.ReadFile(configFilePath)
	if err != nil {
		GetLogger().Info("Failed to read config file", "path", configFilePath, "error", err)
	}

	moduleDir := filepath.ToSlash(commands.GetModuleRoot())
	rootPattern := regexp.MustCompile(`{{MODULE_PATH}}`)
	_config := rootPattern.ReplaceAllString(string(configContent), moduleDir)
	if strings.TrimSpace(moduleDir) != "" {
		_config = applyModuleRootOverride(_config, moduleDir)
	}

	// Parse the configuration file content
	parsedConfig := parser.ParseHyperScript(_config)
	parser.HbConfig = parsedConfig
	GetLogger().Infof("Parsed Configuration %v", parsedConfig)

	// Initialize with default values
	var config = Config{

		Mode: LIVE_MODE, // Default mode

		Logger: LoggerConfig{
			Level: "info",                   // Default level
			Path:  "./logs/hyperbricks.log", // Default path
		},

		Server: ServerConfig{
			Port: 8080, // Default port
			// Default to XHTML-friendly tags when not configured.
			SelfClosingTags: true,

			// Default Low traffic (~50-500 daily visitors).
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  20 * time.Second,

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
		Deploy: DeployConfig{
			Remote: DeployRemoteConfig{
				APIEnabled:  true,
				APIBind:     "127.0.0.1",
				APIPort:     9090,
				Root:        "deploy",
				PortStart:   8080,
				LogsEnabled: true,
			},
			Local: DeployLocalConfig{
				Bind:       "127.0.0.1",
				Port:       9091,
				ModulesDir: "modules",
				BuildRoot:  "deploy",
			},
		},
	}

	// Decode the parsed config into the struct
	err = decodeConfig(parsedConfig["hyperbricks"], &config)
	if err != nil {
		GetLogger().Errorf("Failed to decode configuration", "error", err)
	}
	if int(commands.Port) != 8080 {
		config.Server.Port = int(commands.Port)
	}
	if commands.Production || envTrue("HB_DEPLOY_PRODUCTION") || envTrue("HB_PRODUCTION") {
		config.Mode = LIVE_MODE
	}

	// Validate mode
	if config.Mode == LIVE_MODE {
		GetLogger().Debug("Setting mode to live (production) mode")
	} else if config.Mode == DEVELOPMENT_MODE {
		GetLogger().Debug("Setting mode to development mode")
	} else if config.Mode == DEBUG_MODE {
		GetLogger().Debug("Setting mode to debug mode")
		GetLogger().Debugf("Final Configuration", "config", config)
	} else {
		GetLogger().Debugf("Invalid mode set in package.hyperbricks %v", config.Mode)

		GetLogger().Warn("Setting mode not recognised, setting to live (production) mode")
		config.Mode = LIVE_MODE
	}

	return &config
}

// decodeConfig decodes map to struct with defaults using mapstructure.
func decodeConfig(input interface{}, output interface{}) error {
	var fallback CacheTime
	err := fallback.Parse("24h") // Fallback duration
	if err != nil {
		GetLogger().Errorf("Failed to set fallback CacheTime", "error", err)
	}

	decodeHook := mapstructure.ComposeDecodeHookFunc(
		// Decode CacheTime
		func(srcType reflect.Type, destType reflect.Type, value interface{}) (interface{}, error) {
			if srcType.Kind() == reflect.String && destType == reflect.TypeOf(CacheTime{}) {
				var ct CacheTime
				err := ct.Parse(value.(string))
				if err != nil {
					GetLogger().Errorf("Failed to parse CacheTime", "value", value, "error", err)
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
					GetLogger().Errorf("Failed to parse time.Duration", "value", value, "error", err)
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

func applyModuleRootOverride(content string, moduleRoot string) string {
	lines := strings.Split(content, "\n")
	replaced := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "$module") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) != "$module" {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = fmt.Sprintf("%s$module = %s", indent, moduleRoot)
		replaced = true
	}

	if !replaced {
		lines = append([]string{fmt.Sprintf("$module = %s", moduleRoot), ""}, lines...)
	}

	return strings.Join(lines, "\n")
}
