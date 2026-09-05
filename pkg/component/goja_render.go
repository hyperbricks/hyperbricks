package component

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/gojaruntime"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// GojaPreparedKey is runtime-only metadata owned by the loaded route snapshot.
const GojaPreparedKey = "@goja_prepared"

type GojaRenderConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"Run trusted project JavaScript with isolated request state and render its returned data through a Go HTML template."`
	Script             string                 `mapstructure:"script" validate:"required" description:"JavaScript declaring main(input). Use the file resolver to load source from resources."`
	Template           string                 `mapstructure:"template" description:"Preloaded Go template file. The script result is available as .Data."`
	Inline             string                 `mapstructure:"inline" description:"Inline Go HTML template. Mutually exclusive with template."`
	Values             map[string]interface{} `mapstructure:"values" description:"Plain input data, copied into each script execution. Nested components are not rendered."`
	AllowedQueryKeys   []string               `mapstructure:"querykeys" description:"Explicitly allowed query keys. No query parameters are exposed by default."`
	Timeout            string                 `mapstructure:"timeout" description:"Script deadline, default 100ms. Must be positive and at most 5s."`
	Prepared           interface{}            `mapstructure:"@goja_prepared" json:"-" exclude:"true"`
}

type GojaRenderer struct{}

var _ shared.ComponentRenderer = (*GojaRenderer)(nil)

func GojaRenderConfigGetName() string   { return "<GOJA_RENDER>" }
func (r *GojaRenderer) Types() []string { return []string{GojaRenderConfigGetName()} }

// PreparedGojaRender owns immutable load-time resources, not execution state.
type PreparedGojaRender struct {
	component shared.Component
	program   *gojaruntime.Program
	template  *template.Template
	values    json.RawMessage
	queryKeys []string
	err       error
}

func PrepareGojaRender(config GojaRenderConfig, provider func(string) (string, bool)) *PreparedGojaRender {
	p := &PreparedGojaRender{component: config.Component, queryKeys: append([]string(nil), config.AllowedQueryKeys...)}
	p.err = p.prepare(config, provider)
	return p
}

func (p *PreparedGojaRender) prepare(config GojaRenderConfig, provider func(string) (string, bool)) error {
	timeout := gojaruntime.DefaultTimeout
	if config.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(config.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}
	if (config.Inline == "") == (config.Template == "") {
		return fmt.Errorf("exactly one of inline or template is required")
	}
	content := config.Inline
	if config.Template != "" {
		if provider == nil {
			return fmt.Errorf("template %q is not preloaded", config.Template)
		}
		var found bool
		content, found = provider(config.Template)
		if !found {
			return fmt.Errorf("template %q is not preloaded", config.Template)
		}
	}
	var err error
	p.template, err = shared.ParsedGenericTemplate(content)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	p.values = json.RawMessage(`{}`)
	if config.Values != nil {
		p.values, err = json.Marshal(config.Values)
		if err != nil {
			return fmt.Errorf("values must be JSON-compatible: %w", err)
		}
	}
	if len(p.values) > gojaruntime.MaxDataBytes {
		return fmt.Errorf("values exceed %d bytes", gojaruntime.MaxDataBytes)
	}
	name := config.Meta.HyperBricksFile + "#" + config.Meta.HyperBricksPath
	p.program, err = gojaruntime.Compile(name, config.Script, timeout)
	return err
}

func (p *PreparedGojaRender) Err() error {
	if p == nil {
		return fmt.Errorf("goja_render was not prepared during module loading")
	}
	return p.err
}

func (p *PreparedGojaRender) QueryKeys() []string {
	return append([]string(nil), p.queryKeys...)
}

func (r *GojaRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	config, ok := instance.(GojaRenderConfig)
	if !ok {
		return "", []error{fmt.Errorf("invalid type for GojaRenderer: %T", instance)}
	}
	p, ok := config.Prepared.(*PreparedGojaRender)
	if !ok || p == nil {
		return "", []error{gojaComponentError(config.Component, fmt.Errorf("goja_render was not prepared during module loading"))}
	}
	params := make(map[string]interface{})
	if ctx != nil {
		if request, ok := ctx.Value(shared.Request).(*http.Request); ok && request != nil && request.URL != nil {
			for key, values := range FilterAllowedQueryParams(request, p.queryKeys) {
				if len(values) == 1 {
					params[key] = values[0]
				} else {
					params[key] = values
				}
			}
		}
	}
	return p.Render(ctx, params)
}

func (p *PreparedGojaRender) Render(ctx context.Context, params map[string]interface{}) (string, []error) {
	if err := p.Err(); err != nil {
		if p == nil {
			return "", []error{err}
		}
		return "", []error{gojaComponentError(p.component, err)}
	}
	query := make(map[string]interface{}, len(p.queryKeys))
	for _, key := range p.queryKeys {
		if value, found := params[key]; found {
			query[key] = value
		}
	}
	input, err := json.Marshal(struct {
		Query  map[string]interface{} `json:"query"`
		Values json.RawMessage        `json:"values"`
	}{Query: query, Values: p.values})
	if err != nil {
		return "", []error{gojaComponentError(p.component, fmt.Errorf("encode script input: %w", err))}
	}
	data, err := p.program.Run(ctx, input)
	if err != nil {
		return "", []error{gojaComponentError(p.component, err)}
	}
	var output strings.Builder
	if err := p.template.Execute(&output, map[string]interface{}{"Data": data}); err != nil {
		return "", []error{gojaComponentError(p.component, fmt.Errorf("execute template: %w", err))}
	}
	return shared.EncloseContent(p.component.Enclose, output.String()), nil
}

func gojaComponentError(config shared.Component, err error) error {
	return shared.ComponentError{
		Hash: shared.GenerateHash(), Type: GojaRenderConfigGetName(), Rejected: true,
		File: config.Meta.HyperBricksFile, Path: config.Meta.HyperBricksPath,
		Key: config.Meta.HyperBricksKey, Err: err.Error(),
	}
}
