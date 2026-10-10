package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestResponseIsolation(t *testing.T) {
	h := http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}, "Cache-Control": []string{"no-store"}}
	resp := &http.Response{StatusCode: 503, Header: h}
	_, body := expected("page", "alice", true)
	if e := validateResponse(resp, []byte(body), "page", "alice", true); e != nil {
		t.Fatal(e)
	}
	if e := validateResponse(resp, []byte(body), "page", "bob", true); e == nil {
		t.Fatal("cross-user output accepted")
	}
	resp.StatusCode = 200
	if e := validateResponse(resp, []byte(body), "page", "alice", true); e == nil {
		t.Fatal("wrong failure status accepted")
	}
}
func validResult() result {
	r := result{Schema: 1, OS: "darwin", Arch: "arm64", RunnerSource: strings.Repeat("f", 64), Valid: true, CleanShutdown: true, Host: strings.Repeat("a", 64), Fixture: strings.Repeat("b", 64), Kernel: "test", CPUs: 4, Options: options{Requests: 4, Warmup: 4, Repeats: 5, Concurrency: 2, Settle: time.Second}, Runner: binary{Go: "go1.27.2", SHA256: strings.Repeat("c", 64)}, Server: binary{Go: "go1.27.2", SHA256: strings.Repeat("d", 64), Settings: map[string]string{"vcs.modified": "false", "vcs.revision": strings.Repeat("e", 40)}}}
	for i := 0; i < 5; i++ {
		for j := 0; j < 2; j++ {
			r.Trials = append(r.Trials, trial{Repeat: i, Workload: []string{"success", "mixed"}[(i+j)%2], ElapsedNS: 100, SamplesNS: []int64{10, 10, 10, 10}, RSSSamplesKB: []int64{100, 120}, RSSBeforeKB: 100, RSSPeakKB: 120, RSSAfterKB: 110})
		}
	}
	return r
}
func TestInvalidComparisons(t *testing.T) {
	for name, mutate := range map[string]func(*result){"invalid": func(r *result) { r.Valid = false }, "dirty": func(r *result) { r.Server.Settings["vcs.modified"] = "true" }, "runner": func(r *result) { r.Runner.SHA256 = strings.Repeat("f", 64) }, "fixture": func(r *result) { r.Fixture = strings.Repeat("f", 64) }, "host": func(r *result) { r.Host = strings.Repeat("f", 64) }, "missing trial": func(r *result) { r.Trials = r.Trials[1:] }, "zero sample": func(r *result) { r.Trials[0].SamplesNS[0] = 0 }, "duplicate trial": func(r *result) { r.Trials[1] = r.Trials[0] }, "race": func(r *result) { r.Server.Settings["-race"] = "true" }, "shutdown": func(r *result) { r.CleanShutdown = false }, "rss": func(r *result) { r.Trials[0].RSSAfterKB = 0 }, "order": func(r *result) { r.Trials[0], r.Trials[1] = r.Trials[1], r.Trials[0] }} {
		t.Run(name, func(t *testing.T) {
			a, b := validResult(), validResult()
			mutate(&b)
			if comparable(a, b) == nil {
				t.Fatal("invalid comparison accepted")
			}
		})
	}
	if e := comparable(validResult(), validResult()); e != nil {
		t.Fatal(e)
	}
}
func TestNoiseGate(t *testing.T) {
	base := []float64{100, 101, 99, 100, 100}
	_, threshold, bad, e := gate(base, []float64{120, 120, 120, 120, 120}, true)
	if e != nil || !bad || threshold != .05 {
		t.Fatalf("gate: %v %v %v", threshold, bad, e)
	}
	if _, _, _, e = gate([]float64{80, 100, 120, 100, 100}, base, true); e == nil {
		t.Fatal("noisy baseline accepted")
	}
}

func TestMissingEnvironmentAndImpossibleSamples(t *testing.T) {
	for name, change := range map[string]func(*result){
		"os":          func(r *result) { r.OS = "" },
		"go":          func(r *result) { r.Server.Go = "" },
		"overflow":    func(r *result) { r.Trials[0].SamplesNS = []int64{1 << 62, 1 << 62, 1 << 62, 1 << 62} },
		"rss_samples": func(r *result) { r.Trials[0].RSSSamplesKB = nil },
	} {
		t.Run(name, func(t *testing.T) {
			r := validResult()
			change(&r)
			if checked(r) == nil {
				t.Fatal("bad result accepted")
			}
		})
	}
	if _, _, _, err := gate([]float64{100, 100, 100, 100, 100}, []float64{20, 100, 200, 100, 100}, true); err == nil {
		t.Fatal("noisy candidate accepted")
	}
}
