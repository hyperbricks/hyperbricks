package renderplan

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type typedNode struct {
	renderer shared.Renderer
	instance interface{}
	warnings []string
}

func (n *typedNode) Render(state *renderState) (string, []error) {
	errors := warningErrors(n.warnings)
	output, renderErrors := n.renderer.Render(n.instance, state.ctx)
	return output, append(errors, renderErrors...)
}

type hyperMediaNode struct {
	config   composite.HyperMediaConfig
	template renderNode
	warnings []string
}

func (n *hyperMediaNode) Render(state *renderState) (string, []error) {
	config := n.config
	errors := warningErrors(n.warnings)
	if config.ConfigType != composite.HyperMediaConfigGetName() {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     composite.HyperMediaConfigGetName(),
			Err:      "invalid type",
			Rejected: true,
		})
	}
	errors = append(errors, config.Validate()...)

	output, renderErrors := n.template.Render(state)
	errors = append(errors, renderErrors...)
	output = shared.EncloseContent(config.Enclose, output)

	hbConfig := shared.GetHyperBricksConfiguration()
	if hbConfig.Development.FrontendErrors && hbConfig.Mode != shared.LIVE_MODE {
		output += composite.ErrorPanelTemplate
	}
	return output, errors
}

type templateValue struct {
	key    string
	static interface{}
	child  renderNode
}

type templateNode struct {
	config       composite.TemplateConfig
	template     *template.Template
	values       []templateValue
	querySet     int
	params       paramsPolicy
	warnings     []string
	forwardChild renderNode
	literal      *string
}

func (n *templateNode) Render(state *renderState) (string, []error) {
	config := n.config
	errors := warningErrors(n.warnings)
	errors = append(errors, config.Validate()...)
	if n.literal != nil {
		return shared.EncloseContent(config.Enclose, *n.literal), errors
	}
	if n.forwardChild != nil {
		output, renderErrors := n.forwardChild.Render(state)
		return shared.EncloseContent(config.Enclose, output), append(errors, renderErrors...)
	}

	data := make(map[string]interface{}, len(n.values)+1)
	for _, value := range n.values {
		if value.child != nil {
			result, renderErrors := value.child.Render(state)
			data[value.key] = template.HTML(result)
			errors = append(errors, renderErrors...)
			continue
		}
		switch typed := value.static.(type) {
		case map[string]interface{}:
			data[value.key] = shared.CloneMapDeep(typed)
		case []string:
			data[value.key] = append([]string(nil), typed...)
		default:
			data[value.key] = typed
		}
	}
	if params, ok := state.params(n.querySet, n.params == paramsPrivate); ok {
		data["Params"] = params
	}

	var output strings.Builder
	if err := n.template.Execute(&output, data); err != nil {
		errors = append(errors, fmt.Errorf("error executing template: %v", err))
		return shared.EncloseContent(config.Enclose, ""), errors
	}
	rendered := output.String()
	if config.Enclose != "" {
		rendered = shared.EncloseContent(config.Enclose, rendered)
	}
	return rendered, errors
}

type treeChild struct {
	key  string
	node renderNode
}

type treeNode struct {
	enclose  string
	children []treeChild
	warnings []string
}

func (n *treeNode) Render(state *renderState) (string, []error) {
	errors := warningErrors(n.warnings)
	parts := make([]string, len(n.children))
	for index, child := range n.children {
		rendered, renderErrors := child.node.Render(state)
		parts[index] = rendered
		errors = append(errors, renderErrors...)
	}
	if len(errors) > 1 {
		keys := make([]string, len(n.children))
		for index, child := range n.children {
			keys[index] = child.key
		}
		errors = composite.SortCompositeErrors(errors, keys)
	}
	return shared.EncloseContent(n.enclose, strings.Join(parts, "")), errors
}

func warningErrors(warnings []string) []error {
	if len(warnings) == 0 {
		return nil
	}
	errors := make([]error, 0, len(warnings))
	for _, warning := range warnings {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Err:      warning,
			Rejected: false,
		})
	}
	return errors
}
