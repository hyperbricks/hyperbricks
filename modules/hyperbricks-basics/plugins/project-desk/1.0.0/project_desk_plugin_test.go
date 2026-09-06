package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestReviewActionsReturnTemplateData(t *testing.T) {
	for _, tc := range []struct {
		action, name, step string
	}{
		{"preview", "Garden+team", "preview"},
		{"confirm", "Garden+team", "confirmed"},
		{"confirm", "", "input"},
	} {
		req := httptest.NewRequest("POST", "/review", strings.NewReader("name="+tc.name))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		ctx := context.WithValue(context.Background(), shared.Request, req)
		result, errs := (&projectDeskPlugin{}).Render(map[string]interface{}{
			"data": map[string]interface{}{"action": tc.action, "template": "review.html"},
		}, ctx)
		if len(errs) > 0 {
			t.Fatal(errs)
		}
		node := result.(map[string]interface{})
		values := node["values"].(map[string]interface{})
		if node["@type"] != "<TEMPLATE>" || node["template"] != "review.html" || values["step"] != tc.step {
			t.Fatalf("unexpected template contract: %#v", node)
		}
		if tc.name != "" && values["name"] != "Garden team" {
			t.Fatalf("form name not available to template: %#v", values)
		}
		if tc.step == "preview" && values["words"] != "2" {
			t.Fatalf("word count must be a scalar template value: %#v", values)
		}
	}
}

func TestActionComesFromConfiguration(t *testing.T) {
	req := httptest.NewRequest("POST", "/review?action=confirm", strings.NewReader("name=Garden&action=confirm"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(context.Background(), shared.Request, req)
	result, errs := (&projectDeskPlugin{}).Render(map[string]interface{}{
		"data": map[string]interface{}{"action": "preview", "template": "review.html"},
	}, ctx)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	values := result.(map[string]interface{})["values"].(map[string]interface{})
	if values["step"] != "preview" {
		t.Fatalf("client changed the configured route action: %#v", values)
	}
}
