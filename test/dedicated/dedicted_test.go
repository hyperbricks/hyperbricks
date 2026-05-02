package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// ParsedContent holds the separated sections and optional scope after parsing.
type ParsedContent struct {
	HyperbricksConfig      string
	HyperbricksConfigScope string
	Explainer              string
	ExpectedJSON           map[string]interface{}
	ExpectedJSONAsString   string
	ExpectedOutput         string
	MoreDetails            string
}

var directory = flag.String("directory", "./api-tests/", "Directory to use")

// TestMain is the entry point for testing; it must call flag.Parse()
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
func Test_All_Dedicated_Tests(t *testing.T) {
	startDedicatedAPIFixtures(t)

	// Initialize shared configuration settings.
	shared.Init_configuration()
	conf := shared.GetHyperBricksConfiguration()

	fmt.Printf("%v", conf)
	// Create a new RenderManager instance and register the FragmentRenderer.
	rm := render.NewRenderManager()

	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"example":      "<div>.main_section}}</div>",
			"header":       "<h1>{{.title}}</h1>",
			"youtube.tmpl": `<iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>`,
		}
		content, exists := templates[templateName]
		return content, exists
	}

	// This instanciating of ImageProcessorInstance gives some flexibility for testing
	singleImageRenderer := &component.SingleImageRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}

	multipleImagesRenderer := &component.MultipleImagesRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}

	// Register standard renderers using static-like functions
	rm.RegisterComponent(component.SingleImageConfigGetName(), singleImageRenderer, reflect.TypeOf(component.SingleImageConfig{}))
	rm.RegisterComponent(component.MultipleImagesConfigGetName(), multipleImagesRenderer, reflect.TypeOf(component.MultipleImagesConfig{}))

	// TEMPLATE ....
	pluginRenderer := &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(component.PluginRenderGetName(), pluginRenderer, reflect.TypeOf(component.PluginConfig{}))

	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))

	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))

	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	// Register legacy and new JS component names for compatibility
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))
	rm.RegisterComponent(component.JSConfigGetName(), &component.JSRenderer{}, reflect.TypeOf(component.JSConfig{}))

	//Register Template Menu Renderer
	menuRenderer := &component.MenuRenderer{
		TemplateProvider: templateProvider,
	}
	rm.RegisterComponent(component.MenuConfigGetName(), menuRenderer, reflect.TypeOf(component.MenuConfig{}))

	// Register Local JSON Renderer
	localJsonRenderer := &component.LocalJSONRenderer{
		TemplateProvider: templateProvider,
	}
	rm.RegisterComponent(component.LocalJSONConfigGetName(), localJsonRenderer, reflect.TypeOf(component.LocalJSONConfig{}))

	// TEMPLATE ....
	fragmentRenderer := &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.FragmentConfigGetName(), fragmentRenderer, reflect.TypeOf(composite.FragmentConfig{}))

	apiFragmentRenderer := &composite.ApiFragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.ApiFragmentRenderConfigGetName(), apiFragmentRenderer, reflect.TypeOf(composite.ApiFragmentRenderConfig{}))

	// TEMPLATE ....
	hypermediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.HyperMediaConfigGetName(), hypermediaRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	// TEMPLATE ....
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	// API ....
	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	// COMPONENTS
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	rm.GetRenderComponent(composite.TemplateConfigGetName()).(*composite.TemplateRenderer).TemplateProvider = templateProvider

	temp := make(map[string][]composite.HyperMediaConfig)

	// Add a slices for test....
	temp["demo_main_menu"] = []composite.HyperMediaConfig{
		{
			Title:   "DOCUMENT_1",
			Route:   "doc1",
			Section: "demo_main_menu",
		},
		{
			Title:   "DOCUMENT_2",
			Route:   "doc2",
			Section: "demo_main_menu",
		},
		{
			Title:   "DOCUMENT_3",
			Route:   "doc3",
			Section: "demo_main_menu",
		},
	}
	rm.GetRenderComponent(component.MenuConfigGetName()).(*component.MenuRenderer).HyperMediasBySection = temp

	fmt.Printf("Processing directory: %s\n", *directory)
	err := filepath.Walk(*directory, func(path string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".hyperbricks") {
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()

			var content string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				content += scanner.Text() + "\n" // Collect file content
			}

			// TODO: Process `content` for this file separately
			//fmt.Printf("Processing file: %s\n", path)
			//fmt.Println(content) // Replace this with your processing logic

			// PARSE HYPERSCRIPT
			// RUN THE TEST (compare json with serialized output go object)
			t.Run(path, func(t *testing.T) {

				parsed, err := ParseContent(content)
				if err != nil {
					fmt.Println("Error:", err)
					return
				}
				parsed.HyperbricksConfig = normalizeDedicatedFixtureURLs(parsed.HyperbricksConfig)

				//fmt.Println("Hyperbricks Config:")
				//fmt.Println(parsed.HyperbricksConfig)

				//fmt.Println("\nExplainer:")
				//fmt.Println(parsed.Explainer)

				//fmt.Println("\nExpected JSON (Non-Escaped):")

				var buf bytes.Buffer
				encoder := json.NewEncoder(&buf)
				encoder.SetEscapeHTML(false) // Disable HTML escaping
				encoder.SetIndent("", "  ")  // Enable pretty printing with indentation
				if err := encoder.Encode(parsed.ExpectedJSON); err != nil {
					fmt.Println("Error encoding JSON:", err)
					return
				}
				//fmt.Print(buf.String())
				var expected map[string]interface{}

				// Convert JSON string to bytes and unmarshal into the map
				if err := json.Unmarshal([]byte(buf.String()), &expected); err != nil {
					fmt.Println("Error unmarshaling JSON:", err)
					return
				}
				//fmt.Printf("JSON string: %s", parsed.ExpectedJSONAsString)
				//fmt.Printf("converted JSON object: %v", expected)

				//fmt.Println("\nExpected Output:")
				//fmt.Println(parsed.ExpectedOutput)
				// Parse the combined configuration.
				preprocesses, _ := parser.PreprocessHyperScript(parsed.HyperbricksConfig)
				parsedConfig := parser.ParseHyperScript(preprocesses)
				//fmt.Printf("got obj from hyperscript:%v", parsedConfig)
				// Convert the struct to JSON
				// jsonBytes, err := json.MarshalIndent(parsedConfig[parsed.HyperbricksConfigScope], "", "  ")
				// if err != nil {
				// 	fmt.Println("Error marshaling struct to JSON:", err)
				// 	return
				// }
				// fmt.Printf("Hyperscript object:%s", string(jsonBytes))

				// Prepare a variable of type map[string]interface{}
				// Create a TypeRequest for the TypeFactory.

				// Extract the relevant scope data.
				scopeData, ok := parsedConfig[parsed.HyperbricksConfigScope].(map[string]interface{})
				if !ok {
					t.Errorf("Scope '%s' not found or invalid type", parsed.HyperbricksConfigScope)
					return
				}

				request := typefactory.TypeRequest{
					TypeName: scopeData["@type"].(string),
					Data:     scopeData,
				}

				//Use the RenderManager to instantiate the configuration.
				response, err := rm.MakeInstance(request)
				if err != nil {
					t.Errorf("Error creating instance: %v", err)
					return
				}

				ctx := createMockContext()
				result, errr := rm.Render(request.TypeName, scopeData, ctx)
				if errr != nil {
					log.Printf("%v", errr)
				}

				// fmt.Printf("\nrendered result:%s", result)
				// fmt.Printf("\nexpected result:%s", parsed.ExpectedOutput)

				// TO COMPARE THIS....
				_res_html := stripAllWhitespace(result)
				_exp_html := stripAllWhitespace(parsed.ExpectedOutput)

				// Now compare the normalized strings if expected is given....
				if _res_html != _exp_html && parsed.ExpectedOutput != "" {
					t.Errorf("result and expected html output does not match: \nresult:\n%s \nexpected:\n%s\n", result, parsed.ExpectedOutput)
				}

				if parsed.ExpectedJSONAsString != "" {

					equal, err := JSONDeepEqual(expected, response.Instance)
					if err != nil {

						t.Fatalf("Error comparing JSON for %s: %v", strings.ToLower(path), err)
					}
					if !equal {

						t.Errorf("Test failed for %s!\n", strings.ToLower(path))
						outputJSON := convertToJSON(response.Instance)
						expectedJSON := convertToJSON(expected)

						fmt.Printf("output:\n%s\n", outputJSON)
						fmt.Printf("expected:\n%s\n", expectedJSON)

					} else {

					}
				}

			})

		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
	}
}

func HyperBrickInitialisation() {

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
		"error":  errorString(err),
	})
}

func writeDedicatedJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// stripAllWhitespace removes all whitespace characters from the input string.
func stripAllWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

func createMockContext() context.Context {
	// Mock JWT token
	mockJwtToken := "fake-jwt-token"

	// Add query parameters
	queryParams := url.Values{}
	queryParams.Add("example", "testValue")
	queryParams.Add("otherexample", "otherTestValue") // You can add more if needed

	// Create a fake HTTP request
	// Mock JSON body
	jsonBody := []byte(`{"user_password": "mysupersecretpassword"}`)
	req := httptest.NewRequest(http.MethodPost, "/test-endpoint?"+queryParams.Encode(), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Form = map[string][]string{
		"example": {"testValue"},
	}

	// Assign query parameters to request
	req.URL.RawQuery = queryParams.Encode()

	fmt.Println("RawQuery:", req.URL.RawQuery)

	// Create a fake ResponseWriter
	w := httptest.NewRecorder()

	// Create a base context
	ctx := context.Background()
	ctx = context.WithValue(ctx, shared.JwtKey, mockJwtToken)
	ctx = context.WithValue(ctx, shared.RequestBody, req.Body)
	ctx = context.WithValue(ctx, shared.FormData, req.Form)
	ctx = context.WithValue(ctx, shared.Request, req)
	ctx = context.WithValue(ctx, shared.ResponseWriter, w)

	return ctx
}

// Helper function to normalize and compare two interface{} values
func JSONDeepEqual(a, b interface{}) (bool, error) {
	// Serialize both values to JSON
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false, err
	}

	// Deserialize JSON bytes back into interface{}
	var aJSON interface{}
	var bJSON interface{}

	if err := json.Unmarshal(aBytes, &aJSON); err != nil {
		return false, err
	}
	if err := json.Unmarshal(bBytes, &bJSON); err != nil {
		return false, err
	}

	// Use DeepEqual on the normalized data
	return reflect.DeepEqual(aJSON, bJSON), nil
}

func convertToJSON(obj interface{}) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // Disable escaping of HTML characters
	encoder.SetIndent("", "  ")  // Set indent similar to MarshalIndent

	//if err := encoder.Encode(parsedConfig[parsed.HyperbricksConfigScope]); err != nil {
	if err := encoder.Encode(obj); err != nil {
		fmt.Println("Error encoding to JSON:", err)
		return ""
	}

	// The encoder adds a newline at the end of the output; trim if needed
	return buf.String()
}

// ParseContent parses the provided content string into its respective parts.
// It also extracts an optional scope from the "hyperbricks config" header.
func ParseContent(content string) (*ParsedContent, error) {
	// Regular expression to match section headers like:
	// ==== hyperbricks config {!{fragment}} ====
	// It captures the header title and an optional scope.
	headerRegex := regexp.MustCompile(`^====\s*([^!]+?)(?:\s*\{\!\{([^}]+)\}\})?\s*====$`)

	sections := make(map[string]string)
	var currentSection string
	var sb strings.Builder

	// Variable to store the scope for "hyperbricks config" if found.
	var hyperbricksConfigScope string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Do not trim spaces or tabs here to preserve formatting.
		matches := headerRegex.FindStringSubmatch(line)
		if matches != nil {
			// When encountering a new header, save the current section's content.
			if currentSection != "" {
				sections[strings.ToLower(currentSection)] = sb.String()
				sb.Reset()
			}
			// matches[1] contains the header title.
			currentSection = strings.TrimSpace(matches[1])

			// If a scope was provided, matches[2] will contain it.
			scope := ""
			if len(matches) >= 3 {
				scope = strings.TrimSpace(matches[2])
			}

			// Specifically store scope for "hyperbricks config" header.
			if strings.EqualFold(currentSection, "hyperbricks config") {
				hyperbricksConfigScope = scope
			}
		} else {
			if currentSection != "" {
				sb.WriteString(line)
				sb.WriteString("\n") // Preserve newlines for formatting.
			}
		}
	}
	if currentSection != "" {
		sections[strings.ToLower(currentSection)] = sb.String()
	}

	hyperbricksConfig := sections["hyperbricks config"]
	explainer := sections["explainer"]
	expectedJSONStr := sections["expected json"]
	expectedOutput := sections["expected output"]

	var moreDetails string = ""
	val, ok := sections["more details"]
	if ok {
		moreDetails = val
	}
	var expectedJSON map[string]interface{}
	if expectedJSONStr != "" {
		if err := json.Unmarshal([]byte(expectedJSONStr), &expectedJSON); err != nil {
			return nil, fmt.Errorf("error parsing expected JSON: %v", err)
		}
	}

	return &ParsedContent{
		HyperbricksConfig:      hyperbricksConfig,
		HyperbricksConfigScope: hyperbricksConfigScope,
		Explainer:              explainer,
		ExpectedJSON:           expectedJSON,
		ExpectedJSONAsString:   sections["expected json"],
		ExpectedOutput:         expectedOutput,
		MoreDetails:            moreDetails,
	}, nil
}
