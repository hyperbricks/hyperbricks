package component

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"fmt"
	"html/template"
	"io"
	"testing"

	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v4"
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

	Username  string `mapstructure:"username" description:"Username for basic auth" example:"{!{api-render-username.hyperbricks}}"`
	Password  string `mapstructure:"passpass" description:"Password for basic auth" example:"{!{api-render-password.hyperbricks}}"`
	JwtSecret string `mapstructure:"jwtsecret" description:"When not empty it uses jwtsecret for  Bearer Token Authentication. When false it uses basic auth via http.Request" example:"{!{api-render-bearer.hyperbricks}}"`
	Debug     bool   `mapstructure:"debug" description:"Debug the response data" example:"{!{api-render-debug.hyperbricks}}"`
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
	// Call function to process the request body
	config.Body = processRequest(ctx, config.Body)
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
	if config.JwtSecret == "" {
		var jwtToken string = ""
		if ctx != nil {
			jwtToken, _ = ctx.Value(shared.JwtKey).(string)
			builder.WriteString(fmt.Sprintf("<!-- jwtToken:%s -->", jwtToken))
		}
	}

	builder.WriteString(apiContent)

	return builder.String(), errors
}

func processRequest(ctx context.Context, bodyMap string) string {
	// Retrieve body from context
	body, ok := ctx.Value(shared.RequestBody).(io.ReadCloser)
	if !ok {
		fmt.Println("Failed to retrieve request body from context")
		return bodyMap
	}
	defer body.Close()

	// Read the body
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		fmt.Println("Failed to read request body")
		return bodyMap
	}

	// Parse JSON into a map
	var data map[string]interface{}
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		fmt.Println("Invalid JSON payload")
		return bodyMap
	}

	// Replace placeholders dynamically
	for key, value := range data {
		placeholder := fmt.Sprintf("$%s", key) // e.g., $username
		strValue := fmt.Sprintf("%v", value)   // Convert value to string
		bodyMap = strings.ReplaceAll(bodyMap, placeholder, strValue)
	}

	// Output the updated bodyMap string
	fmt.Printf("Updated body map string: %s\n", bodyMap)
	return bodyMap
}

func fetchDataFromAPI(config APIConfig) (interface{}, error) {
	// Create a new cookie jar
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

	fmt.Printf("config.Body:%s\n", config.Body)

	// Pass unstructured body directly
	req, err := http.NewRequest(config.Method, endpoint.String(), strings.NewReader(config.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Handle JWT if secret is provided
	if config.JwtSecret != "" {
		claims := jwt.MapClaims{
			"sub":  "superuser_id",
			"role": "postgres",
			"exp":  time.Now().Add(time.Hour * 1).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JwtSecret))
		if err != nil {
			return nil, fmt.Errorf("failed to sign JWT token: %w", err)
		}

		fmt.Printf("JWT Token: %s\n", tokenString)

		// Always set the Content-Type for JSON payload
		//req.Header.Set("Content-Type", "application/json")
		// Include the JWT token in the Authorization header
		req.Header.Set("Authorization", "Bearer "+tokenString)

	} else {
		// Set basic authentication if credentials are provided
		if config.Username != "" && config.Password != "" {
			req.SetBasicAuth(config.Username, config.Password)
		}
	}

	// 🛠 Debugging: Print the full request before sending it
	dump, err := httputil.DumpRequestOut(req, true)
	if err == nil {
		fmt.Printf("HTTP Request:\n%s\n", string(dump))
	} else {
		fmt.Printf("Failed to dump request: %v\n", err)
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

	// Decode JSON response
	var result interface{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return result, nil
}

func fetchDataFromAPIOld(config APIConfig) (interface{}, error) {
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

	if config.JwtSecret != "" {
		// Create token claims for the superuser role.
		claims := jwt.MapClaims{
			"sub":  "superuser_id",
			"role": "postgres",
			"exp":  time.Now().Add(time.Hour * 1).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JwtSecret)) // Convert secret to []byte

		if err != nil {
			fmt.Printf("error signing token: %v\n", err)
			return nil, fmt.Errorf("failed to sign JWT token: %w", err)
		}

		fmt.Printf("JWT JwtSecret:%s\n", config.JwtSecret)
		fmt.Printf("JWT Token:%s\n", tokenString)

		req.Header.Set("Content-Type", "application/json")
		// Include the JWT token in the Authorization header.
		req.Header.Set("Authorization", "Bearer "+tokenString)
	} else {
		// Set basic authentication if credentials are provided
		if config.Username != "" && config.Password != "" {
			req.SetBasicAuth(config.Username, config.Password)
		}
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
