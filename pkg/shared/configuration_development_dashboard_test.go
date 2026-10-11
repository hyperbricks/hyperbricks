package shared

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevelopmentDashboardLegacyBooleanAndMapping(t *testing.T) {
	for _, tc := range []struct {
		name, setting   string
		enabled, legacy bool
	}{
		{"legacy enabled", "dashboard: true", true, true},
		{"legacy disabled", "dashboard: false", false, true},
		{"quoted enabled", `dashboard: "true"`, true, true},
		{"quoted disabled", `dashboard: "false"`, false, true},
		{"mapping enabled", "dashboard: {enabled: true}", true, false},
		{"mapping disabled", "dashboard: {enabled: false}", false, false},
		{"omitted", "", true, false},
		{"empty mapping", "dashboard: {}", true, false},
		{"credentials only", "dashboard: {credentials: {}}", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf("hyperbricks: {development: {%s}}", tc.setting)
			var normal Config
			if err := decodeDashboardConfig(t, body, &normal); err != nil {
				t.Fatal(err)
			}
			strict, err := ValidatePackageConfigBytes([]byte(body), t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for _, config := range []*Config{&normal, strict} {
				dashboard := config.Development.Dashboard
				if dashboard.Enabled != tc.enabled || !dashboard.Credentials.Empty() || config.ValidateDevelopmentDashboard() != nil {
					t.Fatalf("dashboard = %#v; error = %v", dashboard, config.ValidateDevelopmentDashboard())
				}
				warning := dashboard.LegacyWarning()
				if tc.legacy {
					if !strings.Contains(warning, fmt.Sprintf("development.dashboard.enabled: %t", tc.enabled)) {
						t.Fatalf("missing migration warning: %q", warning)
					}
				} else if warning != "" {
					t.Fatalf("unexpected migration warning: %q", warning)
				}
			}
		})
	}
}

func TestDevelopmentDashboardLegacyDecodeDoesNotLeakState(t *testing.T) {
	var config Config
	if err := decodeDashboardConfig(t, `hyperbricks: {development: {dashboard: {enabled: true, credentials: {user: first, password: secret}}}}`, &config); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		if err := decodeDashboardConfig(t, fmt.Sprintf("hyperbricks: {development: {dashboard: %t}}", enabled), &config); err != nil {
			t.Fatal(err)
		}
		if config.Development.Dashboard.Enabled != enabled || !config.Development.Dashboard.Credentials.Empty() || config.Development.Dashboard.LegacyWarning() == "" {
			t.Fatalf("legacy decode leaked state: %#v", config.Development.Dashboard)
		}
	}
	if err := decodeDashboardConfig(t, `hyperbricks: {development: {dashboard: {enabled: true}}}`, &config); err != nil {
		t.Fatal(err)
	}
	if !config.Development.Dashboard.Enabled || config.Development.Dashboard.LegacyWarning() != "" {
		t.Fatalf("mapping retained legacy state: %#v", config.Development.Dashboard)
	}
}

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

func TestDevelopmentDashboardRejectsInvalidShape(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"string", `hyperbricks: {development: {dashboard: "yes"}}`, "must be a mapping"},
		{"number", `hyperbricks: {development: {dashboard: 1}}`, "must be a mapping"},
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
