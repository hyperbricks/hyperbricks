package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type apiCookieCommitTestConfig struct{}

type apiCookieCommitTestRenderer struct {
	cookies         []string
	renderErr       error
	handledResponse *shared.HandledResponse
	conflict        bool
}

func (renderer apiCookieCommitTestRenderer) Render(_ interface{}, ctx context.Context) (string, []error) {
	var renderErrors []error
	capture, _ := ctx.Value(shared.APIResponseCookieCaptureKey).(*shared.APIResponseCookieCapture)
	if capture == nil {
		return "", []error{errors.New("missing API response cookie capture")}
	}
	if err := capture.Store(renderer.cookies); err != nil {
		renderErrors = append(renderErrors, err)
	}
	if renderer.renderErr != nil {
		renderErrors = append(renderErrors, renderer.renderErr)
	}

	if renderer.handledResponse != nil {
		handled, _ := ctx.Value(shared.HandledResponseCaptureKey).(*shared.HandledResponseCapture)
		if handled == nil {
			renderErrors = append(renderErrors, errors.New("missing handled response capture"))
		} else {
			_ = handled.Store(renderer.handledResponse)
			if renderer.conflict {
				_ = handled.Store(&shared.HandledResponse{Body: []byte("conflict")})
			}
		}
	}
	return "rendered", renderErrors
}

func (apiCookieCommitTestRenderer) Types() []string { return nil }

func TestServeContentCommitsCapturedAPICookiesOnlyAfterSuccessfulRender(t *testing.T) {
	setupLiveModeServeContentTest(t)

	routeSourceErrorsMutex.RLock()
	oldSourceErrors := cloneErrorsByRoute(routeSourceErrors)
	routeSourceErrorsMutex.RUnlock()
	t.Cleanup(func() { updateGlobalRouteSourceErrors(oldSourceErrors) })

	const stagedCookie = "api_session=opaque+value&part; Path=/; HttpOnly"
	tests := []struct {
		name           string
		renderer       apiCookieCommitTestRenderer
		sourceErr      error
		wantStatus     int
		wantCookies    []string
		wantErrorCount int
	}{
		{
			name:        "successful render commits once",
			renderer:    apiCookieCommitTestRenderer{cookies: []string{stagedCookie}},
			wantStatus:  http.StatusOK,
			wantCookies: []string{stagedCookie},
		},
		{
			name:           "later render error suppresses staged cookies",
			renderer:       apiCookieCommitTestRenderer{cookies: []string{stagedCookie}, renderErr: errors.New("later render failed")},
			wantStatus:     http.StatusOK,
			wantErrorCount: 1,
		},
		{
			name:           "later source error suppresses staged cookies",
			renderer:       apiCookieCommitTestRenderer{cookies: []string{stagedCookie}},
			sourceErr:      errors.New("route source failed after materialization"),
			wantStatus:     http.StatusOK,
			wantErrorCount: 1,
		},
		{
			name: "handled response owns its cookies",
			renderer: apiCookieCommitTestRenderer{
				cookies: []string{stagedCookie},
				handledResponse: &shared.HandledResponse{
					Status:  http.StatusAccepted,
					Cookies: []string{"plugin_session=owned; Path=/"},
					Body:    []byte("handled"),
				},
			},
			wantStatus:  http.StatusAccepted,
			wantCookies: []string{"plugin_session=owned; Path=/"},
		},
		{
			name: "handled response conflict suppresses staged cookies",
			renderer: apiCookieCommitTestRenderer{
				cookies:         []string{stagedCookie},
				handledResponse: &shared.HandledResponse{Body: []byte("first")},
				conflict:        true,
			},
			wantStatus:     http.StatusInternalServerError,
			wantErrorCount: 1,
		},
	}

	sourceErrors := make(map[string][]error)
	for index, test := range tests {
		rendererType := fmt.Sprintf("<API_COOKIE_COMMIT_TEST_%d>", index)
		route := fmt.Sprintf("api-cookie-commit-%d", index)
		rm.RegisterComponent(rendererType, test.renderer, reflect.TypeOf(apiCookieCommitTestConfig{}))
		setTestRouteConfig(route, map[string]interface{}{
			"@type":   rendererType,
			"route":   route,
			"nocache": true,
		})
		if test.sourceErr != nil {
			sourceErrors[route] = []error{test.sourceErr}
		}
	}
	updateGlobalRouteSourceErrors(sourceErrors)

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			route := fmt.Sprintf("api-cookie-commit-%d", index)
			writer := httptest.NewRecorder()
			ServeContent(writer, httptest.NewRequest(http.MethodGet, "/"+route, nil))

			result := writer.Result()
			if result.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", result.StatusCode, test.wantStatus)
			}
			if got := result.Header.Values("Set-Cookie"); !reflect.DeepEqual(got, test.wantCookies) {
				t.Fatalf("Set-Cookie headers = %#v, want %#v", got, test.wantCookies)
			}
			if got := result.Header.Get(renderErrorCountHeader); got != strconv.Itoa(test.wantErrorCount) {
				t.Fatalf("render error count = %q, want %d", got, test.wantErrorCount)
			}
		})
	}
}

func TestServeContentSuppressesAPIFragmentCookieWhenRouteSourceErrorAppearsAfterRender(t *testing.T) {
	setupLiveModeServeContentTest(t)

	routeSourceErrorsMutex.RLock()
	oldSourceErrors := cloneErrorsByRoute(routeSourceErrors)
	routeSourceErrorsMutex.RUnlock()
	t.Cleanup(func() { updateGlobalRouteSourceErrors(oldSourceErrors) })

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"token":"issued+opaque&value"}`))
	}))
	t.Cleanup(upstream.Close)

	const route = "api-cookie-source-error"
	setTestRouteConfig(route, map[string]interface{}{
		"@type":    composite.ApiFragmentRenderConfigGetName(),
		"route":    route,
		"method":   http.MethodGet,
		"endpoint": upstream.URL,
		"inline":   `{{.Data.token}}`,
		"setcookies": []interface{}{map[string]interface{}{
			"name": "api_session", "value": `{{.Data.token}}`, "path": "/", "http_only": true,
		}},
	})
	updateGlobalRouteSourceErrors(map[string][]error{
		route: []error{errors.New("late route source error")},
	})

	writer := httptest.NewRecorder()
	ServeContent(writer, httptest.NewRequest(http.MethodGet, "/"+route, nil))

	if upstreamCalls != 1 {
		t.Fatalf("upstream calls = %d, want 1 so the API fragment reached cookie staging", upstreamCalls)
	}
	if got := writer.Result().Header.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie headers = %#v, want none after late route source error", got)
	}
	if got := writer.Header().Get(renderErrorCountHeader); got != "1" {
		t.Fatalf("render error count = %q, want 1", got)
	}
}
