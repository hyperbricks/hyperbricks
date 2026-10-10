package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
)

func median(v []float64) float64 {
	v = append([]float64(nil), v...)
	sort.Float64s(v)
	n := len(v)
	if n%2 == 0 {
		return (v[n/2-1] + v[n/2]) / 2
	}
	return v[n/2]
}
func checked(r result) error {
	if r.Schema != 1 || !r.Valid || r.Error != "" || !r.CleanShutdown || r.Options.Repeats < 5 || len(r.Trials) != 2*r.Options.Repeats {
		return errors.New("invalid/incomplete run; gate requires at least five repeats")
	}
	if r.Options.Concurrency < 1 || r.Options.Requests < r.Options.Concurrency || r.Options.Warmup < r.Options.Concurrency || r.Options.Settle <= 0 {
		return errors.New("invalid options")
	}
	if len(r.Server.SHA256) != 64 || len(r.Runner.SHA256) != 64 || len(r.Fixture) != 64 || len(r.Host) != 64 || r.CPUs < 1 || r.Kernel == "" || r.OS == "" || r.Arch == "" || r.Server.Go == "" || r.Runner.Go == "" || len(r.RunnerSource) != 64 || r.Server.Settings["vcs.modified"] != "false" || len(r.Server.Settings["vcs.revision"]) != 40 || r.Server.Settings["-race"] == "true" || r.Runner.Settings["-race"] == "true" {
		return errors.New("incomplete or unsuitable provenance")
	}
	seen := map[string]bool{}
	for _, t := range r.Trials {
		k := fmt.Sprintf("%d-%s", t.Repeat, t.Workload)
		if seen[k] || t.Repeat < 0 || t.Repeat >= r.Options.Repeats || (t.Workload != "success" && t.Workload != "mixed") {
			return errors.New("duplicate/invalid trial matrix")
		}
		seen[k] = true
		if len(t.SamplesNS) != r.Options.Requests || t.ElapsedNS <= 0 || t.RSSBeforeKB <= 0 || t.RSSAfterKB <= 0 || t.RSSPeakKB < t.RSSBeforeKB {
			return errors.New("incomplete samples or RSS")
		}
		if len(t.RSSSamplesKB) == 0 {
			return errors.New("missing RSS samples")
		}
		peak := t.RSSBeforeKB
		for _, v := range t.RSSSamplesKB {
			if v <= 0 {
				return errors.New("invalid RSS sample")
			}
			if v > peak {
				peak = v
			}
		}
		if peak != t.RSSPeakKB {
			return errors.New("RSS peak does not match samples")
		}
		var sum float64
		for _, s := range t.SamplesNS {
			if s <= 0 {
				return errors.New("invalid latency")
			}
			sum += float64(s)
		}
		if sum > float64(t.ElapsedNS)*float64(r.Options.Concurrency)*1.01 {
			return errors.New("impossible timing")
		}
	}
	return nil
}
func compileSettings(b binary) map[string]string {
	out := map[string]string{}
	for k, v := range b.Settings {
		if k != "vcs.revision" && k != "vcs.time" && k != "vcs.modified" {
			out[k] = v
		}
	}
	return out
}
func comparable(a, b result) error {
	if e := checked(a); e != nil {
		return e
	}
	if e := checked(b); e != nil {
		return e
	}
	if a.Options != b.Options || a.Host != b.Host || a.OS != b.OS || a.Arch != b.Arch || a.CPUs != b.CPUs || a.Kernel != b.Kernel || a.RunnerSource != b.RunnerSource || a.Fixture != b.Fixture || a.Runner.SHA256 != b.Runner.SHA256 || a.Server.Go != b.Server.Go || !reflect.DeepEqual(compileSettings(a.Server), compileSettings(b.Server)) {
		return errors.New("different runner, fixture, host, build settings, or workload")
	}
	for i := range a.Trials {
		if a.Trials[i].Repeat != b.Trials[i].Repeat || a.Trials[i].Workload != b.Trials[i].Workload {
			return errors.New("different trial order")
		}
	}
	return nil
}

// Noise uses the baseline full range, not a confidence interval. A 5% floor
// avoids declaring tiny changes meaningful; >20% range refuses a noisy gate.
func gate(reference, candidate []float64, lowerBetter bool) (float64, float64, bool, error) {
	base := median(reference)
	lo, hi := reference[0], reference[0]
	for _, v := range reference {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	spread := (hi - lo) / base
	if spread > 0.20 {
		return 0, 0, false, errors.New("baseline spread exceeds 20%; collect a quieter baseline")
	}
	threshold := math.Max(0.05, 2*spread)
	clo, chi := candidate[0], candidate[0]
	for _, v := range candidate {
		clo = math.Min(clo, v)
		chi = math.Max(chi, v)
	}
	if (chi-clo)/median(candidate) > 0.20 {
		return 0, 0, false, errors.New("candidate spread exceeds 20%; repeat under quieter conditions")
	}
	change := median(candidate)/base - 1
	regression := change > threshold
	if !lowerBetter {
		regression = change < -threshold
	}
	return change, threshold, regression, nil
}
func values(r result, workload, metric string) []float64 {
	var out []float64
	for _, t := range r.Trials {
		if t.Workload != workload {
			continue
		}
		var v float64
		switch metric {
		case "requests/s":
			v = float64(len(t.SamplesNS)) * 1e9 / float64(t.ElapsedNS)
		case "p95_ns":
			samples := append([]int64(nil), t.SamplesNS...)
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			v = float64(samples[int(math.Ceil(.95*float64(len(samples))))-1])
		case "settled_RSS_KB":
			v = float64(t.RSSAfterKB)
		}
		out = append(out, v)
	}
	return out
}
func compareFiles(reference, candidate string) error {
	var a, b result
	for _, entry := range []struct {
		path   string
		target *result
	}{{reference, &a}, {candidate, &b}} {
		raw, e := os.ReadFile(entry.path)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, entry.target); e != nil {
			return e
		}
	}
	// Read independently when comparing a file to itself.
	if reference == candidate {
		b = a
	}
	if e := comparable(a, b); e != nil {
		return e
	}
	regressions := false
	unstable := false
	for _, w := range []string{"success", "mixed"} {
		for _, m := range []string{"requests/s", "p95_ns", "settled_RSS_KB"} {
			change, threshold, bad, e := gate(values(a, w, m), values(b, w, m), m != "requests/s")
			if e != nil {
				fmt.Printf("%s %s: inconclusive: %v\n", w, m, e)
				unstable = true
				continue
			}
			fmt.Printf("%s %s: change %+.2f%%, limit %.2f%%, regression=%t\n", w, m, change*100, threshold*100, bad)
			regressions = regressions || bad
		}
	}
	if unstable {
		return errors.New("comparison inconclusive due to measurement noise")
	}
	if regressions {
		return errors.New("observed regression exceeds baseline-derived limit")
	}
	return nil
}
