package parser

import "sync"

var (
	HbConfig map[string]interface{}

	// KnownTypes is populated when renderers register their component aliases.
	KnownTypes = map[string]bool{}
)

var templateStore struct {
	sync.RWMutex
	values map[string]string
}

func init() {
	templateStore.values = make(map[string]string)
}

// AddTemplate stores preloaded template content under its runtime key.
func AddTemplate(name, content string) {
	templateStore.Lock()
	defer templateStore.Unlock()
	templateStore.values[name] = content
}

// GetTemplate retrieves preloaded template content by runtime key.
func GetTemplate(name string) (string, bool) {
	templateStore.RLock()
	defer templateStore.RUnlock()
	content, found := templateStore.values[name]
	return content, found
}

// ClearTemplateStore clears all preloaded templates.
func ClearTemplateStore() {
	templateStore.Lock()
	defer templateStore.Unlock()
	templateStore.values = make(map[string]string)
}

// GetTemplateStore returns a copy of the current template store.
func GetTemplateStore() map[string]string {
	templateStore.RLock()
	defer templateStore.RUnlock()
	copy := make(map[string]string, len(templateStore.values))
	for key, value := range templateStore.values {
		copy[key] = value
	}
	return copy
}
