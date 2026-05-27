package schema

import "testing"

func TestRegistryContainsRuntimeTypes(t *testing.T) {
	schema := ExtractRegistry(Definitions())
	for _, token := range []string{
		"<API_FRAGMENT_RENDER>",
		"<API_RENDER>",
		"<CSS>",
		"<FRAGMENT>",
		"<HEAD>",
		"<HTML>",
		"<HYPERMEDIA>",
		"<IMAGE>",
		"<IMAGES>",
		"<JS>",
		"<JSON_RENDER>",
		"<MENU>",
		"<PLUGIN>",
		"<STYLES>",
		"<TEMPLATE>",
		"<TEXT>",
		"<TREE>",
	} {
		if findType(schema, token) == nil {
			t.Fatalf("missing schema type %s", token)
		}
	}
}

func TestRegistryRepresentsAliases(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	js := findType(schema, "<JS>")
	if js == nil {
		t.Fatal("missing <JS>")
	}
	if !contains(js.Aliases, "<JAVASCRIPT>") {
		t.Fatalf("expected <JS> alias <JAVASCRIPT>, got %v", js.Aliases)
	}

	jsonRender := findType(schema, "<JSON_RENDER>")
	if jsonRender == nil {
		t.Fatal("missing <JSON_RENDER>")
	}
	if !contains(jsonRender.Aliases, "<JSON>") {
		t.Fatalf("expected <JSON_RENDER> alias <JSON>, got %v", jsonRender.Aliases)
	}
}

func TestExtractorFindsNestedTemplateFields(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	for _, token := range []string{"<HYPERMEDIA>", "<FRAGMENT>"} {
		typeSchema := findType(schema, token)
		if typeSchema == nil {
			t.Fatalf("missing %s", token)
		}
		for _, path := range []string{
			"template.template",
			"template.inline",
			"template.querykeys",
			"template.queryparams",
			"template.values",
			"template.enclose",
		} {
			if findField(*typeSchema, path) == nil {
				t.Fatalf("%s missing field %s", token, path)
			}
		}
	}
}

func TestExtractorFindsGuardAndResponseFields(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	hypermedia := findType(schema, "<HYPERMEDIA>")
	if hypermedia == nil {
		t.Fatal("missing <HYPERMEDIA>")
	}
	for _, path := range []string{
		"guard.auth.cookie",
		"guard.require.authenticated",
		"guard.authorize.endpoint",
		"guard.on_unauthenticated.hx_redirect",
		"guard.on_forbidden.status",
	} {
		if findField(*hypermedia, path) == nil {
			t.Fatalf("hypermedia missing field %s", path)
		}
	}

	fragment := findType(schema, "<FRAGMENT>")
	if fragment == nil {
		t.Fatal("missing <FRAGMENT>")
	}
	if findField(*fragment, "response.hx_trigger") == nil {
		t.Fatal("fragment missing response.hx_trigger")
	}

	apiFragment := findType(schema, "<API_FRAGMENT_RENDER>")
	if apiFragment == nil {
		t.Fatal("missing <API_FRAGMENT_RENDER>")
	}
	if findField(*apiFragment, "response.hx_trigger") == nil {
		t.Fatal("api fragment missing response.hx_trigger")
	}
}

func TestExtractorSkipsInternalAndRemainFields(t *testing.T) {
	schema := ExtractRegistry(Definitions())
	for _, typeSchema := range schema.Types {
		for _, forbidden := range []string{
			"@type",
			"hyperbrickskey",
			"hyperbrickspath",
			"hyperbricksfile",
			"hx_response",
		} {
			if field := findField(typeSchema, forbidden); field != nil {
				t.Fatalf("%s should not expose internal field %s", typeSchema.Token, forbidden)
			}
		}
		if findField(typeSchema, "") != nil {
			t.Fatalf("%s exposed empty field path", typeSchema.Token)
		}
	}
}

func TestExtractorMarksRequiredAndValueMapFields(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	text := findType(schema, "<TEXT>")
	if text == nil {
		t.Fatal("missing <TEXT>")
	}
	value := findField(*text, "value")
	if value == nil {
		t.Fatal("text missing value")
	}
	if !value.Required {
		t.Fatal("text.value should be required")
	}

	template := findType(schema, "<TEMPLATE>")
	if template == nil {
		t.Fatal("missing <TEMPLATE>")
	}
	if template.Category != CategoryComponent {
		t.Fatalf("template category = %s, want %s", template.Category, CategoryComponent)
	}
	if template.ChildModel != ChildModelValues {
		t.Fatalf("template child model = %s, want %s", template.ChildModel, ChildModelValues)
	}
	values := findField(*template, "values")
	if values == nil {
		t.Fatal("template missing values")
	}
	if !values.ValueDynamic {
		t.Fatal("template.values should be marked dynamic")
	}
}

func TestExtractorAddsPublishRules(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	head := findType(schema, "<HEAD>")
	if head == nil {
		t.Fatal("missing <HEAD>")
	}
	if findField(*head, "enclose") != nil {
		t.Fatal("head should not expose enclose as a publishable field")
	}

	html := findType(schema, "<HTML>")
	if html == nil {
		t.Fatal("missing <HTML>")
	}
	enclose := findField(*html, "enclose")
	if enclose == nil {
		t.Fatal("html missing enclose")
	}
	if enclose.Authoring == nil || enclose.Authoring.Publish == nil || !enclose.Authoring.Publish.OmitEmpty || !enclose.Authoring.Publish.OmitFalse {
		t.Fatalf("html.enclose publish rules = %#v, want omit empty and false", enclose.Authoring)
	}

	value := findField(*html, "value")
	if value == nil {
		t.Fatal("html missing value")
	}
	if value.Authoring == nil || value.Authoring.Publish == nil || !value.Authoring.Publish.OmitEmpty {
		t.Fatalf("html.value publish rules = %#v, want omit empty", value.Authoring)
	}
	if value.Authoring.Publish.OmitFalse {
		t.Fatalf("html.value should preserve false-like scalar values")
	}

	trimspace := findField(*html, "trimspace")
	if trimspace == nil {
		t.Fatal("html missing trimspace")
	}
	if trimspace.Authoring == nil || trimspace.Authoring.Publish == nil || !trimspace.Authoring.Publish.OmitFalse {
		t.Fatalf("html.trimspace publish rules = %#v, want omit false", trimspace.Authoring)
	}
}

func TestExtractorAddsComposerAuthoringMetadata(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	hypermedia := findType(schema, "<HYPERMEDIA>")
	if hypermedia == nil {
		t.Fatal("missing <HYPERMEDIA>")
	}
	if hypermedia.Authoring == nil {
		t.Fatal("hypermedia missing authoring metadata")
	}
	headSlot := findSlot(hypermedia.Authoring.Slots, "head")
	if headSlot == nil {
		t.Fatal("hypermedia authoring missing head slot")
	}
	if headSlot.Model != string(ChildModelHead) || headSlot.Default {
		t.Fatalf("hypermedia head slot = %#v, want non-default head model", headSlot)
	}
	bodySlot := findSlot(hypermedia.Authoring.Slots, "body")
	if bodySlot == nil {
		t.Fatal("hypermedia authoring missing body slot")
	}
	if bodySlot.Model != string(ChildModelTree) || !bodySlot.Default {
		t.Fatalf("hypermedia body slot = %#v, want default tree model", bodySlot)
	}

	template := findType(schema, "<TEMPLATE>")
	if template == nil {
		t.Fatal("missing <TEMPLATE>")
	}
	if template.Authoring == nil {
		t.Fatal("template missing authoring metadata")
	}
	slot := findSlot(template.Authoring.Slots, "values")
	if slot == nil {
		t.Fatal("template authoring missing values slot")
	}
	if slot.Model != string(ChildModelValues) || !slot.PathKeyRequired {
		t.Fatalf("template values slot = %#v, want values model with required path key", slot)
	}

	values := findField(*template, "values")
	if values == nil {
		t.Fatal("template missing values")
	}
	if values.Authoring == nil || values.Authoring.Control != "template-values" || values.Authoring.Group != "template" {
		t.Fatalf("template.values authoring = %#v, want template-values control in template group", values.Authoring)
	}

	templateField := findField(*template, "template")
	if templateField == nil {
		t.Fatal("template missing template field")
	}
	if templateField.Authoring == nil || templateField.Authoring.Control != "template-asset" || templateField.Authoring.Label != "Template" {
		t.Fatalf("template.template authoring = %#v, want template asset control and label", templateField.Authoring)
	}
}

func TestExtractorAppliesFieldAuthoringOverrides(t *testing.T) {
	schema := ExtractRegistry(Definitions())

	menu := findType(schema, "<MENU>")
	if menu == nil {
		t.Fatal("missing <MENU>")
	}
	for _, path := range []string{"active", "item"} {
		field := findField(*menu, path)
		if field == nil {
			t.Fatalf("menu missing %s", path)
		}
		if field.Authoring == nil || field.Authoring.Control != "textarea" {
			t.Fatalf("menu.%s authoring = %#v, want textarea control", path, field.Authoring)
		}
	}

	section := findField(*menu, "section")
	if section == nil {
		t.Fatal("menu missing section")
	}
	if section.Authoring == nil || section.Authoring.Control != "text" {
		t.Fatalf("menu.section authoring = %#v, want inferred text control", section.Authoring)
	}
}

func findType(schema ComponentSchema, token string) *TypeSchema {
	for i := range schema.Types {
		if schema.Types[i].Token == token {
			return &schema.Types[i]
		}
	}
	return nil
}

func findField(typeSchema TypeSchema, path string) *Field {
	for i := range typeSchema.Fields {
		if typeSchema.Fields[i].Path == path {
			return &typeSchema.Fields[i]
		}
	}
	return nil
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func findSlot(slots []AuthoringSlot, key string) *AuthoringSlot {
	for i := range slots {
		if slots[i].Key == key {
			return &slots[i]
		}
	}
	return nil
}
