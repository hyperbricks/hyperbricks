package composite

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/render"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/internal/typefactory"
)

// Test_HxApiConfigPropertiesAndRenderOutput tests how HxApiConfig
// properties are parsed and rendered.
func Test_HxApiConfigPropertiesAndRenderOutput(t *testing.T) {

	// Define a baseline for default empty values of HxApiConfig fields.
	// NOTE: This is how StructToMap will flatten or nest your data. Adjust if needed.
	defaultExpectedOutput := map[string]interface{}{
		"Composite": shared.CompositeRendererConfig{
			Meta: shared.Meta{
				ConfigType: "<API>",
			},
		},
		// The HxDataContainer (nested struct)
		"HxDataContainer": map[string]interface{}{
			"Data": map[string]interface{}{
				"HxForm":    map[string]interface{}(nil),
				"HxHeaders": map[string]interface{}(nil),
				"HxQuery":   map[string]interface{}(nil),
			},
			"RowResult":           []interface{}(nil),
			"SomeResultContainer": map[string]interface{}(nil),
			"R":                   map[string]interface{}{}, // typically empty in these tests
		},
		// The HxRequest (nested struct)
		"HxRequest": map[string]interface{}{
			"HXDb":            "",
			"HXDescription":   "",
			"HXErrorTemplate": "",
			"HXMethod":        "",
			"HXModel": map[string]interface{}{
				"fields": map[string]interface{}(nil),
				"name":   "",
			},
			"HXQuery": map[string]interface{}{
				"create": map[string]interface{}{
					"fields": []interface{}(nil),
					"sql":    "",
				},
				"delete": map[string]interface{}{
					"fields": []interface{}(nil),
					"sql":    "",
				},
				"read": map[string]interface{}{
					"fields": []interface{}(nil),
					"sql":    "",
				},
				"replace": map[string]interface{}{
					"fields": []interface{}(nil),
					"sql":    "",
				},
				"update": map[string]interface{}{
					"fields": []interface{}(nil),
					"sql":    "",
				},
			},
			"HXTable":          "",
			"HXTemplate":       "",
			"HxBoosted":        "",
			"HxCurrentUrl":     "",
			"HxHistoryRestore": "",
			"HxMethod":         "",
			"HxPrompt":         "",
			"HxRequestFlag":    "",
			"HxRoute":          "",
			"HxTarget":         "",
			"HxTrigger":        "",
			"HxTriggerName":    "",
			"Hx_form_data": map[string]interface{}{
				"HxForm":    map[string]interface{}(nil),
				"HxHeaders": map[string]interface{}(nil),
				"HxQuery":   map[string]interface{}(nil),
			},
		},
		// The HxResponse (nested struct)
		"HxResponse": map[string]interface{}{
			"HxLocation":           "",
			"HxPushedUrl":          "",
			"HxRedirect":           "",
			"HxRefresh":            "",
			"HxReplaceUrl":         "",
			"HxReselect":           "",
			"HxReswap":             "",
			"HxRetarget":           "",
			"HxTemplateResult":     "",
			"HxTrigger":            "",
			"HxTriggerafterSettle": "",
			"HxTriggerafterSwap":   "",
		},
		"HxResponseWriter": interface{}(nil),
		"Template":         map[string]interface{}(nil),
		"Items":            map[string]interface{}(nil),
		"Enclose":          "",
		"IsStatic":         false,
		"Static":           "",
	}

	tests := []struct {
		name           string
		propertyLine   string
		config         string
		scope          string
		expectedOutput map[string]interface{}
		expectError    bool
	}{
		{
			name:         "hxapi-route",
			propertyLine: `hx_route = "some-route"`,
			scope:        "api",
			config: `
api = <API>
api {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				// We must set HxRequest -> HxRoute
				out["HxRequest"].(map[string]interface{})["HxRoute"] = "some-route"
				return out
			}(),
			expectError: false,
		},
		{
			name: "hxapi-db-and-method",
			propertyLine: `
hx_db = testdata/hx.sqlite
hx_method = "GET"
`,
			scope: "api",
			config: `
api = <API>
api {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				req := out["HxRequest"].(map[string]interface{})
				req["HXDb"] = "testdata/hx.sqlite"
				req["HxMethod"] = "GET"
				return out
			}(),
			expectError: false,
		},
		{
			name: "hxapi-hxquery",
			propertyLine: `
hx_query {
  read {
    fields = [ id, name ]
    sql = SELECT id, name FROM some_table WHERE id = ?
  }
}
`,
			scope: "api",
			config: `
api = <API>
api {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				read := out["HxRequest"].(map[string]interface{})["HXQuery"].(map[string]interface{})["read"].(map[string]interface{})
				read["fields"] = []interface{}{"id", "name"}
				read["sql"] = "SELECT id, name FROM some_table WHERE id = ?"
				return out
			}(),
			expectError: false,
		},
		{
			name: "hxapi-model-fields",
			propertyLine: `
hx_model {
  name = User
  fields {
    username {
      type = string
      validate = required
    }
    age {
      type = int
      validate = required
    }
  }
}
`,
			scope: "api",
			config: `
api = <API>
api {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				hxmodel := out["HxRequest"].(map[string]interface{})["HXModel"].(map[string]interface{})
				hxmodel["name"] = "User"
				hxmodel["fields"] = map[string]interface{}{
					"username": map[string]interface{}{
						"type":     "string",
						"validate": "required",
					},
					"age": map[string]interface{}{
						"type":     "int",
						"validate": "required",
					},
				}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "hxapi-static",
			propertyLine: `static = /some/static/path`,
			scope:        "api",
			config: `
api = <API>
api {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Static"] = "/some/static/path"
				return out
			}(),
			expectError: false,
		},
	}

	// Initialize shared configuration settings.
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	// Create a new RenderManager instance and register the HxApiRenderer.
	rm := render.NewRenderManager()
	apiRenderer := &composite.HxApiRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HxApiConfigGetName(), apiRenderer, reflect.TypeOf(composite.HxApiConfig{}))

	// Iterate through each test case.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Combine configuration string with property line for the test case.
			configStr := shared.JoinTestCode(tt.config, tt.propertyLine)

			// Parse the combined configuration.
			parsedConfig := parser.ParseHyperScript(configStr[0])

			// Extract the relevant scope data.
			scopeData, ok := parsedConfig[tt.scope].(map[string]interface{})
			if !ok {
				if tt.expectError {
					return // Expected error case
				}
				t.Errorf("Scope '%s' not found or invalid type", tt.scope)
				return
			}

			// Create a TypeRequest for the TypeFactory.
			request := typefactory.TypeRequest{
				TypeName: "<API>", // This must match HxApiConfigGetName() output
				Data:     scopeData,
			}

			// Use the RenderManager to instantiate the configuration.
			response, err := rm.MakeInstance(request)
			if err != nil {
				if tt.expectError {
					return // Expected error case
				}
				t.Errorf("Error creating instance: %v", err)
				return
			}

			// Convert the instantiated object to a map for validation.
			instanceMap := shared.StructToMap(response.Instance)

			// Validate the generated instance against the expected output.
			if !reflect.DeepEqual(tt.expectedOutput, instanceMap) {
				t.Errorf("Test failed for %s!\nExpected:\n%#v\nGot:\n%#v", tt.name, tt.expectedOutput, instanceMap)
			} else {
				t.Logf("Test passed for %s", tt.name)
			}
		})
	}
}

// cloneMap is a helper to do a shallow clone of map[string]interface{}
func cloneMap(source map[string]interface{}) map[string]interface{} {
	dest := make(map[string]interface{}, len(source))
	for k, v := range source {
		dest[k] = v
	}
	return dest
}
