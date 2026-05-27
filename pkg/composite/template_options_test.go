package composite

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/mapstructure"
)

func TestTemplateConfigMarshalJSONPreservesTopLevelFieldNames(t *testing.T) {
	input := map[string]interface{}{
		"@type":       TemplateConfigGetName(),
		"querykeys":   []interface{}{"somequeryparameter"},
		"queryparams": map[string]interface{}{"somequeryparameter": "helloworld"},
		"values":      map[string]interface{}{"width": "300"},
	}

	var config TemplateConfig
	if err := mapstructure.Decode(input, &config); err != nil {
		t.Fatalf("decode template config: %v", err)
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal template config: %v", err)
	}
	jsonString := string(encoded)

	for _, key := range []string{`"Template"`, `"Inline"`, `"AllowedQueryKeys"`, `"QueryParams"`, `"Values"`, `"Enclose"`} {
		if !strings.Contains(jsonString, key) {
			t.Fatalf("expected top-level key %s in %s", key, jsonString)
		}
	}
	for _, key := range []string{`"template"`, `"inline"`, `"querykeys"`, `"queryparams"`, `"values"`, `"enclose"`} {
		if strings.Contains(jsonString, key) {
			t.Fatalf("did not expect lowercase top-level key %s in %s", key, jsonString)
		}
	}
}

func TestHypermediaTemplateOptionsMarshalAsNestedMapShape(t *testing.T) {
	input := map[string]interface{}{
		"@type": HyperMediaConfigGetName(),
		"template": map[string]interface{}{
			"inline": "<main>{{.content}}</main>",
			"values": map[string]interface{}{
				"content": "hello",
			},
		},
	}

	var config HyperMediaConfig
	if err := mapstructure.Decode(input, &config); err != nil {
		t.Fatalf("decode hypermedia config: %v", err)
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal hypermedia config: %v", err)
	}
	jsonString := string(encoded)

	for _, key := range []string{`"Template"`, `"inline"`, `"values"`} {
		if !strings.Contains(jsonString, key) {
			t.Fatalf("expected nested template key %s in %s", key, jsonString)
		}
	}
	for _, key := range []string{`"Inline"`, `"Values"`} {
		if strings.Contains(jsonString, key) {
			t.Fatalf("did not expect exported nested template key %s in %s", key, jsonString)
		}
	}
}

func TestFragmentTemplateOptionsMarshalAsNestedMapShape(t *testing.T) {
	input := map[string]interface{}{
		"@type": FragmentConfigGetName(),
		"template": map[string]interface{}{
			"inline": "<section>{{.content}}</section>",
			"values": map[string]interface{}{
				"content": "hello",
			},
		},
	}

	var config FragmentConfig
	if err := mapstructure.Decode(input, &config); err != nil {
		t.Fatalf("decode fragment config: %v", err)
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal fragment config: %v", err)
	}
	jsonString := string(encoded)

	for _, key := range []string{`"Template"`, `"inline"`, `"values"`} {
		if !strings.Contains(jsonString, key) {
			t.Fatalf("expected nested template key %s in %s", key, jsonString)
		}
	}
	for _, key := range []string{`"Inline"`, `"Values"`} {
		if strings.Contains(jsonString, key) {
			t.Fatalf("did not expect exported nested template key %s in %s", key, jsonString)
		}
	}
}
