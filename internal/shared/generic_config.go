package shared

// GENERIC CONFIG FIELDS
type Meta struct {
	ConfigType      string `mapstructure:"@type" exclude:"true" description:"Identification for renderer"`
	ConfigCategory  string
	HyperBricksKey  string `mapstructure:"hyperbrickskey" exclude:"true"`
	HyperBricksPath string `mapstructure:"hyperbrickspath" exclude:"true"`
	HyperBricksFile string `mapstructure:"hyperbricksfile" exclude:"true"`
}

type CompositeRendererConfig struct {
	Meta  `mapstructure:",squash"`
	Items map[string]interface{} `mapstructure:",remain"`
}

type Composite = CompositeRendererConfig

// Basic config for ComponentRenderers
type ComponentRendererConfig struct {
	Meta            `mapstructure:",squash"` // Embedding RendererConfig
	ExtraAttributes map[string]interface{}   `mapstructure:"attributes" description:"Extra attributes like id, data-role, data-action"`
	Enclose         string                   `mapstructure:"enclose" description:"The enclosing HTML element for the header divided by |"`
}

type Component = ComponentRendererConfig

// Example Extended struct
type ExtendedRendererConfig struct {
	Meta  // Embedding RendererConfig
	Value string
}
