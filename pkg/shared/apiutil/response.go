package apiutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// DecodeAPIResponse reads the response once. A malformed JSON response must not
// become a successful nil result through a second read of the consumed body.
// The error messages deliberately omit upstream payloads.
func DecodeAPIResponse(resp *http.Response) (interface{}, error) {
	if resp.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read upstream response")
	}
	probe := bytes.TrimSpace(body)
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	jsonType := strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json")
	if len(probe) == 0 {
		if jsonType && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusResetContent {
			return nil, fmt.Errorf("upstream JSON response is empty")
		}
		return nil, nil
	}
	// Accept JSON without a Content-Type for compatibility with existing APIs,
	// but never disguise malformed JSON-looking output as successful plain text.
	if jsonType || json.Valid(probe) || probe[0] == '{' || probe[0] == '[' {
		var value interface{}
		if err := json.Unmarshal(body, &value); err != nil {
			return nil, fmt.Errorf("upstream response contains invalid JSON")
		}
		return value, nil
	}
	copy := *resp
	copy.Body = io.NopCloser(bytes.NewReader(body))
	value, _, err := HandleAPIResponse(&copy)
	if err != nil {
		return nil, fmt.Errorf("failed to decode upstream response")
	}
	return value, nil
}
