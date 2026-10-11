package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"go.uber.org/zap/zapcore"
)

func setupDashboardStartupTest(t *testing.T) *shared.Config {
	t.Helper()
	setupErrorsViewTest(t)
	config := getHyperBricksConfiguration()
	oldConfig, oldMux := *config, http.DefaultServeMux
	oldServer, oldLevel := currentServer(), logging.GetLogger().Level()
	oldLocation := shared.Location
	// Startup must not depend on a previously advertised network address.
	shared.Location = "192.0.2.1:1234"
	config.Server.Port = 0
	config.Server.RuntimeGateway.Enabled = false
	logging.ChangeLevel(zapcore.InfoLevel)
	http.DefaultServeMux = http.NewServeMux()
	t.Cleanup(func() {
		*config, http.DefaultServeMux = oldConfig, oldMux
		publishServer(oldServer)
		shared.Location = oldLocation
		logging.ChangeLevel(oldLevel)
	})
	return config
}

func TestDashboardStartupURL(t *testing.T) {
	for _, scenario := range []struct {
		name                         string
		mode                         string
		disabled, production, static bool
		configuredPort, debugLog     bool
		wantURL                      bool
	}{
		{name: "development assigned port", mode: shared.DEVELOPMENT_MODE, wantURL: true},
		{name: "development configured port", mode: shared.DEVELOPMENT_MODE, configuredPort: true, wantURL: true},
		{name: "debug", mode: shared.DEBUG_MODE, debugLog: true, wantURL: true},
		{name: "disabled", mode: shared.DEVELOPMENT_MODE, disabled: true},
		{name: "live", mode: shared.LIVE_MODE},
		{name: "production", mode: shared.DEVELOPMENT_MODE, production: true},
		{name: "static", mode: shared.DEVELOPMENT_MODE, static: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			config := setupDashboardStartupTest(t)
			config.Mode = scenario.mode
			config.Development.Dashboard.Enabled = !scenario.disabled
			if scenario.configuredPort {
				config.Server.Port = reserveTCPPort(t)
			}
			if scenario.debugLog {
				logging.ChangeLevel(zapcore.DebugLevel)
			}
			runtime := shared.GetRuntimeOptions()
			runtime.Production = scenario.production
			shared.SetRuntimeOptions(runtime)
			commands.RenderStatic = scenario.static
			registerDashboardHandlers(http.DefaultServeMux)

			started := time.Now()
			running, err := startRuntimeHTTPServer()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := running.shutdown(ctx); err != nil {
					t.Error(err)
				}
			})

			var dashboardURLs []string
			listening := false
			for _, event := range logging.GetLogs() {
				if event.Time.Before(started) {
					continue
				}
				if event.Message == "Listening" {
					listening = true
				}
				if event.Message == "Dashboard" {
					if !listening || event.Level != zapcore.InfoLevel {
						t.Fatal("Dashboard URL must be logged at INFO after the listener starts")
					}
					link, _ := event.Fields["url"].(string)
					dashboardURLs = append(dashboardURLs, link)
				}
			}
			if !scenario.wantURL {
				if len(dashboardURLs) != 0 {
					t.Fatalf("unavailable Dashboard advertised: %v", dashboardURLs)
				}
				return
			}
			if len(dashboardURLs) != 1 {
				t.Fatalf("expected one Dashboard URL, got %v", dashboardURLs)
			}
			link := dashboardURLs[0]
			parsed, err := url.Parse(link)
			if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "localhost" ||
				parsed.Path != developerDashboardPath || parsed.User != nil || parsed.RawQuery != "" {
				t.Fatalf("unexpected Dashboard URL: %q (%v)", link, err)
			}
			port, err := strconv.Atoi(parsed.Port())
			if err != nil || port <= 0 || (scenario.configuredPort && port != config.Server.Port) {
				t.Fatalf("Dashboard URL has wrong port: %q", link)
			}

			client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{}}
			defer client.CloseIdleConnections()
			response, err := client.Get(link)
			if err != nil {
				t.Fatalf("logged Dashboard URL is not reachable: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected existing login challenge, got %d", response.StatusCode)
			}
			request, err := http.NewRequest(http.MethodGet, link, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.SetBasicAuth(developerTestCredentials.User, developerTestCredentials.Password)
			response, err = client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Overview | HyperBricks Dashboard") {
				t.Fatalf("logged URL did not open Dashboard: status=%d, error=%v", response.StatusCode, err)
			}
			if !strings.Contains(string(body), link) {
				t.Fatal("Dashboard log view omitted the startup URL")
			}
		})
	}
}

func TestDashboardStartupDoesNotAdvertiseFailedListener(t *testing.T) {
	config := setupDashboardStartupTest(t)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	config.Server.Port = occupied.Addr().(*net.TCPAddr).Port
	started := time.Now()
	running, err := startRuntimeHTTPServer()
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = running.shutdown(ctx)
		t.Fatal("expected occupied port to prevent startup")
	}
	for _, event := range logging.GetLogs() {
		if !event.Time.Before(started) && event.Message == "Dashboard" {
			t.Fatalf("Dashboard advertised after failed startup: %#v", event)
		}
	}
}
