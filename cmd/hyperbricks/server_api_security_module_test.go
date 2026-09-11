package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

type apiSecurityObservedRequest struct {
	Path          string
	Authorization string
}

type apiSecurityRequestLog struct {
	mu       sync.Mutex
	requests []apiSecurityObservedRequest
}

type apiSecurityBarrier struct {
	arrivals chan string
	release  chan struct{}
	once     sync.Once
}

func newAPISecurityBarrier() *apiSecurityBarrier {
	return &apiSecurityBarrier{
		arrivals: make(chan string, 2),
		release:  make(chan struct{}),
	}
}

func (b *apiSecurityBarrier) wait(r *http.Request) bool {
	select {
	case b.arrivals <- r.URL.Path:
	case <-r.Context().Done():
		return false
	case <-b.release:
		return false
	}
	select {
	case <-b.release:
		return true
	case <-r.Context().Done():
		return false
	}
}

func (b *apiSecurityBarrier) releaseAll() {
	b.once.Do(func() { close(b.release) })
}

type apiSecurityCancellationProbe struct {
	started   chan string
	cancelled chan string
	release   chan struct{}
	once      sync.Once
}

func newAPISecurityCancellationProbe() *apiSecurityCancellationProbe {
	return &apiSecurityCancellationProbe{
		started:   make(chan string, 2),
		cancelled: make(chan string, 2),
		release:   make(chan struct{}),
	}
}

func (p *apiSecurityCancellationProbe) wait(r *http.Request) {
	select {
	case p.started <- r.URL.Path:
	case <-r.Context().Done():
		return
	case <-p.release:
		return
	}
	select {
	case <-r.Context().Done():
		select {
		case p.cancelled <- r.URL.Path:
		case <-p.release:
		}
	case <-p.release:
	}
}

func (p *apiSecurityCancellationProbe) releaseAll() {
	p.once.Do(func() { close(p.release) })
}

type apiSecurityHTTPResult struct {
	response *http.Response
	body     string
	err      error
}

func (l *apiSecurityRequestLog) record(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, apiSecurityObservedRequest{
		Path:          r.URL.Path,
		Authorization: r.Header.Get("Authorization"),
	})
}

func (l *apiSecurityRequestLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = nil
}

func (l *apiSecurityRequestLog) matching(path string) []apiSecurityObservedRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	var matches []apiSecurityObservedRequest
	for _, request := range l.requests {
		if request.Path == path {
			matches = append(matches, request)
		}
	}
	return matches
}

func TestAPISecurityModuleCredentialAndCookieBoundaries(t *testing.T) {
	primaryLog := &apiSecurityRequestLog{}
	redirectLog := &apiSecurityRequestLog{}
	barrier := newAPISecurityBarrier()
	cancellationProbe := newAPISecurityCancellationProbe()

	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectLog.record(r)
		writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
			"source":    "redirect-target",
			"auth_kind": apiSecurityAuthKind(r.Header.Get("Authorization")),
			"message":   "redirect target reached",
		})
	}))
	t.Cleanup(redirectTarget.Close)

	primary := httptest.NewServer(apiSecurityUpstreamHandler(primaryLog, redirectTarget.URL, barrier, cancellationProbe))
	t.Cleanup(primary.Close)

	loadAPISecurityTestModule(t, primary.URL)
	application := httptest.NewServer(buildRuntimeHandler(nil))
	t.Cleanup(application.Close)
	t.Cleanup(cancellationProbe.releaseAll)
	t.Cleanup(barrier.releaseAll)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create browser cookie jar: %v", err)
	}
	browser := &http.Client{Jar: jar}

	t.Run("structured login cookies feed only their named API components", func(t *testing.T) {
		primaryLog.reset()
		response, body := apiSecurityRequest(t, browser, http.MethodPost, application.URL+"/actions/login", nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("login status = %d, body = %q", response.StatusCode, body)
		}
		requireNoAPISecurityRenderErrors(t, response)
		cookies := response.Cookies()
		if len(cookies) != 2 {
			t.Fatalf("login Set-Cookie count = %d, want 2: %v", len(cookies), response.Header.Values("Set-Cookie"))
		}
		byName := map[string]*http.Cookie{}
		for _, cookie := range cookies {
			byName[cookie.Name] = cookie
		}
		assertAPISecuritySessionCookie(t, byName["user_session"], "fixture-user-token", http.SameSiteLaxMode)
		assertAPISecuritySessionCookie(t, byName["admin_session"], "fixture-admin-token", http.SameSiteStrictMode)
		for _, cookie := range cookies {
			if cookie.MaxAge != 0 {
				t.Fatalf("login cookie %q MaxAge = %d, want omitted/session value 0", cookie.Name, cookie.MaxAge)
			}
		}
		requireAPISecurityCall(t, primaryLog, "/issue/valid", "")

		primaryLog.reset()
		response, body = apiSecurityRequest(t, browser, http.MethodGet, application.URL+"/dashboard", nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("dashboard status = %d, body = %q", response.StatusCode, body)
		}
		requireNoAPISecurityRenderErrors(t, response)
		for _, marker := range []string{"private/user", "private/admin", "public", "service"} {
			if !strings.Contains(body, `data-source="`+marker+`"`) {
				t.Fatalf("dashboard body is missing %q result: %s", marker, body)
			}
		}
		requireAPISecurityCall(t, primaryLog, "/private/user", "Bearer fixture-user-token")
		requireAPISecurityCall(t, primaryLog, "/private/admin", "Bearer fixture-admin-token")
		requireAPISecurityCall(t, primaryLog, "/public", "")
		requireAPISecurityCall(t, primaryLog, "/service", "Bearer fixture-service-token")
	})

	t.Run("API fragment uses only its configured cookie carrier", func(t *testing.T) {
		primaryLog.reset()
		request, err := http.NewRequest(http.MethodGet, application.URL+"/actions/private", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer browser-header-must-not-forward")
		response, body := apiSecurityDo(t, browser, request)
		if response.StatusCode != http.StatusOK || !strings.Contains(body, `data-source="private/fragment"`) {
			t.Fatalf("private fragment status = %d, body = %q", response.StatusCode, body)
		}
		requireNoAPISecurityRenderErrors(t, response)
		requireAPISecurityCall(t, primaryLog, "/private/fragment", "Bearer fixture-user-token")
	})

	t.Run("omitted forwarding and a wrong cookie send no browser credential", func(t *testing.T) {
		primaryLog.reset()
		request, err := http.NewRequest(http.MethodGet, application.URL+"/actions/public", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer browser-header-must-not-forward")
		response, _ := apiSecurityDo(t, browser, request)
		requireNoAPISecurityRenderErrors(t, response)
		requireAPISecurityCall(t, primaryLog, "/public/fragment", "")

		primaryLog.reset()
		request, err = http.NewRequest(http.MethodGet, application.URL+"/wrong-cookie", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.AddCookie(&http.Cookie{Name: "other_session", Value: "wrong-token"})
		request.Header.Set("Authorization", "Bearer browser-header-must-not-forward")
		response, _ = apiSecurityDo(t, http.DefaultClient, request)
		requireNoAPISecurityRenderErrors(t, response)
		requireAPISecurityCall(t, primaryLog, "/private/wrong-cookie", "")
	})

	t.Run("invalid issuance is atomic", func(t *testing.T) {
		for _, test := range []struct {
			name         string
			route        string
			upstreamPath string
		}{
			{name: "missing", route: "/actions/login-missing-token", upstreamPath: "/issue/missing"},
			{name: "empty", route: "/actions/login-empty-token", upstreamPath: "/issue/empty"},
			{name: "invalid", route: "/actions/login-invalid-token", upstreamPath: "/issue/invalid"},
		} {
			t.Run(test.name, func(t *testing.T) {
				primaryLog.reset()
				response, _ := apiSecurityRequest(t, http.DefaultClient, http.MethodPost, application.URL+test.route, nil)
				if got := response.Header.Values("Set-Cookie"); len(got) != 0 {
					t.Fatalf("invalid issuance Set-Cookie = %v, want none", got)
				}
				requireAPISecurityRenderErrors(t, response)
				requireAPISecurityCall(t, primaryLog, test.upstreamPath, "")
			})
		}
	})

	t.Run("decode and main template errors suppress otherwise valid cookies", func(t *testing.T) {
		for _, test := range []struct {
			name         string
			route        string
			upstreamPath string
		}{
			{name: "trailing vendor JSON", route: "/actions/login-invalid-json", upstreamPath: "/issue/trailing-json"},
			{name: "main template execution", route: "/actions/login-template-error", upstreamPath: "/issue/valid"},
		} {
			t.Run(test.name, func(t *testing.T) {
				primaryLog.reset()
				response, _ := apiSecurityRequest(t, http.DefaultClient, http.MethodPost, application.URL+test.route, nil)
				if got := response.Header.Values("Set-Cookie"); len(got) != 0 {
					t.Fatalf("failed response processing emitted Set-Cookie: %v", got)
				}
				requireAPISecurityRenderErrors(t, response)
				requireAPISecurityCall(t, primaryLog, test.upstreamPath, "")
			})
		}
	})

	t.Run("duplicate carrier cookies reject both API component types before calling upstream", func(t *testing.T) {
		for _, route := range []struct {
			path         string
			upstreamPath string
		}{
			{path: "/duplicate-cookie", upstreamPath: "/private/duplicate"},
			{path: "/actions/private", upstreamPath: "/private/fragment"},
		} {
			primaryLog.reset()
			request, err := http.NewRequest(http.MethodGet, application.URL+route.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Cookie", "user_session=first; user_session=second")
			response, _ := apiSecurityDo(t, http.DefaultClient, request)
			requireAPISecurityRenderErrors(t, response)
			requireNoAPISecurityCall(t, primaryLog, route.upstreamPath)
		}
	})

	t.Run("conflicting component authentication rejects before calling upstream", func(t *testing.T) {
		for _, route := range []struct {
			path         string
			upstreamPath string
		}{
			{path: "/conflicts/header", upstreamPath: "/conflicts/header"},
			{path: "/conflicts/basic", upstreamPath: "/conflicts/basic"},
		} {
			primaryLog.reset()
			request, err := http.NewRequest(http.MethodGet, application.URL+route.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.AddCookie(&http.Cookie{Name: "user_session", Value: "fixture-user-token"})
			response, _ := apiSecurityDo(t, http.DefaultClient, request)
			requireAPISecurityRenderErrors(t, response)
			requireNoAPISecurityCall(t, primaryLog, route.upstreamPath)
		}
	})

	t.Run("boolean and numeric forwardtoken values stay component-local errors", func(t *testing.T) {
		primaryLog.reset()
		response, body := apiSecurityRequest(t, http.DefaultClient, http.MethodGet, application.URL+"/invalid-forwardtoken-types", nil)
		requireAPISecurityRenderErrors(t, response)
		if !strings.Contains(body, `id="invalid-before"`) || !strings.Contains(body, `id="invalid-after"`) {
			t.Fatalf("invalid child configuration removed its valid siblings: %s", body)
		}
		requireNoAPISecurityCall(t, primaryLog, "/invalid/bool")
		requireNoAPISecurityCall(t, primaryLog, "/invalid/number")

		primaryLog.reset()
		response, _ = apiSecurityRequest(t, http.DefaultClient, http.MethodGet, application.URL+"/invalid-forwardtoken-fragment", nil)
		requireAPISecurityRenderErrors(t, response)
		requireNoAPISecurityCall(t, primaryLog, "/invalid/fragment-number")
	})

	t.Run("cross-port redirects never reach the second origin", func(t *testing.T) {
		for _, route := range []struct {
			path         string
			upstreamPath string
		}{
			{path: "/redirects/nested", upstreamPath: "/redirect/nested"},
			{path: "/redirects/fragment", upstreamPath: "/redirect/fragment"},
		} {
			primaryLog.reset()
			redirectLog.reset()
			request, err := http.NewRequest(http.MethodGet, application.URL+route.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.AddCookie(&http.Cookie{Name: "user_session", Value: "fixture-user-token"})
			response, _ := apiSecurityDo(t, http.DefaultClient, request)
			requireAPISecurityRenderErrors(t, response)
			requireAPISecurityCall(t, primaryLog, route.upstreamPath, "Bearer fixture-user-token")
			if calls := redirectLog.matching("/redirect-target/" + strings.TrimPrefix(route.upstreamPath, "/redirect/")); len(calls) != 0 {
				t.Fatalf("cross-port redirect target received %d request(s): %#v", len(calls), calls)
			}
		}
	})

	t.Run("composed API calls overlap at the upstream barrier", func(t *testing.T) {
		primaryLog.reset()
		request, err := http.NewRequest(http.MethodGet, application.URL+"/concurrent-fanout", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.AddCookie(&http.Cookie{Name: "user_session", Value: "fixture-user-token"})
		request.AddCookie(&http.Cookie{Name: "admin_session", Value: "fixture-admin-token"})

		result := make(chan apiSecurityHTTPResult, 1)
		go func() {
			result <- apiSecurityDoResult(http.DefaultClient, request)
		}()

		arrived := make(map[string]int)
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		for len(arrived) < 2 {
			select {
			case path := <-barrier.arrivals:
				arrived[path]++
			case <-deadline.C:
				barrier.releaseAll()
				t.Fatalf("composed API calls did not overlap at the barrier: arrivals=%v", arrived)
			}
		}
		if arrived["/barrier/user"] != 1 || arrived["/barrier/admin"] != 1 {
			barrier.releaseAll()
			t.Fatalf("unexpected barrier arrivals: %v", arrived)
		}

		barrier.releaseAll()
		select {
		case completed := <-result:
			if completed.err != nil {
				t.Fatalf("concurrent fanout request failed: %v", completed.err)
			}
			if completed.response.StatusCode != http.StatusOK {
				t.Fatalf("concurrent fanout status = %d, body = %q", completed.response.StatusCode, completed.body)
			}
			requireNoAPISecurityRenderErrors(t, completed.response)
			for _, source := range []string{"barrier/user", "barrier/admin"} {
				if !strings.Contains(completed.body, `data-source="`+source+`"`) {
					t.Fatalf("concurrent fanout body is missing %q: %s", source, completed.body)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent fanout did not complete after releasing the upstream barrier")
		}
		requireAPISecurityCall(t, primaryLog, "/barrier/user", "Bearer fixture-user-token")
		requireAPISecurityCall(t, primaryLog, "/barrier/admin", "Bearer fixture-admin-token")
	})

	t.Run("incoming cancellation reaches both API component types", func(t *testing.T) {
		for _, test := range []struct {
			name         string
			route        string
			upstreamPath string
		}{
			{name: "nested API render", route: "/cancel/nested", upstreamPath: "/cancel/nested"},
			{name: "API fragment render", route: "/cancel/fragment", upstreamPath: "/cancel/fragment"},
		} {
			t.Run(test.name, func(t *testing.T) {
				primaryLog.reset()
				ctx, cancel := context.WithCancel(context.Background())
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, application.URL+test.route, nil)
				if err != nil {
					cancel()
					t.Fatal(err)
				}
				result := make(chan apiSecurityHTTPResult, 1)
				go func() {
					result <- apiSecurityDoResult(http.DefaultClient, request)
				}()

				select {
				case path := <-cancellationProbe.started:
					if path != test.upstreamPath {
						cancel()
						t.Fatalf("cancellation probe started %q, want %q", path, test.upstreamPath)
					}
				case <-time.After(2 * time.Second):
					cancel()
					t.Fatalf("upstream %q did not start", test.upstreamPath)
				}

				cancelledAt := time.Now()
				cancel()
				select {
				case path := <-cancellationProbe.cancelled:
					if path != test.upstreamPath {
						t.Fatalf("cancellation probe observed %q, want %q", path, test.upstreamPath)
					}
					if elapsed := time.Since(cancelledAt); elapsed > 2*time.Second {
						t.Fatalf("upstream cancellation took %s", elapsed)
					}
				case <-time.After(2 * time.Second):
					t.Fatalf("incoming cancellation did not reach upstream %q promptly", test.upstreamPath)
				}

				select {
				case completed := <-result:
					if !errors.Is(completed.err, context.Canceled) {
						t.Fatalf("browser request error = %v, want context cancellation", completed.err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("browser request did not return promptly after cancellation")
				}
				requireAPISecurityCall(t, primaryLog, test.upstreamPath, "")
			})
		}
	})

	t.Run("real renderer debug output omits secrets", func(t *testing.T) {
		primaryLog.reset()
		output, err := captureAPISecurityStdout(func() error {
			for _, route := range []string{"/debug/nested", "/debug/fragment"} {
				request, requestErr := http.NewRequest(http.MethodGet, application.URL+route, nil)
				if requestErr != nil {
					return requestErr
				}
				request.AddCookie(&http.Cookie{Name: "user_session", Value: "renderer-debug-secret-token"})
				completed := apiSecurityDoResult(http.DefaultClient, request)
				if completed.err != nil {
					return completed.err
				}
				if completed.response.StatusCode != http.StatusOK {
					return fmt.Errorf("GET %s returned %d: %s", route, completed.response.StatusCode, completed.body)
				}
				if value := completed.response.Header.Get(renderErrorCountHeader); value != "" && value != "0" {
					return fmt.Errorf("GET %s produced %s render errors: %s", route, value, completed.body)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("capture renderer debug output: %v", err)
		}
		if strings.Count(output, "API request:") != 2 || strings.Count(output, "API response:") != 2 {
			t.Fatalf("debug output did not exercise both real renderers: %q", output)
		}
		for _, useful := range []string{"Method:GET", "Scheme:http", "Host:", "Authorization", "X-Api-Key", "StatusCode:200"} {
			if !strings.Contains(output, useful) {
				t.Fatalf("debug output is missing safe metadata %q: %q", useful, output)
			}
		}
		for _, secret := range []string{
			"renderer-debug-secret-token",
			"renderer-api-key-secret",
			"path-secret",
			"configured-query-secret",
		} {
			if strings.Contains(output, secret) {
				t.Fatalf("real renderer debug output exposed %q: %q", secret, output)
			}
		}
		requireAPISecurityCall(t, primaryLog, "/debug/nested/path-secret", "Bearer renderer-debug-secret-token")
		requireAPISecurityCall(t, primaryLog, "/debug/fragment/path-secret", "Bearer renderer-debug-secret-token")
	})

	t.Run("204 logout explicitly deletes both carrier cookies", func(t *testing.T) {
		primaryLog.reset()
		response, body := apiSecurityRequest(t, browser, http.MethodPost, application.URL+"/actions/logout", nil)
		if response.StatusCode != http.StatusOK || !strings.Contains(body, "removed") {
			t.Fatalf("logout status = %d, body = %q", response.StatusCode, body)
		}
		requireNoAPISecurityRenderErrors(t, response)
		cookies := response.Cookies()
		if len(cookies) != 2 {
			t.Fatalf("logout Set-Cookie count = %d, want 2: %v", len(cookies), response.Header.Values("Set-Cookie"))
		}
		for _, cookie := range cookies {
			if cookie.Value != "" || cookie.MaxAge >= 0 {
				t.Fatalf("logout cookie = %#v, want empty value with explicit Max-Age=0 deletion", cookie)
			}
		}
		requireAPISecurityCall(t, primaryLog, "/logout", "")

		applicationURL, err := url.Parse(application.URL)
		if err != nil {
			t.Fatal(err)
		}
		if cookies := jar.Cookies(applicationURL); len(cookies) != 0 {
			t.Fatalf("browser jar retained cookies after logout: %#v", cookies)
		}

		primaryLog.reset()
		response, _ = apiSecurityRequest(t, browser, http.MethodGet, application.URL+"/dashboard", nil)
		requireNoAPISecurityRenderErrors(t, response)
		requireAPISecurityCall(t, primaryLog, "/private/user", "")
		requireAPISecurityCall(t, primaryLog, "/private/admin", "")
		requireAPISecurityCall(t, primaryLog, "/public", "")
		requireAPISecurityCall(t, primaryLog, "/service", "Bearer fixture-service-token")
	})
}

func apiSecurityUpstreamHandler(log *apiSecurityRequestLog, redirectTarget string, barrier *apiSecurityBarrier, cancellationProbe *apiSecurityCancellationProbe) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch r.URL.Path {
		case "/issue/valid":
			writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
				"result":      "issued",
				"message":     "issued two credentials",
				"user_token":  "fixture-user-token",
				"admin_token": "fixture-admin-token",
			})
		case "/issue/missing":
			writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
				"result":     "incomplete",
				"message":    "admin token deliberately missing",
				"user_token": "fixture-user-token",
			})
		case "/issue/empty":
			writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
				"result":      "incomplete",
				"message":     "user token deliberately empty",
				"user_token":  "",
				"admin_token": "fixture-admin-token",
			})
		case "/issue/invalid":
			writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
				"result":      "invalid",
				"message":     "user token deliberately contains cookie syntax",
				"user_token":  "invalid; Domain=example.test",
				"admin_token": "fixture-admin-token",
			})
		case "/issue/trailing-json":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"result":"issued","user_token":"fixture-user-token"} trailing`)
		case "/logout":
			w.WriteHeader(http.StatusNoContent)
		case "/redirect/nested":
			http.Redirect(w, r, redirectTarget+"/redirect-target/nested", http.StatusFound)
		case "/redirect/fragment":
			http.Redirect(w, r, redirectTarget+"/redirect-target/fragment", http.StatusFound)
		case "/barrier/user", "/barrier/admin":
			if barrier.wait(r) {
				writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
					"source":    strings.TrimPrefix(r.URL.Path, "/"),
					"auth_kind": apiSecurityAuthKind(r.Header.Get("Authorization")),
					"message":   "fixture barrier released",
				})
			}
		case "/cancel/nested", "/cancel/fragment":
			cancellationProbe.wait(r)
		default:
			writeAPISecurityJSON(w, http.StatusOK, map[string]interface{}{
				"source":    strings.TrimPrefix(r.URL.Path, "/"),
				"auth_kind": apiSecurityAuthKind(r.Header.Get("Authorization")),
				"message":   "fixture request completed",
			})
		}
	})
}

func apiSecurityAuthKind(authorization string) string {
	switch authorization {
	case "":
		return "none"
	case "Bearer fixture-user-token":
		return "user-session"
	case "Bearer fixture-admin-token":
		return "admin-session"
	case "Bearer fixture-service-token":
		return "service"
	default:
		return "unexpected"
	}
}

func writeAPISecurityJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func loadAPISecurityTestModule(t *testing.T, upstreamURL string) {
	t.Helper()
	setupDevelopmentModeServeContentTest(t, false)

	hbConfig := shared.GetHyperBricksConfiguration()
	oldConfig := *hbConfig
	oldModuleRoot := commands.ModuleRoot
	oldRenderStatic := commands.RenderStatic
	oldModuleDirectories := core.ModuleDirectories
	oldParserConfig := parser.HbConfig
	oldHypermedias := hypermediasBySection
	oldSourceErrors := routeSourceErrors

	t.Cleanup(func() {
		*hbConfig = oldConfig
		commands.ModuleRoot = oldModuleRoot
		commands.RenderStatic = oldRenderStatic
		core.ModuleDirectories = oldModuleDirectories
		parser.HbConfig = oldParserConfig
		parser.ClearTemplateStore()

		hypermediasMutex.Lock()
		hypermediasBySection = oldHypermedias
		hypermediasMutex.Unlock()

		routeSourceErrorsMutex.Lock()
		routeSourceErrors = oldSourceErrors
		routeSourceErrorsMutex.Unlock()
	})

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	moduleDir := filepath.Join(repoRoot, "modules", "api-security-test")
	t.Setenv("HYPERBRICKS_API_SECURITY_UPSTREAM", upstreamURL)

	packageConfig, err := shared.LoadPackageConfigMap(filepath.Join(moduleDir, "package.hyperbricks.yaml"), moduleDir)
	if err != nil {
		t.Fatalf("load api-security-test package: %v", err)
	}
	hbConfig.Directories = map[string]string{}
	hbConfig.Plugins.Enabled = nil
	if err := mapstructure.WeakDecode(packageConfig["hyperbricks"], hbConfig); err != nil {
		t.Fatalf("decode api-security-test package: %v", err)
	}
	if hbConfig.Mode != shared.DEVELOPMENT_MODE {
		t.Fatalf("api-security-test mode = %q, want development", hbConfig.Mode)
	}
	hbConfig.Server.Beautify = false
	hbConfig.Development.FrontendErrors = false
	commands.ModuleRoot = moduleDir
	commands.RenderStatic = false
	parser.HbConfig = packageConfig
	parser.ClearTemplateStore()
	initializeComponents()
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatalf("load api-security-test routes: %v", err)
	}

	for _, route := range []string{
		"dashboard", "actions/login", "actions/logout", "actions/private", "actions/public",
		"actions/login-invalid-json", "actions/login-template-error",
		"wrong-cookie", "duplicate-cookie", "conflicts/header", "conflicts/basic",
		"invalid-forwardtoken-types", "invalid-forwardtoken-fragment",
		"redirects/nested", "redirects/fragment",
		"concurrent-fanout", "cancel/nested", "cancel/fragment", "debug/nested", "debug/fragment",
	} {
		if _, ok := getConfig(route); !ok {
			t.Fatalf("api-security-test route %q was not loaded", route)
		}
	}
}

func apiSecurityRequest(t *testing.T, client *http.Client, method, target string, cookies []*http.Cookie) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequest(method, target, nil)
	if err != nil {
		t.Fatalf("create %s %s: %v", method, target, err)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	return apiSecurityDo(t, client, request)
}

func apiSecurityDo(t *testing.T, client *http.Client, request *http.Request) (*http.Response, string) {
	t.Helper()
	completed := apiSecurityDoResult(client, request)
	if completed.err != nil {
		t.Fatalf("%s %s: %v", request.Method, request.URL, completed.err)
	}
	return completed.response, completed.body
}

func apiSecurityDoResult(client *http.Client, request *http.Request) apiSecurityHTTPResult {
	response, err := client.Do(request)
	if err != nil {
		return apiSecurityHTTPResult{response: response, err: err}
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return apiSecurityHTTPResult{response: response, err: readErr}
	}
	if closeErr != nil {
		return apiSecurityHTTPResult{response: response, body: string(body), err: closeErr}
	}
	return apiSecurityHTTPResult{response: response, body: string(body)}
}

func captureAPISecurityStdout(action func() error) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	original := os.Stdout
	os.Stdout = writer
	restored := false
	defer func() {
		if restored {
			return
		}
		os.Stdout = original
		_ = writer.Close()
		_ = reader.Close()
	}()

	type capturedOutput struct {
		value string
		err   error
	}
	result := make(chan capturedOutput, 1)
	go func() {
		var output bytes.Buffer
		_, copyErr := io.Copy(&output, reader)
		result <- capturedOutput{value: output.String(), err: copyErr}
	}()

	actionErr := action()
	os.Stdout = original
	restored = true
	closeErr := writer.Close()
	captured := <-result
	readerCloseErr := reader.Close()

	for _, candidate := range []error{actionErr, closeErr, captured.err, readerCloseErr} {
		if candidate != nil {
			return captured.value, candidate
		}
	}
	return captured.value, nil
}

func assertAPISecuritySessionCookie(t *testing.T, cookie *http.Cookie, value string, sameSite http.SameSite) {
	t.Helper()
	if cookie == nil {
		t.Fatal("expected session cookie, got nil")
	}
	if cookie.Value != value || cookie.Path != "/" || !cookie.HttpOnly || cookie.Secure || cookie.SameSite != sameSite {
		t.Fatalf("session cookie = %#v, want value=%q Path=/ HttpOnly Secure=false SameSite=%d", cookie, value, sameSite)
	}
}

func requireAPISecurityCall(t *testing.T, log *apiSecurityRequestLog, path, authorization string) {
	t.Helper()
	calls := log.matching(path)
	if len(calls) != 1 {
		t.Fatalf("upstream calls to %q = %d, want 1: %#v", path, len(calls), calls)
	}
	if calls[0].Authorization != authorization {
		t.Fatalf("upstream %q Authorization = %q, want %q", path, calls[0].Authorization, authorization)
	}
}

func requireNoAPISecurityCall(t *testing.T, log *apiSecurityRequestLog, path string) {
	t.Helper()
	if calls := log.matching(path); len(calls) != 0 {
		t.Fatalf("upstream %q unexpectedly received %d call(s): %#v", path, len(calls), calls)
	}
}

func requireNoAPISecurityRenderErrors(t *testing.T, response *http.Response) {
	t.Helper()
	value := response.Header.Get(renderErrorCountHeader)
	if value != "" && value != "0" {
		t.Fatalf("render error count = %q, want 0", value)
	}
}

func requireAPISecurityRenderErrors(t *testing.T, response *http.Response) {
	t.Helper()
	value := response.Header.Get(renderErrorCountHeader)
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		t.Fatalf("render error count = %q, want at least 1", value)
	}
}
