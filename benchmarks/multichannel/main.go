// Command multichannel measures verified API fan-out through a separate HyperBricks process.
package main

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

//go:embed *.go
var sourceFiles embed.FS

func sourceHash() string {
	files, _ := sourceFiles.ReadDir(".")
	var all []byte
	for _, f := range files {
		data, _ := sourceFiles.ReadFile(f.Name())
		all = append(all, []byte(f.Name()+"\x00")...)
		all = append(all, data...)
	}
	return hash(all)
}

type options struct {
	Requests, Warmup, Repeats, Concurrency int
	Settle                                 time.Duration
}
type binary struct {
	SHA256, Go string
	Settings   map[string]string
}
type trial struct {
	Repeat                             int
	Workload                           string
	ElapsedNS                          int64
	SamplesNS                          []int64
	RSSSamplesKB                       []int64
	RSSBeforeKB, RSSPeakKB, RSSAfterKB int64
}
type result struct {
	Schema                 int
	Valid                  bool
	Error                  string
	Options                options
	Host, OS, Arch, Kernel string
	CPUs                   int
	Runner, Server         binary
	Fixture                string
	RunnerSource           string
	Started                time.Time
	Trials                 []trial
	CleanShutdown          bool
}

func hash(data []byte) string { v := sha256.Sum256(data); return hex.EncodeToString(v[:]) }
func identify(path string) (binary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return binary{}, err
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return binary{}, err
	}
	b := binary{SHA256: hash(data), Go: info.GoVersion, Settings: map[string]string{}}
	for _, s := range info.Settings {
		b.Settings[s.Key] = s.Value
	}
	if b.Settings["-race"] == "true" {
		return b, errors.New("race builds are not performance measurements")
	}
	return b, nil
}
func rss(pid int) (int64, error) {
	raw, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
}
func validateResponse(resp *http.Response, body []byte, route, rid string, failure bool) error {
	code, want := expected(route, rid, failure)
	if resp.StatusCode != code || string(body) != want {
		return fmt.Errorf("response mismatch: %s want status %d body %q; got %d %q", route, code, want, resp.StatusCode, body)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") || resp.Header.Get("Cache-Control") != "no-store" || len(resp.Cookies()) != 0 {
		return errors.New("response headers violate contract")
	}
	if resp.Header.Get("X-Hyperbricks-Rendered-At") != "" {
		return errors.New("unexpected cached response")
	}
	return nil
}
func batch(ctx context.Context, client *http.Client, base, phase, workload string, count, workers int) ([]int64, int64, error) {
	samples := make([]int64, count)
	var next atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	began := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= count {
					return
				}
				if err := ctx.Err(); err != nil {
					errs <- err
					return
				}
				route := []string{"page", "catalog", "stock", "shipping"}[i%4]
				fail := workload == "mixed" && i%3 == 0
				rid := fmt.Sprintf("%s-%08d", phase, i)
				suffix := "0"
				if fail {
					suffix = "1"
				}
				req, _ := http.NewRequestWithContext(ctx, "GET", base+"/"+route+"?rid="+rid+"&fail="+suffix, nil)
				req.Header.Set("Accept-Encoding", "identity")
				start := time.Now()
				resp, err := client.Do(req)
				if err != nil {
					errs <- err
					return
				}
				body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				closeErr := resp.Body.Close()
				elapsed := time.Since(start).Nanoseconds()
				if err == nil {
					err = closeErr
				}
				if err == nil {
					err = validateResponse(resp, body, route, rid, fail)
				}
				if err != nil {
					errs <- err
					return
				}
				samples[i] = elapsed
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(began).Nanoseconds()
	close(errs)
	for err := range errs {
		return samples, elapsed, err
	}
	return samples, elapsed, nil
}
func run(ctx context.Context, path, out string, cfg options) (r result, err error) {
	r = result{Schema: 1, Options: cfg, OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(), Started: time.Now().UTC(), Fixture: hash([]byte(packageYAML + fixture())), RunnerSource: sourceHash()}
	if cfg.Requests < cfg.Concurrency || cfg.Warmup < cfg.Concurrency || cfg.Concurrency < 1 || cfg.Repeats < 1 || cfg.Settle <= 0 {
		return r, errors.New("positive requests/warmup >= concurrency, repeats and settle required")
	}
	host, e := os.Hostname()
	if e != nil {
		return r, e
	}
	r.Host = hash([]byte(host))
	kernel, e := exec.Command("uname", "-sr").Output()
	if e != nil {
		return r, e
	}
	r.Kernel = strings.TrimSpace(string(kernel))
	path, e = filepath.Abs(path)
	if e != nil {
		return r, e
	}
	r.Server, e = identify(path)
	if e != nil {
		return r, e
	}
	if r.Server.Settings["vcs.modified"] != "false" || len(r.Server.Settings["vcs.revision"]) != 40 {
		return r, errors.New("server must embed a clean exact Git revision; build with -buildvcs=true")
	}
	executable, e := os.Executable()
	if e != nil {
		return r, e
	}
	r.Runner, e = identify(executable)
	if e != nil {
		return r, e
	}
	if err = os.Mkdir(out, 0700); err != nil {
		return r, err
	}
	defer func() {
		r.Valid = err == nil
		if err != nil {
			r.Error = err.Error()
		}
		data, e := json.MarshalIndent(r, "", "  ")
		if e == nil {
			e = os.WriteFile(filepath.Join(out, "result.json"), data, 0600)
		}
		if err == nil {
			err = e
		}
	}()
	tmp, e := os.MkdirTemp("", "hb-multichannel-")
	if e != nil {
		return r, e
	}
	defer os.RemoveAll(tmp)
	mock := httptest.NewServer(http.HandlerFunc(upstream))
	defer mock.Close()
	module := filepath.Join(tmp, "modules", "bench")
	for _, dir := range []string{"hyperbricks", "templates", "resources", "static", "rendered", "plugins"} {
		if e = os.MkdirAll(filepath.Join(module, dir), 0700); e != nil {
			return r, e
		}
	}
	if e = os.WriteFile(filepath.Join(module, "package.hyperbricks.yaml"), []byte(packageYAML), 0600); e != nil {
		return r, e
	}
	if e = os.WriteFile(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"), []byte(strings.ReplaceAll(fixture(), "UPSTREAM", mock.URL)), 0600); e != nil {
		return r, e
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return r, e
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	log, e := os.Create(filepath.Join(out, "server.log"))
	if e != nil {
		return r, e
	}
	defer log.Close()
	cmd := exec.Command(path, "start", "-m", "bench", "--port", strconv.Itoa(port), "--log-level", "error")
	cmd.Dir = tmp
	cmd.Stdout = log
	cmd.Stderr = log
	// Fix runtime tuning for both processes; inherited application variables are excluded.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + tmp, "TMPDIR=" + tmp, "HB_NO_KEYBOARD=1", "GOMAXPROCS=4", "GOGC=100", "GOMEMLIMIT=off"}
	if e = cmd.Start(); e != nil {
		return r, e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case e := <-done:
			r.CleanShutdown = e == nil
			if e != nil && err == nil {
				err = fmt.Errorf("server shutdown: %w", e)
			}
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			if err == nil {
				err = errors.New("server needed forced kill")
			}
		}
	}()
	transport := &http.Transport{Proxy: nil, MaxIdleConnsPerHost: cfg.Concurrency, DisableCompression: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("unexpected redirect") }}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, _, e = batch(ctx, client, base, "ready", "success", 4, 1)
		if e == nil {
			break
		}
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		if time.Now().After(deadline) {
			return r, fmt.Errorf("startup/response preflight: %w", e)
		}
		time.Sleep(100 * time.Millisecond)
	}
	for repeat := 0; repeat < cfg.Repeats; repeat++ {
		for j := 0; j < 2; j++ {
			workload := []string{"success", "mixed"}[(repeat+j)%2]
			phase := fmt.Sprintf("r%02d-%s", repeat, workload)
			if _, _, e = batch(ctx, client, base, phase+"warm", "success", cfg.Warmup, cfg.Concurrency); e != nil {
				return r, e
			}
			t := trial{Repeat: repeat, Workload: workload}
			t.RSSBeforeKB, e = rss(cmd.Process.Pid)
			if e != nil {
				return r, e
			}
			t.RSSPeakKB = t.RSSBeforeKB
			t.RSSSamplesKB = append(t.RSSSamplesKB, t.RSSBeforeKB)
			stop := make(chan struct{})
			monitored := make(chan error, 1)
			go func() {
				ticker := time.NewTicker(50 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-stop:
						monitored <- nil
						return
					case <-ticker.C:
						v, e := rss(cmd.Process.Pid)
						if e != nil {
							monitored <- e
							return
						}
						t.RSSSamplesKB = append(t.RSSSamplesKB, v)
						if v > t.RSSPeakKB {
							t.RSSPeakKB = v
						}
					}
				}
			}()
			t.SamplesNS, t.ElapsedNS, e = batch(ctx, client, base, phase, workload, cfg.Requests, cfg.Concurrency)
			close(stop)
			monitorErr := <-monitored
			if e != nil {
				return r, e
			}
			if monitorErr != nil {
				return r, monitorErr
			}
			select {
			case <-ctx.Done():
				return r, ctx.Err()
			case <-time.After(cfg.Settle):
			}
			t.RSSAfterKB, e = rss(cmd.Process.Pid)
			if e != nil {
				return r, e
			}
			r.Trials = append(r.Trials, t)
		}
	}
	return r, nil
}
func main() {
	binaryPath := flag.String("binary", "", "Clean HyperBricks executable with embedded Git revision")
	output := flag.String("output", "", "New output directory (required)")
	cfg := options{}
	flag.IntVar(&cfg.Requests, "requests", 1200, "Verified requests per trial")
	flag.IntVar(&cfg.Warmup, "warmup", 120, "Warmup requests")
	flag.IntVar(&cfg.Repeats, "repeats", 5, "Repeated success/mixed trials")
	flag.IntVar(&cfg.Concurrency, "concurrency", 8, "Concurrent users")
	flag.DurationVar(&cfg.Settle, "settle", time.Second, "Idle interval before RSS sample")
	compare := flag.String("compare", "", "Reference result.json; supply candidate as only positional argument")
	flag.Parse()
	if *compare != "" {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "one candidate required")
			os.Exit(2)
		}
		if err := compareFiles(*compare, flag.Arg(0)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	if *binaryPath == "" || *output == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "-binary and -output required")
		os.Exit(2)
	}
	runtime.GOMAXPROCS(4)
	// Fix client/upstream GC defaults as part of the workload contract.
	for _, k := range []string{"GOGC", "GOMEMLIMIT", "GODEBUG"} {
		if os.Getenv(k) != "" {
			fmt.Fprintln(os.Stderr, "unset inherited "+k+" for reproducible client measurements")
			os.Exit(2)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, err := run(ctx, *binaryPath, *output, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Validated %d trials; results: %s/result.json\n", len(r.Trials), *output)
}
