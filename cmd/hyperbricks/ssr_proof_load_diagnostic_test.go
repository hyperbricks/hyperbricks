package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// This opt-in test server measures handler time separately from the Node
// client's HTTP duration. Its instrumentation is excluded from acceptance runs.
func TestSSRProofDiagnosticServer(t *testing.T) {
	if os.Getenv("HB_SSR_DIAGNOSTIC") != "1" {
		t.Skip("opt-in load diagnostic")
	}
	setupSSRProofPipelineBenchmark(t)
	const requests = 50000
	durations := make([]int64, requests)
	var completed atomic.Int64
	var before runtime.MemStats
	var started time.Time
	done := make(chan struct{})
	app := buildRuntimeHandler(rate.NewLimiter(1000000, 1000000))
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_diagnostic/start":
			runtime.ReadMemStats(&before)
			started = time.Now()
			w.WriteHeader(http.StatusNoContent)
			return
		case "/_diagnostic/report":
			if completed.Load() != requests {
				http.Error(w, "requests still running", http.StatusConflict)
				return
			}
			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			ordered := slices.Clone(durations)
			slices.Sort(ordered)
			percentile := func(p int) float64 {
				return float64(ordered[(len(ordered)*p+99)/100-1]) / float64(time.Millisecond)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"requests":     completed.Load(),
				"handlerP50Ms": percentile(50), "handlerP95Ms": percentile(95), "handlerP99Ms": percentile(99),
				"allocatedBytesPerRequest": (after.TotalAlloc - before.TotalAlloc) / requests,
				"allocationsPerRequest":    (after.Mallocs - before.Mallocs) / requests,
				"gcCycles":                 after.NumGC - before.NumGC,
				"gcPauseTotalMs":           float64(after.PauseTotalNs-before.PauseTotalNs) / float64(time.Millisecond),
				"elapsedMs":                float64(time.Since(started)) / float64(time.Millisecond),
			})
			return
		case "/_diagnostic/stop":
			w.WriteHeader(http.StatusNoContent)
			close(done)
			return
		}
		index, err := strconv.Atoi(strings.TrimPrefix(r.URL.RawQuery, "rid=rid-"))
		measured := err == nil && index >= 0 && index < requests
		start := time.Now()
		app.ServeHTTP(w, r)
		if measured {
			durations[index] = time.Since(start).Nanoseconds()
			completed.Add(1)
		}
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 65536}
	defer server.Close()
	failures := make(chan error, 1)
	go func() { failures <- server.Serve(listener) }()
	fmt.Println("SSR_DIAGNOSTIC_URL=http://" + listener.Addr().String())
	select {
	case <-done:
	case err := <-failures:
		t.Fatal(err)
	case <-time.After(2 * time.Minute):
		t.Fatal("load diagnostic timed out")
	}
}
