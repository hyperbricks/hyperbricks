package apiutil

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// DefaultQueryKeys provides the standard set of allowed query parameters.
var DefaultQueryKeys = []string{"id", "name", "order"}

// FlattenFormData converts URL-encoded form values into a flat map.
func FlattenFormData(formData url.Values) map[string]interface{} {
	flattened := make(map[string]interface{})
	for key, values := range formData {
		if len(values) == 1 {
			flattened[key] = values[0]
		} else {
			flattened[key] = values
		}
	}
	return flattened
}

// NewHTTPClient creates an HTTP client with isolated cookies and shared transport.
func NewHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: sharedTransport,
		Jar:       jar,
	}
}

// sharedTransport enables connection pooling across clients.
var sharedTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	DisableKeepAlives:   false,
}

// IsJSONResponse reports whether the response contains JSON data.
func IsJSONResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/json")
}

// IsXMLResponse reports whether the response contains XML data.
func IsXMLResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/xml") || strings.HasPrefix(contentType, "text/xml")
}

// HandleAPIResponse decodes JSON or XML responses and falls back to plain text.
func HandleAPIResponse(resp *http.Response) (interface{}, int, error) {
	var result interface{}
	if IsJSONResponse(resp) {
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&result); err != nil {
			return nil, resp.StatusCode, err
		}
		return result, resp.StatusCode, nil
	}
	if IsXMLResponse(resp) {
		var xmlResult map[string]interface{}
		dec := xml.NewDecoder(resp.Body)
		if err := dec.Decode(&xmlResult); err != nil {
			return nil, resp.StatusCode, err
		}
		return xmlResult, resp.StatusCode, nil
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return string(bytes.TrimSpace(bodyBytes)), resp.StatusCode, nil
}
