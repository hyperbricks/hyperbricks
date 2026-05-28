package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/hyperbricks/hyperbricks/pkg/render"
	hbschema "github.com/hyperbricks/hyperbricks/pkg/schema"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

var updateYAMLDocsFlag = flag.Bool("update-yaml-docs", false, "write generated YAML reference docs")

const (
	yamlDocumentationFixtureDir = "hyperbricks-yaml-test-files"
	yamlReferencePath           = "../../docs/REFERENCE_YAML.md"
)

var yamlDocumentationCuratedFixtures = map[string]string{
	"<HEAD>":   "head-assets.hyperbricks.yaml.test",
	"<MENU>":   "menu-items.hyperbricks.yaml.test",
	"<PLUGIN>": "plugin-plugin.hyperbricks.yaml.test",
	"<STYLES>": "styles-file.hyperbricks.yaml.test",
}

var yamlDocumentationExplicitSkips = map[string]string{}

type yamlReferenceData struct {
	Version    string
	Categories []yamlReferenceCategory
}

type yamlReferenceCategory struct {
	Name  string
	Types []yamlReferenceType
}

type yamlReferenceType struct {
	Name        string
	Token       string
	Description string
	Fields      []yamlReferenceField
	Example     yamlReferenceExample
}

type yamlReferenceField struct {
	Path        string
	Kind        string
	Required    bool
	Description string
}

type yamlReferenceExample struct {
	Name           string
	Explainer      string
	Source         string
	ExpectedOutput string
}

func TestYAMLDocumentationFieldExampleReferences(t *testing.T) {
	for _, schemaType := range hbschema.ExtractRegistry(hbschema.Definitions()).Types {
		t.Run(schemaType.Name, func(t *testing.T) {
			for _, field := range uniqueSchemaFields(schemaType.Fields) {
				for _, ref := range yamlDocumentationExampleRefs(field.Example) {
					fixturePath, ok := yamlDocumentationFixturePathForReference(ref)
					if !ok {
						t.Fatalf("%s.%s example reference %q is not a .hyperbricks fixture reference", schemaType.Name, field.Path, ref)
					}
					if _, exists := yamlDocumentationExplicitSkips[fixturePath]; exists {
						continue
					}
					if _, err := os.Stat(fixturePath); err != nil {
						t.Fatalf("%s.%s example %q has no YAML fixture at %s: %v", schemaType.Name, field.Path, ref, fixturePath, err)
					}
				}
			}
		})
	}
}

func TestYAMLDocumentationCuratedExamples(t *testing.T) {
	rm := newYAMLProfileRenderManager(t)
	for _, def := range hbschema.Definitions() {
		t.Run(def.Name, func(t *testing.T) {
			testCase := yamlDocumentationLoadCuratedCase(t, def)
			yamlDocumentationRenderCase(t, rm, testCase)
		})
	}
}

func TestYAMLDocumentationCuratedExamplesAreReadable(t *testing.T) {
	numericChildName := regexp.MustCompile(`(?m)^\s*-\s+[a-z][a-z0-9]*_[0-9]+:\s*$`)
	for _, def := range hbschema.Definitions() {
		t.Run(def.Name, func(t *testing.T) {
			testCase := yamlDocumentationLoadCuratedCase(t, def)
			if strings.Contains(testCase.Source, `"\n`) || strings.Contains(testCase.Source, `\n"`) {
				t.Fatalf("%s contains escaped newline literals; use YAML block scalars for multiline examples", filepath.Base(testCase.Path))
			}
			if match := numericChildName.FindString(testCase.Source); match != "" {
				t.Fatalf("%s contains numeric child name %q; use semantic names in public YAML examples", filepath.Base(testCase.Path), strings.TrimSpace(match))
			}
		})
	}
}

func TestYAMLDocumentationReference(t *testing.T) {
	rendered := renderYAMLReference(t)
	if *updateYAMLDocsFlag {
		if err := os.WriteFile(yamlReferencePath, rendered, 0o644); err != nil {
			t.Fatalf("write %s: %v", yamlReferencePath, err)
		}
		return
	}

	expected, err := os.ReadFile(yamlReferencePath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("%s is not present; verified docs are intentionally curated separately", yamlReferencePath)
		}
		t.Fatalf("read %s: %v; run go test ./test/docs -run TestYAMLDocumentationReference -update-yaml-docs -count=1", yamlReferencePath, err)
	}
	if !bytes.Equal(rendered, expected) {
		t.Fatalf("%s is stale; run go test ./test/docs -run TestYAMLDocumentationReference -update-yaml-docs -count=1", yamlReferencePath)
	}
}

func TestYAMLDocumentationReferenceMarkdownIsHTMLSafe(t *testing.T) {
	rendered := string(renderYAMLReference(t))
	prose := yamlDocumentationMarkdownOutsideFences(rendered)
	prose = regexp.MustCompile("`[^`\n]*`").ReplaceAllString(prose, "")
	if match := angleTag.FindString(prose); match != "" {
		t.Fatalf("generated YAML reference contains raw %s outside code spans/fences; use inline code to avoid Markdown rendering it as HTML", match)
	}
}

func TestYAMLDocumentationReferenceUsesPlainBlockScalars(t *testing.T) {
	rendered := string(renderYAMLReference(t))
	if match := regexp.MustCompile(`\|[1-9]`).FindString(rendered); match != "" {
		t.Fatalf("generated YAML reference contains %q; use plain | or |- block scalars in public examples", match)
	}
}

func renderYAMLReference(t *testing.T) []byte {
	t.Helper()
	tmpl, err := template.New("yaml_template.md").Funcs(template.FuncMap{
		"boolText": func(value bool) string {
			if value {
				return "yes"
			}
			return "no"
		},
		"trim": strings.TrimSpace,
		"mdText": func(value string) string {
			return strings.TrimSpace(codeifyTags(value))
		},
		"mdBlock": func(value string) string {
			return strings.TrimSpace(codeifyTags(value))
		},
		"mdCell": func(value string) string {
			value = codeifyTags(value)
			value = strings.ReplaceAll(value, "\n", " ")
			value = strings.ReplaceAll(value, "|", "\\|")
			value = strings.TrimSpace(value)
			if value == "" {
				return "-"
			}
			return value
		},
	}).ParseFiles("yaml_template.md")
	if err != nil {
		t.Fatalf("parse yaml_template.md: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "main", yamlReferenceDataFromSchema(t)); err != nil {
		t.Fatalf("render YAML reference: %v", err)
	}
	return []byte(compactMarkdownOutsideCodeFences(buf.String()))
}

func yamlDocumentationMarkdownOutsideFences(markdown string) string {
	var out []string
	inFence := false
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func yamlReferenceDataFromSchema(t *testing.T) yamlReferenceData {
	t.Helper()
	registry := hbschema.ExtractRegistry(hbschema.Definitions())
	defsByToken := make(map[string]hbschema.Definition)
	for _, def := range hbschema.Definitions() {
		defsByToken[def.Token] = def
	}

	byCategory := make(map[string][]yamlReferenceType)
	for _, schemaType := range registry.Types {
		def, ok := defsByToken[schemaType.Token]
		if !ok {
			t.Fatalf("definition for token %s not found", schemaType.Token)
		}
		testCase := yamlDocumentationLoadCuratedCase(t, def)
		fields := uniqueSchemaFields(schemaType.Fields)
		refFields := make([]yamlReferenceField, 0, len(fields))
		for _, field := range fields {
			refFields = append(refFields, yamlReferenceField{
				Path:        field.Path,
				Kind:        field.Kind,
				Required:    field.Required,
				Description: field.Description,
			})
		}
		byCategory[string(schemaType.Category)] = append(byCategory[string(schemaType.Category)], yamlReferenceType{
			Name:        schemaType.Name,
			Token:       schemaType.Token,
			Description: schemaType.Description,
			Fields:      refFields,
			Example: yamlReferenceExample{
				Name:           filepath.Base(testCase.Path),
				Explainer:      testCase.Explainer,
				Source:         strings.TrimSpace(testCase.Source),
				ExpectedOutput: strings.TrimSpace(testCase.ExpectedOutput),
			},
		})
	}

	categories := make([]yamlReferenceCategory, 0, len(byCategory))
	for category, types := range byCategory {
		sort.Slice(types, func(i, j int) bool {
			return types[i].Token < types[j].Token
		})
		categories = append(categories, yamlReferenceCategory{
			Name:  category,
			Types: types,
		})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})
	return yamlReferenceData{
		Version:    registry.Version,
		Categories: categories,
	}
}

func yamlDocumentationLoadCuratedCase(t *testing.T, def hbschema.Definition) yamlReadableCase {
	t.Helper()
	fixtureName := yamlDocumentationCuratedFixtureName(def)
	fixturePath := filepath.Join(yamlDocumentationFixtureDir, fixtureName)
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("curated YAML fixture for %s (%s) missing at %s: %v", def.Name, def.Token, fixturePath, err)
	}
	return parseYAMLReadableCase(t, fixturePath)
}

func yamlDocumentationRenderCase(t *testing.T, rm *render.RenderManager, testCase yamlReadableCase) {
	t.Helper()
	result, err := yamlparser.ProcessBytes([]byte(testCase.Source), yamlProfileOptionsForCase(t, testCase.Path))
	if err != nil {
		t.Fatalf("process YAML case %s: %v", testCase.Path, err)
	}
	configureYAMLProfileMenus(t, rm, result.Document, result.Materialized)
	scope := asMap(t, result.Materialized[testCase.Scope], testCase.Scope)
	assertCaseExpectedPreprocessedYAML(t, testCase, result.Preprocessed)
	assertCaseExpectedMaterializedJSON(t, testCase, scope)
	assertCaseExpectedRuntimeJSON(t, rm, testCase, scope)
	assertCaseExpectedJSON(t, rm, testCase, scope)

	typeName, ok := scope["@type"].(string)
	if !ok || typeName == "" {
		t.Fatalf("%s @type = %#v, want runtime type string", testCase.Scope, scope["@type"])
	}
	if strings.TrimSpace(testCase.ExpectedOutput) == "" {
		return
	}
	output, renderErrors := rm.Render(typeName, scope, createMockContext())
	if len(renderErrors) > 0 {
		t.Fatalf("render %s returned errors: %v", testCase.Scope, renderErrors)
	}
	if stripAllWhitespace(output) != stripAllWhitespace(testCase.ExpectedOutput) {
		t.Fatalf("rendered output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(testCase.Path), output, testCase.ExpectedOutput)
	}
}

func yamlDocumentationCuratedFixtureName(def hbschema.Definition) string {
	if fixtureName := yamlDocumentationCuratedFixtures[def.Token]; fixtureName != "" {
		return fixtureName
	}
	return fmt.Sprintf("%s-@doc.hyperbricks.yaml.test", yamlDocumentationKebabName(def.Name))
}

func yamlDocumentationKebabName(name string) string {
	var out strings.Builder
	var previousWasSeparator bool
	for index, r := range name {
		switch {
		case r == '_' || r == '-' || r == ' ':
			if out.Len() > 0 && !previousWasSeparator {
				out.WriteByte('-')
				previousWasSeparator = true
			}
		case r >= 'A' && r <= 'Z':
			if index > 0 && !previousWasSeparator {
				out.WriteByte('-')
			}
			out.WriteRune(r + ('a' - 'A'))
			previousWasSeparator = false
		default:
			out.WriteRune(r)
			previousWasSeparator = false
		}
	}
	return strings.Trim(out.String(), "-")
}

func uniqueSchemaFields(fields []hbschema.Field) []hbschema.Field {
	byPath := make(map[string]hbschema.Field, len(fields))
	for _, field := range fields {
		existing, exists := byPath[field.Path]
		if !exists || yamlDocumentationPreferField(field, existing) {
			byPath[field.Path] = field
		}
	}
	out := make([]hbschema.Field, 0, len(byPath))
	for _, field := range byPath {
		out = append(out, field)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func yamlDocumentationPreferField(candidate hbschema.Field, existing hbschema.Field) bool {
	if candidate.Example != "" && existing.Example == "" {
		return true
	}
	if candidate.Description != "" && existing.Description == "" {
		return true
	}
	if candidate.Required && !existing.Required {
		return true
	}
	return false
}

func yamlDocumentationExampleRefs(example string) []string {
	re := regexp.MustCompile(`\{\!\{([^}]+)\}\}`)
	matches := re.FindAllStringSubmatch(example, -1)
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			refs = append(refs, match[1])
		}
	}
	return refs
}

func yamlDocumentationFixturePathForReference(ref string) (string, bool) {
	if !strings.HasSuffix(ref, ".hyperbricks") {
		return "", false
	}
	name := strings.TrimSuffix(ref, ".hyperbricks") + ".hyperbricks.yaml.test"
	return filepath.Join(yamlDocumentationFixtureDir, name), true
}
