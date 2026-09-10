package renderplan

import (
	"github.com/hyperbricks/hyperbricks/pkg/component"
)

type gojaNode struct {
	prepared *component.PreparedGojaRender
	querySet int
	warnings []string
}

func (c *compiler) compileGoja(raw map[string]interface{}) (renderNode, error) {
	response, err := c.manager.MakeInstance(typeRequest(component.GojaRenderConfigGetName(), raw))
	if err != nil {
		return nil, err
	}
	config := response.Instance.(component.GojaRenderConfig)
	prepared, ok := config.Prepared.(*component.PreparedGojaRender)
	if !ok || prepared == nil {
		prepared = component.PrepareGojaRender(config, c.templateProvider)
	}
	if err := prepared.Err(); err != nil {
		return nil, err
	}
	return &gojaNode{
		prepared: prepared,
		querySet: c.registerQuerySet(prepared.QueryKeys()),
		warnings: append([]string(nil), response.Warnings...),
	}, nil
}

func (n *gojaNode) Render(state *renderState) (string, []error) {
	params, _ := state.params(n.querySet, false)
	output, errs := n.prepared.Render(state.ctx, params)
	return output, append(warningErrors(n.warnings), errs...)
}
