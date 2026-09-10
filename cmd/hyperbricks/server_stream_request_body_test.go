package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestServeContent_NativeStreamRequestBodyPreflight(t *testing.T) {
	for _, tc := range []struct {
		name    string
		size    int
		consume int
		chunked bool
		status  int
	}{
		{name: "absent", status: http.StatusOK},
		{name: "small", size: 32, status: http.StatusOK},
		{name: "chunked", size: 32, chunked: true, status: http.StatusOK},
		{name: "limit", size: maxStreamUnreadRequestBody, status: http.StatusOK},
		{name: "oversized", size: maxStreamUnreadRequestBody + 1, status: http.StatusRequestEntityTooLarge},
		{name: "oversized-chunked", size: maxStreamUnreadRequestBody + 1, chunked: true, status: http.StatusRequestEntityTooLarge},
		{name: "large-consumed", size: 2 * maxStreamUnreadRequestBody, consume: 2 * maxStreamUnreadRequestBody, status: http.StatusOK},
		{name: "large-partially-consumed", size: 2 * maxStreamUnreadRequestBody, consume: maxStreamUnreadRequestBody, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			var producers atomic.Int32
			plugin := &nativeStreamTestPlugin{response: func(ctx context.Context) shared.HandledResponse {
				if tc.consume > 0 {
					request := ctx.Value(shared.Request).(*http.Request)
					if _, err := io.CopyN(io.Discard, request.Body, int64(tc.consume)); err != nil {
						t.Errorf("consume request in Render: %v", err)
					}
				}
				return shared.HandledResponse{
					Headers: map[string]string{"X-Stream-Metadata": "private"},
					Cookies: []string{"stream-cookie=private; Path=/"},
					Stream: func(_ context.Context, w io.Writer, flush func() error) error {
						producers.Add(1)
						_, err := io.WriteString(w, "streamed\n")
						return err
					},
				}
			}}
			config := nativeStreamRouteConfig()
			config["nocache"] = true
			installNativeStreamRoute(t, false, config, plugin)
			server := newStreamBodyTestServer(t, http.HandlerFunc(ServeContent), time.Second)
			var body io.Reader
			if tc.size > 0 {
				body = bytes.NewReader(bytes.Repeat([]byte("x"), tc.size))
				if tc.chunked {
					body = io.NopCloser(body)
				}
			}
			response, err := server.Client().Post(server.URL+"/native-stream", "application/octet-stream", body)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != tc.status {
				t.Fatalf("status=%d body=%q error=%v", response.StatusCode, data, err)
			}
			if tc.status == http.StatusOK {
				if producers.Load() != 1 || string(data) != "streamed\n" {
					t.Fatalf("producers=%d body=%q", producers.Load(), data)
				}
			} else {
				assertStreamBodyRejected(t, response, producers.Load())
			}
		})
	}
}

func TestServeContent_NativeStreamIncompleteBodyNeverStartsProducer(t *testing.T) {
	setupLiveModeServeContentTest(t)
	var producers atomic.Int32
	release := make(chan struct{})
	plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
		return shared.HandledResponse{Stream: func(ctx context.Context, _ io.Writer, _ func() error) error {
			producers.Add(1)
			select {
			case <-ctx.Done():
			case <-release:
			}
			return ctx.Err()
		}}
	}}
	config := nativeStreamRouteConfig()
	config["nocache"] = true
	installNativeStreamRoute(t, false, config, plugin)
	server := newStreamBodyTestServer(t, http.HandlerFunc(ServeContent), 100*time.Millisecond)
	t.Cleanup(func() { close(release) })
	for _, tc := range []struct {
		name    string
		request string
		status  int
	}{
		{"large-unread", "Content-Length: 1048576\r\n\r\n", http.StatusRequestTimeout},
		{"partial", "Content-Length: 10\r\n\r\nabc", http.StatusRequestTimeout},
		{"malformed-chunked", "Transfer-Encoding: chunked\r\n\r\ninvalid\r\n", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := dialStreamBodyTest(t, server)
			defer conn.Close()
			started := time.Now()
			_, err := fmt.Fprint(conn, "POST /native-stream HTTP/1.1\r\nHost: local\r\n"+tc.request)
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPost})
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				body, _ := io.ReadAll(response.Body)
				t.Fatalf("status=%d, want %d; headers=%v body=%q", response.StatusCode, tc.status, response.Header, body)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("preflight extended the 100ms server read timeout: %s", elapsed)
			}
			assertStreamBodyRejected(t, response, producers.Load())
		})
	}
}

func TestWriteStreamResponseBodyReadBoundWithoutServerReadTimeout(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-request=%t", cancelRequest), func(t *testing.T) {
			started := make(chan context.CancelFunc, 1)
			finished := make(chan error, 1)
			var producers atomic.Int32
			server := newStreamBodyTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx, cancel := context.WithCancel(r.Context())
				defer cancel()
				started <- cancel
				err := writeStreamResponse(w, r.WithContext(ctx), RenderContent{Handled: &shared.HandledResponse{
					Stream: func(context.Context, io.Writer, func() error) error {
						producers.Add(1)
						return nil
					},
				}})
				finished <- err
			}), 0)
			conn := dialStreamBodyTest(t, server)
			defer conn.Close()
			_, err := fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: local\r\nContent-Length: 1\r\n\r\n")
			if err != nil {
				t.Fatal(err)
			}
			cancel := <-started
			wait := streamRequestBodyReadTimeout + 2*time.Second
			wantErr := context.DeadlineExceeded
			if cancelRequest {
				cancel()
				wait = time.Second
				wantErr = context.Canceled
			}
			select {
			case err := <-finished:
				invalidErr := !errors.Is(err, wantErr)
				if cancelRequest {
					// Cancellation interrupts the pending socket read by forcing its
					// deadline. The result retains both causes.
					var timeout interface{ Timeout() bool }
					invalidErr = invalidErr || !errors.As(err, &timeout) || !timeout.Timeout()
				}
				if invalidErr || producers.Load() != 0 {
					t.Fatalf("error=%v producers=%d", err, producers.Load())
				}
			case <-time.After(wait):
				t.Fatal("request body preflight failed to stop")
			}
			response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPost})
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusRequestTimeout {
				t.Fatalf("status=%d, want %d", response.StatusCode, http.StatusRequestTimeout)
			}
			assertStreamBodyRejected(t, response, producers.Load())
		})
	}
}

func TestWriteStreamResponseBodyRequiresDeadlineControl(t *testing.T) {
	writer := &nativeStreamFaultWriter{nativeStreamPlainWriter: nativeStreamPlainWriter{header: make(http.Header)}}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	called := false
	err := writeStreamResponse(writer, request, RenderContent{Handled: &shared.HandledResponse{
		Stream: func(context.Context, io.Writer, func() error) error { called = true; return nil },
	}})
	if err == nil || called || writer.status != http.StatusBadRequest {
		t.Fatalf("error=%v producer=%t status=%d", err, called, writer.status)
	}
	remaining, _ := io.ReadAll(request.Body)
	if string(remaining) != "body" {
		t.Fatalf("unsupported writer consumed request body: %q", remaining)
	}
}

func TestWriteStreamResponseSlowReaderHonorsWriteTimeout(t *testing.T) {
	type result struct{ err, contextError error }
	finished := make(chan result, 1)
	server := newStreamBodyTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var observed result
		observed.err = writeStreamResponse(w, r, RenderContent{Handled: &shared.HandledResponse{
			Stream: func(ctx context.Context, output io.Writer, flush func() error) error {
				block := bytes.Repeat([]byte("x"), 1<<20)
				for i := 0; i < 128; i++ {
					_, err := output.Write(block)
					if err == nil {
						err = flush()
					}
					if err != nil {
						observed.contextError = ctx.Err()
						return err
					}
				}
				return errors.New("bounded slow-reader probe exhausted")
			},
		}})
		finished <- observed
	}), 0, 100*time.Millisecond)
	conn := dialStreamBodyTest(t, server)
	defer conn.Close()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(1024)
	}
	_, err := fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: local\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case observed := <-finished:
		var timeout interface{ Timeout() bool }
		if !errors.As(observed.err, &timeout) || !timeout.Timeout() || !errors.Is(observed.contextError, context.Canceled) {
			t.Fatalf("write=%v context=%v", observed.err, observed.contextError)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow-reader write did not stop at configured timeout")
	}
}

func newStreamBodyTestServer(t *testing.T, handler http.Handler, readTimeout time.Duration, writeTimeout ...time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.Config.ReadTimeout = readTimeout
	server.Config.WriteTimeout = streamRequestBodyReadTimeout + 2*time.Second
	if len(writeTimeout) > 0 {
		server.Config.WriteTimeout = writeTimeout[0]
	}
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func dialStreamBodyTest(t *testing.T, server *httptest.Server) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SetDeadline(time.Now().Add(streamRequestBodyReadTimeout + 3*time.Second)); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	return conn
}

func assertStreamBodyRejected(t *testing.T, response *http.Response, producers int32) {
	t.Helper()
	if producers != 0 || response.Header.Get("Cache-Control") != "no-store" || !response.Close {
		t.Fatalf("rejected body: producers=%d headers=%v close=%t", producers, response.Header, response.Close)
	}
	if response.Header.Get("X-Stream-Metadata") != "" || len(response.Cookies()) != 0 {
		t.Fatalf("preflight rejection published stream metadata: %v", response.Header)
	}
}
