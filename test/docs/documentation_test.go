package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
			Embedded:        map[string]string{},
			ConfigType:      "<HYPERMEDIA>",
			ConfigCategory:  "composite",
			Config:          composite.HyperMediaConfig{},
		},
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

	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	rm := render.NewRenderManager()
	fragmentRenderer := &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.FragmentConfigGetName(), fragmentRenderer, reflect.TypeOf(composite.FragmentConfig{}))

	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"api_test_template": `{{ (index .quotes 0).author }}:{{ (index .quotes 0).quote }}`,
		}
		content, exists := templates[templateName]
		return content, exists
	}

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

	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

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
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	for _, cfg := range types {
		val := reflect.ValueOf(cfg.Config)
		fmt.Print(processFieldsWithSquash(val, cfg, t, rm))

		for embeddedName, fieldTag := range cfg.Embedded {
			fmt.Printf("\nEmbedded Field: %s (mapped as: %s)\n", embeddedName, fieldTag)
			field := findFieldByName(val, embeddedName)
			if field.IsValid() {
				fmt.Print(processFieldsWithSquash(field, cfg, t, rm))
			} else {
				fmt.Printf("Field %s not found in Config\n", embeddedName)
			}
		}
	}
}

func processFieldsWithSquash(val reflect.Value, cfg DocumentationTypeStruct, t *testing.T, rm *render.RenderManager) string {
	var out strings.Builder
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return ""
	}
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		if field.Tag.Get("exclude") != "" {
			continue
		}
		tag := field.Tag.Get("mapstructure")
		if tag == ",squash" {
			out.WriteString(processFieldsWithSquash(val.Field(i), cfg, t, rm))
			continue
		}
		if tag != "" && tag != ",squash" && tag != ",remain" {
			var example string
			if field.Tag.Get("example") != "" {
				example = _checkAndReadFile(field.Tag.Get("example"), field.Tag.Get("description"))
			} else if field.Tag.Get("mapstructure") != "" {
				file := strings.ToLower(cfg.Name) + "-" + tag
				example = _checkAndReadFile("{!{"+file+".hyperbricks}}", field.Tag.Get("description"))
			}
			t.Run(strings.ToLower(cfg.Name)+"-"+tag, func(t *testing.T) {
				parsed, err := ParseContent(example)
				if err != nil {
					log.Println("Error:", err)
					return
				}
				var buf bytes.Buffer
				enc := json.NewEncoder(&buf)
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				if err := enc.Encode(parsed.ExpectedJSON); err != nil {
					log.Println("Error encoding JSON:", err)
					return
				}
				var expected map[string]interface{}
				if err := json.Unmarshal(buf.Bytes(), &expected); err != nil {
					log.Println("Error unmarshaling JSON:", err)
					return
				}
				parsedConfig := parser.ParseHyperScript(parsed.HyperbricksConfig)
				scopeData, ok := parsedConfig[parsed.HyperbricksConfigScope].(map[string]interface{})
				if !ok {
					t.Errorf("Scope '%s' not found or invalid type", parsed.HyperbricksConfigScope)
					return
				}
				req := typefactory.TypeRequest{TypeName: scopeData["@type"].(string), Data: scopeData}
				resp, err := rm.MakeInstance(req)
				if err != nil {
					t.Errorf("Error creating instance: %v", err)
					return
				}
				result, renderErr := rm.Render(req.TypeName, scopeData)
				if renderErr != nil {
					log.Printf("%v", renderErr)
				}
				resHTML := removeTabsAndNewlines(result)
				expHTML := removeTabsAndNewlines(parsed.ExpectedOutput)
				if resHTML != expHTML {
					t.Errorf("result and expected HTML differ:\nresult:\n%s\nexpected:\n%s", resHTML, expHTML)
				}
				equal, cmpErr := JSONDeepEqual(expected, resp.Instance)
				if cmpErr != nil {
					t.Fatalf("Error comparing JSON for %s-%s: %v", strings.ToLower(cfg.Name), tag, cmpErr)
				}
				if !equal {
					t.Errorf("Test failed for %s-%s!\n", strings.ToLower(cfg.Name), tag)
					fmt.Printf("output:\n%s\n", convertToJSON(resp.Instance))
					fmt.Printf("expected:\n%s\n", convertToJSON(expected))
				}
			})
		}
	}
	return out.String()
}

type ParsedContent struct {
	HyperbricksConfig      string
	HyperbricksConfigScope string
	Explainer              string
	ExpectedJSON           map[string]interface{}
	ExpectedJSONAsString   string
	ExpectedOutput         string
}

func ParseContent(content string) (*ParsedContent, error) {
	headerRegex := regexp.MustCompile(`^====\s*([^!]+?)(?:\s*\{\!\{([^}]+)\}\})?\s*====$`)
	sections := make(map[string]string)
	var currentSection, hyperbricksConfigScope string
	var sb strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		matches := headerRegex.FindStringSubmatch(l)
		if matches != nil {
			if currentSection != "" {
				sections[strings.ToLower(currentSection)] = strings.TrimSpace(sb.String())
				sb.Reset()
			}
			currentSection = strings.TrimSpace(matches[1])
			if len(matches) >= 3 {
				if strings.EqualFold(currentSection, "hyperbricks config") {
					hyperbricksConfigScope = strings.TrimSpace(matches[2])
				}
			}
		} else if currentSection != "" {
			sb.WriteString(l + "\n")
		}
	}
	if currentSection != "" {
		sections[strings.ToLower(currentSection)] = strings.TrimSpace(sb.String())
	}
	hConfig := sections["hyperbricks config"]
	expl := sections["explainer"]
	expJSONStr := sections["expected json"]
	expOutput := sections["expected output"]

	var expJSON map[string]interface{}
	if err := json.Unmarshal([]byte(expJSONStr), &expJSON); err != nil {
		return nil, fmt.Errorf("error parsing expected JSON: %v", err)
	}
	return &ParsedContent{
		HyperbricksConfig:      hConfig,
		HyperbricksConfigScope: hyperbricksConfigScope,
		Explainer:              expl,
		ExpectedJSON:           expJSON,
		ExpectedJSONAsString:   expJSONStr,
		ExpectedOutput:         expOutput,
	}, nil
}

func _checkAndReadFile(input, description string) string {
	filePath := "hyperbricks-test-files/"
	re := regexp.MustCompile(`\{\!\{([^\}]+)\}\}`)
	matches := re.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		filename := filePath + match[1]
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			f, createErr := os.Create(filename)
			if createErr != nil {
				input = strings.ReplaceAll(input, match[0], "no example yet")
				continue
			}
			defer f.Close()
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
			if _, err = f.WriteString(content); err != nil {
				return ""
			}
		}
		fileContent, err := os.ReadFile(filename)
		if err != nil {
			input = strings.ReplaceAll(input, match[0], "no example yet")
			continue
		}
		input = strings.ReplaceAll(input, match[0], string(fileContent))
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
	f := val.FieldByName(fieldName)
	if f.IsValid() {
		return f
	}
	for i := 0; i < val.NumField(); i++ {
		found := findFieldByName(val.Field(i), fieldName)
		if found.IsValid() {
			return found
		}
	}
	return reflect.Value{}
}

func removeTabsAndNewlines(s string) string {
	return regexp.MustCompile(`[\t\n\r]+`).ReplaceAllString(s, "")
}

func convertToJSON(obj interface{}) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(obj); err != nil {
		return ""
	}
	return buf.String()
}

func JSONDeepEqual(a, b interface{}) (bool, error) {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	var aJSON, bJSON interface{}
	if err := json.Unmarshal(aBytes, &aJSON); err != nil {
		return false, err
	}
	if err := json.Unmarshal(bBytes, &bJSON); err != nil {
		return false, err
	}
	return reflect.DeepEqual(aJSON, bJSON), nil
}
