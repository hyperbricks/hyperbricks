// render/render_manager.go
package render

import (
	"context"
	"fmt"
	"plugin"
	"reflect"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// RenderManager is the central coordinator integrating TypeFactory.
type RenderManager struct {
	typeFactory *typefactory.TypeFactory
	renderers   map[string]shared.Renderer
	Plugins     map[string]shared.PluginRenderer
	HbConfig    *shared.Config
	mu          sync.RWMutex // Read-Write mutex for thread safety
}

// NewRenderManager initializes a new RenderManager with an embedded TypeFactory.
func NewRenderManager() *RenderManager {
	return &RenderManager{
		typeFactory: typefactory.NewTypeFactory(),
		renderers:   make(map[string]shared.Renderer),
		Plugins:     make(map[string]shared.PluginRenderer),
		HbConfig:    shared.GetHyperBricksConfiguration(),
	}
}

// Render renders content based on its type using registered components or plugins.
func (rm *RenderManager) Render(rendererType string, data map[string]interface{}, ctx context.Context) (string, []error) {
	var errors []error
	// Create a TypeRequest for the TypeFactory
	request := typefactory.TypeRequest{
		TypeName: rendererType,
		Data:     data,
	}

	// Create the instance using TypeFactory
	response, err := rm.typeFactory.CreateInstance(request)
	if err != nil {

		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Err:      "Cannot create component instance...",
			Rejected: true,
		})
		// When type is not registerd show tag with error...
		return fmt.Sprintf("<! -- %s -->", err), errors
	}
	//logging.Logger.Debug("Render CreateInstance: ", response)
	// Handle warnings (if any)

	for _, s := range response.Warnings {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Err:      s,
			Rejected: false,
		})
	}

	// Retrieve the appropriate renderer
	rm.mu.RLock()
	renderer, exists := rm.renderers[rendererType]
	rm.mu.RUnlock()

	if exists {
		html, errs := renderer.Render(response.Instance, ctx)
		errors = append(errors, errs...)
		return html, errors

	} else {
		return "", append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Err:      fmt.Errorf("invalid type for HYPERMEDIA").Error(),
			Rejected: true,
		})
	}
}

// GetRenderComponent retrieves a RenderComponent by its content type.
func (rm *RenderManager) GetRenderComponent(rendererType string) shared.Renderer {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.renderers[rendererType]
}

// RegisterComponent registers a RenderComponent along with its type and config.
func (rm *RenderManager) RegisterComponent(rendererType string, component shared.Renderer, configType reflect.Type) {
	// Register the type with the TypeFactory
	rm.typeFactory.RegisterType(rendererType, configType)

	// Register the component
	rm.mu.Lock()
	defer rm.mu.Unlock()
	parserTypes := component.Types()
	for _, ptype := range parserTypes {
		if !parser.KnownTypes[ptype] {
			parser.KnownTypes[ptype] = true
		}
	}
	rm.renderers[rendererType] = component
}

func (rm *RenderManager) GetPlugin(name string) (shared.PluginRenderer, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	pr, ok := rm.Plugins[name]
	return pr, ok
}

func (rm *RenderManager) SetPlugin(name string, pr shared.PluginRenderer) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.Plugins == nil {
		rm.Plugins = make(map[string]shared.PluginRenderer)
	}
	rm.Plugins[name] = pr
}

// RegisterAndLoadPlugin loads a Go plugin dynamically and registers it.
func (rm *RenderManager) RegisterAndLoadPlugin(path string, name string) error {
	logger := logging.GetLogger()
	logger.Infof("Preloading plugin: %s", path)

	// Open the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("error loading plugin %v: %v", name, err) // Return early to prevent nil reference
	}

	// Lookup "Plugin" function symbol
	symbol, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("failed to lookup 'Plugin' symbol: %v", err) // Return early
	}

	// Assert the type of the symbol
	pluginFactory, ok := symbol.(func() (shared.PluginRenderer, error))
	if !ok {
		return fmt.Errorf("plugin symbol is not of expected type 'func() (shared.Renderer, error)'")
	}

	// Call the plugin factory function to create an instance
	renderer, err := pluginFactory()
	if err != nil {
		return fmt.Errorf("error initializing plugin: %v", err)
	}

	// Use the helper to store it safely
	rm.SetPlugin(name, renderer)

	return nil // No error
}

// Render renders content based on its type using registered components or plugins.
func (rm *RenderManager) MakeInstance(request typefactory.TypeRequest) (*typefactory.TypeResponse, error) {
	// Create the instance using TypeFactory
	response, err := rm.typeFactory.CreateInstance(request)
	if err != nil {
		// When type is not registerd show tag with error...
		return nil, err
	}
	return response, nil
}
