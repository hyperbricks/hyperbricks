package render_test

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

var pipelinePlanSink *profileCompiledPage

func BenchmarkSSRProofCompilePlan(b *testing.B) {
	raw := loadSSRProofConfig(b)
	cases := []struct {
		name    string
		options profileCompileOptions
	}{
		{name: "assemble-with-template-cache"},
		{
			name: "fresh-template-parse-and-assemble",
			options: profileCompileOptions{
				freshTemplateParse: true,
			},
		},
		{
			name: "fresh-parse-builder-and-static-folding",
			options: profileCompileOptions{
				foldStatic:         true,
				freshTemplateParse: true,
				useStringBuilder:   true,
			},
		},
	}

	for _, benchCase := range cases {
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				plan, err := compileProfilePageWithOptions(raw, benchCase.options)
				if err != nil {
					b.Fatalf("compile profile page: %v", err)
				}
				pipelinePlanSink = plan
			}
		})
	}
}

func BenchmarkSSRProofCompiledPlan(b *testing.B) {
	rm, _ := benchmarkSSRRenderManager()
	raw := loadSSRProofConfig(b)
	plan, err := compileProfilePage(raw)
	if err != nil {
		b.Fatalf("compile profile page: %v", err)
	}
	builderPlan, err := compileProfilePageWithOptions(raw, profileCompileOptions{useStringBuilder: true})
	if err != nil {
		b.Fatalf("compile builder profile page: %v", err)
	}
	foldedPlan, err := compileProfilePageWithOptions(raw, profileCompileOptions{
		foldStatic:       true,
		useStringBuilder: true,
	})
	if err != nil {
		b.Fatalf("compile folded profile page: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?rid=compiled-plan", nil)
	ctx := benchmarkRequestContext("compiled-plan")

	want, errs := rm.Render(composite.HyperMediaConfigGetName(), raw, ctx)
	if len(errs) != 0 {
		b.Fatalf("current render errors: %v", errs)
	}
	got, err := plan.Render(request)
	if err != nil {
		b.Fatalf("compiled render: %v", err)
	}
	if got != want {
		b.Fatalf("compiled output differs: got %d bytes, want %d", len(got), len(want))
	}
	assertCompiledProfileOutput(b, builderPlan, request, want)
	assertCompiledProfileOutput(b, foldedPlan, request, want)

	b.Run("current-map-pipeline", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for i := 0; i < b.N; i++ {
			output, renderErrs := rm.Render(composite.HyperMediaConfigGetName(), raw, ctx)
			if len(renderErrs) != 0 {
				b.Fatalf("render errors: %v", renderErrs)
			}
			pipelineOutputSink = output
		}
	})

	b.Run("compiled-immutable-plan", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for i := 0; i < b.N; i++ {
			output, renderErr := plan.Render(request)
			if renderErr != nil {
				b.Fatalf("render error: %v", renderErr)
			}
			pipelineOutputSink = output
		}
	})

	b.Run("compiled-string-builder", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for i := 0; i < b.N; i++ {
			output, renderErr := builderPlan.Render(request)
			if renderErr != nil {
				b.Fatalf("render error: %v", renderErr)
			}
			pipelineOutputSink = output
		}
	})

	b.Run("compiled-builder-with-static-folding", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for i := 0; i < b.N; i++ {
			output, renderErr := foldedPlan.Render(request)
			if renderErr != nil {
				b.Fatalf("render error: %v", renderErr)
			}
			pipelineOutputSink = output
		}
	})
}

func BenchmarkSSRProofCompiledPlanParallelRequests(b *testing.B) {
	rm, _ := benchmarkSSRRenderManager()
	raw := loadSSRProofConfig(b)
	plan, err := compileProfilePage(raw)
	if err != nil {
		b.Fatalf("compile profile page: %v", err)
	}
	builderPlan, err := compileProfilePageWithOptions(raw, profileCompileOptions{useStringBuilder: true})
	if err != nil {
		b.Fatalf("compile builder profile page: %v", err)
	}
	foldedPlan, err := compileProfilePageWithOptions(raw, profileCompileOptions{
		foldStatic:       true,
		useStringBuilder: true,
	})
	if err != nil {
		b.Fatalf("compile folded profile page: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?rid=compiled-parallel", nil)
	ctx := benchmarkRequestContext("compiled-parallel")

	want, errs := rm.Render(composite.HyperMediaConfigGetName(), raw, ctx)
	if len(errs) != 0 {
		b.Fatalf("current render errors: %v", errs)
	}
	got, err := plan.Render(request)
	if err != nil || got != want {
		b.Fatalf("compiled validation failed: output=%d bytes errors=%v", len(got), err)
	}
	assertCompiledProfileOutput(b, builderPlan, request, want)
	assertCompiledProfileOutput(b, foldedPlan, request, want)

	b.Run("current-map-pipeline", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				output, renderErrs := rm.Render(composite.HyperMediaConfigGetName(), raw, ctx)
				if len(renderErrs) != 0 || output == "" {
					b.Fatalf("render failed: output=%d bytes errors=%v", len(output), renderErrs)
				}
			}
		})
	})

	b.Run("compiled-immutable-plan", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				output, renderErr := plan.Render(request)
				if renderErr != nil || output == "" {
					b.Fatalf("render failed: output=%d bytes error=%v", len(output), renderErr)
				}
			}
		})
	})

	b.Run("compiled-string-builder", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				output, renderErr := builderPlan.Render(request)
				if renderErr != nil || output == "" {
					b.Fatalf("render failed: output=%d bytes error=%v", len(output), renderErr)
				}
			}
		})
	})

	b.Run("compiled-builder-with-static-folding", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				output, renderErr := foldedPlan.Render(request)
				if renderErr != nil || output == "" {
					b.Fatalf("render failed: output=%d bytes error=%v", len(output), renderErr)
				}
			}
		})
	})
}

func TestProfileCompiledPlanRequestIsolation(t *testing.T) {
	raw := loadSSRProofConfig(t)
	plan, err := compileProfilePageWithOptions(raw, profileCompileOptions{
		foldStatic:       true,
		useStringBuilder: true,
	})
	if err != nil {
		t.Fatalf("compile profile page: %v", err)
	}

	const requests = 256
	errCh := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		requestID := fmt.Sprintf("request-%03d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			request := httptest.NewRequest(http.MethodGet, "/?rid="+requestID, nil)
			output, renderErr := plan.Render(request)
			if renderErr != nil {
				errCh <- renderErr
				return
			}
			if strings.Count(output, requestID) != 3 {
				errCh <- fmt.Errorf("request %q appeared %d times", requestID, strings.Count(output, requestID))
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for renderErr := range errCh {
		t.Error(renderErr)
	}
}

type profileCompiledPage struct {
	root        profileCompiledNode
	queryKeys   []string
	queryKeySet map[string]struct{}
}

func (page *profileCompiledPage) Render(request *http.Request) (string, error) {
	params := make(map[string]interface{}, len(page.queryKeys))
	if request != nil && request.URL != nil {
		query := request.URL.Query()
		for key := range page.queryKeySet {
			values := query[key]
			switch len(values) {
			case 0:
			case 1:
				params[key] = values[0]
			default:
				params[key] = values
			}
		}
	}
	return page.root.Render(params)
}

type profileCompiledNode interface {
	Render(params map[string]interface{}) (string, error)
	Dynamic() bool
}

type profileCompiledTemplate struct {
	template         *template.Template
	values           []profileCompiledValue
	needsParams      bool
	useStringBuilder bool
}

type profileCompiledValue struct {
	key    string
	static interface{}
	child  profileCompiledNode
}

func (node *profileCompiledTemplate) Render(params map[string]interface{}) (string, error) {
	valueCount := len(node.values)
	if node.needsParams {
		valueCount++
	}
	var data map[string]interface{}
	if valueCount > 0 {
		data = make(map[string]interface{}, valueCount)
	}
	for _, value := range node.values {
		if value.child == nil {
			data[value.key] = value.static
			continue
		}
		rendered, err := value.child.Render(params)
		if err != nil {
			return "", err
		}
		data[value.key] = template.HTML(rendered)
	}
	if node.needsParams {
		data["Params"] = params
	}

	if node.useStringBuilder {
		var output strings.Builder
		if err := node.template.Execute(&output, data); err != nil {
			return "", err
		}
		return output.String(), nil
	}

	var output bytes.Buffer
	if err := node.template.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (node *profileCompiledTemplate) Dynamic() bool {
	if node.needsParams {
		return true
	}
	for _, value := range node.values {
		if value.child != nil && value.child.Dynamic() {
			return true
		}
	}
	return false
}

type profileCompiledTree struct {
	children []profileCompiledNode
}

func (node *profileCompiledTree) Render(params map[string]interface{}) (string, error) {
	var output strings.Builder
	for _, child := range node.children {
		rendered, err := child.Render(params)
		if err != nil {
			return "", err
		}
		output.WriteString(rendered)
	}
	return output.String(), nil
}

func (node *profileCompiledTree) Dynamic() bool {
	for _, child := range node.children {
		if child.Dynamic() {
			return true
		}
	}
	return false
}

type profileCompiledStatic struct {
	html string
}

func (node *profileCompiledStatic) Render(map[string]interface{}) (string, error) {
	return node.html, nil
}

func (node *profileCompiledStatic) Dynamic() bool {
	return false
}

type profileCompileOptions struct {
	foldStatic         bool
	freshTemplateParse bool
	useStringBuilder   bool
}

type profilePageCompiler struct {
	queryKeys map[string]struct{}
	options   profileCompileOptions
}

func compileProfilePage(raw map[string]interface{}) (*profileCompiledPage, error) {
	return compileProfilePageWithOptions(raw, profileCompileOptions{})
}

func compileProfilePageWithOptions(
	raw map[string]interface{},
	options profileCompileOptions,
) (*profileCompiledPage, error) {
	compiler := &profilePageCompiler{
		queryKeys: make(map[string]struct{}),
		options:   options,
	}
	root, err := compiler.compileNode(raw)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(compiler.queryKeys))
	for key := range compiler.queryKeys {
		keys = append(keys, key)
	}
	return &profileCompiledPage{
		root:        root,
		queryKeys:   keys,
		queryKeySet: compiler.queryKeys,
	}, nil
}

func (compiler *profilePageCompiler) compileNode(raw map[string]interface{}) (profileCompiledNode, error) {
	typeName, _ := raw["@type"].(string)
	switch typeName {
	case composite.HyperMediaConfigGetName():
		templateRaw, ok := raw["template"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("compiled profile only supports template-backed hypermedia")
		}
		return compiler.compileTemplate(templateRaw)
	case composite.TemplateConfigGetName():
		return compiler.compileTemplate(raw)
	case composite.TreeRendererConfigGetName():
		children := make([]profileCompiledNode, 0)
		for _, key := range profileOrderedTreeKeys(raw) {
			childRaw, ok := raw[key].(map[string]interface{})
			if !ok {
				continue
			}
			child, err := compiler.compileNode(childRaw)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		return compiler.maybeFold(&profileCompiledTree{children: children})
	default:
		return nil, fmt.Errorf("unsupported compiled profile node %q", typeName)
	}
}

func (compiler *profilePageCompiler) compileTemplate(raw map[string]interface{}) (profileCompiledNode, error) {
	inline, _ := raw["inline"].(string)
	if inline == "" {
		return nil, fmt.Errorf("compiled profile template has no inline source")
	}
	var parsed *template.Template
	var err error
	if compiler.options.freshTemplateParse {
		parsed, err = template.New("profile-compiled-template").Funcs(shared.GetGenericFuncMap()).Parse(inline)
	} else {
		parsed, err = shared.ParsedGenericTemplate(inline)
	}
	if err != nil {
		return nil, err
	}

	node := &profileCompiledTemplate{
		template:         parsed,
		useStringBuilder: compiler.options.useStringBuilder,
	}
	for _, key := range profileStringSlice(raw["querykeys"]) {
		compiler.queryKeys[key] = struct{}{}
		node.needsParams = true
	}
	values, _ := raw["values"].(map[string]interface{})
	for _, key := range shared.SortedUniqueKeys(values) {
		value := values[key]
		compiledValue := profileCompiledValue{key: key, static: value}
		if childRaw, ok := value.(map[string]interface{}); ok {
			if _, hasType := childRaw["@type"]; hasType {
				child, err := compiler.compileNode(childRaw)
				if err != nil {
					return nil, err
				}
				compiledValue.static = nil
				compiledValue.child = child
			}
		}
		node.values = append(node.values, compiledValue)
	}
	return compiler.maybeFold(node)
}

func (compiler *profilePageCompiler) maybeFold(node profileCompiledNode) (profileCompiledNode, error) {
	if !compiler.options.foldStatic || node.Dynamic() {
		return node, nil
	}
	rendered, err := node.Render(nil)
	if err != nil {
		return nil, err
	}
	return &profileCompiledStatic{html: rendered}, nil
}

func assertCompiledProfileOutput(
	b *testing.B,
	plan *profileCompiledPage,
	request *http.Request,
	want string,
) {
	b.Helper()
	got, err := plan.Render(request)
	if err != nil || got != want {
		b.Fatalf("compiled validation failed: output=%d bytes errors=%v", len(got), err)
	}
}

func profileStringSlice(raw interface{}) []string {
	switch values := raw.(type) {
	case []string:
		return values
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
