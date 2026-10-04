package commands

import (
	"path/filepath"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestAuthorResponseStatusPolicyNativeValidation(t *testing.T) {
	for _, apiType := range []string{"api_render", "api_fragment_render"} {
		t.Run(apiType, func(t *testing.T) {
			for _, invalid := range []bool{false, true} {
				root := scaffoldFixtureModule(t)
				policy := map[string]interface{}{"enabled": false, "priority": 100, "map": map[string]interface{}{"404": 404, "409": "ignore"}}
				if invalid {
					policy["required"] = "true"
				}
				spec := authorSpec{Version: 2, Operation: "add-child", Target: "base.content", Brick: authorBrick{
					Name: "api_data", Type: apiType, Slot: "values", Properties: map[string]interface{}{
						"endpoint": "https://example.com/api", "method": "GET", "inline": "{{.Status}}", "response_status": policy,
					},
				}}
				if apiType == "api_fragment_render" {
					spec.Operation, spec.Target, spec.File, spec.Brick.Slot = "add-root", "", "app.hyperbricks.yaml", ""
					spec.Brick.Properties["route"] = "api-data"
				}
				result := runAuthor(t, root, spec, false)
				if invalid {
					if result.Status != "error" || !strings.Contains(result.Error, "required must be a boolean") {
						t.Fatalf("author accepted malformed policy: %+v", result)
					}
					continue
				}
				if result.Status != "created" {
					t.Fatalf("author could not create API policy: %+v", result)
				}
				processed, err := yamlparser.ProcessBytes([]byte(scaffoldRead(t, filepath.Join(root, "source", "app.hyperbricks.yaml"))), yamlparser.Options{})
				if err != nil {
					t.Fatal(err)
				}
				var api map[string]interface{}
				if apiType == "api_fragment_render" {
					api = processed.Materialized["api_data"].(map[string]interface{})
				} else {
					page := processed.Materialized["base"].(map[string]interface{})
					content := page["content"].(map[string]interface{})
					api = content["values"].(map[string]interface{})["api_data"].(map[string]interface{})
				}
				actual := api["response_status"].(map[string]interface{})
				if actual["enabled"] != false || actual["priority"] != 100 || actual["map"].(map[string]interface{})["404"] != 404 {
					t.Fatalf("author lost native policy values: %#v", actual)
				}
			}
		})
	}
}
