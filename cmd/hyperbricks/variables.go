package main

import (
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

type CacheEntry struct {
	ContentType string
	Content     string
	Status      int
	Timestamp   time.Time
	Headers     map[string]string
	Cookies     []string
	ErrorCount  int
}

var (
	configs              = make(map[string]map[string]interface{})
	configMutex          sync.RWMutex
	hypermediasBySection = make(map[string][]composite.HyperMediaConfig)
	hypermediasMutex     sync.RWMutex

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
	Hash string
	Type string
	File string
	Path string
	Key  string
	Err  string
}

type RenderDiagnostics struct {
	RequestID string
	Route     string
	CreatedAt time.Time
	Errors    []ComponentErrorTemplate
}
