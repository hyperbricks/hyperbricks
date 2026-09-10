package renderplan

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestForwardsOnlyChild(t *testing.T) {
	for _, test := range []struct {
		name, source, key string
		want              bool
	}{
		{"selector", `{{.child}}`, "child", true},
		{"action whitespace", `{{ .child }}`, "child", true},
		{"trimmed whitespace", " \n{{- .child -}}\n ", "child", true},
		{"comment", `{{/* ignored */}}{{.child}}`, "child", true},
		{"empty", ``, "child", false},
		{"literal whitespace", ` {{.child}}`, "child", false},
		{"repeated selector", `{{.child}}{{.child}}`, "child", false},
		{"wrong key", `{{.other}}`, "child", false},
		{"nested selector", `{{.child.value}}`, "child", false},
		{"request params", `{{.Params}}`, "Params", false},
		{"dot", `{{.}}`, "child", false},
		{"variable selector", `{{$.child}}`, "child", false},
		{"parentheses", `{{(.child)}}`, "child", false},
		{"pipeline", `{{.child | html}}`, "child", false},
		{"function", `{{printf "%s" .child}}`, "child", false},
		{"custom function", `{{safe .child}}`, "child", false},
		{"declaration", `{{$x := .child}}`, "child", false},
		{"conditional", `{{if .child}}{{.child}}{{end}}`, "child", false},
		{"associated template", `{{define "extra"}}x{{end}}{{.child}}`, "child", false},
		{"template call", `{{template "extra" .}}{{define "extra"}}{{.child}}{{end}}`, "child", false},
		{"other root name", `{{define "forward-check"}}{{.child}}{{end}}`, "child", false},
		{"matching root name", `{{define "hyperbricks-generic-template"}}{{.child}}{{end}}`, "child", true},
		{"attribute context", `<a title="{{.child}}">`, "child", false},
		{"script context", `<script>{{.child}}</script>`, "child", false},
		{"invalid syntax", `{{.child`, "child", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := forwardsOnlyChild(test.source, test.key, "hyperbricks-generic-template"); got != test.want {
				t.Fatalf("forwardsOnlyChild(%q, %q) = %t, want %t", test.source, test.key, got, test.want)
			}
		})
	}
	if !forwardsOnlyChild(`{{define "forward-check"}}{{.child}}{{end}}`, "child", "forward-check") {
		t.Fatal("matching a named root must use the supplied root name")
	}
	if forwardsOnlyChild(`{{define "hyperbricks-generic-template"}}{{.child}}{{end}}`, "child", "forward-check") {
		t.Fatal("a different named root must not be forwarded")
	}
}

func TestCompileTemplateSelectsSoleChildForwarding(t *testing.T) {
	shared.Init_configuration()
	manager := render.NewRenderManager()
	manager.RegisterComponent(composite.TemplateConfigGetName(), &composite.TemplateRenderer{}, reflect.TypeOf(composite.TemplateConfig{}))
	manager.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	child := map[string]interface{}{"@type": component.TextConfigGetName(), "value": "child"}
	for _, test := range []struct {
		name, source string
		values       map[string]interface{}
		want         bool
	}{
		{"sole component", `{{.content}}`, map[string]interface{}{"content": child}, true},
		{"scalar", `{{.content}}`, map[string]interface{}{"content": "<b>scalar</b>"}, false},
		{"unused scalar", `{{.content}}`, map[string]interface{}{"content": child, "unused": "value"}, false},
		{"unused component", `{{.content}}`, map[string]interface{}{"content": child, "unused": child}, false},
		{"params component", `{{.Params}}`, map[string]interface{}{"Params": child}, false},
		{"named root", `{{define "hyperbricks-generic-template"}}{{.content}}{{end}}`, map[string]interface{}{"content": child}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Execute first: html/template adds escaping commands to the cached tree.
			parsed, err := shared.ParsedGenericTemplate(test.source)
			if err != nil {
				t.Fatal(err)
			}
			if err := parsed.Execute(io.Discard, map[string]interface{}{"content": "warm", "Params": "warm"}); err != nil {
				t.Fatal(err)
			}
			c := compiler{manager: manager}
			compiled, err := c.compileTemplate(map[string]interface{}{
				"@type": composite.TemplateConfigGetName(), "inline": test.source, "values": test.values,
			})
			if err != nil {
				t.Fatal(err)
			}
			node := compiled.(*templateNode)
			if got := node.forwardChild != nil; got != test.want {
				t.Fatalf("forwardChild present = %t, want %t", got, test.want)
			}
		})
	}
}

func TestTemplateForwardChildPreservesWarningsAndErrors(t *testing.T) {
	childError := errors.New("child failed")
	calls := 0
	node := &templateNode{
		config: composite.TemplateConfig{TemplateOptions: composite.TemplateOptions{
			Enclose: "  <section> | </section>  ",
		}},
		warnings: []string{"template warning"},
		forwardChild: forwardChildNodeFunc(func(*renderState) (string, []error) {
			calls++
			return "<b>partial</b>", []error{childError}
		}),
	}
	output, renderErrors := node.Render(newRenderState(context.Background(), nil))
	if output != "<section><b>partial</b></section>" || calls != 1 {
		t.Fatalf("output = %q, child calls = %d", output, calls)
	}
	if len(renderErrors) != 2 || renderErrors[1] != childError {
		t.Fatalf("errors = %v, want template warning then original child error", renderErrors)
	}
	warning, ok := renderErrors[0].(shared.ComponentError)
	if !ok || warning.Err != "template warning" || warning.Rejected {
		t.Fatalf("warning = %#v", renderErrors[0])
	}
}

type forwardChildNodeFunc func(*renderState) (string, []error)

func (f forwardChildNodeFunc) Render(state *renderState) (string, []error) { return f(state) }
