package main

import (
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type CacheEntry struct {
	ContentType   string
	Content       string
	ContentLength string
	ETag          string
	Status        int
	Timestamp     time.Time
	Headers       map[string]string
	Cookies       []string
	ErrorCount    int
	Handled       *shared.HandledResponse
}

var (
	configs                = make(map[string]map[string]interface{})
	configMutex            sync.RWMutex
	hypermediasBySection   = make(map[string][]composite.HyperMediaConfig)
	hypermediasMutex       sync.RWMutex
	routeSourceErrors      = make(map[string][]error)
	routeSourceErrorsMutex sync.RWMutex

	requestCounter      int = 0
	requestCounterMutex sync.RWMutex

	htmlCache        = make(map[string]CacheEntry)
	htmlCacheMutex   sync.RWMutex
	hyperBricksArray = &parser.HyperScriptStringArray{}

	renderDiagnosticsMutex sync.RWMutex
	renderDiagnostics      = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder []string
	renderDiagnosticsSeq   int64 = 0
)

type ComponentErrorTemplate struct {
	Hash string `json:"hash"`
	Type string `json:"type"`
	File string `json:"file"`
	Path string `json:"path"`
	Key  string `json:"key"`
	Err  string `json:"err"`
}

type RenderDiagnostics struct {
	RequestID string                   `json:"request_id"`
	Route     string                   `json:"route"`
	CreatedAt time.Time                `json:"created_at"`
	Errors    []ComponentErrorTemplate `json:"errors"`
}
