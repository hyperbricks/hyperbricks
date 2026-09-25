package shared

import (
	"strings"
	"testing"
)

func TestDecodeDeployConfigAcceptsIndependentRoles(t *testing.T) {
	local, err := DecodeDeployConfig(map[string]interface{}{
		"local": map[string]interface{}{
			"port": 9191,
			"credentials": map[string]interface{}{
				"user": "local-user", "password": "local-password",
			},
		},
	})
	if err != nil {
		t.Fatalf("decode local-only config: %v", err)
	}
	if local.Local.Port != 9191 || local.Remote.Port != 9090 {
		t.Fatalf("local-only decode = %#v", local)
	}

	remote, err := DecodeDeployConfig(map[string]interface{}{
		"remote": map[string]interface{}{
			"port": 9292, "hmac_secret": "remote-secret",
			"credentials": map[string]interface{}{
				"user": "remote-user", "password": "remote-password",
			},
		},
	})
	if err != nil {
		t.Fatalf("decode remote-only config: %v", err)
	}
	if remote.Remote.Port != 9292 || remote.Remote.HMACSecret != "remote-secret" {
		t.Fatalf("remote-only decode = %#v", remote)
	}
}

func TestDecodeDeployConfigRejectsLegacyOwnership(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  string
	}{
		{name: "top-level HMAC", input: map[string]interface{}{"hmac_secret": "old"}, want: "deploy.hmac_secret"},
		{name: "top-level credentials", input: map[string]interface{}{"credentials": map[string]interface{}{}}, want: "deploy.credentials"},
		{name: "remote API bind", input: map[string]interface{}{"remote": map[string]interface{}{"api_bind": "127.0.0.1"}}, want: "deploy.remote.bind"},
		{name: "pass alias", input: map[string]interface{}{"local": map[string]interface{}{"credentials": map[string]interface{}{"user": "x", "pass": "y"}}}, want: "credentials.password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDeployConfig(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestDecodeDeployConfigRejectsUnknownAndInvalidValues(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input map[string]interface{}
	}{
		{name: "unknown", input: map[string]interface{}{"remote": map[string]interface{}{"mystery": true}}},
		{name: "invalid port", input: map[string]interface{}{"remote": map[string]interface{}{"port": map[string]interface{}{"bad": true}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeDeployConfig(tt.input); err == nil {
				t.Fatal("invalid deployment configuration was accepted")
			}
		})
	}
}
