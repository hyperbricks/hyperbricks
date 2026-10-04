package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const developmentOutputTailLimit = 16 * 1024

// developmentProcesses owns only children started for one opted-in session.
// Session cancellation interrupts startup; Stop deliberately owns service
// termination, so an HTTP shutdown can drain while its dependencies still run.
type developmentProcesses struct {
	moduleRoot    string
	invocationDir string
	executable    string
	serverPort    int
	mu            sync.Mutex
	children      []*developmentChild
	stopping      bool
	failures      chan error
	stopOnce      sync.Once
	stopErr       error
}

type developmentChild struct {
	name        string
	phase       string
	cmd         *exec.Cmd
	done        chan struct{}
	waitErr     error // published by closing done
	tail        *developmentOutputTail
	stopTimeout time.Duration
	stopOnce    sync.Once
	stopErr     error
	ready       bool  // guarded by the owner's mu
	failureErr  error // guarded by the owner's mu
}

func newDevelopmentProcesses(moduleRoot string, serverPort int) (*developmentProcesses, error) {
	if !developmentProcessesSupported() {
		return nil, errors.New("development processes are supported only on macOS and Linux")
	}
	invocationDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("development processes: resolve invocation directory: %w", err)
	}
	moduleRoot, err = filepath.Abs(moduleRoot)
	if err != nil {
		return nil, fmt.Errorf("development processes: resolve module directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("development processes: locate HyperBricks executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("development processes: resolve HyperBricks executable: %w", err)
	}
	return &developmentProcesses{moduleRoot: moduleRoot, invocationDir: invocationDir,
		executable: executable, serverPort: serverPort, failures: make(chan error, 1)}, nil
}

func (p *developmentProcesses) Failures() <-chan error { return p.failures }

func (p *developmentProcesses) RunTasks(ctx context.Context, phase string, tasks []shared.DevelopmentTaskConfig) error {
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		timeout := task.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		started := time.Now()
		child, err := p.startChild(phase, task.Name, task.Command, task.Cwd, task.Env, 5*time.Second)
		if err != nil {
			return err
		}
		taskCtx, cancel := context.WithTimeout(ctx, timeout)
		select {
		case <-child.done:
			err = child.exitError()
		case <-taskCtx.Done():
			err = taskCtx.Err()
		}
		cancel()
		// Successful scripts must not leave background children behind either.
		cleanupErr := child.stop()
		if ctx.Err() != nil && cleanupErr == nil {
			return context.Cause(ctx)
		}
		if err != nil || cleanupErr != nil {
			return child.failure(errors.Join(err, cleanupErr))
		}
		logging.GetLogger().Named("hooks").Infow("completed", "phase", phase, "name", task.Name, "duration", time.Since(started))
	}
	return nil
}

func (p *developmentProcesses) StartServices(ctx context.Context, services []shared.DevelopmentServiceConfig) error {
	for _, service := range services {
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		if err := developmentCheckReadinessAddress(ctx, service.Name, service.Ready.HTTP); err != nil {
			if ctx.Err() != nil {
				return context.Cause(ctx)
			}
			return err
		}
		stopTimeout := service.StopTimeout
		if stopTimeout <= 0 {
			stopTimeout = 5 * time.Second
		}
		started := time.Now()
		child, err := p.startChild("service", service.Name, service.Command, service.Cwd, service.Env, stopTimeout)
		if err != nil {
			return err
		}
		go p.monitorService(child)
		timeout := service.Ready.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		if err := developmentWaitReady(ctx, child, service.Ready.HTTP, timeout); err != nil {
			if ctx.Err() != nil {
				return context.Cause(ctx)
			}
			return child.failure(err)
		}
		p.mu.Lock()
		child.ready = true
		p.mu.Unlock()
		logging.GetLogger().Named("services").Infow("ready", "name", service.Name, "pid", child.cmd.Process.Pid, "duration", time.Since(started))
	}
	return nil
}

func (p *developmentProcesses) startChild(phase, name string, args []string, cwd string, env map[string]string, stopTimeout time.Duration) (*developmentChild, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("%s %q: command requires an executable", phase, name)
	}
	if cwd == "" {
		cwd = p.moduleRoot
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(p.invocationDir, cwd)
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return nil, fmt.Errorf("%s %q: working directory: %w", phase, name, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s %q: working directory is not a directory: %s", phase, name, cwd)
	}
	childEnv, err := p.childEnvironment(env)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", phase, name, err)
	}
	child := &developmentChild{name: name, phase: phase, done: make(chan struct{}), tail: &developmentOutputTail{}, stopTimeout: stopTimeout}
	// No shell expansion and no CommandContext: group cleanup is explicit.
	child.cmd = exec.Command(args[0], args[1:]...)
	child.cmd.Dir, child.cmd.Env = cwd, childEnv
	child.cmd.Stdout = &developmentOutputWriter{child: child, stream: "stdout"}
	child.cmd.Stderr = &developmentOutputWriter{child: child, stream: "stderr"}
	child.cmd.WaitDelay = 200 * time.Millisecond
	developmentConfigureProcessGroup(child.cmd)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopping {
		return nil, fmt.Errorf("%s %q: session is stopping", phase, name)
	}
	if err := child.cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s %q: start command: %w", phase, name, err)
	}
	p.children = append(p.children, child)
	go func() { child.waitErr = child.cmd.Wait(); close(child.done) }()
	logging.GetLogger().Named("processes").Infow("started", "phase", phase, "name", name, "pid", child.cmd.Process.Pid)
	return child, nil
}

func (p *developmentProcesses) childEnvironment(overrides map[string]string) ([]string, error) {
	values := make(map[string]string)
	for _, item := range os.Environ() {
		if key, value, ok := strings.Cut(item, "="); ok {
			values[key] = value
		}
	}
	for key, value := range overrides {
		switch key {
		case "PATH", "HB_EXECUTABLE", "HB_MODULE_ROOT", "HB_SERVER_PORT":
			return nil, fmt.Errorf("environment variable %s is reserved", key)
		}
		values[key] = value
	}
	values["HB_EXECUTABLE"], values["HB_MODULE_ROOT"], values["HB_SERVER_PORT"] = p.executable, p.moduleRoot, strconv.Itoa(p.serverPort)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result, nil
}

func (p *developmentProcesses) monitorService(child *developmentChild) {
	<-child.done
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopping {
		return
	}
	child.failureErr = child.unexpectedExitError()
	select {
	case p.failures <- child.failureErr:
	default:
	}
}

func (child *developmentChild) unexpectedExitError() error {
	err := child.exitError()
	if err == nil {
		err = errors.New("exited with status 0")
	}
	return child.failure(fmt.Errorf("exited unexpectedly: %w", err))
}

func (p *developmentProcesses) Stop() error {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		// The session's failure consumer may finish when HTTP draining begins.
		// Preserve a dependency crash during that drain before marking the actual
		// process shutdown intentional. Keep the original error object: the owner
		// may have already received it and can recognize it with errors.Is.
		for _, child := range p.children {
			if child.ready {
				if child.failureErr != nil {
					p.stopErr = errors.Join(p.stopErr, child.failureErr)
					continue
				}
				select {
				case <-child.done:
					child.failureErr = child.unexpectedExitError()
					p.stopErr = errors.Join(p.stopErr, child.failureErr)
				default:
				}
			}
		}
		p.stopping = true
		children := append([]*developmentChild(nil), p.children...)
		p.mu.Unlock()
		for i := len(children) - 1; i >= 0; i-- {
			if err := children[i].stop(); err != nil {
				p.stopErr = errors.Join(p.stopErr, fmt.Errorf("stop %q: %w", children[i].name, err))
			}
		}
	})
	return p.stopErr
}

func (c *developmentChild) exitError() error {
	// WaitDelay bounds inherited output pipes after the leader exits. A successful
	// task still succeeds when stop() cleans the descendant retaining that pipe.
	if errors.Is(c.waitErr, exec.ErrWaitDelay) && c.cmd.ProcessState.Success() {
		return nil
	}
	return c.waitErr
}

func (c *developmentChild) failure(err error) error {
	tail := c.tail.String()
	if tail == "" {
		return fmt.Errorf("%s %q: %w", c.phase, c.name, err)
	}
	return fmt.Errorf("%s %q: %w\nrecent output (maximum 16 KiB):\n%s", c.phase, c.name, err, tail)
}

func (c *developmentChild) stop() error {
	c.stopOnce.Do(func() {
		pid := c.cmd.Process.Pid
		if err := developmentSignalProcessGroup(pid, false); err != nil {
			c.stopErr = errors.Join(c.stopErr, err)
		}
		deadline := time.NewTimer(c.stopTimeout)
		defer deadline.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for developmentProcessGroupAlive(pid) {
			select {
			case <-tick.C:
			case <-deadline.C:
				c.stopErr = errors.Join(c.stopErr, fmt.Errorf("process group required forced termination after %s", c.stopTimeout))
				if err := developmentSignalProcessGroup(pid, true); err != nil {
					c.stopErr = errors.Join(c.stopErr, err)
				}
				goto wait
			}
		}
	wait:
		select {
		case <-c.done:
		case <-time.After(time.Second):
			c.stopErr = errors.Join(c.stopErr, errors.New("timed out reaping child after termination"))
		}
	})
	return c.stopErr
}

func developmentReadinessAddress(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("readiness URL is invalid")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.User != nil || u.Fragment != "" {
		return "", errors.New("readiness URL must use http and a literal loopback address without credentials or fragment")
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

func developmentCheckReadinessAddress(ctx context.Context, name, rawURL string) error {
	address, err := developmentReadinessAddress(rawURL)
	if err != nil {
		return fmt.Errorf("service %q: %w", name, err)
	}
	conn, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(ctx, "tcp", address)
	if err == nil {
		conn.Close()
		return fmt.Errorf("service %q: readiness address %s is already occupied; the existing process was not adopted or stopped", name, address)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func developmentWaitReady(ctx context.Context, child *developmentChild, rawURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// An exit must interrupt a slow HTTP probe as well as the polling interval.
	go func() {
		select {
		case <-child.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	last := "no response"
	for {
		select {
		case <-child.done:
			return fmt.Errorf("exited before readiness: %v", child.exitError())
		case <-ctx.Done():
			select {
			case <-child.done:
				return fmt.Errorf("exited before readiness: %v", child.exitError())
			default:
			}
			return fmt.Errorf("readiness failed: %w (last result: %s)", ctx.Err(), last)
		default:
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("create readiness request: %w", err)
		}
		response, err := client.Do(request)
		if err != nil {
			last = err.Error()
		} else {
			response.Body.Close()
			last = response.Status
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				select {
				case <-child.done:
					return fmt.Errorf("exited during readiness: %v", child.exitError())
				case <-ctx.Done():
					return ctx.Err()
				default:
					return nil
				}
			}
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-child.done:
			timer.Stop()
			return fmt.Errorf("exited before readiness: %v", child.exitError())
		case <-ctx.Done():
			timer.Stop()
			select {
			case <-child.done:
				return fmt.Errorf("exited before readiness: %v", child.exitError())
			default:
			}
			return fmt.Errorf("readiness failed: %w (last result: %s)", ctx.Err(), last)
		case <-timer.C:
		}
	}
}

type developmentOutputTail struct {
	mu   sync.Mutex
	data []byte
}

func (t *developmentOutputTail) Write(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(data) >= developmentOutputTailLimit {
		t.data = append(t.data[:0], data[len(data)-developmentOutputTailLimit:]...)
		return
	}
	if overflow := len(t.data) + len(data) - developmentOutputTailLimit; overflow > 0 {
		copy(t.data, t.data[overflow:])
		t.data = t.data[:len(t.data)-overflow]
	}
	t.data = append(t.data, data...)
}

func (t *developmentOutputTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.data)
}

type developmentOutputWriter struct {
	child  *developmentChild
	stream string
}

func (w *developmentOutputWriter) Write(data []byte) (int, error) {
	w.child.tail.Write(data)
	count := len(data)
	for len(data) > 0 {
		n := min(len(data), 4096)
		chunk := strings.TrimSuffix(string(data[:n]), "\n")
		if chunk != "" {
			logging.GetLogger().Named("processes").Infow(chunk, "name", w.child.name, "stream", w.stream)
		}
		data = data[n:]
	}
	return count, nil
}
