package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

// Exercise the actual browser request -> API context -> upstream request flow.
// In particular, ParseForm includes browser query values even when querykeys
// excludes those values from the outgoing API URL.
func TestServeContentAPIRequestMapping(t *testing.T) {
	type mappingCase struct {
		name          string
		endpointQuery string
		browserQuery  string
		browserBody   string
		contentType   string
		queryKeys     []string // nil omits the configuration field; [] explicitly disables forwarding.
		queryParams   map[string]string
		bodyTemplate  string
		wantQuery     string
		wantBody      string
	}
	tests := []mappingCase{
		{
			name:         "omitted querykeys forwards default keys only",
			browserQuery: "id=first&id=second&name=Ada&order=asc&page=2",
			browserBody:  `{}`,
			bodyTemplate: `{"page":"$page"}`,
			wantQuery:    "id=first&id=second&name=Ada&order=asc",
			wantBody:     `{"page":"2"}`,
		},
		{
			name:         "empty querykeys forwards no browser query",
			browserQuery: "id=7&name=Ada&order=asc",
			browserBody:  `{}`,
			queryKeys:    []string{},
			bodyTemplate: `{"id":"$id","name":"$name","order":"$order"}`,
			wantBody:     `{"id":"7","name":"Ada","order":"asc"}`,
		},
		{
			name:         "explicit querykeys replaces defaults and preserves repeated values",
			browserQuery: "id=7&tag=blue&tag=yellow&name=Ada&order=asc",
			browserBody:  `{}`,
			queryKeys:    []string{"id", "tag"},
			bodyTemplate: `{"tag":"$tag","name":"$name"}`,
			wantQuery:    "id=7&tag=blue&tag=yellow",
			wantBody:     `{"tag":"[blue yellow]","name":"Ada"}`,
		},
		{
			name:          "endpoint browser and configured query values append without becoming body inputs",
			endpointQuery: "id=endpoint-first&id=endpoint-second&origin=endpoint",
			browserQuery:  "id=browser-first&id=browser-second&name=Ada",
			browserBody:   `{}`,
			queryKeys:     []string{"id"},
			queryParams:   map[string]string{"id": "configured", "fixed": "configured"},
			bodyTemplate:  `{"id":"$id","origin":"$origin","fixed":"$fixed"}`,
			wantQuery:     "fixed=configured&id=endpoint-first&id=endpoint-second&id=browser-first&id=browser-second&id=configured&origin=endpoint",
			wantBody:      `{"id":"[browser-first browser-second]"}`,
		},
		{
			name:          "empty querykeys retains endpoint and configured values",
			endpointQuery: "id=endpoint",
			browserQuery:  "id=browser",
			browserBody:   `{}`,
			queryKeys:     []string{},
			queryParams:   map[string]string{"id": "configured"},
			bodyTemplate:  `{"id":"$id"}`,
			wantQuery:     "id=endpoint&id=configured",
			wantBody:      `{"id":"browser"}`,
		},
		{
			name:         "querykeys does not filter query and JSON body substitutions",
			browserQuery: "id=browser&private=browser-value",
			browserBody:  `{"id":"json-value","only_json":"from-json"}`,
			queryKeys:    []string{},
			bodyTemplate: `{"id":"$id","json_id":"$body_id","private":"$private","only_json":"$only_json"}`,
			wantBody:     `{"id":"browser","json_id":"json-value","private":"browser-value","only_json":"from-json"}`,
		},
		{
			name:         "URL encoded form and query collisions preserve form-first lists",
			browserQuery: "id=query-first&id=query-second&name=Query",
			browserBody:  "id=form-first&id=form-second&name=Form&only_form=present",
			contentType:  "application/x-www-form-urlencoded",
			bodyTemplate: `{"id":"$id","name":"$name","only_form":"$only_form","body_id":"$body_id"}`,
			wantQuery:    "id=query-first&id=query-second&name=Query",
			wantBody:     `{"id":"[form-first form-second query-first query-second]","name":"[Form Query]","only_form":"present"}`,
		},
		{
			name:         "JSON collisions use body prefix and keep noncolliding values",
			browserQuery: "id=query&tag=blue&tag=yellow",
			browserBody:  `{"id":"json","tag":"json-tag","count":42,"enabled":true}`,
			bodyTemplate: `{"id":"$id","body_id":"$body_id","tag":"$tag","body_tag":"$body_tag","count":$count,"enabled":$enabled}`,
			wantQuery:    "id=query",
			wantBody:     `{"id":"query","body_id":"json","tag":"[blue yellow]","body_tag":"json-tag","count":42,"enabled":true}`,
		},
		{
			name:         "missing placeholders omit only their object properties",
			browserQuery: "id=7",
			browserBody:  `{}`,
			bodyTemplate: `{"id":"$id","missing":"$missing","missing_body":"$body_missing"}`,
			wantQuery:    "id=7",
			wantBody:     `{"id":"7"}`,
		},
		{
			name:         "first missing property is omitted",
			browserBody:  `{"keep":"present"}`,
			bodyTemplate: `{"first":"$missing","keep":"$keep"}`,
			wantBody:     `{"keep":"present"}`,
		},
		{
			name:         "middle missing property is omitted",
			browserBody:  `{"keep":"present"}`,
			bodyTemplate: `{"first":"$keep","middle":"$missing","last":"$keep"}`,
			wantBody:     `{"first":"present","last":"present"}`,
		},
		{
			name:         "last missing property is omitted",
			browserBody:  `{"keep":"present"}`,
			bodyTemplate: `{"keep":"$keep","last":"$missing"}`,
			wantBody:     `{"keep":"present"}`,
		},
		{
			name:         "all missing properties leave an empty object",
			browserBody:  `{}`,
			bodyTemplate: `{"first":"$missing","last":$also_missing}`,
			wantBody:     `{}`,
		},
		{
			name:         "nested missing properties are omitted without removing their parent",
			browserBody:  `{"keep":"present"}`,
			bodyTemplate: `{"outer":{"first":"$missing","keep":"$keep","last":$missing},"empty":{"absent":"$missing"},"keep":"$keep"}`,
			wantBody:     `{"outer":{"keep":"present"},"empty":{},"keep":"present"}`,
		},
		{
			name:         "explicit empty null false and zero are preserved",
			browserBody:  `{"empty":"","null":null,"false":false,"zero":0}`,
			bodyTemplate: `{"empty":"$empty","null":"$null","bare_null":$null,"false":$false,"zero":$zero,"absent":"$missing"}`,
			wantBody:     `{"empty":"","null":null,"bare_null":null,"false":false,"zero":0}`,
		},
		{
			name:         "supplied placeholder-looking data is not recursively mapped",
			browserBody:  `{"literal":"$missing"}`,
			bodyTemplate: `{"quoted":"$literal","bare":$literal,"absent":"$missing"}`,
			wantBody:     `{"quoted":"$missing","bare":"$missing"}`,
		},
		{
			name:         "missing bare property value is omitted",
			browserBody:  `{"keep":"present"}`,
			bodyTemplate: `{"absent":$missing,"keep":$keep}`,
			wantBody:     `{"keep":"present"}`,
		},
		{
			name:         "array of objects preserves array positions while omitting missing properties",
			browserBody:  `{"keep":"present"}`,
			bodyTemplate: `[{"absent":"$missing"},{"keep":"$keep","absent":$missing}]`,
			wantBody:     `[{},{"keep":"present"}]`,
		},
		{
			name:         "JSON string substitutions preserve special characters without injecting fields",
			browserBody:  `{"leading":"\"leading","trailing":"trailing\"","slash":"slash\\","newline":"first\nsecond","injection":"\"},\"admin\":true,\"ignored\":\""}`,
			bodyTemplate: `{"leading":"$leading","trailing":"$trailing","slash":"$slash","newline":"$newline","injection":"$injection"}`,
			wantBody:     `{"leading":"\"leading","trailing":"trailing\"","slash":"slash\\","newline":"first\nsecond","injection":"\"},\"admin\":true,\"ignored\":\""}`,
		},
	}
	for _, invalidBody := range []struct{ name, body string }{
		{"malformed JSON", `{"only_json":"partial"`},
		{"JSON array", `["first","second"]`},
		{"JSON string", `"text"`},
		{"JSON number", `42`},
		{"JSON boolean", `true`},
		{"JSON null", `null`},
	} {
		tests = append(tests, mappingCase{
			name:         invalidBody.name + " contributes no body keys",
			browserQuery: "id=query",
			browserBody:  invalidBody.body,
			bodyTemplate: `{"id":"$id","only_json":"$only_json"}`,
			wantQuery:    "id=query",
			wantBody:     `{"id":"query"}`,
		})
	}

	for _, apiFragment := range []bool{false, true} {
		componentName := "nested api_render"
		if apiFragment {
			componentName = "api_fragment_render"
		}
		t.Run(componentName, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					setupDevelopmentModeServeContentTest(t, false)
					type capturedRequest struct {
						method      string
						contentType string
						query       string
						body        string
						err         error
					}
					received := make(chan capturedRequest, 1)
					upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
						body, err := io.ReadAll(request.Body)
						received <- capturedRequest{method: request.Method, contentType: request.Header.Get("Content-Type"), query: request.URL.RawQuery, body: string(body), err: err}
						writer.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(writer, `{"ok":true}`)
					}))
					t.Cleanup(upstream.Close)

					endpoint := upstream.URL
					if test.endpointQuery != "" {
						endpoint += "?" + test.endpointQuery
					}
					apiConfig := map[string]interface{}{
						"@type":    component.APIConfigGetName(),
						"endpoint": endpoint,
						"method":   http.MethodPost,
						"headers":  map[string]string{"Content-Type": "application/json"},
						"body":     test.bodyTemplate,
						"inline":   `{{.Data.ok}}`,
					}
					if test.queryKeys != nil {
						apiConfig["querykeys"] = test.queryKeys
					}
					if test.queryParams != nil {
						apiConfig["queryparams"] = test.queryParams
					}
					const route = "api-request-mapping"
					if apiFragment {
						apiConfig["@type"] = composite.ApiFragmentRenderConfigGetName()
						apiConfig["route"] = route
						setTestRouteConfig(route, apiConfig)
					} else {
						setTestRouteConfig(route, map[string]interface{}{
							"@type": composite.FragmentConfigGetName(),
							"route": route,
							"10":    apiConfig,
						})
					}

					request := httptest.NewRequest(http.MethodPost, "/"+route+"?"+test.browserQuery, strings.NewReader(test.browserBody))
					contentType := test.contentType
					if contentType == "" {
						contentType = "application/json"
					}
					request.Header.Set("Content-Type", contentType)
					response := httptest.NewRecorder()
					ServeContent(response, request)
					if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "true") {
						t.Fatalf("response status=%d body=%q, want rendered successful upstream response", response.Code, response.Body.String())
					}
					select {
					case got := <-received:
						if got.err != nil {
							t.Fatalf("reading upstream body: %v", got.err)
						}
						if got.method != http.MethodPost || got.contentType != "application/json" {
							t.Errorf("upstream method=%s content-type=%q, want configured POST with JSON", got.method, got.contentType)
						}
						if got.query != test.wantQuery {
							t.Errorf("upstream query=%q, want %q", got.query, test.wantQuery)
						}
						assertAPIRequestBodyJSON(t, got.body, test.wantBody)
					default:
						t.Fatal("upstream did not receive the request")
					}
				})
			}
		})
	}
}

// A bodyless browser request can still supply URL values for the separately
// configured upstream body, even when querykeys disables URL forwarding.
func TestServeContentAPIBodylessRequestMapping(t *testing.T) {
	const bodyTemplate = `{"id":"$id","private":"$private","missing":"$missing"}`
	for _, renderer := range []struct {
		name        string
		apiFragment bool
	}{
		{name: "nested api_render"},
		{name: "api_fragment_render", apiFragment: true},
	} {
		for _, browserMethod := range []string{http.MethodGet, http.MethodPost} {
			for _, test := range []struct {
				name, query, template, wantBody string
			}{
				{"URL values", "id=7&private=from-query", bodyTemplate, `{"id":"7","private":"from-query"}`},
				{"empty values", "id=&private=", bodyTemplate, `{"id":"","private":""}`},
				{"missing values", "", bodyTemplate, `{}`},
				{"escaped values", "id=quoted%22&private=path%5Cend", bodyTemplate, `{"id":"quoted\"","private":"path\\end"}`},
				{"literal body", "id=7", `{"id":"fixed"}`, `{"id":"fixed"}`},
			} {
				t.Run(renderer.name+"/"+browserMethod+"/"+test.name, func(t *testing.T) {
					setupDevelopmentModeServeContentTest(t, false)
					received := make(chan string, 1)
					upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
						body, err := io.ReadAll(request.Body)
						if err != nil {
							t.Errorf("reading upstream body: %v", err)
						}
						if request.URL.RawQuery != "" {
							t.Errorf("upstream query=%q, want none with querykeys: []", request.URL.RawQuery)
						}
						if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
							t.Errorf("upstream method=%s content-type=%q, want configured POST with JSON", request.Method, request.Header.Get("Content-Type"))
						}
						received <- string(body)
						writer.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(writer, `{"ok":true}`)
					}))
					t.Cleanup(upstream.Close)
					const route = "api-bodyless-mapping"
					apiConfig := map[string]interface{}{
						"@type":     component.APIConfigGetName(),
						"endpoint":  upstream.URL,
						"method":    http.MethodPost,
						"body":      test.template,
						"headers":   map[string]string{"Content-Type": "application/json"},
						"querykeys": []string{},
						"inline":    `{{.Data.ok}}`,
					}
					if renderer.apiFragment {
						apiConfig["@type"] = composite.ApiFragmentRenderConfigGetName()
						apiConfig["route"] = route
						setTestRouteConfig(route, apiConfig)
					} else {
						setTestRouteConfig(route, map[string]interface{}{
							"@type": composite.FragmentConfigGetName(),
							"route": route,
							"10":    apiConfig,
						})
					}
					var browserBody io.Reader
					if browserMethod == http.MethodPost {
						browserBody = strings.NewReader("")
					}
					request := httptest.NewRequest(browserMethod, "/"+route+"?"+test.query, browserBody)
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					ServeContent(response, request)
					if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "true") {
						t.Fatalf("response status=%d body=%q, want rendered successful upstream response", response.Code, response.Body.String())
					}
					select {
					case got := <-received:
						assertAPIRequestBodyJSON(t, got, test.wantBody)
					default:
						t.Fatal("upstream did not receive the request")
					}
				})
			}
		}
	}
}

func TestServeContentAPIInvalidBodyMappingDoesNotCallUpstream(t *testing.T) {
	for _, apiFragment := range []bool{false, true} {
		componentName := "nested api_render"
		if apiFragment {
			componentName = "api_fragment_render"
		}
		for _, test := range []struct{ name, template string }{
			{"malformed configured JSON", `{"name":"$name",}`},
			{"placeholder embedded in bare JSON literal", `{"number":1$name}`},
			{"missing array element", `["$name","$missing"]`},
			{"missing bare array element", `[$missing,"$name"]`},
			{"missing nested array element", `{"items":["$missing"]}`},
			{"missing value in interpolated string", `{"message":"Hello $missing"}`},
			{"missing top-level string placeholder", `"$missing"`},
			{"missing top-level bare placeholder", `$missing`},
		} {
			t.Run(componentName+"/"+test.name, func(t *testing.T) {
				setupDevelopmentModeServeContentTest(t, false)
				var calls atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					calls.Add(1)
					writer.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(writer, `{"ok":true}`)
				}))
				t.Cleanup(upstream.Close)
				const route = "api-invalid-body-mapping"
				apiConfig := map[string]interface{}{
					"@type":    component.APIConfigGetName(),
					"endpoint": upstream.URL,
					"method":   http.MethodPost,
					"headers":  map[string]string{"Content-Type": "application/json"},
					"body":     test.template,
					"inline":   `upstream-success:{{.Data.ok}}`,
				}
				if apiFragment {
					apiConfig["@type"] = composite.ApiFragmentRenderConfigGetName()
					apiConfig["route"] = route
					setTestRouteConfig(route, apiConfig)
				} else {
					setTestRouteConfig(route, map[string]interface{}{
						"@type": composite.FragmentConfigGetName(),
						"route": route,
						"10":    apiConfig,
					})
				}
				request := httptest.NewRequest(http.MethodPost, "/"+route, strings.NewReader(`{"name":"present"}`))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				ServeContent(response, request)
				if count := calls.Load(); count != 0 {
					t.Errorf("upstream received %d request(s) after invalid body mapping", count)
				}
				if strings.Contains(response.Body.String(), "upstream-success:true") {
					t.Errorf("invalid mapping rendered upstream success: %q", response.Body.String())
				}
			})
		}
	}
}

func assertAPIRequestBodyJSON(t *testing.T, got, want string) {
	t.Helper()
	var gotValue, wantValue interface{}
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("outgoing JSON body is invalid: %v; body=%q", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("invalid expected JSON body: %v; body=%q", err, want)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("upstream JSON body=%s, want %s", got, want)
	}
}
