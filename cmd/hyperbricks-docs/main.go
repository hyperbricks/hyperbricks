package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
)

// These variables will be set at build time
var (
	Version   string
	BuildTime string
)

func main() {

	// Create instances of the configurations with ContentType set
	types := []any{
		// composite
		composite.TreeConfig{},
		composite.TemplateConfig{},
		composite.HeadConfig{},
		composite.HyperMediaConfig{},

		// components
		component.APIConfig{},
		component.HTMLConfig{},
		component.SingleImageConfig{},
		component.MultipleImagesConfig{},
		component.JavaScriptConfig{},
		component.LocalJSONConfig{},
		component.MenuConfig{},
		component.StyleConfig{},
		component.TextConfig{},
	}

	// Map to store documentation grouped by category
	categorizedDocs := make(map[string]map[string][]FieldDoc)

	for _, config := range types {
		t := reflect.TypeOf(config)
		v := reflect.ValueOf(config)

		// Ensure it's a struct
		if t.Kind() != reflect.Struct {
			fmt.Printf("Error: %T is not a struct\n", config)
			continue
		}

		// Get the ContentType field's value
		typeValue := ""
		_, found := t.FieldByName("ContentType")
		if found {
			value := v.FieldByName("ContentType")
			if value.IsValid() && value.Kind() == reflect.String {
				typeValue = value.String()
			}
		}

		if typeValue == "" {
			fmt.Printf("Error: %s does not have a valid ContentType value\n", t.Name())
			continue
		}

		// Generate documentation, including category
		category, docs, err := GenerateDoc(config)
		if err != nil {
			fmt.Println("Error generating doc:", err)
			return
		}

		// Use the ContentType as the key
		typeName := typeValue
		if _, found := categorizedDocs[category]; !found {
			categorizedDocs[category] = make(map[string][]FieldDoc)
		}
		categorizedDocs[category][typeName] = docs
	}

	log.Printf("Version: %s\n", Version)
	log.Printf("Build Time: %s\n", BuildTime)

	data := map[string]any{
		"data":      categorizedDocs,
		"version":   Version,
		"buildtime": BuildTime,
	}

	// Parse the HTML template
	tmpl, err := template.ParseFiles("./cmd/hyperbricks-docs/better-template.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	// Generate the static HTML file
	go renderStaticFile(tmpl, data, "docs/hyperbricks-reference-"+Version+".html")

	// HTTP handler to serve the page dynamically
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Start the HTTP server
	log.Println("Serving at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

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

// FieldDoc represents a field documentation entry
type FieldDoc struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Mapstructure string `json:"mapstructure"`
	Comment      string `json:"comment"`
	Example      string `json:"example"`
}

// GenerateDoc generates documentation for a struct, including categories
func GenerateDoc(input any) (string, []FieldDoc, error) {

	path := "./cmd/hyperbricks-docs/hyperbricks-examples/"

	t := reflect.TypeOf(input)

	// Dereference pointer types if necessary
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("input is not a struct")
	}

	var docs []FieldDoc
	category := "Uncategorized" // Default category

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check for category in the ContentType field
		if field.Name == "ContentType" {
			if cat := field.Tag.Get("category"); cat != "" {
				category = cat
			}
		}

		doc := field.Tag.Get("description")
		example := field.Tag.Get("example")
		// Only include fields that have a doc tag
		if doc != "" {
			fieldDoc := FieldDoc{
				Name:         field.Name,
				Type:         field.Type.String(),
				Mapstructure: field.Tag.Get("mapstructure"),
				Comment:      doc,
				Example:      checkAndReadFile(example, path),
			}
			docs = append(docs, fieldDoc)
		}
	}
	return category, docs, nil
}

func checkAndReadFile(input string, filePath string) string {
	// Define a regex pattern to match {{<filename.extension>}}
	re := regexp.MustCompile(`\{\!\{([^\}]+)\}\}`)

	// Find all matches
	matches := re.FindAllStringSubmatch(input, -1)

	// Process each match
	for _, match := range matches {
		// match[0] is the full placeholder (e.g., "{{api-template.hyperbricks}}")
		// match[1] is the filename.extension (e.g., "api-template.hyperbricks")

		filename := match[1]

		// Read the file content
		content, err := os.ReadFile(filePath + "/" + filename)
		if err != nil {
			// If the file is not found, replace with an error placeholder
			input = strings.ReplaceAll(input, match[0], "no example yet")
			continue
		}

		// Replace the placeholder with the file content
		input = strings.ReplaceAll(input, match[0], string(content))
	}

	return input
}
