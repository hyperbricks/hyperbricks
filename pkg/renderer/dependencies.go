package renderer

import (
	"github.com/hyperbricks/hyperbricks/pkg/render"
)

type CompositeRenderer struct {
	RenderManager    *render.RenderManager
	TemplateProvider func(templateName string) (string, bool)
}

type ComponentRenderer struct {
	TemplateProvider func(templateName string) (string, bool)
}
