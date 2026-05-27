package typefactory

import (
	"reflect"
	"testing"
)

type factoryTestConfig struct {
	Name       string            `mapstructure:"name"`
	Count      int               `mapstructure:"count"`
	Classes    []string          `mapstructure:"classes"`
	Attributes map[string]string `mapstructure:"attributes"`
}

func (config factoryTestConfig) Validate() []string {
	if config.Name == "" {
		return []string{"name is empty"}
	}
	return nil
}

func TestCreateInstanceDecodesRegisteredTypeWithHooks(t *testing.T) {
	factory := NewTypeFactory()
	factory.RegisterType("<TEST>", reflect.TypeOf(factoryTestConfig{}))

	response, err := factory.CreateInstance(TypeRequest{
		TypeName: "<TEST>",
		Data: map[string]interface{}{
			"name":       "demo",
			"count":      "42",
			"classes":    "hero",
			"attributes": "",
		},
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	config, ok := response.Instance.(factoryTestConfig)
	if !ok {
		t.Fatalf("expected factoryTestConfig, got %T", response.Instance)
	}
	if config.Name != "demo" {
		t.Fatalf("expected name to decode, got %q", config.Name)
	}
	if config.Count != 42 {
		t.Fatalf("expected string count to decode to int, got %d", config.Count)
	}
	if !reflect.DeepEqual(config.Classes, []string{"hero"}) {
		t.Fatalf("expected string class to decode to []string, got %#v", config.Classes)
	}
	if len(config.Attributes) != 0 {
		t.Fatalf("expected string map input to decode to empty map, got %#v", config.Attributes)
	}
	if len(response.Warnings) != 0 {
		t.Fatalf("did not expect validation warnings, got %#v", response.Warnings)
	}
}

func TestCreateInstanceReturnsValidationWarnings(t *testing.T) {
	factory := NewTypeFactory()
	factory.RegisterType("<TEST>", reflect.TypeOf(factoryTestConfig{}))

	response, err := factory.CreateInstance(TypeRequest{
		TypeName: "<TEST>",
		Data:     map[string]interface{}{"count": "not-a-number"},
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	config := response.Instance.(factoryTestConfig)
	if config.Count != 0 {
		t.Fatalf("invalid string int input should decode to zero, got %d", config.Count)
	}
	if !reflect.DeepEqual(response.Warnings, []string{"name is empty"}) {
		t.Fatalf("expected validation warning, got %#v", response.Warnings)
	}
}

func TestCreateInstanceRejectsUnregisteredType(t *testing.T) {
	factory := NewTypeFactory()

	response, err := factory.CreateInstance(TypeRequest{TypeName: "<MISSING>"})
	if err == nil {
		t.Fatal("expected unregistered type error")
	}
	if response != nil {
		t.Fatalf("expected nil response for missing type, got %#v", response)
	}
}
