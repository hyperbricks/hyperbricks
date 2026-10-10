package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const packageYAML = `hyperbricks:
  mode: live
  metadata:
    module: multichannel-bench
    moduleversion: 1.0.0
  development:
    watch: false
    reload: false
    frontend_errors: false
    dashboard: {enabled: false}
  server:
    gomaxprocs: 4
    beautify: false
    read_timeout: 30s
    write_timeout: 30s
    idle_timeout: 30s
  rate_limit: {enabled: false}
  plugins: {enabled: []}
  directories:
    hyperbricks: {path: {base: module, path: hyperbricks}}
    templates: {path: {base: module, path: templates}}
    resources: {path: {base: module, path: resources}}
    static: {path: {base: module, path: static}}
    render: {path: {base: module, path: rendered}}
    plugins: {path: {base: module, path: plugins}}
`

// Port-independent source is fingerprinted before replacing UPSTREAM.
func fixture() string {
	var b strings.Builder
	for _, channel := range []string{"catalog", "stock", "shipping"} {
		fmt.Fprintf(&b, `%s_api:
  - type: api_render
  - method: GET
  - endpoint: UPSTREAM/%s
  - querykeys: [rid, fail]
  - response_status:
      required: true
      map: {"503": 503}
  - inline: '%s={{.Status}}:{{.Data.rid}}:{{.Data.payload}};'

%s_fragment:
  - type: api_fragment_render
  - route: %s
  - method: GET
  - endpoint: UPSTREAM/%s
  - querykeys: [rid, fail]
  - response:
      headers:
        Content-Type: text/plain; charset=utf-8
        Cache-Control: no-store
  - response_status:
      required: true
      map: {"503": 503}
  - inline: '%s={{.Status}}:{{.Data.rid}}:{{.Data.payload}};'

`, channel, channel, channel, channel, channel, channel, channel)
	}
	b.WriteString(`page:
  - type: hypermedia
  - route: page
  - nocache: true
  - content_type: text/plain; charset=utf-8
  - response:
      headers:
        Cache-Control: no-store
  - template:
      inline: '{{.catalog}}{{.stock}}{{.shipping}}'
      values:
        catalog:
          - inherit: catalog_api
          - response_status:
              required: true
              map: {"503": 503}
        stock:
          - inherit: stock_api
          - response_status:
              required: true
              map: {"503": 503}
        shipping:
          - inherit: shipping_api
          - response_status:
              required: true
              map: {"503": 503}
`)
	return b.String()
}

func payload(channel string) string {
	size := map[string]int{"catalog": 32768, "stock": 4096, "shipping": 1024}[channel]
	return strings.Repeat("x", size)
}

func upstream(w http.ResponseWriter, r *http.Request) {
	// A small deterministic service delay is part of the workload, not a CPU-only test.
	select {
	case <-time.After(time.Millisecond):
	case <-r.Context().Done():
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/stock" && r.URL.Query().Get("fail") == "1" {
		w.WriteHeader(503)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"rid": r.URL.Query().Get("rid"), "payload": payload(strings.TrimPrefix(r.URL.Path, "/"))})
}

func expected(route, rid string, failure bool) (int, string) {
	status := 200
	channels := []string{route}
	if route == "page" {
		channels = []string{"catalog", "stock", "shipping"}
	}
	var body strings.Builder
	for _, ch := range channels {
		s := 200
		if failure && ch == "stock" {
			s = 503
			status = 503
		}
		fmt.Fprintf(&body, "%s=%d:%s:%s;", ch, s, rid, payload(ch))
	}
	return status, body.String()
}
