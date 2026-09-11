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
)

func TestLoadDeployConfigReadsYAMLSpec(t *testing.T) {
	t.Setenv("HB_DEPLOY_SECRET", "test-secret")
	path := filepath.Join(t.TempDir(), "deploy.hyperbricks.yaml")
	if err := os.WriteFile(path, []byte(`
deploy:
  hmac_secret:
    env: HB_DEPLOY_SECRET
  remote:
    api_enabled: true
    api_bind: 127.0.0.1
    api_port: 9092
    root: deploy-remote
    port_start: 8180
    logs_enabled: true
    binary: /usr/local/bin/hyperbricks
`), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	cfg, err := loadDeployConfig(path)
	if err != nil {
		t.Fatalf("loadDeployConfig() error = %v", err)
	}
	if cfg.HMACSecret != "test-secret" {
		t.Fatalf("hmac_secret = %q", cfg.HMACSecret)
	}
	if cfg.Remote.APIPort != 9092 || cfg.Remote.Root != "deploy-remote" || cfg.Remote.PortStart != 8180 {
		t.Fatalf("remote config = %#v", cfg.Remote)
	}
	if cfg.Remote.Binary != "/usr/local/bin/hyperbricks" {
		t.Fatalf("remote binary = %q", cfg.Remote.Binary)
	}
	if cfg.Remote.Auth.EnvPrefix != "HB_DEPLOY_SECRET_" {
		t.Fatalf("remote auth env_prefix = %q", cfg.Remote.Auth.EnvPrefix)
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
	t.Setenv("HB_DEPLOY_SECRET", "test-secret")
	path := filepath.Join(t.TempDir(), "deploy.hyperbricks.yaml")
	if err := os.WriteFile(path, []byte(`
deploy:
  hmac_secret:
    env: HB_DEPLOY_SECRET
  remote:
    api_enabled: true
    api_bind: 127.0.0.1
    api_port: 9090
    root: deploy
    port_start: 8080
    logs_enabled: true
  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy
  client:
    target: staging
    targets:
      staging:
        api: https://deploy.example.com
        key_id: staging
`), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	cfg, err := loadDeployLocalConfig(path)
	if err != nil {
		t.Fatalf("loadDeployLocalConfig() error = %v", err)
	}
	if cfg.HMACSecret != "test-secret" {
		t.Fatalf("hmac_secret = %q", cfg.HMACSecret)
	}
	if cfg.Local.Port != 9091 || cfg.Local.ModulesDir != "modules" {
		t.Fatalf("local config = %#v", cfg.Local)
	}
	target := cfg.Client.Targets["staging"]
	if cfg.Client.Target != "staging" || target.API != "https://deploy.example.com" || target.KeyID != "staging" {
		t.Fatalf("client config = %#v", cfg.Client)
	}
}
