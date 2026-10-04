package typefactory_test

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

func TestAPIResponseStatusFactoryPreservesStrictTypes(t *testing.T) {
	factory := typefactory.NewTypeFactory()
	factory.RegisterType("api", reflect.TypeOf(component.APIConfig{}))
	factory.RegisterType("api_fragment", reflect.TypeOf(composite.ApiFragmentRenderConfig{}))
	for _, kind := range []string{"api", "api_fragment"} {
		t.Run(kind, func(t *testing.T) {
			for name, raw := range map[string]interface{}{
				"enabled": "false", "required": "true", "priority": "100", "unknown": true,
			} {
				if _, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: kind, Data: map[string]interface{}{
					"response_status": map[string]interface{}{name: raw},
				}}); err == nil {
					t.Errorf("weak decoding accepted malformed %s: %#v", name, raw)
				}
			}
			for _, raw := range []map[string]interface{}{
				{},
				{"response_status": map[string]interface{}{"enabled": false}},
				{"response_status": map[string]interface{}{"required": true, "priority": 100, "map": map[string]interface{}{"404": 404, "409": "ignore"}}},
			} {
				result, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: kind, Data: raw})
				if err != nil {
					t.Fatal(err)
				}
				var policy *shared.ResponseStatusConfig
				switch instance := result.Instance.(type) {
				case component.APIConfig:
					policy = instance.ResponseStatus
				case composite.ApiFragmentRenderConfig:
					policy = instance.ResponseStatus
				}
				if raw["response_status"] == nil {
					if policy != nil {
						t.Fatalf("absent policy decoded as %#v", policy)
					}
					continue
				}
				if policy == nil {
					t.Fatal("explicit policy was lost")
				}
				if policy.Enabled != nil {
					if *policy.Enabled || policy.Active() {
						t.Fatalf("explicit false enabled policy: %#v", policy)
					}
					continue
				}
				if !policy.Required || policy.Priority != 100 || policy.Map["404"] != 404 || policy.Map["409"] != "ignore" {
					t.Fatalf("factory changed policy types: %#v", policy)
				}
			}
		})
	}
}
