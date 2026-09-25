package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func credentialString(value string) *string { return &value }
func credentialBool(value bool) *bool       { return &value }

func TestDeployCredentialsSourcePatching(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		update            deployCredentialsUpdate
	}{
		{
			name:   "missing hyperbricks",
			input:  "# package\nmyconf: {free: value}\n",
			want:   "# package\nhyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\"\nmyconf: {free: value}\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "missing development",
			input:  "hyperbricks:\n  mode: live # retain\n",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\"\n  mode: live # retain\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "missing dashboard",
			input:  "hyperbricks:\n  development:\n    watch: true\n",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\"\n    watch: true\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "missing credentials keeps disabled",
			input:  "hyperbricks:\n  development:\n    dashboard:\n      enabled: false # keep\n",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\"\n        user: \"admin\"\n      enabled: false # keep\n",
			update: deployCredentialsUpdate{User: credentialString("admin"), Password: credentialString("new")},
		},
		{
			name:   "quoted fields and special values",
			input:  "hyperbricks:\n  development:\n    dashboard:\n      enabled: true\n      credentials:\n        'user': old # account\n        password: 'old # password' # comment\nmyconf: {untouched: 'foo'}\n",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      enabled: true\n      credentials:\n        'user': \"new-user\" # account\n        password: \"new: # \\\"秘密\" # comment\nmyconf: {untouched: 'foo'}\n",
			update: deployCredentialsUpdate{User: credentialString("new-user"), Password: credentialString("new: # \"秘密")},
		},
		{
			name:   "preserve LF free vars and user resolver",
			input:  "vars:\n  free: 'literal'\nhyperbricks:\n  mode: live\n  development:\n    dashboard:\n      enabled: false\n      credentials:\n        user:\n          env: DEVELOPER_USER\n        password: old\n# end\n",
			want:   "vars:\n  free: 'literal'\nhyperbricks:\n  mode: live\n  development:\n    dashboard:\n      enabled: false\n      credentials:\n        user:\n          env: DEVELOPER_USER\n        password: \"new\"\n# end\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "replace block env and retain comments",
			input:  "hyperbricks:\r\n  development:\r\n    dashboard:\r\n      credentials:\r\n        password: # password source\r\n          # environment setting\r\n          env: DEVELOPER_PASSWORD\r\n        user: 'developer'\r\n  mode: live\r\n",
			want:   "hyperbricks:\r\n  development:\r\n    dashboard:\r\n      credentials:\r\n        password: \"new\" # password source\r\n          # environment setting\r\n        user: 'developer'\r\n  mode: live\r\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "replace block scalar removes old secret lines",
			input:  "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: |- # note\n          old\n          # secret data\n        user: admin\n",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\" # note\n        user: admin\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "null credentials placeholder",
			input:  "hyperbricks:\n  development:\n    dashboard:\n      enabled: false\n      credentials: # account\n  mode: live\n",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      enabled: false\n      credentials: # account\n        password: \"new\"\n        user: \"admin\"\n  mode: live\n",
			update: deployCredentialsUpdate{User: credentialString("admin"), Password: credentialString("new")},
		},
		{
			name:   "explicit null dashboard placeholder",
			input:  "hyperbricks:\n  development:\n    dashboard: null # dashboard\n  mode: live\n",
			want:   "hyperbricks:\n  development:\n    dashboard: # dashboard\n      credentials:\n        password: \"new\"\n  mode: live\n",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "null hierarchy no final newline",
			input:  "hyperbricks:\n  development: ~",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\"",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
		{
			name:   "preserve no final newline",
			input:  "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: old",
			want:   "hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"new\"",
			update: deployCredentialsUpdate{Password: credentialString("new")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := patchDeployCredentials([]byte(test.input), test.update)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("patched text differs:\nwant: %q\n got: %q", test.want, got)
			}
			if _, err := parseDeployCredentialDocument(got); err != nil {
				t.Fatalf("patched YAML invalid: %v", err)
			}
		})
	}
}

func TestDeployCredentialsVisibilityPatching(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		update            deployCredentialsUpdate
	}{
		{
			name:   "update both without touching credentials or free variables",
			input:  "vars: {free: 'keep'}\nhyperbricks:\n  development:\n    dashboard:\n      enabled: false # dashboard\n      credentials:\n        user: admin\n        password: old\n    frontend_editing:\n      enabled: true\n      spaces:\n        route: /__hyperbricks/spaces\n        enabled: true # spaces\n",
			want:   "vars: {free: 'keep'}\nhyperbricks:\n  development:\n    dashboard:\n      enabled: true # dashboard\n      credentials:\n        user: admin\n        password: old\n    frontend_editing:\n      enabled: true\n      spaces:\n        route: /__hyperbricks/spaces\n        enabled: false # spaces\n",
			update: deployCredentialsUpdate{DashboardEnabled: credentialBool(true), SpacesEnabled: credentialBool(false)},
		},
		{
			name:   "insert missing settings alongside existing credentials",
			input:  "hyperbricks:\n  mode: live\n  development:\n    dashboard:\n      credentials:\n        user: admin\n        password: old\n    watch: true\n",
			want:   "hyperbricks:\n  mode: live\n  development:\n    frontend_editing:\n      spaces:\n        enabled: false\n    dashboard:\n      enabled: true\n      credentials:\n        user: admin\n        password: old\n    watch: true\n",
			update: deployCredentialsUpdate{DashboardEnabled: credentialBool(true), SpacesEnabled: credentialBool(false)},
		},
		{
			name:   "spaces only inserts missing hierarchy without enabling dashboard",
			input:  "hyperbricks:\n  mode: live\n",
			want:   "hyperbricks:\n  development:\n    frontend_editing:\n      spaces:\n        enabled: false\n  mode: live\n",
			update: deployCredentialsUpdate{SpacesEnabled: credentialBool(false)},
		},
		{
			name:   "replace quoted booleans preserving CRLF and comments",
			input:  "hyperbricks:\r\n  development:\r\n    dashboard:\r\n      enabled: 'false' # dashboard\r\n    frontend_editing:\r\n      spaces:\r\n        enabled: \"true\" # spaces\r\n",
			want:   "hyperbricks:\r\n  development:\r\n    dashboard:\r\n      enabled: true # dashboard\r\n    frontend_editing:\r\n      spaces:\r\n        enabled: false # spaces\r\n",
			update: deployCredentialsUpdate{DashboardEnabled: credentialBool(true), SpacesEnabled: credentialBool(false)},
		},
		{
			name:   "null spaces placeholder and no final newline",
			input:  "hyperbricks:\n  development:\n    frontend_editing:\n      spaces: null # editor",
			want:   "hyperbricks:\n  development:\n    frontend_editing:\n      spaces: # editor\n        enabled: false",
			update: deployCredentialsUpdate{SpacesEnabled: credentialBool(false)},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := patchDeployCredentials([]byte(test.input), test.update)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("patched text differs:\nwant: %q\n got: %q", test.want, got)
			}
			if _, err := parseDeployCredentialDocument(got); err != nil {
				t.Fatalf("patched YAML invalid: %v", err)
			}
		})
	}
}

func TestDeployCredentialsVisibilityRejectAmbiguousSource(t *testing.T) {
	for _, source := range []string{
		"hyperbricks: {development: {frontend_editing: {spaces: {enabled: true}}}}\n",
		"hyperbricks:\n  development:\n    frontend_editing:\n      spaces:\n        enabled: &flag true\n",
		"hyperbricks:\n  development:\n    frontend_editing:\n      spaces:\n        enabled: true\n        enabled: false\n",
	} {
		if _, err := patchDeployCredentials([]byte(source), deployCredentialsUpdate{SpacesEnabled: credentialBool(false)}); err == nil {
			t.Fatalf("accepted unsafe source %q", source)
		}
	}
}

func TestDeployCredentialsRejectAmbiguousMutation(t *testing.T) {
	for _, source := range []string{
		"hyperbricks: {development: {dashboard: {credentials: {password: old}}}}\n",
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: \"old\n          multiline\"\n",
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: old\n          multiline\n",
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: &anchor old\n",
		"shared: &shared old\nhyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: *shared\n",
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: old\n        password: duplicate\n",
	} {
		if _, err := patchDeployCredentials([]byte(source), deployCredentialsUpdate{Password: credentialString("new")}); err == nil {
			t.Fatalf("accepted unsafe source %q", source)
		}
	}
}

func TestDeployCredentialsNeverResolveSecrets(t *testing.T) {
	t.Setenv("HB_CREDENTIAL_TEST_PASSWORD", "RESOLVED_PASSWORD_MUST_NOT_LEAK")
	content := []byte("hyperbricks:\n  development:\n    dashboard:\n      enabled: true\n      credentials:\n        user: literal-user\n        password:\n          env: HB_CREDENTIAL_TEST_PASSWORD\n")
	response := httptest.NewRecorder()
	if err := writeDeployCredentialsResponse(response, "demo", "build-1", deployPackageConfigLocation{scope: "runtime"}, content); err != nil {
		t.Fatal(err)
	}
	payload := decodeResponseMap(t, response)
	if payload["user"].(map[string]interface{})["source"] != "literal" || payload["password"].(map[string]interface{})["source"] != "reference" {
		t.Fatalf("credential descriptions = %#v", payload)
	}
	if strings.Contains(response.Body.String(), "RESOLVED_PASSWORD_MUST_NOT_LEAK") || !strings.Contains(response.Body.String(), "HB_CREDENTIAL_TEST_PASSWORD") {
		t.Fatal("response must contain the unresolved reference, never the environment secret")
	}
}

func TestDeployCredentialsVisibilityResponse(t *testing.T) {
	for _, test := range []struct {
		name, source                       string
		dashboard, spaces, frontendEditing bool
	}{
		{"defaults", "hyperbricks:\n  mode: development\n", false, true, true},
		{"explicit", "hyperbricks:\n  development:\n    dashboard:\n      enabled: true\n    frontend_editing:\n      spaces:\n        enabled: false\n", true, false, true},
		{"parent disabled", "hyperbricks:\n  development:\n    frontend_editing:\n      enabled: false\n      spaces:\n        enabled: true\n", false, true, false},
		{"quoted booleans", "hyperbricks:\n  development:\n    dashboard:\n      enabled: 'false'\n    frontend_editing:\n      spaces:\n        enabled: \"true\"\n", false, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			if err := writeDeployCredentialsResponse(response, "demo", "dev", deployPackageConfigLocation{}, []byte(test.source)); err != nil {
				t.Fatal(err)
			}
			payload := decodeResponseMap(t, response)
			if payload["dashboard_enabled"] != test.dashboard || payload["spaces_enabled"] != test.spaces || payload["frontend_editing_enabled"] != test.frontendEditing {
				t.Fatalf("visibility = dashboard %#v spaces %#v parent %#v", payload["dashboard_enabled"], payload["spaces_enabled"], payload["frontend_editing_enabled"])
			}
		})
	}
}

func TestDeployCredentialsDescribeMissing(t *testing.T) {
	for _, source := range []string{
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        user: ' '\n        password: null\n",
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        user: ''\n        password: ''\n",
		"hyperbricks:\n  development:\n    dashboard:\n      credentials:\n",
	} {
		response := httptest.NewRecorder()
		if err := writeDeployCredentialsResponse(response, "demo", "dev", deployPackageConfigLocation{}, []byte(source)); err != nil {
			t.Fatal(err)
		}
		payload := decodeResponseMap(t, response)
		for _, field := range []string{"user", "password"} {
			value := payload[field].(map[string]interface{})
			if value["source"] != "missing" || value["value"] != "" {
				t.Fatalf("%s = %#v", field, value)
			}
		}
	}
}

func TestDeployCredentialsRoutesSaveOnlySelectedConfiguration(t *testing.T) {
	for _, target := range []string{"local-dev", "local-build", "remote-build"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			moduleRoot := filepath.Join(root, "modules", "demo")
			if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			config := []byte("hyperbricks:\n  mode: live\n  development:\n    dashboard:\n      enabled: false\n      credentials:\n        user: original\n        password: old\nmyconf:\n  free: 'keep' # note\n")
			sourcePath := filepath.Join(moduleRoot, shared.PackageConfigFileName)
			if err := os.WriteFile(sourcePath, config, 0o640); err != nil {
				t.Fatal(err)
			}
			buildRoot := filepath.Join(root, "deploy")
			archivePath := filepath.Join(buildRoot, "demo", "build-1.hra")
			archive := writeDeployHRAFixture(t, archivePath, string(config))
			local := &deployLocalServer{modulesDir: filepath.Join(root, "modules"), buildRoot: buildRoot}
			remote := &deployAPI{root: buildRoot}
			endpoint := "/local/modules/demo/builds/build-1/credentials"
			var handler http.HandlerFunc = local.handleModuleRoutes
			configPath := filepath.Join(buildRoot, "demo", "runtime", "build-1", shared.PackageConfigFileName)
			scope := "runtime"
			if target == "remote-build" {
				if err := saveDeployIndex(remote.indexPath("demo"), deployIndex{Versions: []deployIndexRow{{BuildID: "build-1", File: archivePath, Format: "hra", RuntimeMode: "live"}}}); err != nil {
					t.Fatal(err)
				}
				endpoint = "/deploy/modules/demo/builds/build-1/credentials"
				handler = remote.handleDeploy
			} else {
				if err := saveLocalBuildIndex(local.indexPath("demo"), localBuildIndex{Versions: []localBuildRow{{BuildID: "build-1", File: archivePath, Format: "hra", RuntimeMode: "live"}}}); err != nil {
					t.Fatal(err)
				}
				if target == "local-dev" {
					endpoint = "/local/modules/demo/builds/dev/credentials"
					configPath = sourcePath
					scope = "source"
				}
			}
			get := httptest.NewRecorder()
			handler(get, httptest.NewRequest(http.MethodGet, endpoint, nil))
			if get.Code != http.StatusOK || get.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("GET status=%d body=%s headers=%v", get.Code, get.Body.String(), get.Header())
			}
			opened := decodeResponseMap(t, get)
			if opened["scope"] != scope {
				t.Fatalf("scope = %v", opened["scope"])
			}
			if opened["dashboard_enabled"] != false || opened["spaces_enabled"] != true {
				t.Fatalf("GET visibility flags = %#v", opened)
			}
			update := deployCredentialsUpdate{ExpectedSHA256: opened["sha256"].(string), Password: credentialString("new: # password"), DashboardEnabled: credentialBool(true), SpacesEnabled: credentialBool(false)}
			body, _ := json.Marshal(update)
			put := httptest.NewRecorder()
			handler(put, httptest.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body)))
			if put.Code != http.StatusOK || put.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("PUT status=%d body=%s", put.Code, put.Body.String())
			}
			got, err := os.ReadFile(configPath)
			want := strings.Replace(string(config), "password: old", "password: \"new: # password\"", 1)
			want = strings.Replace(want, "enabled: false\n", "enabled: true\n", 1)
			want = strings.Replace(want, "  development:\n", "  development:\n    frontend_editing:\n      spaces:\n        enabled: false\n", 1)
			if err != nil || string(got) != want {
				t.Fatalf("saved configuration differs err=%v\nwant: %q\n got: %q", err, want, got)
			}
			if gotArchive, err := os.ReadFile(archivePath); err != nil || !bytes.Equal(gotArchive, archive) {
				t.Fatal("credential save changed immutable archive")
			}
			stale := httptest.NewRecorder()
			handler(stale, httptest.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body)))
			if stale.Code != http.StatusConflict || stale.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("stale PUT = %d %s", stale.Code, stale.Body.String())
			}
			if target == "local-dev" {
				info, _ := os.Stat(configPath)
				if info.Mode().Perm() != 0o640 {
					t.Fatal("source file permissions changed")
				}
			} else if source, _ := os.ReadFile(sourcePath); !bytes.Equal(source, config) {
				t.Fatal("archive credential save changed source module")
			}
		})
	}
}

func TestDeployCredentialsRouteSavesVisibilityWithoutCredentials(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte("hyperbricks:\n  development:\n    watch: true\nmyconf: {free: 'keep'}\n")
	path := filepath.Join(moduleRoot, shared.PackageConfigFileName)
	if err := os.WriteFile(path, config, 0o640); err != nil {
		t.Fatal(err)
	}
	local := &deployLocalServer{modulesDir: filepath.Join(root, "modules")}
	endpoint := "/local/modules/demo/builds/dev/credentials"
	get := httptest.NewRecorder()
	local.handleModuleRoutes(get, httptest.NewRequest(http.MethodGet, endpoint, nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
	opened := decodeResponseMap(t, get)
	update := deployCredentialsUpdate{
		ExpectedSHA256:   opened["sha256"].(string),
		DashboardEnabled: credentialBool(true),
		SpacesEnabled:    credentialBool(false),
	}
	body, err := json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	put := httptest.NewRecorder()
	local.handleModuleRoutes(put, httptest.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body)))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", put.Code, put.Body.String())
	}
	updated := decodeResponseMap(t, put)
	if updated["dashboard_enabled"] != true || updated["spaces_enabled"] != false || updated["frontend_editing_enabled"] != true || updated["sha256"] == opened["sha256"] {
		t.Fatalf("PUT visibility response = %#v", updated)
	}
	for _, field := range []string{"user", "password"} {
		value := updated[field].(map[string]interface{})
		if value["source"] != "missing" {
			t.Fatalf("unexpected credential value %s: %#v", field, value)
		}
	}
	want := "hyperbricks:\n  development:\n    frontend_editing:\n      spaces:\n        enabled: false\n    dashboard:\n      enabled: true\n    watch: true\nmyconf: {free: 'keep'}\n"
	if saved, err := os.ReadFile(path); err != nil || string(saved) != want {
		t.Fatalf("saved visibility config differs err=%v\nwant: %q\n got: %q", err, want, saved)
	}
}

func TestDeployCredentialsRequireAuthAndSignedRemoteRequests(t *testing.T) {
	api := &deployAPI{secret: "test-hmac", nonceStore: newDeployNonceStore(), root: t.TempDir()}
	endpoint := "/deploy/modules/demo/builds/build-1/credentials"
	handler := shared.BasicAuth(api.wrapAuth(api.handleDeploy), shared.CredentialsConfig{User: "admin", Password: "test-auth"}, "test")
	for _, auth := range []string{"none", "basic", "signed"} {
		t.Run(auth, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, endpoint, nil)
			if auth != "none" {
				request.SetBasicAuth("admin", "test-auth")
			}
			if auth == "signed" {
				timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
				bodyHash := sha256.Sum256(nil)
				canonical := strings.Join([]string{http.MethodGet, endpoint, hex.EncodeToString(bodyHash[:]), timestamp, "credentials-test"}, "\n")
				mac := hmac.New(sha256.New, []byte(api.secret))
				_, _ = mac.Write([]byte(canonical))
				request.Header.Set("X-HB-Timestamp", timestamp)
				request.Header.Set("X-HB-Nonce", "credentials-test")
				request.Header.Set("X-HB-Signature", hex.EncodeToString(mac.Sum(nil)))
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			want := http.StatusUnauthorized
			if auth == "signed" {
				want = http.StatusBadRequest // authenticated, but no selected build
			}
			if response.Code != want || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d want=%d body=%s headers=%v", response.Code, want, response.Body.String(), response.Header())
			}
		})
	}
}

func TestDeployCredentialsRejectInvalidRequestsWithoutReflectingValues(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, body := range []string{
		`{"password":"PRIVATE_NEW_PASSWORD"}`,
		`{"expected_sha256":"` + hash + `","password":""}`,
		`{"expected_sha256":"` + hash + `","user":"bad:name"}`,
		`{"expected_sha256":"` + hash + `","password":"new\npassword"}`,
		`{"expected_sha256":"` + hash + `","unknown":"PRIVATE_NEW_PASSWORD"}`,
		`{"expected_sha256":"` + hash + `","dashboard_enabled":"true"}`,
		`{"expected_sha256":"` + hash + `","frontend_editing_enabled":false}`,
		`{"expected_sha256":"` + hash + `","password":"PRIVATE_NEW_PASSWORD"} {}`,
		`{"expected_sha256":"` + hash + `","password":"` + strings.Repeat("x", maxDeployCredentialsRequestBytes) + `"}`,
	} {
		_, err := decodeDeployCredentialsUpdate(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)))
		if err == nil || strings.Contains(err.Error(), "PRIVATE_NEW_PASSWORD") {
			t.Fatalf("unsafe invalid request result: %v", err)
		}
	}
}

func TestDeployCredentialsRejectNullVisibilityFields(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, field := range []string{"dashboard_enabled", "spaces_enabled"} {
		body := `{"expected_sha256":"` + hash + `","user":"new-user","` + field + `":null}`
		_, err := decodeDeployCredentialsUpdate(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)))
		if err == nil || !strings.Contains(err.Error(), "must be booleans") {
			t.Fatalf("accepted %s:null with a valid username update: %v", field, err)
		}
		// Reject null even when a duplicate later supplies a boolean. A JSON
		// object with conflicting toggle values must not have a hidden winner.
		body = `{"expected_sha256":"` + hash + `","` + field + `":null,"` + field + `":true}`
		_, err = decodeDeployCredentialsUpdate(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)))
		if err == nil || !strings.Contains(err.Error(), "must be booleans") {
			t.Fatalf("accepted duplicate %s with null: %v", field, err)
		}
	}
}

func TestDeployCredentialsValidationErrorsNeverReflectSecrets(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, shared.PackageConfigFileName)
	content := []byte("hyperbricks:\n  server:\n    port: PRIVATE_OLD_PASSWORD\n  development:\n    dashboard:\n      credentials:\n        user: admin\n        password: PRIVATE_OLD_PASSWORD\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(deployCredentialsUpdate{ExpectedSHA256: packageConfigSHA256(content), Password: credentialString("PRIVATE_NEW_PASSWORD")})
	response := httptest.NewRecorder()
	handleDeployCredentialsRequest(response, httptest.NewRequest(http.MethodPut, "/credentials", bytes.NewReader(body)), "demo", "dev", deployPackageConfigLocation{moduleRoot: root, path: path, scope: "source"})
	if response.Code != http.StatusUnprocessableEntity || strings.Contains(response.Body.String(), "PRIVATE_") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe validation response status=%d body=%s", response.Code, response.Body.String())
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, content) {
		t.Fatal("failed validation changed original configuration")
	}
}

func TestDeployCredentialsConfinedSource(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(root, "outside.yaml")
	if err := os.WriteFile(outsidePath, []byte("hyperbricks:\n  development:\n    dashboard:\n      credentials:\n        password: OUTSIDE_SECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(moduleRoot, shared.PackageConfigFileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	api := &deployLocalServer{modulesDir: filepath.Join(root, "modules")}
	response := httptest.NewRecorder()
	api.handleModuleRoutes(response, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/dev/credentials", nil))
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "OUTSIDE_SECRET") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("symlink response status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDeployCredentialsRemoteRequestSizeBoundBeforeHMAC(t *testing.T) {
	api := &deployAPI{secret: "test-hmac", nonceStore: newDeployNonceStore()}
	called := false
	handler := api.wrapAuth(func(http.ResponseWriter, *http.Request) { called = true })
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodPut, "/deploy/modules/demo/builds/build-1/credentials", strings.NewReader(strings.Repeat("x", maxDeployCredentialsRequestBytes+1))))
	if response.Code != http.StatusRequestEntityTooLarge || called || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("oversized response status=%d called=%v headers=%v", response.Code, called, response.Header())
	}
}
