package renderplan

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestTemplateParamsPolicy(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   paramsPolicy
	}{
		{name: "unrelated field", source: `{{.content}}`, want: paramsUnused},
		{name: "direct selector", source: `{{.Params.rid}}`, want: paramsReadOnly},
		{name: "read-only conditional", source: `{{if .Params.rid}}{{.Params.rid}}{{end}}`, want: paramsReadOnly},
		{name: "params map", source: `{{.Params}}`, want: paramsPrivate},
		{name: "mutating function", source: `{{set .Params "rid" "changed"}}`, want: paramsPrivate},
		{name: "function argument", source: `{{toJson .Params.rid}}`, want: paramsPrivate},
		{name: "full dot", source: `{{range .}}{{.}}{{end}}`, want: paramsPrivate},
		{name: "variable alias", source: `{{$params := .Params}}{{$params.rid}}`, want: paramsPrivate},
		{name: "named template", source: `{{template "child" .}}{{define "child"}}{{.Params.rid}}{{end}}`, want: paramsPrivate},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := shared.ParsedGenericTemplate(test.source)
			if err != nil {
				t.Fatalf("parse template: %v", err)
			}
			if got := templateParamsPolicy(parsed); got != test.want {
				t.Fatalf("templateParamsPolicy() = %d, want %d", got, test.want)
			}
		})
	}
}
