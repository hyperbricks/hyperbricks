package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type fields struct {
	Action   string `mapstructure:"action"`
	Template string `mapstructure:"template"`
}

type config struct {
	shared.Component `mapstructure:",squash"`
	Fields           fields `mapstructure:"data"`
}

type projectDeskPlugin struct{}

var _ shared.PluginRenderer = (*projectDeskPlugin)(nil)

func (p *projectDeskPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {
	var cfg config
	if err := shared.DecodeWithBasicHooks(instance, &cfg); err != nil {
		return "", []error{fmt.Errorf("project desk config: %v", err)}
	}
	if cfg.Fields.Template == "" {
		return "", []error{fmt.Errorf("project desk requires data.template")}
	}
	req, _ := ctx.Value(shared.Request).(*http.Request)
	if req == nil {
		return "", []error{fmt.Errorf("project desk requires request context")}
	}
	if cfg.Fields.Action != "preview" && cfg.Fields.Action != "confirm" {
		return "", []error{fmt.Errorf("unknown project desk action %q", cfg.Fields.Action)}
	}

	values := map[string]interface{}{"step": "input"}
	if req.Method != http.MethodPost {
		values["message"] = "Use the project-name form to begin this review."
	} else if err := req.ParseForm(); err != nil {
		values["message"] = "The form could not be read. Please try again."
	} else {
		name := strings.TrimSpace(req.FormValue("name"))
		values["name"] = name
		if name == "" || utf8.RuneCountInString(name) > 80 {
			values["message"] = "Enter a project name between 1 and 80 characters."
		} else if cfg.Fields.Action == "preview" {
			values["step"] = "preview"
			values["words"] = strconv.Itoa(len(strings.Fields(name)))
		} else {
			values["step"] = "confirmed"
		}
	}

	// Let HyperBricks render HTML through its normal template component.
	// This is runtime config, so @type is correct here; authored YAML uses type.
	return map[string]interface{}{
		"@type": "<TEMPLATE>", "template": cfg.Fields.Template, "values": values,
	}, nil
}

func Plugin() (shared.PluginRenderer, error) {
	return &projectDeskPlugin{}, nil
}
