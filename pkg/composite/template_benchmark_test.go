package composite

import (
	"fmt"
	"strings"
	"testing"
)

func benchmarkTemplateContent() string {
	var builder strings.Builder
	builder.WriteString(`<main class="page">`)
	builder.WriteString(`{{ range .Cards }}`)
	builder.WriteString(`<article class="card"><h2>{{ .Title }}</h2><p>{{ .Body }}</p><a href="{{ .Href }}">{{ $.CTA }}</a></article>`)
	builder.WriteString(`{{ end }}`)
	builder.WriteString(`<footer>{{ .Footer }}</footer>`)
	builder.WriteString(`</main>`)
	return builder.String()
}

func benchmarkTemplateData() map[string]interface{} {
	cards := make([]map[string]string, 32)
	for i := range cards {
		cards[i] = map[string]string{
			"Title": fmt.Sprintf("Card %02d", i+1),
			"Body":  "Reusable HyperBricks content rendered through Go templates.",
			"Href":  fmt.Sprintf("/docs/card-%02d", i+1),
		}
	}
	return map[string]interface{}{
		"CTA":    "Read more",
		"Cards":  cards,
		"Footer": "Rendered by HyperBricks",
	}
}

func BenchmarkApplyTemplateMedium(b *testing.B) {
	templateStr := benchmarkTemplateContent()
	data := benchmarkTemplateData()
	config := TemplateConfig{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, errs := applyTemplate(templateStr, data, config)
		if len(errs) > 0 {
			b.Fatalf("applyTemplate returned errors: %v", errs)
		}
		if out == "" {
			b.Fatal("applyTemplate returned empty output")
		}
	}
}
