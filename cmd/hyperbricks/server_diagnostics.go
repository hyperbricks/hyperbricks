package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type deferredDiagnosticsKey struct{}

type diagnosticOutcome struct {
	requestID, route, contextKey, method string
	generation                           uint64
	sequence                             int64
	config                               map[string]interface{}
	errors                               []error
	checked                              bool
	body                                 *diagnosticBody
}

func (outcome *diagnosticOutcome) metadata() shared.Meta {
	return shared.MetaFromConfig(outcome.config)
}

// Observe application-owned reads without buffering or consuming input early.
type diagnosticBody struct {
	io.ReadCloser
	digest       hash.Hash
	read, length int64
	complete     bool
}

func (body *diagnosticBody) Read(buffer []byte) (int, error) {
	n, err := body.ReadCloser.Read(buffer)
	_, _ = body.digest.Write(buffer[:n])
	body.read += int64(n)
	body.complete = err == io.EOF || (body.length >= 0 && body.read == body.length)
	return n, err
}

type currentDiagnostics struct {
	Records         []RenderDiagnostics `json:"records"`
	Generation      uint64              `json:"generation"`
	CheckedRoutes   int                 `json:"checked_routes"`
	TotalRoutes     int                 `json:"total_routes"`
	UncheckedRoutes []string            `json:"unchecked_routes"`
	EvictedContexts int                 `json:"evicted_contexts"`
}

func diagnosticHash(diagnostic ComponentErrorTemplate) string {
	diagnostic.Hash = ""
	encoded, _ := json.Marshal(diagnostic)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:12])
}

func newDiagnosticOutcome(r *http.Request, requestID, route string, generation uint64, config map[string]interface{}) *diagnosticOutcome {
	if getHyperBricksConfiguration().Mode == shared.LIVE_MODE {
		return nil
	}
	sequence, _ := strconv.ParseInt(strings.TrimPrefix(requestID, "hb-"), 10, 64)
	if sequence == 0 {
		sequence = atomic.AddInt64(&renderDiagnosticsSeq, 1)
	}
	key, method := route, ""
	var body *diagnosticBody
	if r != nil {
		method = r.Method
		query := ""
		if r.URL != nil {
			query = r.URL.Query().Encode()
		}
		response, _ := resolveHTTPResponse(config)
		variant := struct {
			Method, Query, Authorization, Cookie string
			Headers                              map[string][]string
		}{method, query, r.Header.Get("Authorization"), r.Header.Get("Cookie"), make(map[string][]string)}
		for _, name := range strings.Split(routeResponseVary(config, response), ",") {
			name = strings.TrimSpace(name)
			if name == "*" {
				variant.Headers = r.Header.Clone()
				break
			}
			if name != "" {
				variant.Headers[name] = requestHeaderValues(r, name)
			}
		}
		encoded, _ := json.Marshal(variant)
		sum := sha256.Sum256(encoded)
		key += "|" + hex.EncodeToString(sum[:])
		if r.Body != nil && r.Body != http.NoBody {
			body = &diagnosticBody{ReadCloser: r.Body, digest: sha256.New(), length: r.ContentLength}
			r.Body = body
		}
	}
	return &diagnosticOutcome{requestID: requestID, route: route, contextKey: key, method: method,
		generation: generation, sequence: sequence, config: config, body: body}
}

// Called while publishing the matching configs/plans under configMutex.
func resetRenderDiagnostics(generation uint64, routes map[string]map[string]interface{}) {
	renderDiagnosticsMutex.Lock()
	defer renderDiagnosticsMutex.Unlock()
	diagnosticsGeneration = generation
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	diagnosticsFloor, diagnosticsEvicted = 0, 0
	diagnosticsRoutes = make([]string, 0, len(routes))
	for route := range routes {
		diagnosticsRoutes = append(diagnosticsRoutes, route)
	}
	sort.Strings(diagnosticsRoutes)
}

func commitRenderDiagnostics(r *http.Request, outcome *diagnosticOutcome) {
	if outcome == nil || !outcome.checked || getHyperBricksConfiguration().Mode == shared.LIVE_MODE {
		return
	}
	issues := collectRenderDiagnostics(outcome.errors)
	contextKey := outcome.contextKey
	if body := outcome.body; body != nil && body.read > 0 {
		if body.complete {
			contextKey += "|body=" + hex.EncodeToString(body.digest.Sum(nil))
		} else {
			contextKey += "|incomplete=" + outcome.requestID
		}
	}
	renderDiagnosticsMutex.Lock()
	if outcome.generation != diagnosticsGeneration || outcome.sequence <= diagnosticsFloor {
		renderDiagnosticsMutex.Unlock()
		return
	}
	contextID := ""
	for index, id := range renderDiagnosticsOrder {
		previous := renderDiagnostics[id]
		if previous.contextKey != contextKey {
			continue
		}
		if previous.sequence > outcome.sequence {
			renderDiagnosticsMutex.Unlock()
			return
		}
		contextID = previous.ContextID
		delete(renderDiagnostics, id)
		renderDiagnosticsOrder = append(renderDiagnosticsOrder[:index], renderDiagnosticsOrder[index+1:]...)
		break
	}
	if contextID == "" {
		contextID = shared.GenerateHash()
	}
	renderDiagnostics[outcome.requestID] = RenderDiagnostics{
		RequestID: outcome.requestID, Route: outcome.route, CreatedAt: time.Now().UTC(), Errors: issues,
		ContextID: contextID, Method: outcome.method, Generation: outcome.generation,
		contextKey: contextKey, sequence: outcome.sequence,
	}
	renderDiagnosticsOrder = append(renderDiagnosticsOrder, outcome.requestID)
	for len(renderDiagnosticsOrder) > maxRenderDiagnostics {
		oldest := renderDiagnosticsOrder[0]
		if sequence := renderDiagnostics[oldest].sequence; sequence > diagnosticsFloor {
			diagnosticsFloor = sequence
		}
		delete(renderDiagnostics, oldest)
		renderDiagnosticsOrder = renderDiagnosticsOrder[1:]
		diagnosticsEvicted++
	}
	renderDiagnosticsMutex.Unlock()
}

// Failure events are independent of the development-only current-outcome store.
func logRenderDiagnostics(r *http.Request, requestID, route string, errors []error) {
	if len(errors) == 0 {
		return
	}
	logger := logging.GetLogger().Named("render")
	for _, issue := range collectRenderDiagnostics(errors) {
		issue.File = runtimeSourcePath(issue.File)
		issue.Resource = runtimeSourcePath(issue.Resource)
		fields := []interface{}{"module", filepath.Base(commands.GetModuleRoot()), "route", runtimeRoutePath(route), "request_id", requestID, "error", logging.ModuleText(commands.GetModuleRoot(), relativeYAMLLoadError(issue.Err))}
		for _, field := range [][2]string{{"component", issue.Type}, {"file", issue.File}, {"path", issue.Path}, {"key", issue.Key}, {"phase", issue.Phase}, {"resource", issue.Resource}} {
			if field[1] != "" {
				fields = append(fields, field[0], field[1])
			}
		}
		for _, field := range []struct {
			name  string
			value int
		}{{"line", issue.Line}, {"column", issue.Column}, {"resource_line", issue.ResourceLine}, {"resource_column", issue.ResourceColumn}} {
			if field.value > 0 {
				fields = append(fields, field.name, field.value)
			}
		}
		if r != nil {
			fields = append(fields, "method", r.Method)
		}
		if link := renderDiagnosticsURL(r, requestID); link != "" {
			if parsed, err := url.Parse(link); err == nil {
				fields = append(fields, "diagnostics_url", parsed.RequestURI())
			}
		}
		switch {
		case (strings.EqualFold(issue.Level, "WARNING") || strings.EqualFold(issue.Level, "WARN")) && !issue.Rejected:
			logger.Warnw("Render warning", fields...)
		case strings.EqualFold(issue.Level, "INFO") && !issue.Rejected:
			logger.Infow("Render notice", fields...)
		default:
			logger.Errorw("Render failed", fields...)
		}
	}
}

func collectCurrentRenderDiagnostics() currentDiagnostics {
	renderDiagnosticsMutex.RLock()
	defer renderDiagnosticsMutex.RUnlock()
	snapshot := currentDiagnostics{Records: make([]RenderDiagnostics, 0, len(renderDiagnosticsOrder)), Generation: diagnosticsGeneration,
		TotalRoutes: len(diagnosticsRoutes), UncheckedRoutes: make([]string, 0, len(diagnosticsRoutes)), EvictedContexts: diagnosticsEvicted}
	checked := make(map[string]bool, len(diagnosticsRoutes))
	for index := len(renderDiagnosticsOrder) - 1; index >= 0; index-- {
		record := renderDiagnostics[renderDiagnosticsOrder[index]]
		checked[record.Route] = true
		if len(record.Errors) > 0 {
			snapshot.Records = append(snapshot.Records, record)
		}
	}
	for _, route := range diagnosticsRoutes {
		if checked[route] {
			snapshot.CheckedRoutes++
		} else {
			snapshot.UncheckedRoutes = append(snapshot.UncheckedRoutes, route)
		}
	}
	return snapshot
}

func finishServedDiagnostics(r *http.Request, outcome *diagnosticOutcome, err error) {
	if outcome == nil {
		return
	}
	if err != nil {
		if r.Context().Err() == context.Canceled {
			return
		}
		outcome.errors = append(outcome.errors, shared.Diagnostic(err, outcome.metadata(), "serve"))
		outcome.checked = true
	}
	commitRenderDiagnostics(r, outcome)
}
