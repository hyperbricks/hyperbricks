//go:build !legacy_hyperbricks_parser

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type documentationExamplePlugin struct{}

func (documentationExamplePlugin) Render(data interface{}, ctx context.Context) (any, []error) {
	return "Plugin example", nil
}

func compactMarkdownOutsideCodeFences(content string) string {
	var out strings.Builder
	inFence := false
	blankRun := 0

	for _, rawLine := range strings.SplitAfter(content, "\n") {
		line, hasNewline := strings.CutSuffix(rawLine, "\n")
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimRight(line, " \t")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			blankRun = 0
			out.WriteString(line)
			if hasNewline {
				out.WriteString("\n")
			}
			continue
		}
		if !inFence && trimmed == "" {
			blankRun++
			if blankRun <= 2 {
				out.WriteString("\n")
			}
			continue
		}

		blankRun = 0
		out.WriteString(line)
		if hasNewline {
			out.WriteString("\n")
		}
	}

	return strings.TrimRight(out.String(), "\n") + "\n"
}

func stripAllWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

func createMockContext() context.Context {
	mockJWTToken := "fake-jwt-token"
	req := httptest.NewRequest(http.MethodPost, "/test-endpoint", bytes.NewBufferString(`{"key": "value"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Form = map[string][]string{
		"example": {"testValue"},
	}

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = context.WithValue(ctx, shared.JwtKey, mockJWTToken)
	ctx = context.WithValue(ctx, shared.RequestBody, req.Body)
	ctx = context.WithValue(ctx, shared.FormData, req.Form)
	ctx = context.WithValue(ctx, shared.Request, req)
	ctx = context.WithValue(ctx, shared.ResponseWriter, w)
	return ctx
}
