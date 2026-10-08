package spaces

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestContextualPayloadUsesManagedSpaceAndSourceAllowlist(t *testing.T) {
	s := testService(t)
	createTest(t, s, "english")
	createTest(t, s, "dutch")
	got, err := s.contextualPayload("portfolio/english")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "english" || got.Editor != s.route || !got.Write || got.Error != "" {
		t.Fatalf("wrong selection: %+v", got)
	}
	found := false
	for _, f := range got.Fields {
		if f.ID == "/content/values/body/values/name" && f.Label == "Name" {
			found = true
		}
		if strings.Contains(f.ID, "private_value") {
			t.Fatal("undeclared value exposed as editable")
		}
	}
	if !found {
		t.Fatalf("nested canonical field missing: %+v", got.Fields)
	}
	for _, route := range []string{"index", "unknown"} {
		got, err := s.contextualPayload(route)
		if err != nil || got.Name != "" || len(got.Fields) != 0 || got.Error == "" {
			t.Fatalf("unmanaged route %q: %+v %v", route, got, err)
		}
	}
	c, sp := currentSpace(t, s, "english")
	if err := s.mutate(c, Mutation{Action: "trash", Revision: c.revision, Name: sp.Name}, nil); err != nil {
		t.Fatal(err)
	}
	got, err = s.contextualPayload("portfolio/english")
	if err != nil || got.Name != "" || got.Error == "" {
		t.Fatalf("trashed route: %+v %v", got, err)
	}
}

func TestContextualInjectionEscapesDataAndPreservesMarkup(t *testing.T) {
	original := `<!doctype html><html><body><script>const sample="</body>";</script><pre>&lt;/body&gt;</pre></body></html>`
	payload := contextualPayload{Name: "english", Editor: "/author/spaces", Fields: []contextualField{{ID: "/values/a.b~1c", Label: `</script><img src=x onerror=alert(1)>`}}}
	got := injectContextualPage(original, payload)
	start := strings.Index(got, `<script id="hb-spaces-context"`)
	if start != strings.LastIndex(original, "</body>") {
		t.Fatalf("inserted inside user script or content: %s", got)
	}
	if !strings.HasSuffix(got, "</body></html>") || !strings.Contains(got, `src="/author/spaces/web/contextual.js"`) {
		t.Fatal(got)
	}
	dataStart := strings.Index(got[start:], ">") + start + 1
	dataEnd := strings.Index(got[dataStart:], "</script>") + dataStart
	var decoded contextualPayload
	if err := json.Unmarshal([]byte(got[dataStart:dataEnd]), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Fields[0] != payload.Fields[0] || strings.Contains(got, `<img src=x`) {
		t.Fatal("unsafe or changed field metadata")
	}
}

func TestContextualPageEnforcesDevelopmentHostAndRequestBoundaries(t *testing.T) {
	s := testService(t)
	createTest(t, s, "english")
	shared.Init_configuration()
	cfg := shared.GetHyperBricksConfiguration()
	old, runtime := *cfg, shared.GetRuntimeOptions()
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime) })
	credentials := shared.CredentialsConfig{User: "developer", Password: "secret"}
	var handler Handler
	const body = "<html><body>Page</body></html>"
	cases := []struct {
		name, mode, method, host, query            string
		production, disabled, spacesDisabled, want bool
	}{
		{name: "development", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost", query: "edit=true", want: true},
		{name: "head", mode: shared.DEVELOPMENT_MODE, method: "HEAD", host: "127.0.0.1", query: "edit=true", want: true},
		{name: "trusted LAN", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "editor.test", query: "q=abc&edit=true", want: true},
		{name: "normal", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost"},
		{name: "false", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost", query: "edit=false"},
		{name: "duplicate", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost", query: "edit=true&edit=false"},
		{name: "untrusted host", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "other.test", query: "edit=true"},
		{name: "disabled", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost", query: "edit=true", disabled: true},
		{name: "Spaces disabled", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost", query: "edit=true", spacesDisabled: true},
		{name: "production", mode: shared.DEVELOPMENT_MODE, method: "GET", host: "localhost", query: "edit=true", production: true},
		{name: "live", mode: shared.LIVE_MODE, method: "GET", host: "localhost", query: "edit=true"},
		{name: "debug", mode: shared.DEBUG_MODE, method: "GET", host: "localhost", query: "edit=true", want: true},
		{name: "post", mode: shared.DEVELOPMENT_MODE, method: "POST", host: "localhost", query: "edit=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Mode = tc.mode
			cfg.Directories = s.dirs
			cfg.Plugins.Enabled = nil
			cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
			cfg.Development.Dashboard.Credentials = credentials
			cfg.Development.FrontendEditing.Enabled = !tc.disabled
			cfg.Development.FrontendEditing.Spaces.Enabled = !tc.spacesDisabled
			cfg.Development.FrontendEditing.Spaces.AllowedHosts = []string{"editor.test"}
			cfg.Development.FrontendEditing.Spaces.Write = false
			shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: s.module, Production: tc.production})
			request := httptest.NewRequest(tc.method, "http://"+tc.host+"/portfolio/english?"+tc.query, nil)
			request.SetBasicAuth(credentials.User, credentials.Password)
			got, active, err := handler.ContextualPage(request, "portfolio/english", body)
			if err != nil || active != tc.want {
				t.Fatalf("active=%v err=%v", active, err)
			}
			if tc.want {
				if !strings.Contains(got, `"name":"english"`) || !strings.Contains(got, `"write":false`) {
					t.Fatal(got)
				}
				// Contextual navigation does not grant write access.
				writer := httptest.NewRecorder()
				request = httptest.NewRequest(http.MethodPost, "http://"+tc.host+cfg.Development.FrontendEditing.Spaces.Route+"/api", strings.NewReader(`{}`))
				request.SetBasicAuth(credentials.User, credentials.Password)
				handler.ServeHTTP(writer, request)
				if writer.Code != 403 {
					t.Fatalf("read-only editor mutation: %d", writer.Code)
				}
			} else if got != body {
				t.Fatal("inactive editing changed response")
			}
		})
	}
}

func TestOrdinaryRequestSkipsInvalidNativeEditingContract(t *testing.T) {
	s := imageService(t)
	createTest(t, s, "first")
	// Invalid editor policy must affect edit=true only, not ordinary rendering.
	writeTestFile(t, s.dirs["hyperbricks"]+"/portfolio.hyperbricks.yaml", strings.Replace(fileText(t, s.dirs["hyperbricks"]+"/portfolio.hyperbricks.yaml"), "alt: text", "width: text", 1))
	shared.Init_configuration()
	cfg := shared.GetHyperBricksConfiguration()
	previous, runtime := *cfg, shared.GetRuntimeOptions()
	t.Cleanup(func() { *cfg = previous; shared.SetRuntimeOptions(runtime) })
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Directories = s.dirs
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{}
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: s.module})
	handler := &Handler{}
	body := "ordinary rendered content"
	for _, query := range []string{"", "?edit=false", "?edit=true"} {
		request := httptest.NewRequest(http.MethodGet, "http://localhost/portfolio/first"+query, nil)
		got, active, err := handler.ContextualPage(request, "portfolio/first", body)
		if query == "?edit=true" {
			if !active || err == nil {
				t.Fatalf("editor did not validate contract: %v %v", active, err)
			}
		} else if active || err != nil || got != body {
			t.Fatalf("ordinary request performed discovery: %v %v", active, err)
		}
	}
}
