package composite

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestAPIFragmentRenderExplicitAuthentication(t *testing.T) {
	shared.Init_configuration()
	conf := shared.GetHyperBricksConfiguration()
	previous := conf.Mode
	conf.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { conf.Mode = previous })
	received := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)
	tests := []struct {
		name      string
		forward   string
		cookies   string
		header    string
		username  string
		password  string
		jwtSecret string
		want      string
		wantJWT   bool
		wantErr   bool
	}{
		{name: "omission disables legacy token bridge", cookies: "token=legacy; session=selected"},
		{name: "configured cookie selected", forward: "session", cookies: "token=legacy; session=selected", want: "Bearer selected"},
		{name: "different cookie selected", forward: "other", cookies: "session=selected; other=second", want: "Bearer second"},
		{name: "selected cookie absent", forward: "session", cookies: "token=legacy"},
		{name: "selected cookie empty", forward: "session", cookies: "session="},
		{name: "duplicate cookie rejected", forward: "session", cookies: "session=first; session=second", wantErr: true},
		{name: "empty duplicate rejected", forward: "session", cookies: "session=; session=second", wantErr: true},
		{name: "explicit header stays independent", cookies: "token=legacy", header: "Bearer service", want: "Bearer service"},
		{name: "complete Basic", cookies: "token=legacy", username: "user", password: "secret", want: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:secret"))},
		{name: "JWT", cookies: "token=legacy", jwtSecret: "test-signing-secret", wantJWT: true},
		{name: "header conflicts with cookie", forward: "session", header: "Bearer service", wantErr: true},
		{name: "JWT conflicts with cookie", forward: "session", jwtSecret: "test-signing-secret", wantErr: true},
		{name: "Basic conflicts with header", header: "Bearer service", username: "user", password: "secret", wantErr: true},
		{name: "Basic conflicts with JWT", jwtSecret: "test-signing-secret", username: "user", password: "secret", wantErr: true},
		{name: "incomplete Basic username", username: "user", wantErr: true},
		{name: "incomplete Basic password", password: "secret", wantErr: true},
		{name: "invalid cookie name", forward: " session", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "https://example.test/page", nil)
			req.Header.Set("Authorization", "Bearer browser-header-must-not-forward")
			req.Header.Set("Cookie", tt.cookies)
			ctx := context.WithValue(req.Context(), shared.Request, req)
			ctx = context.WithValue(ctx, shared.ResponseWriter, httptest.NewRecorder())
			settings := APIConfig{
				Endpoint: upstream.URL, Method: "GET", Inline: `{{.Data.ok}}`,
				ForwardToken: tt.forward, Username: tt.username, Password: tt.password, JwtSecret: tt.jwtSecret,
			}
			if tt.jwtSecret != "" {
				settings.JwtClaims = map[string]string{"role": "reader"}
			}
			if tt.header != "" {
				settings.Headers = map[string]string{"Authorization": tt.header}
			}
			cfg := ApiFragmentRenderConfig{APIConfig: settings, Route: "test-api"}
			output, errs := (&ApiFragmentRenderer{}).Render(cfg, ctx)
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("expected error; output=%q", output)
				}
				select {
				case got := <-received:
					t.Fatalf("rejected configuration reached upstream: %v", got)
				default:
				}
				return
			}
			if len(errs) > 0 || output != "true" {
				t.Fatalf("render failed: output=%q errors=%v", output, errs)
			}
			var got http.Header
			select {
			case got = <-received:
			default:
				t.Fatal("expected upstream call")
			}
			if tt.wantJWT {
				assertAPIFragmentRenderSignedAuthorization(t, got.Get("Authorization"), tt.jwtSecret, settings.JwtClaims)
				return
			}
			if got.Get("Authorization") != tt.want {
				t.Fatalf("expected %q got %q", tt.want, got.Get("Authorization"))
			}
			if tt.want == "" {
				if _, ok := got["Authorization"]; ok {
					t.Fatal("absent credential must not create empty Authorization header")
				}
			}
			if got.Get("Cookie") != "" {
				t.Fatal("browser Cookie header was forwarded")
			}
		})
	}
}

func assertAPIFragmentRenderSignedAuthorization(t *testing.T, authorization, secret string, expectedClaims map[string]string) {
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
	if !token.Valid {
		t.Fatal("generated JWT is invalid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("expected map claims, got %T", token.Claims)
	}
	for key, want := range expectedClaims {
		if got := claims[key]; got != want {
			t.Fatalf("expected JWT claim %s=%q, got %#v", key, want, got)
		}
	}
}
