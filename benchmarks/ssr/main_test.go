package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestRequestIDsAreFixedWidthAndDisjoint(t *testing.T) {
	seen := make(map[string]bool)
	for _, nonce := range []string{"0123456789abcdef", "fedcba9876543210"} {
		for _, sequence := range []int{1, 16, 9999} {
			for _, phase := range []byte{'w', 'm'} {
				for _, index := range []int{0, 1, 1234567890, 9999999999} {
					id := requestID(trialPrefix(nonce, sequence, phase), index)
					if len(id) != 38 || seen[id] {
						t.Fatalf("ID is not fixed-width and unique: %q", id)
					}
					seen[id] = true
				}
			}
		}
	}
}

func TestCanonicalHTMLContractAndEscaping(t *testing.T) {
	contract := workloadContract()
	digest := sha256.Sum256([]byte(contract.CanonicalTemplate))
	if contract.CanonicalTemplateSHA256 != hex.EncodeToString(digest[:]) || strings.Count(contract.CanonicalTemplate, requestIDMarker) != 3 {
		t.Fatal("canonical template hash/positions invalid")
	}
	id := requestID(trialPrefix("0123456789abcdef", 1, 'm'), 0)
	body := string(canonicalBody(nil, id))
	if len(body) != contract.BodyBytes || strings.Count(body, id) != 3 || !strings.HasPrefix(body, "<!DOCTYPE html><html>") || !strings.HasSuffix(body, "</main></body></html>") {
		t.Fatal("canonical body layout/size invalid")
	}
	special := string(canonicalBody(nil, `<&>"'+雪`))
	if strings.Count(special, "&lt;&amp;&gt;&#34;&#39;&#43;雪") != 3 {
		t.Fatalf("escaping-sensitive request ID was not encoded correctly: %s", special)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func successfulResponse(body []byte) *http.Response {
	return &http.Response{StatusCode: 200, ContentLength: int64(len(body)), Proto: "HTTP/1.1", Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{
		"Content-Type": []string{"text/html; charset=utf-8"}, "Cache-Control": []string{"no-store"}, "X-Hyperbricks-Render-Error-Count": []string{"0"}, "X-Hyperbricks-Request-Id": []string{"server-generated-id"},
	}}
}

func TestResponseValidationRejectsLeaksCachingAndBrokenResponses(t *testing.T) {
	expected := canonicalBody(nil, "correct-request")
	for _, mode := range []string{"valid", "other-request", "single-stale-position", "short", "long", "status", "length", "content-type", "compression", "cache-control", "etag", "cache-timestamp", "render-error", "missing-request-header", "closed", "network"} {
		t.Run(mode, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Header.Get("Accept-Encoding") != "identity" || request.Header.Get("If-None-Match") != "" {
					t.Fatal("must request uncompressed full response")
				}
				body := append([]byte(nil), expected...)
				if mode == "other-request" {
					body = canonicalBody(nil, "wrong---request")
				}
				if mode == "single-stale-position" {
					body = []byte(strings.Replace(string(body), "correct-request", "wrong---request", 1))
				}
				if mode == "short" {
					body = body[:len(body)-1]
				}
				if mode == "long" {
					body = append(body, 'x')
				}
				response := successfulResponse(body)
				switch mode {
				case "status":
					response.StatusCode = 304
				case "length":
					response.ContentLength--
				case "content-type":
					response.Header.Set("Content-Type", "text/plain")
				case "compression":
					response.Header.Set("Content-Encoding", "gzip")
				case "cache-control":
					response.Header.Set("Cache-Control", "public")
				case "etag":
					response.Header.Set("ETag", "cached")
				case "cache-timestamp":
					response.Header.Set("X-Hyperbricks-Rendered-At", "now")
				case "render-error":
					response.Header.Set("X-Hyperbricks-Render-Error-Count", "1")
				case "missing-request-header":
					response.Header.Del("X-Hyperbricks-Request-ID")
				case "closed":
					response.Close = true
				case "network":
					return nil, errors.New("connection failure")
				}
				return response, nil
			})}
			_, err := verifiedRequest(context.Background(), client, "http://fixture.invalid/?rid=correct-request", expected, make([]byte, len(expected)+1), nil)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("verification error=%v", err)
			}
		})
	}
}

func TestConcurrentBatchVerifiesEveryDistinctRequest(t *testing.T) {
	seen := make(map[string]bool)
	var mutex sync.Mutex
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		id := request.URL.Query().Get("rid")
		mutex.Lock()
		duplicate := seen[id]
		seen[id] = true
		mutex.Unlock()
		if duplicate {
			return nil, errors.New("request ID reused")
		}
		return successfulResponse(canonicalBody(nil, id)), nil
	})}
	for _, phase := range []byte{'w', 'm'} {
		measured := runBatch(context.Background(), client, "http://fixture.invalid", 16, 200, trialPrefix("0123456789abcdef", 1, phase), workloadContract().BodyBytes)
		if measured.verified != 200 || measured.failureCount != 0 || len(measured.latencies) != 200 {
			t.Fatalf("batch failed: %+v", measured)
		}
	}
	if len(seen) != 400 {
		t.Fatalf("got %d distinct requests", len(seen))
	}
}

func TestOptionsRejectInvalidCountsAndIDOverflow(t *testing.T) {
	for _, args := range [][]string{{}, {"-binary", "hb", "-requests", "0"}, {"-binary", "hb", "-warmup", "0"}, {"-binary", "hb", "-requests", "10000000000"}, {"-binary", "hb", "-repeats", "10000"}, {"-binary", "hb", "-concurrency", "1,1"}, {"-binary", "hb", "-concurrency", "0"}, {"-binary", "hb", "-warmup", "1"}, {"-binary", "hb", "-client-procs", "0"}, {"-binary", "hb", "-request-timeout", "0s"}, {"-binary", "hb", "-startup-timeout", "-1s"}, {"-binary", "hb", "-module", "fixture", "-output", "fixture/results"}} {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("accepted invalid options: %v", args)
		}
	}
	options, err := parseOptions([]string{"-binary", "hb"})
	if err != nil {
		t.Fatal(err)
	}
	plan := trialPlan(options)
	if len(plan) != 9 || !reflect.DeepEqual([]int{plan[0].Concurrency, plan[3].Concurrency, plan[6].Concurrency}, []int{1, 16, 64}) {
		t.Fatalf("concurrency order does not rotate: %+v", plan)
	}
}

func TestStatisticsUseNearestRankAndMedianWithoutMutatingSamples(t *testing.T) {
	samples := []int64{10, 1, 8, 3, 6, 5, 4, 7, 2, 9}
	original := append([]int64(nil), samples...)
	got := summarizeLatency(samples)
	if got.P50NS != 5 || got.P95NS != 10 || got.P99NS != 10 || got.Average != 5.5 || !reflect.DeepEqual(original, samples) {
		t.Fatalf("summary=%+v", got)
	}
	if median([]float64{1, 100, 3}) != 3 || median([]float64{1, 5, 3, 7}) != 4 {
		t.Fatal("median incorrect")
	}
	if summarizeLatency(nil) != (latencySummary{}) {
		t.Fatal("empty summary incorrect")
	}
}
