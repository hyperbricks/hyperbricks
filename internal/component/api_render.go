package component

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"strconv"
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
)

type APIConfig struct {
	shared.Component   `mapstructure:",squash"`
	ApiRenderConfig    `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"<API_RENDER> description" example:"{!{api-render-@doc.hyperbricks}}"`
}

type ApiRenderConfig struct {
	Endpoint         string                 `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-endpoint.hyperbricks}}"`
	Method           string                 `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-method.hyperbricks}}"`
	Headers          map[string]string      `mapstructure:"headers" description:"Optional HTTP headers for API requests" example:"{!{api-render-headers.hyperbricks}}"`
	Body             string                 `mapstructure:"body" description:"Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request." example:"{!{api-render-body.hyperbricks}}"`
	Template         string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{api-render-template.hyperbricks}}"`
	Inline           string                 `mapstructure:"inline" description:"Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>>" example:"{!{api-render-inline.hyperbricks}}"`
	Values           map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{api-render-values.hyperbricks}}"`
	Username         string                 `mapstructure:"username" description:"Username for basic auth" example:"{!{api-render-username.hyperbricks}}"`
	Password         string                 `mapstructure:"password" description:"Password for basic auth" example:"{!{api-render-password.hyperbricks}}"`
	Status           int                    `mapstructure:"status" exclude:"true"` // This adds {{.Status}} to the root level of the template data
	SetCookie        string                 `mapstructure:"setcookie" description:"Set template for cookie" example:"{!{api-render-setcookie.hyperbricks}}"`
	AllowedQueryKeys []string               `mapstructure:"querykeys" description:"Set allowed proxy query keys" example:"{!{api-render-querykeys.hyperbricks}}"`
	QueryParams      map[string]string      `mapstructure:"queryparams" description:"Set proxy query key in the confifuration" example:"{!{api-render-queryparams.hyperbricks}}"`
	JwtSecret        string                 `mapstructure:"jwtsecret" description:"When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request" example:"{!{api-render-jwt-secret.hyperbricks}}"`
	JwtClaims        map[string]string      `mapstructure:"jwtclaims" description:"jwt claim map" example:"{!{api-render-jwt-claims.hyperbricks}}"`
	Debug            bool                   `mapstructure:"debug" description:"Debug the response data" example:"{!{api-render-debug.hyperbricks}}"`
	DebugPanel       bool                   `mapstructure:"debugpanel" description:"Add frontendpanel code, this only works when frontend_errors is set to true in modules package.hyperbricks" example:"{!{api-render-debug.hyperbricks}}"`
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

func (pr *APIRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {

	//return APIConfigGetName(), nil
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

	validateErrors := config.Validate()
	errors = append(errors, validateErrors...)

	if len(validateErrors) > 0 {
		return "[validation errors]", errors
	}

	// Call function to process the request body
	status_override := false
	body, _error := processRequest(ctx, config.Body)
	if _error == nil {
		config.Body = body
	} else {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      _error.Error(),
			Rejected: false,
		})
		status_override = true
	}

	responseData, status, err := fetchDataFromAPI(config, ctx)
	if status_override {
		status = 400
	}
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
		tc, found := pr.TemplateProvider(config.Template)
		if found {
			templateContent = tc
		} else {
			//logging.GetLogger().Errorf("precached template '%s' not found, use {{TEMPLATE:sometemplate.tmpl}} for precaching", config.Template)
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
	config.Status = status
	renderedOutput, _errors := applyApiTemplate(templateContent, responseData, config)

	if _errors != nil {
		errors = append(errors, _errors...)
	}

	apiContent := renderedOutput
	if config.Enclose != "" {
		apiContent = shared.EncloseContent(config.Enclose, apiContent)
	}

	hbconfig := shared.GetHyperBricksConfiguration()
	if hbconfig.Development.FrontendErrors && hbconfig.Mode != shared.LIVE_MODE {
		if config.Debug && config.DebugPanel {
			builder.WriteString(composite.ErrorPanelTemplate)
		}
	}

	builder.WriteString(apiContent)

	return builder.String(), errors
}

func flattenFormData(formData url.Values) map[string]interface{} {
	flattened := make(map[string]interface{})

	for key, values := range formData {
		if len(values) == 1 {
			flattened[key] = values[0] // Single value → string
		} else {
			flattened[key] = values // Multiple values → []string
		}
	}

	return flattened
}

func processRequest(ctx context.Context, bodyMap string) (string, error) {
	mergedData := make(map[string]interface{})

	// Retrieve form data from context (correct type)
	formData, formOk := ctx.Value(shared.FormData).(url.Values)
	if formOk {
		// fmt.Printf("formData:%v", formData)
		flattenedForm := flattenFormData(formData)
		// fmt.Printf("flattenedForm:%v", flattenedForm)
		for key, value := range flattenedForm {
			mergedData[key] = value
		}
	}

	// Retrieve body from context
	body, bodyOk := ctx.Value(shared.RequestBody).(io.ReadCloser)
	if bodyOk {
		defer body.Close()

		// Read entire body
		bodyBytes, err := io.ReadAll(body)
		if err != nil {
			return "Failed to read request body", fmt.Errorf("failed to read request body: %w", err)
		}

		if len(bodyBytes) == 0 {
			// Empty body, do nothing
			return "", nil
		}

		// Parse JSON body into a map
		var bodyData map[string]interface{}
		err = json.Unmarshal(bodyBytes, &bodyData)
		if err != nil {
			bodyData = make(map[string]interface{}) // Default empty map on error
		}

		// Merge body data with conflict resolution
		for key, value := range bodyData {
			if _, exists := mergedData[key]; exists {
				mergedData["body_"+key] = value // Prefix duplicate keys
			} else {
				mergedData[key] = value
			}
		}
	}

	// Replace placeholders dynamically
	for key, value := range mergedData {
		placeholder := fmt.Sprintf("$%s", key)
		strValue := fmt.Sprintf("%v", value)
		bodyMap = strings.ReplaceAll(bodyMap, placeholder, strValue)
	}

	fmt.Printf("Updated body map string: %s\n", bodyMap)
	return bodyMap, nil
}

// Shared HTTP transport for connection pooling
var sharedTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	DisableKeepAlives:   false,
}

// Securely creates a new HTTP client with a unique cookie jar per request.
// - This ensures session cookies are **not shared between users**.
// - While reusing the transport for efficiency, each request has **isolated cookies**.
//
// this approach is secure with respect to cookie isolation. Here’s why:
// 	•	Unique Cookie Jar per Client: Each time you call newHttpClient(), you create a new cookie jar using cookiejar.New(nil). This ensures that each HTTP client instance has its own separate cookie store. Cookies obtained during a request using one client won’t be accessible by another.
// 	•	Shared Transport Is Safe for Connection Pooling: The sharedTransport is used solely for managing connections (for efficiency through connection pooling) and does not store or manage cookie data. The Go http.Transport is designed to be safely shared across multiple clients.
// 	•	Isolation of Session Data: Since the cookie jar is a property of the http.Client and not the transport, each client’s session cookies remain isolated. This design prevents any mix-up of cookies between different users.

// Thus, with each client using its own cookie jar, there is no risk of cookie leakage or mixing between clients even though the transport is shared for efficiency.

func newHttpClient() *http.Client {
	jar, _ := cookiejar.New(nil) // Create a new cookie jar per request to prevent cookie leaks.
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: sharedTransport, // Reuses connections efficiently while isolating cookies.
		Jar:       jar,             // Ensures cookies remain request-specific and cannot be leaked.
	}
}

// Updated fetchDataFromAPI function using newHttpClient()
func fetchDataFromAPI(config APIConfig, ctx context.Context) (interface{}, int, error) {
	client := newHttpClient() // Ensures security while reusing transport

	// Parse the endpoint URL
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Specify the allowed query keys
	allowed := []string{"id", "name", "order"}
	if config.AllowedQueryKeys != nil {
		allowed = config.AllowedQueryKeys
	}

	// Get a filtered copy of the query parameters
	clientReq, ok := ctx.Value(shared.Request).(*http.Request)
	if !ok {
		return nil, 400, fmt.Errorf("failed to extract request context")
	}

	filtered := FilterAllowedQueryParams(clientReq, allowed)

	if len(filtered) == 0 && config.QueryParams == nil {
		// Ensure endpoint.RawQuery remains empty
		endpoint.RawQuery = ""
	} else {
		params := filtered
		if config.QueryParams != nil {
			for key, value := range config.QueryParams {
				params.Add(key, value)
			}
		}
		endpoint.RawQuery = params.Encode()
	}

	// Create request
	req, err := http.NewRequest(config.Method, endpoint.String(), strings.NewReader(config.Body))
	if err != nil {
		return nil, 400, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Pass the client's "token" cookie to the outgoing request if it exists
	if tokenCookie, err := clientReq.Cookie("token"); err == nil {
		req.Header.Set("Authorization", "Bearer "+tokenCookie.Value)
	}

	// Handle JWT if secret is provided
	if config.JwtSecret != "" {
		claims := jwt.MapClaims{}
		for key, value := range config.JwtClaims {
			claims[key] = value
		}
		if _, exists := claims["sub"]; !exists {
			claims["sub"] = "default_user"
		}
		if expStr, exists := config.JwtClaims["exp"]; exists {
			expInt, err := strconv.ParseInt(expStr, 10, 64)
			if err == nil {
				claims["exp"] = time.Now().Unix() + expInt
			} else {
				claims["exp"] = time.Now().Add(time.Hour).Unix()
			}
		} else {
			claims["exp"] = time.Now().Add(time.Hour).Unix()
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JwtSecret))
		if err != nil {
			return nil, 401, fmt.Errorf("failed to sign JWT token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tokenString)
	} else if config.Username != "" && config.Password != "" {
		req.SetBasicAuth(config.Username, config.Password)
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// 🛠 Debugging: Print the full request before sending it
	if config.Debug {
		dump, err := httputil.DumpRequestOut(req, false)
		if err == nil {
			fmt.Printf("HTTP Request:\n%s\n", string(dump))
		} else {
			fmt.Printf("Failed to dump request: %v\n", err)
		}
	}

	// Handle empty response body
	if resp.Body == nil || resp.ContentLength == 0 {
		return nil, resp.StatusCode, nil
	}

	// Decode JSON response
	var result interface{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		result, resp.StatusCode, err = handleAPIResponse(resp)
		if err != nil {
			//return nil, resp.StatusCode, fmt.Errorf("failed to decode JSON response: %w", err)
		}
	}

	// 🛠 Debugging: Print the full response after receiving it
	if config.Debug {
		resdump, err := httputil.DumpResponse(resp, false)
		if err == nil {
			fmt.Printf("HTTP Response:\n%s\n", string(resdump))
		} else {
			fmt.Printf("Failed to dump Response: %v\n", err)
		}
	}

	return result, resp.StatusCode, nil
}

// Check if response is JSON
func isJSONResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/json")
}

// Check if response is XML
func isXMLResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/xml") || strings.HasPrefix(contentType, "text/xml")
}

// Handle API response dynamically
func handleAPIResponse(resp *http.Response) (interface{}, int, error) {
	var result interface{}

	// ✅ Handle JSON Response
	if isJSONResponse(resp) {
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&result); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to decode JSON response: %w", err)
		}
		return result, resp.StatusCode, nil
	}

	// ✅ Handle XML Response
	if isXMLResponse(resp) {
		var xmlResult map[string]interface{} // XML unmarshals into a struct or map
		dec := xml.NewDecoder(resp.Body)
		if err := dec.Decode(&xmlResult); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to decode XML response: %w", err)
		}
		return xmlResult, resp.StatusCode, nil
	}

	// ✅ Fallback: Read as Plain Text
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return string(bodyBytes), resp.StatusCode, nil
}

func applyApiTemplate(templateStr string, data interface{}, config APIConfig) (string, []error) {
	var errors []error

	// in case of an array or object, Values is always in root and use Data to access response data...
	context := struct {
		Data   interface{}
		Values map[string]interface{}
		Status int
	}{
		Data:   data,
		Values: config.Values,
		Status: config.Status,
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
		return "[error parsing template]", errors
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
		return "[error executing template]", errors
	}

	return output.String(), errors
}

// FilterAllowedQueryParams returns only the query parameters whose keys are in allowedKeys.
// If allowedKeys is empty, it returns an empty url.Values (no parameters).
func FilterAllowedQueryParams(req *http.Request, allowedKeys []string) url.Values {
	// If allowedKeys is empty, return an empty url.Values (no params allowed).
	if len(allowedKeys) == 0 {
		return url.Values{}
	}

	// Create a set of allowed keys for quick lookup.
	allowedSet := make(map[string]struct{})
	for _, key := range allowedKeys {
		allowedSet[key] = struct{}{}
	}

	originalQuery := req.URL.Query()
	filteredQuery := url.Values{}

	for key, values := range originalQuery {
		if _, allowed := allowedSet[key]; allowed {
			// Copy values to avoid modifying the original slice.
			filteredQuery[key] = append([]string(nil), values...)
		}
	}

	return filteredQuery
}
