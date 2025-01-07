package shared

// GENERIC CONFIG FIELDS
type Meta struct {
	ConfigType     string `mapstructure:"@type"`
	ConfigCategory string
	Key            string `mapstructure:"key"`
	Path           string `mapstructure:"path"`
	File           string `mapstructure:"file"`
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
