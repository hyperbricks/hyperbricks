package shared

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

// DefaultDeployConfig returns operational, non-secret defaults. Authentication
// values intentionally remain empty so a deployment service starts locked
// until its role-specific credentials are resolved.
func DefaultDeployConfig() DeployConfig {
	return DeployConfig{
		Local: DeployLocalConfig{
			Bind:        "127.0.0.1",
			Port:        9091,
			ModulesDir:  "modules",
			BuildRoot:   "deploy",
			PortStart:   8080,
			LogsEnabled: true,
		},
		Remote: DeployRemoteConfig{
			Bind:        "127.0.0.1",
			Port:        9090,
			Root:        "deploy",
			PortStart:   8080,
			LogsEnabled: true,
			Auth: DeployRemoteAuthConfig{
				EnvPrefix: "HB_DEPLOY_SECRET_",
			},
		},
	}
}

// DecodeDeployConfig decodes the value below the top-level deploy key. Unknown
// fields are rejected so obsolete ownership models cannot silently remain in
// active deployment configuration.
func DecodeDeployConfig(input interface{}) (DeployConfig, error) {
	cfg := DefaultDeployConfig()
	if input == nil {
		return cfg, fmt.Errorf("missing deploy configuration")
	}
	if deploy, ok := input.(map[string]interface{}); ok {
		if err := validateObsoleteDeployFields(deploy); err != nil {
			return cfg, err
		}
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &cfg,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		ErrorUnused:      true,
		DecodeHook:       mapstructure.TextUnmarshallerHookFunc(),
	})
	if err != nil {
		return cfg, err
	}
	if err := decoder.Decode(input); err != nil {
		return cfg, fmt.Errorf("invalid deploy configuration: %w", err)
	}
	return cfg, nil
}

func validateObsoleteDeployFields(deploy map[string]interface{}) error {
	if _, ok := deploy["hmac_secret"]; ok {
		return fmt.Errorf("obsolete deploy.hmac_secret; use deploy.client.targets.<name>.hmac_secret or deploy.remote.hmac_secret")
	}
	if _, ok := deploy["credentials"]; ok {
		return fmt.Errorf("obsolete deploy.credentials; configure credentials under deploy.local, deploy.remote, or a client target")
	}
	if remote, ok := deploy["remote"].(map[string]interface{}); ok {
		if _, exists := remote["api_enabled"]; exists {
			return fmt.Errorf("obsolete deploy.remote.api_enabled; remove it because the deploy remote command selects the service")
		}
		if _, exists := remote["api_bind"]; exists {
			return fmt.Errorf("obsolete deploy.remote.api_bind; use deploy.remote.bind")
		}
		if _, exists := remote["api_port"]; exists {
			return fmt.Errorf("obsolete deploy.remote.api_port; use deploy.remote.port")
		}
		if err := validateCredentialPasswordField(remote["credentials"], "deploy.remote.credentials"); err != nil {
			return err
		}
	}
	if local, ok := deploy["local"].(map[string]interface{}); ok {
		if err := validateCredentialPasswordField(local["credentials"], "deploy.local.credentials"); err != nil {
			return err
		}
	}
	if client, ok := deploy["client"].(map[string]interface{}); ok {
		if targets, ok := client["targets"].(map[string]interface{}); ok {
			for name, rawTarget := range targets {
				target, ok := rawTarget.(map[string]interface{})
				if !ok {
					continue
				}
				if err := validateCredentialPasswordField(target["credentials"], "deploy.client.targets."+name+".credentials"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateCredentialPasswordField(raw interface{}, path string) error {
	credentials, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	if _, exists := credentials["pass"]; exists {
		return fmt.Errorf("obsolete %s.pass; use %s.password", path, path)
	}
	return nil
}

func (credentials CredentialsConfig) Complete() bool {
	return strings.TrimSpace(credentials.User) != "" && credentials.Password != ""
}

func (credentials CredentialsConfig) Empty() bool {
	return strings.TrimSpace(credentials.User) == "" && credentials.Password == ""
}

func (credentials CredentialsConfig) Validate(label string) error {
	if credentials.Complete() {
		return nil
	}
	if credentials.Empty() {
		return fmt.Errorf("%s credentials are not configured", label)
	}
	return fmt.Errorf("%s credentials require both user and password", label)
}
