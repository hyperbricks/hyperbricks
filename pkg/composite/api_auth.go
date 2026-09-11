package composite

import (
	"fmt"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
)

// ValidateRawConfig checks only the security-sensitive API fields whose source
// types must be retained before the component's normal weak decoding.
func (config *ApiFragmentRenderConfig) ValidateRawConfig(raw map[string]interface{}) error {
	if err := apiutil.ValidateForwardTokenRaw(raw); err != nil {
		return err
	}
	if err := ValidateAPIResponseCookiesRaw(raw); err != nil {
		return err
	}
	entries, err := cloneRawAPIResponseCookieEntries(raw)
	if err != nil {
		return err
	}
	config.rawSetCookies = entries
	return nil
}

// responseCookieEntries preserves the exact scalar types accepted by
// ValidateRawConfig. The normal weak decoder may otherwise turn bool and int
// fields inside []interface{} cookie maps into strings. Direct Go configs have
// no captured raw entries and continue to use SetCookies.
func (config ApiFragmentRenderConfig) responseCookieEntries() []interface{} {
	if config.rawSetCookies != nil {
		return config.rawSetCookies
	}
	return config.SetCookies
}

func cloneRawAPIResponseCookieEntries(raw map[string]interface{}) ([]interface{}, error) {
	value, exists := raw["setcookies"]
	if !exists {
		return nil, nil
	}
	cloned := shared.CloneMapDeep(map[string]interface{}{"setcookies": value})["setcookies"]
	switch typed := cloned.(type) {
	case []interface{}:
		return typed, nil
	case []string:
		entries := make([]interface{}, len(typed))
		for index := range typed {
			entries[index] = typed[index]
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("setcookies must be a list of strings or cookie maps, got %T", value)
	}
}

func (config ApiFragmentRenderConfig) authSettings() apiutil.AuthSettings {
	return apiutil.AuthSettings{
		ForwardToken: config.ForwardToken, Headers: config.Headers,
		Username: config.Username, Password: config.Password,
		JwtSecret: config.JwtSecret, JwtClaims: config.JwtClaims,
		HasBody: config.Body != "",
	}
}
