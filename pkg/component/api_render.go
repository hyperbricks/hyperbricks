package component

import (
	"bytes"
	"context"
	"encoding/json"

	"fmt"
	"io"

	"net/http"
	"net/url"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
)

type APIConfig struct {
	shared.Component   `mapstructure:",squash"`
	ApiRenderConfig    `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Nested API fetcher with no upstream-response cache. It renders the upstream response through a template; its parent route owns rendered-output caching." example:"{!{api-render-@doc.hyperbricks.yaml}}"`
}

type ApiRenderConfig struct {
	Endpoint         string                 `mapstructure:"endpoint" validate:"required" description:"The API endpoint" example:"{!{api-render-endpoint.hyperbricks.yaml}}"`
	ForwardToken     string                 `mapstructure:"forwardtoken" json:",omitempty" description:"Exact incoming cookie name to forward as Bearer. Omitted or empty disables forwarding. String only; mutually exclusive with other authentication sources" example:"{!{api-render-forwardtoken.hyperbricks.yaml}}"`
	Method           string                 `mapstructure:"method" validate:"required" description:"HTTP method to use for API calls, GET POST PUT DELETE etc... " example:"{!{api-render-method.hyperbricks.yaml}}"`
	Headers          map[string]string      `mapstructure:"headers" description:"Explicit upstream headers. Authorization, JWT, Basic Auth and forwardtoken are mutually exclusive authentication sources" example:"{!{api-render-headers.hyperbricks.yaml}}"`
	Body             string                 `mapstructure:"body" description:"Raw request body. Use a scalar string value; nested objects are not parsed for this field." example:"{!{api-render-body.hyperbricks.yaml}}"`
	Template         string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{api-render-template.hyperbricks.yaml}}"`
	Inline           string                 `mapstructure:"inline" description:"Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines." example:"{!{api-render-inline.hyperbricks.yaml}}"`
	Values           map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{api-render-values.hyperbricks.yaml}}"`
	Username         string                 `mapstructure:"username" description:"Basic Auth username; both username and password are required" example:"{!{api-render-username.hyperbricks.yaml}}"`
	Password         string                 `mapstructure:"password" description:"Basic Auth password; both username and password are required" example:"{!{api-render-password.hyperbricks.yaml}}"`
	Status           int                    `mapstructure:"status" exclude:"true"` // This adds {{.Status}} to the root level of the template data
	AllowedQueryKeys []string               `mapstructure:"querykeys" description:"Incoming URL query keys to append to the upstream URL. Omitted: id, name, order; empty list: none. Does not filter body placeholders." example:"{!{api-render-querykeys.hyperbricks.yaml}}"`
	QueryParams      map[string]string      `mapstructure:"queryparams" description:"Static upstream URL query values, appended after endpoint and allowed browser query values. Does not supply body placeholders." example:"{!{api-render-queryparams.hyperbricks.yaml}}"`
	JwtSecret        string                 `mapstructure:"jwtsecret" description:"Signs jwtclaims as the sole upstream authentication source; cannot be combined with Basic Auth, Authorization or forwardtoken" example:"{!{api-render-jwt-secret.hyperbricks.yaml}}"`
	JwtClaims        map[string]string      `mapstructure:"jwtclaims" description:"JWT claims to include when signing the bearer token" example:"{!{api-render-jwt-claims.hyperbricks.yaml}}"`
	Debug            bool                   `mapstructure:"debug" description:"Log request and response metadata only; never header values, URL paths or queries, or payloads" example:"{!{api-render-debug.hyperbricks.yaml}}"`
	DebugPanel       bool                   `mapstructure:"debugpanel" description:"Render a frontend debug panel when frontend_errors is enabled in modules package.hyperbricks.yaml" example:"{!{api-render-debug.hyperbricks.yaml}}"`
}

func APIConfigGetName() string {
	return "<API_RENDER>"
}

type APIRenderer struct {
	renderer.ComponentRenderer
}

var _ shared.ComponentRenderer = (*APIRenderer)(nil)

func (api *APIConfig) Validate() []error {
	var errors []error
	if api.Endpoint == "" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      api.Component.Meta.HyperBricksKey,
			Path:     api.Component.Meta.HyperBricksPath,
			File:     api.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      "[field 'endpoint' is required]",
			Rejected: false,
		})
	}
	if api.Method == "" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      api.Component.Meta.HyperBricksKey,
			Path:     api.Component.Meta.HyperBricksPath,
			File:     api.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
			Err:      "[field 'method' is required]",
			Rejected: false,
		})
	}
	errors = append(errors, shared.Validate(api)...)
	endpoint, err := url.Parse(api.Endpoint)
	if err != nil {
		err = fmt.Errorf("invalid API endpoint URL")
	} else {
		err = apiutil.ValidateAuthSettings(endpoint, shared.GetHyperBricksConfiguration().Mode, api.authSettings())
	}
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Type: APIConfigGetName(), Err: err.Error(), Rejected: true,
			Key: api.Component.Meta.HyperBricksKey, Path: api.Component.Meta.HyperBricksPath,
			File: api.Component.Meta.HyperBricksFile,
		})
	}
	for key := range api.Values {
		if key == "Data" || key == "Status" {
			errors = append(errors, shared.ComponentError{Type: APIConfigGetName(), Err: "API values cannot override reserved Data or Status", Rejected: true})
		}
	}
	return errors
}

func (r *APIRenderer) Types() []string {
	return []string{
		APIConfigGetName(),
	}
}

func (pr *APIRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	if ctx == nil {
		ctx = context.Background()
	}

	//return APIConfigGetName(), nil
	var errors []error
	var builder strings.Builder

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
	body, requestErr := processRequest(ctx, config)
	if requestErr != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
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
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
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

func processRequest(ctx context.Context, config APIConfig) (string, error) {
	bodyMap := config.Body
	mergedData := make(map[string]interface{})

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

	// Parsed URL/form values remain available when the incoming body is empty.
	return apiutil.MapRequestBody(bodyMap, mergedData)
}

// fetchDataFromAPI applies the API component's explicit credential policy.
func fetchDataFromAPI(config APIConfig, ctx context.Context) (interface{}, int, error) {
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
	return result, resp.StatusCode, apiRenderStatusError(resp)
}

func apiRenderStatusError(resp *http.Response) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("upstream API returned HTTP %d", resp.StatusCode)
}

func applyApiTemplate(templateStr string, data interface{}, config APIConfig) (string, []error) {
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
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
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
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     APIConfigGetName(),
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
