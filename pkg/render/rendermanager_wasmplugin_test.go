package render_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/pluginruntime"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestRenderManagerLoadsMarkdownWasmPlugin(t *testing.T) {
	const pluginName = "MarkdownWasmPlugin@1.0.0"

	pluginDir := t.TempDir()
	wasmPath := filepath.Join(pluginDir, pluginName+".wasm")
	if err := os.WriteFile(wasmPath, wasmModuleReturningJSON(markdownWasmResponse), 0644); err != nil {
		t.Fatalf("write wasm fixture: %v", err)
	}

	shared.Init_configuration()
	rm := render.NewRenderManager()
	pluginRuntime, err := rm.RegisterAndLoadPluginByName(context.Background(), pluginDir, pluginName)
	if err != nil {
		t.Fatalf("register wasm plugin: %v", err)
	}
	if pluginRuntime != pluginruntime.RuntimeWasm {
		t.Fatalf("runtime = %q, want %q", pluginRuntime, pluginruntime.RuntimeWasm)
	}

	pluginRenderer := &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm},
	}
	rendered, errs := pluginRenderer.Render(component.PluginConfig{
		PluginName: pluginName,
		Data: map[string]interface{}{
			"content": "# Hello WASM",
		},
	}, context.Background())
	if len(errs) > 0 {
		t.Fatalf("render errors = %#v", errs)
	}
	if !strings.Contains(rendered, `<h1>Hello WASM</h1>`) {
		t.Fatalf("rendered = %q, want wasm markdown output", rendered)
	}
}

const markdownWasmResponse = `{"kind":"html","html":"<div class=\"markdown_plugin-content\">\n<h1>Hello WASM</h1>\n</div>\n"}`

func wasmModuleReturningJSON(response string) []byte {
	const dataOffset = 1024
	const heapOffset = 4096

	data := []byte(response)
	wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	wasm = append(wasm, wasmSection(1, []byte{
		0x02,
		0x60, 0x01, 0x7f, 0x01, 0x7f,
		0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7e,
	})...)
	wasm = append(wasm, wasmSection(3, []byte{0x02, 0x00, 0x01})...)
	wasm = append(wasm, wasmSection(5, []byte{0x01, 0x00, 0x01})...)

	globalPayload := []byte{0x01, 0x7f, 0x01, 0x41}
	globalPayload = append(globalPayload, wasmS32(heapOffset)...)
	globalPayload = append(globalPayload, 0x0b)
	wasm = append(wasm, wasmSection(6, globalPayload)...)

	exportPayload := []byte{0x03}
	exportPayload = append(exportPayload, wasmName("memory")...)
	exportPayload = append(exportPayload, 0x02, 0x00)
	exportPayload = append(exportPayload, wasmName("alloc")...)
	exportPayload = append(exportPayload, 0x00, 0x00)
	exportPayload = append(exportPayload, wasmName("render")...)
	exportPayload = append(exportPayload, 0x00, 0x01)
	wasm = append(wasm, wasmSection(7, exportPayload)...)

	allocBody := []byte{0x01, 0x01, 0x7f}
	allocBody = append(allocBody,
		0x23, 0x00,
		0x21, 0x01,
		0x23, 0x00,
		0x20, 0x00,
		0x6a,
		0x24, 0x00,
		0x20, 0x01,
		0x0b,
	)

	renderBody := []byte{0x00, 0x41}
	renderBody = append(renderBody, wasmS32(dataOffset)...)
	renderBody = append(renderBody, 0xad, 0x42)
	renderBody = append(renderBody, wasmS64(32)...)
	renderBody = append(renderBody, 0x86, 0x41)
	renderBody = append(renderBody, wasmS32(len(data))...)
	renderBody = append(renderBody, 0xad, 0x84, 0x0b)

	codePayload := []byte{0x02}
	codePayload = append(codePayload, wasmVec(allocBody)...)
	codePayload = append(codePayload, wasmVec(renderBody)...)
	wasm = append(wasm, wasmSection(10, codePayload)...)

	dataPayload := []byte{0x01, 0x00, 0x41}
	dataPayload = append(dataPayload, wasmS32(dataOffset)...)
	dataPayload = append(dataPayload, 0x0b)
	dataPayload = append(dataPayload, wasmVec(data)...)
	wasm = append(wasm, wasmSection(11, dataPayload)...)

	return wasm
}

func wasmSection(id byte, payload []byte) []byte {
	section := []byte{id}
	section = append(section, wasmU32(uint32(len(payload)))...)
	section = append(section, payload...)
	return section
}

func wasmName(name string) []byte {
	encoded := wasmU32(uint32(len(name)))
	encoded = append(encoded, name...)
	return encoded
}

func wasmVec(data []byte) []byte {
	encoded := wasmU32(uint32(len(data)))
	encoded = append(encoded, data...)
	return encoded
}

func wasmU32(value uint32) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if value == 0 {
			return out
		}
	}
}

func wasmS32(value int) []byte {
	return wasmS64(int64(value))
}

func wasmS64(value int64) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		done := (value == 0 && b&0x40 == 0) || (value == -1 && b&0x40 != 0)
		if !done {
			b |= 0x80
		}
		out = append(out, b)
		if done {
			return out
		}
	}
}
