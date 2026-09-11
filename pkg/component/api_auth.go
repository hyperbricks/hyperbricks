package component

import "github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"

// ValidateRawConfig owns the strict type contract for this API component's
// forwardtoken field. Other YAML fields retain their normal decoding behavior.
func (config APIConfig) ValidateRawConfig(raw map[string]interface{}) error {
	return apiutil.ValidateForwardTokenRaw(raw)
}

func (config APIConfig) authSettings() apiutil.AuthSettings {
	return apiutil.AuthSettings{
		ForwardToken: config.ForwardToken, Headers: config.Headers,
		Username: config.Username, Password: config.Password,
		JwtSecret: config.JwtSecret, JwtClaims: config.JwtClaims,
		HasBody: config.Body != "",
	}
}
