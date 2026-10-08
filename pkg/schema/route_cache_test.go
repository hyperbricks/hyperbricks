package schema

import (
	"reflect"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

func TestRouteCacheSchemaOwnsTypedFields(t *testing.T) {
	schema := ExtractRegistry(Definitions())
	for _, token := range []string{"<HYPERMEDIA>", "<FRAGMENT>"} {
		typ := findType(schema, token)
		for path, kind := range map[string]string{"cache": "object", "cache.storage": "string", "cache.expire": "string"} {
			field := findField(*typ, path)
			if field == nil || field.Kind != kind || field.Description == "" {
				t.Errorf("%s %s = %+v; want documented %s", token, path, field, kind)
			}
		}
		choices := findField(*typ, "cache.storage").AllowedValues()
		if !reflect.DeepEqual(choices, []string{composite.RouteCacheMemory, composite.RouteCacheDisk}) {
			t.Fatalf("%s storage choices = %v", token, choices)
		}
		for _, choice := range choices {
			policy, err := composite.ResolveRouteCache(map[string]interface{}{
				"@type": token, "cache": map[string]interface{}{"storage": choice},
			}, time.Minute)
			if err != nil || policy.Storage != choice {
				t.Fatalf("%s editor choice %q rejected by runtime: %+v, %v", token, choice, policy, err)
			}
		}
	}
	for _, token := range []string{"<API_RENDER>", "<API_FRAGMENT_RENDER>", "<ESBUILD>"} {
		typ := findType(schema, token)
		if findField(*typ, "cache.storage") != nil || findField(*typ, "cache.expire") != nil {
			t.Errorf("%s exposes route cache fields", token)
		}
	}
	if field := findField(*findType(schema, "<ESBUILD>"), "cache"); field == nil || field.Kind != "bool" {
		t.Fatalf("esbuild.cache changed: %+v", field)
	}
}
