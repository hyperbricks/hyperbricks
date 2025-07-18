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

// InitializeRenderers registers all renderers, including special types.
func (rm *RenderManager) InitializeRenderers() {

}

// LoadPlugin loads a Go plugin dynamically and registers it.
func (rm *RenderManager) LoadPlugin(path string, name string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return err
	}
	sym, err := p.Lookup("Plugin")
	if err != nil {
		return err
	}
	renderPlugin, ok := sym.(shared.PluginRenderer)
	if !ok {
		return fmt.Errorf("plugin does not implement RenderPlugin interface")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.Plugins[name] = renderPlugin
	return nil
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

	// Lock and store the plugin instance safely
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Ensure Plugins map is initialized before storing
	if rm.Plugins == nil {
		rm.Plugins = make(map[string]shared.PluginRenderer)
	}

	rm.Plugins[name] = renderer
	return nil // No error
}

// // LoadPlugin loads a Go plugin dynamically and registers it.
// func (rm *RenderManager) RegisterAndLoadPlugin(path string, name string) error {
// 	logger := logging.GetLogger()
// 	var _err error = nil

// 	logger.Infof("Preloading plugin: %s", path)

// 	p, err := plugin.Open(path)
// 	if err != nil {
// 		_err = fmt.Errorf("<!-- error loading plugin %v: %v -->", name, err)
// 	}

// 	// Lookup "Plugin" as a function
// 	symbol, err := p.Lookup("Plugin")
// 	if err != nil {
// 		_err = fmt.Errorf("failed to lookup 'Plugin' symbol: %v", err)
// 	}

// 	// Assert it is of the correct function type
// 	pluginFactory, ok := symbol.(func() (shared.PluginRenderer, error))
// 	if !ok {
// 		_err = fmt.Errorf("plugin symbol is not of expected type 'func() (shared.Renderer, error)'")
// 	}

// 	// Create an instance of the plugin
// 	renderer, err := pluginFactory()
// 	if err != nil {
// 		_err = fmt.Errorf("error initializing plugin: %v", err)
// 	}

// 	rm.mu.Lock()
// 	defer rm.mu.Unlock()
// 	rm.Plugins[name] = renderer

// 	return _err
// }

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

// consolidateErrors transforms a slice of errors into a single string error.
// It returns a single error if required but can also keep the []E structure.
func consolidateErrors(errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	return errs
}
