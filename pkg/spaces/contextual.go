package spaces

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	htmltoken "golang.org/x/net/html"
)

// ContextualPage adds native editorial controls to an explicitly requested
// development preview. The caller owns response type, status and static-export
// boundaries. This method independently enforces the editor's access policy.
func (h *Handler) ContextualPage(r *http.Request, route, content string) (string, bool, error) {
	cfg := shared.GetHyperBricksConfiguration()
	runtime := shared.GetRuntimeOptions()
	if r == nil || r.URL == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) ||
		cfg.Mode != shared.DEVELOPMENT_MODE || runtime.Production || !cfg.Development.FrontendEditing.Enabled || cfg.ValidateFrontendEditing() != nil {
		return content, false, nil
	}
	query := r.URL.Query()["edit"]
	if len(query) != 1 || query[0] != "true" {
		return content, false, nil
	}
	options := cfg.Development.FrontendEditing.Spaces
	// Check the same trusted hosts before loading any editorial source metadata.
	if !(&service{allowedHosts: options.AllowedHosts}).allowedHost(r.Host) {
		return content, false, nil
	}
	payload := contextualPayload{Editor: options.Route, Write: options.Write, Fields: []contextualField{}}
	s, err := newService(runtime.ModuleRoot, options.Route, cfg.Directories, options, cfg.Development.Watch)
	if err == nil {
		s.config = parser.HbConfig
		h.mu.Lock()
		payload, err = s.contextualPayload(route)
		h.mu.Unlock()
	}
	if err != nil {
		payload = contextualPayload{Editor: options.Route, Write: options.Write, Fields: []contextualField{}, Error: "Contextual editing could not load this Space. Check the development render diagnostics."}
	}
	return injectContextualPage(content, payload), true, err
}

type contextualField struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type contextualPayload struct {
	Name   string            `json:"name"`
	Editor string            `json:"editor"`
	Write  bool              `json:"write"`
	Fields []contextualField `json:"fields"`
	Error  string            `json:"error,omitempty"`
}

func (s *service) contextualPayload(route string) (contextualPayload, error) {
	result := contextualPayload{Editor: s.route, Write: s.write, Fields: []contextualField{}}
	c, err := s.catalog()
	if err != nil {
		return result, err
	}
	route = strings.Trim(route, "/")
	if route == "" {
		route = "index"
	}
	for _, entry := range c.entries {
		if entry.trashed {
			continue
		}
		space, err := s.space(c, entry)
		if err != nil {
			return result, err
		}
		candidate := strings.Trim(space.Route, "/")
		if candidate == "" {
			candidate = "index"
		}
		if candidate != route {
			continue
		}
		if result.Name != "" {
			return result, fmt.Errorf("multiple Spaces own route %q", route)
		}
		result.Name = space.Name
		for _, field := range space.Fields {
			result.Fields = append(result.Fields, contextualField{ID: field.ID, Label: field.Label})
		}
	}
	if result.Name == "" {
		result.Error = "This page is not an active managed Space. Open Spaces to select a page."
	}
	return result, nil
}

func injectContextualPage(content string, payload contextualPayload) string {
	// encoding/json escapes <, > and &, so field labels cannot end the data script.
	data, _ := json.Marshal(payload)
	assets := `<script id="hb-spaces-context" type="application/json">` + string(data) + `</script>` +
		`<script src="` + html.EscapeString(payload.Editor+"/web/contextual.js") + `" defer></script>`
	// Tokenization locates the real body end without matching text inside scripts
	// or reserializing the author's markup (which could alter whitespace/content).
	tokens := htmltoken.NewTokenizer(strings.NewReader(content))
	offset, insertion := 0, -1
	for {
		token := tokens.Next()
		start := offset
		offset += len(tokens.Raw())
		if token == htmltoken.ErrorToken {
			break
		}
		if token == htmltoken.EndTagToken {
			name, _ := tokens.TagName()
			if string(name) == "body" {
				insertion = start
			}
		}
	}
	if insertion < 0 {
		return content + assets
	}
	return content[:insertion] + assets + content[insertion:]
}
