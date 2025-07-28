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
	Timestamp   time.Time
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
)
