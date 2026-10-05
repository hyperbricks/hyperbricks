// Command response-cache measures verified HTTP responses from an isolated
// copy of the response-cache-test module. It uses only the Go standard library.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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

const bodyPattern = "0123456789abcdef"

var backends = []string{"fresh", "mem", "disk"}

type options struct {
	Repo           string        `json:"repository"`
	Binary         string        `json:"server_binary"`
	Module         string        `json:"module_source"`
	Output         string        `json:"output_directory"`
	Requests       int           `json:"requests_per_trial"`
	Warmup         int           `json:"warmup_requests_per_trial"`
	Repeats        int           `json:"repeats"`
	Sizes          []int         `json:"body_sizes_bytes"`
	Concurrency    []int         `json:"concurrency"`
	ClientProcs    int           `json:"client_gomaxprocs"`
	ServerProcs    int           `json:"server_gomaxprocs"`
	RequestTimeout time.Duration `json:"request_timeout_ns"`
	StartupTimeout time.Duration `json:"startup_timeout_ns"`
}

func parseOptions(args []string) (options, error) {
	var cfg options
	var sizes, concurrency string
	flags := flag.NewFlagSet("response-cache", flag.ContinueOnError)
	flags.StringVar(&cfg.Repo, "repo", ".", "HyperBricks checkout directory")
	flags.StringVar(&cfg.Binary, "binary", "", "HyperBricks executable built before measurement (required)")
	flags.StringVar(&cfg.Module, "module", "modules/response-cache-test", "Fixture path, relative to repo unless absolute")
	flags.StringVar(&cfg.Output, "output", "", "New result directory; refuses to overwrite existing output")
	flags.IntVar(&cfg.Requests, "requests", 3000, "Verified measured requests per trial")
	flags.IntVar(&cfg.Warmup, "warmup", 200, "Verified warmup requests per trial, excluded from measurement")
	flags.IntVar(&cfg.Repeats, "repeats", 3, "Repeats of the full matrix with rotating order")
	flags.StringVar(&sizes, "sizes", "16384,262144", "Comma-separated response byte sizes, each a positive multiple of 16")
	flags.StringVar(&concurrency, "concurrency", "1,16", "Comma-separated concurrent closed-loop HTTP workers")
	flags.IntVar(&cfg.ClientProcs, "client-procs", 4, "Client Go execution parallelism")
	flags.DurationVar(&cfg.RequestTimeout, "request-timeout", 10*time.Second, "Timeout for each complete HTTP response")
	flags.DurationVar(&cfg.StartupTimeout, "startup-timeout", 30*time.Second, "Time allowed for the isolated server to listen")
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if flags.NArg() != 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	var err error
	if cfg.Sizes, err = positiveList(sizes, "sizes"); err != nil {
		return cfg, err
	}
	if cfg.Concurrency, err = positiveList(concurrency, "concurrency"); err != nil {
		return cfg, err
	}
	cfg.ServerProcs = 4 // This is an explicit, checked fixture contract.
	if err := validateOptions(cfg); err != nil {
		return cfg, err
	}
	cfg.Repo, err = filepath.Abs(cfg.Repo)
	if err != nil {
		return cfg, err
	}
	if !filepath.IsAbs(cfg.Module) {
		cfg.Module = filepath.Join(cfg.Repo, cfg.Module)
	}
	cfg.Binary, err = filepath.Abs(cfg.Binary)
	if err != nil {
		return cfg, err
	}
	if cfg.Output == "" {
		cfg.Output = filepath.Join(cfg.Repo, "benchmarks", "response-cache", "results", time.Now().UTC().Format("20060102T150405.000000000Z"))
	}
	cfg.Output, err = filepath.Abs(cfg.Output)
	if err != nil {
		return cfg, err
	}
	if pathWithin(cfg.Module, cfg.Output) {
		return cfg, errors.New("result directory must be outside the source module")
	}
	return cfg, nil
}

func positiveList(raw, field string) ([]int, error) {
	var values []int
	seen := make(map[int]bool)
	for _, part := range strings.Split(raw, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value <= 0 || seen[value] {
			return nil, fmt.Errorf("%s must contain distinct positive integers", field)
		}
		seen[value] = true
		values = append(values, value)
	}
	return values, nil
}

func validateOptions(cfg options) error {
	if cfg.Binary == "" {
		return errors.New("-binary is required; use run.sh to build both executables first")
	}
	if cfg.Requests <= 0 || cfg.Warmup <= 0 || cfg.Repeats <= 0 || cfg.ClientProcs <= 0 {
		return errors.New("requests, warmup, repeats and client-procs must be positive")
	}
	if cfg.RequestTimeout <= 0 || cfg.StartupTimeout <= 0 {
		return errors.New("request-timeout and startup-timeout must be positive")
	}
	if len(cfg.Sizes) == 0 || len(cfg.Concurrency) == 0 {
		return errors.New("sizes and concurrency must not be empty")
	}
	for _, size := range cfg.Sizes {
		if size <= 0 || size%len(bodyPattern) != 0 {
			return errors.New("each response size must be a positive multiple of 16 bytes")
		}
	}
	for _, concurrency := range cfg.Concurrency {
		if concurrency <= 0 || concurrency > cfg.Requests || concurrency > cfg.Warmup {
			return errors.New("each concurrency must be positive and no greater than requests or warmup")
		}
	}
	return nil
}

type result struct {
	FormatVersion int          `json:"format_version"`
	StartedAt     time.Time    `json:"started_at_utc"`
	CompletedAt   time.Time    `json:"completed_at_utc"`
	Valid         bool         `json:"valid"`
	Error         string       `json:"error,omitempty"`
	Options       options      `json:"options"`
	Environment   environment  `json:"environment"`
	Provenance    provenance   `json:"provenance"`
	Methodology   []string     `json:"methodology"`
	Server        serverRecord `json:"server"`
	Trials        []trial      `json:"trials"`
	Summary       []aggregate  `json:"summary,omitempty"`
}

type serverRecord struct {
	Command                     []string `json:"command"`
	Address                     string   `json:"address"`
	StartupNS                   int64    `json:"startup_ns"`
	Log                         string   `json:"log"`
	ShutdownClean               bool     `json:"shutdown_clean"`
	DiskEntriesBeforeShutdown   int      `json:"disk_entries_before_shutdown"`
	DiskBodyBytesBeforeShutdown int64    `json:"disk_body_bytes_before_shutdown"`
	NamespacesAfterShutdown     int      `json:"namespaces_after_shutdown"`
}

var methodology = []string{
	"Real loopback TCP HTTP/1.1 with keep-alive; no proxy, compression, conditional requests, application-level retries, browser cache, or load generator process per backend. Go Transport may retry an idempotent request after a reused connection failure; connection acquisitions are recorded.",
	"Closed-loop workers issue one request at a time. Latency starts before client.Do and ends after reading and closing the full response, before byte/hash validation. There is no arrival-rate scheduling or coordinated-omission correction.",
	"Every warmup and measured response must have HTTP 200, exact Content-Length, text/plain content type, and the exact expected body bytes and SHA-256. Cached routes must retain warmup ETag/cache timestamps; fresh routes must have no internal cache metadata.",
	"Throughput is verified successful requests divided by measured batch wall time. Wall time includes client scheduling, body reads, byte comparisons and SHA-256 verification; only latency samples exclude payload verification.",
	"Raw latency_ns contains one successful response sample per request in worker reservation order. p50/p95/p99 use nearest rank: sorted[ceil(p*N)-1]. Average is the arithmetic mean; no outliers are dropped.",
	"Each trial gets a fresh client connection pool and excluded warmup. Backend order rotates by repeat and case index; case order also rotates across repeats. The server remains alive for the complete run.",
	"Memory and disk application caches are warm. Disk files and filesystem pages are warmed too; this is not cold physical-disk I/O and operating-system page-cache memory is not measured.",
	"The server and verifying client share one machine. Client CPU, local network stack, scheduler and other processes can limit results. These are local comparison measurements, not production capacity claims.",
	"No process RSS, heap profile or total system memory measurement is made. Disk file-body bytes are inventory only, not proof of process-memory savings.",
	"Outside timing, disk file counts, sizes and SHA-256 hashes are checked after each trial against only the disk variants warmed so far. This verifies the backend labels against actual disk storage.",
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := execute(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(ctx context.Context, cfg options) (runErr error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return errors.New("this runtime benchmark supports macOS and Linux")
	}
	if runtime.NumCPU() < cfg.ServerProcs {
		return fmt.Errorf("fixture requires at least %d logical CPUs", cfg.ServerProcs)
	}
	runtime.GOMAXPROCS(cfg.ClientProcs)
	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(cfg.Output, 0755); err != nil {
		return fmt.Errorf("create NEW result directory (existing output is never overwritten): %w", err)
	}
	output := result{FormatVersion: 1, StartedAt: time.Now().UTC(), Options: cfg, Environment: collectEnvironment(), Methodology: methodology}
	defer func() {
		output.CompletedAt = time.Now().UTC()
		output.Valid = runErr == nil
		if runErr != nil {
			output.Error = runErr.Error()
		} else {
			output.Summary = summarizeTrials(output.Trials)
		}
		if err := writeResults(cfg.Output, output); err != nil {
			runErr = errors.Join(runErr, err)
		}
		fmt.Printf("Results: %s (valid=%t)\n", cfg.Output, output.Valid)
	}()
	var err error
	output.Provenance, err = collectProvenance(cfg)
	if err != nil {
		return err
	}
	moduleConfig, err := os.ReadFile(filepath.Join(cfg.Module, "package.hyperbricks.yaml"))
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`(?m)^\s+gomaxprocs:\s*4\s*(?:#.*)?$`).Match(moduleConfig) || !regexp.MustCompile(`(?m)^\s+mode:\s*live\s*(?:#.*)?$`).Match(moduleConfig) {
		return errors.New("fixture contract requires literal mode: live and server gomaxprocs: 4 in the default package")
	}
	temporary, err := os.MkdirTemp("", "hyperbricks-response-cache-")
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
		return fmt.Errorf("verify isolated fixture: %w", err)
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
	args := []string{"start", "--module", module, "--port", strconv.Itoa(port)}
	command := exec.Command(cfg.Binary, args...)
	command.Dir = temporary
	command.Env = serverEnvironment(cfg.ServerProcs)
	logFile, err := os.OpenFile(filepath.Join(cfg.Output, "server.log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	command.Stdout, command.Stderr = logFile, logFile
	output.Server = serverRecord{Command: append([]string{cfg.Binary}, args...), Address: address, Log: "server.log"}
	started := time.Now()
	if err := command.Start(); err != nil {
		return fmt.Errorf("start isolated server: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() {
		if files, err := filepath.Glob(filepath.Join(module, ".cache", "responses", "runtime-*", "*.entry")); err == nil {
			output.Server.DiskEntriesBeforeShutdown = len(files)
			for _, path := range files {
				if info, err := os.Stat(path); err == nil {
					output.Server.DiskBodyBytesBeforeShutdown += info.Size()
				}
			}
		}
		stopErr := stopServer(command, done)
		remaining, _ := filepath.Glob(filepath.Join(module, ".cache", "responses", "runtime-*"))
		output.Server.NamespacesAfterShutdown = len(remaining)
		if len(remaining) != 0 {
			stopErr = errors.Join(stopErr, errors.New("runtime cache namespace remained after shutdown"))
		}
		output.Server.ShutdownClean = stopErr == nil
		runErr = errors.Join(runErr, stopErr)
	}()
	if err := waitForServer(ctx, address, done, cfg.StartupTimeout); err != nil {
		return fmt.Errorf("server startup failed (see server.log): %w", err)
	}
	output.Server.StartupNS = time.Since(started).Nanoseconds()
	if _, err := os.Stat(filepath.Join(module, ".cache")); !os.IsNotExist(err) {
		return fmt.Errorf("fixture cache was not empty and lazy before benchmark: %v", err)
	}
	warmedDiskSizes := make(map[int]bool)
	for _, spec := range trialPlan(cfg) {
		fmt.Printf("Trial %02d: repeat=%d body=%d concurrency=%d backend=%s\n", spec.Sequence, spec.Repeat, spec.BodyBytes, spec.Concurrency, spec.Backend)
		measured := runTrial(ctx, "http://"+address, cfg, spec)
		output.Trials = append(output.Trials, measured)
		if !measured.Valid {
			return fmt.Errorf("trial %d failed verification: %s", spec.Sequence, measured.Error)
		}
		if spec.Backend == "disk" {
			warmedDiskSizes[spec.BodyBytes] = true
		}
		if err := verifyDiskInventory(module, warmedDiskSizes); err != nil {
			return fmt.Errorf("disk storage verification after trial %d: %w", spec.Sequence, err)
		}
		fmt.Printf("  %.1f req/s; p50 %.3f ms; p95 %.3f ms; p99 %.3f ms\n", measured.RequestsPerSecond, float64(measured.Latency.P50NS)/1e6, float64(measured.Latency.P95NS)/1e6, float64(measured.Latency.P99NS)/1e6)
	}
	return nil
}

func verifyDiskInventory(module string, sizes map[int]bool) error {
	files, err := filepath.Glob(filepath.Join(module, ".cache", "responses", "runtime-*", "*.entry"))
	if err != nil {
		return err
	}
	if len(files) != len(sizes) {
		return fmt.Errorf("got %d disk entries, want %d warmed disk variants", len(files), len(sizes))
	}
	seen := make(map[int]bool)
	for _, path := range files {
		hash, count, err := hashFile(path)
		if err != nil {
			return err
		}
		if !sizes[int(count)] || seen[int(count)] {
			return fmt.Errorf("unexpected or duplicated disk body size %d", count)
		}
		seen[int(count)] = true
		expected := sha256.Sum256(bytes.Repeat([]byte(bodyPattern), int(count)/len(bodyPattern)))
		if hash != hex.EncodeToString(expected[:]) {
			return fmt.Errorf("disk body %s failed SHA-256 verification", filepath.Base(path))
		}
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
			done <- err // Preserve the process result for shutdown bookkeeping.
			return fmt.Errorf("server exited before listening: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("server did not start before timeout")
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
