package main

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
)

type DocumentationTypeStruct struct {
	Name            string
	TypeDescription string
	ConfigType      string
	ConfigCategory  string
	Fields          map[string]string
	Embedded        map[string]string
	Config          any
}

func Test_TestAndDocumentationRender(t *testing.T) {
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
			Embedded: map[string]string{
				"HxResponse": "response",
			},
			ConfigType:     "<HYPERMEDIA>",
			ConfigCategory: "composite",
			Config:         composite.HyperMediaConfig{},
		},
		{
			Name:            "Api",
			TypeDescription: "Basic type description here.....",
			Embedded: map[string]string{
				"HxResponse": "response",
			},
			ConfigType:     "<API>",
			ConfigCategory: "composite",
			Config:         composite.HxApiConfig{},
		},
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

	for _, cfg := range types {
		fmt.Printf("\n\n======= Processing type: %s =======\n", cfg.Name)

		// Process non-embedded fields first
		val := reflect.ValueOf(cfg.Config)
		fmt.Print(processFieldsWithSquash(val, cfg))

		//fmt.Print(processNonEmbeddedFields(val, cfg))

		// Iterate through embedded fields
		for embeddedName, fieldTag := range cfg.Embedded {
			fmt.Printf("\nEmbedded Field: %s (mapped as: %s)\n", embeddedName, fieldTag)

			field := findFieldByName(val, embeddedName)
			if field.IsValid() {
				fmt.Print(processFieldsWithSquash(field, cfg))
			} else {
				fmt.Printf("Field %s not found in Config\n", embeddedName)
			}
		}
	}
}

func processNonEmbeddedFields(val reflect.Value, cfg DocumentationTypeStruct) string {
	var out strings.Builder
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		fmt.Println("Not a struct, skipping.")
		return ""
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		if field.Anonymous {
			continue // Skip embedded fields
		}
		tag := field.Tag

		if tag.Get("mapstructure") != "" {
			out.WriteString(fmt.Sprintf("Field: %s ->%s\n", tag.Get("mapstructure"), field.Name))
			out.WriteString(fmt.Sprintf("description: %s\n", field.Tag.Get("description")))
			if field.Tag.Get("example") != "" {
				example := _checkAndReadFile(field.Tag.Get("example"))
				out.WriteString(fmt.Sprintf("example: %s\n", example))
			} else if field.Tag.Get("mapstructure") != "" {
				file := strings.ToLower(cfg.Name) + "-" + tag.Get("mapstructure")
				example := _checkAndReadFile("{!{" + file + "}}")
				out.WriteString(fmt.Sprintf("example: %s\n", example))
			}
		}
		//
		//out.WriteString(fmt.Sprintf("File: %s\n", file))
		//out.WriteString(fmt.Sprintf("\tmapstructure: %s\n", tag))
		//out.WriteString(fmt.Sprintf("\tdescription: %s\n", field.Tag.Get("description")))
		//out.WriteString(fmt.Sprintf("\texample: %s\n", field.Tag.Get("example")))

	}
	return out.String()
}

func processFields(val reflect.Value, cfg DocumentationTypeStruct) string {

	var out strings.Builder
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		fmt.Println("Not a struct, skipping.")
		return ""
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		tag := field.Tag

		if tag.Get("mapstructure") != "" {
			out.WriteString(fmt.Sprintf("Field: %s ->%s\n", tag.Get("mapstructure"), field.Name))
			out.WriteString(fmt.Sprintf("description: %s\n", field.Tag.Get("description")))
			if field.Tag.Get("example") != "" {
				example := _checkAndReadFile(field.Tag.Get("example"))
				out.WriteString(fmt.Sprintf("example: %s\n", example))
			} else if field.Tag.Get("mapstructure") != "" {
				file := strings.ToLower(cfg.Name) + "-" + tag.Get("mapstructure")
				example := _checkAndReadFile("{!{" + file + "}}")
				out.WriteString(fmt.Sprintf("example: %s\n", example))
			}
		}

		//file := strings.ToLower(cfg.Name) + "-" + tag.Get("mapstructure")
		//out.WriteString(fmt.Sprintf("File: %s\n", file))
		//out.WriteString(fmt.Sprintf("\tmapstructure: %s\n", tag))
		//out.WriteString(fmt.Sprintf("\tdescription: %s\n", field.Tag.Get("description")))
		//out.WriteString(fmt.Sprintf("\texample: %s\n", field.Tag.Get("example")))

	}
	return out.String()
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

func processFieldsWithSquash(val reflect.Value, cfg DocumentationTypeStruct) string {
	var out strings.Builder

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		fmt.Println("Not a struct, skipping.")
		return ""
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == ",squash" {
			// Recursively process embedded fields
			out.WriteString(processFieldsWithSquash(val.Field(i), cfg))

		}

		if tag != "" && tag != ",squash" && tag != ",remain" {
			out.WriteString(fmt.Sprintf("Field: %s ->%s\n", tag, field.Name))
			out.WriteString(fmt.Sprintf("description: %s\n", field.Tag.Get("description")))
			if field.Tag.Get("example") != "" {
				example := _checkAndReadFile(field.Tag.Get("example"))
				out.WriteString(fmt.Sprintf("example: %s\n", example))
			} else if field.Tag.Get("mapstructure") != "" {
				file := strings.ToLower(cfg.Name) + "-" + tag
				example := _checkAndReadFile("{!{" + file + "}}")
				out.WriteString(fmt.Sprintf("example: %s\n", example))
			}
		}

		//file := strings.ToLower(cfg.Name) + "-" + tag
		//out.WriteString(fmt.Sprintf("File: %s\n", file))
		//out.WriteString(fmt.Sprintf("\tmapstructure: %s\n", tag))
		//out.WriteString(fmt.Sprintf("\tdescription: %s\n", field.Tag.Get("description")))
		//out.WriteString(fmt.Sprintf("\texample: %s\n", field.Tag.Get("example")))

	}
	return out.String()
}

func _checkAndReadFile(input string) string {
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
			f.Close()
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
