package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type trialSpec struct {
	Sequence    int    `json:"sequence"`
	Repeat      int    `json:"repeat"`
	Backend     string `json:"backend"`
	BodyBytes   int    `json:"body_bytes"`
	Concurrency int    `json:"concurrency"`
}

type trial struct {
	trialSpec
	Valid             bool           `json:"valid"`
	Error             string         `json:"error,omitempty"`
	WarmupRequests    int            `json:"warmup_requests"`
	WarmupElapsedNS   int64          `json:"warmup_elapsed_ns"`
	Requested         int            `json:"requested"`
	Verified          int            `json:"verified"`
	Failures          []failure      `json:"failures,omitempty"`
	FailureCount      int            `json:"failure_count"`
	ElapsedNS         int64          `json:"elapsed_ns"`
	RequestsPerSecond float64        `json:"requests_per_second"`
	MiBPerSecond      float64        `json:"body_mib_per_second"`
	Latency           latencySummary `json:"latency"`
	LatencyNS         []int64        `json:"latency_ns"`
	Checks            responseChecks `json:"checks"`
	NewConnections    int64          `json:"new_connections"`
	ReusedConnections int64          `json:"reused_connections"`
}

type responseChecks struct {
	BodySHA256              string `json:"expected_body_sha256"`
	VerifiedBytes           int64  `json:"verified_measured_body_bytes"`
	EveryBodyCompared       bool   `json:"every_body_compared"`
	EveryBodyHashed         bool   `json:"every_body_sha256_checked"`
	Status                  int    `json:"expected_status"`
	ContentType             string `json:"expected_media_type"`
	CachedMetadataStable    bool   `json:"cached_metadata_stable"`
	RenderErrorCountChecked bool   `json:"render_error_count_zero_checked"`
}

type failure struct {
	Request int    `json:"request_index"`
	Error   string `json:"error"`
}

type latencySummary struct {
	Count   int     `json:"sample_count"`
	Average float64 `json:"average_ns"`
	MinNS   int64   `json:"min_ns"`
	P50NS   int64   `json:"p50_ns"`
	P95NS   int64   `json:"p95_ns"`
	P99NS   int64   `json:"p99_ns"`
	MaxNS   int64   `json:"max_ns"`
}

type cacheMetadata struct {
	ETag, RenderedAt, ExpiresAt string
}

func trialPlan(cfg options) []trialSpec {
	type caseSpec struct{ size, concurrency int }
	var cases []caseSpec
	for _, size := range cfg.Sizes {
		for _, concurrency := range cfg.Concurrency {
			cases = append(cases, caseSpec{size, concurrency})
		}
	}
	var plan []trialSpec
	for repeat := 0; repeat < cfg.Repeats; repeat++ {
		for position := range cases {
			index := (position + repeat) % len(cases)
			current := cases[index]
			for offset := range backends {
				backend := backends[(offset+repeat+index)%len(backends)]
				plan = append(plan, trialSpec{Sequence: len(plan) + 1, Repeat: repeat + 1, Backend: backend, BodyBytes: current.size, Concurrency: current.concurrency})
			}
		}
	}
	return plan
}

func runTrial(ctx context.Context, address string, cfg options, spec trialSpec) trial {
	result := trial{trialSpec: spec, Requested: cfg.Requests, WarmupRequests: cfg.Warmup}
	expected := bytes.Repeat([]byte(bodyPattern), spec.BodyBytes/len(bodyPattern))
	digest := sha256.Sum256(expected)
	result.Checks = responseChecks{BodySHA256: hex.EncodeToString(digest[:]), Status: http.StatusOK, ContentType: "text/plain"}
	transport := &http.Transport{
		Proxy: nil, DialContext: (&net.Dialer{Timeout: cfg.RequestTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns: spec.Concurrency, MaxIdleConnsPerHost: spec.Concurrency, MaxConnsPerHost: spec.Concurrency,
		IdleConnTimeout: time.Minute, DisableCompression: true, ForceAttemptHTTP2: false,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: cfg.RequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	url := address + "/bench/" + spec.Backend + "?blocks=" + strconv.Itoa(spec.BodyBytes/len(bodyPattern))
	warmStarted := time.Now()
	// Seed one complete response before concurrent warmup. Otherwise the very
	// first warmup wave could contain legitimate concurrent cache misses with
	// different rendering timestamps instead of a stable warm-cache contract.
	buffer := make([]byte, len(expected)+1)
	_, metadata, err := verifiedRequest(ctx, client, url, spec.Backend, expected, digest, buffer, nil, nil)
	if err != nil {
		result.Error = "warmup seed: " + err.Error()
		return result
	}
	if cfg.Warmup > 1 {
		warm := runBatch(ctx, client, url, spec, cfg.Warmup-1, expected, digest, &metadata)
		if warm.failureCount > 0 || warm.verified != cfg.Warmup-1 {
			result.Error = "warmup did not verify every response"
			result.Failures, result.FailureCount = warm.failures, warm.failureCount
			return result
		}
	}
	result.WarmupElapsedNS = time.Since(warmStarted).Nanoseconds()
	measured := runBatch(ctx, client, url, spec, cfg.Requests, expected, digest, &metadata)
	result.ElapsedNS, result.Verified = measured.elapsedNS, measured.verified
	result.Failures, result.FailureCount = measured.failures, measured.failureCount
	result.LatencyNS = measured.latencies
	result.NewConnections, result.ReusedConnections = measured.newConnections, measured.reusedConnections
	if measured.failureCount != 0 || measured.verified != cfg.Requests {
		result.Error = fmt.Sprintf("verified %d of %d responses, %d failures", measured.verified, cfg.Requests, measured.failureCount)
		if len(measured.failures) > 0 {
			result.Error += ": " + measured.failures[0].Error
		}
		return result
	}
	result.Valid = true
	result.RequestsPerSecond = float64(measured.verified) / (float64(measured.elapsedNS) / 1e9)
	result.MiBPerSecond = result.RequestsPerSecond * float64(spec.BodyBytes) / (1024 * 1024)
	result.Latency = summarizeLatency(measured.latencies)
	result.Checks.VerifiedBytes = int64(measured.verified) * int64(spec.BodyBytes)
	result.Checks.EveryBodyCompared, result.Checks.EveryBodyHashed = true, true
	result.Checks.CachedMetadataStable = spec.Backend != "fresh"
	result.Checks.RenderErrorCountChecked = true
	return result
}

type batch struct {
	verified, failureCount            int
	latencies                         []int64
	failures                          []failure
	elapsedNS                         int64
	newConnections, reusedConnections int64
}

type connectionCounters struct{ fresh, reused atomic.Int64 }

func runBatch(parent context.Context, client *http.Client, url string, spec trialSpec, count int, expected []byte, digest [32]byte, metadata *cacheMetadata) batch {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	latencies := make([]int64, count)
	var next, verified atomic.Int64
	var connections connectionCounters
	var workers sync.WaitGroup
	var ready sync.WaitGroup
	var failuresMutex sync.Mutex
	var failures []failure
	failureCount := 0
	start := make(chan struct{})
	for worker := 0; worker < spec.Concurrency; worker++ {
		workers.Add(1)
		ready.Add(1)
		go func() {
			defer workers.Done()
			buffer := make([]byte, len(expected)+1)
			ready.Done()
			<-start
			for ctx.Err() == nil {
				index := int(next.Add(1) - 1)
				if index >= count {
					return
				}
				elapsed, _, err := verifiedRequest(ctx, client, url, spec.Backend, expected, digest, buffer, metadata, &connections)
				if err != nil {
					failuresMutex.Lock()
					failureCount++
					if len(failures) < 20 {
						failures = append(failures, failure{index, err.Error()})
					}
					failuresMutex.Unlock()
					cancel()
					return
				}
				latencies[index] = elapsed
				verified.Add(1)
			}
		}()
	}
	ready.Wait()
	started := time.Now()
	close(start)
	workers.Wait()
	elapsed := time.Since(started).Nanoseconds()
	// Failed runs keep only actual successful response samples. Their result
	// is invalid and receives no aggregate performance comparison.
	if int(verified.Load()) != count {
		filtered := make([]int64, 0, verified.Load())
		for _, latency := range latencies {
			if latency != 0 {
				filtered = append(filtered, latency)
			}
		}
		latencies = filtered
	}
	return batch{verified: int(verified.Load()), failureCount: failureCount, latencies: latencies, failures: failures,
		elapsedNS: elapsed, newConnections: connections.fresh.Load(), reusedConnections: connections.reused.Load()}
}

func verifiedRequest(ctx context.Context, client *http.Client, url, backend string, expected []byte, digest [32]byte, buffer []byte, metadata *cacheMetadata, connections *connectionCounters) (int64, cacheMetadata, error) {
	if connections != nil {
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				connections.reused.Add(1)
			} else {
				connections.fresh.Add(1)
			}
		}})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, cacheMetadata{}, err
	}
	request.Header.Set("Accept-Encoding", "identity")
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return time.Since(started).Nanoseconds(), cacheMetadata{}, err
	}
	n, readErr := io.ReadFull(response.Body, buffer)
	closeErr := response.Body.Close()
	elapsed := time.Since(started).Nanoseconds()
	current := cacheMetadata{response.Header.Get("ETag"), response.Header.Get("X-Hyperbricks-Rendered-At"), response.Header.Get("X-Hyperbricks-Cache-Expires-At")}
	if readErr != io.ErrUnexpectedEOF || n != len(expected) || closeErr != nil {
		return elapsed, current, fmt.Errorf("body length/read: got %d bytes, want %d, read=%v close=%v", n, len(expected), readErr, closeErr)
	}
	if response.StatusCode != http.StatusOK || response.ContentLength != int64(len(expected)) {
		return elapsed, current, fmt.Errorf("status/length: got %d/%d, want 200/%d", response.StatusCode, response.ContentLength, len(expected))
	}
	contentType := response.Header.Get("Content-Type")
	if contentType != "text/plain" && !strings.HasPrefix(contentType, "text/plain;") {
		return elapsed, current, fmt.Errorf("unexpected Content-Type %q", contentType)
	}
	if response.Proto != "HTTP/1.1" || response.Close || response.Header.Get("Content-Encoding") != "" {
		return elapsed, current, fmt.Errorf("expected uncompressed HTTP/1.1 keep-alive; proto=%s close=%t encoding=%q", response.Proto, response.Close, response.Header.Get("Content-Encoding"))
	}
	if response.Header.Get("X-Hyperbricks-Render-Error-Count") != "0" {
		return elapsed, current, fmt.Errorf("render error count is %q", response.Header.Get("X-Hyperbricks-Render-Error-Count"))
	}
	if !bytes.Equal(buffer[:n], expected) || sha256.Sum256(buffer[:n]) != digest {
		return elapsed, current, errorsPayloadMismatch
	}
	if backend == "fresh" {
		if current != (cacheMetadata{}) {
			return elapsed, current, fmt.Errorf("fresh route unexpectedly returned cache metadata: %+v", current)
		}
	} else if current.ETag == "" || current.RenderedAt == "" || current.ExpiresAt == "" || metadata != nil && current != *metadata {
		return elapsed, current, fmt.Errorf("cached route omitted or changed its warmup metadata: %+v", current)
	}
	return elapsed, current, nil
}

var errorsPayloadMismatch = fmt.Errorf("response bytes/SHA-256 differ from the fixture contract")

func summarizeLatency(samples []int64) latencySummary {
	if len(samples) == 0 {
		return latencySummary{}
	}
	ordered := append([]int64(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var sum float64
	for _, sample := range ordered {
		sum += float64(sample)
	}
	percentile := func(p float64) int64 { return ordered[int(math.Ceil(p*float64(len(ordered))))-1] }
	return latencySummary{Count: len(samples), Average: sum / float64(len(samples)), MinNS: ordered[0], P50NS: percentile(.50), P95NS: percentile(.95), P99NS: percentile(.99), MaxNS: ordered[len(ordered)-1]}
}
