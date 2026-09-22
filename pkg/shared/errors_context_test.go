package shared

import (
	"errors"
	"fmt"
	"testing"
)

func TestDiagnosticPreservesCausesAndChildLocations(t *testing.T) {
	meta := Meta{ConfigType: "<TEMPLATE>", HyperBricksFile: "hyperbricks/page.hyperbricks.yaml", HyperBricksPath: "page.template", HyperBricksKey: "template",
		Source: &SourceContext{File: "hyperbricks/page.hyperbricks.yaml", Line: 5, Column: 7}}
	child := ComponentError{Err: "child failed", File: "hyperbricks/parts/item.hyperbricks.yaml", Path: "page.template.values.item", Type: "<TEXT>", Line: 8, Column: 3}
	for _, cause := range []error{errors.New("plain error"), child, &child, fmt.Errorf("outer: %w", &child)} {
		got := Diagnostic(cause, meta, "render")
		if !errors.Is(got, cause) {
			t.Errorf("cause lost: %#v", got)
		}
		if got.Err != cause.Error() {
			t.Errorf("message lost: %s", got.Err)
		}
		if _, structured := AsComponentError(cause); structured {
			if got.File != child.File || got.Path != child.Path || got.Line != child.Line || got.Type != child.Type {
				t.Errorf("child context overwritten: %#v", got)
			}
		} else if got.File != meta.HyperBricksFile || got.Line != 5 || got.Phase != "render" {
			t.Errorf("missing context: %#v", got)
		}
	}
}

func TestResourceDiagnosticTemplateLocation(t *testing.T) {
	meta := Meta{ConfigType: "<TEMPLATE>", HyperBricksPath: "page.template", Source: &SourceContext{
		Fields:    map[string]SourceLocation{"template": {File: "hyperbricks/page.hyperbricks.yaml", Line: 4, Column: 9}},
		Resources: map[string]string{"template": "templates/card.html"},
	}}
	template, err := ParsedNamedTemplate(meta.TemplateName(""), "<p>{{index .items 99}}</p>")
	if err != nil {
		t.Fatal(err)
	}
	var output discardTemplateOutput
	err = template.Execute(&output, map[string]interface{}{"items": []string{"one"}})
	got := ResourceDiagnostic(err, meta, "render", "template")
	if got.File != "hyperbricks/page.hyperbricks.yaml" || got.Line != 4 || got.Resource != "templates/card.html" || got.ResourceLine != 1 || got.ResourceColumn != 5 {
		t.Fatalf("location = %#v", got)
	}
}

type discardTemplateOutput struct{}

func (*discardTemplateOutput) Write(value []byte) (int, error) { return len(value), nil }
