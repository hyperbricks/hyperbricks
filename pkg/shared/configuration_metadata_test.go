package shared

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
)

// Package metadata follows the component description/example convention without
// adding documentation-only fields to the runtime decoding contract.
func TestPackageConfigurationFieldMetadata(t *testing.T) {
	visited := map[reflect.Type]bool{}
	count := 0
	var walk func(reflect.Type, string)
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Map || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || typ == reflect.TypeOf(CacheTime{}) || typ == reflect.TypeOf(time.Duration(0)) || visited[typ] {
			return
		}
		visited[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			key := field.Tag.Get("mapstructure")
			if key == "" || strings.Contains(key, ",") || strings.HasPrefix(key, "@") {
				t.Errorf("%s.%s: unexpected package field mapping %q", path, field.Name, key)
				continue
			}
			fieldPath := path + "." + key
			if strings.TrimSpace(field.Tag.Get("description")) == "" {
				t.Errorf("%s: missing description", fieldPath)
			}
			example := field.Tag.Get("example")
			if example == "" {
				t.Errorf("%s: missing example", fieldPath)
			}
			var node yaml.Node
			if err := yaml.Unmarshal([]byte(example), &node); err != nil {
				t.Errorf("%s: invalid YAML example: %v", fieldPath, err)
			}
			count++
			walk(field.Type, fieldPath)
		}
	}
	walk(reflect.TypeOf(Config{}), "hyperbricks")
	for _, typ := range []reflect.Type{reflect.TypeOf(SpacesConfig{}), reflect.TypeOf(SpacesUploadPolicy{}), reflect.TypeOf(FrontendEditorConfig{}), reflect.TypeOf(CredentialsConfig{})} {
		if !visited[typ] {
			t.Errorf("metadata traversal missed %s", typ)
		}
	}
	if visited[reflect.TypeOf(DeployConfig{})] {
		t.Fatal("deployment-service configuration must remain separate")
	}
	t.Logf("Checked descriptions and YAML examples for %d package configuration fields", count)
}

func TestPackageConfigurationSectionExamplesUseRuntimeValidation(t *testing.T) {
	t.Setenv("HB_DEVELOPER_USER", "metadata-test-user")
	t.Setenv("HB_DEVELOPER_PASSWORD", "metadata-test-password")
	// Section examples are configuration, not component-rendering fixtures.
	// Read-only validation must accept them without opening resources or services.
	for _, typ := range []struct {
		value any
		path  []string
	}{
		{Config{}, nil},
		{ServerConfig{}, []string{"server"}},
		{DevelopmentConfig{}, []string{"development"}},
		{DevelopmentDashboardConfig{}, []string{"development", "dashboard"}},
		{FrontendEditingConfig{}, []string{"development", "frontend_editing"}},
		{SpacesConfig{}, []string{"development", "frontend_editing", "spaces"}},
	} {
		reflected := reflect.TypeOf(typ.value)
		for i := 0; i < reflected.NumField(); i++ {
			field := reflected.Field(i)
			if !field.IsExported() {
				continue
			}
			t.Run(reflected.Name()+"/"+field.Name, func(t *testing.T) {
				var value any
				if err := yaml.Unmarshal([]byte(field.Tag.Get("example")), &value); err != nil {
					t.Fatal(err)
				}
				settings := map[string]any{field.Tag.Get("mapstructure"): value}
				for j := len(typ.path) - 1; j >= 0; j-- {
					settings = map[string]any{typ.path[j]: settings}
				}
				input, err := yaml.Marshal(map[string]any{"hyperbricks": settings})
				if err != nil {
					t.Fatal(err)
				}
				_, err = ValidatePackageConfigBytesWithResourceReader(input, t.TempDir(), func(path string) ([]byte, error) {
					t.Fatalf("metadata example unexpectedly read resource %s", path)
					return nil, nil
				})
				if err != nil {
					t.Fatalf("example rejected by configuration validation: %v\n%s", err, input)
				}
			})
		}
	}
}
