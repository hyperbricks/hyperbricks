package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"github.com/mitchellh/mapstructure"
)

func TestRuntimeSourceMetadataIsDevelopmentOnly(t *testing.T) {
	config := getHyperBricksConfiguration()
	previous := config.Mode
	t.Cleanup(func() { config.Mode = previous })
	for _, mode := range []string{shared.LIVE_MODE, shared.DEVELOPMENT_MODE, shared.DEBUG_MODE} {
		t.Run(mode, func(t *testing.T) {
			config.Mode = mode
			opts := yamlRuntimeOptions()
			wantSource := mode != shared.LIVE_MODE
			if opts.IncludeSourceMetadata != wantSource {
				t.Fatalf("source metadata enabled=%t for mode %s", opts.IncludeSourceMetadata, mode)
			}
			result, err := yamlparser.ProcessBytes([]byte("page:\n  - type: hypermedia\n  - route: index\n  - content:\n      - type: text\n      - value: Hello\n"), opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := prepareSourceMetadata(result.Materialized); err != nil {
				t.Fatal(err)
			}
			page := result.Materialized["page"].(map[string]interface{})
			for name, raw := range map[string]map[string]interface{}{"page": page, "content": page["content"].(map[string]interface{})} {
				if hasSource := shared.MetaFromConfig(raw).Source != nil; hasSource != wantSource {
					t.Fatalf("%s source present=%t in mode %s", name, hasSource, mode)
				}
			}
		})
	}
}

func TestPrepareSourceMetadataPreservesLocationsAndRequestIsolation(t *testing.T) {
	child := map[string]interface{}{"@type": "<TEMPLATE>", "values": map[string]interface{}{"title": "original"},
		"@source": map[string]interface{}{"file": "hyperbricks/imports/base.hyperbricks.yaml", "line": 12, "column": 3,
			"path": "page.template", "key": "template", "resources": map[string]interface{}{"template": "templates/page.html"},
			"fields": map[string]interface{}{"template": map[string]interface{}{"file": "hyperbricks/imports/base.hyperbricks.yaml", "line": 13, "column": 7}}}}
	raw := map[string]interface{}{"nested": []interface{}{child}}
	before := shared.MetaFromConfig(child)
	beforeJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareSourceMetadata(raw); err != nil {
		t.Fatal(err)
	}
	if err := prepareSourceMetadata(raw); err != nil {
		t.Fatal(err)
	}
	after := shared.MetaFromConfig(child)
	afterJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	var beforeValue, afterValue interface{}
	if err := json.Unmarshal(beforeJSON, &beforeValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(afterJSON, &afterValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeValue, afterValue) {
		t.Fatalf("preparation changed metadata JSON: before=%s after=%s", beforeJSON, afterJSON)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("preparation changed metadata: before=%#v after=%#v", before, after)
	}
	clone := shared.CloneMapDeep(child)
	if clone["@source"] != child["@source"] {
		t.Fatal("immutable source metadata was copied")
	}
	clone["values"].(map[string]interface{})["title"] = "request-local"
	if child["values"].(map[string]interface{})["title"] != "original" {
		t.Fatal("ordinary request data is no longer isolated")
	}
	var decoded shared.Meta
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &decoded, WeaklyTypedInput: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(clone); err != nil || !reflect.DeepEqual(decoded, before) {
		t.Fatalf("typed source decode failed: meta=%#v err=%v", decoded, err)
	}
	issue := shared.ResourceDiagnostic(errors.New("template failed"), decoded, "render", "template")
	if issue.Line != 13 || issue.Column != 7 || issue.File != before.Source.File || issue.Resource != "templates/page.html" {
		t.Fatalf("prepared error location = %#v", issue)
	}
}

func TestPrepareSourceMetadataRejectsInvalidInternalMetadata(t *testing.T) {
	err := prepareSourceMetadata(map[string]interface{}{"@source": map[string]interface{}{"line": []string{"invalid"}}})
	if err == nil {
		t.Fatal("invalid metadata was silently accepted")
	}
}
