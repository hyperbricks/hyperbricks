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
	Backend       string  `json:"backend"`
	BodyBytes     int     `json:"body_bytes"`
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
	type key struct {
		backend           string
		size, concurrency int
	}
	groups := make(map[key][]trial)
	for _, current := range trials {
		if !current.Valid {
			return nil
		}
		id := key{current.Backend, current.BodyBytes, current.Concurrency}
		groups[id] = append(groups[id], current)
	}
	var result []aggregate
	for id, values := range groups {
		var throughput, average, p50, p95, p99 []float64
		for _, value := range values {
			throughput = append(throughput, value.RequestsPerSecond)
			average = append(average, value.Latency.Average)
			p50 = append(p50, float64(value.Latency.P50NS))
			p95 = append(p95, float64(value.Latency.P95NS))
			p99 = append(p99, float64(value.Latency.P99NS))
		}
		sort.Float64s(throughput)
		value := aggregate{Backend: id.backend, BodyBytes: id.size, Concurrency: id.concurrency, Trials: len(values), MedianRPS: median(throughput), MinRPS: throughput[0], MaxRPS: throughput[len(throughput)-1], MedianAverage: median(average), MedianP50: median(p50), MedianP95: median(p95), MedianP99: median(p99)}
		if value.MedianRPS > 0 {
			value.SpreadPercent = (value.MaxRPS - value.MinRPS) / value.MedianRPS * 100
		}
		result = append(result, value)
	}
	backendOrder := map[string]int{"fresh": 0, "mem": 1, "disk": 2}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BodyBytes != result[j].BodyBytes {
			return result[i].BodyBytes < result[j].BodyBytes
		}
		if result[i].Concurrency != result[j].Concurrency {
			return result[i].Concurrency < result[j].Concurrency
		}
		return backendOrder[result[i].Backend] < backendOrder[result[j].Backend]
	})
	return result
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
	// Compact JSON keeps all per-request samples without multi-megabyte indent
	// padding. The Markdown report is the human-readable companion.
	jsonFile, err := os.OpenFile(filepath.Join(directory, "result.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	encodeErr := json.NewEncoder(jsonFile).Encode(value)
	closeErr := jsonFile.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	report, err := os.OpenFile(filepath.Join(directory, "report.md"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := report.WriteString(markdownReport(value))
	closeErr = report.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func markdownReport(value result) string {
	var out strings.Builder
	fmt.Fprint(&out, "# Response-cache HTTP benchmark\n\n")
	fmt.Fprintf(&out, "Run: %s to %s. **Valid: %t.**\n\n", value.StartedAt.Format("2006-01-02 15:04:05Z07:00"), value.CompletedAt.Format("2006-01-02 15:04:05Z07:00"), value.Valid)
	if !value.Valid {
		fmt.Fprintf(&out, "**Invalid run: %s** No aggregate comparison is reported.\n\n", value.Error)
	}
	fmt.Fprintf(&out, "Host: %s, %s %s (%s); %d logical CPUs. Client Go %s, GOMAXPROCS %d; server Go %s, GOMAXPROCS %d. Both processes share this machine.\n\n", value.Environment.CPU, value.Environment.OS, value.Environment.OSVersion, value.Environment.Architecture, value.Environment.LogicalCPUs, value.Environment.ClientGoVersion, value.Options.ClientProcs, value.Provenance.ServerBinary.GoVersion, value.Options.ServerProcs)
	fmt.Fprintf(&out, "Git: `%s` on `%s`, dirty=%t. The JSON records exact status, binary hashes, build settings and source-file manifests.\n\n", value.Provenance.GitRevision, value.Provenance.GitBranch, value.Provenance.GitDirty)
	fmt.Fprintf(&out, "Each trial: %d excluded verified warmup requests, then %d measured verified requests; %d repeats. Sizes: %v bytes; concurrency: %v.\n\n", value.Options.Warmup, value.Options.Requests, value.Options.Repeats, value.Options.Sizes, value.Options.Concurrency)
	if value.Valid {
		fmt.Fprint(&out, "## Comparison across repeats\n\n")
		fmt.Fprint(&out, "Throughput columns summarize whole trials. Spread = (maximum − minimum) / median × 100. Latency columns are medians of the individual trial averages or percentiles, not pooled percentiles or averages of percentiles. These small-sample ranges are descriptive, not confidence intervals.\n\n")
		fmt.Fprintln(&out, "| Bytes | Workers | Backend | Trials | Median req/s | Min–max req/s | Spread | Avg ms | p50 ms | p95 ms | p99 ms |")
		fmt.Fprintln(&out, "| ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
		for _, summary := range value.Summary {
			fmt.Fprintf(&out, "| %d | %d | %s | %d | %.1f | %.1f–%.1f | %.1f%% | %.3f | %.3f | %.3f | %.3f |\n", summary.BodyBytes, summary.Concurrency, summary.Backend, summary.Trials, summary.MedianRPS, summary.MinRPS, summary.MaxRPS, summary.SpreadPercent, summary.MedianAverage/1e6, summary.MedianP50/1e6, summary.MedianP95/1e6, summary.MedianP99/1e6)
		}
		fmt.Fprintln(&out)
	}
	fmt.Fprint(&out, "## Raw trials\n\n")
	fmt.Fprintln(&out, "| Order | Repeat | Bytes | Workers | Backend | Valid | Verified | Elapsed s | req/s | Avg ms | p50 ms | p95 ms | p99 ms | New/reused connections |")
	fmt.Fprintln(&out, "| ---: | ---: | ---: | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, trial := range value.Trials {
		fmt.Fprintf(&out, "| %d | %d | %d | %d | %s | %t | %d | %.6f | %.1f | %.3f | %.3f | %.3f | %.3f | %d/%d |\n", trial.Sequence, trial.Repeat, trial.BodyBytes, trial.Concurrency, trial.Backend, trial.Valid, trial.Verified, float64(trial.ElapsedNS)/1e9, trial.RequestsPerSecond, trial.Latency.Average/1e6, float64(trial.Latency.P50NS)/1e6, float64(trial.Latency.P95NS)/1e6, float64(trial.Latency.P99NS)/1e6, trial.NewConnections, trial.ReusedConnections)
	}
	fmt.Fprint(&out, "\n## Method\n\n")
	for _, method := range value.Methodology {
		fmt.Fprintf(&out, "- %s\n", method)
	}
	fmt.Fprintf(&out, "\nBefore shutdown: %d disk body files, %d body bytes. Clean shutdown: %t; remaining runtime namespaces: %d. This inventory does not measure RSS, heap or filesystem page-cache memory.\n\n", value.Server.DiskEntriesBeforeShutdown, value.Server.DiskBodyBytesBeforeShutdown, value.Server.ShutdownClean, value.Server.NamespacesAfterShutdown)
	fmt.Fprintf(&out, "Fixture SHA-256: `%s`. Runner source SHA-256: `%s`.\n\n", value.Provenance.Fixture.SHA256, value.Provenance.RunnerSource.SHA256)
	fmt.Fprintln(&out, "`result.json` contains every successful latency sample in nanoseconds, exact response hashes, connection counts, failures and provenance. `server.log` contains the isolated server output. Build and server startup are excluded from HTTP timing. Disk hits are warm filesystem/page-cache reads; these local results do not establish production capacity.")
	return out.String()
}
