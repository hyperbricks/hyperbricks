package shared

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBasicAuthLocksServiceWithoutCompleteCredentials(t *testing.T) {
	for _, credentials := range []CredentialsConfig{
		{},
		{User: "deploy"},
		{Password: "secret"},
	} {
		handler := BasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("locked service reached protected handler")
		}), credentials, "Deploy")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
		if response.Header().Get("WWW-Authenticate") != "" {
			t.Fatal("locked service must not send an authentication challenge")
		}
		if !strings.Contains(response.Body.String(), "authentication is not configured") {
			t.Fatalf("body = %q", response.Body.String())
		}
	}
}

func TestBasicAuthChallengesAndAcceptsConfiguredCredentials(t *testing.T) {
	called := false
	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}), CredentialsConfig{User: "deploy", Password: "secret"}, "Deploy")

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/", nil))
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("unauthorized response = %d, challenge %q", unauthorized.Code, unauthorized.Header().Get("WWW-Authenticate"))
	}
	if called {
		t.Fatal("unauthorized request reached protected handler")
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetBasicAuth("deploy", "secret")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusNoContent || !called {
		t.Fatalf("authorized response = %d, called = %t", authorized.Code, called)
	}
}

func TestApplyBasicAuthUsesConfiguredCredentials(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://deploy.example.com/status", nil)
	credentials := CredentialsConfig{User: "target-user", Password: "target-password"}
	if err := ApplyBasicAuth(request, credentials, "deploy target"); err != nil {
		t.Fatal(err)
	}
	user, password, ok := request.BasicAuth()
	if !ok || user != credentials.User || password != credentials.Password {
		t.Fatalf("basic auth = (%q, %q, %t)", user, password, ok)
	}
}

func TestApplyBasicAuthRejectsIncompleteCredentials(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://deploy.example.com/status", nil)
	if err := ApplyBasicAuth(request, CredentialsConfig{User: "target-user"}, "deploy target"); err == nil {
		t.Fatal("incomplete target credentials were accepted")
	}
	if request.Header.Get("Authorization") != "" {
		t.Fatal("authorization header was set for incomplete credentials")
	}
}

func TestBasicAuthWithUnavailableUsesDeveloperLockedResponse(t *testing.T) {
	handler := BasicAuthWithUnavailable(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("locked handler was called") }),
		CredentialsConfig{},
		DeveloperInterfaceRealm,
		DeveloperInterfaceUnavailableMessage,
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), DeveloperInterfaceUnavailableMessage) {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("WWW-Authenticate") != "" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("locked headers = %#v", response.Header())
	}
}

func TestBasicAuthAuthorizedPreservesCredentialBytes(t *testing.T) {
	credentials := CredentialsConfig{User: "développeur", Password: " leading and trailing 密碼 "}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetBasicAuth(credentials.User, credentials.Password)
	if !BasicAuthAuthorized(request, credentials) {
		t.Fatal("exact non-ASCII credentials were rejected")
	}
	request.SetBasicAuth(credentials.User, strings.TrimSpace(credentials.Password))
	if BasicAuthAuthorized(request, credentials) {
		t.Fatal("password normalization was accepted")
	}
}

func TestSpacesAuthRequiresCompleteOrEmptyAccount(t *testing.T) {
	account := CredentialsConfig{User: "developer", Password: "secret"}
	for _, tc := range []struct {
		name        string
		credentials CredentialsConfig
		login       CredentialsConfig
		want        int
	}{
		{name: "empty account", want: http.StatusOK},
		{name: "user only", credentials: CredentialsConfig{User: "developer"}, want: http.StatusServiceUnavailable},
		{name: "password only", credentials: CredentialsConfig{Password: "secret"}, want: http.StatusServiceUnavailable},
		{name: "missing login", credentials: account, want: http.StatusUnauthorized},
		{name: "incorrect login", credentials: account, login: CredentialsConfig{User: "developer", Password: "wrong"}, want: http.StatusUnauthorized},
		{name: "correct login", credentials: account, login: account, want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/__hyperbricks/spaces", nil)
			if !tc.login.Empty() {
				request.SetBasicAuth(tc.login.User, tc.login.Password)
			}
			response := httptest.NewRecorder()
			authorized := RequireSpacesAuth(response, request, tc.credentials)
			if response.Code != tc.want || authorized != (tc.want == http.StatusOK) || SpacesAuthAuthorized(request, tc.credentials) != authorized {
				t.Fatalf("response=%d authorized=%v body=%s", response.Code, authorized, response.Body.String())
			}
			if (response.Header().Get("WWW-Authenticate") != "") != (tc.want == http.StatusUnauthorized) {
				t.Fatalf("unexpected challenge: %q", response.Header().Get("WWW-Authenticate"))
			}
			if !authorized && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("rejected authentication response can be cached")
			}
			if tc.want == http.StatusServiceUnavailable && !strings.Contains(response.Body.String(), "set both development.dashboard.credentials.user and password, or leave both empty") {
				t.Fatalf("missing partial-account guidance: %s", response.Body.String())
			}
		})
	}
	if SpacesAuthAuthorized(nil, CredentialsConfig{}) {
		t.Fatal("missing request authorized contextual editing")
	}
}
