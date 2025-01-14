package shared

// GENERIC CONFIG FIELDS
type Meta struct {
	ConfigType     string `mapstructure:"@type" exclude:"true" description:"Identification for renderer"`
	ConfigCategory string
	Key            string `mapstructure:"key" exclude:"true"`
	Path           string `mapstructure:"path" exclude:"true"`
	File           string `mapstructure:"file" exclude:"true"`
}

type CompositeRendererConfig struct {
	Meta  `mapstructure:",squash"`
	Items map[string]interface{} `mapstructure:",remain"`
}

type Composite = CompositeRendererConfig

// Basic config for ComponentRenderers
type ComponentRendererConfig struct {
	Meta            `mapstructure:",squash"` // Embedding RendererConfig
	ExtraAttributes map[string]interface{}   `mapstructure:"attributes" description:"Extra attributes like id, data-role, data-action" example:"{!{link-attributes.hyperbricks}}"`
	Enclose         string                   `mapstructure:"enclose" description:"The wrapping HTML element for the header divided by |" example:"{!{link-wrap.hyperbricks}}"`
}

type Component = ComponentRendererConfig

// Example Extended struct
type ExtendedRendererConfig struct {
	Meta  // Embedding RendererConfig
	Value string
}
