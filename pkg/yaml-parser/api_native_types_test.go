package yamlparser

import (
	"reflect"
	"testing"
)

func TestAPISecurityFieldsMaterializeNativeYAMLTypes(t *testing.T) {
	doc, err := ParseBytes([]byte(`
api_fragment:
  - forwardtoken: false
  - setcookie: 42
  - setcookies:
      - name: session
        value: token
        secure: true
        http_only: false
        max_age: 0
      - null
  - type: api_fragment_render
`))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if got := doc.Roots[0].Props["forwardtoken"]; got != "false" {
		t.Fatalf("legacy Props forwardtoken = %#v, want stringified scalar", got)
	}
	if got := doc.Roots[0].nativeAPIProps["forwardtoken"]; got != false {
		t.Fatalf("native forwardtoken = %#v, want bool false", got)
	}

	materialized, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	api := materializedNodeMap(t, materialized, "api_fragment")
	if got := api["forwardtoken"]; got != false {
		t.Fatalf("forwardtoken = %#v (%T), want bool false", got, got)
	}
	if got := api["setcookie"]; got != 42 {
		t.Fatalf("setcookie = %#v (%T), want int 42", got, got)
	}
	entries, ok := api["setcookies"].([]interface{})
	if !ok || len(entries) != 2 {
		t.Fatalf("setcookies = %#v, want two native entries", api["setcookies"])
	}
	cookie, ok := entries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("setcookies[0] = %#v (%T), want map", entries[0], entries[0])
	}
	wantCookie := map[string]interface{}{
		"name": "session", "value": "token", "secure": true, "http_only": false, "max_age": 0,
	}
	if !reflect.DeepEqual(cookie, wantCookie) {
		t.Fatalf("setcookies[0] = %#v, want %#v", cookie, wantCookie)
	}
	if entries[1] != nil {
		t.Fatalf("setcookies[1] = %#v, want nil", entries[1])
	}
}

func TestAPISecurityFieldsKeepQuotedScalarsAsStrings(t *testing.T) {
	result, err := ProcessBytes([]byte(`
api_fragment:
  - type: api_fragment_render
  - forwardtoken: "false"
  - setcookie: "42"
  - setcookies:
      - name: session
        value: token
        secure: "true"
        http_only: "false"
        max_age: "0"
      - "null"
`), Options{})
	if err != nil {
		t.Fatalf("ProcessBytes: %v", err)
	}
	api := materializedNodeMap(t, result.Materialized, "api_fragment")
	if api["forwardtoken"] != "false" || api["setcookie"] != "42" {
		t.Fatalf("quoted direct values changed type: forwardtoken=%#v setcookie=%#v", api["forwardtoken"], api["setcookie"])
	}
	entries := api["setcookies"].([]interface{})
	cookie := entries[0].(map[string]interface{})
	for field, want := range map[string]string{"secure": "true", "http_only": "false", "max_age": "0"} {
		if got := cookie[field]; got != want {
			t.Fatalf("quoted %s = %#v (%T), want string %q", field, got, got, want)
		}
	}
	if entries[1] != "null" {
		t.Fatalf("quoted null = %#v (%T), want string", entries[1], entries[1])
	}
}

func TestNativeAPIPropertiesSurviveInheritedTypeAndOverrides(t *testing.T) {
	result, err := ProcessBytes([]byte(`
base_api:
  - type: api_fragment_render
  - forwardtoken: base_session
  - setcookie: false
  - setcookies:
      - name: base
        value: base-token
        secure: true
        max_age: 60

derived_api:
  - setcookie: null
  - setcookies:
      - secure: false
  - forwardtoken: 123
  - inherit: base_api
`), Options{})
	if err != nil {
		t.Fatalf("ProcessBytes: %v", err)
	}

	base := materializedNodeMap(t, result.Materialized, "base_api")
	baseCookie := base["setcookies"].([]interface{})[0].(map[string]interface{})
	if base["forwardtoken"] != "base_session" || base["setcookie"] != false || baseCookie["secure"] != true || baseCookie["max_age"] != 60 {
		t.Fatalf("base API native values = %#v", base)
	}

	derived := materializedNodeMap(t, result.Materialized, "derived_api")
	if derived["@type"] != "<API_FRAGMENT_RENDER>" {
		t.Fatalf("derived type = %#v", derived["@type"])
	}
	if derived["forwardtoken"] != 123 {
		t.Fatalf("inherited override forwardtoken = %#v (%T), want int", derived["forwardtoken"], derived["forwardtoken"])
	}
	if value, exists := derived["setcookie"]; !exists || value != nil {
		t.Fatalf("inherited override setcookie = %#v (exists %t), want explicit nil", value, exists)
	}
	entries, ok := derived["setcookies"].([]interface{})
	if !ok || len(entries) != 1 {
		t.Fatalf("inherited override setcookies = %#v", derived["setcookies"])
	}
	cookie, ok := entries[0].(map[string]interface{})
	if !ok || cookie["secure"] != false {
		t.Fatalf("inherited override cookie = %#v, want native false", entries[0])
	}
	if _, leakedChild := derived["@order"]; leakedChild {
		t.Fatalf("ambiguous inherited setcookies entry materialized as a child: %#v", derived)
	}
}

func TestNestedAPINodesMaterializeNativeSecurityFields(t *testing.T) {
	result, err := ProcessBytes([]byte(`
page:
  - type: tree
  - api_call:
      - forwardtoken: true
      - setcookies:
          - name: nested
            value: token
            secure: false
            max_age: 15
      - type: api_render
`), Options{})
	if err != nil {
		t.Fatalf("ProcessBytes: %v", err)
	}
	page := materializedNodeMap(t, result.Materialized, "page")
	nested := materializedNodeMap(t, page, "api_call")
	if nested["forwardtoken"] != true {
		t.Fatalf("nested forwardtoken = %#v (%T), want bool", nested["forwardtoken"], nested["forwardtoken"])
	}
	cookie := nested["setcookies"].([]interface{})[0].(map[string]interface{})
	if cookie["secure"] != false || cookie["max_age"] != 15 {
		t.Fatalf("nested structured cookie = %#v", cookie)
	}
}

func TestNativeAPIPropertiesFollowTheResolvedTypeAcrossInheritance(t *testing.T) {
	result, err := ProcessBytes([]byte(`
ordinary_base:
  - type: tree
  - forwardtoken: false
  - setcookie: 12

promoted_api:
  - inherit: ordinary_base
  - type: api_render

api_base:
  - type: api_render
  - forwardtoken: true
  - setcookie: 24

demoted_tree:
  - inherit: api_base
  - type: tree
`), Options{})
	if err != nil {
		t.Fatalf("ProcessBytes: %v", err)
	}

	promoted := materializedNodeMap(t, result.Materialized, "promoted_api")
	if promoted["forwardtoken"] != false || promoted["setcookie"] != 12 {
		t.Fatalf("promoted API did not receive native inherited values: %#v", promoted)
	}

	demoted := materializedNodeMap(t, result.Materialized, "demoted_tree")
	if demoted["forwardtoken"] != "true" || demoted["setcookie"] != "24" {
		t.Fatalf("non-API resolved type received native values: %#v", demoted)
	}
}

func TestSameNamedNonAPIValuesRetainLegacyScalarMaterialization(t *testing.T) {
	result, err := ProcessBytes([]byte(`
ordinary:
  - type: tree
  - forwardtoken: false
  - setcookie: 42
  - setcookies: [true, 7, null]
  - values:
      forwardtoken: false
      setcookie: 9
      setcookies: [false, 11, null]
`), Options{})
	if err != nil {
		t.Fatalf("ProcessBytes: %v", err)
	}
	ordinary := materializedNodeMap(t, result.Materialized, "ordinary")
	if ordinary["forwardtoken"] != "false" || ordinary["setcookie"] != "42" {
		t.Fatalf("non-API direct scalars changed: %#v", ordinary)
	}
	if got, want := ordinary["setcookies"], []interface{}{"true", "7", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("non-API setcookies = %#v, want %#v", got, want)
	}
	values := ordinary["values"].(map[string]interface{})
	if values["forwardtoken"] != "false" || values["setcookie"] != "9" {
		t.Fatalf("nested same-named data changed: %#v", values)
	}
	if got, want := values["setcookies"], []interface{}{"false", "11", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nested same-named setcookies = %#v, want %#v", got, want)
	}
}

func materializedNodeMap(t *testing.T, parent map[string]interface{}, name string) map[string]interface{} {
	t.Helper()
	value, exists := parent[name]
	if !exists {
		t.Fatalf("materialized node %q is missing from %#v", name, parent)
	}
	result, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("materialized node %q = %#v (%T), want map", name, value, value)
	}
	return result
}
