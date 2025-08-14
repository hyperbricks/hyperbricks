package main

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
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

// Config structure with default values.
type Plugin struct {
	Name string
	Key  string
	Path string
}

func registerPlugins() {

	pluginDir := "./bin/plugins"
	if tbplugindir, ok := rm.HbConfig.Directories["plugins"]; ok {
		pluginDir = tbplugindir
	}

	if commands.Debug {
		// preloading basic plugins for debug:
		pluginDir += "/debug"
	}

	for _, value := range rm.HbConfig.Plugins.Enabled {
		pluginPath := pluginDir + "/" + value + ".so"
		absPath, _ := filepath.Abs(pluginPath)

		// Check if the file exists
		if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
			logging.GetLogger().Warnf("Plugin file %s not found. Skipping preloading.", value)
			continue // Skip loading this plugin
		}
		logging.GetLogger().Infof("Plugin file %s found at %s", value, absPath)
		rm.RegisterAndLoadPlugin(pluginPath, value)
	}
}

func registerRenderers() {

	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"example": "<div>{{main_section}}</div>",
			"header":  "<h1>{{title}}</h1>",
		}
		content, exists := templates[templateName]
		return content, exists
	}

	singleImageRenderer := &component.SingleImageRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}

	multipleImagesRenderer := &component.MultipleImagesRenderer{
		ImageProcessorInstance: &component.ImageProcessor{},
	}
	pluginRenderer := &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	menuRenderer := &component.MenuRenderer{
		TemplateProvider: templateProvider,
	}

	localJsonRenderer := &component.LocalJSONRenderer{
		TemplateProvider: templateProvider,
	}

	fragmentRenderer := &composite.FragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	apiFragmentRenderer := &composite.ApiFragmentRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	hypermediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}
	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

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

	rm.RegisterComponent(component.SingleImageConfigGetName(), singleImageRenderer, reflect.TypeOf(component.SingleImageConfig{}))
	rm.RegisterComponent(component.MultipleImagesConfigGetName(), multipleImagesRenderer, reflect.TypeOf(component.MultipleImagesConfig{}))
	rm.RegisterComponent(component.MenuConfigGetName(), menuRenderer, reflect.TypeOf(component.MenuConfig{}))
	rm.RegisterComponent(component.LocalJSONConfigGetName(), localJsonRenderer, reflect.TypeOf(component.LocalJSONConfig{}))
	rm.RegisterComponent(component.PluginRenderGetName(), pluginRenderer, reflect.TypeOf(component.PluginConfig{}))
	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))
	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))
	rm.RegisterComponent(composite.FragmentConfigGetName(), fragmentRenderer, reflect.TypeOf(composite.FragmentConfig{}))
	rm.RegisterComponent(composite.ApiFragmentRenderConfigGetName(), apiFragmentRenderer, reflect.TypeOf(composite.ApiFragmentRenderConfig{}))
	rm.RegisterComponent(composite.HyperMediaConfigGetName(), hypermediaRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))
	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))
}

func linkRendererResources() {
	rm.GetRenderComponent(composite.TemplateConfigGetName()).(*composite.TemplateRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.APIConfigGetName()).(*component.APIRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.LocalJSONConfigGetName()).(*component.LocalJSONRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.MenuConfigGetName()).(*component.MenuRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.PluginRenderGetName()).(*component.PluginRenderer).TemplateProvider = parser.GetTemplate
	rm.GetRenderComponent(component.MenuConfigGetName()).(*component.MenuRenderer).HyperMediasBySection = GetGlobalHyperMediasBySection()
}
