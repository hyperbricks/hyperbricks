package composite

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

func TestRenderAPIResponseCookiesStructuredValuePreservesOpaqueCharacters(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://app.example.com/login", nil)
	entries := []interface{}{
		map[string]interface{}{
			"name":      "__Host-session",
			"value":     "{{.Data.token}}",
			"path":      "/",
			"http_only": true,
			"secure":    true,
			"same_site": "lax",
			"max_age":   3600,
		},
	}

	got, err := RenderAPIResponseCookies("", entries, map[string]interface{}{
		"token": "opaque+value&part",
	}, http.StatusNoContent, nil, request)
	if err != nil {
		t.Fatalf("RenderAPIResponseCookies() error = %v", err)
	}
	want := []string{"__Host-session=opaque+value&part; Path=/; Max-Age=3600; HttpOnly; Secure; SameSite=Lax"}
	assertAPIResponseCookieHeaders(t, got, want)
}

func TestRenderAPIResponseCookiesPreservesLegacyOrderAndSupportsStructuredEntries(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://app.example.com/login", nil)
	entries := []interface{}{
		"account={{.Data.token}}; Path=/; HttpOnly; Secure; SameSite=Strict",
		map[string]interface{}{
			"name":      "theme",
			"value":     "{{.theme}}",
			"path":      "/",
			"same_site": "lax",
		},
	}

	got, err := RenderAPIResponseCookies(
		"old_session=; Path=/; HttpOnly; Secure; Max-Age=0",
		entries,
		map[string]interface{}{"token": "abc123"},
		http.StatusOK,
		map[string]interface{}{"theme": "dark"},
		request,
	)
	if err != nil {
		t.Fatalf("RenderAPIResponseCookies() error = %v", err)
	}
	want := []string{
		"old_session=; Path=/; Max-Age=0; HttpOnly; Secure",
		"account=abc123; Path=/; HttpOnly; Secure; SameSite=Strict",
		"theme=dark; Path=/; SameSite=Lax",
	}
	assertAPIResponseCookieHeaders(t, got, want)
}

func TestRenderAPIResponseCookiesUsesTopLevelStringData(t *testing.T) {
	got, err := RenderAPIResponseCookies(
		"session={{.Data}}; Path=/; HttpOnly",
		nil,
		"top-level-token",
		http.StatusCreated,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("RenderAPIResponseCookies() error = %v", err)
	}
	assertAPIResponseCookieHeaders(t, got, []string{"session=top-level-token; Path=/; HttpOnly"})
}

func TestRenderAPIResponseCookiesIgnoresNonSuccessfulUpstreamStatus(t *testing.T) {
	for _, status := range []int{http.StatusContinue, http.StatusFound, http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			got, err := RenderAPIResponseCookies("not even a cookie", nil, nil, status, nil, nil)
			if err != nil {
				t.Fatalf("status %d returned error: %v", status, err)
			}
			if got != nil {
				t.Fatalf("status %d returned cookies: %v", status, got)
			}
		})
	}
}

func TestRenderAPIResponseCookiesRejectsInvalidDynamicValues(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		want string
	}{
		{name: "missing", data: map[string]interface{}{}, want: "is missing"},
		{name: "null", data: map[string]interface{}{"token": nil}, want: "is null"},
		{name: "non-string", data: map[string]interface{}{"token": 42}, want: "must resolve to a string"},
		{name: "empty", data: map[string]interface{}{"token": ""}, want: "nonempty string"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := RenderAPIResponseCookies("session={{.Data.token}}; Path=/", nil, test.data, http.StatusOK, nil, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
			if got != nil {
				t.Fatalf("invalid dynamic value returned staged cookies: %v", got)
			}
		})
	}
}

func TestRenderAPIResponseCookiesAllowsOnlyExplicitLiteralDeletion(t *testing.T) {
	got, err := RenderAPIResponseCookies("session=; Path=/; HttpOnly; Max-Age=0", nil, nil, http.StatusNoContent, nil, nil)
	if err != nil {
		t.Fatalf("literal deletion error = %v", err)
	}
	assertAPIResponseCookieHeaders(t, got, []string{"session=; Path=/; Max-Age=0; HttpOnly"})

	for name, single := range map[string]string{
		"empty without deletion": "session=; Path=/; HttpOnly",
		"dynamic deletion":       "session={{.Data.token}}; Path=/; HttpOnly; Max-Age=0",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := RenderAPIResponseCookies(single, nil, map[string]interface{}{"token": ""}, http.StatusOK, nil, nil)
			if err == nil {
				t.Fatal("expected empty cookie value to be rejected")
			}
			if got != nil {
				t.Fatalf("invalid empty value returned staged cookies: %v", got)
			}
		})
	}
}

func TestRenderAPIResponseCookiesRejectsUnsupportedTemplateConstructs(t *testing.T) {
	tests := []string{
		`session={{printf "%s" .Data.token}}; Path=/`,
		`session={{if .Data.token}}{{.Data.token}}{{end}}; Path=/`,
		`session={{.Data.token | printf "%s"}}; Path=/`,
		`{{.Data.name}}=fixed; Path=/`,
		`session=fixed; Domain={{.Data.domain}}`,
	}
	for _, single := range tests {
		got, err := RenderAPIResponseCookies(single, nil, map[string]interface{}{
			"name": "session", "token": "abc", "domain": "example.com",
		}, http.StatusOK, nil, nil)
		if err == nil {
			t.Fatalf("template %q was accepted", single)
		}
		if got != nil {
			t.Fatalf("invalid template returned staged cookies: %v", got)
		}
	}
}

func TestRenderAPIResponseCookiesRejectsHeaderInjection(t *testing.T) {
	for _, value := range []string{"abc; Domain=evil.example", "abc\r\nX-Injected: true"} {
		got, err := RenderAPIResponseCookies(
			"session={{.Data.token}}; Path=/; HttpOnly",
			nil,
			map[string]interface{}{"token": value},
			http.StatusOK,
			nil,
			nil,
		)
		if err == nil {
			t.Fatalf("injected value %q was accepted", value)
		}
		if got != nil {
			t.Fatalf("injected value returned staged cookies: %v", got)
		}
	}
}

func TestAPIResponseCookiesRejectRawControlsBeforeWhitespaceNormalization(t *testing.T) {
	tests := []struct {
		name    string
		single  string
		entries []interface{}
		raw     map[string]interface{}
	}{
		{
			name:   "leading CRLF",
			single: "\r\nsession=abc; Path=/",
			raw:    map[string]interface{}{"setcookie": "\r\nsession=abc; Path=/"},
		},
		{
			name:   "trailing newline",
			single: "session=abc; Path=/\n",
			raw:    map[string]interface{}{"setcookie": "session=abc; Path=/\n"},
		},
		{
			name:   "control-only singular",
			single: "\r\n",
			raw:    map[string]interface{}{"setcookie": "\r\n"},
		},
		{
			name:    "control-only plural",
			entries: []interface{}{"\x00"},
			raw:     map[string]interface{}{"setcookies": []interface{}{"\x00"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateAPIResponseCookiesRaw(test.raw); err == nil || !strings.Contains(err.Error(), "control byte") {
				t.Fatalf("ValidateAPIResponseCookiesRaw() error = %v, want control-byte rejection", err)
			}
			got, err := RenderAPIResponseCookies(test.single, test.entries, nil, http.StatusOK, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "control byte") {
				t.Fatalf("RenderAPIResponseCookies() error = %v, want control-byte rejection", err)
			}
			if got != nil {
				t.Fatalf("invalid raw cookie returned staged cookies: %v", got)
			}
		})
	}
}

func TestRenderAPIResponseCookiesRejectsWhitespaceNormalizedLegacyNames(t *testing.T) {
	for _, single := range []string{
		" session=abc; Path=/",
		"session =abc; Path=/",
	} {
		got, err := RenderAPIResponseCookies(single, nil, nil, http.StatusOK, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "cookie name") {
			t.Fatalf("legacy cookie %q error = %v, want cookie-name rejection", single, err)
		}
		if got != nil {
			t.Fatalf("invalid cookie name returned staged cookies: %v", got)
		}
	}
}

func TestRenderAPIResponseCookiesKeepsQuotedValuesAndAttributeSpacing(t *testing.T) {
	got, err := RenderAPIResponseCookies(
		`session="opaque+value&part";   Path=/;  HttpOnly`,
		nil,
		nil,
		http.StatusOK,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("RenderAPIResponseCookies() error = %v", err)
	}
	assertAPIResponseCookieHeaders(t, got, []string{`session="opaque+value&part"; Path=/; HttpOnly`})
}

func TestRenderAPIResponseCookiesStagesAtomically(t *testing.T) {
	got, err := RenderAPIResponseCookies(
		"first=valid; Path=/",
		[]interface{}{"second={{.Data.missing}}; Path=/"},
		map[string]interface{}{},
		http.StatusOK,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected second cookie to fail")
	}
	if got != nil {
		t.Fatalf("partial staged result escaped: %v", got)
	}
}

func TestRenderAPIResponseCookiesRejectsReservedTemplateContextKeys(t *testing.T) {
	for _, key := range []string{"Data", "Status"} {
		got, err := RenderAPIResponseCookies(
			"session=literal; Path=/",
			nil,
			nil,
			http.StatusOK,
			map[string]interface{}{key: "override"},
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("key %q error = %v, want reserved-key error", key, err)
		}
		if got != nil {
			t.Fatalf("reserved key returned staged cookies: %v", got)
		}
	}
}

func TestRenderAPIResponseCookiesValidatesDomainAgainstIncomingRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://app.example.com/login", nil)
	tests := []struct {
		name     string
		domain   string
		request  *http.Request
		wantFail bool
	}{
		{name: "parent domain", domain: "example.com", request: request},
		{name: "leading dot parent domain", domain: ".example.com", request: request},
		{name: "unrelated domain", domain: "evil.example", request: request, wantFail: true},
		{name: "public suffix", domain: "com", request: request, wantFail: true},
		{name: "missing request", domain: "example.com", wantFail: true},
		{name: "matching IP", domain: "127.0.0.1", request: httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)},
		{name: "different IP", domain: "127.0.0.2", request: httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil), wantFail: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := []interface{}{map[string]interface{}{
				"name": "session", "value": "abc", "domain": test.domain, "path": "/",
			}}
			got, err := RenderAPIResponseCookies("", entries, nil, http.StatusOK, nil, test.request)
			if test.wantFail {
				if err == nil || got != nil {
					t.Fatalf("got cookies=%v error=%v, want atomic failure", got, err)
				}
				return
			}
			if err != nil || len(got) != 1 {
				t.Fatalf("got cookies=%v error=%v, want one cookie", got, err)
			}
		})
	}
}

func TestRenderAPIResponseCookiesEnforcesCookieControls(t *testing.T) {
	tests := []struct {
		name  string
		entry map[string]interface{}
	}{
		{name: "SameSite None without Secure", entry: map[string]interface{}{"name": "session", "value": "abc", "same_site": "none"}},
		{name: "Secure prefix without Secure", entry: map[string]interface{}{"name": "__Secure-session", "value": "abc"}},
		{name: "Host prefix without root Path", entry: map[string]interface{}{"name": "__Host-session", "value": "abc", "secure": true, "path": "/nested"}},
		{name: "Host prefix with Domain", entry: map[string]interface{}{"name": "__Host-session", "value": "abc", "secure": true, "path": "/", "domain": "example.com"}},
		{name: "Http prefix without HttpOnly", entry: map[string]interface{}{"name": "__Http-session", "value": "abc", "secure": true}},
		{name: "relative Path", entry: map[string]interface{}{"name": "session", "value": "abc", "path": "nested"}},
	}

	request := httptest.NewRequest(http.MethodGet, "https://app.example.com/", nil)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := RenderAPIResponseCookies("", []interface{}{test.entry}, nil, http.StatusOK, nil, request)
			if err == nil || got != nil {
				t.Fatalf("got cookies=%v error=%v, want atomic failure", got, err)
			}
		})
	}

	valid := map[string]interface{}{
		"name": "__Host-Http-session", "value": "abc", "path": "/", "secure": true, "http_only": true, "same_site": "none",
	}
	got, err := RenderAPIResponseCookies("", []interface{}{valid}, nil, http.StatusOK, nil, request)
	if err != nil {
		t.Fatalf("valid prefixed cookie error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("valid prefixed cookie headers = %v", got)
	}
}

func TestRenderAPIResponseCookiesRejectsMalformedLegacyAttributes(t *testing.T) {
	for _, single := range []string{
		"session=abc; Path=/; Path=/other",
		"session=abc; Priority=High",
		"session=abc; Secure=true",
		"session=abc; SameSite=maybe",
		"session=abc; Partitioned",
	} {
		got, err := RenderAPIResponseCookies(single, nil, nil, http.StatusOK, nil, nil)
		if err == nil || got != nil {
			t.Fatalf("legacy cookie %q returned cookies=%v error=%v, want atomic failure", single, got, err)
		}
	}
}

func TestValidateAPIResponseCookiesRawEnforcesCookieTypes(t *testing.T) {
	valid := map[string]interface{}{
		"setcookie": "legacy=abc; Path=/",
		"setcookies": []interface{}{
			"second=abc; Path=/",
			map[string]interface{}{
				"name": "session", "value": "{{.Data.token}}", "path": "/", "http_only": true,
				"secure": true, "same_site": "lax", "max_age": 60, "expires": "Wed, 21 Oct 2037 07:28:00 GMT",
			},
		},
	}
	if err := ValidateAPIResponseCookiesRaw(valid); err != nil {
		t.Fatalf("valid raw cookie config error = %v", err)
	}

	tests := []struct {
		name string
		raw  map[string]interface{}
	}{
		{name: "single boolean", raw: map[string]interface{}{"setcookie": false}},
		{name: "plural scalar", raw: map[string]interface{}{"setcookies": "session=abc"}},
		{name: "entry scalar", raw: map[string]interface{}{"setcookies": []interface{}{42}}},
		{name: "unknown field", raw: map[string]interface{}{"setcookies": []interface{}{map[string]interface{}{"name": "a", "value": "b", "priority": "high"}}}},
		{name: "non-string value", raw: map[string]interface{}{"setcookies": []interface{}{map[string]interface{}{"name": "a", "value": 12}}}},
		{name: "string boolean", raw: map[string]interface{}{"setcookies": []interface{}{map[string]interface{}{"name": "a", "value": "b", "secure": "true"}}}},
		{name: "float max age", raw: map[string]interface{}{"setcookies": []interface{}{map[string]interface{}{"name": "a", "value": "b", "max_age": float64(60)}}}},
		{name: "negative max age", raw: map[string]interface{}{"setcookies": []interface{}{map[string]interface{}{"name": "a", "value": "b", "max_age": -1}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateAPIResponseCookiesRaw(test.raw); err == nil {
				t.Fatalf("raw config was accepted: %#v", test.raw)
			}
		})
	}
}

func TestApiFragmentConfigPreservesRawStructuredCookieScalarTypes(t *testing.T) {
	factory := typefactory.NewTypeFactory()
	factory.RegisterType(ApiFragmentRenderConfigGetName(), reflect.TypeOf(ApiFragmentRenderConfig{}))

	rawCookie := map[string]interface{}{
		"name":      "session",
		"value":     "{{.Data.token}}",
		"path":      "/",
		"http_only": true,
		"secure":    true,
		"max_age":   60,
	}
	raw := map[string]interface{}{
		"setcookies": []interface{}{rawCookie},
	}
	response, err := factory.CreateInstance(typefactory.TypeRequest{
		TypeName: ApiFragmentRenderConfigGetName(),
		Data:     raw,
	})
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	config, ok := response.Instance.(ApiFragmentRenderConfig)
	if !ok {
		t.Fatalf("CreateInstance() type = %T, want ApiFragmentRenderConfig", response.Instance)
	}
	entries := config.responseCookieEntries()
	if len(entries) != 1 {
		t.Fatalf("responseCookieEntries() = %#v, want one entry", entries)
	}
	captured, ok := entries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("captured cookie type = %T, want map[string]interface{}", entries[0])
	}
	if _, ok := captured["http_only"].(bool); !ok {
		t.Fatalf("captured http_only type = %T, want bool", captured["http_only"])
	}
	if _, ok := captured["max_age"].(int); !ok {
		t.Fatalf("captured max_age type = %T, want int", captured["max_age"])
	}

	// The captured security input is immutable with respect to later parser or
	// caller mutations.
	rawCookie["http_only"] = "true"
	rawCookie["max_age"] = "60"
	if captured["http_only"] != true || captured["max_age"] != 60 {
		t.Fatalf("captured cookie changed after raw input mutation: %#v", captured)
	}

	got, err := RenderAPIResponseCookies(
		config.SetCookie,
		config.responseCookieEntries(),
		map[string]interface{}{"token": "opaque+value&part"},
		http.StatusOK,
		config.Values,
		httptest.NewRequest(http.MethodPost, "https://app.example.com/login", nil),
	)
	if err != nil {
		t.Fatalf("RenderAPIResponseCookies() with captured entries error = %v", err)
	}
	assertAPIResponseCookieHeaders(t, got, []string{
		"session=opaque+value&part; Path=/; Max-Age=60; HttpOnly; Secure",
	})
}

func TestApiFragmentConfigDirectStructUsesDecodedCookieEntries(t *testing.T) {
	direct := []interface{}{map[string]interface{}{
		"name": "session", "value": "literal", "path": "/", "http_only": true,
	}}
	config := ApiFragmentRenderConfig{APIConfig: APIConfig{SetCookies: direct}}

	got := config.responseCookieEntries()
	if len(got) != 1 || !reflect.DeepEqual(got, direct) {
		t.Fatalf("responseCookieEntries() = %#v, want direct SetCookies %#v", got, direct)
	}
}

func assertAPIResponseCookieHeaders(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("cookie headers = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("cookie header[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}
