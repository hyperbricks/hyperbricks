package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestServeContent_NativeStreamConcurrentClientIsolation(t *testing.T) {
	for _, live := range []bool{false, true} {
		for _, guarded := range []bool{false, true} {
			t.Run(fmt.Sprintf("live=%t/guarded=%t", live, guarded), func(t *testing.T) {
				runNativeStreamConcurrentClientIsolation(t, live, guarded)
			})
		}
	}
}

func runNativeStreamConcurrentClientIsolation(t *testing.T, live, guarded bool) {
	t.Helper()
	setupNativeStreamMode(t, live)
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	aCtx, cancelA := context.WithCancel(ctx)
	defer cancelA()
	identities := []string{"client_alpha", "client_beta"}
	type controls struct {
		advance chan int
		done    chan error
	}
	clients := map[string]*controls{}
	for _, identity := range identities {
		clients[identity] = &controls{advance: make(chan int), done: make(chan error, 1)}
	}
	var producers atomic.Int32
	plugin := &nativeStreamTestPlugin{response: func(renderCtx context.Context) shared.HandledResponse {
		req := renderCtx.Value(shared.Request).(*http.Request)
		identity := req.Header.Get("X-Client-Identity")
		payloadBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read request body for %s: %v", identity, err)
			return shared.HandledResponse{Status: http.StatusInternalServerError}
		}
		// The registered plugin is shared. Each callback must capture its own
		// request values, while the test controls below only schedule its steps.
		payload := string(payloadBytes)
		query := req.URL.Query().Get("secret")
		cookie := req.Header.Get("Cookie")
		auth := req.Header.Get("Authorization")
		control := clients[identity]
		if control == nil {
			t.Errorf("unexpected client reached the plugin: %q", identity)
			return shared.HandledResponse{Status: http.StatusInternalServerError}
		}
		return shared.HandledResponse{
			ContentType: "text/plain; charset=utf-8",
			Headers:     map[string]string{"X-Client-Identity": identity, "X-Client-Secret": query},
			Cookies:     []string{"stream-client=" + identity + "; HttpOnly; SameSite=Strict"},
			Stream: func(streamCtx context.Context, out io.Writer, flush func() error) (err error) {
				producers.Add(1)
				defer func() { control.done <- err }()
				for step := 0; step < 4; step++ {
					if step > 0 {
						select {
						case <-streamCtx.Done():
							return streamCtx.Err()
						case next := <-control.advance:
							if next != step {
								return fmt.Errorf("wrong scheduled step %d, wanted %d", next, step)
							}
						}
					}
					if _, err := fmt.Fprintf(out, "%s|%s|%s|%s|%s|step=%d\n", identity, payload, query, cookie, auth, step); err != nil {
						return err
					}
					if err := flush(); err != nil {
						return err
					}
				}
				return nil
			},
		}
	}}
	config := nativeStreamRouteConfig()
	var authorizationCalls atomic.Int32
	if guarded {
		authorizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authorizationCalls.Add(1)
			for _, identity := range identities {
				if req.Header.Get("Authorization") == "Bearer token_"+identity {
					w.WriteHeader(http.StatusOK)
					return
				}
			}
			w.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(authorizer.Close)
		config["guard"] = map[string]interface{}{
			"enabled": true, "require": map[string]interface{}{"authenticated": true},
			"authorize":          map[string]interface{}{"endpoint": authorizer.URL},
			"on_unauthenticated": map[string]interface{}{"default": map[string]interface{}{"status": http.StatusUnauthorized}},
		}
	}
	installNativeStreamRoute(t, false, config, plugin)
	server := newNativeStreamHTTPServer(t)
	type receivedResponse struct {
		identity string
		response *http.Response
		err      error
	}
	arrivals := make(chan receivedResponse, 2)
	for index, identity := range identities {
		requestCtx := ctx
		if index == 0 {
			requestCtx = aCtx
		}
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, server.URL+"/native-stream?secret=query_"+identity, strings.NewReader("payload_"+identity))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Client-Identity", identity)
		req.Header.Set("Cookie", "identity="+identity)
		req.Header.Set("Authorization", "Bearer token_"+identity)
		go func() {
			response, err := server.Client().Do(req)
			arrivals <- receivedResponse{identity, response, err}
		}()
	}
	responses := map[string]*http.Response{}
	readers := map[string]*bufio.Reader{}
	for range 2 {
		var result receivedResponse
		select {
		case result = <-arrivals:
		case <-ctx.Done():
			t.Fatal("both clients did not receive response headers before the deadline")
		}
		if result.err != nil {
			t.Fatal(result.err)
		}
		defer result.response.Body.Close()
		responses[result.identity] = result.response
		readers[result.identity] = bufio.NewReader(result.response.Body)
		if result.response.StatusCode != http.StatusOK {
			t.Fatalf("client %s status %d", result.identity, result.response.StatusCode)
		}
		if result.response.Header.Get("X-Client-Identity") != result.identity || result.response.Header.Get("X-Client-Secret") != "query_"+result.identity {
			t.Fatalf("metadata crossed clients: identity=%s headers=%v", result.identity, result.response.Header)
		}
		gotCookies := result.response.Cookies()
		if len(gotCookies) != 1 || gotCookies[0].Name != "stream-client" || gotCookies[0].Value != result.identity {
			t.Fatalf("cookie crossed clients: %s %v", result.identity, gotCookies)
		}
		assertNativeStreamUncachedHeaders(t, result.response)
	}
	if responses[identities[0]].Header.Get(requestIDHeader) == responses[identities[1]].Header.Get(requestIDHeader) {
		t.Fatal("separate requests shared request ID")
	}
	readStep := func(identity string, step int) {
		t.Helper()
		line, err := readers[identity].ReadString('\n')
		want := fmt.Sprintf("%s|payload_%s|query_%s|identity=%s|Bearer token_%s|step=%d\n", identity, identity, identity, identity, identity, step)
		if err != nil || line != want {
			t.Fatalf("client %s step %d got %q error=%v; want %q", identity, step, line, err, want)
		}
	}
	advance := func(identity string, step int) {
		t.Helper()
		select {
		case clients[identity].advance <- step:
		case <-ctx.Done():
			t.Fatal("stream stopped accepting its own steps")
		}
		readStep(identity, step)
	}
	// Both streams stay open while their chunks alternate. Cancelling the
	// first client must leave the second client's later steps unaffected.
	readStep(identities[0], 0)
	readStep(identities[1], 0)
	advance(identities[1], 1)
	advance(identities[0], 1)
	if guarded {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/native-stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer invalid_third_client_token")
		// A nonempty invalid token must reach the authorizer and be rejected;
		// this checks authorization rather than only the token-presence rule.
		denied, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(denied.Body)
		denied.Body.Close()
		if err != nil || denied.StatusCode != http.StatusForbidden {
			t.Fatalf("third client bypassed guard: %d body=%q err=%v", denied.StatusCode, body, err)
		}
		for _, identity := range identities {
			if strings.Contains(string(body)+fmt.Sprint(denied.Header), identity) {
				t.Fatal("denied client received another client's sentinel")
			}
		}
		if plugin.calls.Load() != 2 || producers.Load() != 2 {
			t.Fatalf("denied client started plugin/producer: %d/%d", plugin.calls.Load(), producers.Load())
		}
		if authorizationCalls.Load() != 3 {
			t.Fatalf("expected external authorizer to validate both accepted tokens and reject invalid third token: calls=%d", authorizationCalls.Load())
		}
	}
	cancelA()
	responses[identities[0]].Body.Close()
	select {
	case err := <-clients[identities[0]].done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled A returned %v", err)
		}
	case <-ctx.Done():
		t.Fatal("cancelled A producer did not stop")
	}
	advance(identities[1], 2)
	advance(identities[1], 3)
	if trailing, err := readers[identities[1]].ReadString('\n'); trailing != "" || !errors.Is(err, io.EOF) {
		t.Fatalf("B end-of-stream contains unexpected output %q err=%v", trailing, err)
	}
	select {
	case err := <-clients[identities[1]].done:
		if err != nil {
			t.Fatalf("A cancellation affected B: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("B did not complete")
	}
	if plugin.calls.Load() != 2 || producers.Load() != 2 {
		t.Fatalf("expected one render/producer per accepted client: %d/%d", plugin.calls.Load(), producers.Load())
	}
	htmlCacheMutex.RLock()
	cacheSize := len(htmlCache)
	htmlCacheMutex.RUnlock()
	if cacheSize != 0 {
		t.Fatalf("request streams left %d cache entries", cacheSize)
	}
}
