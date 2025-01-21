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

	errors = append(errors, config.Validate()...)

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

	renderedOutput, _errors := applyTemplate(templateContent, responseData, config)

	if _errors != nil {
		errors = append(errors, _errors...)
	}

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

	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	req, err := http.NewRequest(config.Method, endpoint.String(), bytes.NewBufferString(config.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	if config.User != "" && config.Pass != "" {
		req.SetBasicAuth(config.User, config.Pass)
	}

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

	return data, nil
}

func applyTemplate(templateStr string, data map[string]interface{}, config APIConfig) (string, []error) {
	var errors []error

	tmpl, err := template.New("apiTemplate").Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Err:      fmt.Errorf("error parsing template: %v", err).Error(),
			Rejected: false,
		})

		return fmt.Sprintf("Error parsing template: %v", err), errors
	}

	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	if err != nil {

		errors = append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Err:      fmt.Errorf("error executing template: %v", err).Error(),
			Rejected: false,
		})
		return fmt.Sprintf("error executing template: %v", err), errors
	}

	return output.String(), errors
}
