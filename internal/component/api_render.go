package component

import (
	"bytes"
	"context"
	"encoding/json"

	"fmt"
	"html/template"
	"io"
	"testing"

	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

type APIConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"<API_RENDER> description" example:"{!{api-render-@doc.hyperbricks}}"`
	Endpoint           string                 `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-endpoint.hyperbricks}}"`
	Method             string                 `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-method.hyperbricks}}"`
	Headers            map[string]string      `mapstructure:"headers" description:"Optional HTTP headers for API requests" example:"{!{api-render-headers.hyperbricks}}"`
	Body               string                 `mapstructure:"body" description:"Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request." example:"{!{api-render-body.hyperbricks}}"`
	Template           string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{api-render-template.hyperbricks}}"`
	Inline             string                 `mapstructure:"inline" description:"Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>>" example:"{!{api-render-inline.hyperbricks}}"`
	Values             map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{api-render-values.hyperbricks}}"`
	User               string                 `mapstructure:"user" description:"User for basic auth" example:"{!{api-render-user.hyperbricks}}"`
	Pass               string                 `mapstructure:"pass" description:"User for basic auth" example:"{!{api-render-pass.hyperbricks}}"`
	Debug              bool                   `mapstructure:"debug" description:"Debug the response data" example:"{!{api-render-debug.hyperbricks}}"`
}

func APIConfigGetName() string {
	return "<API_RENDER>"
}

func APIConfigGetName_test(t *testing.T) {
	if APIConfigGetName() != "API" {
		t.Errorf("Failed")
	}
}

type APIRenderer struct {
	renderer.ComponentRenderer
}

var _ shared.ComponentRenderer = (*APIRenderer)(nil)

func (api *APIConfig) Validate() []error {
	warnings := shared.Validate(api)
	return warnings
}

func (r *APIRenderer) Types() []string {
	return []string{
		APIConfigGetName(),
	}
}

func (ar *APIRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder
	hbConfig := shared.GetHyperBricksConfiguration()

	config, ok := instance.(APIConfig)
	if !ok {
		return "", append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      fmt.Errorf("invalid type for APIRenderer").Error(),
			Rejected: true,
		})
	}

	errors = append(errors, config.Validate()...)

	responseData, err := fetchDataFromAPI(config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      fmt.Errorf("failed to fetch data from API: %w", err).Error(),
			Rejected: false,
		})
	}

	if config.Debug && hbConfig.Mode != shared.LIVE_MODE {
		jsonBytes, err := json.MarshalIndent(responseData, "", "  ")
		if err != nil {
			fmt.Println("Error marshaling struct to JSON:", err)

		}
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      "Debug in <API_RENDER> is enabled. Please disable in production",
			Rejected: false,
		})
		builder.WriteString(fmt.Sprintf("<!-- API_RENDER.debug = true -->\n<!--  <![CDATA[ \n%s\n ]]> -->", string(jsonBytes)))
	}

	var templateContent string

	if config.Inline != "" {
		templateContent = config.Inline
	} else {
		// Fetch the template content
		tc, found := ar.TemplateProvider(config.Template)
		if found {
			templateContent = tc
		} else {
			logging.GetLogger().Errorf("precached template '%s' not found, use {{TEMPLATE:sometemplate.tmpl}} for precaching", config.Template)
			// MARKER_FOR_CODE:
			// Attempt to load the file from disk and cache it.
			fileContent, err := composite.GetTemplateFileContent(config.Template)
			if err != nil {
				errors = append(errors, shared.ComponentError{
					Hash: shared.GenerateHash(),
					Key:  config.Component.Meta.HyperBricksKey,
					Path: config.Component.Meta.HyperBricksPath,
					File: config.Component.Meta.HyperBricksFile,
					Type: APIConfigGetName(),
					Err:  fmt.Errorf("failed to load template file '%s': %v", config.Template, err).Error(),
				})
			} else {
				templateContent = fileContent
			}
		}
	}

	renderedOutput, _errors := applyTemplate(templateContent, responseData, config)

	if _errors != nil {
		errors = append(errors, _errors...)
	}

	apiContent := renderedOutput
	if config.Enclose != "" {
		apiContent = shared.EncloseContent(config.Enclose, apiContent)
	}

	var jwtToken string = ""
	if ctx != nil {
		jwtToken, _ = ctx.Value(shared.JwtKey).(string)
		builder.WriteString(fmt.Sprintf("<!-- jwtToken:%s -->", jwtToken))
	}

	builder.WriteString(apiContent)

	return builder.String(), errors
}

// func fetchDataFromAPI_OldCode(config APIConfig) (interface{}, error) {
// 	jar, _ := cookiejar.New(nil)
// 	client := &http.Client{Jar: jar}

// 	endpoint, err := url.Parse(config.Endpoint)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
// 	}

// 	req, err := http.NewRequest(config.Method, endpoint.String(), bytes.NewBufferString(config.Body))
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create request: %w", err)
// 	}

// 	for key, value := range config.Headers {
// 		req.Header.Set(key, value)
// 	}

// 	if config.User != "" && config.Pass != "" {
// 		req.SetBasicAuth(config.User, config.Pass)
// 	}

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return nil, fmt.Errorf("error making HTTP request: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
// 		body, _ := io.ReadAll(resp.Body)
// 		return nil, fmt.Errorf("unexpected HTTP status code: %d, response: %s", resp.StatusCode, string(body))
// 	}

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read response body: %w", err)
// 	}

// 	var jsonArray []map[string]interface{}
// 	if err := json.Unmarshal(body, &jsonArray); err == nil {
// 		return jsonArray, nil
// 	}

// 	// If it's not an array, try to unmarshal into a map
// 	var jsonMap map[string]interface{}
// 	if err := json.Unmarshal(body, &jsonMap); err == nil {
// 		return jsonMap, nil
// 	}

// 	// If both fail, return an error
// 	return nil, fmt.Errorf("failed to parse JSON response: %s", string(body))
// }

func fetchDataFromAPI(config APIConfig) (interface{}, error) {
	// Create a new cookie jar (for production use, consider reusing an HTTP client)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}
	client := &http.Client{Jar: jar}

	// Parse the endpoint URL
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Create the HTTP request, using strings.NewReader for the body
	req, err := http.NewRequest(config.Method, endpoint.String(), strings.NewReader(config.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set request headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Set basic authentication if credentials are provided
	if config.User != "" && config.Pass != "" {
		req.SetBasicAuth(config.User, config.Pass)
	}

	// Execute the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check for a valid HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected HTTP status code: %d, response: %s", resp.StatusCode, string(body))
	}

	// Use a JSON decoder to stream decode the response directly
	var result interface{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return result, nil
}

func applyTemplate(templateStr string, data interface{}, config APIConfig) (string, []error) {
	var errors []error

	// in case of an array or object, Values is always in root and use Data to access response data...
	context := struct {
		Data   interface{}
		Values map[string]interface{}
	}{
		Data:   data,
		Values: config.Values,
	}

	tmpl, err := template.New("apiTemplate").Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      fmt.Sprintf("error parsing template: %v", err),
			Rejected: false,
		})
		return fmt.Sprintf("Error parsing template: %v", err), errors
	}

	var output bytes.Buffer
	err = tmpl.Execute(&output, context)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      fmt.Sprintf("error executing template: %v", err),
			Rejected: false,
		})
		return fmt.Sprintf("Error executing template: %v", err), errors
	}

	return output.String(), errors
}
