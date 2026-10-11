package main

import (
	"fmt"
	htmlparser "golang.org/x/net/html"
	"html"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

const statusIndicatorPath = "/__hyperbricks/status-indicator.js"

func statusIndicatorEnabled() bool {
	return developerDashboardEnabled() && getHyperBricksConfiguration().Development.StatusIndicator
}

func handleStatusIndicator(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != statusIndicatorPath && r.URL.Path != "/__hyperbricks/status-indicator.css" {
		return false
	}
	if !statusIndicatorEnabled() {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	content := assets.StatusIndicatorScript
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	if r.URL.Path == "/__hyperbricks/status-indicator.css" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		content = assets.StatusIndicatorStylesheet
	}
	if r.Method == http.MethodGet {
		_, _ = w.Write(content)
	}
	return true
}

func decorateStatusIndicator(content, kind, contentType, requestID string, errors int, r *http.Request) string {
	if !statusIndicatorEnabled() || r == nil || r.Method != http.MethodGet || kind != composite.HyperMediaConfigGetName() {
		return content
	}
	if contentType != "" && !strings.EqualFold(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]), "text/html") {
		return content
	}
	// Tokenize the document so attributes, case and head-like text in comments
	// cannot move the script outside the real head element.
	tokenizer := htmlparser.NewTokenizer(strings.NewReader(content))
	start := 0
	for {
		kind := tokenizer.Next()
		if kind == htmlparser.ErrorToken {
			return content
		}
		start += len(tokenizer.Raw())
		if kind == htmlparser.StartTagToken && tokenizer.Token().Data == "head" {
			break
		}
	}
	tag := fmt.Sprintf(`<script src="%s" data-request="%s" data-errors="%d"></script>`, statusIndicatorPath, html.EscapeString(requestID), errors)
	return content[:start] + tag + content[start:]
}
