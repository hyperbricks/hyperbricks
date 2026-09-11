package commands

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployConfigPathDefaultsToYAML(t *testing.T) {
	t.Setenv("HB_DEPLOY_CONFIG", "")
	if got := deployConfigPath(); got != DeployConfigFileName {
		t.Fatalf("deployConfigPath() = %q, want %q", got, DeployConfigFileName)
	}
}

func TestLoadDeployPushConfigReadsGeneratedYAML(t *testing.T) {
	t.Setenv("HB_DEPLOY_SECRET", "test-secret")
	path := filepath.Join(t.TempDir(), DeployConfigFileName)
	if err := os.WriteFile(path, []byte(deployInitTemplate("local")), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	cfg, err := loadDeployPushConfig(path)
	if err != nil {
		t.Fatalf("loadDeployPushConfig() error = %v", err)
	}
	if cfg.HMACSecret != "test-secret" {
		t.Fatalf("hmac_secret = %q", cfg.HMACSecret)
	}
	if cfg.Remote.APIPort != 9090 {
		t.Fatalf("remote api port = %d", cfg.Remote.APIPort)
	}
	target := cfg.Client.Targets["prod"]
	if cfg.Client.Target != "prod" || target.API != "https://deploy.example.com" || target.KeyID != "" {
		t.Fatalf("client config = %#v", cfg.Client)
	}
}

func TestRemoteDeployInitTemplateUsesSharedSecretEnvironment(t *testing.T) {
	t.Setenv("HB_DEPLOY_SECRET", "shared-secret")
	path := filepath.Join(t.TempDir(), DeployConfigFileName)
	if err := os.WriteFile(path, []byte(deployInitTemplate("remote")), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	deploy, err := loadDeployYAMLRoot(path)
	if err != nil {
		t.Fatalf("loadDeployYAMLRoot() error = %v", err)
	}
	if got := deploy["hmac_secret"]; got != "shared-secret" {
		t.Fatalf("hmac_secret = %q, want %q", got, "shared-secret")
	}
}

func TestResolveDeploySecretSelectsSharedAndScopedModes(t *testing.T) {
	cfg := deployPushConfig{HMACSecret: "shared-secret"}
	if got := resolveDeploySecret(cfg, "demo", ""); got != "shared-secret" {
		t.Fatalf("shared secret = %q, want %q", got, "shared-secret")
	}

	t.Setenv("HB_DEPLOY_SECRET_DEMO_STAGING", "scoped-secret")
	if got := resolveDeploySecret(cfg, "demo", "staging"); got != "scoped-secret" {
		t.Fatalf("scoped secret = %q, want %q", got, "scoped-secret")
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
