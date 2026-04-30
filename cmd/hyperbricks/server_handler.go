package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if handleRenderDiagnosticsEndpoint(w, r) {
		return
	}

	ServeContent(w, r)

	elapsed := time.Since(start)

	// counter logic (temporary...)
	requestCounterMutex.Lock()
	requestCounter = requestCounter + 1
	counter := requestCounter
	requestCounterMutex.Unlock()

	if counter%100 == 0 {
		fmt.Println("Requests processed", counter)
	}

	logging.GetLogger().Debugw("Request processed", "duration", elapsed)
}

func handleRenderDiagnosticsEndpoint(w http.ResponseWriter, r *http.Request) bool {
	if strings.Trim(r.URL.Path, "/") != "__hyperbricks/render-diagnostics" {
		return false
	}
	if shared.GetHyperBricksConfiguration().Mode == shared.LIVE_MODE {
		http.NotFound(w, r)
		return true
	}

	if requestID := strings.TrimSpace(r.URL.Query().Get("request_id")); requestID != "" {
		renderDiagnosticsMutex.RLock()
		diagnostics, ok := renderDiagnostics[requestID]
		renderDiagnosticsMutex.RUnlock()
		if !ok {
			http.NotFound(w, r)
			return true
		}
		writeDiagnosticsJSON(w, http.StatusOK, diagnostics)
		return true
	}

	limit := 10
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	payload := collectRecentRenderDiagnostics(limit)
	writeDiagnosticsJSON(w, http.StatusOK, payload)
	return true
}

func collectRecentRenderDiagnostics(limit int) []RenderDiagnostics {
	renderDiagnosticsMutex.RLock()
	defer renderDiagnosticsMutex.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	if limit > len(renderDiagnosticsOrder) {
		limit = len(renderDiagnosticsOrder)
	}

	results := make([]RenderDiagnostics, 0, limit)
	for index := len(renderDiagnosticsOrder) - 1; index >= 0 && len(results) < limit; index-- {
		requestID := renderDiagnosticsOrder[index]
		diagnostics, ok := renderDiagnostics[requestID]
		if !ok {
			continue
		}
		results = append(results, diagnostics)
	}
	return results
}

func writeDiagnosticsJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logging.GetLogger().Errorw("Failed to encode render diagnostics response", "error", err)
	}
}
