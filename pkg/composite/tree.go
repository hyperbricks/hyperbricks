package composite

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

type TreeRenderer struct {
	renderer.CompositeRenderer
}

func TreeRendererConfigGetName() string {
	return "<TREE>"
}

var _ shared.CompositeRenderer = (*TreeRenderer)(nil)

// TreeConfig
type TreeConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Tree composite element can render types in alphanumeric order. Tree elements can have nested types." example:"{!{tree-@doc.hyperbricks}}"`
	Enclose            string `mapstructure:"enclose" description:"Wrapping property for the tree" example:"{!{tree-enclose.hyperbricks}}"`
}

func (r *TreeRenderer) Types() []string {
	return []string{
		TreeRendererConfigGetName(),
	}
}

func (config *TreeConfig) Validate() []error {
	var validationErrors []error
	if config.Meta.ConfigType != TreeRendererConfigGetName() {
		validationErrors = append(validationErrors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			File:     config.Meta.HyperBricksFile,
			Key:      config.Meta.HyperBricksKey,
			Path:     config.Meta.HyperBricksPath,
			Type:     "<TREE>",
			Err:      fmt.Errorf("invalid type").Error(),
			Rejected: true,
		})
	}
	if len(config.Items) == 0 {
		validationErrors = append(validationErrors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			File: config.Meta.HyperBricksFile,
			Key:  config.Meta.HyperBricksKey,
			Path: config.Meta.HyperBricksPath,
			Type: "<TREE>",
			Err:  fmt.Errorf("no items to render").Error(),
		})
	}

	return validationErrors
}

// Concurrent and Recursive Renderer and returns the result and errors.
// This function is a blueprint function for all concurent rendering of pages, render and template objects
func (r *TreeRenderer) Render(data interface{}, ctx context.Context) (string, []error) {
	var renderErrors []error

	// Decode the instance into TemplateConfig without type assertion
	var config TreeConfig
	err := mapstructure.Decode(data, &config)
	if err != nil {
		return "", []error{
			shared.ComponentError{
				Err: fmt.Errorf("can not decode config to type RenderConfig{}").Error(),
			},
		}
	}

	// Step 1: Prefer explicit source order from the YAML source model and fall
	// back to deterministic alphanumeric sorting when no order metadata is present.
	itemsSortedOnKeys := orderedTreeKeys(config.Items)

	var wg sync.WaitGroup

	// Preallocate slices for outputs and errors
	outputs := make([]string, len(itemsSortedOnKeys))
	errorsChan := make(chan []error, len(itemsSortedOnKeys)) // Buffered to prevent goroutine blocking

	// Step 2: Iterate over sorted keys and spawn goroutines for rendering
	for idx, key := range itemsSortedOnKeys {

		switch key {
		case "@type", "@order", "hyperbricksfile", "hyperbrickspath", "hyperbrickskey":
			continue
		}

		component, ok := config.Items[key].(map[string]interface{})
		if !ok {
			renderErrors = append(renderErrors, shared.ComponentError{
				Hash:     shared.GenerateHash(),
				File:     config.Composite.Meta.HyperBricksFile,
				Key:      key,
				Path:     config.Composite.Meta.HyperBricksPath,
				Err:      "render problem, value is not of any type. parsing as raw data",
				Type:     "<TREE>",
				Rejected: true,
			})
			val, _ok := config.Composite.Items[key].(string)
			if _ok {
				//if config.Composite.Items[key].(string) != "" {
				outputs[idx] = "<!-- begin raw value -->" + val + "<!-- end raw value -->"
				//}
				// this is left herefor debugging empty fields...
				// else {
				// 	outputs[idx] = "<!-- empty key-->" + key + "<!-- empty key -->"
				// }
			}
			continue
		}

		// make a local copy of the component's raw configuration
		localConfig := make(map[string]interface{}, len(component))
		for k, v := range component {
			localConfig[k] = v
		}

		// Update componentConfig with path and key
		localConfig["hyperbrickskey"] = key
		localConfig["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		localConfig["hyperbrickspath"] = joinTreePath(config.Composite.Meta.HyperBricksPath, key)

		componentType := ""
		if rawType, ok := component["@type"]; ok {
			if ct, isString := rawType.(string); isString {
				// @type exists and is a string
				componentType = ct
			} else {
				// @type exists but is not a string
				renderErrors = append(renderErrors, shared.ComponentError{
					Hash:     shared.GenerateHash(),
					Type:     "<TREE>",
					File:     config.Composite.Meta.HyperBricksFile,
					Path:     config.Composite.Meta.HyperBricksPath,
					Key:      key,
					Err:      "render Item has no valid @type, skipping",
					Rejected: true,
				})
				continue
			}
		} else {
			// @type does not exist
			renderErrors = append(renderErrors, shared.ComponentError{
				Hash:     shared.GenerateHash(),
				File:     config.Composite.Meta.HyperBricksFile,
				Path:     config.Composite.Meta.HyperBricksPath,
				Key:      key,
				Type:     "<TREE>",
				Err:      "render Item has no (or valid) TYPE, skipping item",
				Rejected: true,
			})
			continue
		}

		wg.Add(1)
		go func(idx int, key, componentType string, componentConfig map[string]interface{}) {
			defer wg.Done()

			// Render the component
			output, errors := r.RenderManager.Render(componentType, componentConfig, ctx)
			// output += fmt.Sprintf("<!-- %s -->", shared.GenerateHash())
			// Store results in preallocated slices
			outputs[idx] = output
			if errors != nil {
				// Uses a buffered channel (errorsChan), which allows Goroutines to send errors asynchronously without blocking, as long as the buffer size is sufficient.
				// Errors are directly preallocated as slices (chan []shared.ComponentError), where each Goroutine sends a batch of errors.
				// After wg.Wait(), the buffered channel is closed, and errors are processed.
				// Errors are collected in batches (slices) from each Goroutine,
				// 		reducing overhead during aggregation compared to one-error-at-a-time collection.
				// The use of a buffered channel and batch error sending makes error management more structured, as each Goroutine sends its results independently.
				errorsChan <- errors // The magic is here
			}
		}(idx, key, componentType, localConfig)
	}

	// Step 3: Wait for all goroutines to finish
	// After wg.Wait(), the buffered error channel is closed, and errors are processed.
	wg.Wait()
	close(errorsChan)

	// Step 4: Aggregate errors from the channel
	for errs := range errorsChan {
		renderErrors = append(renderErrors, errs...)
	}

	// Step 5: Concatenate the outputs in the order of sorted keys
	var renderedComponentOutput strings.Builder
	for _, output := range outputs {
		renderedComponentOutput.WriteString(output)
	}

	// Step 6: Sort the errors based on the order of keys
	sortedErrors := SortCompositeErrors(renderErrors, itemsSortedOnKeys)
	outputHtml := shared.EncloseContent(config.Enclose, renderedComponentOutput.String())
	return outputHtml, sortedErrors
}

func joinTreePath(base string, key string) string {
	base = strings.Trim(base, ".")
	key = strings.Trim(key, ".")
	if base == "" {
		return key
	}
	if key == "" {
		return base
	}
	return base + "." + key
}

func orderedTreeKeys(items map[string]interface{}) []string {
	if len(items) == 0 {
		return nil
	}

	rawOrder, hasOrder := items["@order"]
	if !hasOrder {
		return filterTreeRenderKeys(shared.SortedUniqueKeys(items), nil)
	}

	seen := make(map[string]bool, len(items))
	keys := make([]string, 0, len(items))
	for _, key := range extractTreeOrder(rawOrder) {
		if treeMetadataKey(key) || seen[key] {
			continue
		}
		if _, exists := items[key]; !exists {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func extractTreeOrder(raw interface{}) []string {
	switch typed := raw.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			key, ok := value.(string)
			if !ok || key == "" {
				continue
			}
			out = append(out, key)
		}
		return out
	default:
		return nil
	}
}

func filterTreeRenderKeys(keys []string, seen map[string]bool, prefixed ...string) []string {
	out := append([]string(nil), prefixed...)
	if seen == nil {
		seen = make(map[string]bool, len(out))
		for _, key := range out {
			seen[key] = true
		}
	}
	for _, key := range keys {
		if treeMetadataKey(key) || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func treeMetadataKey(key string) bool {
	switch key {
	case "@type", "@order", "hyperbricksfile", "hyperbrickspath", "hyperbrickskey":
		return true
	default:
		return false
	}
}

func SortCompositeErrors(errors []error, sortedKeys []string) []error {

	keyOrder := make(map[string]int)
	for i, key := range sortedKeys {
		keyOrder[key] = i
	}

	sort.Slice(errors, func(i, j int) bool {
		ci, iok := errors[i].(shared.ComponentError)
		cj, jok := errors[j].(shared.ComponentError)
		if !iok && !jok {
			// Both are not ComponentErrors, keep original order
			return false
		}
		if !iok {
			// Non-ComponentErrors go after ComponentErrors
			return false
		}
		if !jok {
			return true
		}
		return keyOrder[ci.Key] < keyOrder[cj.Key]
	})

	return errors
}
