package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLatencySummaryNearestRankAndNoMutation(t *testing.T) {
	samples := make([]int64, 100)
	for i := range samples {
		samples[i] = int64(100 - i)
	}
	original := append([]int64(nil), samples...)
	got := summarizeLatency(samples)
	want := latencySummary{Count: 100, Average: 50.5, MinNS: 1, P50NS: 50, P95NS: 95, P99NS: 99, MaxNS: 100}
	if got != want || !reflect.DeepEqual(samples, original) {
		t.Fatalf("summary=%+v want=%+v; raw samples preserved=%t", got, want, reflect.DeepEqual(samples, original))
	}
	if got := summarizeLatency(nil); got != (latencySummary{}) {
		t.Fatalf("empty samples: %+v", got)
	}
	if got := summarizeLatency([]int64{7}); got.P99NS != 7 || got.Average != 7 {
		t.Fatalf("single sample: %+v", got)
	}
}

func TestAggregateUsesMedianTrialStatistics(t *testing.T) {
	var trials []trial
	for i, rps := range []float64{100, 300, 110} {
		trials = append(trials, trial{trialSpec: trialSpec{Backend: "mem", BodyBytes: 16384, Concurrency: 1}, Valid: true, RequestsPerSecond: rps, Latency: latencySummary{Average: rps, P50NS: int64(1 + i), P95NS: int64(rps), P99NS: int64(rps * 10)}})
	}
	got := summarizeTrials(trials)
	if len(got) != 1 || got[0].MedianRPS != 110 || got[0].MinRPS != 100 || got[0].MaxRPS != 300 || got[0].MedianP95 != 110 || got[0].MedianP99 != 1100 || got[0].SpreadPercent != 200.0/110*100 {
		t.Fatalf("unexpected trial aggregation: %+v", got)
	}
	if median([]float64{30, 10, 20, 40}) != 25 || median(nil) != 0 {
		t.Fatal("even/empty median")
	}
	trials[0].Valid = false
	if summarizeTrials(trials) != nil {
		t.Fatal("invalid trial must suppress comparison")
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{}, {"-binary", "hb", "-requests", "0"}, {"-binary", "hb", "-warmup", "0"},
		{"-binary", "hb", "-repeats", "-1"}, {"-binary", "hb", "-client-procs", "0"},
		{"-binary", "hb", "-sizes", "17"}, {"-binary", "hb", "-sizes", "0"},
		{"-binary", "hb", "-sizes", "16,16"}, {"-binary", "hb", "-concurrency", "1,1"},
		{"-binary", "hb", "-concurrency", ""}, {"-binary", "hb", "-requests", "1"},
		{"-binary", "hb", "-warmup", "1"}, {"-binary", "hb", "-request-timeout", "0s"},
		{"-binary", "hb", "-startup-timeout", "-1s"}, {"-binary", "hb", "unexpected"},
		{"-binary", "hb", "-module", "modules/response-cache-test", "-output", "modules/response-cache-test/results"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseOptions(args); err == nil {
				t.Fatal("expected invalid options")
			}
		})
	}
}

func TestTrialPlanBalancesBackendPositions(t *testing.T) {
	cfg, err := parseOptions([]string{"-binary", "hb"})
	if err != nil {
		t.Fatal(err)
	}
	plan := trialPlan(cfg)
	if len(plan) != 36 {
		t.Fatalf("got %d trials", len(plan))
	}
	positions := make(map[[3]int]map[string]bool)
	for index, trial := range plan {
		if trial.Sequence != index+1 {
			t.Fatal("noncontiguous sequence")
		}
		key := [3]int{trial.BodyBytes, trial.Concurrency, index % 3}
		if positions[key] == nil {
			positions[key] = make(map[string]bool)
		}
		positions[key][trial.Backend] = true
	}
	for key, seen := range positions {
		if len(seen) != 3 {
			t.Fatalf("case/position %v saw %v", key, seen)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestResponseValidationRejectsFalseSuccess(t *testing.T) {
	expected := []byte(bodyPattern)
	digest := sha256.Sum256(expected)
	for _, mode := range []string{"valid", "status", "short", "long", "wrong-bytes", "length", "content-type", "compression", "render-error", "fresh-cached", "cached-metadata", "cached-changed", "network"} {
		t.Run(mode, func(t *testing.T) {
			backend := "fresh"
			metadata := &cacheMetadata{ETag: "expected", RenderedAt: "now", ExpiresAt: "later"}
			if strings.HasPrefix(mode, "cached-") {
				backend = "disk"
			}
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Header.Get("Accept-Encoding") != "identity" || request.Header.Get("If-None-Match") != "" {
					t.Fatal("request must demand full uncompressed body")
				}
				body := append([]byte(nil), expected...)
				response := &http.Response{StatusCode: 200, ContentLength: int64(len(expected)), Proto: "HTTP/1.1", Header: make(http.Header)}
				response.Header.Set("Content-Type", "text/plain; charset=utf-8")
				response.Header.Set("X-Hyperbricks-Render-Error-Count", "0")
				switch mode {
				case "status":
					response.StatusCode = 304
				case "short":
					body = body[:len(body)-1]
				case "long":
					body = append(body, 'x')
				case "wrong-bytes":
					body[0] = 'x'
				case "length":
					response.ContentLength--
				case "content-type":
					response.Header.Set("Content-Type", "text/html")
				case "compression":
					response.Header.Set("Content-Encoding", "gzip")
				case "render-error":
					response.Header.Set("X-Hyperbricks-Render-Error-Count", "1")
				case "fresh-cached":
					response.Header.Set("ETag", "unexpected")
				case "cached-changed":
					response.Header.Set("ETag", "changed")
					response.Header.Set("X-Hyperbricks-Rendered-At", "now")
					response.Header.Set("X-Hyperbricks-Cache-Expires-At", "later")
				case "network":
					return nil, errors.New("network failure")
				}
				response.Body = io.NopCloser(bytes.NewReader(body))
				return response, nil
			})}
			_, _, err := verifiedRequest(context.Background(), client, "http://example.invalid/bench/"+backend, backend, expected, digest, make([]byte, len(expected)+1), metadata, nil)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("validation result: %v", err)
			}
		})
	}
}

func TestManifestCopyExcludesGeneratedCacheAndDetectsChange(t *testing.T) {
	source := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.hyperbricks.yaml", "fixture")
	write("templates/.gitkeep", "")
	write(".cache/responses/ignored.entry", "cache")
	write("generated-cache/responses/ignored.entry", "custom cache")
	recorded, err := sourceManifest(source, false)
	if err != nil || len(recorded.Files) != 2 || recorded.SHA256 == "" {
		t.Fatalf("manifest=%+v error=%v", recorded, err)
	}
	target := filepath.Join(t.TempDir(), "copy")
	if err := copyFixture(source, target, recorded); err != nil {
		t.Fatal(err)
	}
	copied, err := sourceManifest(target, false)
	if err != nil || copied.SHA256 != recorded.SHA256 {
		t.Fatalf("copy manifest=%+v error=%v", copied, err)
	}
	write("package.hyperbricks.yaml", "changed")
	if err := copyFixture(source, filepath.Join(t.TempDir(), "changed-copy"), recorded); err == nil {
		t.Fatal("changed source was accepted")
	}
}

func TestDiskInventoryRejectsMissingExtraAndCorruptBodies(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".cache", "responses", "runtime-test", "body.entry")
	if err := verifyDiskInventory(root, map[int]bool{16: true}); err == nil {
		t.Fatal("missing disk body accepted")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(bodyPattern), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyDiskInventory(root, map[int]bool{16: true}); err != nil {
		t.Fatal(err)
	}
	if err := verifyDiskInventory(root, nil); err == nil {
		t.Fatal("unexpected disk body accepted")
	}
	if err := os.WriteFile(path, []byte("xxxxxxxxxxxxxxxx"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyDiskInventory(root, map[int]bool{16: true}); err == nil {
		t.Fatal("corrupt disk body accepted")
	}
}
