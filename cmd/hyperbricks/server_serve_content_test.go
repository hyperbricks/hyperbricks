package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func setupLiveModeServeContentTest(t *testing.T) {
	t.Helper()

	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldCacheDuration := hbConfig.Live.CacheTime.Duration
	oldConfigs := configs
	oldRM := rm

	htmlCacheMutex.Lock()
	oldHTMLCache := htmlCache
	htmlCache = make(map[string]CacheEntry)
	htmlCacheMutex.Unlock()

	configMutex.Lock()
	configs = make(map[string]map[string]interface{})
	configMutex.Unlock()

	hbConfig.Mode = shared.LIVE_MODE
	hbConfig.Live.CacheTime.Duration = time.Hour

	initializeComponents()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Live.CacheTime.Duration = oldCacheDuration
		rm = oldRM

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		htmlCacheMutex.Lock()
		htmlCache = oldHTMLCache
		htmlCacheMutex.Unlock()
	})
}

func setTestRouteConfig(route string, config map[string]interface{}) {
	configMutex.Lock()
	defer configMutex.Unlock()
	configs[route] = config
}

func cachedEntry(route string) (CacheEntry, bool) {
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	entry, ok := htmlCache[route]
	return entry, ok
}

func TestServeContent_LiveMode_DoesNotLeakRequestSensitiveContent(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("leak", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "leak",
		"template": map[string]interface{}{
			"@type":     composite.TemplateConfigGetName(),
			"inline":    `viewer={{index .Params "viewer"}}`,
			"querykeys": "viewer",
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	firstWriter := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/leak?viewer=alice", nil)
	ServeContent(firstWriter, firstRequest)

	secondWriter := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/leak?viewer=bob", nil)
	ServeContent(secondWriter, secondRequest)

	firstBody := firstWriter.Body.String()
	secondBody := secondWriter.Body.String()

	if !strings.Contains(firstBody, "viewer=alice") {
		t.Fatalf("expected first response to contain alice, got %q", firstBody)
	}
	if !strings.Contains(secondBody, "viewer=bob") {
		t.Fatalf("expected second response to contain bob, got %q", secondBody)
	}
	if strings.Contains(secondBody, "viewer=alice") {
		t.Fatalf("expected second response not to leak alice, got %q", secondBody)
	}
}

func TestServeContent_LiveMode_StillCachesRequestInsensitiveRoute(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("static", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "static",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `stable-content`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	firstWriter := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/static", nil)
	ServeContent(firstWriter, firstRequest)

	firstEntry, found := cachedEntry("static")
	if !found {
		t.Fatalf("expected static route to be cached after first request")
	}
	firstRenderedAt := firstWriter.Header().Get(liveCacheRenderedAtHeader)
	firstExpiresAt := firstWriter.Header().Get(liveCacheExpiresAtHeader)
	if firstRenderedAt == "" || firstExpiresAt == "" {
		t.Fatalf("expected cache metadata headers to be present on first response, got rendered=%q expires=%q", firstRenderedAt, firstExpiresAt)
	}

	secondWriter := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/static", nil)
	ServeContent(secondWriter, secondRequest)

	secondEntry, found := cachedEntry("static")
	if !found {
		t.Fatalf("expected static route cache entry to remain present")
	}

	if !firstEntry.Timestamp.Equal(secondEntry.Timestamp) {
		t.Fatalf("expected second request to reuse cached entry, timestamps differ: %v vs %v", firstEntry.Timestamp, secondEntry.Timestamp)
	}
	if secondWriter.Header().Get(liveCacheRenderedAtHeader) != firstRenderedAt {
		t.Fatalf("expected cached response to reuse rendered-at header, got %q want %q", secondWriter.Header().Get(liveCacheRenderedAtHeader), firstRenderedAt)
	}
	if secondWriter.Header().Get(liveCacheExpiresAtHeader) != firstExpiresAt {
		t.Fatalf("expected cached response to reuse cache-expires header, got %q want %q", secondWriter.Header().Get(liveCacheExpiresAtHeader), firstExpiresAt)
	}

	if !strings.Contains(firstWriter.Body.String(), "stable-content") {
		t.Fatalf("expected first response to contain stable content, got %q", firstWriter.Body.String())
	}
	if !strings.Contains(secondWriter.Body.String(), "stable-content") {
		t.Fatalf("expected second response to contain stable content, got %q", secondWriter.Body.String())
	}
	if strings.Contains(firstWriter.Body.String(), "Rendered at") || strings.Contains(firstWriter.Body.String(), "Cache expires at") {
		t.Fatalf("expected first HTML response body not to include cache comments, got %q", firstWriter.Body.String())
	}
	if strings.Contains(secondWriter.Body.String(), "Rendered at") || strings.Contains(secondWriter.Body.String(), "Cache expires at") {
		t.Fatalf("expected cached HTML response body not to include cache comments, got %q", secondWriter.Body.String())
	}
}

func TestServeContent_LiveMode_DoesNotAppendHTMLCacheCommentsToJSON(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("json", map[string]interface{}{
		"@type":        composite.FragmentConfigGetName(),
		"route":        "json",
		"content_type": "application/json",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `{"status":"ok"}`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/json", nil)
	ServeContent(writer, request)

	body := writer.Body.Bytes()
	if !json.Valid(body) {
		t.Fatalf("expected live-mode JSON response to remain valid JSON, got %q", string(body))
	}
	if writer.Header().Get(liveCacheRenderedAtHeader) == "" || writer.Header().Get(liveCacheExpiresAtHeader) == "" {
		t.Fatalf("expected cache metadata headers on JSON response, got rendered=%q expires=%q", writer.Header().Get(liveCacheRenderedAtHeader), writer.Header().Get(liveCacheExpiresAtHeader))
	}
	if strings.Contains(string(body), "Rendered at") || strings.Contains(string(body), "Cache expires at") {
		t.Fatalf("expected live-mode JSON response not to include HTML cache comments, got %q", string(body))
	}

	entry, found := cachedEntry("json")
	if !found {
		t.Fatalf("expected json route to be cached")
	}
	if !json.Valid([]byte(entry.Content)) {
		t.Fatalf("expected cached JSON body to remain valid JSON, got %q", entry.Content)
	}
	if entry.Headers[liveCacheRenderedAtHeader] == "" || entry.Headers[liveCacheExpiresAtHeader] == "" {
		t.Fatalf("expected cached JSON entry to include cache metadata headers, got %#v", entry.Headers)
	}
	if strings.Contains(entry.Content, "Rendered at") || strings.Contains(entry.Content, "Cache expires at") {
		t.Fatalf("expected cached JSON body not to include HTML cache comments, got %q", entry.Content)
	}
}
