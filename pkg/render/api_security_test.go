package render_test

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// Exercise the real factory used by both compiled plans and interpreted
// rendering. Security-field validation must precede weak string conversion.
func TestAPIComponentForwardTokenRawTypes(t *testing.T) {
	factory := typefactory.NewTypeFactory()
	factory.RegisterType("api", reflect.TypeOf(component.APIConfig{}))
	factory.RegisterType("api_fragment", reflect.TypeOf(composite.ApiFragmentRenderConfig{}))
	factory.RegisterType("text", reflect.TypeOf(component.TextConfig{}))
	for _, kind := range []string{"api", "api_fragment"} {
		for _, invalid := range []interface{}{false, true, 0, 1, nil, []interface{}{"session"}, map[string]interface{}{"cookie": "session"}} {
			_, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: kind, Data: map[string]interface{}{"forwardtoken": invalid}})
			if err == nil {
				t.Errorf("%s accepted non-string forwardtoken %T", kind, invalid)
			}
		}
		for _, valid := range []map[string]interface{}{{}, {"forwardtoken": ""}, {"forwardtoken": "account_session"}} {
			if _, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: kind, Data: valid}); err != nil {
				t.Errorf("%s rejected string/omitted field: %v", kind, err)
			}
		}
	}
	// General component decoding is intentionally unaffected by the API policy.
	result, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: "text", Data: map[string]interface{}{"value": false}})
	if err != nil || result.Instance.(component.TextConfig).Value != "0" {
		t.Fatalf("ordinary weak string conversion changed: result=%v error=%v", result, err)
	}
}
