package composite

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestRouteCacheSerializedConfigurationCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  map[string]interface{}
		want string
	}{
		{"omitted", nil, `""`},
		{"scalar", map[string]interface{}{"cache": "30s"}, `"30s"`},
		{"zero", map[string]interface{}{"cache": "0s"}, `"0s"`},
		{"mapping", map[string]interface{}{"cache": map[string]interface{}{"storage": "disk", "expire": "30s"}}, `{"storage":"disk","expire":"30s"}`},
		{"empty mapping", map[string]interface{}{"cache": map[string]interface{}{}}, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var config HyperMediaConfig
			if err := decodeRouteRenderConfig(tc.raw, &config); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(config.Cache)
			if err != nil || string(got) != tc.want {
				t.Fatalf("serialized cache = %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestResolveRouteCachePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  map[string]interface{}
		want RouteCachePolicy
	}{
		{"omitted", nil, RouteCachePolicy{"mem", 10 * time.Minute}},
		{"scalar", map[string]interface{}{"cache": "30s"}, RouteCachePolicy{"mem", 30 * time.Second}},
		{"memory mapping", map[string]interface{}{"cache": map[string]interface{}{"storage": "mem", "expire": "30s"}}, RouteCachePolicy{"mem", 30 * time.Second}},
		{"disk inherited expiry", map[string]interface{}{"cache": map[string]interface{}{"storage": "disk"}}, RouteCachePolicy{"disk", 10 * time.Minute}},
		{"empty mapping", map[string]interface{}{"cache": map[string]interface{}{}}, RouteCachePolicy{"mem", 10 * time.Minute}},
		{"zero", map[string]interface{}{"cache": "0s"}, RouteCachePolicy{"mem", 0}},
		{"disk zero", map[string]interface{}{"cache": map[string]interface{}{"storage": "disk", "expire": "0s"}}, RouteCachePolicy{"disk", 0}},
		{"esbuild independent", map[string]interface{}{"@type": "<ESBUILD>", "cache": true}, RouteCachePolicy{"mem", 10 * time.Minute}},
		{"api independent", map[string]interface{}{"@type": "<API_FRAGMENT_RENDER>", "cache": true}, RouteCachePolicy{"mem", 10 * time.Minute}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveRouteCache(tc.raw, 10*time.Minute)
			if err != nil || got != tc.want {
				t.Fatalf("policy = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func BenchmarkResolveRouteCache(b *testing.B) {
	for _, tc := range []struct {
		name string
		raw  map[string]interface{}
	}{
		{"default", map[string]interface{}{"@type": "<HYPERMEDIA>"}},
		{"scalar", map[string]interface{}{"@type": "<HYPERMEDIA>", "cache": "30s"}},
		{"mapping", map[string]interface{}{"@type": "<HYPERMEDIA>", "cache": map[string]interface{}{"storage": "disk", "expire": "30s"}}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ResolveRouteCache(tc.raw, time.Minute); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestRouteCacheConfigStrictAcrossDecodePaths(t *testing.T) {
	invalid := []interface{}{
		nil, "", "eventually", "-1s", true, 0, []interface{}{"30s"},
		map[string]interface{}{"storage": ""}, map[string]interface{}{"storage": nil}, map[string]interface{}{"storage": "file"},
		map[string]interface{}{"expire": ""}, map[string]interface{}{"expire": nil}, map[string]interface{}{"expire": 0},
		map[string]interface{}{"expire": true}, map[string]interface{}{"expire": "-1s"}, map[string]interface{}{"expires": "30s"},
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(HyperMediaConfig{}), reflect.TypeOf(FragmentConfig{})} {
		factory := typefactory.NewTypeFactory()
		factory.RegisterType("route", typ)
		for _, value := range invalid {
			raw := map[string]interface{}{"cache": value}
			if _, err := ResolveRouteCache(raw, time.Minute); err == nil {
				t.Errorf("%s accepted raw %#v", typ, value)
			}
			if _, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: "route", Data: raw}); err == nil {
				t.Errorf("%s factory accepted %#v", typ, value)
			}
			if err := decodeRouteRenderConfig(raw, reflect.New(typ).Interface()); err == nil {
				t.Errorf("%s direct decoder accepted %#v", typ, value)
			}
			if errs := shared.DecodeWithBasicHooks(raw, reflect.New(typ).Interface()); len(errs) == 0 {
				t.Errorf("%s basic decoder accepted %#v", typ, value)
			}
		}
		for _, value := range []interface{}{"30s", map[string]interface{}{"storage": "disk", "expire": "0s"}, map[string]interface{}{}} {
			raw := map[string]interface{}{"cache": value}
			response, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: "route", Data: raw})
			if err != nil {
				t.Fatalf("%s factory rejected %#v: %v", typ, value, err)
			}
			direct := reflect.New(typ)
			if err := decodeRouteRenderConfig(raw, direct.Interface()); err != nil {
				t.Fatal(err)
			}
			factoryCache := reflect.ValueOf(response.Instance).FieldByName("Cache").Interface()
			directCache := direct.Elem().FieldByName("Cache").Interface()
			if !reflect.DeepEqual(factoryCache, directCache) {
				t.Fatalf("decoder policy mismatch: %+v != %+v", factoryCache, directCache)
			}
		}
	}
}

func TestRouteCacheInheritanceAndScalarReplacement(t *testing.T) {
	result, err := yamlparser.ProcessBytes([]byte(`
base:
  - type: hypermedia
  - cache: {storage: disk, expire: 1h}
child:
  - inherit: base
  - cache: {expire: 30s}
empty_child:
  - inherit: base
  - cache: {}
scalar_child:
  - inherit: base
  - cache: 45s
disabled_child:
  - inherit: base
  - cache: {expire: 0s}
`), yamlparser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]RouteCachePolicy{
		"base": {"disk", time.Hour}, "child": {"disk", 30 * time.Second}, "empty_child": {"disk", time.Hour},
		"scalar_child": {"mem", 45 * time.Second}, "disabled_child": {"disk", 0},
	} {
		raw, ok := result.Materialized[name].(map[string]interface{})
		if !ok {
			t.Fatalf("%s materialized as %#v", name, result.Materialized[name])
		}
		got, err := ResolveRouteCache(raw, 10*time.Minute)
		if err != nil || got != want {
			t.Errorf("%s policy = %+v, %v; want %+v", name, got, err, want)
		}
	}
}

func TestRouteCacheYAMLRejectsNullAndInvalidTypes(t *testing.T) {
	for _, value := range []string{"null", "false", "0", "{expire: 0}", "{expire: null}", "{storage: null}", "{storage: disk, typo: 1m}"} {
		result, err := yamlparser.ProcessBytes([]byte("page:\n  - type: hypermedia\n  - cache: "+value+"\n"), yamlparser.Options{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = ResolveRouteCache(result.Materialized["page"].(map[string]interface{}), time.Minute)
		if err == nil || !strings.Contains(err.Error(), "cache") {
			t.Errorf("cache: %s produced error %v", value, err)
		}
	}
}
