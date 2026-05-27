package component

import (
	"context"
	"testing"
)

type contextCapturingPlugin struct {
	ctx context.Context
}

func (p *contextCapturingPlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	p.ctx = ctx
	return "plugin output", nil
}

func TestPluginRendererProvidesContextWhenNil(t *testing.T) {
	plugin := &contextCapturingPlugin{}
	renderer := &PluginRenderer{}

	rendered, errs := renderer.renderAndWrap(plugin, PluginConfig{}, PluginConfig{}, nil, nil)
	if len(errs) > 0 {
		t.Fatalf("render errors = %#v", errs)
	}
	if rendered != "plugin output" {
		t.Fatalf("rendered = %q, want plugin output", rendered)
	}
	if plugin.ctx == nil {
		t.Fatal("plugin context is nil, want runtime fallback context")
	}
}

func TestPluginRendererPreservesProvidedContext(t *testing.T) {
	plugin := &contextCapturingPlugin{}
	renderer := &PluginRenderer{}
	ctx := context.WithValue(context.Background(), struct{}{}, "sentinel")

	rendered, errs := renderer.renderAndWrap(plugin, PluginConfig{}, PluginConfig{}, ctx, nil)
	if len(errs) > 0 {
		t.Fatalf("render errors = %#v", errs)
	}
	if rendered != "plugin output" {
		t.Fatalf("rendered = %q, want plugin output", rendered)
	}
	if plugin.ctx != ctx {
		t.Fatal("plugin context was not preserved")
	}
}
