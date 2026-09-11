package composite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestAPIFragmentRenderBodyReadFailureDoesNotCallUpstreamOrExposeCause(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	incoming := httptest.NewRequest(http.MethodPost, "https://browser.example.test/action", nil)
	ctx := context.WithValue(incoming.Context(), shared.Request, incoming)
	ctx = context.WithValue(ctx, shared.RequestBody, failingAPIFragmentRequestBody{err: errors.New("reader-secret")})
	config := ApiFragmentRenderConfig{
		APIConfig: APIConfig{
			Endpoint: upstream.URL,
			Method:   http.MethodPost,
			Body:     `{"name":"$name"}`,
			Inline:   `{{.Status}}`,
		},
		Route: "body-read-failure",
	}

	output, renderErrors := (&ApiFragmentRenderer{}).Render(config, ctx)
	if calls := upstreamCalls.Load(); calls != 0 {
		t.Fatalf("upstream received %d request(s) after body read failure", calls)
	}
	if len(renderErrors) != 1 || !strings.Contains(renderErrors[0].Error(), "failed to read request body") {
		t.Fatalf("render errors = %v, want one body read error", renderErrors)
	}
	diagnostic := output + fmt.Sprint(renderErrors)
	if strings.Contains(diagnostic, "reader-secret") {
		t.Fatalf("body reader cause reached component diagnostics: %q", diagnostic)
	}
}

type failingAPIFragmentRequestBody struct {
	err error
}

func (body failingAPIFragmentRequestBody) Read([]byte) (int, error) {
	return 0, body.err
}

func (failingAPIFragmentRequestBody) Close() error {
	return nil
}
