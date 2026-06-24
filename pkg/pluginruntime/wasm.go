package pluginruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const (
	WasmAllocExport          = "alloc"
	WasmRenderExport         = "render"
	WasmMemoryExport         = "memory"
	DefaultWasmRenderTimeout = 2 * time.Second
	DefaultWasmMemoryPages   = 128
	WasmInitializeExport     = "_initialize"
)

type WasmPluginRenderer struct {
	Name          string
	Path          string
	RenderTimeout time.Duration

	runtime  wazero.Runtime
	compiled wazero.CompiledModule
}

type wasmPluginInput struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string                 `mapstructure:"plugin"`
	Classes          []string               `mapstructure:"classes"`
	Data             map[string]interface{} `mapstructure:"data"`
}

type wasmRenderResponse struct {
	Kind   string            `json:"kind"`
	HTML   string            `json:"html,omitempty"`
	Errors []wasmRenderError `json:"errors,omitempty"`
}

type wasmRenderError struct {
	File     string `json:"file,omitempty"`
	Err      string `json:"err,omitempty"`
	Key      string `json:"key,omitempty"`
	Path     string `json:"path,omitempty"`
	Rejected bool   `json:"rejected,omitempty"`
	Type     string `json:"type,omitempty"`
	Level    string `json:"level,omitempty"`
}

func NewWasmPluginRenderer(ctx context.Context, path string, name string) (*WasmPluginRenderer, error) {
	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm plugin %s: %v", name, err)
	}

	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(DefaultWasmMemoryPages))
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to compile wasm plugin %s: %v", name, err)
	}
	if compiledModuleImportsWASI(compiled) {
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
			_ = compiled.Close(ctx)
			_ = runtime.Close(ctx)
			return nil, fmt.Errorf("failed to instantiate WASI imports for wasm plugin %s: %v", name, err)
		}
	}

	return &WasmPluginRenderer{
		Name:          name,
		Path:          path,
		RenderTimeout: DefaultWasmRenderTimeout,
		runtime:       runtime,
		compiled:      compiled,
	}, nil
}

func (r *WasmPluginRenderer) Render(instance interface{}, ctx context.Context) (any, []error) {
	input, decodeErrs := normalizeWasmPluginInput(instance)
	if len(decodeErrs) > 0 {
		return "<!-- failed to prepare wasm plugin input -->", decodeErrs
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return r.renderFailure(fmt.Sprintf("failed to encode wasm plugin input: %v", err))
	}

	outputJSON, err := r.callRender(ctx, inputJSON)
	if err != nil {
		return r.renderFailure(err.Error())
	}

	var response wasmRenderResponse
	if err := json.Unmarshal(outputJSON, &response); err != nil {
		return r.renderFailure(fmt.Sprintf("failed to decode wasm plugin output: %v", err))
	}

	errs := response.componentErrors()
	switch response.Kind {
	case "html":
		return response.HTML, errs
	default:
		errs = append(errs, r.componentError(fmt.Sprintf("unsupported wasm plugin response kind %q", response.Kind), true))
		return "<!-- unsupported wasm plugin response kind -->", errs
	}
}

func (r *WasmPluginRenderer) callRender(ctx context.Context, input []byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok && r.RenderTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.RenderTimeout)
		defer cancel()
	}

	if len(input) > math.MaxUint32 {
		return nil, fmt.Errorf("wasm plugin input is too large")
	}

	module, err := r.runtime.InstantiateModule(ctx, r.compiled, wazero.NewModuleConfig().
		WithName("").
		WithStartFunctions())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate wasm plugin %s: %v", r.Name, err)
	}
	defer module.Close(context.Background())

	memory := module.ExportedMemory(WasmMemoryExport)
	if memory == nil {
		return nil, fmt.Errorf("wasm plugin %s does not export %q memory", r.Name, WasmMemoryExport)
	}
	alloc := module.ExportedFunction(WasmAllocExport)
	if alloc == nil {
		return nil, fmt.Errorf("wasm plugin %s does not export %q", r.Name, WasmAllocExport)
	}
	render := module.ExportedFunction(WasmRenderExport)
	if render == nil {
		return nil, fmt.Errorf("wasm plugin %s does not export %q", r.Name, WasmRenderExport)
	}
	if initialize := module.ExportedFunction(WasmInitializeExport); initialize != nil {
		if _, err := initialize.Call(ctx); err != nil {
			return nil, fmt.Errorf("wasm plugin %s initialize failed: %v", r.Name, err)
		}
	}

	allocResult, err := alloc.Call(ctx, uint64(len(input)))
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s alloc failed: %v", r.Name, err)
	}
	if len(allocResult) != 1 {
		return nil, fmt.Errorf("wasm plugin %s alloc returned %d values", r.Name, len(allocResult))
	}
	inputPtr := uint32(allocResult[0])
	if ok := memory.Write(inputPtr, input); !ok {
		return nil, fmt.Errorf("wasm plugin %s input write out of bounds", r.Name)
	}

	renderResult, err := render.Call(ctx, uint64(inputPtr), uint64(len(input)))
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s render failed: %v", r.Name, err)
	}
	if len(renderResult) != 1 {
		return nil, fmt.Errorf("wasm plugin %s render returned %d values", r.Name, len(renderResult))
	}

	outputPtr, outputLen := unpackPtrLen(renderResult[0])
	output, ok := memory.Read(outputPtr, outputLen)
	if !ok {
		return nil, fmt.Errorf("wasm plugin %s output read out of bounds", r.Name)
	}

	return append([]byte(nil), output...), nil
}

func normalizeWasmPluginInput(instance interface{}) (map[string]interface{}, []error) {
	var config wasmPluginInput
	if errs := shared.DecodeWithBasicHooks(instance, &config); len(errs) > 0 {
		return nil, errs
	}

	input := map[string]interface{}{
		"plugin": config.PluginName,
		"data":   config.Data,
	}
	if len(config.Classes) > 0 {
		input["classes"] = config.Classes
	}
	if len(config.ExtraAttributes) > 0 {
		input["attributes"] = config.ExtraAttributes
	}
	if config.Enclose != "" {
		input["enclose"] = config.Enclose
	}
	if config.ConfigType != "" {
		input["@type"] = config.ConfigType
	}
	if config.HyperBricksKey != "" {
		input["hyperbrickskey"] = config.HyperBricksKey
		input["hyperbricks_key"] = config.HyperBricksKey
	}
	if config.HyperBricksPath != "" {
		input["hyperbrickspath"] = config.HyperBricksPath
		input["hyperbricks_path"] = config.HyperBricksPath
	}
	if config.HyperBricksFile != "" {
		input["hyperbricksfile"] = config.HyperBricksFile
		input["hyperbricks_file"] = config.HyperBricksFile
	}
	return input, nil
}

func unpackPtrLen(value uint64) (uint32, uint32) {
	return uint32(value >> 32), uint32(value)
}

func compiledModuleImportsWASI(compiled wazero.CompiledModule) bool {
	for _, fn := range compiled.ImportedFunctions() {
		moduleName, _, ok := fn.Import()
		if ok && moduleName == wasi_snapshot_preview1.ModuleName {
			return true
		}
	}
	for _, memory := range compiled.ImportedMemories() {
		moduleName, _, ok := memory.Import()
		if ok && moduleName == wasi_snapshot_preview1.ModuleName {
			return true
		}
	}
	return false
}

func (r *WasmPluginRenderer) renderFailure(message string) (any, []error) {
	return "<!-- failed to render wasm plugin -->", []error{r.componentError(message, true)}
}

func (r *WasmPluginRenderer) componentError(message string, rejected bool) shared.ComponentError {
	return shared.ComponentError{
		Hash:     shared.GenerateHash(),
		Type:     "<PLUGIN>",
		Rejected: rejected,
		Err:      fmt.Sprintf("wasm plugin %s: %s", r.Name, message),
	}
}

func (response wasmRenderResponse) componentErrors() []error {
	if len(response.Errors) == 0 {
		return nil
	}
	errs := make([]error, 0, len(response.Errors))
	for _, renderErr := range response.Errors {
		if renderErr.Err == "" {
			continue
		}
		errType := renderErr.Type
		if errType == "" {
			errType = "<PLUGIN>"
		}
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			File:     renderErr.File,
			Err:      renderErr.Err,
			Key:      renderErr.Key,
			Path:     renderErr.Path,
			Rejected: renderErr.Rejected,
			Type:     errType,
			Level:    renderErr.Level,
		})
	}
	return errs
}
