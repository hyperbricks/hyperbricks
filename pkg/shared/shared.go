package shared

import "context"

// Define a context key for the JWT token
type contextKey string

const JwtKey contextKey = "jwtToken"
const RequestBody contextKey = "requestBody"
const Request contextKey = "request"
const FormData contextKey = "formData"
const ResponseWriter contextKey = "ResponseWriter"

// PluginConfig is a generic configuration map for plugins.
type PluginConfig map[string]interface{}

// RenderPlugin defines the interface for dynamic plugins.
type PluginRenderer interface {
	Render(data interface{}, ctx context.Context) (any, []error)
}

type Renderer interface {
	Render(instance interface{}, ctx context.Context) (string, []error)
	Types() []string
}

// CompositeRenderer extends Renderer to handle composite rendering (itself and children).
type CompositeRenderer interface {
	Renderer
	// Additional methods for CompositeRenderer can be added here
}

type ComponentRenderer interface {
	Renderer
	// Additional methods for ComponentRenderer can be added here
}

// RenderFunc defines the signature for the render callback.
type RenderFunc func(contentType string, data interface{}) (string, []error)
