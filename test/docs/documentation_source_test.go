package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/render"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/internal/typefactory"
	"golang.org/x/net/html"
)

var (
	versionFlag   = flag.String("version", "dev", "override version in tests")
	buildTimeFlag = flag.String("buildtime", "undefined", "override build time in tests")
)

// These variables will be set at build time
var (
	Version   string
	BuildTime string
)

// FieldDoc represents a field documentation entry
type FieldDoc struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	Mapstructure       string `json:"mapstructure"`
	Category           string `json:"category"`
	Description        string `json:"description"`
	Example            string `json:"example"`
	MetaDocDescription string `json:"@metadoc"`
	Result             string `json:"result"`
	TypeDescription    string `json:"@doc"`
}

type DocumentationTypeStruct struct {
	Name            string
	TypeDescription string
	ConfigType      string
	ConfigCategory  string
	Fields          map[string]string
	Embedded        map[string]string
	Config          any
}

// ParsedContent holds the separated sections and optional scope after parsing.
type ParsedContent struct {
	HyperbricksConfig      string
	HyperbricksConfigScope string
	Explainer              string
	ExpectedJSON           map[string]interface{}
	ExpectedJSONAsString   string
	ExpectedOutput         string
}

func Test_TestAndDocumentationRender(t *testing.T) {

	flag.Parse()
	// e.g., assign these to a package-level Version/BuildTime if desired
	Version = *versionFlag
	BuildTime = *buildTimeFlag

	types := []DocumentationTypeStruct{
		{
			Name:            "Fragment",
			TypeDescription: "Basic type description here.....",
			Embedded: map[string]string{
				"HxResponse": "response",
			},
			ConfigType:     "<FRAGMENT>",
			ConfigCategory: "composite",
			Config:         composite.FragmentConfig{},
		},
		{
			Name:            "Hypermedia",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<HYPERMEDIA>",
			ConfigCategory:  "composite",
			Config:          composite.HyperMediaConfig{},
		},
		// API is for version 2.0.0
		// {
		// 	Name:            "Api",
		// 	TypeDescription: "Basic type description here.....",
		// 	Embedded: map[string]string{
		// 		"HxResponse": "response",
		// 	},
		// 	ConfigType:     "<API>",
		// 	ConfigCategory: "composite",
		// 	Config:         composite.HxApiConfig{},
		// },
		// COMPONENTS
		{
			Name:            "Html",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<HTML>",
			ConfigCategory:  "component",
			Config:          component.HTMLConfig{},
		},
		{
			Name:            "Css",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<CSS>",
			ConfigCategory:  "component",
			Config:          component.CssConfig{},
		},
		{
			Name:            "Js",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<JS>",
			ConfigCategory:  "component",
			Config:          component.JavaScriptConfig{},
		},
		{
			Name:            "Image",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<IMAGE>",
			ConfigCategory:  "component",
			Config:          component.SingleImageConfig{},
		},
		{
			Name:            "Images",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<IMAGES>",
			ConfigCategory:  "component",
			Config:          component.MultipleImagesConfig{},
		},
		{
			Name:            "Json",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<JSON>",
			ConfigCategory:  "component",
			Config:          component.LocalJSONConfig{},
		},
		{
			Name:            "Plugin",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<PLUGIN>",
			ConfigCategory:  "component",
			Config:          component.PluginConfig{},
		},
		{
			Name:            "Text",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<TEXT>",
			ConfigCategory:  "component",
			Config:          component.TextConfig{},
		},
		{
			Name:            "Menu",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<MENU>",
			ConfigCategory:  "component",
			Config:          component.MenuConfig{},
		},
		{
			Name:            "Api_Render",
			TypeDescription: "Basic type description here.....",
			Embedded:        map[string]string{},
			ConfigType:      "<API_RENDER>",
			ConfigCategory:  "component",
			Config:          component.APIConfig{},
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

	// Mock template provider
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"api_test_template": `{{ (index .quotes 0).author }}:{{ (index .quotes 0).quote }}`,
		}
		content, exists := templates[templateName]
		return content, exists
	}
	// TEMPLATE ....
	pageRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.HyperMediaConfigGetName(), pageRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	// TEMPLATE ....
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	// API ....
	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	// COMPONENTS
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	categorizedDocs := make(map[string]map[string][]FieldDoc)
	for _, cfg := range types {
		//fmt.Printf("\n\n======= Processing type: %s =======\n", cfg.Name)
		var fields []FieldDoc
		// Process non-embedded fields first
		val := reflect.ValueOf(cfg.Config)
		fields = processFieldsWithSquash(val, cfg, t, rm, nil)

		// Iterate through embedded fields
		for embeddedName, fieldTag := range cfg.Embedded {
			fmt.Printf("\nEmbedded Field: %s (mapped as: %s)\n", embeddedName, fieldTag)

			field := findFieldByName(val, embeddedName)
			if field.IsValid() {
				fields = append(fields, processFieldsWithSquash(field, cfg, t, rm, nil)...)
			} else {
				fmt.Printf("Field %s not found in Config\n", embeddedName)
			}
		}
		// Use the ContentType as the key
		typeName := cfg.ConfigType
		if _, found := categorizedDocs[cfg.ConfigCategory]; !found {
			categorizedDocs[cfg.ConfigCategory] = make(map[string][]FieldDoc)
		}
		categorizedDocs[cfg.ConfigCategory][typeName] = fields
	}

	log.Printf("Version: %s\n", Version)
	log.Printf("Build Time: %s\n", BuildTime)

	data := map[string]any{
		"data":      categorizedDocs,
		"version":   Version,
		"buildtime": BuildTime,
	}

	// Parse the HTML template
	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	// Generate the static HTML file
	renderStaticFile(tmpl, data, "../../docs/hyperbricks-reference-"+Version+".html")

	// // HTTP handler to serve the page dynamically
	// http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	if err := tmpl.Execute(w, data); err != nil {
	// 		http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	}
	// })

	// // Start the HTTP server
	// log.Println("Serving at http://localhost:8080")
	// if err := http.ListenAndServe(":8080", nil); err != nil {
	// 	log.Fatalf("Error starting server: %v", err)
	// }
}
func processFieldsWithSquash(val reflect.Value, cfg DocumentationTypeStruct, t *testing.T, rm *render.RenderManager, _fields []FieldDoc) []FieldDoc {

	var fields []FieldDoc
	if len(_fields) > 0 {
		fields = _fields
	}

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		fmt.Println("Not a struct, skipping.")
		return fields
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)

		// skip if exclude:"true" in metadata of field
		exclude := field.Tag.Get("exclude")
		if exclude != "" {
			continue
		}

		tag := field.Tag.Get("mapstructure")
		if tag == ",squash" {
			// Recursively process embedded fields
			par := val.Field(i)
			fields = processFieldsWithSquash(par, cfg, t, rm, fields)

		}
		var example string
		if tag != "" && tag != ",squash" && tag != ",remain" {
			//out.WriteString(fmt.Sprintf("Field: %s ->%s\n", tag, field.Name))
			//out.WriteString(fmt.Sprintf("description: %s\n", field.Tag.Get("description")))
			if field.Tag.Get("example") != "" {
				example = _checkAndReadFile(field.Tag.Get("example"), field.Tag.Get("description"))
				//out.WriteString(fmt.Sprintf("example: %s\n", example))
			} else if field.Tag.Get("mapstructure") != "" {
				file := strings.ToLower(cfg.Name) + "-" + tag
				example = _checkAndReadFile("{!{"+file+".hyperbricks}}", field.Tag.Get("description"))
				//out.WriteString(fmt.Sprintf("example: %s\n", example))
			}
			// PARSE HYPERSCRIPT
			// RUN THE TEST (compare json with serialized output go object)
			t.Run(strings.ToLower(cfg.Name)+"-"+tag, func(t *testing.T) {

				parsed, err := ParseContent(example)
				if err != nil {
					fmt.Println("Error:", err)
					return
				}

				//fmt.Println("Hyperbricks Config:")
				//fmt.Println(parsed.HyperbricksConfig)

				//fmt.Println("\nExplainer:")
				//fmt.Println(parsed.Explainer)

				//fmt.Println("\nExpected JSON (Non-Escaped):")

				var buf bytes.Buffer
				encoder := json.NewEncoder(&buf)
				encoder.SetEscapeHTML(false) // Disable HTML escaping
				encoder.SetIndent("", "  ")  // Enable pretty printing with indentation
				if err := encoder.Encode(parsed.ExpectedJSON); err != nil {
					fmt.Println("Error encoding JSON:", err)
					return
				}
				//fmt.Print(buf.String())
				var expected map[string]interface{}

				// Convert JSON string to bytes and unmarshal into the map
				if err := json.Unmarshal([]byte(buf.String()), &expected); err != nil {
					fmt.Println("Error unmarshaling JSON:", err)
					return
				}
				//fmt.Printf("JSON string: %s", parsed.ExpectedJSONAsString)
				//fmt.Printf("converted JSON object: %v", expected)

				//fmt.Println("\nExpected Output:")
				//fmt.Println(parsed.ExpectedOutput)
				// Parse the combined configuration.
				parsedConfig := parser.ParseHyperScript(parsed.HyperbricksConfig)
				//fmt.Printf("got obj from hyperscript:%v", parsedConfig)
				// Convert the struct to JSON
				// jsonBytes, err := json.MarshalIndent(parsedConfig[parsed.HyperbricksConfigScope], "", "  ")
				// if err != nil {
				// 	fmt.Println("Error marshaling struct to JSON:", err)
				// 	return
				// }
				// fmt.Printf("Hyperscript object:%s", string(jsonBytes))

				// Prepare a variable of type map[string]interface{}
				// Create a TypeRequest for the TypeFactory.

				// Extract the relevant scope data.
				scopeData, ok := parsedConfig[parsed.HyperbricksConfigScope].(map[string]interface{})
				if !ok {
					t.Errorf("Scope '%s' not found or invalid type", parsed.HyperbricksConfigScope)
					return
				}

				request := typefactory.TypeRequest{
					TypeName: scopeData["@type"].(string),
					Data:     scopeData,
				}

				//Use the RenderManager to instantiate the configuration.
				response, err := rm.MakeInstance(request)
				if err != nil {
					t.Errorf("Error creating instance: %v", err)
					return
				}
				//fmt.Printf("response object:%v", response)

				result, errr := rm.Render(request.TypeName, scopeData)
				if errr != nil {
					log.Printf("%v", errr)
				}

				// fmt.Printf("\nrendered result:%s", result)
				// fmt.Printf("\nexpected result:%s", parsed.ExpectedOutput)

				// TO COMPARE THIS....
				_res_html := removeTabsAndNewlines(result)
				_exp_html := removeTabsAndNewlines(parsed.ExpectedOutput)

				// Now compare the normalized strings.
				if _res_html != _exp_html {
					t.Errorf("result and expected html output does not match: result:\n%s \nexpected:%s", _res_html, _exp_html)
				}

				equal, err := JSONDeepEqual(expected, response.Instance)
				if err != nil {
					t.Fatalf("Error comparing JSON for %s-%s: %v", strings.ToLower(cfg.Name), tag, err)
				}
				if !equal {
					t.Errorf("Test failed for %s-%s!\n", strings.ToLower(cfg.Name), tag)
					outputJSON := convertToJSON(response.Instance)
					expectedJSON := convertToJSON(expected)

					fmt.Printf("output:\n%s\n", outputJSON)
					fmt.Printf("expected:\n%s\n", expectedJSON)

				} else {

				}

				// add to docs....
				fields = append(fields, FieldDoc{
					Name:            cfg.Name,
					Type:            request.TypeName,
					Mapstructure:    field.Tag.Get("mapstructure"),
					Description:     field.Tag.Get("description"),
					Category:        cfg.ConfigCategory,
					Example:         parsed.HyperbricksConfig,
					Result:          _res_html,
					TypeDescription: cfg.TypeDescription,
				})
			})

		}

	}

	return fields
}

func renderStaticFile(tmpl *template.Template, data interface{}, outputPath string) {
	// Create the static output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outputFile.Close()

	// Execute the template and write to the file
	if err := tmpl.Execute(outputFile, data); err != nil {
		log.Fatalf("Error rendering template to file: %v", err)
	}

	log.Printf("Static HTML file generated at %s", outputPath)
}

// ParseContent parses the provided content string into its respective parts.
// It also extracts an optional scope from the "hyperbricks config" header.
func ParseContent(content string) (*ParsedContent, error) {
	// Regular expression to match section headers like:
	// ==== hyperbricks config {!{fragment}} ====
	// It captures the header title and an optional scope.
	headerRegex := regexp.MustCompile(`^====\s*([^!]+?)(?:\s*\{\!\{([^}]+)\}\})?\s*====$`)

	sections := make(map[string]string)
	var currentSection string
	var sb strings.Builder

	// Variable to store the scope for "hyperbricks config" if found.
	var hyperbricksConfigScope string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := headerRegex.FindStringSubmatch(line)
		if matches != nil {
			// When encountering a new header, save the current section's content.
			if currentSection != "" {
				sections[strings.ToLower(currentSection)] = strings.TrimSpace(sb.String())
				sb.Reset()
			}
			// matches[1] contains the header title.
			currentSection = strings.TrimSpace(matches[1])

			// If a scope was provided, matches[2] will contain it.
			scope := ""
			if len(matches) >= 3 {
				scope = strings.TrimSpace(matches[2])
			}

			// Specifically store scope for "hyperbricks config" header.
			if strings.EqualFold(currentSection, "hyperbricks config") {
				hyperbricksConfigScope = scope
			}
		} else {
			if currentSection != "" {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}
	if currentSection != "" {
		sections[strings.ToLower(currentSection)] = strings.TrimSpace(sb.String())
	}

	hyperbricksConfig := sections["hyperbricks config"]
	explainer := sections["explainer"]
	expectedJSONStr := sections["expected json"]
	expectedOutput := sections["expected output"]

	var expectedJSON map[string]interface{}
	if err := json.Unmarshal([]byte(expectedJSONStr), &expectedJSON); err != nil {
		return nil, fmt.Errorf("error parsing expected JSON: %v", err)
	}

	return &ParsedContent{
		HyperbricksConfig:      hyperbricksConfig,
		HyperbricksConfigScope: hyperbricksConfigScope,
		Explainer:              explainer,
		ExpectedJSON:           expectedJSON,
		ExpectedJSONAsString:   sections["expected json"],
		ExpectedOutput:         expectedOutput,
	}, nil
}

func _checkAndReadFile(input string, description string) string {
	filePath := "hyperbricks-test-files/"
	// Define a regex pattern to match {{<filename.extension>}}
	re := regexp.MustCompile(`\{\!\{([^\}]+)\}\}`)

	// Find all matches
	matches := re.FindAllStringSubmatch(input, -1)

	// Process each match
	for _, match := range matches {
		// match[0] is the full placeholder (e.g., "{{api-template.hyperbricks}}")
		// match[1] is the filename.extension (e.g., "api-template.hyperbricks")

		filename := match[1]
		fileFullPath := filePath + filename

		// Check if file exists, create if not
		if _, err := os.Stat(fileFullPath); os.IsNotExist(err) {
			//fmt.Printf("File %s does not exist. Creating it...\n", fileFullPath)
			f, createErr := os.Create(fileFullPath)
			if createErr != nil {
				fmt.Printf("Error creating file %s: %v\n", fileFullPath, createErr)
				input = strings.ReplaceAll(input, match[0], "no example yet")
				continue
			}
			// Ensure the file is closed when we're done
			defer f.Close()

			// Define some content to write and fail
			content := `==== hyperbricks config {!{fragment}} ====
fragment = <FRAGMENT>
fragment {
	
}
==== explainer ====
` + description + `
==== expected json ====
{
	
}
==== expected output ====
<div>this test fails</div>
`
			// Write content to the file
			_, err = f.WriteString(content)
			if err != nil {
				fmt.Println("Error writing to file:", err)
				return ""
			}
			//fmt.Printf("File %s created successfully.\n", fileFullPath)
		}

		// Read the file content
		content, err := os.ReadFile(fileFullPath)
		if err != nil {
			// If the file is not found, replace with an error placeholder
			fmt.Printf("Error reading file %s: %v\n", fileFullPath, err)
			input = strings.ReplaceAll(input, match[0], "no example yet")
			continue
		}

		// Replace the placeholder with the file content
		//fmt.Printf("Replacing placeholder %s with content from file %s.\n", match[0], fileFullPath)
		input = strings.ReplaceAll(input, match[0], string(content))
	}

	return input
}

func findFieldByName(val reflect.Value, fieldName string) reflect.Value {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if !val.IsValid() || val.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	fieldVal := val.FieldByName(fieldName)
	if fieldVal.IsValid() {
		return fieldVal
	}

	for i := 0; i < val.NumField(); i++ {
		subVal := val.Field(i)
		found := findFieldByName(subVal, fieldName)
		if found.IsValid() {
			return found
		}
	}

	return reflect.Value{}
}

// normalizeHTML parses the input HTML string and renders it back into a
// canonical form, removing insignificant whitespace differences.
func normalizeHTML(input string) (string, error) {
	// Parse the HTML into a DOM tree.
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return "", err
	}

	// Render the DOM tree back to HTML.
	var buf bytes.Buffer
	err = html.Render(&buf, doc)
	if err != nil {
		return "", err
	}

	// The rendered HTML may include an outer <html><head></head><body>...
	// structure depending on the input. If you only need the body’s content,
	// additional processing might be necessary. For simplicity, this example
	// compares the entire document structure.
	return buf.String(), nil
}

// stripAllWhitespace removes all whitespace characters from the input string.
func stripAllWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

// removeTabsAndNewlines removes tab, newline, and carriage return characters from the input string.
func removeTabsAndNewlines(s string) string {
	re := regexp.MustCompile(`[\t\n\r]+`)
	return re.ReplaceAllString(s, "")
}

func convertToJSON(obj interface{}) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // Disable escaping of HTML characters
	encoder.SetIndent("", "  ")  // Set indent similar to MarshalIndent

	//if err := encoder.Encode(parsedConfig[parsed.HyperbricksConfigScope]); err != nil {
	if err := encoder.Encode(obj); err != nil {
		fmt.Println("Error encoding to JSON:", err)
		return ""
	}

	// The encoder adds a newline at the end of the output; trim if needed
	return buf.String()
}

// Helper function to normalize and compare two interface{} values
func JSONDeepEqual(a, b interface{}) (bool, error) {
	// Serialize both values to JSON
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false, err
	}

	// Deserialize JSON bytes back into interface{}
	var aJSON interface{}
	var bJSON interface{}

	if err := json.Unmarshal(aBytes, &aJSON); err != nil {
		return false, err
	}
	if err := json.Unmarshal(bBytes, &bJSON); err != nil {
		return false, err
	}

	// Use DeepEqual on the normalized data
	return reflect.DeepEqual(aJSON, bJSON), nil
}
