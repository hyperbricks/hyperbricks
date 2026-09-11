package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

func TestServeContentWiresAPIAuthenticationSources(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	type upstreamRequest struct {
		path          string
		authorization string
	}
	received := make(chan upstreamRequest, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- upstreamRequest{
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	setTestRouteConfig("nested-api-render", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "nested-api-render",
		"10": map[string]interface{}{
			"@type":        component.APIConfigGetName(),
			"endpoint":     upstream.URL + "/api-render",
			"forwardtoken": "token",
			"method":       http.MethodGet,
			"inline":       `{{.Data.ok}}`,
		},
	})
	setTestRouteConfig("api-fragment-render", map[string]interface{}{
		"@type":        composite.ApiFragmentRenderConfigGetName(),
		"route":        "api-fragment-render",
		"endpoint":     upstream.URL + "/api-fragment-render",
		"forwardtoken": "token",
		"method":       http.MethodGet,
		"inline":       `{{.Data.ok}}`,
	})

	const signingSecret = "serve-content-signing-secret"
	signedConfig := func(componentType, route, endpoint string) map[string]interface{} {
		return map[string]interface{}{
			"@type":     componentType,
			"route":     route,
			"endpoint":  endpoint,
			"method":    http.MethodGet,
			"inline":    `{{.Data.ok}}`,
			"jwtsecret": signingSecret,
			"jwtclaims": map[string]interface{}{"role": "serve-content"},
		}
	}
	setTestRouteConfig("nested-api-render-signed", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "nested-api-render-signed",
		"10": signedConfig(
			component.APIConfigGetName(),
			"",
			upstream.URL+"/api-render-signed",
		),
	})
	setTestRouteConfig("api-fragment-render-signed", signedConfig(
		composite.ApiFragmentRenderConfigGetName(),
		"api-fragment-render-signed",
		upstream.URL+"/api-fragment-render-signed",
	))

	for _, test := range []struct {
		route        string
		upstreamPath string
		wantJWT      bool
	}{
		{route: "nested-api-render", upstreamPath: "/api-render"},
		{route: "api-fragment-render", upstreamPath: "/api-fragment-render"},
		{route: "nested-api-render-signed", upstreamPath: "/api-render-signed", wantJWT: true},
		{route: "api-fragment-render-signed", upstreamPath: "/api-fragment-render-signed", wantJWT: true},
	} {
		t.Run(test.route, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/"+test.route, nil)
			request.Header.Set("Authorization", "Bearer browser-token")
			request.AddCookie(&http.Cookie{Name: "token", Value: "context-token"})
			response := httptest.NewRecorder()

			ServeContent(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d with body %q", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "true") {
				t.Fatalf("expected rendered upstream response, got %q", response.Body.String())
			}

			select {
			case got := <-received:
				if got.path != test.upstreamPath {
					t.Fatalf("expected upstream path %q, got %q", test.upstreamPath, got.path)
				}
				if test.wantJWT {
					assertServeContentSignedAuthorization(t, got.authorization, signingSecret)
				} else if got.authorization != "Bearer context-token" {
					t.Fatalf("expected token cookie to reach upstream as Bearer auth, got %q", got.authorization)
				}
			case <-time.After(time.Second):
				t.Fatal("upstream did not receive the request")
			}
		})
	}
}

func assertServeContentSignedAuthorization(t *testing.T, authorization, secret string) {
	t.Helper()

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authorization, bearerPrefix) {
		t.Fatalf("expected generated Bearer token, got %q", authorization)
	}
	token, err := jwt.Parse(strings.TrimPrefix(authorization, bearerPrefix), func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %s", token.Method.Alg())
		}
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("generated JWT did not verify: %v", err)
	}
	if token == nil || !token.Valid {
		t.Fatal("generated JWT is invalid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["role"] != "serve-content" {
		t.Fatalf("expected decoded jwtclaims role, got %#v", token.Claims)
	}
}
