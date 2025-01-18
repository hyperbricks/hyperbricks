package composite

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// Define a specific implementation of RenderComponent with a concrete error type.
// RENDER
// TEMPLATE
// PAGE
// HEAD
type TreeRenderer struct {
	renderer.CompositeRenderer
}

func TreeRendererConfigGetName() string {
	return "<TREE>"
}

// Ensure ContentRenderer implements RenderComponent with the concrete type `shared.ComponentError`.
var _ shared.CompositeRenderer = (*TreeRenderer)(nil)

// TreeConfig represents the configuration for a RENDER (Container of Assets) type.
type TreeConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"TREE description" example:"{!{tree-@doc.hyperbricks}}"`
	Enclose            string `mapstructure:"enclose" description:"Wrapping property for the tree" example:"{!{tree-enclosure.hyperbricks}}"`
}

func (r *TreeRenderer) Types() []string {
	return []string{
		TreeRendererConfigGetName(),
	}
}

// Validate ensures that the RENDER has valid data.
func (config *TreeConfig) Validate() []error {
	var validationErrors []error
	if config.Meta.ConfigType != TreeRendererConfigGetName() {
		validationErrors = append(validationErrors, shared.ComponentError{
			Key:      config.Meta.Key,
			Path:     config.Meta.Path,
			Err:      fmt.Errorf("invalid type for TREE").Error(),
			Rejected: true,
		})
	}
	if len(config.Items) == 0 {
		validationErrors = append(validationErrors, shared.ComponentError{
			Key:  config.Meta.Key,
			Path: config.Meta.Path,
			Err:  fmt.Errorf("type TREE has not items to render").Error(),
		})
	}

	return validationErrors
}

// Concurrent and Recursive Renderer and returns the result and errors. See render.go.md.
// This function is a blueprint function for all concurent rendering of pages, render and template objects
func (r *TreeRenderer) Render(data interface{}) (string, []error) {
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

	// Step 1: Sort the keys
	itemsSortedOnKeys := shared.SortedUniqueKeys(config.Items)

	var wg sync.WaitGroup

	// Preallocate slices for outputs and errors
	outputs := make([]string, len(itemsSortedOnKeys))
	errorsChan := make(chan []error, len(itemsSortedOnKeys)) // Buffered to prevent goroutine blocking

	// Step 2: Iterate over sorted keys and spawn goroutines for rendering
	for idx, key := range itemsSortedOnKeys {

		switch key {
		case "@type":
			continue
		case "@file":
			continue
		case "@key":
			continue
		}

		component, ok := config.Items[key].(map[string]interface{})
		if !ok {
			path := ""
			if config.Path == "" {
				path = key
			}
			renderErrors = append(renderErrors, shared.ComponentError{
				Key:      key,
				Path:     path,
				Err:      fmt.Sprintf("key  '%s' value is not of any type. parsing as raw data", key),
				Rejected: true,
			})
			val, _ok := config.Items[key].(string)
			if _ok {
				outputs[idx] = "<!-- begin raw value -->" + val + "<!-- end raw value -->"
			}
			continue
		}

		// make a local copy of the component's raw configuration
		localConfig := make(map[string]interface{}, len(component))
		for k, v := range component {
			localConfig[k] = v
		}

		// Update componentConfig with path and key
		localConfig["key"] = key
		if config.Path == "" {
			localConfig["path"] = key
		} else {
			localConfig["path"] = fmt.Sprintf("%s.%s", config.Path, key)
		}

		componentType := ""
		if rawType, ok := component["@type"]; ok {
			if ct, isString := rawType.(string); isString {
				// @type exists and is a string
				componentType = ct
			} else {
				// @type exists but is not a string
				renderErrors = append(renderErrors, shared.ComponentError{
					Path:     localConfig["path"].(string),
					Key:      key,
					Err:      "Render Item has no valid @type, skipping",
					Rejected: true,
				})
				continue
			}
		} else {
			// @type does not exist
			renderErrors = append(renderErrors, shared.ComponentError{
				Path:     localConfig["path"].(string),
				Key:      key,
				Err:      fmt.Sprintf("rendering problems at path %s, Render Item has no (or valid) TYPE, skipping item at key:%s", localConfig["path"].(string), key),
				Rejected: true,
			})
			continue
		}

		wg.Add(1)
		go func(idx int, key, componentType string, componentConfig map[string]interface{}) {
			defer wg.Done()

			// Render the component
			output, errors := r.RenderManager.Render(componentType, componentConfig)

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
