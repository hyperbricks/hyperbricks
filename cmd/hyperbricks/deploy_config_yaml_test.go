package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
)

func TestLoadDeployConfigReadsYAMLSpec(t *testing.T) {
	t.Setenv("HB_DEPLOY_REMOTE_USER", "deploy-user")
	t.Setenv("HB_DEPLOY_REMOTE_PASSWORD", "deploy-password")
	t.Setenv("HB_DEPLOY_REMOTE_HMAC_SECRET", "test-secret")
	path := filepath.Join(t.TempDir(), "deploy.hyperbricks.yaml")
	if err := os.WriteFile(path, []byte(`
deploy:
  remote:
    bind: 127.0.0.1
    port: 9092
    root: deploy-remote
    port_start: 8180
    logs_enabled: true
    binary: /usr/local/bin/hyperbricks
    credentials:
      user:
        env: HB_DEPLOY_REMOTE_USER
      password:
        env: HB_DEPLOY_REMOTE_PASSWORD
    hmac_secret:
      env: HB_DEPLOY_REMOTE_HMAC_SECRET
`), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	cfg, err := loadDeployConfig(path)
	if err != nil {
		t.Fatalf("loadDeployConfig() error = %v", err)
	}
	if cfg.Remote.HMACSecret != "test-secret" {
		t.Fatalf("remote hmac_secret = %q", cfg.Remote.HMACSecret)
	}
	if cfg.Remote.Port != 9092 || cfg.Remote.Root != "deploy-remote" || cfg.Remote.PortStart != 8180 {
		t.Fatalf("remote config = %#v", cfg.Remote)
	}
	if cfg.Remote.Credentials.User != "deploy-user" || cfg.Remote.Credentials.Password != "deploy-password" {
		t.Fatalf("remote credentials = %#v", cfg.Remote.Credentials)
	}
	if cfg.Remote.Binary != "/usr/local/bin/hyperbricks" {
		t.Fatalf("remote binary = %q", cfg.Remote.Binary)
	}
	if cfg.Remote.Auth.EnvPrefix != "HB_DEPLOY_SECRET_" {
		t.Fatalf("remote auth env_prefix = %q", cfg.Remote.Auth.EnvPrefix)
	}
}

func TestDeployServiceLoadersRequireOnlyTheirSelectedRole(t *testing.T) {
	localPath := filepath.Join(t.TempDir(), "local.yaml")
	if err := os.WriteFile(localPath, []byte(`
deploy:
  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy
    port_start: 8181
    logs_enabled: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	local, err := loadDeployLocalConfig(localPath)
	if err != nil {
		t.Fatalf("load local-only config: %v", err)
	}
	if local.Local.PortStart != 8181 || local.Local.LogsEnabled {
		t.Fatalf("local ownership = %#v", local.Local)
	}
	if _, err := loadDeployConfig(localPath); err == nil || !strings.Contains(err.Error(), "deploy.remote") {
		t.Fatalf("remote loader accepted local-only config: %v", err)
	}

	remotePath := filepath.Join(t.TempDir(), "remote.yaml")
	if err := os.WriteFile(remotePath, []byte(`
deploy:
  remote:
    bind: 127.0.0.1
    port: 9090
    root: deploy
    port_start: 8282
    logs_enabled: true
    hmac_secret: remote-secret
`), 0o644); err != nil {
		t.Fatal(err)
	}
	remote, err := loadDeployConfig(remotePath)
	if err != nil {
		t.Fatalf("load remote-only config: %v", err)
	}
	if remote.Remote.PortStart != 8282 || remote.Remote.HMACSecret != "remote-secret" {
		t.Fatalf("remote ownership = %#v", remote.Remote)
	}
	if _, err := loadDeployLocalConfig(remotePath); err == nil || !strings.Contains(err.Error(), "deploy.local") {
		t.Fatalf("local loader accepted remote-only config: %v", err)
	}
}

func TestValidateDeployArchiveMetadata(t *testing.T) {
	version := strings.TrimSpace(assets.VersionMD)
	tests := []struct {
		name       string
		module     string
		metadata   map[string]string
		wantStatus int
		wantError  string
	}{
		{
			name:       "matching metadata",
			module:     "demo",
			metadata:   map[string]string{"module": "demo", "hyperbricks": version},
			wantStatus: http.StatusOK,
		},
		{
			name:       "module mismatch",
			module:     "demo",
			metadata:   map[string]string{"module": "other", "hyperbricks": version},
			wantStatus: http.StatusBadRequest,
			wantError:  "does not match deploy module",
		},
		{
			name:       "version mismatch",
			module:     "demo",
			metadata:   map[string]string{"module": "demo", "hyperbricks": "v0.0.0"},
			wantStatus: http.StatusConflict,
			wantError:  "does not match deploy host version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := validateDeployArchiveMetadata(tt.module, tt.metadata)
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantStatus)
			}
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestDeploySecretEnvNameNormalizesModuleAndKey(t *testing.T) {
	got := deploySecretEnvName("HB_DEPLOY_SECRET_", "owner-example-test-test", "prod")
	want := "HB_DEPLOY_SECRET_OWNER_EXAMPLE_TEST_TEST_PROD"
	if got != want {
		t.Fatalf("deploySecretEnvName() = %q, want %q", got, want)
	}
}

func TestDeployKeyedEnvSecretForRequest(t *testing.T) {
	t.Setenv("HB_DEPLOY_SECRET_OWNER_EXAMPLE_TEST_TEST_PROD", "deploy-secret")
	api := deployAPI{authEnvPrefix: "HB_DEPLOY_SECRET_"}
	req, err := http.NewRequest(http.MethodPost, "/deploy/v1/modules/owner-example-test-test/releases", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-HB-Key-ID", "prod")

	secret, err := api.secretForRequest(req)
	if err != nil {
		t.Fatalf("secretForRequest() error = %v", err)
	}
	if secret != "deploy-secret" {
		t.Fatalf("secret = %q", secret)
	}
}

func TestDeploySharedSecretForRequest(t *testing.T) {
	api := deployAPI{secret: "shared-secret", authEnvPrefix: "HB_DEPLOY_SECRET_"}
	req, err := http.NewRequest(http.MethodPost, "/deploy/v1/modules/demo/releases", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	secret, err := api.secretForRequest(req)
	if err != nil {
		t.Fatalf("secretForRequest() error = %v", err)
	}
	if secret != "shared-secret" {
		t.Fatalf("secret = %q, want %q", secret, "shared-secret")
	}
}

func TestDeployKeyedRequestDoesNotUseSharedSecret(t *testing.T) {
	t.Setenv("HB_DEPLOY_SECRET_DEMO_STAGING", "")
	api := deployAPI{secret: "shared-secret", authEnvPrefix: "HB_DEPLOY_SECRET_"}
	req, err := http.NewRequest(http.MethodPost, "/deploy/v1/modules/demo/releases", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-HB-Key-ID", "staging")

	if _, err := api.secretForRequest(req); err == nil {
		t.Fatal("keyed request unexpectedly fell back to the shared secret")
	}
}

func TestVerifyRequestUsesKeyedEnvSecret(t *testing.T) {
	t.Setenv("HB_DEPLOY_SECRET_OWNER_EXAMPLE_TEST_TEST_PROD", "deploy-secret")
	api := deployAPI{
		authEnvPrefix: "HB_DEPLOY_SECRET_",
		nonceStore:    newDeployNonceStore(),
	}
	body := []byte("hra archive bytes")
	req, err := http.NewRequest(http.MethodPost, "/deploy/v1/modules/owner-example-test-test/releases", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	timestamp := time.Now().UTC().Unix()
	nonce := "nonce-for-test"
	keyID := "prod"
	buildID := "build-123"
	hash := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(hash[:])
	canonical := strings.Join([]string{
		http.MethodPost,
		"/deploy/v1/modules/owner-example-test-test/releases",
		bodyHash,
		strconv.FormatInt(timestamp, 10),
		nonce,
		keyID,
		buildID,
	}, "\n")
	mac := hmac.New(sha256.New, []byte("deploy-secret"))
	_, _ = mac.Write([]byte(canonical))

	req.Header.Set("X-HB-Key-ID", keyID)
	req.Header.Set("X-HB-Build-ID", buildID)
	req.Header.Set("X-HB-SHA256", bodyHash)
	req.Header.Set("X-HB-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-HB-Nonce", nonce)
	req.Header.Set("X-HB-Signature", hex.EncodeToString(mac.Sum(nil)))

	if err := api.verifyRequest(req, body); err != nil {
		t.Fatalf("verifyRequest() error = %v", err)
	}
}

func TestVerifyRequestUsesSharedSecretForArchiveUpload(t *testing.T) {
	api := deployAPI{
		secret:     "shared-secret",
		nonceStore: newDeployNonceStore(),
	}
	body := []byte("hra archive bytes")
	req, err := http.NewRequest(http.MethodPost, "/deploy/v1/modules/demo/releases", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	timestamp := time.Now().UTC().Unix()
	nonce := "shared-nonce-for-test"
	buildID := "build-123"
	hash := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(hash[:])
	canonical := strings.Join([]string{
		http.MethodPost,
		"/deploy/v1/modules/demo/releases",
		bodyHash,
		strconv.FormatInt(timestamp, 10),
		nonce,
		"",
		buildID,
	}, "\n")
	mac := hmac.New(sha256.New, []byte("shared-secret"))
	_, _ = mac.Write([]byte(canonical))

	req.Header.Set("X-HB-Build-ID", buildID)
	req.Header.Set("X-HB-SHA256", bodyHash)
	req.Header.Set("X-HB-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-HB-Nonce", nonce)
	req.Header.Set("X-HB-Signature", hex.EncodeToString(mac.Sum(nil)))

	if err := api.verifyRequest(req, body); err != nil {
		t.Fatalf("verifyRequest() error = %v", err)
	}
}

func TestLoadDeployLocalConfigReadsYAMLSpec(t *testing.T) {
	t.Setenv("HB_DEPLOY_LOCAL_USER", "local-user")
	t.Setenv("HB_DEPLOY_LOCAL_PASSWORD", "local-password")
	t.Setenv("HB_DEPLOY_CLIENT_STAGING_USER", "client-user")
	t.Setenv("HB_DEPLOY_CLIENT_STAGING_PASSWORD", "client-password")
	t.Setenv("HB_DEPLOY_CLIENT_STAGING_HMAC_SECRET", "test-secret")
	path := filepath.Join(t.TempDir(), "deploy.hyperbricks.yaml")
	if err := os.WriteFile(path, []byte(`
deploy:
  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy
    port_start: 8180
    logs_enabled: true
    credentials:
      user:
        env: HB_DEPLOY_LOCAL_USER
      password:
        env: HB_DEPLOY_LOCAL_PASSWORD
  client:
    target: staging
    targets:
      staging:
        api: https://deploy.example.com
        credentials:
          user:
            env: HB_DEPLOY_CLIENT_STAGING_USER
          password:
            env: HB_DEPLOY_CLIENT_STAGING_PASSWORD
        hmac_secret:
          env: HB_DEPLOY_CLIENT_STAGING_HMAC_SECRET
        key_id: staging
`), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	cfg, err := loadDeployLocalConfig(path)
	if err != nil {
		t.Fatalf("loadDeployLocalConfig() error = %v", err)
	}
	if cfg.Local.Port != 9091 || cfg.Local.ModulesDir != "modules" || cfg.Local.PortStart != 8180 {
		t.Fatalf("local config = %#v", cfg.Local)
	}
	if cfg.Local.Credentials.User != "local-user" || cfg.Local.Credentials.Password != "local-password" {
		t.Fatalf("local credentials = %#v", cfg.Local.Credentials)
	}
	target := cfg.Client.Targets["staging"]
	if cfg.Client.Target != "staging" || target.API != "https://deploy.example.com" || target.KeyID != "staging" {
		t.Fatalf("client config = %#v", cfg.Client)
	}
	if target.HMACSecret != "test-secret" || target.Credentials.User != "client-user" || target.Credentials.Password != "client-password" {
		t.Fatalf("client target auth = %#v", target)
	}
}
