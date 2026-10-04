package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeReloadGatesAndCoalesces(t *testing.T) {
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var active, maximum atomic.Int32
	r := newRuntimeReload(func() {
		n := active.Add(1)
		if n > maximum.Load() {
			maximum.Store(n)
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
	})
	if r.Request() {
		t.Fatal("reload accepted before startup")
	}
	r.Enable()
	r.Request()
	<-started
	for i := 0; i < 10; i++ {
		r.Request()
	}
	release <- struct{}{}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("changes during reload were lost")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := r.Stop(ctx); err == nil {
		t.Fatal("expected active-reload deadline")
	}
	if r.Request() {
		t.Fatal("reload accepted during shutdown")
	}
	r.Enable()
	if r.Request() {
		t.Fatal("stopped gate reopened")
	}
	close(release)
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() != 1 {
		t.Fatal("reload callbacks ran concurrently")
	}
}

func TestRuntimeWatcherCancellationAndChange(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan struct{}, 1)
	done, err := startRuntimeWatcher(ctx, []string{dir}, func() bool { changed <- struct{}{}; return true })
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page.yaml"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("source change was not delivered")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop")
	}
}
