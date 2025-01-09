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

// Test_TemplateConfigPropertiesAndRenderOutput tests how TemplateConfig
// properties are parsed and rendered.
func Test_TemplateConfigPropertiesAndRenderOutput(t *testing.T) {

	// Define a baseline for default empty values of TemplateConfig fields.
	defaultExpectedOutput := map[string]interface{}{
		"Composite": shared.CompositeRendererConfig{
			Meta: shared.Meta{
				ConfigType: "<TEMPLATE>",
			},
		},
		"Template":   "",
		"IsTemplate": false,
		"Values":     map[string]interface{}(nil),
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
			name:         "template-template",
			propertyLine: `template = <<[<div>Hello Template</div>]>>`,
			scope:        "template",
			config: `
template = <TEMPLATE>
template {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := shared.CloneMap(defaultExpectedOutput)
				out["Template"] = "<div>Hello Template</div>"
				return out
			}(),
		},
		{
			name:         "template-istemplate",
			propertyLine: `istemplate = true`,
			scope:        "template",
			config: `
template = <TEMPLATE>
template {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := shared.CloneMap(defaultExpectedOutput)
				out["IsTemplate"] = true
				return out
			}(),
		},
		{
			name: "template-values",
			propertyLine: `
			
			template = {{TEMPLATE:test/composite/template.html}}
			values = {
                msg = A message
                count = 10
			}`,
			scope: "template",
			config: `
template = <TEMPLATE>
template {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := shared.CloneMap(defaultExpectedOutput)
				out["Template"] = "{{TEMPLATE:test/composite/template.html}}"
				out["Values"] = map[string]interface{}{
					"msg":   "A message",
					"count": "10", // note: numeric values from config might parse as float64
				}
				return out
			}(),
		},
	}

	// Initialize shared configuration settings.
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	// Create a new RenderManager instance and register the TemplateRenderer.
	rm := render.NewRenderManager()
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	// Assume you have a function TemplateConfigGetName() returning "<TEMPLATE>"
	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

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
				TypeName: "<TEMPLATE>", // This must match TemplateConfigGetName() output
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
