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

// Test_HeadConfigPropertiesAndRenderOutput tests how HeadConfig
// properties are parsed and rendered.
func Test_HeadConfigPropertiesAndRenderOutput(t *testing.T) {

	// Define a baseline for default empty values of HeadConfig fields.
	defaultExpectedOutput := map[string]interface{}{
		"Composite": shared.CompositeRendererConfig{
			Meta: shared.Meta{
				ConfigType: "<HEAD>",
			},
		},
		// HeadConfig-specific fields:
		"MetaData": map[string]string(nil),
		"Css":      []string(nil),
		"Js":       []string(nil),
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
			name: "head-meta",
			propertyLine: `meta { 
			description = Example head description
			}`,
			scope: "head",
			config: `
head = <HEAD>
head {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := shared.CloneMap(defaultExpectedOutput)
				out["MetaData"] = map[string]string{"description": "Example head description"}
				return out
			}(),
		},
		{
			name:         "head-css",
			propertyLine: `css = [ styles.css, more.css ]`,
			scope:        "head",
			config: `
head = <HEAD>
head {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := shared.CloneMap(defaultExpectedOutput)
				out["Css"] = []string{"styles.css", "more.css"}
				return out
			}(),
		},
		{
			name:         "head-js",
			propertyLine: `js = [ script.js, extra.js ]`,
			scope:        "head",
			config: `
head = <HEAD>
head {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := shared.CloneMap(defaultExpectedOutput)
				out["Js"] = []string{"script.js", "extra.js"}
				return out
			}(),
		},
	}

	// Initialize shared configuration settings.
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	// Create a new RenderManager instance and register the HeadRenderer.
	rm := render.NewRenderManager()
	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	// Assume you have a function HeadConfigGetName() that returns "<HEAD>"
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

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
					return // Expected error case.
				}
				t.Errorf("Scope '%s' not found or invalid type", tt.scope)
				return
			}

			// Create a TypeRequest for the TypeFactory.
			request := typefactory.TypeRequest{
				TypeName: "<HEAD>", // This must match HeadConfigGetName() output
				Data:     scopeData,
			}

			// Use the RenderManager to instantiate the configuration.
			response, err := rm.MakeInstance(request)
			if err != nil {
				if tt.expectError {
					return // Expected error case.
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

				// Write the configuration to a file for documentation purposes.
				outputFile := shared.Outputpath + tt.name + ".hyperbricks"
				err := shared.WriteToFile(outputFile, configStr[0])
				if err != nil {
					t.Errorf("Failed to write to file %s: %v", outputFile, err)
				} else {
					t.Logf("Written to file: %s", outputFile)
				}
			}
		})
	}
}
