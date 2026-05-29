//go:build legacy_hyperbricks_parser

package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/core"
)

func configureTemplateFixture(t *testing.T, filename string, content string) {
	t.Helper()

	root := t.TempDir()
	templatesDir := filepath.Join(root, "templates")
	templatePath := filepath.Join(templatesDir, filename)
	if err := os.MkdirAll(filepath.Dir(templatePath), 0755); err != nil {
		t.Fatalf("failed to create template directory: %v", err)
	}
	if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write template fixture: %v", err)
	}

	previousDirectories := core.ModuleDirectories
	ClearTemplateStore()
	t.Cleanup(func() {
		core.ModuleDirectories = previousDirectories
		ClearTemplateStore()
	})

	core.ModuleDirectories.Root = root + string(os.PathSeparator)
	core.ModuleDirectories.TemplateDir = "templates"
}

func registerKnownTypeForTest(t *testing.T, name string) {
	t.Helper()

	previousValue, hadValue := KnownTypes[name]
	KnownTypes[name] = true
	t.Cleanup(func() {
		if hadValue {
			KnownTypes[name] = previousValue
		} else {
			delete(KnownTypes, name)
		}
	})
}

func TestPreprocessHyperScriptTemplateMarkerStoresTemplate(t *testing.T) {
	templateContent := "<!DOCTYPE html><html><body><div>{{content}}</div></body></html>"
	configureTemplateFixture(t, "test.html", templateContent)

	input := `
	$template_001 = {{TEMPLATE:test.html}}
	template = {{VAR:template_001}}
	`

	processed, err := PreprocessHyperScript(input)
	if err != nil {
		t.Fatalf("failed to preprocess hyperscript: %v", err)
	}
	if strings.Contains(processed, "{{TEMPLATE:test.html}}") {
		t.Fatalf("template marker was not replaced: %s", processed)
	}

	parsedConfig := ParseHyperScript(processed)
	if got := parsedConfig["template"]; got != "test.html" {
		t.Fatalf("template = %q, want test.html", got)
	}

	if template, found := GetTemplate("test.html"); !found || template != templateContent {
		t.Fatalf("template store entry = %q, found = %v", template, found)
	}
}

func TestParseHyperScriptVariablesAndNestedBlocks(t *testing.T) {
	registerKnownTypeForTest(t, "<TEXT>")

	input := `
	$myname = John
	$mylastname = Doe

	name = <TEXT>
	name.value = Hello {{VAR:myname}} {{VAR:mylastname}}!

	greeting {
		message = <TEXT>
		message.value = Welcome, {{VAR:myname}}!
	}

	myvalues {
		just_funny = hehe
		just_laughing = laughing
	}

	othervalues = {
		key1 = value1
		key2 = value2
	}
	`

	parsedConfig := ParseHyperScript(input)

	expected := map[string]interface{}{
		"name": map[string]interface{}{
			"@type": "<TEXT>",
			"value": "Hello John Doe!",
		},
		"greeting": map[string]interface{}{
			"message": map[string]interface{}{
				"@type": "<TEXT>",
				"value": "Welcome, John!",
			},
		},
		"myvalues": map[string]interface{}{
			"just_funny":    "hehe",
			"just_laughing": "laughing",
		},
		"othervalues": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Fatalf("parsed config mismatch\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}
