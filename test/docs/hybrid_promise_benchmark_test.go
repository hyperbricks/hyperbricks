package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

const hybridPromiseSharedYAML = `
vars:
  product:
    name: HyperBricks Bench
    status: fast

base_shell:
  - type: hypermedia
  - section: bench
  - head:
      - type: head
      - title: Hybrid benchmark
  - body:
      - type: tree
      - hero:
          - type: template
          - inline: |
              <section class="hero">
                <h1>{{ .title }}</h1>
                <p>{{ .summary }}</p>
              </section>
          - values:
              title:
                var: product.name
              summary:
                format: "%s server-rendered app shell"
                args:
                  - var: product.status

base_card:
  - type: template
  - inline: |
      <article class="card" data-kind="{{ .kind }}">
        <h2>{{ .title }}</h2>
        <p>{{ .body }}</p>
      </article>
  - values:
      kind: base
      title: Base card
      body: Reusable card body
`

const hybridPromiseAppYAML = `
imports:
  - shared.hyperbricks.yaml

dashboard:
  - inherit: base_shell
  - route: bench
  - title: Hybrid Benchmark
  - guard:
      enabled: true
      require:
        authenticated: true
      on_unauthenticated:
        default:
          status: 401
  - body:
      - type: tree
      - hero:
          - inherit: base_shell.body.hero
      - stats:
          - type: tree
          - latency:
              - inherit: base_card
              - values:
                  kind: metric
                  title: Latency
                  body: Server-rendered fragment updates
          - setup:
              - inherit: base_card
              - values:
                  kind: metric
                  title: Setup
                  body: YAML module with imports and inheritance
          - flexibility:
              - type: template
              - inline: |
                  <section id="flexibility">
                    {{ range .cards }}<span data-card="{{ . }}">{{ . }}</span>{{ end }}
                  </section>
              - values:
                  cards:
                    - route
                    - template
                    - fragment
                    - guard
                    - plugin

metric_fragment:
  - type: fragment
  - route: fragments/metric
  - response:
      headers:
        HX-Retarget: "#metric"
        HX-Reswap: outerHTML
        HX-Trigger-After-Swap: metric-updated
  - body:
      - type: template
      - inline: |
          <div id="metric" data-source="{{ .source }}">{{ .label }}: {{ .value }}</div>
      - values:
          source: htmx
          label: Requests
          value: 42
`

const hybridPromiseChangedYAML = `
imports:
  - shared.hyperbricks.yaml

dashboard:
  - inherit: base_shell
  - route: bench
  - title: Hybrid Benchmark
  - guard:
      enabled: true
      require:
        authenticated: true
      on_unauthenticated:
        default:
          status: 401
  - body:
      - type: tree
      - hero:
          - inherit: base_shell.body.hero
      - stats:
          - type: tree
          - latency:
              - inherit: base_card
              - values:
                  kind: metric
                  title: Latency
                  body: Server-rendered fragment updates
          - setup:
              - inherit: base_card
              - values:
                  kind: metric
                  title: Setup
                  body: YAML module with imports and inheritance
          - flexibility:
              - type: template
              - inline: |
                  <section id="flexibility">
                    {{ range .cards }}<span data-card="{{ . }}">{{ . }}</span>{{ end }}
                  </section>
              - values:
                  cards:
                    - route
                    - template
                    - fragment
                    - guard
                    - plugin
                    - static

metric_fragment:
  - type: fragment
  - route: fragments/metric
  - response:
      headers:
        HX-Retarget: "#metric"
        HX-Reswap: outerHTML
        HX-Trigger-After-Swap: metric-updated
  - body:
      - type: template
      - inline: |
          <div id="metric" data-source="{{ .source }}">{{ .label }}: {{ .value }}</div>
      - values:
          source: htmx
          label: Requests
          value: 42
`

type hybridPromiseFixture struct {
	dir    string
	app    string
	shared string
}

type hybridPromiseMetrics struct {
	SourceLOC               int      `json:"source_loc"`
	ChangedSourceLOC        int      `json:"changed_source_loc"`
	ChangedSemanticLines    int      `json:"changed_semantic_lines"`
	RuntimeObjects          int      `json:"runtime_objects"`
	RouteOwners             int      `json:"route_owners"`
	FlexibilityCapabilities []string `json:"flexibility_capabilities"`
}

func TestHybridPromiseMetricsAreQuantifiable(t *testing.T) {
	fixture := writeHybridPromiseFixture(t, hybridPromiseAppYAML)
	result := processHybridPromiseFile(t, fixture.app)
	changedFixture := writeHybridPromiseFixture(t, hybridPromiseChangedYAML)
	changedResult := processHybridPromiseFile(t, changedFixture.app)

	rm := newYAMLProfileRenderManager(t)
	page := mustHybridScope(t, result, "dashboard")
	fragment := mustHybridScope(t, result, "metric_fragment")

	pageOutput := renderHybridScope(t, rm, page)
	if !strings.Contains(pageOutput, `data-card="plugin"`) {
		t.Fatalf("page output does not include composed flexibility cards: %s", pageOutput)
	}
	fragmentOutput := renderHybridScope(t, rm, fragment)
	if !strings.Contains(fragmentOutput, `id="metric"`) {
		t.Fatalf("fragment output missing HTMX target body: %s", fragmentOutput)
	}

	changedPageOutput := renderHybridScope(t, rm, mustHybridScope(t, changedResult, "dashboard"))
	if !strings.Contains(changedPageOutput, `data-card="static"`) {
		t.Fatalf("changed page output did not materialize low-line-count flexibility change: %s", changedPageOutput)
	}

	metrics := hybridPromiseMetrics{
		SourceLOC:               countNonBlankLines(hybridPromiseSharedYAML) + countNonBlankLines(hybridPromiseAppYAML),
		ChangedSourceLOC:        countNonBlankLines(hybridPromiseSharedYAML) + countNonBlankLines(hybridPromiseChangedYAML),
		ChangedSemanticLines:    changedSemanticLines(hybridPromiseAppYAML, hybridPromiseChangedYAML),
		RuntimeObjects:          len(result.Materialized),
		RouteOwners:             countHybridRouteOwners(t, result),
		FlexibilityCapabilities: hybridFlexibilityCapabilities(t, result),
	}
	if metrics.ChangedSemanticLines != 1 {
		t.Fatalf("changed semantic lines = %d, want 1", metrics.ChangedSemanticLines)
	}
	if metrics.RouteOwners != 2 {
		t.Fatalf("route owners = %d, want 2", metrics.RouteOwners)
	}
	if len(metrics.FlexibilityCapabilities) < 6 {
		t.Fatalf("flexibility capability coverage too small: %#v", metrics.FlexibilityCapabilities)
	}

	raw, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}
	t.Logf("hybrid promise metrics: %s", raw)
	t.Logf("\n%s", hybridPromiseReport(metrics))
}

func BenchmarkHybridPromiseSetupParseMaterialize(b *testing.B) {
	fixture := writeHybridPromiseFixture(b, hybridPromiseAppYAML)
	opts := hybridPromiseOptions(filepath.Dir(fixture.app))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := yamlparser.ProcessFile(fixture.app, opts)
		if err != nil {
			b.Fatalf("process fixture: %v", err)
		}
		if len(result.Materialized) == 0 {
			b.Fatal("materialized result is empty")
		}
	}
}

func BenchmarkHybridPromiseFullPageRender(b *testing.B) {
	fixture := writeHybridPromiseFixture(b, hybridPromiseAppYAML)
	result := processHybridPromiseFile(b, fixture.app)
	rm := newYAMLProfileRenderManager(b)
	scope := mustHybridScope(b, result, "dashboard")
	ctx := createMockContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := renderHybridScopeWithContext(b, rm, scope, ctx)
		if len(out) == 0 {
			b.Fatal("rendered output is empty")
		}
	}
}

func BenchmarkHybridPromiseHTMXFragmentRender(b *testing.B) {
	fixture := writeHybridPromiseFixture(b, hybridPromiseAppYAML)
	result := processHybridPromiseFile(b, fixture.app)
	rm := newYAMLProfileRenderManager(b)
	scope := mustHybridScope(b, result, "metric_fragment")
	ctx := createMockContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := renderHybridScopeWithContext(b, rm, scope, ctx)
		if len(out) == 0 {
			b.Fatal("rendered output is empty")
		}
	}
}

func BenchmarkHybridPromiseOneLineFlexibilityChange(b *testing.B) {
	fixture := writeHybridPromiseFixture(b, hybridPromiseChangedYAML)
	rm := newYAMLProfileRenderManager(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := processHybridPromiseFile(b, fixture.app)
		scope := mustHybridScope(b, result, "dashboard")
		out := renderHybridScopeWithContext(b, rm, scope, createMockContext())
		if !strings.Contains(out, `data-card="static"`) {
			b.Fatal("changed capability was not rendered")
		}
	}
}

func writeHybridPromiseFixture(tb testing.TB, appYAML string) hybridPromiseFixture {
	tb.Helper()
	dir := tb.TempDir()
	sharedPath := filepath.Join(dir, "shared.hyperbricks.yaml")
	appPath := filepath.Join(dir, "app.hyperbricks.yaml")
	if err := os.WriteFile(sharedPath, []byte(strings.TrimSpace(hybridPromiseSharedYAML)+"\n"), 0o644); err != nil {
		tb.Fatalf("write shared fixture: %v", err)
	}
	if err := os.WriteFile(appPath, []byte(strings.TrimSpace(appYAML)+"\n"), 0o644); err != nil {
		tb.Fatalf("write app fixture: %v", err)
	}
	return hybridPromiseFixture{
		dir:    dir,
		app:    appPath,
		shared: sharedPath,
	}
}

func processHybridPromiseFile(tb testing.TB, path string) *yamlparser.Result {
	tb.Helper()
	result, err := yamlparser.ProcessFile(path, hybridPromiseOptions(filepath.Dir(path)))
	if err != nil {
		tb.Fatalf("process hybrid promise fixture: %v", err)
	}
	if len(result.Diagnostics) > 0 {
		tb.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
	return result
}

func hybridPromiseOptions(dir string) yamlparser.Options {
	return yamlparser.Options{
		Paths: yamlparser.PathMarkers{
			ModuleRoot:  dir,
			Module:      dir,
			HyperBricks: dir,
			Templates:   filepath.Join(dir, "templates"),
			Resources:   filepath.Join(dir, "resources"),
			Static:      filepath.Join(dir, "static"),
			Render:      filepath.Join(dir, "rendered"),
		},
		RecoverDuplicateChildren: true,
	}
}

func mustHybridScope(tb testing.TB, result *yamlparser.Result, name string) map[string]interface{} {
	tb.Helper()
	scope, ok := result.Materialized[name].(map[string]interface{})
	if !ok {
		tb.Fatalf("scope %q type = %T, want map[string]interface{}", name, result.Materialized[name])
	}
	return scope
}

func renderHybridScope(tb testing.TB, rm *render.RenderManager, scope map[string]interface{}) string {
	tb.Helper()
	return renderHybridScopeWithContext(tb, rm, scope, createMockContext())
}

func renderHybridScopeWithContext(tb testing.TB, rm *render.RenderManager, scope map[string]interface{}, ctx context.Context) string {
	tb.Helper()
	typeName, ok := scope["@type"].(string)
	if !ok || typeName == "" {
		tb.Fatalf("scope @type = %#v, want runtime type string", scope["@type"])
	}
	out, errs := rm.Render(typeName, scope, ctx)
	if len(errs) > 0 {
		tb.Fatalf("render returned errors: %v", errs)
	}
	return out
}

func countHybridRouteOwners(tb testing.TB, result *yamlparser.Result) int {
	tb.Helper()
	count := 0
	for name, raw := range result.Materialized {
		scope, ok := raw.(map[string]interface{})
		if !ok {
			tb.Fatalf("scope %q type = %T, want map[string]interface{}", name, raw)
		}
		switch scope["@type"] {
		case composite.HyperMediaConfigGetName(), composite.FragmentConfigGetName(), composite.ApiFragmentRenderConfigGetName():
			if route, _ := scope["route"].(string); strings.TrimSpace(route) != "" {
				count++
			}
		}
	}
	return count
}

func hybridFlexibilityCapabilities(tb testing.TB, result *yamlparser.Result) []string {
	tb.Helper()
	capabilities := map[string]bool{}
	for name, raw := range result.Materialized {
		scope, ok := raw.(map[string]interface{})
		if !ok {
			tb.Fatalf("scope %q type = %T, want map[string]interface{}", name, raw)
		}
		collectHybridCapabilities(scope, capabilities)
	}
	out := make([]string, 0, len(capabilities))
	for name := range capabilities {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func collectHybridCapabilities(scope map[string]interface{}, capabilities map[string]bool) {
	switch scope["@type"] {
	case composite.HyperMediaConfigGetName():
		capabilities["full_page_route"] = true
	case composite.FragmentConfigGetName():
		capabilities["htmx_fragment_route"] = true
	}
	if _, ok := scope["response"].(map[string]interface{}); ok {
		capabilities["htmx_response_headers"] = true
	}
	if _, ok := scope["guard"].(map[string]interface{}); ok {
		capabilities["route_guard"] = true
	}
	if _, ok := scope["inline"].(string); ok && scope["@type"] == composite.TemplateConfigGetName() {
		capabilities["inline_template"] = true
	}
	if _, ok := scope["values"].(map[string]interface{}); ok {
		capabilities["template_values"] = true
	}
	if _, ok := scope["@order"].([]string); ok {
		capabilities["ordered_children"] = true
	}
	for _, raw := range scope {
		child, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasType := child["@type"]; hasType {
			collectHybridCapabilities(child, capabilities)
		}
	}
}

func hybridPromiseReport(metrics hybridPromiseMetrics) string {
	var builder strings.Builder
	builder.WriteString("HyperBricks hybrid promise benchmark report\n")
	builder.WriteString("\n")
	builder.WriteString("Claim under test:\n")
	builder.WriteString("HyperBricks can express a server-rendered interactive app with a small YAML surface, reusable structure, route-owned fragments, and measurable runtime costs.\n")
	builder.WriteString("\n")
	builder.WriteString("Evidence collected by this test:\n")
	builder.WriteString(fmt.Sprintf("- Source size: %d non-blank YAML lines for shared base components plus one page and one fragment.\n", metrics.SourceLOC))
	builder.WriteString(fmt.Sprintf("- Flexibility change size: %d semantic source line to add one rendered capability to the existing app model.\n", metrics.ChangedSemanticLines))
	builder.WriteString(fmt.Sprintf("- Runtime surface: %d materialized objects, %d addressable route owners.\n", metrics.RuntimeObjects, metrics.RouteOwners))
	builder.WriteString(fmt.Sprintf("- Covered capabilities: %s.\n", strings.Join(metrics.FlexibilityCapabilities, ", ")))
	builder.WriteString("\n")
	builder.WriteString("Benchmark operations printed below:\n")
	builder.WriteString("- SetupParseMaterialize measures module-file loading with imports and YAML materialization.\n")
	builder.WriteString("- FullPageRender measures rendering the inherited page shell and nested templates.\n")
	builder.WriteString("- HTMXFragmentRender measures route-owned partial rendering with HTMX response metadata.\n")
	builder.WriteString("- OneLineFlexibilityChange measures parse plus render after a one-line feature addition.\n")
	builder.WriteString("\n")
	builder.WriteString("What this does not prove yet:\n")
	builder.WriteString("It does not compare HyperBricks against another stack. It establishes the local measurement contract that a later cross-stack benchmark must use.\n")
	return builder.String()
}

func countNonBlankLines(value string) int {
	count := 0
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func changedSemanticLines(before string, after string) int {
	beforeLines := normalizedSemanticLineCounts(before)
	afterLines := normalizedSemanticLineCounts(after)
	changes := 0
	for line, afterCount := range afterLines {
		if beforeCount := beforeLines[line]; afterCount > beforeCount {
			changes += afterCount - beforeCount
		}
	}
	for line, beforeCount := range beforeLines {
		if afterCount := afterLines[line]; beforeCount > afterCount {
			changes += beforeCount - afterCount
		}
	}
	return changes
}

func normalizedSemanticLineCounts(value string) map[string]int {
	lines := map[string]int{}
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines[trimmed]++
	}
	return lines
}
