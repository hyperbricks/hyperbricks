package renderplan

import (
	"context"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestLiteralTemplatePolicy(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   bool
	}{
		{"", true}, {"<b>text</b>", true}, {"{{/* comment */}}", true},
		{"<!-- HTML comment --><b>text</b>", true},
		{`{{define "root"}}literal{{end}}`, true},
		{`{{define "other"}}literal{{end}}`, false},
		{`{{define "root"}}{{.value}}{{end}}`, false},
		{"{{.value}}", false}, {"{{.Params.rid}}", false},
		{"{{printf \"constant\"}}", false}, {"{{random}}", false},
		{"{{$x := 1}}", false}, {"{{if true}}literal{{end}}", false},
		{"{{range .items}}literal{{end}}", false}, {"{{with .}}literal{{end}}", false},
		{"{{template \"other\"}}", false}, {"{{invalid", false},
	} {
		t.Run(tc.source, func(t *testing.T) {
			if got := isLiteralTemplate(tc.source, "root"); got != tc.want {
				t.Fatalf("literal policy = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompileTemplateSelectsLiteral(t *testing.T) {
	shared.Init_configuration()
	manager := render.NewRenderManager()
	manager.RegisterComponent(composite.TemplateConfigGetName(), &composite.TemplateRenderer{}, reflect.TypeOf(composite.TemplateConfig{}))
	manager.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	for _, tc := range []struct {
		name, source string
		values       map[string]interface{}
		want         bool
	}{
		{"plain", "<b>text</b>", nil, true},
		{"empty values", "<b>text</b>", map[string]interface{}{}, true},
		{"unused scalar", "<b>text</b>", map[string]interface{}{"unused": "value"}, false},
		{"unused child", "<b>text</b>", map[string]interface{}{"unused": map[string]interface{}{"@type": component.TextConfigGetName(), "value": "child"}}, false},
		{"params", "{{.Params.rid}}", map[string]interface{}{}, false},
		{"invalid context", "<script>", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := compiler{manager: manager}
			compiled, err := c.compileTemplate(map[string]interface{}{
				"@type": composite.TemplateConfigGetName(), "inline": tc.source, "values": tc.values,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := compiled.(*templateNode).literal != nil; got != tc.want {
				t.Fatalf("literal present = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLiteralTemplateKeepsWarningsRequestLocal(t *testing.T) {
	text := "<b>literal</b>"
	node := &templateNode{
		config:  composite.TemplateConfig{TemplateOptions: composite.TemplateOptions{Inline: text, Enclose: "section"}},
		literal: &text, warnings: []string{"source warning"},
	}
	first, firstErrors := node.Render(newRenderState(context.Background(), nil))
	second, secondErrors := node.Render(newRenderState(context.Background(), nil))
	if first != "<section><b>literal</b></section>" || second != first || len(firstErrors) != 1 || len(secondErrors) != 1 {
		t.Fatalf("outputs=%q/%q errors=%v/%v", first, second, firstErrors, secondErrors)
	}
	if firstErrors[0].(shared.ComponentError).Hash == secondErrors[0].(shared.ComponentError).Hash {
		t.Fatal("literal output reused request-specific diagnostic errors")
	}
}
