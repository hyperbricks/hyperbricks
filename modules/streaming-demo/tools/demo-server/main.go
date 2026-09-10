// The demo server serves one simulated event stream and proxies the module UI
// to a separately started HyperBricks server. It never runs a build.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"
)

const streamTimeout = 4 * time.Second

func main() {
	port := flag.Int("port", 18110, "port on 127.0.0.1 for the demo UI and event stream")
	upstreamAddress := flag.String("upstream", "http://127.0.0.1:18111", "URL of the separately started HyperBricks module")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		log.Fatal("port must be between 1 and 65535")
	}
	upstream, err := url.Parse(*upstreamAddress)
	if err != nil || upstream.Host == "" || (upstream.Scheme != "http" && upstream.Scheme != "https") {
		log.Fatal("upstream must be an absolute HTTP or HTTPS URL")
	}

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	server := &http.Server{
		Addr:              address,
		Handler:           newDemoHandler(upstream, log.Default(), time.Second),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	log.Printf("Demo listening on http://%s; HyperBricks upstream %s", address, upstream)
	log.Fatal(server.ListenAndServe())
}

func newDemoHandler(upstream *url.URL, logger *log.Logger, stepDelay time.Duration) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ErrorLog = logger
	mux := http.NewServeMux()
	mux.Handle("/demo/events", &eventHandler{logger: logger, stepDelay: stepDelay})
	mux.HandleFunc("/demo/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintln(w, `{"ok":true}`)
	})
	mux.Handle("/", proxy)
	return mux
}

type eventHandler struct {
	logger    *log.Logger
	stepDelay time.Duration
	requests  atomic.Uint64
}

func (h *eventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "Streaming is unavailable", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), streamTimeout)
	defer cancel()
	requestID := h.requests.Add(1)
	h.logger.Printf("stream=%d START", requestID)
	complete := false
	defer func() {
		if !complete {
			h.logger.Printf("stream=%d CANCELLED", requestID)
		}
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(w)
	steps := []struct {
		label   string
		percent int
		message string
	}{
		{"Started", 0, "The simulated build has started."},
		{"Preparing", 25, "Preparing the simulated build."},
		{"Processing", 65, "Processing the simulated build."},
		{"Finished", 100, "Demo complete. No project was built."},
	}
	for index, step := range steps {
		if index > 0 {
			if err := waitForStep(ctx, h.stepDelay); err != nil {
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		// One line of HTML and a blank line delimit one unnamed SSE data event.
		// Each event replaces the contents of the page's #stream-panel.
		_, err := fmt.Fprintf(w, "data: <section class=\"stream-step\" data-step=\"%d\"><p class=\"step-label\">Step %d of 4 &middot; Simulated build</p><h2>%s</h2><progress max=\"100\" value=\"%d\" aria-label=\"Simulated build progress\">%d%%</progress><p class=\"step-progress\">%d%%</p><p>%s</p></section>\n\n", index+1, index+1, step.label, step.percent, step.percent, step.percent, step.message)
		if err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
		h.logger.Printf("stream=%d STEP %d", requestID, index+1)
	}
	complete = true
	h.logger.Printf("stream=%d COMPLETE", requestID)
}

func waitForStep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
