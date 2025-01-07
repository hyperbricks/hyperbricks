package shared

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ExtractHxData extracts headers, form data, JSON payloads, and query parameters from an HTTP request.
func ExtractHxData(r *http.Request) (map[string]interface{}, error) {
	hxdata := make(map[string]interface{})

	// Extract Headers
	headers := make(map[string]interface{})
	for key, values := range r.Header {
		headers[key] = JoinHeaderValues(values)
	}
	hxdata["headers"] = headers

	// Extract Form Data
	formData := make(map[string]interface{})
	contentType := r.Header.Get("Content-Type")

	switch {
	case IsMultipartForm(contentType):
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
			return nil, fmt.Errorf("error parsing multipart form: %v", err)
		}
		for key, values := range r.MultipartForm.Value {
			if len(values) == 1 {
				formData[key] = values[0]
			} else {
				formData[key] = values
			}
		}
	case IsJSONContent(contentType):
		var jsonData map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&jsonData); err != nil {
			return nil, fmt.Errorf("error decoding JSON body: %v", err)
		}
		formData["json"] = jsonData
		defer r.Body.Close()
	default:
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("error parsing form: %v", err)
		}
		for key, values := range r.PostForm {
			if len(values) == 1 {
				formData[key] = values[0]
			} else {
				formData[key] = values
			}
		}
	}
	hxdata["form"] = formData

	// Extract Query Parameters
	queryParams := make(map[string]interface{})
	for key, values := range r.URL.Query() {
		if len(values) == 1 {
			queryParams[key] = values[0]
		} else {
			queryParams[key] = values
		}
	}
	hxdata["query"] = queryParams

	return hxdata, nil
}

// JoinHeaderValues concatenates multiple header values into a single string separated by commas.
func JoinHeaderValues(values []string) string {
	return strings.Join(values, ", ")
}

// IsMultipartForm checks if the Content-Type is multipart/form-data.
func IsMultipartForm(contentType string) bool {
	return strings.HasPrefix(contentType, "multipart/form-data")
}

// IsJSONContent checks if the Content-Type is application/json.
func IsJSONContent(contentType string) bool {
	return strings.HasPrefix(contentType, "application/json")
}
