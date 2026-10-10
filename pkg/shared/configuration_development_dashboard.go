package shared

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// ValidateDevelopmentDashboard reports schema errors retained by the package
// configuration loader. Missing credentials are valid configuration; each
// developer interface applies its own policy for an unconfigured account.
func (c *Config) ValidateDevelopmentDashboard() error {
	return c.dashboardConfigError
}

// LegacyWarning describes the supported Boolean shorthand without making it a
// configuration error. Consumers choose where to display the migration advice.
func (c DevelopmentDashboardConfig) LegacyWarning() string {
	if !c.legacyBoolean {
		return ""
	}
	return fmt.Sprintf("development.dashboard: %t uses the legacy Boolean form; use development.dashboard.enabled: %t instead", c.Enabled, c.Enabled)
}

func decodeDevelopmentDashboard(input interface{}) (DevelopmentDashboardConfig, error) {
	var config DevelopmentDashboardConfig
	root, _ := input.(map[string]interface{})
	development, _ := root["development"].(map[string]interface{})
	raw, exists := development["dashboard"]
	if !exists {
		return config, nil
	}
	if frontendBoolean(raw) {
		return DevelopmentDashboardConfig{Enabled: raw == true || raw == "true", legacyBoolean: true}, nil
	}
	settings, ok := raw.(map[string]interface{})
	if !ok {
		return config, fmt.Errorf("development.dashboard must be a mapping with enabled and credentials, or a legacy Boolean")
	}
	if enabled, exists := settings["enabled"]; exists && !frontendBoolean(enabled) {
		return config, fmt.Errorf("development.dashboard.enabled must be a boolean")
	}
	if rawCredentials, exists := settings["credentials"]; exists {
		credentials, ok := rawCredentials.(map[string]interface{})
		if !ok {
			return config, fmt.Errorf("development.dashboard.credentials must be a mapping")
		}
		if _, exists := credentials["pass"]; exists {
			return config, fmt.Errorf("obsolete development.dashboard.credentials.pass; use development.dashboard.credentials.password")
		}
		for _, field := range []string{"user", "password"} {
			if value, exists := credentials[field]; exists {
				if _, ok := value.(string); !ok {
					return config, fmt.Errorf("development.dashboard.credentials.%s must resolve to a string", field)
				}
			}
		}
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      &config,
		TagName:     "mapstructure",
		ErrorUnused: true,
		DecodeHook: func(from, to reflect.Type, value interface{}) (interface{}, error) {
			if from.Kind() == reflect.String && to.Kind() == reflect.Bool {
				if !frontendBoolean(value) {
					return nil, fmt.Errorf("expected true or false")
				}
				return value == "true", nil
			}
			return value, nil
		},
	})
	if err != nil {
		return config, err
	}
	if err := decoder.Decode(settings); err != nil {
		return DevelopmentDashboardConfig{}, fmt.Errorf("development.dashboard: %w", err)
	}
	return config, nil
}
