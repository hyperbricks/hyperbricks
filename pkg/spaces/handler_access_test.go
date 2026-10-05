package spaces

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func configureHandlerTest(t *testing.T, s *service) *shared.Config {
	t.Helper()
	shared.Init_configuration()
	cfg := shared.GetHyperBricksConfiguration()
	old, runtime := *cfg, shared.GetRuntimeOptions()
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime) })
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Directories = s.dirs
	cfg.Plugins.Enabled = nil
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{}
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: s.module})
	return cfg
}

func TestHandlerAndContextualPageShareAccessPolicy(t *testing.T) {
	account := shared.CredentialsConfig{User: "developer", Password: "secret"}
	for _, tc := range []struct {
		name, mode, host               string
		credentials, login             shared.CredentialsConfig
		production, disabled, readonly bool
		watch, wantWatch               bool
		want                           int
	}{
		{name: "development without account", want: 200},
		{name: "debug without account", mode: shared.DEBUG_MODE, want: 200},
		{name: "development watching", watch: true, wantWatch: true, want: 200},
		{name: "debug does not watch", mode: shared.DEBUG_MODE, watch: true, wantWatch: false, want: 200},
		{name: "read-only without account", readonly: true, want: 200},
		{name: "allowed LAN without account", host: "192.0.2.10:8080", want: 200},
		{name: "IPv6 loopback", host: "[::1]:8080", want: 200},
		{name: "unlisted host", host: "other.test", want: 403},
		{name: "public origin does not allow editor host", host: "www.example.com", want: 403},
		{name: "configured account", credentials: account, want: 401},
		{name: "wrong password", credentials: account, login: shared.CredentialsConfig{User: "developer", Password: "wrong"}, want: 401},
		{name: "authenticated account", credentials: account, login: account, want: 200},
		{name: "authenticated debug", mode: shared.DEBUG_MODE, credentials: account, login: account, want: 200},
		{name: "authenticated unlisted host", host: "other.test", credentials: account, login: account, want: 403},
		{name: "user only", credentials: shared.CredentialsConfig{User: "developer"}, want: 503},
		{name: "password only", credentials: shared.CredentialsConfig{Password: "secret"}, want: 503},
		{name: "disabled", disabled: true, want: 404},
		{name: "live", mode: shared.LIVE_MODE, want: 404},
		{name: "production", production: true, want: 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testService(t)
			createTest(t, s, "english")
			cfg := configureHandlerTest(t, s)
			if tc.mode != "" {
				cfg.Mode = tc.mode
			}
			cfg.Development.Dashboard.Credentials = tc.credentials
			cfg.Development.Watch = tc.watch
			cfg.Development.FrontendEditing.Spaces.Enabled = !tc.disabled
			cfg.Development.FrontendEditing.Spaces.Write = !tc.readonly
			cfg.Development.FrontendEditing.Spaces.AllowedHosts = []string{"192.0.2.10"}
			cfg.Development.FrontendEditing.Spaces.PublicOrigin = "https://www.example.com"
			shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: s.module, Production: tc.production})
			host := tc.host
			if host == "" {
				host = "localhost"
			}
			request := httptest.NewRequest(http.MethodGet, "http://"+host+s.route+"/api", nil)
			if !tc.login.Empty() {
				request.SetBasicAuth(tc.login.User, tc.login.Password)
			}
			var handler Handler
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("Spaces status=%d want=%d body=%s", response.Code, tc.want, response.Body.String())
			}
			if tc.want == 200 {
				var snapshot Snapshot
				if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
					t.Fatal(err)
				}
				if snapshot.Write == tc.readonly {
					t.Fatalf("write policy=%v, readonly=%v", snapshot.Write, tc.readonly)
				}
				if snapshot.Watch != tc.wantWatch {
					t.Fatalf("watch=%v want=%v", snapshot.Watch, tc.wantWatch)
				}
			}
			request.URL.Path = "/portfolio/english"
			request.URL.RawQuery = "edit=true"
			const content = "<html><body>Preview</body></html>"
			page, active, err := handler.ContextualPage(request, "portfolio/english", content)
			if err != nil || active != (tc.want == 200) {
				t.Fatalf("contextual active=%v err=%v", active, err)
			}
			if active {
				wantWrite := `"write":true`
				if tc.readonly {
					wantWrite = `"write":false`
				}
				if !strings.Contains(page, `"name":"english"`) || !strings.Contains(page, wantWrite) {
					t.Fatalf("contextual payload missing policy: %s", page)
				}
			} else if page != content {
				t.Fatal("unavailable contextual editor modified page")
			}
		})
	}
}

func TestHandlerUnauthenticatedWritesRetainHostOriginAndWriteGuards(t *testing.T) {
	for _, tc := range []struct {
		name, mode, host, origin, fetchSite string
		readonly, noMarker                  bool
		credentials                         shared.CredentialsConfig
		want                                int
	}{
		{name: "development write", want: 200},
		{name: "debug write", mode: shared.DEBUG_MODE, want: 200},
		{name: "allowed LAN write", host: "192.0.2.10:8080", origin: "http://192.0.2.10:8080", want: 200},
		{name: "read-only", readonly: true, want: 403},
		{name: "cross-origin", origin: "https://evil.example", want: 403},
		{name: "wrong origin port", origin: "http://localhost:8080", want: 403},
		{name: "cross-site fetch", fetchSite: "cross-site", want: 403},
		{name: "missing marker", noMarker: true, want: 403},
		{name: "unlisted host", host: "other.test", origin: "http://other.test", want: 403},
		{name: "partial account", credentials: shared.CredentialsConfig{User: "developer"}, want: 503},
		{name: "configured account", credentials: shared.CredentialsConfig{User: "developer", Password: "secret"}, want: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testService(t)
			createTest(t, s, "english")
			cfg := configureHandlerTest(t, s)
			if tc.mode != "" {
				cfg.Mode = tc.mode
			}
			cfg.Development.FrontendEditing.Spaces.Write = !tc.readonly
			cfg.Development.FrontendEditing.Spaces.AllowedHosts = []string{"192.0.2.10"}
			cfg.Development.Dashboard.Credentials = tc.credentials
			c, sp := currentSpace(t, s, "english")
			mutation := saveMutation(c, sp)
			mutation.Title = "Updated title"
			body, err := json.Marshal(mutation)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(s.dirs["hyperbricks"], filepath.FromSlash(sp.File))
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			host, origin := tc.host, tc.origin
			if host == "" {
				host = "localhost"
			}
			if origin == "" {
				origin = "http://localhost"
			}
			request := httptest.NewRequest(http.MethodPost, "http://"+host+s.route+"/api", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", origin)
			request.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			if !tc.noMarker {
				request.Header.Set("X-Spaces-Request", "1")
			}
			response := httptest.NewRecorder()
			var handler Handler
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("write status=%d want=%d body=%s", response.Code, tc.want, response.Body.String())
			}
			after, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == 200 {
				_, saved := currentSpace(t, s, "english")
				if saved.Title != "Updated title" {
					t.Fatalf("mutation was not saved: %q", saved.Title)
				}
			} else if !bytes.Equal(before, after) {
				t.Fatal("rejected write changed the source file")
			}
		})
	}
}
