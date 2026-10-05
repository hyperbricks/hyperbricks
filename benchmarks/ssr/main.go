// Command ssr measures request-isolated uncached HTML over real HTTP.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type options struct {
	Repo           string        `json:"repository"`
	Binary         string        `json:"server_binary"`
	Module         string        `json:"module_source"`
	Output         string        `json:"output_directory"`
	Requests       int           `json:"requests_per_trial"`
	Warmup         int           `json:"warmup_requests_per_trial"`
	Repeats        int           `json:"repeats"`
	Concurrency    []int         `json:"concurrency"`
	ClientProcs    int           `json:"client_gomaxprocs"`
	ServerProcs    int           `json:"server_gomaxprocs"`
	Profile        string        `json:"profile"`
	LogLevel       string        `json:"log_level"`
	RequestTimeout time.Duration `json:"request_timeout_ns"`
	StartupTimeout time.Duration `json:"startup_timeout_ns"`
}

func parseOptions(args []string) (options, error) {
	var cfg options
	var concurrency string
	flags := flag.NewFlagSet("ssr", flag.ContinueOnError)
	flags.StringVar(&cfg.Repo, "repo", ".", "HyperBricks checkout directory")
	flags.StringVar(&cfg.Binary, "binary", "", "HyperBricks executable built before measurement (required)")
	flags.StringVar(&cfg.Module, "module", "modules/ssr-proof-hyperbricks", "Fixture path, relative to repo unless absolute")
	flags.StringVar(&cfg.Output, "output", "", "New result directory; existing output is never overwritten")
	flags.IntVar(&cfg.Requests, "requests", 10000, "Measured verified requests per trial")
	flags.IntVar(&cfg.Warmup, "warmup", 1000, "Excluded verified warmup requests per trial")
	flags.IntVar(&cfg.Repeats, "repeats", 3, "Complete matrix repeats with rotating concurrency order")
	flags.StringVar(&concurrency, "concurrency", "1,16,64", "Distinct concurrent closed-loop worker counts")
	flags.IntVar(&cfg.ClientProcs, "client-procs", 4, "Client Go execution slots")
	flags.DurationVar(&cfg.RequestTimeout, "request-timeout", 10*time.Second, "Each complete HTTP response timeout")
	flags.DurationVar(&cfg.StartupTimeout, "startup-timeout", 30*time.Second, "Server readiness timeout")
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if flags.NArg() != 0 {
		return cfg, errors.New("unexpected positional arguments")
	}
	seen := make(map[int]bool)
	for _, part := range strings.Split(concurrency, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value <= 0 || seen[value] {
			return cfg, errors.New("concurrency must contain distinct positive integers")
		}
		seen[value] = true
		cfg.Concurrency = append(cfg.Concurrency, value)
	}
	cfg.ServerProcs, cfg.Profile, cfg.LogLevel = 4, "package.hyperbricks.yaml", "info"
	if cfg.Binary == "" {
		return cfg, errors.New("-binary is required; run.sh builds both executables first")
	}
	if cfg.Requests <= 0 || cfg.Warmup <= 0 || cfg.Repeats <= 0 || cfg.ClientProcs <= 0 || cfg.RequestTimeout <= 0 || cfg.StartupTimeout <= 0 {
		return cfg, errors.New("counts, execution slots and timeouts must be positive")
	}
	if int64(cfg.Requests) > 9999999999 || int64(cfg.Warmup) > 9999999999 || cfg.Repeats > 9999/len(cfg.Concurrency) {
		return cfg, errors.New("request/trial count exceeds fixed-width request ID contract")
	}
	for _, workers := range cfg.Concurrency {
		if workers > cfg.Requests || workers > cfg.Warmup {
			return cfg, errors.New("concurrency must not exceed measured or warmup requests")
		}
	}
	var err error
	if cfg.Repo, err = filepath.Abs(cfg.Repo); err != nil {
		return cfg, err
	}
	if cfg.Binary, err = filepath.Abs(cfg.Binary); err != nil {
		return cfg, err
	}
	if !filepath.IsAbs(cfg.Module) {
		cfg.Module = filepath.Join(cfg.Repo, cfg.Module)
	}
	if cfg.Output == "" {
		cfg.Output = filepath.Join(cfg.Repo, "benchmarks", "ssr", "results", time.Now().UTC().Format("20060102T150405.000000000Z"))
	}
	if cfg.Output, err = filepath.Abs(cfg.Output); err != nil {
		return cfg, err
	}
	if pathWithin(cfg.Module, cfg.Output) {
		return cfg, errors.New("output must be outside source module")
	}
	return cfg, nil
}

type result struct {
	FormatVersion int             `json:"format_version"`
	Benchmark     string          `json:"benchmark"`
	StartedAt     time.Time       `json:"started_at_utc"`
	CompletedAt   time.Time       `json:"completed_at_utc"`
	Valid         bool            `json:"valid"`
	Error         string          `json:"error,omitempty"`
	Options       options         `json:"options"`
	Workload      workload        `json:"workload"`
	Environment   environment     `json:"environment"`
	Provenance    provenance      `json:"provenance"`
	Methodology   []string        `json:"methodology"`
	Server        serverRecord    `json:"server"`
	Preflight     preflightRecord `json:"preflight"`
	Trials        []trial         `json:"trials"`
	Summary       []aggregate     `json:"summary,omitempty"`
}

type serverRecord struct {
	Command              []string `json:"command"`
	Address              string   `json:"address"`
	StartupNS            int64    `json:"startup_ns"`
	Log                  string   `json:"log"`
	ShutdownClean        bool     `json:"shutdown_clean"`
	ResponseCacheEntries int      `json:"response_cache_entries_after_run"`
}

type preflightRecord struct {
	Valid    bool   `json:"valid"`
	Requests int    `json:"requests"`
	Error    string `json:"error,omitempty"`
}

var methodology = []string{
	"One isolated HyperBricks server uses the fixture's default live profile: four Go execution slots, 30-second transport timeouts, keep-alive and a 1,000,000 requests/second and burst limiter. Explicit CLI info logging excludes per-request debug output.",
	"The index route renders fresh per request with nocache:true and Cache-Control:no-store. Nested templates and trees interpolate the unique query rid at three positions. Every complete response is compared byte-for-byte and by SHA-256 with an independently authored canonical HTML expectation.",
	"Each timed/warmup rid is 38 ASCII bytes: ssr-<16lowerhex>-<4digittrial>-<w|m>-<10digitindex>. A random per-run nonce, trial sequence, phase and reservation index prevent reuse across the run; all measured responses have the same byte length.",
	"Four excluded preflight requests check ordinary and escaping-sensitive request IDs and repeated input. Each trial then has its own excluded warmup and a new connection pool; the server remains alive across the complete run. Concurrency order rotates each repeat.",
	"Real loopback HTTP/1.1 TCP with keep-alive, identity encoding, no proxy, conditional requests or application retries. Go Transport can retry an idempotent GET after a reused connection failure; new/reused connection acquisitions are recorded.",
	"Closed-loop workers send the next request after verification. Throughput is verified requests divided by measured batch wall time, including request-ID/expected-body generation, byte/hash verification and scheduling. Client buffers are allocated before timing.",
	"Latency begins immediately before client.Do and ends after the complete response body is read and closed, before byte/hash validation. It includes connection waiting and network/server work. There is no arrival-rate scheduling or coordinated-omission correction.",
	"Every successful measured response contributes one raw nanosecond sample in reservation order. Trial p50/p95/p99 use nearest rank sorted[ceil(p*N)-1], average is arithmetic, and no outliers are dropped. Across repeats report medians of trial statistics and observed throughput ranges.",
	"The verifying client and server share one host, so client CPU, the scheduler and local network stack can limit throughput. No Caddy, upstream service, process RSS, heap profile or production-capacity measurement is included.",
}

func main() {
	cfg, err := parseOptions(os.Args[1:])
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := execute(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(ctx context.Context, cfg options) (runErr error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return errors.New("benchmark supports macOS and Linux")
	}
	if runtime.NumCPU() < cfg.ServerProcs {
		return errors.New("fixture requires at least four logical CPUs")
	}
	runtime.GOMAXPROCS(cfg.ClientProcs)
	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(cfg.Output, 0755); err != nil {
		return fmt.Errorf("create new result directory: %w", err)
	}
	output := result{FormatVersion: 1, Benchmark: "ssr", StartedAt: time.Now().UTC(), Options: cfg, Workload: workloadContract(), Environment: collectEnvironment(), Methodology: methodology}
	defer func() {
		output.CompletedAt, output.Valid = time.Now().UTC(), runErr == nil
		if runErr != nil {
			output.Error = runErr.Error()
		} else {
			output.Summary = summarizeTrials(output.Trials)
		}
		runErr = errors.Join(runErr, writeResults(cfg.Output, output))
		fmt.Printf("Results: %s (valid=%t)\n", cfg.Output, output.Valid)
	}()
	var err error
	if output.Provenance, err = collectProvenance(cfg); err != nil {
		return err
	}
	config, err := os.ReadFile(filepath.Join(cfg.Module, cfg.Profile))
	if err != nil {
		return err
	}
	for _, pattern := range []string{`(?m)^  mode: live\s*$`, `(?m)^    gomaxprocs: 4\s*$`, `(?m)^    read_timeout: 30s\s*$`, `(?m)^    write_timeout: 30s\s*$`, `(?m)^    idle_timeout: 30s\s*$`, `(?m)^    keep_alives_enabled: true\s*$`, `(?m)^  rate_limit:\s*\n    enabled: true\s*\n    requests_per_second: 1000000\s*\n    burst: 1000000\s*$`} {
		if !regexp.MustCompile(pattern).Match(config) {
			return fmt.Errorf("default fixture profile contract missing %q", pattern)
		}
	}
	temporary, err := os.MkdirTemp("", "hyperbricks-ssr-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	module := filepath.Join(temporary, "module")
	if err := copyFixture(cfg.Module, module, output.Provenance.Fixture); err != nil {
		return err
	}
	output.Provenance.CopiedFixture, err = sourceManifest(module, false)
	if err != nil {
		return err
	}
	if output.Provenance.CopiedFixture.SHA256 != output.Provenance.Fixture.SHA256 {
		return errors.New("isolated fixture differs from recorded source")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	address := "127.0.0.1:" + strconv.Itoa(port)
	args := []string{"start", "--module", module, "--config", cfg.Profile, "--port", strconv.Itoa(port), "--log-level", cfg.LogLevel}
	command := exec.Command(cfg.Binary, args...)
	command.Dir, command.Env = temporary, serverEnvironment(cfg.ServerProcs)
	logFile, err := os.OpenFile(filepath.Join(cfg.Output, "server.log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	command.Stdout, command.Stderr = logFile, logFile
	output.Server = serverRecord{Command: append([]string{cfg.Binary}, args...), Address: address, Log: "server.log"}
	started := time.Now()
	if err := command.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() {
		files, _ := filepath.Glob(filepath.Join(module, ".cache", "responses", "runtime-*", "*.entry"))
		output.Server.ResponseCacheEntries = len(files)
		stopErr := stopServer(command, done)
		output.Server.ShutdownClean = stopErr == nil
		if len(files) != 0 {
			stopErr = errors.Join(stopErr, errors.New("uncached SSR workload unexpectedly wrote response cache entries"))
		}
		runErr = errors.Join(runErr, stopErr)
	}()
	if err := waitForServer(ctx, address, done, cfg.StartupTimeout); err != nil {
		return fmt.Errorf("startup failed (see server.log): %w", err)
	}
	output.Server.StartupNS = time.Since(started).Nanoseconds()
	output.Preflight = runPreflight(ctx, "http://"+address, cfg)
	if !output.Preflight.Valid {
		return fmt.Errorf("preflight: %s", output.Preflight.Error)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	for _, spec := range trialPlan(cfg) {
		fmt.Printf("Trial %02d: repeat=%d concurrency=%d\n", spec.Sequence, spec.Repeat, spec.Concurrency)
		measured := runTrial(ctx, "http://"+address, cfg, spec, hex.EncodeToString(nonce[:]))
		output.Trials = append(output.Trials, measured)
		if !measured.Valid {
			return fmt.Errorf("trial %d failed: %s", spec.Sequence, measured.Error)
		}
		fmt.Printf("  %.1f req/s; p50 %.3f ms; p95 %.3f ms; p99 %.3f ms\n", measured.RequestsPerSecond, float64(measured.Latency.P50NS)/1e6, float64(measured.Latency.P95NS)/1e6, float64(measured.Latency.P99NS)/1e6)
	}
	return nil
}

func waitForServer(ctx context.Context, address string, done chan error, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		connection, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return nil
		}
		select {
		case err := <-done:
			done <- err
			return fmt.Errorf("server exited before listening: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("startup timeout")
		case <-ticker.C:
		}
	}
}

func stopServer(command *exec.Cmd, done <-chan error) error {
	select {
	case err := <-done:
		return fmt.Errorf("server exited before requested shutdown: %v", err)
	default:
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("server SIGTERM shutdown: %w", err)
		}
		return nil
	case <-time.After(10 * time.Second):
		_ = command.Process.Kill()
		<-done
		return errors.New("server required forced shutdown after 10 seconds")
	}
}
