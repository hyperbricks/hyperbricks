package composite

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// FragmentConfig represents configuration for a single fragment.
type ApiFragmentRenderConfig struct {
	shared.Composite   `mapstructure:",squash"`
	APIConfig          `mapstructure:",squash"`
	HxResponse         `mapstructure:"response" description:"HTMX response header configuration." example:"{!{api-fragment-render-response.hyperbricks}}"`
	MetaDocDescription string              `mapstructure:"@doc" description:"A <FRAGMENT> dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience." example:"{!{api-fragment-render-@doc.hyperbricks}}"`
	HxResponseWriter   http.ResponseWriter `mapstructure:"hx_response" exclude:"true"`
	Title              string              `mapstructure:"title" description:"The title of the fragment" example:"{!{api-fragment-render-title.hyperbricks}}"`
	Route              string              `mapstructure:"route" description:"The route (URL-friendly identifier) for the fragment" example:"{!{api-fragment-render-route.hyperbricks}}"`
	Section            string              `mapstructure:"section" description:"The section the fragment belongs to" example:"{!{api-fragment-render-section.hyperbricks}}"`
	Enclose            string              `mapstructure:"enclose" description:"Wrapping property for the fragment rendered output" example:"{!{api-fragment-render-enclose.hyperbricks}}"`
	NoCache            bool                `mapstructure:"nocache" description:"Explicitly deisable cache" example:"{!{api-fragment-render-nocache.hyperbricks}}"`
	Index              int                 `mapstructure:"index" description:"Index number is a sort order option for the api-fragment-render menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{fragment-index.hyperbricks}}"`
}

type APIConfig struct {
	Endpoint  string                 `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-fragment-endpoint.hyperbricks}}"`
	Method    string                 `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-fragment-method.hyperbricks}}"`
	Headers   map[string]string      `mapstructure:"headers" description:"Optional HTTP headers for API requests" example:"{!{api-render-fragment-headers.hyperbricks}}"`
	Body      string                 `mapstructure:"body" description:"Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request." example:"{!{api-render-fragment-body.hyperbricks}}"`
	Template  string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{api-render-fragment-template.hyperbricks}}"`
	Inline    string                 `mapstructure:"inline" description:"Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>>" example:"{!{api-render-fragment-inline.hyperbricks}}"`
	Values    map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{api-render-fragment-values.hyperbricks}}"`
	Username  string                 `mapstructure:"username" description:"Username for basic auth" example:"{!{api-render-fragment-username.hyperbricks}}"`
	Password  string                 `mapstructure:"password" description:"Password for basic auth" example:"{!{api-render-fragment-password.hyperbricks}}"`
	Status    int                    `mapstructure:"status" exclude:"true"` // This adds {{.Status}} to the root level of the template data
	SetCookie string                 `mapstructure:"setcookie" description:"Set template for cookie" example:"{!{api-render-fragment-setcookie.hyperbricks}}"`
	// PassCookie       string                 `mapstructure:"passcookie" description:"Pass a cookie in eindpoint request" example:"{!{api-render-setcookie.hyperbricks}}"`
	AllowedQueryKeys []string          `mapstructure:"querykeys" description:"Set allowed proxy query keys" example:"{!{api-render-fragment-querykeys.hyperbricks}}"`
	QueryParams      map[string]string `mapstructure:"queryparams" description:"Set proxy query key in the confifuration" example:"{!{api-render-fragment-queryparams.hyperbricks}}"`
	JwtSecret        string            `mapstructure:"jwtsecret" description:"When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request" example:"{!{api-render-fragment-jwt-secret.hyperbricks}}"`
	JwtClaims        map[string]string `mapstructure:"jwtclaims" description:"jwt claim map" example:"{!{api-render-fragment-jwt-claims.hyperbricks}}"`
	Debug            bool              `mapstructure:"debug" description:"Debug the response data" example:"{!{api-render-fragment-debug.hyperbricks}}"`
	DebugPanel       bool              `mapstructure:"debugpanel" description:"Add frontendpanel code, this only works when frontend_errors is set to true in modules package.hyperbricks" example:"{!{api-render-fragment-debug.hyperbricks}}"`
}

// FragmentConfigGetName returns the HyperBricks type associated with the FragmentConfig.
func ApiFragmentRenderConfigGetName() string {
	return "<API_FRAGMENT_RENDER>"
}

// Validate ensures that the fragment has valid data.
func (conf *ApiFragmentRenderConfig) Validate() []error {
	var errors []error
	// = shared.Validate(conf)

	if conf.Endpoint == "" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      conf.Composite.Meta.HyperBricksKey,
			Path:     conf.Composite.Meta.HyperBricksPath,
			File:     conf.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      "[field 'endpoint' is required]",
			Rejected: false,
		})
	}

	if conf.Method == "" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      conf.Composite.Meta.HyperBricksKey,
			Path:     conf.Composite.Meta.HyperBricksPath,
			File:     conf.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      "[field 'method' is required]",
			Rejected: false,
		})
	}
	if conf.Route == "" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      conf.Composite.Meta.HyperBricksKey,
			Path:     conf.Composite.Meta.HyperBricksPath,
			File:     conf.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      "[field 'Route' is required]",
			Rejected: false,
		})
	}

	if conf.Route == "" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      conf.Composite.Meta.HyperBricksKey,
			Path:     conf.Composite.Meta.HyperBricksPath,
			File:     conf.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      "[field 'Route' is required]",
			Rejected: false,
		})
	}
	return errors
}

// ApiFragmentRenderer handles rendering of PAGE content.
type ApiFragmentRenderer struct {
	renderer.CompositeRenderer
}

// Ensure ApiFragmentRenderer implements renderer.RenderComponent interface.
var _ shared.CompositeRenderer = (*ApiFragmentRenderer)(nil)

func (r *ApiFragmentRenderer) Types() []string {
	return []string{
		ApiFragmentRenderConfigGetName(),
	}
}

// Render implements the RenderComponent interface.
func (pr *ApiFragmentRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {

	//return ApiFragmentRenderConfigGetName(), nil
	var errors []error
	var builder strings.Builder
	hbConfig := shared.GetHyperBricksConfiguration()

	config, ok := instance.(ApiFragmentRenderConfig)
	if !ok {
		return "", append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      fmt.Errorf("invalid type for APIRenderer").Error(),
			Rejected: true,
		})
	}

	config.NoCache = true

	validateErrors := config.Validate()
	errors = append(errors, validateErrors...)

	if len(validateErrors) > 0 {
		return "[validation errors]", errors
	}

	// Call function to process the request body
	status_override := false
	body, _error := processRequest(ctx, config)
	if _error == nil {
		config.Body = body
	} else {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
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
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
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
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
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
			fileContent, err := GetTemplateFileContent(config.Template)
			if err != nil {
				errors = append(errors, shared.ComponentError{
					Hash: shared.GenerateHash(),
					Key:  config.Composite.Meta.HyperBricksKey,
					Path: config.Composite.Meta.HyperBricksPath,
					File: config.Composite.Meta.HyperBricksFile,
					Type: ApiFragmentRenderConfigGetName(),
					Err:  fmt.Errorf("failed to load template file '%s': %v", config.Template, err).Error(),
				})
			} else {
				templateContent = fileContent
			}
		}
	}
	config.Status = status
	renderedOutput, _errors := applyApiFragmentTemplate(templateContent, responseData, config)

	if _errors != nil {
		errors = append(errors, _errors...)
	}

	apiContent := renderedOutput
	if config.Enclose != "" {
		apiContent = shared.EncloseContent(config.Enclose, apiContent)
	}
	// if config.JwtSecret == "" {
	// 	var jwtToken string = ""
	// 	if ctx != nil {
	// 		jwtToken, _ = ctx.Value(shared.JwtKey).(string)
	// 		//builder.WriteString(fmt.Sprintf("<!-- jwtToken:%s -->", jwtToken))
	// 	}
	// }

	writer := ctx.Value(shared.ResponseWriter).(http.ResponseWriter)
	if config.SetCookie != "" && status == 200 {

		// tmplItem, err := template.New("item").Parse(config.SetCookie)
		// if err != nil {
		// 	errors = append(errors, fmt.Errorf("failed to parse 'item' template: %w", err))
		// }

		// var buf strings.Builder
		// err = tmplItem.Execute(&buf, responseData)
		// if err != nil {
		// 	errors = append(errors, fmt.Errorf("failed to execute template: %w", err))
		// }

		cookie, _errors := applyApiFragmentTemplate(config.SetCookie, responseData, config)
		config.SetCookie = cookie
		if _errors != nil {
			errors = append(errors, _errors...)
		} else {
			if writer != nil {
				writer.Header().Set("Set-Cookie", cookie)
			}
		}
	}
	hbconfig := shared.GetHyperBricksConfiguration()
	if hbconfig.Development.FrontendErrors && hbconfig.Mode != shared.LIVE_MODE {
		if config.Debug && config.DebugPanel {
			builder.WriteString(ErrorPanelTemplate)
		}
	}

	// 🛠 Debugging
	if config.Debug {
		// Convert map to pretty JSON
		prettyJSON, err := json.MarshalIndent(writer, "", "  ")
		if err != nil {
			fmt.Println("Error formatting JSON:", err)

		}

		// Print formatted JSON
		fmt.Printf("HyperBricks Response:\n%s\n", string(prettyJSON))
	}

	builder.WriteString(apiContent)
	if config.HxResponseWriter != nil {
		SetHeadersFromHxRequest(&config.HxResponse, writer)
	}

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

func processRequest(ctx context.Context, config ApiFragmentRenderConfig) (string, error) {
	mergedData := make(map[string]interface{})

	// Specify the allowed query keys
	allowed := []string{"id", "name", "order"}
	if config.AllowedQueryKeys != nil {
		allowed = config.AllowedQueryKeys
	}
	var filtered = url.Values{}

	// Get a filtered copy of the query parameters
	clientReq, ok := ctx.Value(shared.Request).(*http.Request)
	if ok {
		filtered = FilterAllowedQueryParams(clientReq, allowed)
		for key, value := range filtered {
			mergedData[key] = value
		}
	}

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
			// Empty body, return the unmapped body
			return config.Body, nil
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
	} else {
		fmt.Printf("No body provided with post...: %s\n", config.Body)
	}

	// Replace placeholders dynamically
	for key, value := range mergedData {
		placeholder := fmt.Sprintf("$%s", key)
		strValue := fmt.Sprintf("%v", value)
		config.Body = strings.ReplaceAll(config.Body, placeholder, strValue)
	}

	fmt.Printf("Updated body map string: %s\n", config.Body)
	return config.Body, nil
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
func fetchDataFromAPI(config ApiFragmentRenderConfig, ctx context.Context) (interface{}, int, error) {
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

func applyApiFragmentTemplate(templateStr string, data interface{}, config ApiFragmentRenderConfig) (string, []error) {
	var errors []error

	context := map[string]interface{}{
		"Data":   data, // Ensure Data is explicitly typed as interface{}
		"Status": config.Status,
	}

	// Merge config.Values into the root
	for k, v := range config.Values {
		context[k] = v
	}

	tmpl, err := shared.GenericTemplate().Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
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
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
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
