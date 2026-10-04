package component

import (
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
)

// ValidateRawConfig owns the strict type contract for this API component's
// authentication and response-status fields before ordinary weak decoding.
func (config APIConfig) ValidateRawConfig(raw map[string]interface{}) error {
	if err := apiutil.ValidateForwardTokenRaw(raw); err != nil {
		return err
	}
	return shared.ValidateResponseStatusFieldRaw(raw)
}

func (config APIConfig) authSettings() apiutil.AuthSettings {
	return apiutil.AuthSettings{
		ForwardToken: config.ForwardToken, Headers: config.Headers,
		Username: config.Username, Password: config.Password,
		JwtSecret: config.JwtSecret, JwtClaims: config.JwtClaims,
		HasBody: config.Body != "",
	}
}
