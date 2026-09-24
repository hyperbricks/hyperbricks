package commands

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestDeployConfigPathDefaultsToYAML(t *testing.T) {
	t.Setenv("HB_DEPLOY_CONFIG", "")
	previous := DeployConfigPath
	DeployConfigPath = ""
	t.Cleanup(func() { DeployConfigPath = previous })
	if got := deployConfigPath(); got != DeployConfigFileName {
		t.Fatalf("deployConfigPath() = %q, want %q", got, DeployConfigFileName)
	}
}

func TestResolveDeployConfigPathPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		explicit    string
		environment string
		want        string
	}{
		{name: "explicit overrides environment", explicit: " configs/explicit.yaml ", environment: "configs/environment.yaml", want: filepath.Join("configs", "explicit.yaml")},
		{name: "environment overrides default", environment: " configs/environment.yaml ", want: filepath.Join("configs", "environment.yaml")},
		{name: "default", want: DeployConfigFileName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDeployConfigPath(tt.explicit, tt.environment); got != tt.want {
				t.Fatalf("resolveDeployConfigPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadDeployPushConfigReadsGeneratedYAML(t *testing.T) {
	t.Setenv("HB_DEPLOY_CLIENT_PRODUCTION_USER", "deploy-user")
	t.Setenv("HB_DEPLOY_CLIENT_PRODUCTION_PASSWORD", "deploy-password")
	t.Setenv("HB_DEPLOY_CLIENT_PRODUCTION_HMAC_SECRET", "test-secret")
	path := filepath.Join(t.TempDir(), DeployConfigFileName)
	if err := os.WriteFile(path, []byte(deployInitTemplate()), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	cfg, err := loadDeployPushConfig(path)
	if err != nil {
		t.Fatalf("loadDeployPushConfig() error = %v", err)
	}
	target := cfg.Client.Targets["production"]
	if cfg.Client.Target != "production" || target.API != "https://deploy.example.com" || target.KeyID != "" {
		t.Fatalf("client config = %#v", cfg.Client)
	}
	if target.Credentials.User != "deploy-user" || target.Credentials.Password != "deploy-password" || target.HMACSecret != "test-secret" {
		t.Fatalf("client target secrets were not resolved: %#v", target)
	}
}

func TestDeployInitTemplateUsesRoleOwnedSecrets(t *testing.T) {
	t.Setenv("HB_DEPLOY_REMOTE_HMAC_SECRET", "remote-secret")
	path := filepath.Join(t.TempDir(), DeployConfigFileName)
	if err := os.WriteFile(path, []byte(deployInitTemplate()), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	deploy, err := loadDeployYAMLRoot(path)
	if err != nil {
		t.Fatalf("loadDeployYAMLRoot() error = %v", err)
	}
	remote, ok := deploy["remote"].(map[string]interface{})
	if !ok {
		t.Fatalf("remote block = %#v", deploy["remote"])
	}
	if got := remote["hmac_secret"]; got != "remote-secret" {
		t.Fatalf("remote hmac_secret = %q, want %q", got, "remote-secret")
	}
	if _, exists := deploy["hmac_secret"]; exists {
		t.Fatal("obsolete top-level hmac_secret is present")
	}
}

func TestResolveDeploySecretUsesSelectedTarget(t *testing.T) {
	target := shared.DeployClientTarget{HMACSecret: "target-secret", KeyID: "staging"}
	if got := resolveDeploySecret(target); got != "target-secret" {
		t.Fatalf("target secret = %q, want %q", got, "target-secret")
	}
}

func TestResolveDeployTargetKeepsTargetAuthenticationIsolated(t *testing.T) {
	cfg := deployPushConfig{
		Client: shared.DeployClientConfig{
			Target: "staging",
			Targets: map[string]shared.DeployClientTarget{
				"staging": {
					API:         "https://staging.example.com",
					Credentials: shared.CredentialsConfig{User: "staging-user", Password: "staging-password"},
					HMACSecret:  "staging-secret",
				},
				"production": {
					API:         "https://production.example.com",
					Credentials: shared.CredentialsConfig{User: "production-user", Password: "production-password"},
					HMACSecret:  "production-secret",
				},
			},
		},
	}
	name, target, err := resolveDeployTarget(cfg, "production")
	if err != nil {
		t.Fatal(err)
	}
	if name != "production" || target.Credentials.User != "production-user" || resolveDeploySecret(target) != "production-secret" {
		t.Fatalf("selected target = %q %#v", name, target)
	}
}

func TestLoadDeployPushConfigRejectsLegacyOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), DeployConfigFileName)
	if err := os.WriteFile(path, []byte("deploy:\n  hmac_secret: legacy\n  client:\n    targets: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadDeployPushConfig(path); err == nil || !strings.Contains(err.Error(), "hmac_secret") {
		t.Fatalf("legacy config error = %v", err)
	}
}

func TestBuildReleaseURLTargetsHTTPUploadEndpoint(t *testing.T) {
	url, err := buildReleaseURL("https://deploy.example.com/api", "owner-example-test-test")
	if err != nil {
		t.Fatalf("buildReleaseURL() error = %v", err)
	}
	want := "https://deploy.example.com/api/deploy/v1/modules/owner-example-test-test/releases"
	if url.String() != want {
		t.Fatalf("url = %q, want %q", url.String(), want)
	}
}

func TestSignDeployHeadersIncludesKeyAndBuildScope(t *testing.T) {
	body := []byte("hra bytes")
	headers, err := signDeployHeaders("POST", "/deploy/v1/modules/demo/releases", body, "secret", "prod", "build-1")
	if err != nil {
		t.Fatalf("signDeployHeaders() error = %v", err)
	}
	for _, key := range []string{"X-HB-Key-ID", "X-HB-Build-ID", "X-HB-SHA256", "X-HB-Timestamp", "X-HB-Nonce", "X-HB-Signature"} {
		if strings.TrimSpace(headers[key]) == "" {
			t.Fatalf("missing header %s in %#v", key, headers)
		}
	}

	canonical := strings.Join([]string{
		"POST",
		"/deploy/v1/modules/demo/releases",
		headers["X-HB-SHA256"],
		headers["X-HB-Timestamp"],
		headers["X-HB-Nonce"],
		"prod",
		"build-1",
	}, "\n")
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte(canonical))
	if got := headers["X-HB-Signature"]; got != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("signature = %q", got)
	}
}

func TestSignDeployHeadersSharedUploadOmitsKeyAndIncludesBuildScope(t *testing.T) {
	body := []byte("hra bytes")
	headers, err := signDeployHeaders("POST", "/deploy/v1/modules/demo/releases", body, "secret", "", "build-1")
	if err != nil {
		t.Fatalf("signDeployHeaders() error = %v", err)
	}
	if _, ok := headers["X-HB-Key-ID"]; ok {
		t.Fatalf("shared upload unexpectedly contains X-HB-Key-ID: %#v", headers)
	}
	if headers["X-HB-Build-ID"] != "build-1" || strings.TrimSpace(headers["X-HB-SHA256"]) == "" {
		t.Fatalf("shared upload is missing build scope headers: %#v", headers)
	}

	canonical := strings.Join([]string{
		"POST",
		"/deploy/v1/modules/demo/releases",
		headers["X-HB-SHA256"],
		headers["X-HB-Timestamp"],
		headers["X-HB-Nonce"],
		"",
		"build-1",
	}, "\n")
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte(canonical))
	if got := headers["X-HB-Signature"]; got != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("signature = %q", got)
	}
}
