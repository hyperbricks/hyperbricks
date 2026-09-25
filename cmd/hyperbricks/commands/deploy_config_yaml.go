package commands

import (
	"fmt"
	"os"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func loadDeployYAMLRoot(path string) (map[string]interface{}, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("deployment config %s not found; run hyperbricks deploy init or select one with --config", path)
		}
		return nil, fmt.Errorf("cannot access deployment config %s: %w", path, err)
	}
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
