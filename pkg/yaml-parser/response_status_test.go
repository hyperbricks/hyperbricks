package yamlparser

import (
	"fmt"
	"reflect"
	"testing"
)

func TestResponseStatusNativeTypesAndInheritanceOwnership(t *testing.T) {
	for _, includeMetadata := range []bool{false, true} {
		for _, apiType := range []string{"api_render", "api_fragment_render"} {
			t.Run(fmt.Sprintf("%s/metadata=%t", apiType, includeMetadata), func(t *testing.T) {
				result, err := ProcessBytes([]byte(fmt.Sprintf(`
api:
  - type: %s
  - endpoint: https://example.com/data
  - response_status:
      required: true
      priority: 100
      map: {"404": 404, "503": 503}
copy:
  - inherit: api
local:
  - inherit: api
  - response_status: {enabled: false}
page:
  - type: hypermedia
  - feed:
      - inherit: api
      - response_status: {required: true, priority: 50, map: {"410": 410}}
  - retry:
      - inherit: api
  - content:
      - type: template
      - values:
          item:
            - inherit: api
            - response_status: {map: {"404": ignore}}
page_copy:
  - inherit: page
feed_copy:
  - inherit: page.feed
`, apiType)), Options{IncludeSourceMetadata: includeMetadata})
				if err != nil {
					t.Fatal(err)
				}
				api := materializedNodeMap(t, result.Materialized, "api")
				policy := materializedNodeMap(t, api, "response_status")
				if policy["required"] != true || policy["priority"] != 100 || materializedNodeMap(t, policy, "map")["404"] != 404 {
					t.Fatalf("policy lost native scalar types: %#v", policy)
				}
				for _, name := range []string{"copy", "feed_copy"} {
					if copied := materializedNodeMap(t, result.Materialized, name); copied["response_status"] != nil {
						t.Fatalf("direct API inheritance imported policy into %s: %#v", name, copied)
					}
				}
				local := materializedNodeMap(t, result.Materialized, "local")
				if got, want := local["response_status"], map[string]interface{}{"enabled": false}; !reflect.DeepEqual(got, want) {
					t.Fatalf("local policy merged inherited fields: got %#v, want %#v", got, want)
				}
				for _, pageName := range []string{"page", "page_copy"} {
					page := materializedNodeMap(t, result.Materialized, pageName)
					feed := materializedNodeMap(t, page, "feed")
					policy := materializedNodeMap(t, feed, "response_status")
					if got, want := policy, (map[string]interface{}{"required": true, "priority": 50, "map": map[string]interface{}{"410": 410}}); !reflect.DeepEqual(got, want) {
						t.Fatalf("%s lost mounted policy: %#v", pageName, got)
					}
					if retry := materializedNodeMap(t, page, "retry"); retry["response_status"] != nil {
						t.Fatalf("%s retry imported policy: %#v", pageName, retry)
					}
					content := materializedNodeMap(t, page, "content")
					values := materializedNodeMap(t, content, "values")
					item := materializedNodeMap(t, values, "item")
					if got := materializedNodeMap(t, materializedNodeMap(t, item, "response_status"), "map")["404"]; got != "ignore" {
						t.Fatalf("%s lost value-mounted policy: %#v", pageName, item)
					}
				}
			})
		}
	}
}

func TestResponseStatusQuotedTypesRemainInvalidAndOrdinaryDataIsPreserved(t *testing.T) {
	result, err := ProcessBytes([]byte(`
api:
  - type: api_render
  - response_status: {enabled: "false", required: "true", priority: "10", map: {"404": "404"}}
ordinary:
  - type: tree
  - values:
      response_status: {enabled: false}
copy:
  - inherit: ordinary
`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	policy := materializedNodeMap(t, materializedNodeMap(t, result.Materialized, "api"), "response_status")
	if policy["enabled"] != "false" || policy["required"] != "true" || policy["priority"] != "10" || materializedNodeMap(t, policy, "map")["404"] != "404" {
		t.Fatalf("quoted policy scalar types changed: %#v", policy)
	}
	copy := materializedNodeMap(t, result.Materialized, "copy")
	values := materializedNodeMap(t, copy, "values")
	if policy := materializedNodeMap(t, values, "response_status"); policy["enabled"] != "false" {
		t.Fatalf("ordinary inherited data changed: %#v", copy)
	}
}
