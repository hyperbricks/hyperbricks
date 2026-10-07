package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// Requests hold the read side through cache publication and HTTP delivery.
// Maintenance takes the write side, so no new render can acquire a soon-to-be
// deleted generation between the reference check and unlink.
var runtimeAssetGate sync.RWMutex
var runtimeAssetRequests atomic.Int64

func runtimeEsbuildRenderer() *component.EsbuildRenderer {
	if rm == nil {
		return nil
	}
	renderer, _ := rm.GetRenderComponent(component.EsbuildConfigGetName()).(*component.EsbuildRenderer)
	return renderer
}

func protectRuntimeAsset(path string) bool {
	if commands.RenderStatic || runtimeAssetRequests.Load() > 0 {
		return true
	}
	root := getHyperBricksConfiguration().Directories["static"]
	root, _ = filepath.Abs(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return true
	}
	public := (&url.URL{Path: "/static/" + filepath.ToSlash(rel)}).EscapedPath()
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	for _, entry := range htmlCache {
		body := entry.Content
		if entry.cacheDiskPath != "" {
			raw, err := os.ReadFile(entry.cacheDiskPath)
			if err != nil {
				return true
			}
			body = string(raw)
		}
		if strings.Contains(body, public) {
			return true
		}
	}
	return false
}

func assetRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimeAssetGate.RLock()
		runtimeAssetRequests.Add(1)
		defer func() {
			runtimeAssetRequests.Add(-1)
			runtimeAssetGate.RUnlock()
			if runtimeAssetGate.TryLock() {
				defer runtimeAssetGate.Unlock()
				if renderer := runtimeEsbuildRenderer(); renderer != nil {
					if err := renderer.ReconcileAssets(nil); err != nil {
						logging.GetLogger().Errorw("Generated asset cleanup failed", "error", err)
					}
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func closeRuntimeAssets() {
	if renderer := runtimeEsbuildRenderer(); renderer != nil {
		renderer.CloseAssets()
	}
}
