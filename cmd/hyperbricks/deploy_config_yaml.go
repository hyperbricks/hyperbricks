package main

import (
	"fmt"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func loadDeployYAMLRoot(path string) (map[string]interface{}, error) {
	result, err := yamlparser.ProcessConfigFile(path, deployConfigYAMLOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to read deploy config %s: %w", path, err)
	}
	deployRaw, ok := result.Materialized["deploy"].(map[string]interface{})
	if !ok || deployRaw == nil {
		return nil, fmt.Errorf("missing deploy block in %s", path)
	}
	return deployRaw, nil
}

func deployConfigYAMLOptions() yamlparser.Options {
	return yamlparser.Options{
		Paths: yamlparser.PathMarkers{
			Root: ".",
		},
	}
}
