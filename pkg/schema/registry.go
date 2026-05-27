package schema

import (
	"reflect"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

type Category string

const (
	CategoryComponent Category = "component"
	CategoryComposite Category = "composite"
	CategoryData      Category = "data"
	CategoryMenu      Category = "menu"
	CategoryResource  Category = "resources"
)

type ChildModel string

const (
	ChildModelNone   ChildModel = "none"
	ChildModelTree   ChildModel = "tree"
	ChildModelHead   ChildModel = "head"
	ChildModelValues ChildModel = "values"
)

type FormGroup struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Collapsed   bool     `json:"collapsed,omitempty"`
	Fields      []string `json:"fields"`
}

type FieldAuthoringOverride struct {
	Path    string `json:"path"`
	Label   string `json:"label,omitempty"`
	Control string `json:"control,omitempty"`
	Group   string `json:"group,omitempty"`
}

type Definition struct {
	Name                    string                   `json:"name"`
	Token                   string                   `json:"token"`
	Aliases                 []string                 `json:"aliases,omitempty"`
	Category                Category                 `json:"category"`
	ChildModel              ChildModel               `json:"child_model"`
	ConfigType              reflect.Type             `json:"-"`
	FormGroups              []FormGroup              `json:"form_groups,omitempty"`
	AuthoringSlots          []AuthoringSlot          `json:"authoring_slots,omitempty"`
	FieldAuthoringOverrides []FieldAuthoringOverride `json:"field_authoring_overrides,omitempty"`
	Description             string                   `json:"description,omitempty"`
}

func Definitions() []Definition {
	return []Definition{
		{
			Name:        "Fragment",
			Token:       composite.FragmentConfigGetName(),
			Category:    CategoryComposite,
			ChildModel:  ChildModelTree,
			ConfigType:  reflect.TypeOf(composite.FragmentConfig{}),
			Description: "A <FRAGMENT> dynamically renders part of an HTML page, allowing updates without a full page reload.",
			FormGroups: []FormGroup{
				routeGroup("title", "route", "section", "static", "cache", "nocache", "index", "content_type", "enclose"),
				templateGroup("template.template", "template.inline", "template.querykeys", "template.queryparams", "template.values", "template.enclose"),
				responseGroup(),
				guardGroup(),
			},
		},
		{
			Name:        "ApiFragmentRender",
			Token:       composite.ApiFragmentRenderConfigGetName(),
			Category:    CategoryComposite,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(composite.ApiFragmentRenderConfig{}),
			Description: "Request-time API fragment that forwards to an upstream endpoint and renders the response.",
			FormGroups: []FormGroup{
				routeGroup("title", "route", "section", "enclose", "index"),
				apiGroup("endpoint", "method", "headers", "body", "username", "password", "jwtsecret", "jwtclaims"),
				templateGroup("template", "inline", "querykeys", "queryparams", "values"),
				responseGroup(),
				guardGroup(),
			},
		},
		{
			Name:        "Hypermedia",
			Token:       composite.HyperMediaConfigGetName(),
			Category:    CategoryComposite,
			ChildModel:  ChildModelTree,
			ConfigType:  reflect.TypeOf(composite.HyperMediaConfig{}),
			Description: "Route-owning page shell that renders the main HyperBricks document.",
			AuthoringSlots: []AuthoringSlot{
				headAuthoringSlot(false),
				bodyAuthoringSlot(true),
			},
			FormGroups: []FormGroup{
				routeGroup("title", "route", "section", "static", "cache", "nocache", "index", "content_type"),
				documentGroup("doctype", "htmltag", "bodytag", "enclose", "favicon", "head", "headers", "cookies"),
				templateGroup("template.template", "template.inline", "template.querykeys", "template.queryparams", "template.values", "template.enclose"),
				guardGroup(),
			},
		},
		{
			Name:        "Head",
			Token:       composite.HeadConfigGetName(),
			Category:    CategoryComposite,
			ChildModel:  ChildModelHead,
			ConfigType:  reflect.TypeOf(composite.HeadConfig{}),
			Description: "Document head helper that assembles title, meta, CSS, and JavaScript.",
			FormGroups: []FormGroup{
				{Key: "head", Label: "Head", Fields: []string{"title", "favicon", "meta", "css", "js"}},
			},
		},
		{
			Name:        "Template",
			Token:       composite.TemplateConfigGetName(),
			Category:    CategoryComponent,
			ChildModel:  ChildModelValues,
			ConfigType:  reflect.TypeOf(composite.TemplateConfig{}),
			Description: "Template-backed component that binds scalar values and value-mounted bricks into generated HTML.",
			FormGroups: []FormGroup{
				templateGroup("template", "inline", "querykeys", "queryparams", "values", "enclose"),
			},
		},
		{
			Name:        "Tree",
			Token:       composite.TreeRendererConfigGetName(),
			Category:    CategoryComposite,
			ChildModel:  ChildModelTree,
			ConfigType:  reflect.TypeOf(composite.TreeConfig{}),
			Description: "Ordered container that renders nested child items in key order.",
			FormGroups: []FormGroup{
				{Key: "tree", Label: "Tree", Fields: []string{"enclose"}},
			},
		},
		{
			Name:        "Html",
			Token:       component.HTMLConfigGetName(),
			Category:    CategoryComponent,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.HTMLConfig{}),
			Description: "Raw HTML snippet for leaf content or small escaped blocks.",
			FormGroups: []FormGroup{
				{Key: "content", Label: "Content", Fields: []string{"value", "trimspace", "enclose", "attributes"}},
			},
		},
		{
			Name:        "Text",
			Token:       component.TextConfigGetName(),
			Category:    CategoryComponent,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.TextConfig{}),
			Description: "Plain text leaf node.",
			FormGroups: []FormGroup{
				{Key: "content", Label: "Content", Fields: []string{"value", "enclose", "attributes"}},
			},
		},
		{
			Name:        "Css",
			Token:       component.CssConfigGetName(),
			Category:    CategoryResource,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.CssConfig{}),
			Description: "Stylesheet leaf that can emit inline CSS or link to a stylesheet.",
			FormGroups: []FormGroup{
				{Key: "source", Label: "Source", Fields: []string{"inline", "link", "file"}},
			},
		},
		{
			Name:        "Javascript",
			Token:       component.JSConfigGetName(),
			Aliases:     []string{component.JavaScriptConfigGetName()},
			Category:    CategoryResource,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.JSConfig{}),
			Description: "JavaScript leaf that can emit inline script or link to a script file.",
			FormGroups: []FormGroup{
				{Key: "source", Label: "Source", Fields: []string{"inline", "link", "file"}},
			},
		},
		{
			Name:        "Image",
			Token:       component.SingleImageConfigGetName(),
			Category:    CategoryResource,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.SingleImageConfig{}),
			Description: "Single image renderer with optional optimization and HTML output.",
			FormGroups: []FormGroup{
				{Key: "image", Label: "Image", Fields: []string{"src", "width", "height", "alt", "title", "id", "class", "quality", "loading"}},
			},
		},
		{
			Name:        "Images",
			Token:       component.MultipleImagesConfigGetName(),
			Category:    CategoryResource,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.MultipleImagesConfig{}),
			Description: "Multiple image renderer for a directory of images.",
			FormGroups: []FormGroup{
				{Key: "images", Label: "Images", Fields: []string{"directory", "width", "height", "id", "class", "alt", "title", "quality", "loading"}},
			},
		},
		{
			Name:        "Json",
			Token:       component.LocalJSONConfigGetName(),
			Aliases:     []string{"<JSON>"},
			Category:    CategoryData,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.LocalJSONConfig{}),
			Description: "Local JSON renderer that loads a file and feeds it into a template.",
			FormGroups: []FormGroup{
				{Key: "source", Label: "Source", Fields: []string{"file", "template", "inline", "values", "debug"}},
			},
		},
		{
			Name:        "Api_Render",
			Token:       component.APIConfigGetName(),
			Category:    CategoryData,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.APIConfig{}),
			Description: "Remote API fetcher that renders the upstream response through a template.",
			FormGroups: []FormGroup{
				apiGroup("endpoint", "method", "headers", "body", "username", "password", "jwtsecret", "jwtclaims"),
				templateGroup("template", "inline", "querykeys", "queryparams", "values"),
			},
		},
		{
			Name:        "Menu",
			Token:       component.MenuConfigGetName(),
			Category:    CategoryMenu,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.MenuConfig{}),
			Description: "Menu renderer that sorts and formats page links by section.",
			FormGroups: []FormGroup{
				{Key: "menu", Label: "Menu", Fields: []string{"section", "order", "sort", "active", "item", "enclose"}},
			},
			FieldAuthoringOverrides: []FieldAuthoringOverride{
				{Path: "active", Control: "textarea"},
				{Path: "item", Control: "textarea"},
			},
		},
		{
			Name:        "Plugin",
			Token:       component.PluginRenderGetName(),
			Category:    CategoryComponent,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.PluginConfig{}),
			Description: "Plugin renderer that delegates output to a loaded HyperBricks plugin.",
			FormGroups: []FormGroup{
				{Key: "plugin", Label: "Plugin", Fields: []string{"plugin", "classes", "data"}},
			},
		},
		{
			Name:        "Style",
			Token:       component.StyleConfigGetName(),
			Category:    CategoryResource,
			ChildModel:  ChildModelNone,
			ConfigType:  reflect.TypeOf(component.StyleConfig{}),
			Description: "Stylesheet file renderer for project style assets.",
			FormGroups: []FormGroup{
				{Key: "source", Label: "Source", Fields: []string{"file"}},
			},
		},
	}
}

func Tokens(def Definition) []string {
	tokens := []string{def.Token}
	tokens = append(tokens, def.Aliases...)
	return tokens
}

func routeGroup(fields ...string) FormGroup {
	return FormGroup{
		Key:    "route",
		Label:  "Route",
		Fields: fields,
	}
}

func documentGroup(fields ...string) FormGroup {
	return FormGroup{
		Key:    "document",
		Label:  "Document",
		Fields: fields,
	}
}

func templateGroup(fields ...string) FormGroup {
	return FormGroup{
		Key:         "template",
		Label:       "Template",
		Description: "Template source, query proxy, value map, and enclosure fields.",
		Collapsed:   true,
		Fields:      fields,
	}
}

func apiGroup(fields ...string) FormGroup {
	return FormGroup{
		Key:    "api",
		Label:  "API",
		Fields: fields,
	}
}

func responseGroup() FormGroup {
	return FormGroup{
		Key:       "response",
		Label:     "Response",
		Collapsed: true,
		Fields: []string{
			"response.hx_location",
			"response.hx_push_url",
			"response.hx_redirect",
			"response.hx_refresh",
			"response.hx_replace_url",
			"response.hx_reswap",
			"response.hx_retarget",
			"response.hx_reselect",
			"response.hx_trigger",
			"response.hx_trigger_after_settle",
			"response.hx_trigger_after_swap",
		},
	}
}

func guardGroup() FormGroup {
	return FormGroup{
		Key:       "guard",
		Label:     "Guard",
		Collapsed: true,
		Fields: []string{
			"guard.enabled",
			"guard.auth.cookie",
			"guard.auth.header",
			"guard.auth.scheme",
			"guard.require.authenticated",
			"guard.require.query",
			"guard.authorize.endpoint",
			"guard.authorize.method",
			"guard.authorize.headers",
			"guard.authorize.body",
			"guard.on_unauthenticated.redirect",
			"guard.on_unauthenticated.hx_redirect",
			"guard.on_unauthenticated.status",
			"guard.on_forbidden.redirect",
			"guard.on_forbidden.hx_redirect",
			"guard.on_forbidden.status",
		},
	}
}
