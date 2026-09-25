package shared

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestStarterModulesResolveDeveloperCredentials(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := os.ReadFile(filepath.Join(repo, "modules", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(catalog), "### Starter modules\n")
	if !found {
		t.Fatal("starter catalog section is missing")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	// Follow the maintained starter catalog instead of keeping a second module list.
	entries := regexp.MustCompile("(?m)^\\| \\[`([a-z0-9-]+)`\\]\\(").FindAllStringSubmatch(section, -1)
	if len(entries) == 0 {
		t.Fatal("starter catalog contains no modules")
	}
	for _, entry := range entries {
		t.Run(entry[1], func(t *testing.T) {
			moduleDir := filepath.Join(repo, "modules", entry[1])
			source, err := os.ReadFile(filepath.Join(moduleDir, PackageConfigFileName))
			if err != nil {
				t.Fatal(err)
			}
			readme, err := os.ReadFile(filepath.Join(moduleDir, "README.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, variable := range []string{"HB_DEVELOPER_USER", "HB_DEVELOPER_PASSWORD"} {
				if !strings.Contains(string(readme), "export "+variable+"=") {
					t.Errorf("README must document setting %s before startup", variable)
				}
			}
			for _, tc := range []struct {
				name, user, password string
				want                 int
			}{
				{"configured", "starter-developer", "test-only: password with spaces", http.StatusNoContent},
				{"unset", "", "", http.StatusServiceUnavailable},
				{"missing user", "", "test-only-password", http.StatusServiceUnavailable},
				{"missing password", "starter-developer", "", http.StatusServiceUnavailable},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Setenv("HB_DEVELOPER_USER", tc.user)
					t.Setenv("HB_DEVELOPER_PASSWORD", tc.password)
					config, err := ValidatePackageConfigBytes(source, moduleDir)
					if err != nil {
						t.Fatal(err)
					}
					credentials := config.Development.Dashboard.Credentials
					if credentials.User != tc.user || credentials.Password != tc.password {
						t.Fatal("developer credentials must resolve the documented environment variables without a built-in fallback")
					}
					handler := BasicAuthWithUnavailable(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}), credentials, DeveloperInterfaceRealm, DeveloperInterfaceUnavailableMessage)
					request := httptest.NewRequest(http.MethodGet, DefaultSpacesRoute, nil)
					request.SetBasicAuth(tc.user, tc.password)
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request)
					if response.Code != tc.want {
						t.Fatalf("developer access = %d, want %d", response.Code, tc.want)
					}
					if credentials.Complete() {
						for _, password := range []string{"", "wrong-password"} {
							request := httptest.NewRequest(http.MethodGet, DefaultSpacesRoute, nil)
							if password != "" {
								request.SetBasicAuth(tc.user, password)
							}
							response := httptest.NewRecorder()
							handler.ServeHTTP(response, request)
							if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") == "" {
								t.Fatalf("missing/incorrect login must challenge, got %d", response.Code)
							}
						}
					}
				})
			}
		})
	}
}
