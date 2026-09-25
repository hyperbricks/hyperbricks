package main

import (
	"fmt"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

// Convert parser-owned location maps once, before configs are published. The
// immutable typed value survives request cloning without copying its field map.
func prepareSourceMetadata(value interface{}) error {
	switch value := value.(type) {
	case map[string]interface{}:
		if raw, ok := value["@source"].(map[string]interface{}); ok {
			var source shared.SourceContext
			if err := mapstructure.Decode(raw, &source); err != nil {
				return fmt.Errorf("decode runtime source metadata: %w", err)
			}
			value["@source"] = &source
		}
		for key, child := range value {
			if key != "@source" {
				if err := prepareSourceMetadata(child); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		for _, child := range value {
			if err := prepareSourceMetadata(child); err != nil {
				return err
			}
		}
	}
	return nil
}
