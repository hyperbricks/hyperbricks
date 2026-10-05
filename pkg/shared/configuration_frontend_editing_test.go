package shared

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpacesAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, mode                                          string
		production, disabled, spacesDisabled, invalid, want bool
	}{
		{name: "development", mode: DEVELOPMENT_MODE, want: true},
		{name: "debug", mode: DEBUG_MODE, want: true},
		{name: "live", mode: LIVE_MODE},
		{name: "unknown mode", mode: "unknown"},
		{name: "production development", mode: DEVELOPMENT_MODE, production: true},
		{name: "production debug", mode: DEBUG_MODE, production: true},
		{name: "parent disabled", mode: DEVELOPMENT_MODE, disabled: true},
		{name: "Spaces disabled", mode: DEBUG_MODE, spacesDisabled: true},
		{name: "invalid frontend configuration", mode: DEVELOPMENT_MODE, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := Config{Mode: tc.mode}
			config.Development.FrontendEditing = DefaultFrontendEditingConfig()
			config.Development.FrontendEditing.Enabled = !tc.disabled
			config.Development.FrontendEditing.Spaces.Enabled = !tc.spacesDisabled
			if tc.invalid {
				config.frontendEditingError = errors.New("invalid frontend configuration")
			}
			if got := SpacesAvailable(&config, RuntimeOptions{Production: tc.production}); got != tc.want {
				t.Fatalf("available = %v, want %v", got, tc.want)
			}
		})
	}
	config := Config{Mode: DEVELOPMENT_MODE, dashboardConfigError: errors.New("invalid dashboard configuration")}
	config.Development.FrontendEditing = DefaultFrontendEditingConfig()
	if SpacesAvailable(&config, RuntimeOptions{}) || SpacesAvailable(nil, RuntimeOptions{}) {
		t.Fatal("invalid configuration exposed Spaces")
	}
}

func frontendConfig(t *testing.T, body string, config *Config) error {
	t.Helper()
	dir := t.TempDir()
	writePackageConfig(t, dir, "", body)
	parsed, err := LoadPackageConfigMap(filepath.Join(dir, PackageConfigFileName), dir)
	if err != nil {
		return err
	}
	if err := decodeConfig(parsed["hyperbricks"], config); err != nil {
		return err
	}
	return config.ValidateFrontendEditing()
}

func TestFrontendEditingDefaultsAndSequentialDecode(t *testing.T) {
	Init_configuration()
	var config Config
	for _, tc := range []struct {
		yaml                          string
		enabled, spacesEnabled, write bool
		route                         string
	}{
		{"hyperbricks: {mode: development}", true, true, true, DefaultSpacesRoute},
		{"hyperbricks: {development: {frontend_editing: {spaces: {write: true, route: /__hyperbricks/content, allowed_hosts: [192.168.2.55]}}}}", true, true, true, "/__hyperbricks/content"},
		{"hyperbricks: {development: {frontend_editing: {spaces: {enabled: false, write: true}}}}", true, false, true, DefaultSpacesRoute},
		{"hyperbricks: {development: {frontend_editing: {enabled: false}}}", false, true, true, DefaultSpacesRoute},
		{"hyperbricks: {mode: development}", true, true, true, DefaultSpacesRoute},
		{"hyperbricks: {development: {frontend_editing: {spaces: {write: false}}}}", true, true, false, DefaultSpacesRoute},
		{"hyperbricks: {mode: debug}", true, true, true, DefaultSpacesRoute},
	} {
		if err := frontendConfig(t, tc.yaml, &config); err != nil {
			t.Fatal(err)
		}
		got := config.Development.FrontendEditing
		if got.Enabled != tc.enabled || got.Spaces.Enabled != tc.spacesEnabled || got.Spaces.Write != tc.write || got.Spaces.Route != tc.route {
			t.Fatalf("unexpected config: %+v", got)
		}
		if tc.route == DefaultSpacesRoute && len(got.Spaces.AllowedHosts) != 0 {
			t.Fatal("host policy leaked from previous config")
		}
	}
}

func TestFrontendEditingStrictConfigurationAndRoutes(t *testing.T) {
	Init_configuration()
	for _, tc := range []struct{ name, yaml, message string }{
		{"top-level-disabled", "hyperbricks: {frontend_editing: {enabled: false}}", "hyperbricks.frontend_editing is not supported"},
		{"both-locations", "hyperbricks: {frontend_editing: {}, development: {frontend_editing: {spaces: {write: true}}}}", "hyperbricks.frontend_editing is not supported"},
		{"spaces-plugin", "hyperbricks: {plugins: {enabled: [SpacesPlugin@0.1.0]}}", "not supported in hyperbricks.plugins.enabled"},
		{"spaces-editor-selector", "hyperbricks: {development: {frontend_editing: {editors: {custom: {plugin: SpacesPlugin@0.1.0, route: /__hyperbricks/custom}}}}}", "built in"},
		{"spaces-data-wrapper", "hyperbricks: {development: {frontend_editing: {spaces: {data: {write: true}}}}}", "invalid keys"},
		{"unknown", "hyperbricks: {development: {frontend_editing: {spaces: {allowed_host: [example.com]}}}}", "invalid keys"},
		{"enabled-ambiguous", "hyperbricks: {development: {frontend_editing: {enabled: 'no'}}}", "boolean"},
		{"enabled-null", "hyperbricks: {development: {frontend_editing: {enabled: null}}}", "boolean"},
		{"spaces-enabled-ambiguous", "hyperbricks: {development: {frontend_editing: {spaces: {enabled: 'no'}}}}", "spaces.enabled must be a boolean"},
		{"spaces-enabled-null", "hyperbricks: {development: {frontend_editing: {spaces: {enabled: null}}}}", "spaces.enabled must be a boolean"},
		{"write-number", "hyperbricks: {development: {frontend_editing: {spaces: {write: 1}}}}", "boolean"},
		{"write-null", "hyperbricks: {development: {frontend_editing: {spaces: {write: null}}}}", "boolean"},
		{"null", "hyperbricks: {development: {frontend_editing: null}}", "mapping"},
		{"spaces-null", "hyperbricks: {development: {frontend_editing: {spaces: null}}}", "mapping"},
		{"reserved", "hyperbricks: {development: {frontend_editing: {spaces: {route: /__hyperbricks/render-diagnostics}}}}", "non-reserved"},
		{"errors-reserved", "hyperbricks: {development: {frontend_editing: {spaces: {route: /__hyperbricks/errors}}}}", "non-reserved"},
		{"errors-child-reserved", "hyperbricks: {development: {frontend_editing: {editors: {other: {plugin: Other@1, route: /__hyperbricks/errors/web}}}}}", "non-reserved"},
		{"trailing-slash", "hyperbricks: {development: {frontend_editing: {spaces: {route: /__hyperbricks/spaces/}}}}", "non-reserved"},
		{"same-route", "hyperbricks: {development: {frontend_editing: {editors: {other: {plugin: Other@1, route: /__hyperbricks/spaces}}}}}", "overlap"},
		{"child-route", "hyperbricks: {development: {frontend_editing: {editors: {other: {plugin: Other@1, route: /__hyperbricks/spaces/other}}}}}", "overlap"},
		{"external-overlap", "hyperbricks: {development: {frontend_editing: {editors: {one: {plugin: One@1, route: /__hyperbricks/other}, two: {plugin: Two@1, route: /__hyperbricks/other/sub}}}}}", "overlap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var config Config
			err := frontendConfig(t, tc.yaml, &config)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error = %v, expected %q", err, tc.message)
			}
			if config.ValidateFrontendEditing() == nil {
				t.Fatal("invalid editor configuration was not retained for startup validation")
			}
		})
	}
}

func TestFrontendEditingOptionalPoliciesAndExternalEditors(t *testing.T) {
	Init_configuration()
	var config Config
	err := frontendConfig(t, `hyperbricks:
  plugins:
    enabled: [OtherEditor@1.0.0]
  development:
    frontend_editing:
      spaces:
        write: true
        public_origin: https://example.com
        allowed_hosts: [192.168.2.55]
        sharing_image:
          accept: [.png, .jpg]
          max_bytes: 5242880
          directory: {base: static, path: uploads/images}
      editors:
        other:
          plugin: OtherEditor@1.0.0
          route: /__hyperbricks/other
          data: {custom: value}
`, &config)
	if err != nil {
		t.Fatal(err)
	}
	spaces := config.Development.FrontendEditing.Spaces
	if spaces.SharingImage == nil || spaces.SharingImage.MaxBytes != 5242880 || spaces.SharingImage.Directory.Path != "uploads/images" || spaces.PublicOrigin != "https://example.com" || len(spaces.AllowedHosts) != 1 {
		t.Fatalf("lost policy: %+v", spaces)
	}
	if config.Development.FrontendEditing.Editors["other"].Data["custom"] != "value" {
		t.Fatal("lost external editor data")
	}
}
