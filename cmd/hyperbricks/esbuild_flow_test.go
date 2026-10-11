package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type esbuildFlowFixture struct {
	t                                 *testing.T
	config                            *shared.Config
	entry, dependency, source, static string
	handler                           http.Handler
}

func newEsbuildFlowFixture(t *testing.T, mode string, cached, fingerprint, nocache bool) *esbuildFlowFixture {
	t.Helper()
	setupLiveModeServeContentTest(t)
	config := shared.GetHyperBricksConfiguration()
	oldConfig := *config
	oldRoot, oldDirectories, oldParser := commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig
	oldSections, oldErrors := hypermediasBySection, routeSourceErrors
	oldDiagnostics, oldOrder, oldSequence := renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq
	t.Cleanup(func() {
		closeRuntimeAssets()
		*config = oldConfig
		commands.ModuleRoot, core.ModuleDirectories, parser.HbConfig = oldRoot, oldDirectories, oldParser
		hypermediasBySection, routeSourceErrors = oldSections, oldErrors
		renderDiagnostics, renderDiagnosticsOrder, renderDiagnosticsSeq = oldDiagnostics, oldOrder, oldSequence
		parser.ClearTemplateStore()
	})
	root := t.TempDir()
	commands.ModuleRoot = root
	config.Mode = mode
	config.Directories = make(map[string]string)
	for _, name := range []string{"resources", "static", "templates", "hyperbricks", "render", "plugins"} {
		config.Directories[name] = filepath.Join(root, name)
		if err := os.MkdirAll(config.Directories[name], 0755); err != nil {
			t.Fatal(err)
		}
	}
	config.Development.FrontendErrors = false
	config.Development.WatchDirs = []string{"hyperbricks", "templates", "resources"}
	config.Development.Reload = false
	config.Development.Watch = false
	config.RateLimit.Enabled = false
	config.Server.Beautify = false
	parser.HbConfig = map[string]interface{}{}
	initializeComponents()
	f := &esbuildFlowFixture{t: t, config: config, entry: filepath.Join(config.Directories["resources"], "main.js"), dependency: filepath.Join(config.Directories["resources"], "label.js"), source: filepath.Join(config.Directories["hyperbricks"], "page.hyperbricks.yaml"), static: config.Directories["static"]}
	f.write(f.entry, `import {label} from "./label.js"; window.hbBuildLabel = label;`)
	f.setLabel("initial")
	f.write(f.source, fmt.Sprintf(`bundle:
  - type: esbuild
  - entry: {path: {base: resources, path: main.js}}
  - outfile: {path: {base: static, path: js/app.js}}
  - cache: %t
  - fingerprint: %t
  - enclose: '<script src="|" defer></script>'
page:
  - type: hypermedia
  - route: index
  - nocache: %t
  - head:
      - type: head
      - script:
          - inherit: bundle
  - body:
      - type: html
      - value: '<p>Esbuild flow</p>'
`, cached, fingerprint, nocache))
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatal(err)
	}
	f.handler = buildRuntimeHandler(nil)
	return f
}

func (f *esbuildFlowFixture) write(path, value string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *esbuildFlowFixture) setLabel(value string) {
	f.write(f.dependency, fmt.Sprintf("export const label = %q;", value))
}
func (f *esbuildFlowFixture) request() *httptest.ResponseRecorder {
	f.t.Helper()
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	return w
}
func (f *esbuildFlowFixture) asset(w *httptest.ResponseRecorder, want string) (string, os.FileInfo) {
	f.t.Helper()
	if w.Code != 200 {
		f.t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	if count := w.Header().Get(renderErrorCountHeader); count != "" && count != "0" {
		f.t.Fatalf("render errors %s: %s", count, w.Body.String())
	}
	url := esbuildScriptURL(f.t, w.Body.String(), "/static/js/app.")
	asset := httptest.NewRecorder()
	f.handler.ServeHTTP(asset, httptest.NewRequest("GET", url, nil))
	if asset.Code != 200 || !strings.Contains(asset.Body.String(), want) {
		f.t.Fatalf("asset %s: HTTP %d, want %q: %s", url, asset.Code, want, asset.Body.String())
	}
	info, err := os.Stat(filepath.Join(f.static, filepath.FromSlash(strings.TrimPrefix(url, "/static/"))))
	if err != nil {
		f.t.Fatal(err)
	}
	return url, info
}
func (f *esbuildFlowFixture) expirePage() {
	f.t.Helper()
	htmlCacheMutex.Lock()
	defer htmlCacheMutex.Unlock()
	if len(htmlCache) == 0 {
		f.t.Fatal("live page did not populate its cache")
	}
	for key, entry := range htmlCache {
		entry.Timestamp = time.Now().Add(-2 * time.Hour)
		htmlCache[key] = entry
	}
}

func TestEsbuildFlowModeAndCacheMatrix(t *testing.T) {
	for _, flow := range []struct {
		name, mode string
		nocache    bool
	}{{"development", shared.DEVELOPMENT_MODE, false}, {"live-page-cache", shared.LIVE_MODE, false}, {"live-nocache", shared.LIVE_MODE, true}} {
		for _, cached := range []bool{false, true} {
			for _, fingerprint := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/cache=%t/fingerprint=%t", flow.name, cached, fingerprint), func(t *testing.T) {
					f := newEsbuildFlowFixture(t, flow.mode, cached, fingerprint, flow.nocache)
					files, err := os.ReadDir(f.static)
					if err != nil || len(files) != 0 {
						t.Fatalf("init built assets: %v %v", files, err)
					}
					firstURL, first := f.asset(f.request(), "initial")
					fresh := flow.mode == shared.DEVELOPMENT_MODE || flow.nocache
					_, unchanged := f.asset(f.request(), "initial")
					if os.SameFile(first, unchanged) != (!fresh || cached) {
						t.Fatal("unchanged request did not follow page/component cache policy")
					}
					info, err := os.Stat(f.dependency)
					if err != nil {
						t.Fatal(err)
					}
					f.setLabel("updated") // Equal length, and deliberately preserve mtime.
					if err := os.Chtimes(f.dependency, info.ModTime(), info.ModTime()); err != nil {
						t.Fatal(err)
					}
					if !fresh {
						url, same := f.asset(f.request(), "initial")
						if url != firstURL || !os.SameFile(unchanged, same) {
							t.Fatal("cached live page unexpectedly ran esbuild")
						}
						f.expirePage()
					}
					updatedURL, updated := f.asset(f.request(), "updated")
					if fingerprint == (updatedURL == firstURL) {
						t.Fatal("changed source did not follow fingerprint URL policy")
					}
					if os.SameFile(updated, unchanged) {
						t.Fatal("changed source did not rebuild output")
					}
					_, again := f.asset(f.request(), "updated")
					if os.SameFile(updated, again) != (!fresh || cached) {
						t.Fatal("follow-up request did not reuse/rebuild as configured")
					}
				})
			}
		}
	}
}

func TestEsbuildFlowWatchChangeAndOutputExclusion(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%t", cached), func(t *testing.T) {
			f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, cached, true, false)
			f.asset(f.request(), "initial")
			f.config.Development.Watch = true
			f.config.Development.WatchDirs = append(f.config.Development.WatchDirs, "static")
			events := make(chan error, 8)
			reloads := newRuntimeReload(func() { events <- PreProcessAndPopulateConfigs() })
			reloads.Enable()
			ctx, cancel := context.WithCancel(context.Background())
			done, err := startRuntimeWatcher(ctx, resolveDevelopmentWatchDirectories(f.config), reloads.Request)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("watcher failed to stop")
				}
				stopCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				if err := reloads.Stop(stopCtx); err != nil {
					t.Error(err)
				}
			})
			f.setLabel("watched")
			select {
			case err := <-events:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("source watch event did not reload module")
			}
			// Wait for callback completion before using newly published globals.
			deadline := time.Now().Add(5 * time.Second)
			for {
				reloads.mu.Lock()
				running := reloads.running
				reloads.mu.Unlock()
				if !running {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("reload did not finish")
				}
				time.Sleep(time.Millisecond)
			}
			_, updated := f.asset(f.request(), "watched")
			_, again := f.asset(f.request(), "watched")
			if os.SameFile(updated, again) != cached {
				t.Fatal("watch follow-up did not follow component cache")
			}
			select {
			case err := <-events:
				t.Fatalf("generated output triggered another reload: %v", err)
			case <-time.After(650 * time.Millisecond):
			}
		})
	}
}

func TestEsbuildFlowFailureAndRecovery(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%t", cached), func(t *testing.T) {
			f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, cached, true, false)
			url, before := f.asset(f.request(), "initial")
			f.write(f.entry, `import "./missing.js";`)
			failed := f.request()
			if count := failed.Header().Get(renderErrorCountHeader); count == "" || count == "0" {
				t.Fatalf("failed build silently succeeded: %s", failed.Body.String())
			}
			old, err := os.Stat(filepath.Join(f.static, filepath.FromSlash(strings.TrimPrefix(url, "/static/"))))
			if err != nil || !os.SameFile(before, old) {
				t.Fatal("failed build lost last successful output", err)
			}
			f.write(f.entry, `import {label} from "./label.js"; window.hbBuildLabel = label;`)
			f.setLabel("recovered")
			f.asset(f.request(), "recovered")
		})
	}
}

func TestEsbuildFlowConcurrentChangedSource(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%t", cached), func(t *testing.T) {
			f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, cached, true, false)
			initialURL, _ := f.asset(f.request(), "initial")
			f.setLabel("concurrent")
			const count = 12
			replies := make(chan *httptest.ResponseRecorder, count)
			for i := 0; i < count; i++ {
				go func() {
					w := httptest.NewRecorder()
					f.handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
					replies <- w
				}()
			}
			var url string
			for i := 0; i < count; i++ {
				select {
				case response := <-replies:
					got, _ := f.asset(response, "concurrent")
					if got == initialURL {
						t.Fatal("changed source reused old URL")
					}
					if url != "" && got != url {
						t.Fatal("concurrent requests returned different generations")
					}
					url = got
				case <-time.After(5 * time.Second):
					t.Fatal("concurrent request blocked")
				}
			}
		})
	}
}

func TestEsbuildFlowSourceAndSettingsReload(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, false)
	initialURL, initial := f.asset(f.request(), "initial")
	// A template-only module reload currently invalidates esbuild reuse too.
	f.write(filepath.Join(f.config.Directories["templates"], "unused.html"), "<p>Changed template</p>")
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatal(err)
	}
	url, reloaded := f.asset(f.request(), "initial")
	if url != initialURL || os.SameFile(initial, reloaded) {
		t.Fatal("module reload did not rebuild identical JavaScript")
	}
	_, reused := f.asset(f.request(), "initial")
	if !os.SameFile(reloaded, reused) {
		t.Fatal("unchanged request after reload rebuilt again")
	}
	source, err := os.ReadFile(f.source)
	if err != nil {
		t.Fatal(err)
	}
	f.write(f.source, strings.Replace(string(source), "  - cache: true", "  - cache: true\n  - minify: true", 1))
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatal(err)
	}
	changedURL, _ := f.asset(f.request(), "initial")
	if changedURL == initialURL {
		t.Fatal("changed compiler settings reused previous output")
	}
}

func TestEsbuildFlowDevelopmentFixedURLDoesNotReturnStaleNotModified(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, false, false)
	url, first := f.asset(f.request(), "initial")
	original := httptest.NewRecorder()
	f.handler.ServeHTTP(original, httptest.NewRequest("GET", url, nil))
	modified := original.Header().Get("Last-Modified")
	if modified == "" {
		t.Fatal("static response missing modification time")
	}
	f.setLabel("updated")
	f.asset(f.request(), "updated")
	// HTTP modification dates have second precision. Simulate two different
	// publications within the same tick of a filesystem/browser cache clock.
	path := filepath.Join(f.static, filepath.FromSlash(strings.TrimPrefix(url, "/static/")))
	if err := os.Chtimes(path, first.ModTime(), first.ModTime()); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", url, nil)
	request.Header.Set("If-Modified-Since", modified)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "updated") {
		t.Fatalf("development browser can retain old JS: HTTP %d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("development assets can be stored in the browser cache")
	}
}

func TestEsbuildFlowLiveStaticConditionalRequestsRemainSupported(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.LIVE_MODE, true, true, false)
	url, _ := f.asset(f.request(), "initial")
	first := httptest.NewRecorder()
	f.handler.ServeHTTP(first, httptest.NewRequest("GET", url, nil))
	request := httptest.NewRequest("GET", url, nil)
	request.Header.Set("If-Modified-Since", first.Header().Get("Last-Modified"))
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified {
		t.Fatalf("live static caching changed: HTTP %d", response.Code)
	}
	if response.Header().Get("Cache-Control") == "no-store" {
		t.Fatal("development browser policy leaked into live mode")
	}
}
