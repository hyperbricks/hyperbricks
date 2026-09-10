package component

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

func TestGojaPreparedInputSnapshotAndEscaping(t *testing.T) {
	config := GojaRenderConfig{
		Script: `function main(input) {
  input.values.nested.items.push(input.query.rid);
  return {message: input.values.nested.items.join("/"), count: Object.keys(input.query).length};
}`,
		Inline:           `<p>{{.Data.message}}:{{.Data.count}}</p>`,
		Values:           map[string]interface{}{"nested": map[string]interface{}{"items": []string{"base"}}},
		AllowedQueryKeys: []string{"rid"},
	}
	prepared := PrepareGojaRender(config, nil)
	if err := prepared.Err(); err != nil {
		t.Fatal(err)
	}
	config.Prepared = prepared
	config.Values["nested"].(map[string]interface{})["items"].([]string)[0] = "changed after preparation"
	config.AllowedQueryKeys[0] = "secret"
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rid := fmt.Sprintf("<request-%d>", i)
			req := httptest.NewRequest("GET", "/?rid="+url.QueryEscape(rid)+"&secret=hidden", nil)
			ctx := context.WithValue(req.Context(), shared.Request, req)
			got, errs := (&GojaRenderer{}).Render(config, ctx)
			want := fmt.Sprintf("<p>base/&lt;request-%d&gt;:1</p>", i)
			if len(errs) != 0 || got != want {
				t.Errorf("output=%q want=%q errors=%v", got, want, errs)
			}
		}(i)
	}
	wg.Wait()
}

func TestGojaQueryDefaultsAndRepeatedValues(t *testing.T) {
	for _, keys := range [][]string{nil, {"rid"}} {
		config := GojaRenderConfig{
			Script: `function main(input) { return {query: JSON.stringify(input.query)}; }`,
			Inline: `{{.Data.query}}`, AllowedQueryKeys: keys,
		}
		config.Prepared = PrepareGojaRender(config, nil)
		req := httptest.NewRequest("GET", "/?rid=a&rid=b&secret=c", nil)
		output, errs := (&GojaRenderer{}).Render(config, context.WithValue(req.Context(), shared.Request, req))
		want := "{}"
		if len(keys) > 0 {
			want = `{&#34;rid&#34;:[&#34;a&#34;,&#34;b&#34;]}`
		}
		if output != want || len(errs) != 0 {
			t.Fatalf("keys=%v output=%q want=%q errs=%v", keys, output, want, errs)
		}
	}
}

func TestGojaPreparationErrorsAndMissingPreparation(t *testing.T) {
	for _, config := range []GojaRenderConfig{
		{Script: "function main() { return {}; }"},
		{Script: "function main() { return {}; }", Inline: "ok", Template: "also.html"},
		{Script: "function main() { return {}; }", Template: "missing.html"},
		{Script: "function main() { return {}; }", Inline: "{{"},
		{Script: "function main() { return {}; }", Inline: "ok", Timeout: "0s"},
		{Script: "function main() { return {}; }", Inline: "ok", Timeout: "6s"},
		{Script: "function main() { return {}; }", Inline: "ok", Timeout: "invalid"},
		{Script: "function main() { return {}; }", Inline: "ok", Values: map[string]interface{}{"fn": func() {}}},
		{Inline: "ok"},
	} {
		prepared := PrepareGojaRender(config, nil)
		if prepared.Err() == nil {
			t.Errorf("accepted invalid config: %#v", config)
		}
		if output, errs := prepared.Render(nil, nil); output != "" || len(errs) == 0 {
			t.Errorf("invalid preparation rendered: %q %v", output, errs)
		}
	}
	if _, errs := (&GojaRenderer{}).Render(GojaRenderConfig{Script: `throw new Error("must not execute")`}, nil); len(errs) != 1 || !strings.Contains(errs[0].Error(), "not prepared") {
		t.Fatalf("unprepared render: %v", errs)
	}
}

func TestGojaTemplateProviderAndRuntimeMetadataDecode(t *testing.T) {
	config := GojaRenderConfig{Script: `function main() { return {message: "<b>text</b>"}; }`, Template: "test.html"}
	config.Enclose = "<section>|</section>"
	prepared := PrepareGojaRender(config, func(name string) (string, bool) { return "<p>{{.Data.message}}</p>", name == "test.html" })
	factory := typefactory.NewTypeFactory()
	factory.RegisterType(GojaRenderConfigGetName(), reflect.TypeOf(GojaRenderConfig{}))
	response, err := factory.CreateInstance(typefactory.TypeRequest{
		TypeName: GojaRenderConfigGetName(),
		Data:     map[string]interface{}{GojaPreparedKey: prepared},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded := response.Instance.(GojaRenderConfig)
	if decoded.Prepared != prepared {
		t.Fatal("preparation identity changed while decoding")
	}
	got, errs := (&GojaRenderer{}).Render(decoded, nil)
	if got != "<section><p>&lt;b&gt;text&lt;/b&gt;</p></section>" || len(errs) != 0 {
		t.Fatalf("output=%q errs=%v", got, errs)
	}
}
