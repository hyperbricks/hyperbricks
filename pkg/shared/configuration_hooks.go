package shared

import "fmt"

// LifecycleHooksConfig contains finite, explicitly enabled operation tasks.
type LifecycleHooksConfig struct {
	BeforeStart  []DevelopmentTaskConfig `mapstructure:"before_start" description:"Finite tasks before startup services and application initialization; requires direct development/debug start --with-processes." example:"[{name: prepare, command: [sh, prepare.sh]}]"`
	AfterStart   []DevelopmentTaskConfig `mapstructure:"after_start" description:"Finite tasks after HTTP starts; requires start --with-processes in development/debug; failure stops the session." example:"[{name: verify, command: [sh, verify.sh]}]"`
	BeforeStatic []DevelopmentTaskConfig `mapstructure:"before_static" description:"Finite tasks after export acceptance and before application initialization; requires static --with-processes." example:"[{name: generate, command: [sh, generate.sh]}]"`
	AfterStatic  []DevelopmentTaskConfig `mapstructure:"after_static" description:"Finite tasks after successful rendering, built-in asset cleanup, asset copy, and optional ZIP; requires static --with-processes; failure retains local output." example:"[{name: publish, command: [sh, publish.sh]}]"`
	Finish       []DevelopmentTaskConfig `mapstructure:"finish" description:"Finite tasks once after owned-resource cleanup on eligible success, failure, or cancellation; requires --with-processes and receives HB_OPERATION, HB_OUTCOME, HB_FAILED_PHASE, and HB_EXIT_CODE under a fresh timeout." example:"[{name: report, command: [sh, report.sh]}]"`
}

func (c *Config) HookTasks(phase string) []DevelopmentTaskConfig {
	switch phase {
	case "before_start":
		if c.Hooks.BeforeStart != nil {
			return c.Hooks.BeforeStart
		}
		return c.Development.Hooks.BeforeStart
	case "after_start":
		if c.Hooks.AfterStart != nil {
			return c.Hooks.AfterStart
		}
		return c.Development.Hooks.AfterStart
	case "before_static":
		return c.Hooks.BeforeStatic
	case "after_static":
		return c.Hooks.AfterStatic
	case "finish":
		return c.Hooks.Finish
	}
	return nil
}

func ReservedProcessEnv(key string) bool {
	switch key {
	case "PATH", "HB_EXECUTABLE", "HB_MODULE_ROOT", "HB_SERVER_PORT", "HB_HOOK_PHASE", "HB_OPERATION", "HB_OUTCOME", "HB_FAILED_PHASE", "HB_EXIT_CODE", "HB_RENDER_DIR", "HB_EXPORT_ZIP":
		return true
	}
	return false
}

func decodeLifecycleHooks(input interface{}) (LifecycleHooksConfig, error) {
	var hooks LifecycleHooksConfig
	root, _ := input.(map[string]interface{})
	raw, exists := root["hooks"]
	if !exists {
		return hooks, nil
	}
	settings, err := processMap(raw, "hyperbricks.hooks", "before_start", "after_start", "before_static", "after_static", "finish")
	if err != nil {
		return hooks, err
	}
	development, _ := root["development"].(map[string]interface{})
	legacy, _ := development["hooks"].(map[string]interface{})
	for _, list := range []struct {
		name   string
		target *[]DevelopmentTaskConfig
	}{
		{"before_start", &hooks.BeforeStart}, {"after_start", &hooks.AfterStart}, {"before_static", &hooks.BeforeStatic}, {"after_static", &hooks.AfterStatic}, {"finish", &hooks.Finish},
	} {
		raw, exists := settings[list.name]
		if !exists {
			continue
		}
		if _, duplicate := legacy[list.name]; duplicate {
			return hooks, fmt.Errorf("hyperbricks.hooks.%s is also declared under development.hooks", list.name)
		}
		items, ok := raw.([]interface{})
		if !ok {
			return hooks, fmt.Errorf("hyperbricks.hooks.%s must be a list", list.name)
		}
		*list.target = make([]DevelopmentTaskConfig, 0, len(items))
		for i, item := range items {
			task, err := decodeDevelopmentTask(item, fmt.Sprintf("hyperbricks.hooks.%s[%d]", list.name, i))
			if err != nil {
				return hooks, err
			}
			*list.target = append(*list.target, task)
		}
	}
	return hooks, nil
}
