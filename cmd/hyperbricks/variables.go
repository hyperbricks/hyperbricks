package main

import (
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
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

const configDiagnosticsRoute = "__config"

var (
	configs                = make(map[string]map[string]interface{})
	routePlans             = make(map[string]*renderplan.Plan)
	configMutex            sync.RWMutex
	hypermediasBySection   = make(map[string][]composite.HyperMediaConfig)
	hypermediasMutex       sync.RWMutex
	routeSourceErrors      = make(map[string][]error)
	routeSourceErrorsMutex sync.RWMutex

	requestCounter      int = 0
	requestCounterMutex sync.RWMutex

	htmlCache      = make(map[string]CacheEntry)
	htmlCacheMutex sync.RWMutex

	renderDiagnosticsMutex sync.RWMutex
	renderDiagnostics      = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder []string
	renderDiagnosticsSeq   int64 = 0
	routeGeneration        uint64
	diagnosticsGeneration  uint64
	diagnosticsRoutes      []string
	diagnosticsEvicted     int
	diagnosticsFloor       int64
)

type ComponentErrorTemplate struct {
	Hash           string `json:"hash"`
	Type           string `json:"type"`
	File           string `json:"file"`
	Path           string `json:"path"`
	Key            string `json:"key"`
	Err            string `json:"err"`
	Level          string `json:"level,omitempty"`
	Rejected       bool   `json:"rejected,omitempty"`
	Line           int    `json:"line,omitempty"`
	Column         int    `json:"column,omitempty"`
	Resource       string `json:"resource,omitempty"`
	ResourceLine   int    `json:"resource_line,omitempty"`
	ResourceColumn int    `json:"resource_column,omitempty"`
	Phase          string `json:"phase,omitempty"`
}

type RenderDiagnostics struct {
	RequestID  string                   `json:"request_id"`
	Route      string                   `json:"route"`
	CreatedAt  time.Time                `json:"created_at"`
	Errors     []ComponentErrorTemplate `json:"errors"`
	ContextID  string                   `json:"context_id,omitempty"`
	Method     string                   `json:"method,omitempty"`
	Generation uint64                   `json:"generation"`
	contextKey string
	sequence   int64
}
