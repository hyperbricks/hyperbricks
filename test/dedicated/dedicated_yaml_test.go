package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

var dedicatedYAMLDirectory = flag.String("directory", "./yaml-api-tests/", "Directory with .hyperbricks.yaml.test dedicated fixtures")

type dedicatedYAMLCase struct {
	Source            string
	Scope             string
	Explainer         string
	ExpectedJSON      string
	ExpectedOutput    string
	HasExpectedOutput bool
}

func Test_All_Dedicated_YAML_Tests(t *testing.T) {
	startDedicatedAPIFixtures(t)

	rm := newDedicatedYAMLRenderManager(t)
	count := 0
	err := filepath.WalkDir(*dedicatedYAMLDirectory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".hyperbricks.yaml.test") {
			return nil
		}

		count++
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			testCase, err := parseDedicatedYAMLContent(string(raw))
			if err != nil {
				t.Fatalf("parse fixture sections: %v", err)
			}
			testCase.Source = normalizeDedicatedFixtureURLs(testCase.Source)

			result, err := yamlparser.ProcessBytes([]byte(testCase.Source), dedicatedYAMLOptions(path))
			if err != nil {
				t.Fatalf("process YAML fixture: %v", err)
			}

			scopeData, ok := result.Materialized[testCase.Scope].(map[string]interface{})
			if !ok {
				t.Fatalf("scope %q not found or invalid type", testCase.Scope)
			}
			typeName, ok := scopeData["@type"].(string)
			if !ok || typeName == "" {
				t.Fatalf("scope %q has invalid @type: %#v", testCase.Scope, scopeData["@type"])
			}

			response, err := rm.MakeInstance(typefactory.TypeRequest{
				TypeName: typeName,
				Data:     scopeData,
			})
			if err != nil {
				t.Fatalf("create instance: %v", err)
			}

			if strings.TrimSpace(testCase.ExpectedJSON) != "" {
				var expected interface{}
				if err := json.Unmarshal([]byte(testCase.ExpectedJSON), &expected); err != nil {
					t.Fatalf("parse expected JSON: %v", err)
				}
				equal, err := dedicatedJSONDeepEqual(expected, response.Instance)
				if err != nil {
					t.Fatalf("compare expected JSON: %v", err)
				}
				if !equal {
					t.Fatalf("instance JSON mismatch\n--- got ---\n%s\n--- want ---\n%s", dedicatedConvertToJSON(response.Instance), dedicatedConvertToJSON(expected))
				}
			}

			output, renderErrors := rm.Render(typeName, scopeData, createDedicatedMockContext())
			if len(renderErrors) > 0 {
				t.Fatalf("render %s returned errors: %v", testCase.Scope, renderErrors)
			}
			if testCase.HasExpectedOutput && stripAllWhitespace(output) != stripAllWhitespace(testCase.ExpectedOutput) {
				t.Fatalf("rendered output mismatch\n--- got ---\n%s\n--- want ---\n%s", output, testCase.ExpectedOutput)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan dedicated YAML fixtures: %v", err)
	}
	if count == 0 {
		t.Fatalf("no dedicated YAML fixtures found in %s", *dedicatedYAMLDirectory)
	}
}

func dedicatedYAMLOptions(path string) yamlparser.Options {
	fixtureDir := filepath.Dir(path)
	return yamlparser.Options{
		Paths: yamlparser.PathMarkers{
			Resources: filepath.Join(fixtureDir, "assets"),
			Templates: filepath.Join(fixtureDir, "templates"),
		},
		RecoverDuplicateChildren: true,
	}
}

func newDedicatedYAMLRenderManager(t *testing.T) *render.RenderManager {
	t.Helper()

	shared.Init_configuration()
	conf := shared.GetHyperBricksConfiguration()
	testOutputRoot := t.TempDir()
	conf.Directories["static"] = filepath.Join(testOutputRoot, "static")
	conf.Directories["render"] = filepath.Join(testOutputRoot, "rendered")

	rm := render.NewRenderManager()
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"example":         "<div>.main_section}}</div>",
			"header":          "<h1>{{.title}}</h1>",
			"youtube.tmpl":    `<iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>`,
			"cards/card.html": `<article><h2>{{.title}}</h2>{{if .lead}}<p>{{.lead}}</p>{{end}}{{if .body}}<p>{{.body}}</p>{{end}}</article>`,
		}
		content, exists := templates[templateName]
		return content, exists
	}

	rm.RegisterComponent(component.SingleImageConfigGetName(), &component.SingleImageRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}, reflect.TypeOf(component.SingleImageConfig{}))
	rm.RegisterComponent(component.MultipleImagesConfigGetName(), &component.MultipleImagesRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}, reflect.TypeOf(component.MultipleImagesConfig{}))
	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))
	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))
	rm.RegisterComponent(component.JSConfigGetName(), &component.JSRenderer{}, reflect.TypeOf(component.JSConfig{}))
	rm.RegisterComponent(component.LocalJSONConfigGetName(), &component.LocalJSONRenderer{
		TemplateProvider: templateProvider,
	}, reflect.TypeOf(component.LocalJSONConfig{}))
	rm.RegisterComponent(component.APIConfigGetName(), &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}, reflect.TypeOf(component.APIConfig{}))
	rm.RegisterComponent(component.MenuConfigGetName(), &component.MenuRenderer{
		TemplateProvider: templateProvider,
		HyperMediasBySection: map[string][]composite.HyperMediaConfig{
			"demo_main_menu": {
				{Title: "DOCUMENT_1", Route: "doc1", Section: "demo_main_menu"},
				{Title: "DOCUMENT_2", Route: "doc2", Section: "demo_main_menu"},
				{Title: "DOCUMENT_3", Route: "doc3", Section: "demo_main_menu"},
			},
		},
	}, reflect.TypeOf(component.MenuConfig{}))
	rm.RegisterComponent(component.PluginRenderGetName(), &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}, reflect.TypeOf(component.PluginConfig{}))

	rm.RegisterComponent(composite.FragmentConfigGetName(), &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}, reflect.TypeOf(composite.FragmentConfig{}))
	rm.RegisterComponent(composite.ApiFragmentRenderConfigGetName(), &composite.ApiFragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}, reflect.TypeOf(composite.ApiFragmentRenderConfig{}))
	rm.RegisterComponent(composite.HyperMediaConfigGetName(), &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}, reflect.TypeOf(composite.HyperMediaConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}, reflect.TypeOf(composite.TreeConfig{}))
	rm.RegisterComponent(composite.TemplateConfigGetName(), &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}, reflect.TypeOf(composite.TemplateConfig{}))
	rm.RegisterComponent(composite.HeadConfigGetName(), &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}, reflect.TypeOf(composite.HeadConfig{}))

	return rm
}

func parseDedicatedYAMLContent(content string) (dedicatedYAMLCase, error) {
	headerRegex := regexp.MustCompile(`^====\s*([^!]+?)(?:\s*\{\!\{([^}]+)\}\})?\s*====$`)
	sections := make(map[string]string)
	sectionExists := make(map[string]bool)
	currentSection := ""
	scope := ""
	var sb strings.Builder

	for _, line := range strings.Split(content, "\n") {
		matches := headerRegex.FindStringSubmatch(line)
		if matches != nil {
			if currentSection != "" {
				sectionKey := strings.ToLower(currentSection)
				sections[sectionKey] = sb.String()
				sectionExists[sectionKey] = true
				sb.Reset()
			}
			currentSection = strings.TrimSpace(matches[1])
			if strings.EqualFold(currentSection, "hyperbricks yaml") && len(matches) >= 3 {
				scope = strings.TrimSpace(matches[2])
			}
			continue
		}
		if currentSection != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	if currentSection != "" {
		sectionKey := strings.ToLower(currentSection)
		sections[sectionKey] = sb.String()
		sectionExists[sectionKey] = true
	}
	if strings.TrimSpace(sections["hyperbricks yaml"]) == "" {
		return dedicatedYAMLCase{}, fmt.Errorf("missing hyperbricks yaml section")
	}
	if scope == "" {
		return dedicatedYAMLCase{}, fmt.Errorf("missing hyperbricks yaml scope")
	}
	return dedicatedYAMLCase{
		Source:            sections["hyperbricks yaml"],
		Scope:             scope,
		Explainer:         sections["explainer"],
		ExpectedJSON:      sections["expected json"],
		ExpectedOutput:    sections["expected output"],
		HasExpectedOutput: sectionExists["expected output"],
	}, nil
}

func normalizeDedicatedFixtureURLs(config string) string {
	replacer := strings.NewReplacer(
		"http://localhost:8090", "http://127.0.0.1:8090",
		"http://localhost:3000", "http://127.0.0.1:3000",
	)
	return replacer.Replace(config)
}

const dedicatedJWTSecret = "a-string-secret-at-least-256-bits-long"

func startDedicatedAPIFixtures(t *testing.T) {
	t.Helper()

	startDedicatedServer(t, ":8090", dedicatedEchoMux())
	startDedicatedServer(t, ":3000", dedicatedPostgRESTMux())
}

func startDedicatedServer(t *testing.T, address string, handler http.Handler) {
	t.Helper()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		if dedicatedFixtureAvailable(address) {
			t.Logf("using existing dedicated fixture server on %s", address)
			return
		}
		t.Fatalf("failed to start dedicated fixture server on %s: %v", address, err)
	}

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Errorf("dedicated fixture server on %s failed: %v", address, err)
		}
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("failed to shut down dedicated fixture server on %s: %v", address, err)
		}
	})
}

func dedicatedFixtureAvailable(address string) bool {
	baseURL := dedicatedFixtureBaseURL(address)
	if baseURL == "" {
		return false
	}

	switch address {
	case ":8090":
		resp, err := http.Get(baseURL + "/echo/query?code=fixture")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var payload struct {
			QueryParams map[string]interface{} `json:"queryParams"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return false
		}
		return payload.QueryParams["code"] == "fixture"
	case ":3000":
		req, err := http.NewRequest(http.MethodGet, baseURL+"/tasks", nil)
		if err != nil {
			return false
		}
		req.Header.Set("Authorization", "Bearer dedicated-fixture-probe")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode < http.StatusInternalServerError
	default:
		return false
	}
}

func dedicatedFixtureBaseURL(address string) string {
	if strings.HasPrefix(address, ":") {
		return "http://127.0.0.1" + address
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func dedicatedEchoMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", dedicatedValidateToken)
	mux.HandleFunc("/validate/body", dedicatedValidateBody)
	mux.HandleFunc("/echo/query", dedicatedEchoQuery)
	mux.HandleFunc("/echo/data", dedicatedEchoData)
	mux.HandleFunc("/echo/token/validate", dedicatedEchoTokenValidation)
	return mux
}

func dedicatedPostgRESTMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/create_user", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/rpc/login_user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"dedicated-test-token"`))
	})
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1,"title":"Dit is een test"}`))
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":1,"title":"Dit is een test"}]`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func dedicatedValidateToken(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "Bearer 12345abcdef" {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"Token is valid"}`))
}

func dedicatedValidateBody(w http.ResponseWriter, r *http.Request) {
	var bodyData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bodyData); err != nil {
		bodyData = map[string]interface{}{}
	}
	if bodyData["password"] != "mysupersecretpassword" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Token is not valid"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"Token is valid"}`))
}

func dedicatedEchoQuery(w http.ResponseWriter, r *http.Request) {
	paramsMap := make(map[string]interface{})
	for key, values := range r.URL.Query() {
		if len(values) == 1 {
			paramsMap[key] = values[0]
		} else {
			paramsMap[key] = values
		}
	}
	response := map[string]interface{}{
		"queryParams": paramsMap,
		"valid":       len(paramsMap) > 0,
		"message":     "",
	}
	if len(paramsMap) == 0 {
		response["message"] = "No query parameters provided"
	}
	writeDedicatedJSON(w, http.StatusOK, response)
}

func dedicatedEchoData(w http.ResponseWriter, r *http.Request) {
	var bodyData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bodyData); err != nil {
		http.Error(w, `{"message":"Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}
	writeDedicatedJSON(w, http.StatusOK, bodyData)
}

func dedicatedEchoTokenValidation(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(dedicatedJWTSecret), nil
	})
	writeDedicatedJSON(w, http.StatusOK, map[string]interface{}{
		"token":  tokenString,
		"valid":  err == nil && token.Valid,
		"claims": claims,
		"error":  dedicatedErrorString(err),
	})
}

func writeDedicatedJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func dedicatedErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func stripAllWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

func createDedicatedMockContext() context.Context {
	mockJWTToken := "fake-jwt-token"
	queryParams := url.Values{}
	queryParams.Add("example", "testValue")
	queryParams.Add("otherexample", "otherTestValue")

	jsonBody := []byte(`{"user_password": "mysupersecretpassword"}`)
	req := httptest.NewRequest(http.MethodPost, "/test-endpoint?"+queryParams.Encode(), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Form = map[string][]string{
		"example": {"testValue"},
	}
	req.URL.RawQuery = queryParams.Encode()

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = context.WithValue(ctx, shared.JwtKey, mockJWTToken)
	ctx = context.WithValue(ctx, shared.RequestBody, req.Body)
	ctx = context.WithValue(ctx, shared.FormData, req.Form)
	ctx = context.WithValue(ctx, shared.Request, req)
	ctx = context.WithValue(ctx, shared.ResponseWriter, w)
	return ctx
}

func dedicatedJSONDeepEqual(a, b interface{}) (bool, error) {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false, err
	}

	var aJSON interface{}
	var bJSON interface{}
	if err := json.Unmarshal(aBytes, &aJSON); err != nil {
		return false, err
	}
	if err := json.Unmarshal(bBytes, &bJSON); err != nil {
		return false, err
	}
	return reflect.DeepEqual(aJSON, bJSON), nil
}

func dedicatedConvertToJSON(obj interface{}) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(obj); err != nil {
		return ""
	}
	return buf.String()
}
