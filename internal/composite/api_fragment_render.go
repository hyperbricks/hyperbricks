package composite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
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
	HxResponse         `mapstructure:"response" description:"HTMX response header configuration." example:"{!{fragment-response.hyperbricks}}"`
	MetaDocDescription string              `mapstructure:"@doc" description:"A <FRAGMENT> dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience." example:"{!{fragment-@doc.hyperbricks}}"`
	HxResponseWriter   http.ResponseWriter `mapstructure:"hx_response" exclude:"true"`
	Title              string              `mapstructure:"title" description:"The title of the fragment" example:"{!{fragment-title.hyperbricks}}"`
	Route              string              `mapstructure:"route" description:"The route (URL-friendly identifier) for the fragment" example:"{!{fragment-route.hyperbricks}}"`
	Section            string              `mapstructure:"section" description:"The section the fragment belongs to" example:"{!{fragment-section.hyperbricks}}"`
	Enclose            string              `mapstructure:"enclose" description:"Wrapping property for the fragment rendered output" example:"{!{fragment-enclose.hyperbricks}}"`
	NoCache            bool                `mapstructure:"nocache" description:"Explicitly deisable cache" example:"{!{fragment-nocache.hyperbricks}}"`
	Index              int                 `mapstructure:"index" description:"Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{fragment-index.hyperbricks}}"`
}

type APIConfig struct {
	Endpoint  string                 `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-endpoint.hyperbricks}}"`
	Method    string                 `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-method.hyperbricks}}"`
	Headers   map[string]string      `mapstructure:"headers" description:"Optional HTTP headers for API requests" example:"{!{api-render-headers.hyperbricks}}"`
	Body      string                 `mapstructure:"body" description:"Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request." example:"{!{api-render-body.hyperbricks}}"`
	Template  string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{api-render-template.hyperbricks}}"`
	Inline    string                 `mapstructure:"inline" description:"Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>>" example:"{!{api-render-inline.hyperbricks}}"`
	Values    map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{api-render-values.hyperbricks}}"`
	Username  string                 `mapstructure:"username" description:"Username for basic auth" example:"{!{api-render-username.hyperbricks}}"`
	Password  string                 `mapstructure:"password" description:"Password for basic auth" example:"{!{api-render-password.hyperbricks}}"`
	Status    int                    `mapstructure:"status" exclude:"true"` // This adds {{.Status}} to the root level of the template data
	SetCookie string                 `mapstructure:"setcookie" description:"Set template for cookie" example:"{!{api-render-setcookie.hyperbricks}}"`
	// PassCookie       string                 `mapstructure:"passcookie" description:"Pass a cookie in eindpoint request" example:"{!{api-render-setcookie.hyperbricks}}"`
	AllowedQueryKeys []string          `mapstructure:"querykeys" description:"Set allowed proxy query keys" example:"{!{api-render-querykeys.hyperbricks}}"`
	JwtSecret        string            `mapstructure:"jwtsecret" description:"When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request" example:"{!{api-render-jwt-secret.hyperbricks}}"`
	JwtClaims        map[string]string `mapstructure:"jwtclaims" description:"jwt claim map" example:"{!{api-render-jwt-claims.hyperbricks}}"`
	Debug            bool              `mapstructure:"debug" description:"Debug the response data" example:"{!{api-render-debug.hyperbricks}}"`
	DebugPanel       bool              `mapstructure:"debugpanel" description:"Add frontendpanel code, this only works when frontend_errors is set to true in modules package.hyperbricks" example:"{!{api-render-debug.hyperbricks}}"`
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
	body, _error := processRequest(ctx, config.Body)
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

		buf := make([]byte, 1) // Read 1 byte to check if the body is empty
		n, err := body.Read(buf)

		if err == io.EOF || n == 0 {
			// empty so fo nothing....
		} else {
			bodyBytes, err := io.ReadAll(body)
			if err != nil {
				return "Failed to read request body", fmt.Errorf("failed to read request body: %w", err)
			}

			// Parse JSON body into a map
			var bodyData map[string]interface{}
			err = json.Unmarshal(bodyBytes, &bodyData)
			if err != nil {
				return "Invalid or no JSON payload in body", fmt.Errorf("invalid or no JSON payload in body: %w", err)
			}

			// Merge body data with conflicts resolved
			for key, value := range bodyData {
				if _, exists := mergedData[key]; exists {
					mergedData["body_"+key] = value
				} else {
					mergedData[key] = value
				}
			}
		}

	}

	// Replace placeholders dynamically
	for key, value := range mergedData {
		placeholder := fmt.Sprintf("$%s", key)
		strValue := fmt.Sprintf("%v", value)
		bodyMap = strings.ReplaceAll(bodyMap, placeholder, strValue)
	}

	//fmt.Printf("Updated body map string: %s\n", bodyMap)
	return bodyMap, nil
}

func fetchDataFromAPI(config ApiFragmentRenderConfig, ctx context.Context) (interface{}, int, error) {

	// Create a new cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, 400, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second, // Add a timeout
	}
	// Parse the endpoint URL
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	///fmt.Printf("config.Body:%s\n", config.Body)

	// Specify the allowed keys.
	allowed := []string{"id", "name", "order"}
	// Specify the allowed keys.
	if config.AllowedQueryKeys != nil {
		allowed = config.AllowedQueryKeys
	}
	// Pass the client's "token" cookie to the outgoing request if it exists.
	if clientReq, ok := ctx.Value(shared.Request).(*http.Request); ok {
		// Get a filtered copy of the query parameters.
		filtered := FilterAllowedQueryParams(clientReq, allowed)
		endpoint.RawQuery = filtered.Encode()
	} else {
		return nil, 400, fmt.Errorf("failed to extract request %w", err)
	}

	// Pass unstructured body directly
	req, err := http.NewRequest(config.Method, endpoint.String(), strings.NewReader(config.Body))
	if err != nil {
		return nil, 400, fmt.Errorf("failed to create request: %w", err)
	}

	// Set custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Pass the client's "token" cookie to the outgoing request if it exists.
	if clientReq, ok := ctx.Value(shared.Request).(*http.Request); ok {
		if tokenCookie, err := clientReq.Cookie("token"); err == nil {
			req.Header.Set("Authorization", "Bearer "+tokenCookie.Value)
		}
	}

	// Handle JWT if secret is provided
	if config.JwtSecret != "" {

		claims := jwt.MapClaims{}
		// Add all claims dynamically from map
		for key, value := range config.JwtClaims {
			claims[key] = value
		}
		fmt.Printf("onfig.JwtClaims:%v", config.JwtClaims)
		// Ensure mandatory claims are set
		if _, exists := claims["sub"]; !exists {
			claims["sub"] = "default_user"
		}

		if expStr, exists := config.JwtClaims["exp"]; exists {
			expInt, err := strconv.ParseInt(expStr, 10, 64)
			if err == nil {
				claims["exp"] = time.Now().Unix() + expInt // Correct: Add to current time
			} else {
				fmt.Println("Invalid exp value, using default (1 hour)")
				claims["exp"] = time.Now().Add(time.Hour).Unix()
			}
		} else {
			claims["exp"] = time.Now().Add(time.Hour).Unix() // Default: 1-hour expiration
		}
		fmt.Printf("claims:%v", claims)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JwtSecret))
		if err != nil {
			return nil, 401, fmt.Errorf("failed to sign JWT token: %w", err)
		}

		// fmt.Printf("JWT Token: %s\n", tokenString)

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
		return nil, resp.StatusCode, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if config.Debug {

		resdump, err := httputil.DumpResponse(resp, true)
		if err == nil {
			fmt.Printf("HTTP Response:\n%s\n\n\n", string(resdump))
		} else {
			fmt.Printf("Failed to dump Response: %v\n\n\n", err)
		}
		// 🛠 Debugging: Print the full request before sending it
		dump, err := httputil.DumpRequestOut(req, false)
		if err == nil {
			fmt.Printf("HTTP Request:\n%s\n\n\n", string(dump))
		} else {
			fmt.Printf("Failed to dump request: %v\n\n\n", err)
		}
	}

	var result interface{}
	if resp.Body != nil {
		defer resp.Body.Close() // Ensure the body is closed

		// Read first byte to check if body is empty
		buf := make([]byte, 1)
		n, err := resp.Body.Read(buf)

		if err != nil && err != io.EOF {
			return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
		}

		if n == 0 { // No data in the body
			return nil, resp.StatusCode, nil
		}

		// Reset the body reader (since we already read one byte)
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf[:n]), resp.Body))

		// Decode JSON response
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&result); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to decode JSON response: %w", err)
		}
	}

	return result, resp.StatusCode, nil
}

func applyApiFragmentTemplate(templateStr string, data interface{}, config ApiFragmentRenderConfig) (string, []error) {
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

// FilterAllowedQueryParams returns a copy of the request's query parameters,
// but only includes the keys specified in allowedKeys.
func FilterAllowedQueryParams(req *http.Request, allowedKeys []string) url.Values {
	// Create a set of allowed keys for quick lookup.
	allowedSet := make(map[string]struct{})
	for _, key := range allowedKeys {
		allowedSet[key] = struct{}{}
	}

	// Get the original query parameters.
	originalQuery := req.URL.Query()
	// Create a new url.Values to hold the filtered query.
	filteredQuery := url.Values{}

	// Iterate over the original query parameters.
	for key, values := range originalQuery {
		if _, allowed := allowedSet[key]; allowed {
			// Copy the slice of values (an "array" of strings) for this key.
			filteredQuery[key] = append([]string(nil), values...)
		}
	}
	return filteredQuery
}
