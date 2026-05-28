package commands

import (
	"os"
	"path/filepath"
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
	if cfg.Client.Target != "prod" || target.User != "deploy" || target.Root != "/opt/hyperbricks/deploy" {
		t.Fatalf("client config = %#v", cfg.Client)
	}
}

func TestLoadDeployPushConfigRejectsUnsupportedDeployDSL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.hyperbricks")
	if err := os.WriteFile(path, []byte(`
deploy {
  hmac_secret = old-dsl
}
`), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	if _, err := loadDeployPushConfig(path); err == nil {
		t.Fatal("loadDeployPushConfig() error = nil, want YAML parse error")
	}
}
