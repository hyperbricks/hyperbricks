package composite

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

var validate = validator.New()

// HxApiConfig represents configuration for a single hxapi.
type HxApiConfig struct {
	shared.Composite `mapstructure:",squash"`
	HxDataContainer
	HxRequest        `mapstructure:",squash"`
	HxResponse       `mapstructure:"response"`
	HxResponseWriter http.ResponseWriter    `mapstructure:"hx"`
	Template         map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering output" example:"{!{hxendopint-template.hyperbricks}}"`
	Items            map[string]interface{} `mapstructure:",remain"`
	Enclose          string                 `mapstructure:"enclose" description:"Wrapping property for the hxapi" example:"{!{hxapi-wrap.hyperbricks}}"`
	IsStatic         bool                   `mapstructure:"isstatic"`
	Static           string                 `mapstructure:"static" description:"Static file path associated with the hxapi" example:"{!{hxapi-static.hyperbricks}}"`
}

// DataContainer holds the data for the update operation
type HxDataContainer struct {
	Data                HxFormData `mapstructure:"hx_data"`
	RowResult           []map[string]interface{}
	SomeResultContainer map[string]interface{}
	R                   http.Request
}

type HxFormData struct {
	HxHeaders map[string]interface{} `mapstructure:"headers" `
	HxForm    map[string]interface{} `mapstructure:"form"`
	HxQuery   map[string]interface{} `mapstructure:"query"`
}

type HxRequest struct {
	HxFormData    HxFormData    `mapstructure:"hx_form_data"`
	HXQuery       HxQueryConfig `mapstructure:"hx_query" description:"Middleware chain query"`
	HXDescription string        `mapstructure:"hx_description" description:"HxRoute description"`
	HXModel       ModelConfig   `mapstructure:"hx_model" description:"A map[string]interface{} with field descriptions"`
	HXTable       string        `mapstructure:"hx_table" description:"The database table name"`
	HXDb          string        `mapstructure:"hx_db" description:"The database (a path to a sqlite3 database at this stage)"`

	HXTemplate      string `mapstructure:"hx_template" description:"Template for rendering the data and send back to the client" example:"{!{hxapi-template.hyperbricks}}"`
	HXErrorTemplate string `mapstructure:"hx_error_template" description:"Template that is renderend send when an error occurs"`

	HxRoute          string `mapstructure:"hx_route" description:"identifier for the hxapi" example:"{!{hxapi-route.hyperbricks}}"`
	HxMethod         string `mapstructure:"hx_method"`
	HxBoosted        string `mapstructure:"hx_boosted" description:"indicates that the request is via an element using hx_boost"`
	HxCurrentUrl     string `mapstructure:"hx_current_url" description:"the current url of the browser"`
	HxHistoryRestore string `mapstructure:"hx_history_restore_request" description:"true if the request is for history restoration after a miss in the local history cache"`
	HxPrompt         string `mapstructure:"hx_prompt" description:"the user response to an hx_prompt"`
	HxRequestFlag    string `mapstructure:"hx_request" description:"always true"`
	HxTarget         string `mapstructure:"hx_target" description:"the id of the target element if it exists"`
	HxTriggerName    string `mapstructure:"hx_trigger_name" description:"the name of the triggered element if it exists"`
	HxTrigger        string `mapstructure:"hx_trigger" description:"the id of the triggered element if it exists"`
}

type HxResponse struct {
	HxTemplateResult     string // just for output of the parsed template
	HxLocation           string `mapstructure:"hx_location" header:"HX-Location"  description:"allows you to do a client-side redirect that does not do a full page reload" `
	HxPushedUrl          string `mapstructure:"hx_push_url" header:"HX-Pushed-Url" description:"pushes a new url into the history stack"`
	HxRedirect           string `mapstructure:"hx_redirect" header:"HX-Redirect" description:"can be used to do a client-side redirect to a new location"`
	HxRefresh            string `mapstructure:"hx_refresh" header:"HX-Refresh" description:"if set to 'true' the client-side will do a full refresh of the page"`
	HxReplaceUrl         string `mapstructure:"hx_replace_url" header:"HX-Replace-Url" description:"replaces the current url in the location bar"`
	HxReswap             string `mapstructure:"hx_reswap" header:"HX-Reswap" description:"allows you to specify how the response will be swapped"`
	HxRetarget           string `mapstructure:"hx_retarget" header:"HX-Retarget" description:"a css selector that updates the target of the content update"`
	HxReselect           string `mapstructure:"hx_reselect" header:"HX-Reselect" description:"a css selector that allows you to choose which part of the response is used to be swapped in"`
	HxTrigger            string `mapstructure:"hx_trigger" header:"HX-Trigger" description:"allows you to trigger client-side events"`
	HxTriggerafterSettle string `mapstructure:"hx_trigger_after_settle"  header:"HX-Trigger-After-Settle" description:"allows you to trigger client-side events after the settle step"`
	HxTriggerafterSwap   string `mapstructure:"hx_trigger_after_swap"  header:"HX-Trigger-After-Swap" description:"allows you to trigger client-side events after the swap step"`
}

type ModelConfig struct {
	Fields map[string]FieldConfig `mapstructure:"fields"  description:"Fields contain a type (float64, int, string etc) and a required value"` // Map of fields with their configuration
	Name   string                 `mapstructure:"name"`
}

// FieldConfig holds the configuration for each field in the model
type FieldConfig struct {
	Type     string `mapstructure:"type"  description:"The type of the field (string, float64, etc.)"` // The type of the field (string, float64, etc.)
	Validate string `mapstructure:"validate" description:"List of fields that are required"`           // List of fields that are required
}

// QueryConfig holds the dynamic query configuration
type HxQueryConfig struct {
	HxQueryCreate  HxQuery `mapstructure:"create"`
	HxQueryReplace HxQuery `mapstructure:"replace"`
	HxQueryUpdate  HxQuery `mapstructure:"update"`
	HxQueryRead    HxQuery `mapstructure:"read"`
	HxQueryDelete  HxQuery `mapstructure:"delete"`
}

type HxQuery struct {
	Fields []string `mapstructure:"fields"`
	SQL    string   `mapstructure:"sql" description:""`
}

// HxApiRenderer handles rendering of PAGE content.
type HxApiRenderer struct {
	renderer.CompositeRenderer
}

// Add interface requirements
type HxApiRendererInterface interface {
	shared.CompositeRenderer
	//middleware
	ParseRequestData(config *HxApiConfig) error
	ValidateRequestData(config *HxApiConfig) error
	ExecuteQuery(config *HxApiConfig) error
	ParseTemplate(config *HxApiConfig) error
}

// Ensure HxApiRenderer implements HxApiRendererInterface interface.
var _ HxApiRendererInterface = (*HxApiRenderer)(nil)

// HxApiConfigGetName returns the HyperBricks type associated with the HxApiConfig.
func HxApiConfigGetName() string {
	return "<API>"
}
func (r *HxApiRenderer) Types() []string {
	return []string{
		HxApiConfigGetName(),
	}
}

// Validate ensures that the hxapi has valid data.
func (hxapi *HxApiConfig) Validate() []error {
	var warnings []error
	return warnings
}

// Render implements the RenderComponent interface.
func (pr *HxApiRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var config HxApiConfig

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to decode instance into HxApiConfig: %w", err).Error(),
		})
	}

	if err := pr.ParseRequestData(&config); err != nil {
		errors = append(errors, err)
	}

	if err := pr.ValidateRequestData(&config); err != nil {
		errors = append(errors, err)
	}

	if err := pr.ExecuteQuery(&config); err != nil {
		errors = append(errors, err)
	}

	if err := pr.ParseTemplate(&config); err != nil {
		errors = append(errors, err)
	}

	SetHeadersFromHxRequest(&config.HxResponse, config.HxResponseWriter)
	return config.HxResponse.HxTemplateResult, errors

}

func SetHeadersFromHxRequest(config *HxResponse, writer http.ResponseWriter) {
	// Use reflection to access struct fields
	v := reflect.ValueOf(*config)
	t := reflect.TypeOf(*config)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Use the "header" tag to get the HTTP header name
		headerName := fieldType.Tag.Get("header")
		if headerName == "" || !field.IsValid() || (field.Kind() == reflect.String && field.String() == "") {
			// Skip fields without a header tag or empty string fields
			continue
		}

		// Convert the field value to a string
		headerValue := ""
		switch field.Kind() {
		case reflect.String:
			headerValue = field.String()
		case reflect.Int, reflect.Int64, reflect.Float64, reflect.Bool:
			headerValue = fmt.Sprintf("%v", field.Interface())
		default:
			// Skip unsupported types
			continue
		}

		// Set the header using Go's default canonicalization
		writer.Header().Set(headerName, headerValue)
		log.Println(writer.Header())
	}
}

func (pr *HxApiRenderer) ParseRequestData(config *HxApiConfig) error {
	// Use form data if available
	if len(config.HxFormData.HxForm) > 0 {
		// Convert the formData values from strings to the expected types
		if err := convertFormData(config.HXModel, config.HxFormData.HxForm); err != nil {
			return shared.ComponentError{
				Key:      config.Key,
				Path:     config.Path,
				Err:      fmt.Sprintf("Data conversion error: %v %d", err, http.StatusBadRequest),
				Rejected: true,
			}
		}
		config.HxDataContainer.Data = config.HxFormData
		return nil
	}

	// Fallback to query parameters if form data is empty
	if len(config.HxFormData.HxQuery) > 0 {
		// Convert the query values from strings to the expected types
		if err := convertFormData(config.HXModel, config.HxFormData.HxQuery); err != nil {
			return shared.ComponentError{
				Key:      config.Key,
				Path:     config.Path,
				Err:      fmt.Sprintf("Query data conversion error: %v %d", err, http.StatusBadRequest),
				Rejected: true,
			}
		}
		config.HxDataContainer.Data = config.HxFormData
		return nil
	}

	// If both form data and query parameters are empty, return an error
	return shared.ComponentError{
		Key:      config.Key,
		Path:     config.Path,
		Err:      fmt.Sprintf("No form data or query parameters provided %d", http.StatusBadRequest),
		Rejected: true,
	}
}

func (pr *HxApiRenderer) ValidateRequestData(config *HxApiConfig) error {
	if err := validateData(config.HxDataContainer, config.HxRequest.HXModel); err != nil {
		return fmt.Errorf("data validation failed: %v", err)
	}
	return nil
}
func (pr *HxApiRenderer) ExecuteQuery(config *HxApiConfig) error {
	db, err := sql.Open("sqlite3", config.HxRequest.HXDb)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := execute(db, config); err != nil {
		return fmt.Errorf("query execution failed: %v HXQuery:%v HxMethod:%s", err, config.HXQuery, config.HxRequest.HxMethod)
	}
	return nil
}

func (pr *HxApiRenderer) ParseTemplate(config *HxApiConfig) error {
	if config.HxRequest.HXTemplate == "" {
		return fmt.Errorf("no template provided")
	}

	tmpl, err := template.New("hxTemplate").Parse(config.HxRequest.HXTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}
	var result strings.Builder

	switch config.HxRequest.HxMethod {
	case "GET":
		err = tmpl.Execute(&result, config.HxDataContainer.RowResult)
	case "POST":
		err = tmpl.Execute(&result, config.HxDataContainer.SomeResultContainer)
	case "PUT", "PATCH", "DELETE":
		err = tmpl.Execute(&result, config.HxDataContainer.SomeResultContainer)
	}

	if err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	// Add the parsed template result to the HxResponse.HxTemplateResult string for later output...
	config.HxResponse.HxTemplateResult = result.String()
	return nil
}

// ====================== Helpers ======================
// convertFormData converts string values in formData to the expected types defined in the modelConfig.
func convertFormData(modelConfig ModelConfig, formData map[string]interface{}) error {
	for fieldName, fieldConfig := range modelConfig.Fields {
		value, exists := formData[fieldName]
		if !exists {
			// If the field doesn't exist in the form, skip it
			continue
		}

		// Check if value is already of the correct type
		expectedType := fieldConfig.Type
		if reflect.TypeOf(value).String() == expectedType {
			continue
		}

		strVal, ok := value.(string)
		if !ok {
			// If not a string, raise a type mismatch error
			return fmt.Errorf("field '%s' expected %s, got %T", fieldName, expectedType, value)
		}

		switch expectedType {
		case "string":
			// Already a string, no conversion needed
		case "float64":
			// Convert string to float64
			floatVal, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return fmt.Errorf("field '%s' expected float64, got invalid value: %v", fieldName, err)
			}
			formData[fieldName] = floatVal
		case "int":
			// Convert string to int
			intVal, err := strconv.Atoi(strVal)
			if err != nil {
				return fmt.Errorf("field '%s' expected int, got invalid value: %v", fieldName, err)
			}
			formData[fieldName] = intVal
		case "bool":
			// Convert string to bool
			boolVal, err := strconv.ParseBool(strVal)
			if err != nil {
				return fmt.Errorf("field '%s' expected bool, got invalid value: %v", fieldName, err)
			}
			formData[fieldName] = boolVal
		default:
			// Unsupported type
			return fmt.Errorf("unsupported type '%s' for field '%s'", expectedType, fieldName)
		}
	}
	return nil
}

// validateData validates the data in HxDataContainer based on the provided ModelConfig.
func validateData(data HxDataContainer, modelConfig ModelConfig) error {
	for field, fieldConfig := range modelConfig.Fields {
		var _data map[string]interface{}
		if len(data.Data.HxForm) > 0 {
			_data = data.Data.HxForm
		} else if len(data.Data.HxQuery) > 0 {
			_data = data.Data.HxQuery
		}
		value, exists := _data[field]
		if !exists {
			if fieldConfig.Validate == "required" {
				return fmt.Errorf("field '%s' is required but missing", field)
			}
			continue
		}

		if err := validate.Var(value, fieldConfig.Validate); err != nil {
			return fmt.Errorf("validation failed for field '%s': %v", field, err)
		}
	}
	return nil
}

func execute(db *sql.DB, config *HxApiConfig) error {
	if config.HxRequest.HxMethod != "GET" {
		// Validate SQL query and placeholders
		if err := validateSQLQuery(config.HxRequest.HXQuery.HxQueryCreate.SQL); err != nil {
			return fmt.Errorf("invalid SQL query: %v", err)
		}

		if err := validatePlaceholders(config.HxRequest.HXQuery.HxQueryCreate.SQL, config.HxRequest.HXQuery.HxQueryCreate.Fields); err != nil {
			return fmt.Errorf("placeholder validation failed: %v", err)
		}

	}

	// Validate data
	if err := validateData(config.HxDataContainer, config.HXModel); err != nil {
		return fmt.Errorf("data validation failed: %v", err)
	}

	var query string
	var fields []string

	// Determine query and fields based on method
	switch config.HxRequest.HxMethod {
	case "POST":
		query = config.HxRequest.HXQuery.HxQueryCreate.SQL
		fields = config.HxRequest.HXQuery.HxQueryCreate.Fields
	case "GET":
		query = config.HxRequest.HXQuery.HxQueryRead.SQL
		fields = config.HxRequest.HXQuery.HxQueryRead.Fields
	case "PUT":
		query = config.HxRequest.HXQuery.HxQueryReplace.SQL
		fields = config.HxRequest.HXQuery.HxQueryReplace.Fields
	case "PATCH":
		query = config.HxRequest.HXQuery.HxQueryUpdate.SQL
		fields = config.HxRequest.HXQuery.HxQueryUpdate.Fields
	case "DELETE":
		query = config.HxRequest.HXQuery.HxQueryDelete.SQL
		fields = config.HxRequest.HXQuery.HxQueryDelete.Fields
	default:
		return fmt.Errorf("unsupported method: %s", config.HxRequest.HxMethod)
	}

	var argsz map[string]interface{}
	if len(config.HxDataContainer.Data.HxForm) > 0 {
		argsz = config.HxDataContainer.Data.HxForm
	} else if len(config.HxDataContainer.Data.HxQuery) > 0 {
		argsz = config.HxDataContainer.Data.HxQuery
	}
	args, err := extractArgs(fields, argsz)
	if err != nil {
		return err
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	switch config.HxRequest.HxMethod {
	case "GET":
		rows, err := stmt.Query(args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		var results []map[string]interface{}

		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("failed to get columns: %v", err)
		}

		// Iterate through rows
		for rows.Next() {
			// Create a slice to hold column values
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			// Scan the row into value pointers
			if err := rows.Scan(valuePtrs...); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			// Map the row to a map[string]interface{} with dereferenced values
			rowMap := make(map[string]interface{})
			for i, col := range columns {
				// Dereference the value
				val := *(valuePtrs[i].(*interface{}))
				rowMap[col] = val
			}

			// Add the row to the results
			results = append(results, rowMap)
		}

		// Check for errors during iteration
		if err := rows.Err(); err != nil {
			return fmt.Errorf("error during row iteration: %v", err)
		}

		// Store results in the data container
		config.HxDataContainer.RowResult = results

		log.Printf("GET query executed successfully: %+v", results)
		return nil

	case "POST", "PUT", "PATCH", "DELETE":
		result, err := stmt.Exec(args...)
		if err != nil {
			return err
		}

		// Initialize the result container if it's nil
		if config.HxDataContainer.SomeResultContainer == nil {
			config.HxDataContainer.SomeResultContainer = make(map[string]interface{})
		}

		switch config.HxRequest.HxMethod {
		case "POST":
			lastInsertID, err := result.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to retrieve LastInsertId: %v", err)
			}
			config.HxDataContainer.SomeResultContainer["LastInsertID"] = lastInsertID
			log.Printf("POST executed successfully, LastInsertID: %d", lastInsertID)

		case "PUT", "PATCH", "DELETE":
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("failed to retrieve RowsAffected: %v", err)
			}
			config.HxDataContainer.SomeResultContainer["RowsAffected"] = rowsAffected
			log.Printf("%s executed successfully, RowsAffected: %d", config.HxRequest.HxMethod, rowsAffected)
		}

		return nil

	default:
		return fmt.Errorf("unsupported method after switch: %s", config.HxRequest.HxMethod)
	}
}

func extractArgs(fields []string, data map[string]interface{}) ([]interface{}, error) {
	args := make([]interface{}, len(fields))
	for i, field := range fields {
		val, exists := data[field]
		if !exists {
			return nil, fmt.Errorf("missing required field: %s", field)
		}
		args[i] = val
	}
	return args, nil
}

func validateSQLQuery(query string) error {
	forbiddenKeywords := []string{"DROP", "TRUNCATE", "ALTER", "EXEC", "--", ";"}
	for _, keyword := range forbiddenKeywords {
		if strings.Contains(strings.ToUpper(query), keyword) {
			return fmt.Errorf("forbidden keyword detected: %s", keyword)
		}
	}
	return nil
}

func validatePlaceholders(query string, fields []string) error {
	placeholderCount := strings.Count(query, "?")
	if placeholderCount != len(fields) {
		return fmt.Errorf("placeholder count (%d) does not match field count (%d)", placeholderCount, len(fields))
	}
	return nil
}
