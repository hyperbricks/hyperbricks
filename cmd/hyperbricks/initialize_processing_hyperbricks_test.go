package main

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"go.uber.org/zap"
)

func TestProcessScriptIndexesAPIFragmentRenderWithScalarTemplate(t *testing.T) {
	shared.Init_configuration()

	config := map[string]interface{}{
		"api_panel": map[string]interface{}{
			"@type":    composite.ApiFragmentRenderConfigGetName(),
			"route":    "api/panel",
			"title":    "API Panel",
			"section":  "api",
			"endpoint": "http://example.invalid/api",
			"method":   "GET",
			"template": "{{TEMPLATE:api/panel.html}}",
		},
	}
	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	filenameToRoutes := make(map[string][]string)

	err := processScript("test", config, tempConfigs, tempHyperMediasBySection, zap.NewNop().Sugar(), filenameToRoutes)
	if err != nil {
		t.Fatalf("processScript returned error: %v", err)
	}

	if _, ok := tempConfigs["api/panel"]; !ok {
		t.Fatalf("expected API fragment route to be indexed; routes: %#v", tempConfigs)
	}
	if got := filenameToRoutes["test"]; len(got) != 1 || got[0] != "api/panel" {
		t.Fatalf("expected filename route mapping [api/panel], got %#v", got)
	}
	if got := tempHyperMediasBySection["api"]; len(got) != 1 || got[0].Route != "api/panel" {
		t.Fatalf("expected menu metadata for api/panel, got %#v", got)
	}
}

func TestProcessScriptRejectsFragmentScalarTemplate(t *testing.T) {
	shared.Init_configuration()

	config := map[string]interface{}{
		"bad_fragment": map[string]interface{}{
			"@type":    composite.FragmentConfigGetName(),
			"route":    "bad-fragment",
			"title":    "Bad Fragment",
			"section":  "api",
			"template": "{{TEMPLATE:fragment.html}}",
		},
	}
	tempConfigs := make(map[string]map[string]interface{})
	tempHyperMediasBySection := make(map[string][]composite.HyperMediaConfig)
	filenameToRoutes := make(map[string][]string)

	err := processScript("test", config, tempConfigs, tempHyperMediasBySection, zap.NewNop().Sugar(), filenameToRoutes)
	if err != nil {
		t.Fatalf("processScript returned error: %v", err)
	}

	if _, ok := tempConfigs["bad-fragment"]; ok {
		t.Fatalf("expected scalar <FRAGMENT>.template to be rejected")
	}
}
