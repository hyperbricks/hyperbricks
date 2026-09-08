package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"net/http"
	"strings"
	"sync"
	"time"
)

type session struct {
	user    string
	expires time.Time
}
type authPlugin struct {
	mu       sync.Mutex
	sessions map[string]session
}

func reply(status int, body string) shared.HandledResponse {
	return shared.HandledResponse{Status: status, ContentType: "text/plain; charset=utf-8", Body: []byte(body), Headers: map[string]string{"Cache-Control": "no-store"}, NoCache: true}
}
func (p *authPlugin) Render(input interface{}, ctx context.Context) (any, []error) {
	var cfg struct {
		Data struct {
			Action string `mapstructure:"action"`
		} `mapstructure:"data"`
	}
	if err := shared.DecodeWithBasicHooks(input, &cfg); err != nil {
		return reply(500, "Invalid configuration"), err
	}
	r, _ := ctx.Value(shared.Request).(*http.Request)
	if r == nil {
		return reply(500, "Request required"), nil
	}
	if cfg.Data.Action == "authorize" {
		if r.Method != "GET" {
			return reply(405, "GET required"), nil
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		p.mu.Lock()
		s, ok := p.sessions[token]
		p.mu.Unlock()
		if !ok || time.Now().After(s.expires) {
			return reply(401, "Invalid session"), nil
		}
		if s.user != "member" {
			return reply(403, "Not permitted"), nil
		}
		return reply(200, "Allowed"), nil
	}
	// Same-origin browser actions: cross-origin forms cannot send this custom header.
	// No CORS permission is granted by this demo.
	if r.Method != "POST" {
		return reply(405, "POST required"), nil
	}
	if r.Header.Get("X-Demo-Action") != "true" {
		return reply(403, "Action header required"), nil
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return reply(403, "Same-origin action required"), nil
	}
	cookie := &http.Cookie{Name: "unpoly_guard_session", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil}
	if cfg.Data.Action == "logout" {
		if c, err := r.Cookie(cookie.Name); err == nil {
			p.mu.Lock()
			delete(p.sessions, c.Value)
			p.mu.Unlock()
		}
		cookie.MaxAge = -1
	} else if cfg.Data.Action == "login" {
		r.Body = http.MaxBytesReader(nil, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			return reply(400, "Invalid form"), nil
		}
		user := r.PostForm.Get("username")
		if (user != "member" && user != "blocked") || r.PostForm.Get("password") != "open-sesame" {
			return reply(401, "Invalid credentials"), nil
		}
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return reply(500, "Session creation failed"), nil
		}
		cookie.Value = hex.EncodeToString(random[:])
		cookie.MaxAge = 900
		p.mu.Lock()
		for token, s := range p.sessions {
			if time.Now().After(s.expires) {
				delete(p.sessions, token)
			}
		}
		if c, err := r.Cookie(cookie.Name); err == nil {
			delete(p.sessions, c.Value)
		}
		if len(p.sessions) >= 128 {
			p.mu.Unlock()
			return reply(503, "Demo session limit reached"), nil
		}
		p.sessions[cookie.Value] = session{user: user, expires: time.Now().Add(15 * time.Minute)}
		p.mu.Unlock()
	} else {
		return reply(400, "Unknown action"), nil
	}
	out := reply(200, "OK")
	out.Cookies = []string{cookie.String()}
	return out, nil
}
func Plugin() (shared.PluginRenderer, error) {
	return &authPlugin{sessions: make(map[string]session)}, nil
}
