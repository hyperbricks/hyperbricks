package renderplan

import "github.com/hyperbricks/hyperbricks/pkg/component"

type esbuildNode struct {
	prepared *component.PreparedEsbuild
	warnings []string
}

func (c *compiler) compileEsbuild(raw map[string]interface{}) (renderNode, error) {
	response, err := c.manager.MakeInstance(typeRequest(component.EsbuildConfigGetName(), raw))
	if err != nil {
		return nil, err
	}
	config := response.Instance.(component.EsbuildConfig)
	prepared, _ := config.Prepared.(*component.PreparedEsbuild)
	if err := prepared.Err(); err != nil {
		return nil, err
	}
	return &esbuildNode{prepared: prepared, warnings: append([]string(nil), response.Warnings...)}, nil
}

func (n *esbuildNode) Render(state *renderState) (string, []error) {
	output, errs := n.prepared.Render(state.ctx)
	return output, append(warningErrors(n.warnings), errs...)
}
