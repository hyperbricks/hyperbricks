package shared

import (
	"fmt"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const DefaultSpacesRoute = "/__hyperbricks/spaces"

type SpacesDirectory struct {
	Base string `json:"base" mapstructure:"base"`
	Path string `json:"path" mapstructure:"path"`
}

type SpacesUploadPolicy struct {
	Accept    []string        `json:"accept" mapstructure:"accept"`
	MaxBytes  int64           `json:"max_bytes" mapstructure:"max_bytes"`
	Directory SpacesDirectory `json:"directory" mapstructure:"directory"`
}

type SpacesConfig struct {
	Route        string              `mapstructure:"route"`
	Write        bool                `mapstructure:"write"`
	PublicOrigin string              `mapstructure:"public_origin"`
	AllowedHosts []string            `mapstructure:"allowed_hosts"`
	SharingImage *SpacesUploadPolicy `mapstructure:"sharing_image"`
}

func DefaultFrontendEditingConfig() FrontendEditingConfig {
	return FrontendEditingConfig{Enabled: true, Spaces: SpacesConfig{Route: DefaultSpacesRoute}}
}

var frontendEditorRoutePattern = regexp.MustCompile(`^/__hyperbricks/[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)

func ValidFrontendEditorRoute(route string) bool {
	return path.Clean(route) == route && frontendEditorRoutePattern.MatchString(route) &&
		!strings.HasPrefix(route, "/__hyperbricks/render-diagnostics") &&
		route != "/__hyperbricks/errors" && !strings.HasPrefix(route, "/__hyperbricks/errors/")
}

func isSpacesPlugin(name string) bool {
	return name == "SpacesPlugin" || strings.HasPrefix(name, "SpacesPlugin@")
}

// ValidateFrontendEditing also reports errors retained by the config loader,
// which otherwise logs decode errors and continues with defaults.
func (c *Config) ValidateFrontendEditing() error {
	if c.frontendEditingError != nil {
		return c.frontendEditingError
	}
	for _, name := range c.Plugins.Enabled {
		if isSpacesPlugin(name) {
			return fmt.Errorf("Spaces is built in: %q is not supported in hyperbricks.plugins.enabled; configure hyperbricks.development.frontend_editing.spaces", name)
		}
	}
	return validateFrontendEditing(c.Development.FrontendEditing)
}

func validateFrontendEditing(config FrontendEditingConfig) error {
	if !ValidFrontendEditorRoute(config.Spaces.Route) {
		return fmt.Errorf("development.frontend_editing.spaces.route must be a non-reserved /__hyperbricks/ route")
	}
	keys := make([]string, 0, len(config.Editors))
	for key := range config.Editors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	routes := []string{config.Spaces.Route}
	for _, key := range keys {
		editor := config.Editors[key]
		if key == "spaces" || isSpacesPlugin(editor.Plugin) {
			return fmt.Errorf("Spaces is built in: development.frontend_editing.editors.%s cannot configure Spaces; use development.frontend_editing.spaces directly (without plugin/data)", key)
		}
		if editor.Plugin == "" || !ValidFrontendEditorRoute(editor.Route) {
			return fmt.Errorf("development.frontend_editing.editors.%s requires a plugin and a non-reserved /__hyperbricks/ route", key)
		}
		for _, route := range routes {
			if editor.Route == route || strings.HasPrefix(editor.Route, route+"/") || strings.HasPrefix(route, editor.Route+"/") {
				return fmt.Errorf("frontend editor routes overlap: %s and %s", route, editor.Route)
			}
		}
		routes = append(routes, editor.Route)
	}
	return nil
}

func decodeFrontendEditing(input interface{}) (FrontendEditingConfig, error) {
	config := DefaultFrontendEditingConfig()
	fail := func(err error) (FrontendEditingConfig, error) { return FrontendEditingConfig{}, err }
	root, _ := input.(map[string]interface{})
	if _, exists := root["frontend_editing"]; exists {
		return fail(fmt.Errorf("hyperbricks.frontend_editing is not supported; configure frontend editing under hyperbricks.development.frontend_editing"))
	}
	development, _ := root["development"].(map[string]interface{})
	if raw, exists := development["frontend_editing"]; exists {
		settings, ok := raw.(map[string]interface{})
		if !ok {
			return fail(fmt.Errorf("development.frontend_editing must be a mapping"))
		}
		if enabled, exists := settings["enabled"]; exists {
			if !frontendBoolean(enabled) {
				return fail(fmt.Errorf("development.frontend_editing.enabled must be a boolean"))
			}
		}
		if rawSpaces, exists := settings["spaces"]; exists {
			spaces, ok := rawSpaces.(map[string]interface{})
			if !ok {
				return fail(fmt.Errorf("development.frontend_editing.spaces must be a mapping"))
			}
			if write, exists := spaces["write"]; exists {
				if !frontendBoolean(write) {
					return fail(fmt.Errorf("development.frontend_editing.spaces.write must be a boolean"))
				}
			}
		}
		// The package parser materializes scalar booleans/numbers as strings.
		// Accept their canonical forms without weak coercion of arbitrary values.
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &config, TagName: "mapstructure", ErrorUnused: true, DecodeHook: func(from, to reflect.Type, value interface{}) (interface{}, error) {
			if from.Kind() == reflect.String {
				switch to.Kind() {
				case reflect.Bool:
					if !frontendBoolean(value) {
						return nil, fmt.Errorf("expected true or false")
					}
					return value == "true", nil
				case reflect.Int64:
					return strconv.ParseInt(value.(string), 10, 64)
				}
			}
			return value, nil
		}})
		if err != nil {
			return fail(err)
		}
		if err := decoder.Decode(settings); err != nil {
			return fail(fmt.Errorf("development.frontend_editing: %w", err))
		}
	}
	if err := validateFrontendEditing(config); err != nil {
		return fail(err)
	}
	return config, nil
}

func frontendBoolean(value interface{}) bool {
	if _, ok := value.(bool); ok {
		return true
	}
	text, ok := value.(string)
	return ok && (text == "true" || text == "false")
}
