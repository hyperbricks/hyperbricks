package shared

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"sync"
)

// ResponseStatusConfig is an optional, occurrence-local API response policy.
// It selects only a status; the HTTP server retains ownership of the response.
type ResponseStatusConfig struct {
	Enabled  *bool                  `mapstructure:"enabled" description:"Enable this component's response status policy; omitted means enabled when the block is present"`
	Required bool                   `mapstructure:"required" description:"Make an executing component's unhandled upstream or render failure determine the HTTP status; defaults to false"`
	Priority int                    `mapstructure:"priority" description:"Nonnegative priority among mapped status proposals; higher wins, defaults to zero"`
	Map      map[string]interface{} `mapstructure:"map" description:"Received upstream error status (400–599) to browser 404, 410, 502, 503, 504, or ignore; does not match network failures"`
}

func (p *ResponseStatusConfig) Active() bool { return p != nil && (p.Enabled == nil || *p.Enabled) }

func (p *ResponseStatusConfig) Validate() error {
	if p == nil {
		return nil
	}
	if p.Priority < 0 {
		return fmt.Errorf("response_status.priority must be a nonnegative integer")
	}
	keys := make([]string, 0, len(p.Map))
	for key := range p.Map {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		n, err := strconv.Atoi(key)
		if err != nil || n < 400 || n > 599 || strconv.Itoa(n) != key {
			return fmt.Errorf("response_status.map key %q must be an upstream HTTP status from 400 to 599", key)
		}
		if _, err := responseStatusTarget(p.Map[key]); err != nil {
			return fmt.Errorf("response_status.map.%s: %w", key, err)
		}
	}
	return nil
}

func responseStatusTarget(value interface{}) (int, error) {
	if s, ok := value.(string); ok && s == "ignore" {
		return 0, nil
	}
	v := reflect.ValueOf(value)
	var n int64
	if v.IsValid() {
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n = v.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v.Uint() <= 599 {
				n = int64(v.Uint())
			}
		case reflect.Float32, reflect.Float64:
			f := v.Float()
			if f >= 0 && f <= 599 && math.Trunc(f) == f {
				n = int64(f)
			}
		}
	}
	switch n {
	case 404, 410, 502, 503, 504:
		return int(n), nil
	default:
		return 0, fmt.Errorf("must be 404, 410, 502, 503, 504, or ignore")
	}
}

// Received and Failure preserve the difference between an HTTP response and a
// synthetic template status, including decoding failure after a real response.
type APIResponseFailure uint8

const (
	APIFailureNone APIResponseFailure = iota
	APIFailureLocal
	APIFailureTransport
	APIFailureDecode
	APIFailureTemplate
)

type APIResponseOutcome struct {
	Status   int
	Received bool
	Failure  APIResponseFailure
	Err      error
}

const ResponseStatusCaptureKey contextKey = "responseStatusCapture"

type responseStatusEntry struct {
	status     int
	priority   int
	failure    bool
	meta       Meta
	diagnostic error
}

// ResponseStatusCapture lives for one render request. Result is read only after
// every child has finished; arrival order cannot determine the response.
type ResponseStatusCapture struct {
	mu      sync.Mutex
	entries []responseStatusEntry
	seen    map[string]bool
}

func CaptureResponseStatus(ctx context.Context, policy *ResponseStatusConfig, meta Meta, outcome APIResponseOutcome) {
	if ctx == nil || !policy.Active() {
		return
	}
	capture, _ := ctx.Value(ResponseStatusCaptureKey).(*ResponseStatusCapture)
	if capture == nil {
		return
	} // Non-HTTP rendering has no response to select.
	capture.record(ctx, policy, meta, outcome)
}

func (c *ResponseStatusCapture) record(ctx context.Context, p *ResponseStatusConfig, meta Meta, outcome APIResponseOutcome) {
	entry := responseStatusEntry{priority: p.Priority, meta: meta}
	if err := p.Validate(); err != nil {
		entry.status, entry.failure, entry.diagnostic = 500, true, err
	} else if ctx.Err() != nil || errors.Is(outcome.Err, context.Canceled) {
		return // A canceled request must not become an upstream failure.
	} else if outcome.Failure != APIFailureNone {
		if p.Required {
			entry.failure = true
			switch outcome.Failure {
			case APIFailureLocal, APIFailureTemplate:
				entry.status = 500
			default:
				entry.status = 502
				var timeout interface{ Timeout() bool }
				if errors.Is(outcome.Err, context.DeadlineExceeded) || (errors.As(outcome.Err, &timeout) && timeout.Timeout()) {
					entry.status = 504
				}
			}
			// Do not put underlying request URLs, credentials or payloads in a new diagnostic.
			entry.diagnostic = fmt.Errorf("required API component failed; response_status selected HTTP %d", entry.status)
		}
	} else if outcome.Received {
		if value, exists := p.Map[strconv.Itoa(outcome.Status)]; exists {
			entry.status, _ = responseStatusTarget(value)
		} else if p.Required && (outcome.Status < 200 || outcome.Status >= 300) {
			entry.status, entry.failure = 502, true
			entry.diagnostic = fmt.Errorf("required API returned unmapped HTTP %d; response_status selected HTTP 502", outcome.Status)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen == nil {
		c.seen = make(map[string]bool)
	}
	identity := meta.HyperBricksFile + ":" + meta.HyperBricksPath
	if meta.HyperBricksPath == "" {
		identity = fmt.Sprintf("policy:%p", p)
	}
	if c.seen[identity] {
		entry.status, entry.failure = 500, true
		entry.diagnostic = fmt.Errorf("response_status component executed more than once")
	}
	c.seen[identity] = true
	if entry.status != 0 {
		c.entries = append(c.entries, entry)
	}
}

// Result returns zero when no active component proposes a status. Required
// failures outrank mapped statuses; among failures use 500 > 504 > 502.
func (c *ResponseStatusCapture) Result() (int, []error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries := append([]responseStatusEntry(nil), c.entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.failure != b.failure {
			return a.failure
		}
		if a.failure && a.status != b.status {
			return responseFailureRank(a.status) > responseFailureRank(b.status)
		}
		if a.priority != b.priority {
			return a.priority > b.priority
		}
		if a.meta.HyperBricksPath != b.meta.HyperBricksPath {
			return a.meta.HyperBricksPath < b.meta.HyperBricksPath
		}
		return a.status < b.status
	})
	if len(entries) == 0 {
		return 0, nil
	}
	var diagnostics []error
	for _, entry := range entries {
		if entry.diagnostic != nil {
			diagnostics = append(diagnostics, Diagnostic(entry.diagnostic, entry.meta, "response_status"))
		}
	}
	winner := entries[0]
	if !winner.failure {
		for _, entry := range entries[1:] {
			if entry.priority != winner.priority {
				break
			}
			if entry.status != winner.status {
				diagnostics = append(diagnostics, Diagnostic(fmt.Errorf("response_status has conflicting HTTP statuses %d and %d at priority %d; use different priorities", winner.status, entry.status, winner.priority), entry.meta, "response_status"))
				return 500, diagnostics
			}
		}
	}
	return winner.status, diagnostics
}

func responseFailureRank(status int) int {
	switch status {
	case 500:
		return 3
	case 504:
		return 2
	default:
		return 1
	}
}
