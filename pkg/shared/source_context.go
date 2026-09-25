package shared

import "github.com/mitchellh/mapstructure"

// SourceContext is immutable runtime provenance, never source-authored config.
type SourceContext struct {
	File      string                    `mapstructure:"file" json:"file"`
	Line      int                       `mapstructure:"line" json:"line"`
	Column    int                       `mapstructure:"column" json:"column"`
	Path      string                    `mapstructure:"path" json:"path"`
	Key       string                    `mapstructure:"key" json:"key"`
	Fields    map[string]SourceLocation `mapstructure:"fields" json:"fields"`
	Resources map[string]string         `mapstructure:"resources" json:"resources"`
}

type SourceLocation struct {
	File   string `mapstructure:"file" json:"file"`
	Line   int    `mapstructure:"line" json:"line"`
	Column int    `mapstructure:"column" json:"column"`
}

func MetaFromConfig(data map[string]interface{}) Meta {
	var meta Meta
	_ = mapstructure.Decode(data, &meta)
	return meta
}

func (meta Meta) RenderPath() string {
	if meta.Source != nil {
		return meta.Source.Path
	}
	return meta.HyperBricksPath + meta.HyperBricksKey
}

func (meta Meta) Resource(field string) string {
	if meta.Source == nil {
		return ""
	}
	return meta.Source.Resources[field]
}

func (meta Meta) TemplateName(fallback string) string {
	if resource := meta.Resource("template"); resource != "" {
		return resource
	}
	if fallback != "" {
		return fallback
	}
	if meta.Source != nil {
		return meta.Source.File + "#" + meta.Source.Path + ".inline"
	}
	return "hyperbricks-generic-template"
}

func (source *SourceContext) Apply(data map[string]interface{}) {
	if source == nil {
		return
	}
	data["@source"] = source
	data["hyperbricksfile"] = source.File
	data["hyperbrickspath"] = source.Path
	data["hyperbrickskey"] = source.Key
}
