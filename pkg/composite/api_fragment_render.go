package composite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
)

// FragmentConfig represents configuration for a single fragment.
type ApiFragmentRenderConfig struct {
	shared.Composite   `mapstructure:",squash"`
	APIConfig          `mapstructure:",squash"`
	Response           HTTPResponseConfig `mapstructure:"response" description:"Browser HTTP status and headers; separate from upstream request headers"`
	MetaDocDescription string             `mapstructure:"@doc" description:"Route-owning API fragment that always bypasses rendered-output caching and makes a fresh upstream request when invoked." example:"{!{api-fragment-render-@doc.hyperbricks.yaml}}"`
	Title              string             `mapstructure:"title" description:"The title of the fragment" example:"{!{api-fragment-render-title.hyperbricks.yaml}}"`
	Route              string             `mapstructure:"route" description:"The route (URL-friendly identifier) for the fragment" example:"{!{api-fragment-render-route.hyperbricks.yaml}}"`
	Section            string             `mapstructure:"section" description:"The section the fragment belongs to" example:"{!{api-fragment-render-section.hyperbricks.yaml}}"`
	Enclose            string             `mapstructure:"enclose" description:"Wrapping property for the fragment rendered output" example:"{!{api-fragment-render-enclose.hyperbricks.yaml}}"`
	NoCache            bool               `mapstructure:"nocache" exclude:"true"` // description:"Explicitly disable cache" example:"{!{api-fragment-render-nocache.hyperbricks.yaml}}"`
	Index              int                `mapstructure:"index" description:"Index number is a sort order option for the api-fragment-render menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{fragment-index.hyperbricks.yaml}}"`
	Guard              *RouteGuardConfig  `mapstructure:"guard" json:",omitempty" description:"Optional pre-render route guard. When omitted or disabled, current API_FRAGMENT_RENDER behavior remains unchanged"`
	rawSetCookies      []interface{}
}

type APIConfig struct {
	Endpoint     string                 `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-fragment-endpoint.hyperbricks.yaml}}"`
	ForwardToken string                 `mapstructure:"forwardtoken" json:",omitempty" description:"Exact incoming cookie name to forward as Bearer. Omitted or empty disables forwarding. String only; mutually exclusive with other authentication sources" example:"{!{api-render-fragment-forwardtoken.hyperbricks.yaml}}"`
	Method       string                 `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-fragment-method.hyperbricks.yaml}}"`
	Headers      map[string]string      `mapstructure:"headers" description:"Explicit upstream headers. Authorization, JWT, Basic Auth and forwardtoken are mutually exclusive authentication sources" example:"{!{api-render-fragment-headers.hyperbricks.yaml}}"`
	Body         string                 `mapstructure:"body" description:"Raw request body. Use a scalar string value; nested objects are not parsed for this field." example:"{!{api-render-fragment-body.hyperbricks.yaml}}"`
	Template     string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{api-render-fragment-template.hyperbricks.yaml}}"`
	Inline       string                 `mapstructure:"inline" description:"Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines." example:"{!{api-render-fragment-inline.hyperbricks.yaml}}"`
	Values       map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{api-render-fragment-values.hyperbricks.yaml}}"`
	Username     string                 `mapstructure:"username" description:"Basic Auth username; both username and password are required" example:"{!{api-render-fragment-username.hyperbricks.yaml}}"`
	Password     string                 `mapstructure:"password" description:"Basic Auth password; both username and password are required" example:"{!{api-render-fragment-password.hyperbricks.yaml}}"`
	Status       int                    `mapstructure:"status" exclude:"true"` // This adds {{.Status}} to the root level of the template data
	SetCookie    string                 `mapstructure:"setcookie" description:"Legacy Set-Cookie shorthand. Only the value may be templated; validated and emitted atomically after successful upstream and fragment rendering." example:"{!{api-render-fragment-setcookie.hyperbricks.yaml}}"`
	SetCookies   []interface{}          `mapstructure:"setcookies" json:",omitempty" description:"List of structured cookie configurations or legacy strings. Cookie values are validated separately; all headers are emitted together only after successful rendering." example:"{!{api-render-fragment-setcookies.hyperbricks.yaml}}"`
	// PassCookie       string                 `mapstructure:"passcookie" description:"Pass a cookie in eindpoint request" example:"{!{api-render-setcookie.hyperbricks.yaml}}"`
	AllowedQueryKeys []string          `mapstructure:"querykeys" description:"Set allowed proxy query keys" example:"{!{api-render-fragment-querykeys.hyperbricks.yaml}}"`
	QueryParams      map[string]string `mapstructure:"queryparams" description:"Set proxy query keys in the configuration" example:"{!{api-render-fragment-queryparams.hyperbricks.yaml}}"`
	JwtSecret        string            `mapstructure:"jwtsecret" description:"Signs jwtclaims as the sole upstream authentication source; cannot be combined with Basic Auth, Authorization or forwardtoken" example:"{!{api-render-fragment-jwt-secret.hyperbricks.yaml}}"`
	JwtClaims        map[string]string `mapstructure:"jwtclaims" description:"JWT claims to include when signing the bearer token" example:"{!{api-render-fragment-jwt-claims.hyperbricks.yaml}}"`
	Debug            bool              `mapstructure:"debug" description:"Log request and response metadata only; never header values, URL paths or queries, or payloads" example:"{!{api-render-fragment-debug.hyperbricks.yaml}}"`
	DebugPanel       bool              `mapstructure:"debugpanel" description:"Render a frontend debug panel when frontend_errors is enabled in modules package.hyperbricks.yaml" example:"{!{api-render-fragment-debug.hyperbricks.yaml}}"`
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
	endpoint, err := url.Parse(conf.Endpoint)
	if err != nil {
		err = fmt.Errorf("invalid API endpoint URL")
	} else {
		err = apiutil.ValidateAuthSettings(endpoint, shared.GetHyperBricksConfiguration().Mode, conf.authSettings())
	}
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Type: ApiFragmentRenderConfigGetName(), Err: err.Error(), Rejected: true,
			Key: conf.Composite.Meta.HyperBricksKey, Path: conf.Composite.Meta.HyperBricksPath,
			File: conf.Composite.Meta.HyperBricksFile,
		})
	}
	for key := range conf.Values {
		if key == "Data" || key == "Status" {
			errors = append(errors, shared.ComponentError{Type: ApiFragmentRenderConfigGetName(), Err: "API values cannot override reserved Data or Status", Rejected: true})
		}
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
	if ctx == nil {
		ctx = context.Background()
	}

	//return ApiFragmentRenderConfigGetName(), nil
	var errors []error
	var builder strings.Builder

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
	body, requestErr := processRequest(ctx, config)
	if requestErr != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      requestErr.Error(),
			Rejected: false,
		})
		return "[request body error]", errors
	}
	config.Body = body

	responseData, status, err := fetchDataFromAPI(config, ctx)
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
	// Cookies are staged only after the body and upstream processing succeed.
	// Nothing is added to the response if any configured cookie fails.
	if len(errors) == 0 && status >= http.StatusOK && status < http.StatusMultipleChoices {
		incoming, _ := ctx.Value(shared.Request).(*http.Request)
		cookies, cookieErr := RenderAPIResponseCookies(config.SetCookie, config.responseCookieEntries(), responseData, status, config.Values, incoming)
		if cookieErr != nil {
			errors = append(errors, shared.ComponentError{Type: ApiFragmentRenderConfigGetName(), Err: cookieErr.Error(), Rejected: true})
		} else if len(cookies) > 0 {
			if capture, _ := ctx.Value(shared.APIResponseCookieCaptureKey).(*shared.APIResponseCookieCapture); capture != nil {
				if err := capture.Store(cookies); err != nil {
					errors = append(errors, shared.ComponentError{Type: ApiFragmentRenderConfigGetName(), Err: err.Error(), Rejected: true})
				}
			} else {
				// Standalone component callers historically provide only a writer.
				// The helper has already validated this component's complete group,
				// so that compatibility path retains component-level atomicity.
				writer, _ := ctx.Value(shared.ResponseWriter).(http.ResponseWriter)
				if writer == nil {
					errors = append(errors, shared.ComponentError{Type: ApiFragmentRenderConfigGetName(), Err: "missing response writer or API cookie capture", Rejected: true})
				} else {
					for _, cookie := range cookies {
						writer.Header().Add("Set-Cookie", cookie)
					}
				}
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

func processRequest(ctx context.Context, config ApiFragmentRenderConfig) (string, error) {
	mergedData := make(map[string]interface{})

	// Specify the allowed query keys
	allowed := apiutil.DefaultQueryKeys
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
		flattenedForm := apiutil.FlattenFormData(formData)
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
			return "", fmt.Errorf("failed to read request body")
		}

		if len(bodyBytes) > 0 {
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

	}

	config.Body = replaceAPIBodyPlaceholders(config.Body, mergedData)

	return config.Body, nil
}

func replaceAPIBodyPlaceholders(templateBody string, mergedData map[string]interface{}) string {
	re := regexp.MustCompile(`\$([A-Za-z0-9_]+)\b`)
	return re.ReplaceAllStringFunc(templateBody, func(match string) string {
		key := strings.TrimPrefix(match, "$")
		value, ok := mergedData[key]
		if !ok {
			return match
		}

		if s, ok := value.(string); ok {
			escaped, _ := json.Marshal(s)
			return strings.Trim(string(escaped), `"`)
		}

		return fmt.Sprintf("%v", value)
	})
}

// fetchDataFromAPI applies the API component's explicit credential policy.
func fetchDataFromAPI(config ApiFragmentRenderConfig, ctx context.Context) (interface{}, int, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid API endpoint URL")
	}
	if err := apiutil.ValidateAuthSettings(endpoint, shared.GetHyperBricksConfiguration().Mode, config.authSettings()); err != nil {
		return nil, 400, err
	}
	if ctx == nil {
		return nil, 400, fmt.Errorf("missing API request context")
	}
	incoming, ok := ctx.Value(shared.Request).(*http.Request)
	if !ok || incoming == nil {
		return nil, 400, fmt.Errorf("missing API request context")
	}
	allowed := apiutil.DefaultQueryKeys
	if config.AllowedQueryKeys != nil {
		allowed = config.AllowedQueryKeys
	}
	params := endpoint.Query()
	for key, values := range FilterAllowedQueryParams(incoming, allowed) {
		for _, value := range values {
			params.Add(key, value)
		}
	}
	for key, value := range config.QueryParams {
		params.Add(key, value)
	}
	endpoint.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, config.Method, endpoint.String(), strings.NewReader(config.Body))
	if err != nil {
		return nil, 400, fmt.Errorf("invalid upstream request")
	}
	if err := apiutil.ApplyAuth(req, incoming, config.authSettings()); err != nil {
		return nil, 400, err
	}
	if config.Debug {
		fmt.Printf("API request: %+v\n", apiutil.DescribeRequest(req))
	}
	resp, err := apiutil.NewAPIHTTPClient().Do(req)
	if err != nil {
		return nil, 502, apiutil.SafeRequestError("upstream request", req, err)
	}
	defer resp.Body.Close()
	if config.Debug {
		fmt.Printf("API response: %+v\n", apiutil.DescribeResponse(resp))
	}
	result, err := apiutil.DecodeAPIResponse(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return result, resp.StatusCode, nil
}

func applyApiFragmentTemplate(templateStr string, data interface{}, config ApiFragmentRenderConfig) (string, []error) {
	var errors []error

	context := map[string]interface{}{
		"Data":   data, // Ensure Data is explicitly typed as interface{}
		"Status": config.Status,
	}

	// Merge config.Values into the root
	for k, v := range config.Values {
		if k != "Data" && k != "Status" {
			context[k] = v
		}
	}

	tmpl, err := shared.GenericTemplate().Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     ApiFragmentRenderConfigGetName(),
			Err:      "error parsing API template",
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
			Err:      "error executing API template",
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
