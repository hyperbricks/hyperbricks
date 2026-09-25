package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

func writeStrictPackageConfig(t *testing.T, moduleDir, body string) string {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create module directory: %v", err)
	}
	path := filepath.Join(moduleDir, PackageConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write package configuration: %v", err)
	}
	return path
}

func preserveStrictConfigGlobals(t *testing.T) {
	t.Helper()
	previousInstance := instance
	previousModule := Module
	previousRuntimeOptions := GetRuntimeOptions()
	previousParserConfig := parser.HbConfig
	t.Cleanup(func() {
		instance = previousInstance
		Module = previousModule
		SetRuntimeOptions(previousRuntimeOptions)
		parser.HbConfig = previousParserConfig
	})
}

func TestLoadPackageConfigStrictUsesRuntimeDefaultsAndOverridesWithoutGlobalMutation(t *testing.T) {
	preserveStrictConfigGlobals(t)
	t.Setenv("HB_DEPLOY_PRODUCTION", "")
	t.Setenv("HB_PRODUCTION", "")

	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	configPath := writeStrictPackageConfig(t, moduleDir, `
hyperbricks:
  mode: development
  server:
    port: 7070
  development:
    dashboard:
      enabled: true
      credentials:
        user: developer
        password: secret
`)

	sentinelInstance := &Config{Mode: "sentinel"}
	sentinelParserConfig := map[string]interface{}{"sentinel": "unchanged"}
	runtimeOptions := RuntimeOptions{
		ModuleRoot:               "unrelated/global/module",
		Port:                     9099,
		PortOverride:             true,
		RuntimeGatewayEnabled:    true,
		RuntimeGatewayDomain:     " doctor.example.test ",
		RuntimeGatewayHostSuffix: " -doctor.example.test ",
		RuntimeGatewayResolver:   " http://resolver.example.test/resolve ",
	}
	instance = sentinelInstance
	Module = "sentinel/package.hyperbricks.yaml"
	parser.HbConfig = sentinelParserConfig
	SetRuntimeOptions(runtimeOptions)

	config, err := LoadPackageConfigStrict(configPath, moduleDir)
	if err != nil {
		t.Fatalf("LoadPackageConfigStrict() error = %v", err)
	}

	if config.Mode != DEVELOPMENT_MODE {
		t.Fatalf("mode = %q, want %q", config.Mode, DEVELOPMENT_MODE)
	}
	if config.Server.Port != 9099 {
		t.Fatalf("server.port = %d, want runtime override 9099", config.Server.Port)
	}
	if !config.Server.SelfClosingTags || !config.RateLimit.Enabled {
		t.Fatalf("runtime defaults were not applied: %#v", config)
	}
	if config.Live.CacheTime.Duration != 10*time.Minute {
		t.Fatalf("live.cache = %s, want 10m", config.Live.CacheTime.Duration)
	}
	if got, want := config.Directories["hyperbricks"], moduleDir+"/hyperbricks"; got != want {
		t.Fatalf("hyperbricks directory = %q, want %q", got, want)
	}
	if !config.Server.RuntimeGateway.Enabled || config.Server.RuntimeGateway.Domain != "doctor.example.test" ||
		config.Server.RuntimeGateway.HostSuffix != "-doctor.example.test" ||
		config.Server.RuntimeGateway.Resolver != "http://resolver.example.test/resolve" {
		t.Fatalf("runtime gateway overrides = %#v", config.Server.RuntimeGateway)
	}
	if !config.Development.Dashboard.Enabled || config.Development.Dashboard.Credentials.User != "developer" {
		t.Fatalf("dashboard = %#v", config.Development.Dashboard)
	}
	if !config.Development.FrontendEditing.Enabled || config.Development.FrontendEditing.Spaces.Route != DefaultSpacesRoute {
		t.Fatalf("frontend editing defaults = %#v", config.Development.FrontendEditing)
	}

	if instance != sentinelInstance {
		t.Fatal("strict load changed the configuration singleton")
	}
	if Module != "sentinel/package.hyperbricks.yaml" {
		t.Fatalf("Module = %q, want sentinel unchanged", Module)
	}
	if !reflect.DeepEqual(parser.HbConfig, sentinelParserConfig) {
		t.Fatalf("parser.HbConfig = %#v, want sentinel unchanged", parser.HbConfig)
	}
	if got := GetRuntimeOptions(); !reflect.DeepEqual(got, runtimeOptions) {
		t.Fatalf("runtime options = %#v, want %#v", got, runtimeOptions)
	}
}

func TestValidatePackageConfigBytesAcceptsDisabledLiveCacheWithoutRuntimeOverrides(t *testing.T) {
	preserveStrictConfigGlobals(t)
	t.Setenv("HB_DEPLOY_PRODUCTION", "1")
	t.Setenv("HB_PRODUCTION", "1")
	SetRuntimeOptions(RuntimeOptions{Production: true, ModeOverride: LIVE_MODE})

	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	config, err := ValidatePackageConfigBytes([]byte(`
hyperbricks:
  mode: development
  live:
    cache: 0s
`), moduleDir)
	if err != nil {
		t.Fatalf("ValidatePackageConfigBytes() error = %v", err)
	}
	if config.Mode != DEVELOPMENT_MODE || config.Live.CacheTime.Duration != 0 {
		t.Fatalf("validated config = mode %q cache %s", config.Mode, config.Live.CacheTime.Duration)
	}
}

func TestValidatePackageConfigBytesRejectsNegativeLiveCache(t *testing.T) {
	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	_, err := ValidatePackageConfigBytes([]byte(`
hyperbricks:
  mode: live
  live:
    cache: -1s
`), moduleDir)
	if err == nil || !strings.Contains(err.Error(), "live.cache") {
		t.Fatalf("negative cache error = %v", err)
	}
}

func TestLoadPackageConfigStrictProductionOverride(t *testing.T) {
	preserveStrictConfigGlobals(t)
	t.Setenv("HB_DEPLOY_PRODUCTION", "")
	t.Setenv("HB_PRODUCTION", "")

	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	configPath := writeStrictPackageConfig(t, moduleDir, "hyperbricks: {mode: development}\n")
	SetRuntimeOptions(RuntimeOptions{Production: true})

	config, err := LoadPackageConfigStrict(configPath, moduleDir)
	if err != nil {
		t.Fatalf("LoadPackageConfigStrict() error = %v", err)
	}
	if config.Mode != LIVE_MODE {
		t.Fatalf("mode = %q, want production override %q", config.Mode, LIVE_MODE)
	}
}

func TestLoadPackageConfigStrictExplicitModeOverridesLegacyProductionSignals(t *testing.T) {
	preserveStrictConfigGlobals(t)
	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	configPath := writeStrictPackageConfig(t, moduleDir, "hyperbricks: {mode: live}\n")

	for _, tt := range []struct {
		name              string
		options           RuntimeOptions
		environmentMode   string
		deployProduction  string
		genericProduction string
		want              string
	}{
		{
			name:              "command development beats production option and environment",
			options:           RuntimeOptions{Production: true, ModeOverride: DEVELOPMENT_MODE},
			deployProduction:  "1",
			genericProduction: "1",
			want:              DEVELOPMENT_MODE,
		},
		{
			name:              "environment development beats legacy environments",
			environmentMode:   DEVELOPMENT_MODE,
			deployProduction:  "1",
			genericProduction: "1",
			want:              DEVELOPMENT_MODE,
		},
		{
			name:            "command live beats development environment",
			options:         RuntimeOptions{ModeOverride: LIVE_MODE},
			environmentMode: DEVELOPMENT_MODE,
			want:            LIVE_MODE,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HB_DEPLOY_RUNTIME_MODE", tt.environmentMode)
			t.Setenv("HB_DEPLOY_PRODUCTION", tt.deployProduction)
			t.Setenv("HB_PRODUCTION", tt.genericProduction)
			SetRuntimeOptions(tt.options)

			config, err := LoadPackageConfigStrict(configPath, moduleDir)
			if err != nil {
				t.Fatalf("LoadPackageConfigStrict() error = %v", err)
			}
			if config.Mode != tt.want {
				t.Fatalf("mode = %q, want %q", config.Mode, tt.want)
			}
		})
	}
}

func TestLoadPackageConfigStrictReturnsConfigurationErrors(t *testing.T) {
	preserveStrictConfigGlobals(t)
	t.Setenv("HB_DEPLOY_PRODUCTION", "")
	t.Setenv("HB_PRODUCTION", "")
	SetRuntimeOptions(RuntimeOptions{Production: true})
	const missingEnvironment = "HB_STRICT_REQUIRED_ENV_MUST_NOT_EXIST"
	previousEnvironment, environmentExisted := os.LookupEnv(missingEnvironment)
	if err := os.Unsetenv(missingEnvironment); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if environmentExisted {
			_ = os.Setenv(missingEnvironment, previousEnvironment)
		} else {
			_ = os.Unsetenv(missingEnvironment)
		}
	})

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "yaml", body: "hyperbricks: [\n", want: "load package configuration"},
		{name: "typed decode", body: "hyperbricks: {server: {port: invalid}}\n", want: "decode package configuration"},
		{name: "dashboard", body: "hyperbricks: {development: {dashboard: true}}\n", want: "development.dashboard must be a mapping"},
		{name: "frontend editing", body: "hyperbricks: {development: {frontend_editing: {spaces: {route: /public/spaces}}}}\n", want: "non-reserved /__hyperbricks/ route"},
		{name: "spaces plugin conflict", body: "hyperbricks: {plugins: {enabled: [SpacesPlugin@0.1.0]}}\n", want: "Spaces is built in"},
		{name: "invalid mode despite production override", body: "hyperbricks: {mode: preview}\n", want: "invalid hyperbricks.mode \"preview\""},
		{name: "invalid cache duration", body: "hyperbricks: {live: {cache: eventually}}\n", want: "invalid cache duration"},
		{name: "invalid duration", body: "hyperbricks: {system: {metrics_watch_interval: eventually}}\n", want: "invalid duration"},
		{name: "nonpositive metrics interval", body: "hyperbricks: {system: {metrics_watch_interval: 0s}}\n", want: "metrics_watch_interval must be greater than zero"},
		{name: "invalid gomaxprocs", body: "hyperbricks: {server: {gomaxprocs: 0}}\n", want: "server.gomaxprocs must be auto"},
		{name: "invalid runtime gateway", body: "hyperbricks: {server: {runtime_gateway: {enabled: true, domain: example.test}}}\n", want: "runtime gateway requires runtime resolver"},
		{name: "required package environment", body: "hyperbricks: {system: {metrics_watch_interval: {env: {name: " + missingEnvironment + ", required: true}}}}\n", want: "env_missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
			configPath := writeStrictPackageConfig(t, moduleDir, tt.body)
			config, err := LoadPackageConfigStrict(configPath, moduleDir)
			if err == nil {
				t.Fatalf("LoadPackageConfigStrict() config = %#v, want error containing %q", config, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadPackageConfigStrict() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestRuntimeLoaderStillRecoversInvalidDurations(t *testing.T) {
	Init_configuration()
	resetConfigurationForTest(t)
	t.Setenv("HB_DEPLOY_PRODUCTION", "")
	t.Setenv("HB_PRODUCTION", "")

	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	configPath := writeStrictPackageConfig(t, moduleDir, `
hyperbricks:
  live:
    cache: eventually
  system:
    metrics_watch_interval: eventually
`)
	Module = configPath
	SetRuntimeOptions(RuntimeOptions{ModuleRoot: moduleDir})

	config := GetHyperBricksConfiguration()
	if config.Live.CacheTime.Duration != 24*time.Hour {
		t.Fatalf("runtime cache fallback = %s, want 24h", config.Live.CacheTime.Duration)
	}
	if config.System.MetricsWatchInterval != 0 {
		t.Fatalf("runtime metrics fallback = %s, want zero", config.System.MetricsWatchInterval)
	}
}

func TestRuntimeLoaderStillFallsBackToLiveForInvalidMode(t *testing.T) {
	Init_configuration()
	resetConfigurationForTest(t)
	t.Setenv("HB_DEPLOY_PRODUCTION", "")
	t.Setenv("HB_PRODUCTION", "")

	moduleDir := filepath.Join(t.TempDir(), "modules", "demo")
	configPath := writeStrictPackageConfig(t, moduleDir, "hyperbricks: {mode: preview}\n")
	Module = configPath
	SetRuntimeOptions(RuntimeOptions{ModuleRoot: moduleDir})

	config := GetHyperBricksConfiguration()
	if config.Mode != LIVE_MODE {
		t.Fatalf("runtime mode = %q, want fallback %q", config.Mode, LIVE_MODE)
	}
}
