package renderplan

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// ErrNotEligible marks routes or nodes that must keep using the legacy map
// pipeline. It is not a configuration error.
var ErrNotEligible = errors.New("render plan not eligible")

type notEligibleError struct {
	reason string
}

func (e notEligibleError) Error() string {
	return fmt.Sprintf("%s: %s", ErrNotEligible, e.reason)
}

func (e notEligibleError) Unwrap() error {
	return ErrNotEligible
}

func notEligible(reason string) error {
	return notEligibleError{reason: reason}
}

// Plan is an immutable route render graph assembled during module load.
type Plan struct {
	root                   renderNode
	querySets              [][]string
	needsAPIRequestContext bool
}

// Render executes the prepared graph with request-local context.
func (p *Plan) Render(ctx context.Context) (string, []error) {
	if p == nil || p.root == nil {
		return "", []error{fmt.Errorf("render plan is empty")}
	}
	return p.root.Render(newRenderState(ctx, p.querySets))
}

type renderNode interface {
	Render(state *renderState) (string, []error)
}

type renderState struct {
	ctx         context.Context
	hasRequest  bool
	paramsBySet []map[string]interface{}
}

func newRenderState(ctx context.Context, querySets [][]string) *renderState {
	state := &renderState{ctx: ctx}
	if ctx == nil {
		return state
	}
	request, ok := ctx.Value(shared.Request).(*http.Request)
	if !ok || request == nil || request.URL == nil {
		return state
	}

	state.hasRequest = true
	query := request.URL.Query()
	state.paramsBySet = make([]map[string]interface{}, len(querySets))
	for index, allowedKeys := range querySets {
		params := make(map[string]interface{}, len(allowedKeys))
		for _, key := range allowedKeys {
			values, found := query[key]
			if !found {
				continue
			}
			if len(values) == 1 {
				params[key] = values[0]
			} else {
				params[key] = append([]string(nil), values...)
			}
		}
		state.paramsBySet[index] = params
	}
	return state
}

func (s *renderState) params(querySet int, private bool) (map[string]interface{}, bool) {
	if s == nil || !s.hasRequest || querySet < 0 || querySet >= len(s.paramsBySet) {
		return nil, false
	}
	source := s.paramsBySet[querySet]
	if !private {
		return source, true
	}
	params := make(map[string]interface{}, len(source))
	for key, value := range source {
		if values, ok := value.([]string); ok {
			params[key] = append([]string(nil), values...)
			continue
		}
		params[key] = value
	}
	return params, true
}
