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

// Test_FragmentConfigPropertiesAndRenderOutput tests how FragmentConfig
// properties are parsed and rendered.
func Test_FragmentConfigPropertiesAndRenderOutput(t *testing.T) {
	// Define a baseline for default empty values of FragmentConfig fields.
	defaultExpectedOutput := map[string]interface{}{
		"BodyTag": "",
		"Composite": shared.CompositeRendererConfig{
			Meta: shared.Meta{
				ConfigType:     "<FRAGMENT>",
				ConfigCategory: "",
				Key:            "",
				Path:           "",
				File:           "",
			},
			Items: map[string]interface{}(nil),
		},
		"Css":     []string(nil),
		"Doctype": "",
		"Enclose": "",
		"Favicon": "",
		"File":    "",
		"Head":    map[string]interface{}(nil),
		"HtmlTag": "",
		"HxResponse": composite.HxResponse{
			HxTemplateResult:     "",
			HxLocation:           "",
			HxPushedUrl:          "",
			HxRedirect:           "",
			HxRefresh:            "",
			HxReplaceUrl:         "",
			HxReswap:             "",
			HxRetarget:           "",
			HxReselect:           "",
			HxTrigger:            "",
			HxTriggerafterSettle: "",
			HxTriggerafterSwap:   "",
		},
		"HxResponseWriter": interface{}(nil),
		"Index":            0,
		"IsStatic":         false,
		"Items":            map[string]interface{}(nil),
		"Js":               []string(nil),
		"Meta":             map[string]string(nil),
		"Route":            "",
		"Section":          "",
		"Static":           "",
		"Template":         map[string]interface{}(nil),
		"Title":            "",
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
			name:         "fragment-title",
			propertyLine: `title = A Fragment Title`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Title"] = "A Fragment Title"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-route",
			propertyLine: `route = fragment-route`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Route"] = "fragment-route"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-section",
			propertyLine: `section = some-section`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Section"] = "some-section"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-bodytag",
			propertyLine: `bodytag = <body>|</body>`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["BodyTag"] = "<body>|</body>"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-enclose",
			propertyLine: `enclose = <div>|</div>`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Enclose"] = "<div>|</div>"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-favicon",
			propertyLine: `favicon = /myfragment.ico`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Favicon"] = "/myfragment.ico"
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-doctype",
			propertyLine: `doctype = <!DOCTYPE html>`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Doctype"] = "<!DOCTYPE html>"
				return out
			}(),
			expectError: false,
		},
		{
			name: "fragment-meta",
			propertyLine: `meta = { 
				description = Fragment description
				keywords = fragment,test
			}`,
			scope: "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Meta"] = map[string]string{"description": "Fragment description", "keywords": "fragment,test"}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-css",
			propertyLine: `css = [ fragstyles.css, morefrag.css ]`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Css"] = []string{"fragstyles.css", "morefrag.css"}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-js",
			propertyLine: `js = [ fragscript.js, anotherfrag.js ]`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Js"] = []string{"fragscript.js", "anotherfrag.js"}
				return out
			}(),
			expectError: false,
		},
		{
			name: "fragment-template",
			propertyLine: `template = {
				template = <<[
					<div>Fragment: {{value}}</div>
				]>> 
				values {
					value = some fragment value
				}
			}`,
			scope: "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Template"] = map[string]interface{}{
					"template": "\n\t\t\t\t\t<div>Fragment: {{value}}</div>\n\t\t\t\t]>> \n\t\t\t\tvalues {\n\t\t\t\t\tvalue = some fragment value\n\t\t\t\t}\n\t\t\t}\n}\n\n",
				}
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-isstatic",
			propertyLine: `isstatic = true`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["IsStatic"] = true
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-static",
			propertyLine: `static = /some/static/path`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
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
		{
			name:         "fragment-index",
			propertyLine: `index = 99`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Index"] = 99
				return out
			}(),
			expectError: false,
		},
		{
			name:         "fragment-htmltag",
			propertyLine: `htmltag = <html lang="fr">`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["HtmlTag"] = `<html lang="fr">`
				return out
			}(),
			expectError: false,
		},
		{
			name: "fragment-head",
			propertyLine: `head {
				meta {
					charset = UTF-8
				}
			}`,
			scope: "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Head"] = map[string]interface{}{
					"meta": map[string]interface{}{
						"charset": "UTF-8",
					},
				}
				return out
			}(),
			expectError: false,
		},
		// Example test of HxResponse fields: hx_refresh = true
		{
			name:         "fragment-hxrefresh",
			propertyLine: `hx_refresh = true`,
			scope:        "fragment",
			config: `
fragment = <FRAGMENT>
fragment {
%s
}
`,
			expectedOutput: func() map[string]interface{} {
				out := cloneMap(defaultExpectedOutput)
				out["Composite"] = shared.CompositeRendererConfig{
					Meta: shared.Meta{
						ConfigType:     "<FRAGMENT>",
						ConfigCategory: "",
						Key:            "",
						Path:           "",
						File:           "",
					},
					Items: map[string]interface{}{
						"hx_refresh": "true",
					},
				}
				out["HxResponse"] = composite.HxResponse{
					HxTemplateResult:     "",
					HxLocation:           "",
					HxPushedUrl:          "",
					HxRedirect:           "",
					HxRefresh:            "true",
					HxReplaceUrl:         "",
					HxReswap:             "",
					HxRetarget:           "",
					HxReselect:           "",
					HxTrigger:            "",
					HxTriggerafterSettle: "",
					HxTriggerafterSwap:   "",
				}
				return out
			}(),
			expectError: false,
		},
	}

	// Initialize shared configuration settings.
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	// Create a new RenderManager instance and register the FragmentRenderer.
	rm := render.NewRenderManager()
	fragmentRenderer := &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.FragmentConfigGetName(), fragmentRenderer, reflect.TypeOf(composite.FragmentConfig{}))

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
				TypeName: "<FRAGMENT>",
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

				// If you want to write the configuration to a file for doc purposes.
				outputFile := shared.Outputpath + tt.name + ".hyperbricks"
				if shared.Write {
					err := shared.WriteToFile(outputFile, configStr[0])
					if err != nil {
						t.Errorf("Failed to write to file %s: %v", outputFile, err)
					} else {
						t.Logf("Written to file: %s", outputFile)
					}
				}
			}
		})
	}
}

// cloneMap is a helper to do a deep clone of map[string]interface{} that points
// to simple or nil typed values (works fine for this test).
func cloneMap(source map[string]interface{}) map[string]interface{} {
	dest := make(map[string]interface{}, len(source))
	for k, v := range source {
		dest[k] = v
	}
	return dest
}
