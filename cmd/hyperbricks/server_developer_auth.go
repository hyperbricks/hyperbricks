package main

import (
	"net/http"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func developerInterfaceHandler(next http.Handler) http.Handler {
	credentials := getHyperBricksConfiguration().Development.Dashboard.Credentials
	return shared.BasicAuthWithUnavailable(
		next,
		credentials,
		shared.DeveloperInterfaceRealm,
		shared.DeveloperInterfaceUnavailableMessage,
	)
}

func requireDeveloperInterfaceAuth(w http.ResponseWriter, r *http.Request) bool {
	credentials := getHyperBricksConfiguration().Development.Dashboard.Credentials
	return shared.RequireBasicAuth(
		w,
		r,
		credentials,
		shared.DeveloperInterfaceRealm,
		shared.DeveloperInterfaceUnavailableMessage,
	)
}

func contextualEditingRequested(r *http.Request) bool {
	if r == nil || r.URL == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	cfg := getHyperBricksConfiguration()
	if cfg.Mode != shared.DEVELOPMENT_MODE || shared.GetRuntimeOptions().Production ||
		!cfg.Development.FrontendEditing.Enabled || !cfg.Development.FrontendEditing.Spaces.Enabled || cfg.ValidateFrontendEditing() != nil {
		return false
	}
	values := r.URL.Query()["edit"]
	return len(values) == 1 && values[0] == "true"
}
