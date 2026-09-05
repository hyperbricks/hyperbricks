package renderplan_test

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestNeedsAPIRequestContextRawDetection(t *testing.T) {
	api := map[string]interface{}{"@type": component.APIConfigGetName()}
	for _, tc := range []struct {
		name string
		raw  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"nil_map", map[string]interface{}(nil), false},
		{"scalar", component.APIConfigGetName(), false},
		{"plain", map[string]interface{}{"@type": composite.HyperMediaConfigGetName()}, false},
		{"api", api, true},
		{"api_fragment", map[string]interface{}{"@type": composite.ApiFragmentRenderConfigGetName()}, true},
		{"nested_map", map[string]interface{}{"data": map[string]interface{}{"api": api}}, true},
		{"nested_slice", []interface{}{nil, "text", map[string]interface{}{"data": []interface{}{api}}}, true},
		{"non_string_type", map[string]interface{}{"@type": 12, "child": api}, true},
		{"typed_slice_unchanged", []map[string]interface{}{api}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderplan.NeedsAPIRequestContext(tc.raw); got != tc.want {
				t.Fatalf("NeedsAPIRequestContext = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompiledAPIRequestRequirementSnapshotsRawConfig(t *testing.T) {
	manager := newTestRenderManager()
	raw := loadSSRProofRoute(t)
	plain, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain.NeedsAPIRequestContext() {
		t.Fatal("plain route unexpectedly requires API context")
	}

	// The legacy scan includes raw data outside the executed component graph.
	raw["metadata"] = []interface{}{map[string]interface{}{"nested": map[string]interface{}{
		"@type": component.APIConfigGetName(),
	}}}
	before := shared.CloneMapDeep(raw)
	withAPI, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw, before) {
		t.Fatal("compilation mutated the raw config")
	}
	if !withAPI.NeedsAPIRequestContext() || plain.NeedsAPIRequestContext() {
		t.Fatal("plans did not retain their respective raw configuration requirements")
	}

	delete(raw, "metadata")
	replacement, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.NeedsAPIRequestContext() || !withAPI.NeedsAPIRequestContext() {
		t.Fatal("replacement or previously compiled requirement changed unexpectedly")
	}

	want, wantErrors := plain.Render(requestContext("state-one"))
	got, gotErrors := withAPI.Render(requestContext("state-one"))
	if got != want || len(gotErrors) != len(wantErrors) {
		t.Fatalf("request metadata changed rendered output: errors=%v, want %v", gotErrors, wantErrors)
	}
}
