package shared

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// Define a context key for the JWT token
type contextKey string

const JwtKey contextKey = "jwtToken"
const RequestBody contextKey = "requestBody"
const Request contextKey = "request"
const FormData contextKey = "formData"
const ResponseWriter contextKey = "ResponseWriter"
const CurrentRoute contextKey = "currentRoute"
const HandledResponseCaptureKey contextKey = "handledResponseCapture"

// PluginConfig is a generic configuration map for plugins.
type PluginConfig map[string]interface{}

// RenderPlugin defines the interface for dynamic plugins.
type PluginRenderer interface {
	Render(data interface{}, ctx context.Context) (any, []error)
}

type HandledResponse struct {
	Status      int
	ContentType string
	Headers     map[string]string
	Cookies     []string
	Body        []byte
	NoCache     bool
	// Stream produces the response body after rendering has finished. The HTTP
	// server owns status, headers and cookies; the callback owns only the body.
	// Body and Stream are mutually exclusive. Stream responses are never cached.
	// Write sequentially, flush when a chunk is ready, and stop on context
	// cancellation or a write/flush error. Do not retain output or flush after
	// the callback returns. The server's configured write timeout still applies.
	Stream func(ctx context.Context, output io.Writer, flush func() error) error
}

type HandledResponseCapture struct {
	// Response is retained for callers inspecting a completed render. Use Store
	// and Result when capturing a response from concurrently rendered children.
	Response *HandledResponse
	mu       sync.Mutex
	err      error
}

// Store allows exactly one response owner for a request. A conflict invalidates
// the entire capture so the order of concurrent children cannot choose a winner.
func (c *HandledResponseCapture) Store(response *HandledResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	if response == nil {
		return nil
	}
	if response.Stream != nil && len(response.Body) > 0 {
		c.err = fmt.Errorf("plugin response cannot contain both Body and Stream")
	} else if c.Response != nil {
		c.err = fmt.Errorf("multiple plugins returned a response for one request")
	}
	if c.err != nil {
		c.Response = nil
		return c.err
	}
	c.Response = response
	return nil
}

// Result is read after rendering finishes, before the HTTP response is written.
func (c *HandledResponseCapture) Result() (*HandledResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Response, c.err
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
