package main

import (
	"net/http"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func dashboardHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !developerDashboardEnabled() {
			http.NotFound(w, r)
			return
		}
		if requireDashboardAuth(w, r) {
			next.ServeHTTP(w, r)
		}
	})
}

// An explicitly enabled development dashboard may run without an account.
// Keep partial accounts locked. Spaces uses its own optional-login policy.
// Diagnostics also use this policy, but still require an account when the
// dashboard is disabled.
func requireDashboardAuth(w http.ResponseWriter, r *http.Request) bool {
	if developerDashboardEnabled() && getHyperBricksConfiguration().Development.Dashboard.Credentials.Empty() {
		return true
	}
	return requireDeveloperInterfaceAuth(w, r)
}

func requireDeveloperInterfaceAuth(w http.ResponseWriter, r *http.Request) bool {
	credentials := getHyperBricksConfiguration().Development.Dashboard.Credentials
	message := shared.DeveloperInterfaceUnavailableMessage
	if credentials.Empty() {
		message += ". Dashboard is disabled. To view diagnostics, set hyperbricks.development.dashboard.enabled: true in package.hyperbricks.yaml, or configure both development.dashboard.credentials.user and password. Restart HyperBricks after changing settings. An enabled Dashboard without credentials is accessible without login."
	} else if !credentials.Complete() {
		message += ". The developer account is incomplete: set both development.dashboard.credentials.user and password, then restart HyperBricks."
	}
	return shared.RequireBasicAuth(
		w,
		r,
		credentials,
		shared.DeveloperInterfaceRealm,
		message,
	)
}

func contextualEditingRequested(r *http.Request) bool {
	if r == nil || r.URL == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	if !spacesEditorAvailable() {
		return false
	}
	values := r.URL.Query()["edit"]
	return len(values) == 1 && values[0] == "true"
}
