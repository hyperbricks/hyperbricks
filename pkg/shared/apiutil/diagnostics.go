package apiutil

import (
	"fmt"
	"net/http"
	"sort"
)

// RequestMetadata is safe to include in diagnostics. It deliberately omits
// header values, the body, URL user information, path, and query parameters.
type RequestMetadata struct {
	Method        string
	Scheme        string
	Host          string
	HeaderNames   []string
	ContentLength int64
}

// ResponseMetadata is safe to include in diagnostics. It deliberately omits
// header values and the response body.
type ResponseMetadata struct {
	StatusCode    int
	HeaderNames   []string
	ContentLength int64
}

// DescribeRequest returns secret-safe metadata for an upstream request.
func DescribeRequest(request *http.Request) RequestMetadata {
	if request == nil {
		return RequestMetadata{}
	}
	metadata := RequestMetadata{
		Method:        request.Method,
		HeaderNames:   sortedHeaderNames(request.Header),
		ContentLength: request.ContentLength,
	}
	if request.URL != nil {
		metadata.Scheme = request.URL.Scheme
		metadata.Host = request.URL.Host
	}
	return metadata
}

// DescribeResponse returns secret-safe metadata for an upstream response.
func DescribeResponse(response *http.Response) ResponseMetadata {
	if response == nil {
		return ResponseMetadata{}
	}
	return ResponseMetadata{
		StatusCode:    response.StatusCode,
		HeaderNames:   sortedHeaderNames(response.Header),
		ContentLength: response.ContentLength,
	}
}

// SafeRequestError retains the original error for errors.Is/errors.As while
// keeping its potentially sensitive text out of the rendered diagnostic.
func SafeRequestError(operation string, request *http.Request, cause error) error {
	return &safeRequestError{
		operation: operation,
		metadata:  DescribeRequest(request),
		cause:     cause,
	}
}

type safeRequestError struct {
	operation string
	metadata  RequestMetadata
	cause     error
}

func (err *safeRequestError) Error() string {
	return fmt.Sprintf("%s failed (method=%s scheme=%s host=%s header_names=%v content_length=%d)",
		err.operation,
		err.metadata.Method,
		err.metadata.Scheme,
		err.metadata.Host,
		err.metadata.HeaderNames,
		err.metadata.ContentLength,
	)
}

func (err *safeRequestError) Unwrap() error {
	return err.cause
}

func sortedHeaderNames(headers http.Header) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, http.CanonicalHeaderKey(name))
	}
	sort.Strings(names)
	return names
}
