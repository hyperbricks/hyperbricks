package main

import (
	"os"
	"reflect"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/render"
	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

var (
	rm *render.RenderManager
)

// Centralized component and plugin initialization
func initializeComponents() {
	rm = render.NewRenderManager()
	registerRenderers()
	registerPlugins()

}

func registerPlugins() {

	pluginDir := "./bin/plugins"
	if tbplugindir, ok := rm.HbConfig.Directories["plugins"]; ok {
		pluginDir = tbplugindir
	}

	if commands.Debug {
		// PRELOADING BASIC PLUGINS FOR DEBUG:
		pluginDir += "/debug"
	}

	// for key, value := range rm.HbConfig.Plugins {
	// 	fmt.Printf("Key: %s, Value: %s\n", key, value)
	// 	if value == "enabled" {
	// 		logging.GetLogger().Infof("%s -> %s", pluginDir+"/"+key+".so", key)
	// 		rm.RegisterAndLoadPlugin(pluginDir+"/"+key+".so", key)
	// 	}
	// }

	for key, value := range rm.HbConfig.Plugins {
		//fmt.Printf("Key: %s, Value: %s\n", key, value)

		if value == "enabled" {
			pluginPath := pluginDir + "/" + key + ".so"

			// Check if the file exists
			if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
				logging.GetLogger().Warnf("Plugin file %s not found. Skipping preloading.", key)
				continue // Skip loading this plugin
			}

			rm.RegisterAndLoadPlugin(pluginPath, key)
		}
	}
}

func registerRenderers() {
	rm = render.NewRenderManager()
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"example": "<div>{{main_section}}</div>",
			"header":  "<h1>{{title}}</h1>",
		}
		content, exists := templates[templateName]
		return content, exists
	}

	// This instanciating of ImageProcessorInstance gives some flexibility for testing
	singleImageRenderer := &component.SingleImageRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}

	multipleImagesRenderer := &component.MultipleImagesRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}

	// Register standard renderers using static-like functions
	rm.RegisterComponent(component.SingleImageConfigGetName(), singleImageRenderer, reflect.TypeOf(component.SingleImageConfig{}))
	rm.RegisterComponent(component.MultipleImagesConfigGetName(), multipleImagesRenderer, reflect.TypeOf(component.MultipleImagesConfig{}))

	// TEMPLATE ....
	pluginRenderer := &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(component.PluginRenderGetName(), pluginRenderer, reflect.TypeOf(component.PluginConfig{}))

	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))

	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))

	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))

	//Register Template Menu Renderer
	menuRenderer := &component.MenuRenderer{
		TemplateProvider: templateProvider,
	}
	rm.RegisterComponent(component.MenuConfigGetName(), menuRenderer, reflect.TypeOf(component.MenuConfig{}))

	// Register Local JSON Renderer
	localJsonRenderer := &component.LocalJSONRenderer{
		TemplateProvider: templateProvider,
	}
	rm.RegisterComponent(component.LocalJSONConfigGetName(), localJsonRenderer, reflect.TypeOf(component.LocalJSONConfig{}))

	// TEMPLATE ....
	fragmentRenderer := &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.FragmentConfigGetName(), fragmentRenderer, reflect.TypeOf(composite.FragmentConfig{}))

	// TEMPLATE ....
	hypermediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.HyperMediaConfigGetName(), hypermediaRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	// TEMPLATE ....
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	// API ....
	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	// COMPONENTS
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))
}

func configureRenderers() {
	// populating renderers with template from hyperbricks
	rm.GetRenderComponent(composite.TemplateConfigGetName()).(*composite.TemplateRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.APIConfigGetName()).(*component.APIRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.LocalJSONConfigGetName()).(*component.LocalJSONRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.MenuConfigGetName()).(*component.MenuRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.PluginRenderGetName()).(*component.PluginRenderer).TemplateProvider = parser.GetTemplate

	hypermediasMutex.Lock()
	temp := hypermediasBySection // Copy the map for use outside the lock
	hypermediasMutex.Unlock()

	rm.GetRenderComponent(component.MenuConfigGetName()).(*component.MenuRenderer).HyperMediasBySection = temp

}
