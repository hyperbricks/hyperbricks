package component

import (
	"bytes"
	"encoding/json"

	"fmt"
	"html/template"
	"io"
	"regexp"
	"testing"

	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// Basic config for ComponentRenderers
type APIConfig struct {
	shared.Component `mapstructure:",squash"`
	Endpoint         string            `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-endpoint.hyperbricks}}"`
	Method           string            `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-method.hyperbricks}}"`
	Headers          map[string]string `mapstructure:"headers" description:"Optional HTTP headers for API requests" example:"{!{api-render-headers.hyperbricks}}"`
	Body             string            `mapstructure:"body" description:"Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request." example:"{!{api-render-body.hyperbricks}}"`
	Template         string            `mapstructure:"template" validate:"required" description:"Template used for rendering API output" example:"{!{api-render-template.hyperbricks}}"`
	IsTemplate       bool              `mapstructure:"istemplate"`
	User             string            `mapstructure:"user" description:"User for basic auth" example:"{!{api-render-user.hyperbricks}}"`
	Pass             string            `mapstructure:"pass" description:"User for basic auth" example:"{!{api-render-pass.hyperbricks}}"`
	Debug            bool              `mapstructure:"debug" description:"Debug the response data" example:"{!{api-render-debug.hyperbricks}}"`
}

/*

map[string]interface {}{
	"Component":shared.ComponentRendererConfig{
					Meta:shared.Meta {
							ConfigType:"API",
							ConfigCategory:"",
							Key:"", Path:""
						},
		Value:""
	},
	"Body":"test",
	"Endpoint":"http://dummyjson.com/quotes",
	"Headers":map[string]string {
		"key":"value"
	},
	"IsTemplate":false,
	"Method":"GET",
	"Pass":"",
	"Template":"api_test_template",
	"User":""
}

*/

// APIConfigGetName returns the HyperBricks type associated with the APIConfig.
func APIConfigGetName() string {
	return "<API_RENDER>"
}

func APIConfigGetName_test(t *testing.T) {
	if APIConfigGetName() != "API" {
		t.Errorf("Failed")
	}
}

// APIRenderer handles rendering of data fetched from an API endpoint.
type APIRenderer struct {
	renderer.ComponentRenderer
}

// Ensure APIRenderer implements renderer.Renderer
var _ shared.ComponentRenderer = (*APIRenderer)(nil)

// Validate ensures the API configuration is correct.
func (api *APIConfig) Validate() []error {

	// standard validation on struct metadata of APIConfig
	warnings := shared.Validate(api)

	// detect template....
	rangeRegex := regexp.MustCompile(`{{range\s+[^}]+}}`)
	input := api.Template
	api.IsTemplate = rangeRegex.MatchString(input) || strings.Contains(input, "\n") || strings.Contains(input, "{{") || strings.Contains(input, "}}")

	return warnings
}

func (r *APIRenderer) Types() []string {
	return []string{
		APIConfigGetName(),
	}
}

// Render processes API data and outputs it according to the specified template.
func (ar *APIRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder
	config, ok := instance.(APIConfig)
	if !ok {
		return "", append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Err:      fmt.Errorf("invalid type for APIRenderer").Error(),
			Rejected: true,
		})
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// Fetch data from the API
	responseData, err := fetchDataFromAPI(config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Err:      fmt.Errorf("failed to fetch data from API: %w", err).Error(),
			Rejected: false,
		})
	}

	if config.Debug {
		// Convert the struct to JSON
		jsonBytes, err := json.MarshalIndent(responseData, "", "  ")
		if err != nil {
			fmt.Println("Error marshaling struct to JSON:", err)

		}
		builder.WriteString(fmt.Sprintf("<!-- API_RENDER.debug = true -->\n<!--  <![CDATA[ \n%s\n ]]> -->", string(jsonBytes)))
	}

	var templateContent string
	if config.IsTemplate {
		templateContent = config.Template
	} else {
		// Fetch the template content
		tc, found := ar.TemplateProvider(config.Template)
		if !found {
			return "",
				append(errors, shared.ComponentError{
					Path:     config.Path,
					Key:      config.Key,
					Err:      fmt.Errorf("<!-- Template '%s' not found -->", config.Template).Error(),
					Rejected: false,
				})
		} else {
			templateContent = tc
		}

	}

	// Apply the template
	renderedOutput, _errors := applyTemplate(templateContent, responseData, config)

	if _errors != nil {
		errors = append(errors, _errors...)
	}

	// Apply enlose if specified
	apiContent := renderedOutput
	if config.Enclose != "" {
		apiContent = shared.EncloseContent(config.Enclose, apiContent)
	}

	builder.WriteString(apiContent)

	return builder.String(), errors
}

func fetchDataFromAPI(config APIConfig) (map[string]interface{}, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Parse URL and add query parameters if needed
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	req, err := http.NewRequest(config.Method, endpoint.String(), bytes.NewBufferString(config.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Set Basic Auth if credentials are provided
	if config.User != "" && config.Pass != "" {
		req.SetBasicAuth(config.User, config.Pass)
	}

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected HTTP status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Step 4: Output cookies stored in the jar
	// fmt.Println("\nCookies stored in the jar:")
	// u, _ := url.Parse(config.Endpoint)
	// for _, cookie := range jar.Cookies(u) {
	// 	fmt.Printf("Cookie: %s = %s\n", cookie.Name, cookie.Value)
	// 	fmt.Printf("Path: %s, Domain: %s, Expires: %v\n", cookie.Path, cookie.Domain, cookie.Expires)
	// }

	return data, nil
}

// applyTemplate generates output based on the provided template and API data.
func applyTemplate(templateStr string, data map[string]interface{}, config APIConfig) (string, []error) {
	var errors []error

	// Parse the template string
	tmpl, err := template.New("apiTemplate").Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Err:      fmt.Errorf("error parsing template: %v", err).Error(),
			Rejected: false,
		})
		// Handle parsing error gracefully
		return fmt.Sprintf("Error parsing template: %v", err), errors
	}

	// Execute the template with the provided data
	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	if err != nil {
		// Handle execution error gracefully
		errors = append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Err:      fmt.Errorf("error executing template: %v", err).Error(),
			Rejected: false,
		})
		return fmt.Sprintf("error executing template: %v", err), errors
	}

	// Return the rendered output
	return output.String(), errors
}
