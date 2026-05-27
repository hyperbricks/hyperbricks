package parser

import (
	"fmt"
	"strings"
	"testing"
)

func benchmarkHyperScript(repeats int) string {
	KnownTypes["<TEXT>"] = true
	KnownTypes["<TEMPLATE>"] = true

	var builder strings.Builder
	builder.WriteString("$brand = HyperBricks\n")
	builder.WriteString("page = <TEMPLATE>\n")
	builder.WriteString("page.values {\n")
	for i := 0; i < repeats; i++ {
		builder.WriteString(fmt.Sprintf("item_%03d = <TEXT>\n", i))
		builder.WriteString(fmt.Sprintf("item_%03d.value = {{VAR:brand}} item %03d\n", i, i))
		builder.WriteString(fmt.Sprintf("item_%03d.class = card {{ENV:HB_BENCH_CLASS}}\n", i))
	}
	builder.WriteString("}\n")
	return builder.String()
}

func BenchmarkParseHyperScriptMedium(b *testing.B) {
	input := benchmarkHyperScript(64)
	b.Setenv("HB_BENCH_CLASS", "bench")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := ParseHyperScript(input)
		if len(config) == 0 {
			b.Fatal("ParseHyperScript returned empty config")
		}
	}
}

func BenchmarkApplyPathMarkers(b *testing.B) {
	input := strings.Repeat("{{ROOT}}/{{MODULE}}/{{RESOURCES}}/{{TEMPLATES}}/{{STATIC}}/{{HYPERBRICKS}}/{{MODULE_ROOT}}\n", 128)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := applyPathMarkers(input)
		if out == "" {
			b.Fatal("applyPathMarkers returned empty output")
		}
	}
}
