package component

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/render"
	"github.com/hyperbricks/hyperbricks/internal/typefactory"
)

func Test_Api_Fields(t *testing.T) {
	// Test cases for each property
	tests := []struct {
		name            string
		propertyLine    string
		config          string
		scope           string
		expectedExample string
		expectedOutput  map[string]interface{}
		expectError     bool
	}{
		{
			name:         "api-render-endpoint",
			propertyLine: `endpoint = https://dummyjson.com/auth/login`,
			scope:        "api_test",
			config: `
api_test = <API_RENDER>
api_test {
	%s
}`,
			expectedExample: "",
			expectedOutput: map[string]interface{}{
				"endpoint": "https://dummyjson.com/quotes",
			},
			expectError: false,
		},
	}

	// Initialize the RenderManager
	rm := render.NewRenderManager()

	// Initialize the HeadRenderer
	APIRenderer := &component.APIRenderer{}

	// Register the HeadRenderer component
	rm.RegisterComponent(component.APIConfigGetName(), APIRenderer, reflect.TypeOf(component.APIRenderer{}))

	// Execute test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Generate the TyperScript configuration
			configStr := joinTestCode(tt.config, tt.propertyLine)

			// Parse the TyperScript configuration
			config := parser.ParseHyperScript(configStr[0])

			// Retrieve the specific scope
			scopeData, ok := config[tt.scope].(map[string]interface{})
			if !ok {
				if tt.expectError {
					return // Expected error scenario
				}
				t.Errorf("Scope '%s' not found or invalid type", tt.scope)
				return
			}

			// Create a TypeRequest for the TypeFactory
			request := typefactory.TypeRequest{
				TypeName: "<API_RENDER>",
				Data:     scopeData,
			}

			// Create the instance using TypeFactory
			response, err := rm.MakeInstance(request)
			if err != nil {
				if tt.expectError {
					return // Expected error scenario
				}
				t.Errorf("Error creating instance: %v", err)
				return
			}

			// Convert the response instance to a map for comparison
			instanceMap := structToMap(response.Instance)

			// Compare the parsed config with the expected config
			if !reflect.DeepEqual(tt.expectedOutput, instanceMap) {
				t.Errorf("Test failed for %s!\nExpected:\n%#v\nGot:\n%#v", tt.name, tt.expectedOutput, instanceMap)
			} else {
				t.Logf("Test passed for %s", tt.name)

				// Define the output file path
				outputFile := outputpath + tt.name + ".typerscript"

				// Write the example string to the output file
				err = writeToFile(outputFile, tt.propertyLine)
				if err != nil {
					t.Errorf("Failed to write to file %s: %v", outputFile, err)
				} else {
					t.Logf("Written to file: %s", outputFile)
				}
			}
		})
	}
}
