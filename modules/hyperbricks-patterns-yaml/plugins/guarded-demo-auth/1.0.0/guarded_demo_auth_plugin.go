package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type guardedDemoAuthFields struct {
	Action            string `mapstructure:"action"`
	CookieName        string `mapstructure:"cookie_name"`
	SuccessRedirectTo string `mapstructure:"success_redirect"`
	ForbiddenTo       string `mapstructure:"forbidden_redirect"`
	LoginTo           string `mapstructure:"login_redirect"`
}

type guardedDemoAuthConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string                `mapstructure:"plugin"`
	Fields           guardedDemoAuthFields `mapstructure:"data"`
}

type guardedDemoAuthPlugin struct{}

var _ shared.PluginRenderer = (*guardedDemoAuthPlugin)(nil)

func (p *guardedDemoAuthPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {
	var errs []error

	var cfg guardedDemoAuthConfig
	if err := shared.DecodeWithBasicHooks(instance, &cfg); err != nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("failed to decode guarded demo auth config: %v", err),
		})
		return "<!-- failed to decode guarded demo auth config -->", errs
	}

	req, _ := ctx.Value(shared.Request).(*http.Request)
	if req == nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "guarded demo auth plugin requires request context",
		})
		return "<!-- guarded demo auth plugin requires request context -->", errs
	}

	switch strings.TrimSpace(cfg.Fields.Action) {
	case "login":
		return handleGuardedDemoLogin(req, cfg), errs
	case "logout":
		return handleGuardedDemoLogout(req, cfg), errs
	case "authorize":
		return handleGuardedDemoAuthorize(req, cfg), errs
	default:
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "unknown guarded demo auth action",
		})
		return "<!-- unknown guarded demo auth action -->", errs
	}
}

func handleGuardedDemoLogin(req *http.Request, cfg guardedDemoAuthConfig) shared.HandledResponse {
	if err := req.ParseForm(); err != nil {
		return handledHTMLResponse(http.StatusBadRequest, []byte(alertHTML("Could not parse login input.", "error")))
	}

	identifier := strings.TrimSpace(req.FormValue("identifier"))
	password := strings.TrimSpace(req.FormValue("password"))
	if identifier == "" || password == "" {
		return handledHTMLResponse(http.StatusBadRequest, []byte(alertHTML("Username and password are required.", "error")))
	}

	cookieName := cookieNameOrDefault(cfg.Fields.CookieName)
	switch {
	case identifier == "demo" && password == "open-sesame":
		return handledRedirectResponse(req, cfg.Fields.SuccessRedirectTo, []string{demoCookie(cookieName, "allow")})
	case identifier == "blocked" && password == "open-sesame":
		return handledRedirectResponse(req, cfg.Fields.ForbiddenTo, []string{demoCookie(cookieName, "forbidden")})
	default:
		return handledHTMLResponse(http.StatusUnauthorized, []byte(alertHTML("Unknown demo credentials. Try demo/open-sesame or blocked/open-sesame.", "error")))
	}
}

func handleGuardedDemoLogout(req *http.Request, cfg guardedDemoAuthConfig) shared.HandledResponse {
	return handledRedirectResponse(req, cfg.Fields.LoginTo, []string{clearDemoCookie(cookieNameOrDefault(cfg.Fields.CookieName))})
}

func handleGuardedDemoAuthorize(req *http.Request, cfg guardedDemoAuthConfig) shared.HandledResponse {
	token := strings.TrimSpace(resolveGuardedDemoToken(req, cookieNameOrDefault(cfg.Fields.CookieName)))
	if token == "" {
		return handledJSONResponse(http.StatusUnauthorized, map[string]any{
			"authenticated": false,
			"authorized":    false,
			"reason":        "missing_token",
		})
	}

	switch token {
	case "allow":
		return handledJSONResponse(http.StatusOK, map[string]any{
			"authenticated": true,
			"authorized":    true,
			"role":          "demo-user",
		})
	case "forbidden":
		return handledJSONResponse(http.StatusForbidden, map[string]any{
			"authenticated": true,
			"authorized":    false,
			"reason":        "forbidden_demo_user",
		})
	default:
		return handledJSONResponse(http.StatusUnauthorized, map[string]any{
			"authenticated": false,
			"authorized":    false,
			"reason":        "unknown_cookie_value",
		})
	}
}

func resolveGuardedDemoToken(req *http.Request, cookieName string) string {
	authHeader := strings.TrimSpace(req.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}

	if req == nil {
		return ""
	}
	cookie, err := req.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func cookieNameOrDefault(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "guarded_demo_session"
	}
	return value
}

func handledRedirectResponse(req *http.Request, location string, cookies []string) shared.HandledResponse {
	location = strings.TrimSpace(location)
	headers := map[string]string{}
	status := http.StatusSeeOther

	if isHTMXRequest(req) {
		headers["HX-Redirect"] = location
		status = http.StatusOK
	} else {
		headers["Location"] = location
	}

	return shared.HandledResponse{
		Status:      status,
		ContentType: "text/html; charset=utf-8",
		Headers:     headers,
		Cookies:     cookies,
		Body:        []byte(""),
		NoCache:     true,
	}
}

func handledHTMLResponse(status int, body []byte) shared.HandledResponse {
	return shared.HandledResponse{
		Status:      status,
		ContentType: "text/html; charset=utf-8",
		Body:        body,
		NoCache:     true,
	}
}

func handledJSONResponse(status int, payload map[string]any) shared.HandledResponse {
	body, _ := json.Marshal(payload)
	return shared.HandledResponse{
		Status:      status,
		ContentType: "application/json; charset=utf-8",
		Body:        body,
		NoCache:     true,
	}
}

func alertHTML(message string, level string) string {
	return fmt.Sprintf("<div class=\"guarded-alert\" data-level=\"%s\">%s</div>", htmlEscape(level), htmlEscape(message))
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func demoCookie(name string, value string) string {
	return fmt.Sprintf("%s=%s; Path=/; HttpOnly; SameSite=Lax", name, value)
}

func clearDemoCookie(name string) string {
	return fmt.Sprintf("%s=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0", name)
}

func isHTMXRequest(req *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(req.Header.Get("HX-Request")), "true")
}

func Plugin() (shared.PluginRenderer, error) {
	return &guardedDemoAuthPlugin{}, nil
}
