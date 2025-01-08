package composite

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/render"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/internal/typefactory"
)

func Test_HyperMediaConfigPropertiesAndRenderOutput(t *testing.T) {
	// Define a baseline for default empty values of HyperMediaConfig fields.
	defaultExpectedOutput := map[string]interface{}{
		"BodyTag": "",
		"Composite": shared.CompositeRendererConfig{
			Meta: shared.Meta{
				ConfigType:     "<HYPERMEDIA>",
				ConfigCategory: "",
				Key:            "",
				Path:           "",
				File:           "",
			},
			Items: map[string]interface{}(nil),
		},
		"Css":      []string(nil),
		"Doctype":  "",
		"Enclose":  "",
		"Favicon":  "",
		"Head":     map[string]interface{}(nil),
		"HtmlTag":  "",
		"Index":    0,
		"IsStatic": false,
		"Items":    map[string]interface{}(nil),
		"Js":       []string(nil),
		"Meta":     map[string]string(nil),
		"Route":    "",
		"Section":  "",
		"Static":   "",
		"Template": map[string]interface{}(nil),
		"Title":    "",
	}

	tests := []struct {
		name            string
		propertyLine    string
		config          string
		scope           string
		expectedExample string
		expectedOutput  map[string]interface{}
		mainexample     bool
		expectError     bool
	}{
		{
			name: "A basic minimal hypermedia setup",
			propertyLine: `
	title = just a title
	route = index
	section = main
	`,
			scope: "hypermedia",
			config: `
hypermedia = <HYPERMEDIA>
hypermedia {
%s
}
`,
			expectedExample: "{!{hypermedia.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Title"] = "just a title"
				out["Route"] = "index"
				out["Section"] = "main"
				return out
			}(),
			mainexample: true,
			expectError: false,
		},
		{
			name:         "hypermedia-title",
			propertyLine: `title = Home`,
			scope:        "hypermedia",
			config: `
hypermedia = <HYPERMEDIA>
hypermedia {
%s
}
`,
			expectedExample: "{!{hypermedia-title.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Title"] = "Home"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "hypermedia-title",
			propertyLine: `title = Welcome`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-title.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Title"] = "Welcome"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Route field",
			propertyLine: `route = home`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-route.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Route"] = "home"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Section field",
			propertyLine: `section = main`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-section.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Section"] = "main"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "BodyTag field",
			propertyLine: `bodytag = <body>|</body>`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-bodywrap.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["BodyTag"] = "<body>|</body>"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Favicon field",
			propertyLine: `favicon = /favicon.ico`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-favicon.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Favicon"] = "/favicon.ico"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Doctype field",
			propertyLine: `doctype = <!DOCTYPE html>`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-doctype.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Doctype"] = "<!DOCTYPE html>"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Meta field",
			propertyLine: `meta = { description = \"Test meta\", keywords = \"test,hypermedia\" }`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-meta.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Meta"] = map[string]string{
					"description": "Test meta",
					"keywords":    "test,hypermedia",
				}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Css field",
			propertyLine: `css = [ \"styles.css\", \"theme.css\" ]`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-css.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Css"] = []string{"styles.css", "theme.css"}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Js field",
			propertyLine: `js = [ \"script.js\", \"app.js\" ]`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-js.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Js"] = []string{"script.js", "app.js"}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Enclose field",
			propertyLine: `enclose = <div>|</div>`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-wrap.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Enclose"] = "<div>|</div>"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Template field",
			propertyLine: `template = { key = value }`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-template.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Template"] = map[string]interface{}{"key": "value"}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "IsStatic field",
			propertyLine: `isstatic = true`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["IsStatic"] = true
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Static field",
			propertyLine: `static = /static/path`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-static.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Static"] = "/static/path"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Index field",
			propertyLine: `index = 5`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{page-index.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Index"] = 5
				return out
			}(),
			expectError: false,
		},
		{
			name:         "HtmlTag field",
			propertyLine: `htmltag = <html lang="en">`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-htmltag.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["HtmlTag"] = `<html lang="en">`
				return out
			}(),
			expectError: false,
		},
		{
			name:         "Head field",
			propertyLine: `head = { meta = { charset = "UTF-8" } }`,
			scope:        "hypermedia",
			config: `
	hypermedia = <HYPERMEDIA>
	hypermedia {
		%s
	}
	`,
			expectedExample: "{!{hypermedia-head.hyperbricks}}",
			expectedOutput: func() map[string]interface{} {
				out := make(map[string]interface{})
				for k, v := range defaultExpectedOutput {
					out[k] = v
				}
				out["Head"] = map[string]interface{}{
					"meta": map[string]interface{}{
						"charset": "UTF-8",
					},
				}
				return out
			}(),
			expectError: false,
		},
	}

	// Initialize shared configuration settings.
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	// Create a new RenderManager instance and register the HyperMediaRenderer.
	rm := render.NewRenderManager()
	hypermediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HyperMediaConfigGetName(), hypermediaRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))

	// Iterate through each test case and execute.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Combine configuration string with property line for the test case.
			configStr := joinTestCode(tt.config, tt.propertyLine)

			// Parse the combined configuration.
			config := parser.ParseHyperScript(configStr[0])

			// Extract the relevant scope data.
			scopeData, ok := config[tt.scope].(map[string]interface{})
			if !ok {
				if tt.expectError {
					return // Expected error case.
				}
				t.Errorf("Scope '%s' not found or invalid type", tt.scope)
				return
			}

			// Create a TypeRequest for the TypeFactory.
			request := typefactory.TypeRequest{
				TypeName: "<HYPERMEDIA>",
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
			instanceMap := structToMap(response.Instance)

			// Validate the generated instance against the expected output.
			if !reflect.DeepEqual(tt.expectedOutput, instanceMap) {
				t.Errorf("Test failed for %s!\nExpected:\n%#v\nGot:\n%#v", tt.name, tt.expectedOutput, instanceMap)
			} else {
				t.Logf("Test passed for %s", tt.name)

				// Write the configuration to a file for documentation purposes.
				outputFile := outputpath + tt.name + ".hyperbricks"
				err := writeToFile(outputFile, configStr[0])
				if err != nil {
					t.Errorf("Failed to write to file %s: %v", outputFile, err)
				} else {
					t.Logf("Written to file: %s", outputFile)
				}
			}
		})
	}
}

var (
	write      bool
	outputpath string = "../../cmd/hyperbricks-docs/hyperbricks-examples/"
)

// Helper function to combine the main configuration string with a property-specific test case.
func joinTestCode(hyperbricks string, propertyTest string) []string {
	return []string{fmt.Sprintf(hyperbricks, propertyTest), propertyTest}
}

// Helper function to convert a struct to a map for easy validation in test cases.
func structToMap(data interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)
		result[field.Name] = value.Interface()
	}

	return result
}

// Helper function to write test outputs to a file for documentation purposes.
func writeToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}
