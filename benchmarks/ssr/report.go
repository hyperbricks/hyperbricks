package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type aggregate struct {
	Concurrency   int     `json:"concurrency"`
	Trials        int     `json:"trials"`
	MedianRPS     float64 `json:"median_requests_per_second"`
	MinRPS        float64 `json:"min_requests_per_second"`
	MaxRPS        float64 `json:"max_requests_per_second"`
	SpreadPercent float64 `json:"throughput_range_over_median_percent"`
	MedianAverage float64 `json:"median_trial_average_ns"`
	MedianP50     float64 `json:"median_trial_p50_ns"`
	MedianP95     float64 `json:"median_trial_p95_ns"`
	MedianP99     float64 `json:"median_trial_p99_ns"`
}

func summarizeTrials(trials []trial) []aggregate {
	groups := make(map[int][]trial)
	for _, value := range trials {
		if !value.Valid {
			return nil
		}
		groups[value.Concurrency] = append(groups[value.Concurrency], value)
	}
	var output []aggregate
	for workers, values := range groups {
		var throughput, average, p50, p95, p99 []float64
		for _, value := range values {
			throughput = append(throughput, value.RequestsPerSecond)
			average = append(average, value.Latency.Average)
			p50 = append(p50, float64(value.Latency.P50NS))
			p95 = append(p95, float64(value.Latency.P95NS))
			p99 = append(p99, float64(value.Latency.P99NS))
		}
		sort.Float64s(throughput)
		value := aggregate{Concurrency: workers, Trials: len(values), MedianRPS: median(throughput), MinRPS: throughput[0], MaxRPS: throughput[len(throughput)-1], MedianAverage: median(average), MedianP50: median(p50), MedianP95: median(p95), MedianP99: median(p99)}
		value.SpreadPercent = (value.MaxRPS - value.MinRPS) / value.MedianRPS * 100
		output = append(output, value)
	}
	sort.Slice(output, func(i, j int) bool { return output[i].Concurrency < output[j].Concurrency })
	return output
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 0 {
		return (ordered[middle-1] + ordered[middle]) / 2
	}
	return ordered[middle]
}

func writeResults(directory string, value result) error {
	file, err := os.OpenFile(filepath.Join(directory, "result.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	encodeErr, closeErr := json.NewEncoder(file).Encode(value), file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	file, err = os.OpenFile(filepath.Join(directory, "report.md"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(markdownReport(value))
	closeErr = file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func markdownReport(value result) string {
	var out strings.Builder
	fmt.Fprint(&out, "# HyperBricks SSR HTTP benchmark\n\n")
	fmt.Fprintf(&out, "Run: %s to %s. **Valid: %t.**\n\n", value.StartedAt.Format("2006-01-02 15:04:05Z07:00"), value.CompletedAt.Format("2006-01-02 15:04:05Z07:00"), value.Valid)
	if !value.Valid {
		fmt.Fprintf(&out, "**Invalid run: %s** No aggregate comparison is reported.\n\n", value.Error)
	}
	fmt.Fprintf(&out, "Host: %s, %s %s (%s); %d logical CPUs. Client Go %s, GOMAXPROCS %d; server Go %s, GOMAXPROCS %d. Both processes share this machine.\n\n", value.Environment.CPU, value.Environment.OS, value.Environment.OSVersion, value.Environment.Architecture, value.Environment.LogicalCPUs, value.Environment.ClientGoVersion, value.Options.ClientProcs, value.Provenance.ServerBinary.GoVersion, value.Options.ServerProcs)
	serverRevision := value.Provenance.ServerBinary.Settings["vcs.revision"]
	if serverRevision == "" {
		serverRevision = "not embedded"
	}
	fmt.Fprintf(&out, "Server revision: `%s`, modified=%q, SHA-256 `%s`. Runner checkout: `%s` on `%s`, dirty=%t. The binary's embedded revision identifies the measured server; the runner checkout can differ for a prebuilt server.\n\n", serverRevision, value.Provenance.ServerBinary.Settings["vcs.modified"], value.Provenance.ServerBinary.SHA256, value.Provenance.GitRevision, value.Provenance.GitBranch, value.Provenance.GitDirty)
	fmt.Fprintf(&out, "Fixed profile `%s`, %s logging. Each trial: %d excluded verified warmup requests then %d measured verified requests, %d repeats. Concurrency: %v. Each measured HTML body is %d bytes with a unique %d-byte request ID repeated at three positions. Excluded preflight requests: %d, valid=%t.\n\n", value.Options.Profile, value.Options.LogLevel, value.Options.Warmup, value.Options.Requests, value.Options.Repeats, value.Options.Concurrency, value.Workload.BodyBytes, value.Workload.RequestIDBytes, value.Preflight.Requests, value.Preflight.Valid)
	if value.Valid {
		fmt.Fprint(&out, "## Comparison across repeats\n\nThroughput spread = (maximum − minimum) / median × 100. Latency columns are medians of individual trial averages or percentiles; percentiles are not pooled or averaged. Ranges are descriptive, not confidence intervals.\n\n")
		fmt.Fprintln(&out, "| Workers | Trials | Median req/s | Min–max req/s | Spread | Avg ms | p50 ms | p95 ms | p99 ms |")
		fmt.Fprintln(&out, "| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
		for _, summary := range value.Summary {
			fmt.Fprintf(&out, "| %d | %d | %.1f | %.1f–%.1f | %.1f%% | %.3f | %.3f | %.3f | %.3f |\n", summary.Concurrency, summary.Trials, summary.MedianRPS, summary.MinRPS, summary.MaxRPS, summary.SpreadPercent, summary.MedianAverage/1e6, summary.MedianP50/1e6, summary.MedianP95/1e6, summary.MedianP99/1e6)
		}
	}
	fmt.Fprint(&out, "\n## Raw trials\n\n")
	fmt.Fprintln(&out, "| Order | Repeat | Workers | Valid | Verified | Elapsed s | req/s | Avg ms | p50 ms | p95 ms | p99 ms | New/reused connections |")
	fmt.Fprintln(&out, "| ---: | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, trial := range value.Trials {
		fmt.Fprintf(&out, "| %d | %d | %d | %t | %d | %.6f | %.1f | %.3f | %.3f | %.3f | %.3f | %d/%d |\n", trial.Sequence, trial.Repeat, trial.Concurrency, trial.Valid, trial.Verified, float64(trial.ElapsedNS)/1e9, trial.RequestsPerSecond, trial.Latency.Average/1e6, float64(trial.Latency.P50NS)/1e6, float64(trial.Latency.P95NS)/1e6, float64(trial.Latency.P99NS)/1e6, trial.NewConnections, trial.ReusedConnections)
	}
	fmt.Fprint(&out, "\n## Method\n\n")
	for _, method := range value.Methodology {
		fmt.Fprintf(&out, "- %s\n", method)
	}
	fmt.Fprintf(&out, "\nClean server shutdown: %t. Response-cache body files after run: %d.\n\nFixture SHA-256: `%s`. Runner source SHA-256: `%s`. Runner binary SHA-256: `%s`.\n\n", value.Server.ShutdownClean, value.Server.ResponseCacheEntries, value.Provenance.Fixture.SHA256, value.Provenance.RunnerSource.SHA256, value.Provenance.RunnerBinary.SHA256)
	fmt.Fprintln(&out, "`result.json` retains every measured response latency, validation evidence, request-ID prefixes, exact canonical template and source/binary provenance. `server.log` retains the isolated server output. Build, startup, preflight and warmup are excluded from measured trials.")
	return out.String()
}
