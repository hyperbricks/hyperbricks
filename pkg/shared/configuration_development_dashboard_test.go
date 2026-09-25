package shared

import (
	"path/filepath"
	"strings"
	"testing"
)

func decodeDashboardConfig(t *testing.T, body string, config *Config) error {
	t.Helper()
	dir := t.TempDir()
	writePackageConfig(t, dir, "", body)
	parsed, err := LoadPackageConfigMap(filepath.Join(dir, PackageConfigFileName), dir)
	if err != nil {
		return err
	}
	return decodeConfig(parsed["hyperbricks"], config)
}

func TestDevelopmentDashboardMapDecodesResolvedCredentials(t *testing.T) {
	t.Setenv("HB_TEST_DEVELOPER_USER", "module-developer")
	t.Setenv("HB_TEST_DEVELOPER_PASSWORD", "sp ace:秘密")
	var config Config
	err := decodeDashboardConfig(t, `hyperbricks:
  development:
    dashboard:
      enabled: true
      credentials:
        user:
          env: HB_TEST_DEVELOPER_USER
        password:
          env: HB_TEST_DEVELOPER_PASSWORD
`, &config)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := config.Development.Dashboard
	if !dashboard.Enabled || dashboard.Credentials.User != "module-developer" || dashboard.Credentials.Password != "sp ace:秘密" {
		t.Fatalf("dashboard = %#v", dashboard)
	}
}

func TestDevelopmentDashboardRejectsBooleanAndInvalidCredentialShape(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"boolean", `hyperbricks: {development: {dashboard: true}}`, "must be a mapping"},
		{"null", `hyperbricks: {development: {dashboard: null}}`, "must be a mapping"},
		{"enabled", `hyperbricks: {development: {dashboard: {enabled: yes}}}`, "must be a boolean"},
		{"credentials", `hyperbricks: {development: {dashboard: {credentials: invalid}}}`, "credentials must be a mapping"},
		{"pass", `hyperbricks: {development: {dashboard: {credentials: {user: dev, pass: secret}}}}`, "use development.dashboard.credentials.password"},
		{"unknown", `hyperbricks: {development: {dashboard: {enabled: false, account: dev}}}`, "invalid keys"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var config Config
			err := decodeDashboardConfig(t, tc.body, &config)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if config.ValidateDevelopmentDashboard() == nil {
				t.Fatal("invalid dashboard configuration was not retained")
			}
		})
	}
}

func TestDevelopmentDashboardSequentialDecodeDoesNotLeakCredentials(t *testing.T) {
	var config Config
	if err := decodeDashboardConfig(t, `hyperbricks: {development: {dashboard: {enabled: true, credentials: {user: first, password: secret}}}}`, &config); err != nil {
		t.Fatal(err)
	}
	if err := decodeDashboardConfig(t, `hyperbricks: {development: {dashboard: {enabled: false}}}`, &config); err != nil {
		t.Fatal(err)
	}
	if config.Development.Dashboard.Enabled || !config.Development.Dashboard.Credentials.Empty() {
		t.Fatalf("dashboard state leaked across decode: %#v", config.Development.Dashboard)
	}
}
