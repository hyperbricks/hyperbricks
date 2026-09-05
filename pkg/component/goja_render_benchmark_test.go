package component

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

var gojaBenchmarkOutput string

// This measures the added scripting cost for one component, not HTTP latency
// or maximum server capacity. Both variants calculate and render the same result.
func BenchmarkGojaRender(b *testing.B) {
	const markup = `<p>{{.Data.message}}</p>`
	config := GojaRenderConfig{
		Script: `function main(input) {
  const quantity = Number(input.query.quantity);
  return {message: quantity <= Number(input.values.stock) ? "Op voorraad" : "Onvoldoende voorraad"};
}`,
		Inline: markup, Values: map[string]interface{}{"stock": 12}, AllowedQueryKeys: []string{"quantity"},
	}
	p := PrepareGojaRender(config, nil)
	if p.Err() != nil {
		b.Fatal(p.Err())
	}
	tmpl, err := shared.ParsedGenericTemplate(markup)
	if err != nil {
		b.Fatal(err)
	}
	params := map[string]interface{}{"quantity": "2"}
	goRender := func() string {
		quantity, err := strconv.Atoi(params["quantity"].(string))
		if err != nil {
			b.Fatal(err)
		}
		message := "Onvoldoende voorraad"
		if quantity <= 12 {
			message = "Op voorraad"
		}
		var out strings.Builder
		if err := tmpl.Execute(&out, map[string]interface{}{"Data": map[string]interface{}{"message": message}}); err != nil {
			b.Fatal(err)
		}
		return out.String()
	}
	scriptRender := func() string {
		out, errs := p.Render(context.Background(), params)
		if len(errs) != 0 {
			b.Fatal(errs)
		}
		return out
	}
	if goRender() != scriptRender() || scriptRender() != "<p>Op voorraad</p>" {
		b.Fatal("baseline and script output differ")
	}
	b.Run("GoLogicAndTemplate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gojaBenchmarkOutput = goRender()
		}
	})
	b.Run("IsolatedGojaAndTemplate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gojaBenchmarkOutput = scriptRender()
		}
	})
}
