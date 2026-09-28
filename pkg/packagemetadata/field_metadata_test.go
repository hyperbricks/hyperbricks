package packagemetadata

import (
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestPackageMetadataFieldDescriptions(t *testing.T) {
	keys := map[string]bool{}
	for _, typ := range []reflect.Type{reflect.TypeOf(SourceMetadata{}), reflect.TypeOf(ArtifactMetadata{})} {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				if field.Tag.Get("mapstructure") != ",squash" {
					t.Fatal("source metadata must retain its flat mapping")
				}
				continue
			}
			key := field.Tag.Get("mapstructure")
			if key == "" || strings.TrimSpace(field.Tag.Get("description")) == "" || field.Tag.Get("example") == "" {
				t.Errorf("%s.%s lacks field metadata", typ, field.Name)
			}
			var example string
			if err := yaml.Unmarshal([]byte(field.Tag.Get("example")), &example); err != nil {
				t.Errorf("%s example is not a string value: %v", key, err)
			}
			keys[key] = true
		}
	}
	for _, key := range artifactFieldOrder {
		if !keys[key] {
			t.Errorf("package metadata field %s lacks metadata", key)
		}
	}
}
