package main

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeHTTPShutdownWaitsForActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finish := make(chan struct{})
	var dependencyAlive atomic.Bool
	dependencyAlive.Store(true)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		if !dependencyAlive.Load() {
			t.Error("dependency stopped before request drained")
		}
		w.WriteHeader(http.StatusOK)
	})}
	running := &runtimeHTTPServer{server: srv, done: make(chan struct{})}
	go func() { running.err = srv.Serve(listener); close(running.done) }()
	defer srv.Close()
	requestDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			resp.Body.Close()
		}
		requestDone <- err
	}()
	<-started
	stopDone := make(chan error, 1)
	go func() { stopDone <- running.shutdown(context.Background()); dependencyAlive.Store(false) }()
	<-running.done // Serve returns as soon as Shutdown closes the listener.
	select {
	case <-stopDone:
		t.Fatal("shutdown finished before in-flight request")
	default:
	}
	close(finish)
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown never finished")
	}
}
