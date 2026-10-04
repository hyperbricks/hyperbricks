package schema

import (
	"strings"
	"testing"
)

func TestResponseStatusSchemaOnlyOnAPIComponents(t *testing.T) {
	catalog := ExtractRegistry(Definitions())
	for _, definition := range catalog.Types {
		isAPI := definition.Token == "<API_RENDER>" || definition.Token == "<API_FRAGMENT_RENDER>"
		for path, kind := range map[string]string{
			"response_status.enabled": "bool", "response_status.required": "bool",
			"response_status.priority": "int", "response_status.map": "map",
		} {
			field := findField(definition, path)
			if !isAPI {
				if field != nil {
					t.Errorf("%s exposes API-only field %s", definition.Token, path)
				}
				continue
			}
			if field == nil || field.Kind != kind {
				t.Fatalf("%s field %s = %#v, want %s", definition.Token, path, field, kind)
			}
			if field.Description == "" || field.Authoring == nil || field.Authoring.Group != "response_status" {
				t.Errorf("%s field %s lacks its description or authoring group: %#v", definition.Token, path, field)
			}
			if strings.HasSuffix(path, ".enabled") && (field.Authoring.Publish.OmitFalse || field.Authoring.Publish.OmitEmpty) {
				t.Errorf("%s must preserve explicit enabled:false", definition.Token)
			}
		}
	}
}
