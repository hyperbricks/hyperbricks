package main

import (
	"os"
	"path/filepath"
	"testing"
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
        host: 192.168.2.35
        user: deploy
        port: 22
        root: /opt/hyperbricks/deploy
        api: http://192.168.2.35:9090
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
	if cfg.Client.Target != "staging" || target.Host != "192.168.2.35" || target.API != "http://192.168.2.35:9090" {
		t.Fatalf("client config = %#v", cfg.Client)
	}
}

func TestLoadDeployYAMLRootRejectsUnsupportedDeployDSL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.hyperbricks")
	if err := os.WriteFile(path, []byte(`
deploy {
  hmac_secret = old-dsl
}
`), 0o644); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	if _, err := loadDeployYAMLRoot(path); err == nil {
		t.Fatal("loadDeployYAMLRoot() error = nil, want YAML parse error")
	}
}
