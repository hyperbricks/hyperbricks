package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

const (
	maxStreamUnreadRequestBody   = 256 << 10
	streamRequestBodyReadTimeout = 5 * time.Second
)

var errStreamRequestBodyTooLarge = errors.New("streaming request has more than 256 KiB of unread body")

// writeStreamResponse owns the HTTP response after component rendering has
// finished. The producer receives body output and flushing, not header access.
func writeStreamResponse(w http.ResponseWriter, r *http.Request, content RenderContent) error {
	status := content.Status
	if status == 0 {
		status = http.StatusOK
	}
	var validationErr error
	switch {
	case content.Handled == nil || content.Handled.Stream == nil:
		validationErr = fmt.Errorf("stream response has no producer")
	case len(content.Handled.Body) > 0:
		validationErr = fmt.Errorf("handled response cannot contain both Body and Stream")
	case status < 200 || status > 599:
		validationErr = fmt.Errorf("stream response status must be between 200 and 599")
	case !responseSupportsFlush(w):
		validationErr = fmt.Errorf("response writer does not support streaming flush")
	}
	if validationErr != nil {
		clearStreamCacheHeaders(w.Header())
		w.Header().Set(requestIDHeader, content.RequestID)
		w.Header().Set(renderErrorCountHeader, strconv.Itoa(content.ErrorCount+1))
		http.Error(w, "invalid streaming response", http.StatusInternalServerError)
		return validationErr
	}
	if err := r.Context().Err(); err != nil {
		return err
	}
	if err := drainStreamRequestBody(w, r); err != nil {
		status := http.StatusBadRequest
		var timeout interface{ Timeout() bool }
		if errors.Is(err, errStreamRequestBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		} else if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
			status = http.StatusRequestTimeout
		}
		clearStreamCacheHeaders(w.Header())
		if r.ProtoMajor == 1 {
			// Never try to reuse a connection with an incomplete request body
			// or a read deadline shortened by the preflight watchdog.
			w.Header().Set("Connection", "close")
		}
		w.Header().Set(requestIDHeader, content.RequestID)
		w.Header().Set(renderErrorCountHeader, strconv.Itoa(content.ErrorCount+1))
		http.Error(w, http.StatusText(status), status)
		return err
	}

	applyResponseHeaders(content.Headers, w)
	applyResponseCookies(content.Cookies, w)
	if content.ContentType != "" {
		w.Header().Set("Content-Type", content.ContentType)
	} else {
		w.Header().Set("Content-Type", "text/html")
	}
	clearStreamCacheHeaders(w.Header())
	w.Header().Set(requestIDHeader, content.RequestID)
	w.Header().Set(renderErrorCountHeader, strconv.Itoa(content.ErrorCount))

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	output := &streamBodyWriter{ctx: ctx, cancel: cancel, writer: w, controller: http.NewResponseController(w)}
	w.WriteHeader(status)
	// Flushing the headers also prevents net/http from adding an inferred
	// Content-Length to an empty stream. Existing server write deadlines remain.
	if err := output.flush(); err != nil {
		return err
	}
	if r.Method == http.MethodHead || !responseAllowsBody(status) {
		return nil
	}
	producerErr := content.Handled.Stream(ctx, output, output.flush)
	if err := output.result(); err != nil {
		return err
	}
	return producerErr
}

// HTTP/1 only watches for client disconnect after the request body reaches EOF.
// Settle unread input before starting a callback that may wait on cancellation.
// The limit applies to remaining bytes, so plugins may consume larger bodies in
// Render. Neither successful reads nor timer setup change transport deadlines.
func drainStreamRequestBody(w http.ResponseWriter, r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody {
		return r.Context().Err()
	}
	if !responseSupportsReadDeadline(w) {
		return fmt.Errorf("response writer cannot bound streaming request body reads")
	}
	controller := http.NewResponseController(w)
	var mu sync.Mutex
	var aborted error
	finished := false
	abortRead := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if !finished && aborted == nil {
			aborted = err
			// Shorten only after cancellation or our maximum wait expires.
			// An earlier configured read deadline therefore remains effective.
			_ = controller.SetReadDeadline(time.Now())
		}
	}
	timerDone := make(chan struct{})
	timer := time.AfterFunc(streamRequestBodyReadTimeout, func() {
		defer close(timerDone)
		abortRead(context.DeadlineExceeded)
	})
	cancelDone := make(chan struct{})
	stopCancel := context.AfterFunc(r.Context(), func() {
		defer close(cancelDone)
		abortRead(r.Context().Err())
	})
	defer func() {
		mu.Lock()
		finished = true
		mu.Unlock()
		if !timer.Stop() {
			<-timerDone
		}
		if !stopCancel() {
			<-cancelDone
		}
	}()
	n, err := io.CopyN(io.Discard, r.Body, maxStreamUnreadRequestBody+1)
	mu.Lock()
	finished = true
	abortErr := aborted
	mu.Unlock()
	if n > maxStreamUnreadRequestBody {
		return errStreamRequestBodyTooLarge
	}
	if err != nil && !errors.Is(err, io.EOF) {
		// net/http cancels the request context before returning connection
		// read errors. Keep the concrete body result authoritative so a socket
		// timeout or malformed body cannot be changed by callback scheduling.
		// Retain a concurrent watchdog or request-cancellation cause without
		// letting it replace the concrete read result.
		if abortErr != nil {
			return errors.Join(err, abortErr)
		}
		return err
	}
	if abortErr != nil {
		return abortErr
	}
	return r.Context().Err()
}

func responseSupportsReadDeadline(w http.ResponseWriter) bool {
	for w != nil {
		switch writer := w.(type) {
		case interface{ SetReadDeadline(time.Time) error }:
			return true
		case interface{ Unwrap() http.ResponseWriter }:
			w = writer.Unwrap()
		default:
			return false
		}
	}
	return false
}

func logStreamResponseError(ctx context.Context, route, requestID string, err error) {
	var timeout interface{ Timeout() bool }
	timedOut := errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout())
	if !timedOut && (errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)) {
		logging.GetLogger().Debugw("Stream request cancelled", "route", route, "request_id", requestID, "error", err)
		return
	}
	logging.GetLogger().Errorw("Error streaming response", "route", route, "request_id", requestID, "error", err)
}

func responseSupportsFlush(w http.ResponseWriter) bool {
	for w != nil {
		switch writer := w.(type) {
		case interface{ FlushError() error }:
			return true
		case http.Flusher:
			return true
		case interface{ Unwrap() http.ResponseWriter }:
			w = writer.Unwrap()
		default:
			return false
		}
	}
	return false
}

func clearStreamCacheHeaders(headers http.Header) {
	for name := range headers {
		if strings.EqualFold(name, "Content-Length") || strings.EqualFold(name, "ETag") ||
			strings.EqualFold(name, "Cache-Control") || strings.EqualFold(name, liveCacheRenderedAtHeader) ||
			strings.EqualFold(name, liveCacheExpiresAtHeader) {
			delete(headers, name)
		}
	}
	headers.Set("Cache-Control", "no-store")
}

// The first output error remains visible even if a producer ignores a return
// value. Cancellation asks the producer to stop any work between writes.
type streamBodyWriter struct {
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	writer     io.Writer
	controller *http.ResponseController
	err        error
}

func (w *streamBodyWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.currentError(); err != nil {
		return 0, err
	}
	n, err := w.writer.Write(body)
	if err == nil && n != len(body) {
		err = io.ErrShortWrite
	}
	w.recordError(err)
	return n, err
}

func (w *streamBodyWriter) flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.currentError(); err != nil {
		return err
	}
	err := w.controller.Flush()
	w.recordError(err)
	return err
}

func (w *streamBodyWriter) result() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentError()
}

func (w *streamBodyWriter) currentError() error {
	if w.err != nil {
		return w.err
	}
	return w.ctx.Err()
}

func (w *streamBodyWriter) recordError(err error) {
	if err != nil && w.err == nil {
		w.err = err
		w.cancel()
	}
}
