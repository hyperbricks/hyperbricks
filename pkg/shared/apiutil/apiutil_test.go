package apiutil

import (
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestFlattenFormDataKeepsSingleAndMultiValueFields(t *testing.T) {
	form := url.Values{
		"title": {"Hello"},
		"tags":  {"go", "html"},
	}

	got := FlattenFormData(form)

	if got["title"] != "Hello" {
		t.Fatalf("expected single value to flatten to string, got %#v", got["title"])
	}
	if !reflect.DeepEqual(got["tags"], []string{"go", "html"}) {
		t.Fatalf("expected multi value to stay as []string, got %#v", got["tags"])
	}
}

func TestHTTPClientUsesIsolatedCookieJarAndSharedTransport(t *testing.T) {
	first := NewHTTPClient()
	second := NewHTTPClient()

	if first.Jar == nil || second.Jar == nil {
		t.Fatal("expected clients to have cookie jars")
	}
	if first.Jar == second.Jar {
		t.Fatal("expected separate cookie jars per client")
	}
	if first.Transport != second.Transport {
		t.Fatal("expected shared transport for connection pooling")
	}
	if first.Timeout == 0 {
		t.Fatal("expected client timeout to be configured")
	}
}

func TestResponseContentTypeHelpers(t *testing.T) {
	jsonResp := &http.Response{Header: http.Header{"Content-Type": {"application/json; charset=utf-8"}}}
	xmlResp := &http.Response{Header: http.Header{"Content-Type": {"text/xml"}}}
	textResp := &http.Response{Header: http.Header{"Content-Type": {"text/plain"}}}

	if !IsJSONResponse(jsonResp) {
		t.Fatal("expected JSON response to be detected")
	}
	if !IsXMLResponse(xmlResp) {
		t.Fatal("expected XML response to be detected")
	}
	if IsJSONResponse(textResp) || IsXMLResponse(textResp) {
		t.Fatal("did not expect text/plain to be detected as JSON or XML")
	}
}

func TestHandleAPIResponseDecodesJSONAndText(t *testing.T) {
	jsonResp := &http.Response{
		StatusCode: http.StatusCreated,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(stringsReader(`{"ok":true,"name":"demo"}`)),
	}

	jsonResult, status, err := HandleAPIResponse(jsonResp)
	if err != nil {
		t.Fatalf("unexpected JSON decode error: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, status)
	}
	jsonMap, ok := jsonResult.(map[string]interface{})
	if !ok || jsonMap["ok"] != true || jsonMap["name"] != "demo" {
		t.Fatalf("unexpected JSON result: %#v", jsonResult)
	}

	textResp := &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{"Content-Type": {"text/plain"}},
		Body:       io.NopCloser(stringsReader("  plain text  \n")),
	}

	textResult, status, err := HandleAPIResponse(textResp)
	if err != nil {
		t.Fatalf("unexpected text read error: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, status)
	}
	if textResult != "plain text" {
		t.Fatalf("expected trimmed text result, got %#v", textResult)
	}
}

func stringsReader(value string) io.Reader {
	return strings.NewReader(value)
}
