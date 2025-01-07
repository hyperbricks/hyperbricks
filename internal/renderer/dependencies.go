package renderer

import (
	"github.com/hyperbricks/hyperbricks/internal/render"
)

type CompositeRenderer struct {
	RenderManager    *render.RenderManager
	TemplateProvider func(templateName string) (string, bool)
}

type ComponentRenderer struct {
	TemplateProvider func(templateName string) (string, bool)
}
