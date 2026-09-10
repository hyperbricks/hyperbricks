package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestRenderContentPreparedRequestContext(t *testing.T) {
	for _, compiled := range []bool{true, false} {
		name := "compiled"
		if !compiled {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			fixture := setupSSRProofPipelineBenchmark(t)
			plain, _, _ := getConfigAndPlan("index")
			withAPI := shared.CloneMapDeep(plain)
			// This raw data is unused by the template, but still requires API context.
			withAPI["unused"] = map[string]interface{}{
				"data": []interface{}{map[string]interface{}{"@type": component.APIConfigGetName()}},
			}
			apiPlan, err := renderplan.Compile(rm, withAPI, parser.GetTemplate)
			if err != nil {
				t.Fatalf("compile route with unused API marker: %v", err)
			}
			if fixture.plan.NeedsAPIRequestContext() || !apiPlan.NeedsAPIRequestContext() {
				t.Fatal("fixture plans must have opposite API context requirements")
			}
			rawByFlag := map[bool]map[string]interface{}{false: plain, true: withAPI}
			planByFlag := map[bool]*renderplan.Plan{false: fixture.plan, true: apiPlan}
			for _, tc := range []struct {
				name     string
				route    string
				needsAPI bool
			}{
				{"plain", "index", false},
				{"replacement_with_api", "index", true},
				{"replacement_without_api", "index", false},
				{"404_with_api", "missing", true},
				{"404_without_api", "missing", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					indexAPI, fallbackAPI := tc.needsAPI, !tc.needsAPI
					wantStatus := http.StatusOK
					if tc.route == "missing" {
						indexAPI, fallbackAPI = !tc.needsAPI, tc.needsAPI
						wantStatus = http.StatusNotFound
					}
					plans := make(map[string]*renderplan.Plan)
					if compiled {
						plans["index"], plans["404"] = planByFlag[indexAPI], planByFlag[fallbackAPI]
					}
					updateGlobalRoutes(map[string]map[string]interface{}{
						"index": rawByFlag[indexAPI], "404": rawByFlag[fallbackAPI],
					}, plans)

					const payload = "field=first&field=second"
					body := &preparedContextBody{Reader: strings.NewReader(payload)}
					request := httptest.NewRequest(http.MethodPost, "/"+tc.route+"?rid=benchmark-request", nil)
					request.Body = body
					request.ContentLength = int64(len(payload))
					request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					result := renderContent(httptest.NewRecorder(), tc.route, request, "prepared-context")
					if result.Status != wantStatus || result.ErrorCount != 0 || result.Content != fixture.expected {
						t.Fatalf("status=%d errors=%d output matches=%t", result.Status, result.ErrorCount, result.Content == fixture.expected)
					}
					if tc.needsAPI {
						if body.reads == 0 || body.closes != 1 || request.Body == body {
							t.Fatalf("body was not cloned: reads=%d closes=%d replaced=%t", body.reads, body.closes, request.Body != body)
						}
						if !reflect.DeepEqual(request.PostForm["field"], []string{"first", "second"}) || request.Form.Get("rid") != "benchmark-request" {
							t.Fatalf("form data was not preserved: PostForm=%v Form=%v", request.PostForm, request.Form)
						}
					} else if body.reads != 0 || body.closes != 0 || request.Body != body || request.Form != nil || request.PostForm != nil {
						t.Fatalf("plain route touched request body/form: reads=%d closes=%d Form=%v PostForm=%v", body.reads, body.closes, request.Form, request.PostForm)
					}
				})
			}
		})
	}
}

type preparedContextBody struct {
	*strings.Reader
	reads  int
	closes int
}

func (b *preparedContextBody) Read(p []byte) (int, error) {
	b.reads++
	return b.Reader.Read(p)
}

func (b *preparedContextBody) Close() error {
	b.closes++
	return nil
}
