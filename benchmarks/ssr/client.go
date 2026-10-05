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
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const requestIDMarker = "__REQUEST_ID__"
const canonicalTemplate = `<!DOCTYPE html><html><head><title>SSR Proof Landing</title></head><body><main id="landing" data-request-id="__REQUEST_ID__"><section class="hero"><h1>SSR Proof Landing</h1><p>Nested server-rendered output with request isolation.</p><a href="/start">Start</a></section><section class="features"><div class="feature-list"><article class="feature-card"><h2>Native rendering</h2><p>HTML is generated on the server.</p></article><article class="feature-card"><h2>Nested composition</h2><p>Components contain components across three levels.</p></article><article class="feature-card"><h2>Request state</h2><p>Request __REQUEST_ID__ stays isolated.</p></article></div></section><section class="proof"><dl class="metrics"><div class="metric"><dt>Request</dt><dd>__REQUEST_ID__</dd></div><div class="metric"><dt>Mode</dt><dd>SSR</dd></div></dl></section></main></body></html>`

var canonicalParts = strings.Split(canonicalTemplate, requestIDMarker)
var escapeHTML = strings.NewReplacer("&", "&amp;", "+", "&#43;", "'", "&#39;", `"`, "&#34;", "<", "&lt;", ">", "&gt;")

type workload struct {
	Name                    string `json:"name"`
	Version                 int    `json:"version"`
	Profile                 string `json:"profile"`
	CanonicalTemplate       string `json:"canonical_template"`
	CanonicalTemplateSHA256 string `json:"canonical_template_sha256"`
	RequestIDFormat         string `json:"request_id_format"`
	RequestIDBytes          int    `json:"request_id_bytes"`
	RequestIDOccurrences    int    `json:"request_id_occurrences"`
	BodyBytes               int    `json:"body_bytes"`
}

func workloadContract() workload {
	digest := sha256.Sum256([]byte(canonicalTemplate))
	return workload{Name: "nested-request-isolated-html", Version: 1, Profile: "package.hyperbricks.yaml", CanonicalTemplate: canonicalTemplate, CanonicalTemplateSHA256: hex.EncodeToString(digest[:]), RequestIDFormat: "ssr-<16lowerhex>-<4digittrial>-<w|m>-<10digitindex>", RequestIDBytes: 38, RequestIDOccurrences: 3, BodyBytes: len(canonicalTemplate) + 3*(38-len(requestIDMarker))}
}

func canonicalBody(buffer []byte, requestID string) []byte {
	escaped := escapeHTML.Replace(requestID)
	buffer = append(buffer[:0], canonicalParts[0]...)
	for _, part := range canonicalParts[1:] {
		buffer = append(buffer, escaped...)
		buffer = append(buffer, part...)
	}
	return buffer
}

func requestID(prefix string, index int) string {
	var decimal [10]byte
	for position := len(decimal) - 1; position >= 0; position-- {
		decimal[position] = byte(index%10) + '0'
		index /= 10
	}
	return prefix + string(decimal[:])
}

type trialSpec struct {
	Sequence    int `json:"sequence"`
	Repeat      int `json:"repeat"`
	Concurrency int `json:"concurrency"`
}

type trial struct {
	trialSpec
	BodyBytes         int            `json:"body_bytes"`
	MeasuredPrefix    string         `json:"measured_request_id_prefix"`
	WarmupPrefix      string         `json:"warmup_request_id_prefix"`
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
	VerifiedBytes           int64  `json:"verified_measured_body_bytes"`
	EveryBodyCompared       bool   `json:"every_body_compared"`
	EveryBodyHashed         bool   `json:"every_body_sha256_checked"`
	UniqueRequestIDs        bool   `json:"unique_request_ids"`
	RequestIDOccurrences    int    `json:"request_id_occurrences"`
	RequestIDHeaderPresent  bool   `json:"request_id_header_present"`
	Status                  int    `json:"expected_status"`
	ContentType             string `json:"expected_media_type"`
	CacheControl            string `json:"cache_control"`
	CacheMetadataAbsent     bool   `json:"cache_metadata_absent"`
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

func trialPlan(cfg options) []trialSpec {
	var result []trialSpec
	for repeat := 0; repeat < cfg.Repeats; repeat++ {
		for position := range cfg.Concurrency {
			result = append(result, trialSpec{Sequence: len(result) + 1, Repeat: repeat + 1, Concurrency: cfg.Concurrency[(position+repeat)%len(cfg.Concurrency)]})
		}
	}
	return result
}

func newClient(cfg options, workers int) (*http.Client, *http.Transport) {
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: cfg.RequestTimeout, KeepAlive: 30 * time.Second}).DialContext, MaxIdleConns: workers, MaxIdleConnsPerHost: workers, MaxConnsPerHost: workers, IdleConnTimeout: time.Minute, DisableCompression: true, ForceAttemptHTTP2: false}
	return &http.Client{Transport: transport, Timeout: cfg.RequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, transport
}

func runPreflight(ctx context.Context, address string, cfg options) preflightRecord {
	client, transport := newClient(cfg, 1)
	defer transport.CloseIdleConnections()
	result := preflightRecord{}
	for _, id := range []string{"preflight-safe-123", `preflight-<&>"'+-雪`, "preflight-isolation-other", "preflight-safe-123"} {
		expected := canonicalBody(nil, id)
		_, err := verifiedRequest(ctx, client, address+"/?rid="+url.QueryEscape(id), expected, make([]byte, len(expected)+1), nil)
		result.Requests++
		if err != nil {
			result.Error = err.Error()
			return result
		}
	}
	result.Valid = true
	return result
}

func runTrial(ctx context.Context, address string, cfg options, spec trialSpec, nonce string) trial {
	result := trial{trialSpec: spec, BodyBytes: workloadContract().BodyBytes, MeasuredPrefix: trialPrefix(nonce, spec.Sequence, 'm'), WarmupPrefix: trialPrefix(nonce, spec.Sequence, 'w'), Requested: cfg.Requests, WarmupRequests: cfg.Warmup}
	client, transport := newClient(cfg, spec.Concurrency)
	defer transport.CloseIdleConnections()
	warm := runBatch(ctx, client, address, spec.Concurrency, cfg.Warmup, result.WarmupPrefix, result.BodyBytes)
	result.WarmupElapsedNS = warm.elapsedNS
	if warm.failureCount != 0 || warm.verified != cfg.Warmup {
		result.Error = "warmup verification failed"
		result.Failures, result.FailureCount = warm.failures, warm.failureCount
		if len(warm.failures) > 0 {
			result.Error += ": " + warm.failures[0].Error
		}
		return result
	}
	measured := runBatch(ctx, client, address, spec.Concurrency, cfg.Requests, result.MeasuredPrefix, result.BodyBytes)
	result.ElapsedNS, result.Verified, result.LatencyNS = measured.elapsedNS, measured.verified, measured.latencies
	result.Failures, result.FailureCount = measured.failures, measured.failureCount
	result.NewConnections, result.ReusedConnections = measured.newConnections, measured.reusedConnections
	if measured.failureCount != 0 || measured.verified != cfg.Requests {
		result.Error = fmt.Sprintf("verified %d of %d, %d failures", measured.verified, cfg.Requests, measured.failureCount)
		if len(measured.failures) > 0 {
			result.Error += ": " + measured.failures[0].Error
		}
		return result
	}
	result.Valid = true
	result.RequestsPerSecond = float64(cfg.Requests) * 1e9 / float64(measured.elapsedNS)
	result.MiBPerSecond = result.RequestsPerSecond * float64(result.BodyBytes) / 1048576
	result.Latency = summarizeLatency(measured.latencies)
	result.Checks = responseChecks{VerifiedBytes: int64(cfg.Requests) * int64(result.BodyBytes), EveryBodyCompared: true, EveryBodyHashed: true, UniqueRequestIDs: true, RequestIDOccurrences: 3, RequestIDHeaderPresent: true, Status: 200, ContentType: "text/html", CacheControl: "no-store", CacheMetadataAbsent: true, RenderErrorCountChecked: true}
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

func runBatch(parent context.Context, client *http.Client, address string, concurrency, count int, prefix string, bodyBytes int) batch {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	latencies := make([]int64, count)
	var next, verified atomic.Int64
	var connections connectionCounters
	var workers, ready sync.WaitGroup
	var mutex sync.Mutex
	var failures []failure
	failureCount := 0
	start := make(chan struct{})
	for worker := 0; worker < concurrency; worker++ {
		workers.Add(1)
		ready.Add(1)
		go func() {
			defer workers.Done()
			buffer, expected := make([]byte, bodyBytes+1), make([]byte, 0, bodyBytes)
			ready.Done()
			<-start
			for ctx.Err() == nil {
				index := int(next.Add(1) - 1)
				if index >= count {
					return
				}
				id := requestID(prefix, index)
				expected = canonicalBody(expected, id)
				elapsed, err := verifiedRequest(ctx, client, address+"/?rid="+id, expected, buffer, &connections)
				if err != nil {
					mutex.Lock()
					failureCount++
					if len(failures) < 20 {
						failures = append(failures, failure{Request: index, Error: err.Error()})
					}
					mutex.Unlock()
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
	if int(verified.Load()) != count {
		filtered := make([]int64, 0, verified.Load())
		for _, value := range latencies {
			if value > 0 {
				filtered = append(filtered, value)
			}
		}
		latencies = filtered
	}
	return batch{verified: int(verified.Load()), failureCount: failureCount, latencies: latencies, failures: failures, elapsedNS: elapsed, newConnections: connections.fresh.Load(), reusedConnections: connections.reused.Load()}
}

func verifiedRequest(ctx context.Context, client *http.Client, address string, expected, buffer []byte, connections *connectionCounters) (int64, error) {
	if connections != nil {
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				connections.reused.Add(1)
			} else {
				connections.fresh.Add(1)
			}
		}})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept-Encoding", "identity")
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return time.Since(started).Nanoseconds(), err
	}
	n, readErr := io.ReadFull(response.Body, buffer)
	closeErr := response.Body.Close()
	elapsed := time.Since(started).Nanoseconds()
	if n != len(expected) || readErr != io.ErrUnexpectedEOF || closeErr != nil {
		return elapsed, fmt.Errorf("body length/read: got %d want %d read=%v close=%v", n, len(expected), readErr, closeErr)
	}
	if response.StatusCode != 200 || response.ContentLength != int64(len(expected)) {
		return elapsed, fmt.Errorf("status/length: got %d/%d", response.StatusCode, response.ContentLength)
	}
	if mediaType := response.Header.Get("Content-Type"); mediaType != "text/html" && !strings.HasPrefix(mediaType, "text/html;") {
		return elapsed, fmt.Errorf("unexpected Content-Type %q", mediaType)
	}
	if response.Proto != "HTTP/1.1" || response.Close || response.Header.Get("Content-Encoding") != "" {
		return elapsed, fmt.Errorf("expected uncompressed HTTP/1.1 keep-alive")
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("ETag") != "" || response.Header.Get("X-Hyperbricks-Rendered-At") != "" || response.Header.Get("X-Hyperbricks-Cache-Expires-At") != "" {
		return elapsed, fmt.Errorf("response cache headers violate uncached SSR contract")
	}
	if response.Header.Get("X-Hyperbricks-Render-Error-Count") != "0" || response.Header.Get("X-Hyperbricks-Request-ID") == "" {
		return elapsed, fmt.Errorf("render diagnostics/request ID headers missing or error count nonzero")
	}
	if !bytes.Equal(buffer[:n], expected) || sha256.Sum256(buffer[:n]) != sha256.Sum256(expected) {
		return elapsed, fmt.Errorf("canonical HTML/request isolation failed for %s", request.URL.RawQuery)
	}
	return elapsed, nil
}

func summarizeLatency(samples []int64) latencySummary {
	if len(samples) == 0 {
		return latencySummary{}
	}
	ordered := append([]int64(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var sum float64
	for _, value := range ordered {
		sum += float64(value)
	}
	percentile := func(p float64) int64 { return ordered[int(math.Ceil(p*float64(len(ordered))))-1] }
	return latencySummary{Count: len(samples), Average: sum / float64(len(samples)), MinNS: ordered[0], MaxNS: ordered[len(ordered)-1], P50NS: percentile(.5), P95NS: percentile(.95), P99NS: percentile(.99)}
}

func trialPrefix(nonce string, sequence int, phase byte) string {
	return "ssr-" + nonce + "-" + fmt.Sprintf("%04d", sequence) + "-" + string(phase) + "-"
}
