package shared

import (
	"fmt"
	"strings"
)

// CredentialsConfig is shared by module developer interfaces and deployment
// services. Each owning interface selects and resolves its own credentials.
type CredentialsConfig struct {
	User     string `mapstructure:"user" description:"Authentication username for the owning interface. Prefer an environment resolver rather than committing credentials." example:"{env: HB_DEVELOPER_USER}"`
	Password string `mapstructure:"password" description:"Authentication password for the owning interface. Treat as secret; never display its resolved value in editor help or diagnostics." example:"{env: HB_DEVELOPER_PASSWORD}"`
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
