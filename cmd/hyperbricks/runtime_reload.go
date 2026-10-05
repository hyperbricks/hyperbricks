package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// One gate serves keyboard and filesystem requests. It owns at most one
// preprocessing callback and never queues a reload across startup/shutdown.
type runtimeReload struct {
	mu      sync.Mutex
	enabled bool
	stopped bool
	running bool
	pending bool
	idle    chan struct{}
	reload  func()
}

func newRuntimeReload(reload func()) *runtimeReload {
	idle := make(chan struct{})
	close(idle)
	return &runtimeReload{idle: idle, reload: reload}
}

func (r *runtimeReload) Enable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped {
		r.enabled = true
	}
}

func (r *runtimeReload) Request() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled {
		return false
	}
	if r.running {
		r.pending = true // one trailing pass preserves changes arriving mid-reload
		return true
	}
	r.running = true
	r.idle = make(chan struct{})
	go func() {
		for {
			r.reload()
			r.mu.Lock()
			if r.enabled && r.pending {
				r.pending = false
				r.mu.Unlock()
				continue
			}
			r.running = false
			close(r.idle)
			r.mu.Unlock()
			return
		}
	}()
	return true
}

func (r *runtimeReload) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.enabled, r.stopped = false, true
	r.pending = false
	idle := r.idle
	r.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for active source reload: %w", ctx.Err())
	}
}

func startRuntimeWatcher(ctx context.Context, directories []string, reload func() bool, cacheDirectories ...string) (<-chan error, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	addRecursive := func(dir string) error {
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if isResponseCacheWatchPath(path, cacheDirectories) {
					return filepath.SkipDir
				}
				return watcher.Add(path)
			}
			return nil
		})
	}
	for _, directory := range directories {
		if err := addRecursive(directory); err != nil {
			_ = watcher.Close()
			return nil, fmt.Errorf("watch directory %s: %w", directory, err)
		}
	}
	done := make(chan error, 1)
	go func() {
		defer close(done)
		defer watcher.Close()
		var timer *time.Timer
		var pending <-chan time.Time
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if isResponseCacheWatchPath(event.Name, cacheDirectories) {
					continue
				}
				if strings.HasPrefix(filepath.Base(event.Name), ".hb-esbuild-") || isEsbuildOutput(event.Name) {
					continue
				}
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						if err := addRecursive(event.Name); err != nil {
							done <- err
							return
						}
					}
				}
				if timer == nil {
					timer = time.NewTimer(500 * time.Millisecond)
				} else {
					timer.Reset(500 * time.Millisecond)
				}
				pending = timer.C
			case <-pending:
				pending = nil
				logging.GetLogger().Debug("Source changes detected; requesting reload")
				reload()
			case err, ok := <-watcher.Errors:
				if ok && err != nil {
					done <- err
				}
				return
			}
		}
	}()
	return done, nil
}

func isResponseCacheWatchPath(path string, configured []string) bool {
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == ".cache" {
			return true
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, directory := range configured {
		if directory == "" {
			continue
		}
		root, err := filepath.Abs(directory)
		if err == nil && cachePathContains(root, absolute) {
			return true
		}
	}
	return false
}
