package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

func TestServeContentAPIRequestBodyReadFailureIsStickyAndFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name        string
		live        bool
		noCache     bool
		apiFragment bool
	}{
		{name: "development"},
		{name: "live cached", live: true},
		{name: "live nocache API fragment", live: true, noCache: true, apiFragment: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.live {
				setupLiveModeServeContentTest(t)
			} else {
				setupDevelopmentModeServeContentTest(t, false)
			}

			var upstreamCalls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				upstreamCalls.Add(1)
				writer.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(writer, `{"ok":true}`)
			}))
			t.Cleanup(upstream.Close)

			const route = "api-body-read-failure"
			if test.apiFragment {
				setTestRouteConfig(route, map[string]interface{}{
					"@type":    composite.ApiFragmentRenderConfigGetName(),
					"route":    route,
					"nocache":  test.noCache,
					"endpoint": upstream.URL,
					"method":   http.MethodGet,
					"inline":   `{{.Data.ok}}`,
				})
			} else {
				setTestRouteConfig(route, map[string]interface{}{
					"@type":   composite.FragmentConfigGetName(),
					"route":   route,
					"nocache": test.noCache,
					"10": map[string]interface{}{
						"@type":    component.APIConfigGetName(),
						"endpoint": upstream.URL,
						"method":   http.MethodGet,
						"inline":   `{{.Data.ok}}`,
					},
				})
			}

			reader := &oneShotFailingAPIRequestBody{cause: errors.New("body-reader-secret")}
			request := httptest.NewRequest(http.MethodPost, "/"+route, nil)
			request.Body = reader
			request.ContentLength = 32
			response := httptest.NewRecorder()

			ServeContent(response, request)

			if response.Code != http.StatusBadRequest || response.Body.String() != "Bad Request\n" {
				t.Fatalf("response status=%d body=%q, want generic 400", response.Code, response.Body.String())
			}
			if calls := upstreamCalls.Load(); calls != 0 {
				t.Fatalf("upstream received %d request(s) after body read failure", calls)
			}
			if reads := reader.reads.Load(); reads != 1 {
				t.Fatalf("underlying failing body was read %d times, want one sticky failure", reads)
			}
			if strings.Contains(response.Body.String(), "body-reader-secret") {
				t.Fatalf("response exposed body reader cause: %q", response.Body.String())
			}
		})
	}
}

func TestServeContentAPIFormParseFailureFailsClosed(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	const route = "api-form-parse-failure"
	setTestRouteConfig(route, map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": route,
		"10": map[string]interface{}{
			"@type":    component.APIConfigGetName(),
			"endpoint": upstream.URL,
			"method":   http.MethodGet,
			"inline":   `ok`,
		},
	})

	request := httptest.NewRequest(http.MethodPost, "/"+route, strings.NewReader("token=%zz-secret"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	ServeContent(response, request)

	if response.Code != http.StatusBadRequest || response.Body.String() != "Bad Request\n" {
		t.Fatalf("response status=%d body=%q, want generic 400", response.Code, response.Body.String())
	}
	if calls := upstreamCalls.Load(); calls != 0 {
		t.Fatalf("upstream received %d request(s) after form parse failure", calls)
	}
	if strings.Contains(response.Body.String(), "%zz-secret") {
		t.Fatalf("response exposed form parser input: %q", response.Body.String())
	}
}

type oneShotFailingAPIRequestBody struct {
	cause error
	reads atomic.Int32
}

func (body *oneShotFailingAPIRequestBody) Read(buffer []byte) (int, error) {
	if body.reads.Add(1) == 1 {
		return copy(buffer, "partial"), body.cause
	}
	return 0, io.EOF
}

func (*oneShotFailingAPIRequestBody) Close() error {
	return nil
}
