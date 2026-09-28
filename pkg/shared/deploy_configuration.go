package shared

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

// DeployConfig owns the separate deployment-service configuration, not the
// hyperbricks mapping in a module's package configuration.
type DeployConfig struct {
	Remote DeployRemoteConfig `mapstructure:"remote"`
	Local  DeployLocalConfig  `mapstructure:"local"`
	Client DeployClientConfig `mapstructure:"client"`
}

type DeployRemoteConfig struct {
	Bind        string                 `mapstructure:"bind"`
	Port        int                    `mapstructure:"port"`
	Root        string                 `mapstructure:"root"`
	PortStart   int                    `mapstructure:"port_start"`
	LogsEnabled bool                   `mapstructure:"logs_enabled"`
	Binary      string                 `mapstructure:"binary"`
	Credentials CredentialsConfig      `mapstructure:"credentials"`
	HMACSecret  string                 `mapstructure:"hmac_secret"`
	Auth        DeployRemoteAuthConfig `mapstructure:"auth"`
}

type DeployRemoteAuthConfig struct {
	EnvPrefix string `mapstructure:"env_prefix"`
}

type DeployLocalConfig struct {
	Bind        string            `mapstructure:"bind"`
	Port        int               `mapstructure:"port"`
	ModulesDir  string            `mapstructure:"modules_dir"`
	BuildRoot   string            `mapstructure:"build_root"`
	PortStart   int               `mapstructure:"port_start"`
	LogsEnabled bool              `mapstructure:"logs_enabled"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
}

type DeployClientConfig struct {
	Target  string                        `mapstructure:"target"`
	Targets map[string]DeployClientTarget `mapstructure:"targets"`
}

type DeployClientTarget struct {
	API         string            `mapstructure:"api"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
	HMACSecret  string            `mapstructure:"hmac_secret"`
	KeyID       string            `mapstructure:"key_id"`
}

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
