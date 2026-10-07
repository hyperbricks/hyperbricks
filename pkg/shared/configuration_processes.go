package shared

import (
	"fmt"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

// DevelopmentHooksConfig declares finite tasks owned by an opted-in start.
type DevelopmentHooksConfig struct {
	BeforeStart []DevelopmentTaskConfig `mapstructure:"before_start" description:"Required finite tasks run once in declaration order before services and component/plugin initialization. Defaults to no tasks." example:"[{name: prepare, command: [sh, prepare.sh], timeout: 30s}]"`
	AfterStart  []DevelopmentTaskConfig `mapstructure:"after_start" description:"Required finite tasks run once after HTTP serving begins. A failure shuts down the session; requests already served are not rolled back." example:"[{name: verify, command: [sh, verify.sh], timeout: 10s}]"`
}

type DevelopmentTaskConfig struct {
	Name    string            `mapstructure:"name" description:"Required readable identifier, unique across all hooks and services in the module; used in diagnostics." example:"prepare-demo"`
	Command []string          `mapstructure:"command" description:"Required nonempty executable and literal argument list. No implicit shell expansion; invoke sh explicitly for scripts." example:"[sh, prepare.sh]"`
	Cwd     string            `mapstructure:"cwd" description:"Child working directory, defaulting to the selected module. Accepts normal path resolvers; relative results use the invocation directory. Does not change the parent directory." example:"{path: {base: module, path: demo-api}}"`
	Env     map[string]string `mapstructure:"env" description:"Child-only string environment overrides after resolver evaluation. PATH, HB_EXECUTABLE, HB_MODULE_ROOT, and HB_SERVER_PORT are reserved and cannot be overridden." example:"{PYTHONUNBUFFERED: '1'}"`
	Timeout time.Duration     `mapstructure:"timeout" description:"Positive execution duration, default 30s; timeout is followed by bounded process-group termination." example:"30s"`
}

type DevelopmentServiceConfig struct {
	Name        string                        `mapstructure:"name" description:"Required readable identifier, unique across all hooks and services in the module; used in diagnostics." example:"demo-api"`
	Command     []string                      `mapstructure:"command" description:"Required foreground executable and literal argument list. The process must stay alive and own its listener; do not daemonize or background it." example:"[python3, server.py]"`
	Cwd         string                        `mapstructure:"cwd" description:"Child working directory, defaulting to the selected module. Accepts normal path resolvers; relative results use the invocation directory." example:"{path: {base: module, path: demo-api}}"`
	Env         map[string]string             `mapstructure:"env" description:"Child-only string environment overrides. Parent environment is inherited; PATH and the HB_EXECUTABLE, HB_MODULE_ROOT, HB_SERVER_PORT variables are reserved." example:"{PYTHONUNBUFFERED: '1'}"`
	Ready       DevelopmentServiceReadyConfig `mapstructure:"ready" description:"Required local HTTP startup probe. A 2xx response marks readiness; redirects are not followed. Does not continuously monitor health." example:"{http: 'http://127.0.0.1:4319/health', timeout: 10s}"`
	StopTimeout time.Duration                 `mapstructure:"stop_timeout" description:"Positive grace period after SIGTERM before forced process-group termination. Defaults to 5s." example:"5s"`
}

type DevelopmentServiceReadyConfig struct {
	HTTP    string        `mapstructure:"http" description:"Required absolute plain HTTP URL using a literal loopback address (127.0.0.0/8 or [::1]); no DNS names, credentials, HTTPS, or fragment." example:"http://127.0.0.1:4319/health"`
	Timeout time.Duration `mapstructure:"timeout" description:"Positive startup readiness deadline measured from process spawn. Defaults to 10s." example:"10s"`
}

const developmentProcessesPath = "hyperbricks.development"

// HasDevelopmentProcesses reports whether this package declares any commands.
func (c *Config) HasDevelopmentProcesses() bool {
	return c != nil && (len(c.HookTasks("before_start"))+len(c.HookTasks("after_start"))+len(c.Hooks.Finish)+len(c.Development.Services) > 0)
}

// ValidateDevelopmentProcesses performs host-independent structural validation.
// It neither looks up executables nor touches working directories or services,
// so inspection and disabled process configuration need no development tools.
func (c *Config) ValidateDevelopmentProcesses() error {
	if c == nil {
		return nil
	}
	if c.processesConfigError != nil {
		return c.processesConfigError
	}
	names := map[string]string{}
	validateCommand := func(name string, command []string, cwd string, env map[string]string, path string) error {
		if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s.name must be a nonempty readable identifier without surrounding whitespace or control characters", path)
		}
		if earlier, exists := names[name]; exists {
			return fmt.Errorf("%s.name %q duplicates %s.name; names must be unique across hooks and services", path, name, earlier)
		}
		names[name] = path
		if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
			return fmt.Errorf("%s.command must be a nonempty string argument list with an executable", path)
		}
		for i, arg := range command {
			if strings.ContainsRune(arg, '\x00') {
				return fmt.Errorf("%s.command[%d] cannot contain NUL", path, i)
			}
		}
		if strings.ContainsRune(cwd, '\x00') {
			return fmt.Errorf("%s.cwd cannot contain NUL", path)
		}
		for _, key := range sortedProcessKeys(env) {
			if key == "" || strings.ContainsAny(key, "=\x00") {
				return fmt.Errorf("%s.env has an invalid environment variable name", path)
			}
			if ReservedProcessEnv(key) {
				return fmt.Errorf("%s.env.%s is reserved and cannot be overridden", path, key)
			}
			if strings.ContainsRune(env[key], '\x00') {
				return fmt.Errorf("%s.env.%s cannot contain NUL", path, key)
			}
		}
		return nil
	}
	for _, list := range []struct {
		name  string
		tasks []DevelopmentTaskConfig
	}{{"before_start", c.HookTasks("before_start")}, {"after_start", c.HookTasks("after_start")}, {"before_static", c.Hooks.BeforeStatic}, {"after_static", c.Hooks.AfterStatic}, {"finish", c.Hooks.Finish}} {
		for i, task := range list.tasks {
			path := fmt.Sprintf("hyperbricks.hooks.%s[%d]", list.name, i)
			if (list.name == "before_start" && c.Hooks.BeforeStart == nil) || (list.name == "after_start" && c.Hooks.AfterStart == nil) {
				path = fmt.Sprintf("%s.hooks.%s[%d]", developmentProcessesPath, list.name, i)
			}
			if err := validateCommand(task.Name, task.Command, task.Cwd, task.Env, path); err != nil {
				return err
			}
			if task.Timeout <= 0 {
				return fmt.Errorf("%s.timeout must be a positive duration", path)
			}
		}
	}
	for i, service := range c.Development.Services {
		path := fmt.Sprintf("%s.services[%d]", developmentProcessesPath, i)
		if err := validateCommand(service.Name, service.Command, service.Cwd, service.Env, path); err != nil {
			return err
		}
		if err := validateDevelopmentReadyURL(service.Ready.HTTP); err != nil {
			return fmt.Errorf("%s.ready.http: %w", path, err)
		}
		if service.Ready.Timeout <= 0 {
			return fmt.Errorf("%s.ready.timeout must be a positive duration", path)
		}
		if service.StopTimeout <= 0 {
			return fmt.Errorf("%s.stop_timeout must be a positive duration", path)
		}
	}
	return nil
}

func validateDevelopmentReadyURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.Opaque != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("must be an absolute http:// URL with a literal loopback IP, without credentials or fragment")
	}
	ip, err := netip.ParseAddr(u.Hostname())
	if err != nil || ip.Zone() != "" || !(ip.Is4() && ip.IsLoopback() || ip == netip.IPv6Loopback()) {
		return fmt.Errorf("host must be a literal IPv4 loopback address (127.0.0.0/8) or [::1]")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("port cannot be empty")
	}
	return nil
}

func sortedProcessKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func decodeDevelopmentProcesses(input interface{}) (DevelopmentHooksConfig, []DevelopmentServiceConfig, error) {
	hooks := DevelopmentHooksConfig{}
	var services []DevelopmentServiceConfig
	fail := func(err error) (DevelopmentHooksConfig, []DevelopmentServiceConfig, error) {
		return DevelopmentHooksConfig{}, nil, err
	}
	root, _ := input.(map[string]interface{})
	development, _ := root["development"].(map[string]interface{})
	if raw, exists := development["hooks"]; exists {
		settings, err := processMap(raw, developmentProcessesPath+".hooks", "before_start", "after_start")
		if err != nil {
			return fail(err)
		}
		for _, list := range []struct {
			name   string
			target *[]DevelopmentTaskConfig
		}{{"before_start", &hooks.BeforeStart}, {"after_start", &hooks.AfterStart}} {
			raw, exists := settings[list.name]
			if !exists {
				continue
			}
			path := developmentProcessesPath + ".hooks." + list.name
			items, ok := raw.([]interface{})
			if !ok {
				return fail(fmt.Errorf("%s must be a list", path))
			}
			for i, item := range items {
				task, err := decodeDevelopmentTask(item, fmt.Sprintf("%s[%d]", path, i))
				if err != nil {
					return fail(err)
				}
				*list.target = append(*list.target, task)
			}
		}
	}
	if raw, exists := development["services"]; exists {
		path := developmentProcessesPath + ".services"
		items, ok := raw.([]interface{})
		if !ok {
			return fail(fmt.Errorf("%s must be a list", path))
		}
		for i, item := range items {
			service, err := decodeDevelopmentService(item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return fail(err)
			}
			services = append(services, service)
		}
	}
	config := &Config{Development: DevelopmentConfig{Hooks: hooks, Services: services}}
	if err := config.ValidateDevelopmentProcesses(); err != nil {
		return fail(err)
	}
	return hooks, services, nil
}

func processMap(raw interface{}, path string, allowed ...string) (map[string]interface{}, error) {
	settings, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a mapping", path)
	}
	for _, key := range sortedProcessKeys(settings) {
		known := false
		for _, candidate := range allowed {
			if candidate == key {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("%s.%s is an unknown field", path, key)
		}
	}
	return settings, nil
}

func processString(settings map[string]interface{}, key, path string) (string, error) {
	raw, exists := settings[key]
	if !exists {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s.%s must be a string", path, key)
	}
	return value, nil
}

func processCommandFields(settings map[string]interface{}, path string) (DevelopmentTaskConfig, error) {
	var task DevelopmentTaskConfig
	var err error
	if task.Name, err = processString(settings, "name", path); err != nil {
		return task, err
	}
	if task.Cwd, err = processString(settings, "cwd", path); err != nil {
		return task, err
	}
	args, ok := settings["command"].([]interface{})
	if !ok {
		return task, fmt.Errorf("%s.command must be a nonempty string argument list", path)
	}
	for i, arg := range args {
		value, ok := arg.(string)
		if !ok {
			return task, fmt.Errorf("%s.command[%d] must be a string", path, i)
		}
		task.Command = append(task.Command, value)
	}
	if raw, exists := settings["env"]; exists {
		env, ok := raw.(map[string]interface{})
		if !ok {
			return task, fmt.Errorf("%s.env must be a string mapping", path)
		}
		task.Env = make(map[string]string, len(env))
		for _, key := range sortedProcessKeys(env) {
			value, ok := env[key].(string)
			if !ok {
				return task, fmt.Errorf("%s.env.%s must be a string", path, key)
			}
			task.Env[key] = value
		}
	}
	return task, nil
}

func processDuration(settings map[string]interface{}, key, path string, fallback time.Duration) (time.Duration, error) {
	if _, exists := settings[key]; !exists {
		return fallback, nil
	}
	value, err := processString(settings, key, path)
	if err != nil {
		return 0, err
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s.%s must be a positive Go duration, such as 10s", path, key)
	}
	return duration, nil
}

func decodeDevelopmentTask(raw interface{}, path string) (DevelopmentTaskConfig, error) {
	settings, err := processMap(raw, path, "name", "command", "cwd", "env", "timeout")
	if err != nil {
		return DevelopmentTaskConfig{}, err
	}
	task, err := processCommandFields(settings, path)
	if err != nil {
		return task, err
	}
	task.Timeout, err = processDuration(settings, "timeout", path, 30*time.Second)
	return task, err
}

func decodeDevelopmentService(raw interface{}, path string) (DevelopmentServiceConfig, error) {
	var service DevelopmentServiceConfig
	settings, err := processMap(raw, path, "name", "command", "cwd", "env", "ready", "stop_timeout")
	if err != nil {
		return service, err
	}
	common, err := processCommandFields(settings, path)
	if err != nil {
		return service, err
	}
	service.Name, service.Command, service.Cwd, service.Env = common.Name, common.Command, common.Cwd, common.Env
	if service.StopTimeout, err = processDuration(settings, "stop_timeout", path, 5*time.Second); err != nil {
		return service, err
	}
	ready, err := processMap(settings["ready"], path+".ready", "http", "timeout")
	if err != nil {
		return service, err
	}
	if service.Ready.HTTP, err = processString(ready, "http", path+".ready"); err != nil {
		return service, err
	}
	service.Ready.Timeout, err = processDuration(ready, "timeout", path+".ready", 10*time.Second)
	return service, err
}

// Generic package parsing intentionally materializes scalar values as strings.
// Retain strict literal types for process commands before that compatibility
// behavior can turn `false` or `123` into executable arguments/environment.
// Resolver mappings are checked after normal resolver evaluation by the decoder.
type developmentProcessesSourceError struct{ error }

// The legacy runtime loader tolerates resolver diagnostics elsewhere. Process
// declarations must retain required-input failures, including a resolver in a
// variable referenced by a hook/service, without changing that broader policy.
func validateDevelopmentProcessResolverDiagnostics(result *yamlparser.ConfigResult) error {
	var failures []yamlparser.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if strings.EqualFold(diagnostic.Level, "error") {
			failures = append(failures, diagnostic)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	paths := []string{"hyperbricks.hooks", developmentProcessesPath + ".hooks", developmentProcessesPath + ".services"}
	var source yaml.Node
	if err := yaml.Unmarshal([]byte(result.Preprocessed), &source); err != nil {
		return &developmentProcessesSourceError{err}
	}
	if len(source.Content) > 0 {
		root := source.Content[0]
		variables := developmentProcessSourceField(root, "vars")
		visited := map[string]bool{}
		var collectReferences func(*yaml.Node)
		collectReferences = func(node *yaml.Node) {
			if node == nil {
				return
			}
			if node.Kind == yaml.MappingNode && len(node.Content) == 2 && strings.TrimSpace(node.Content[0].Value) == "var" {
				spec := node.Content[1]
				name := spec.Value
				if spec.Kind == yaml.MappingNode {
					if field := developmentProcessSourceField(spec, "name"); field != nil {
						name = field.Value
					}
				}
				name = strings.TrimSpace(name)
				if name != "" && !visited[name] {
					visited[name] = true
					paths = append(paths, "vars."+name)
					value := variables
					for _, part := range strings.Split(name, ".") {
						value = developmentProcessSourceField(value, part)
					}
					collectReferences(value)
				}
			}
			for _, child := range node.Content {
				collectReferences(child)
			}
		}
		development := developmentProcessSourceField(developmentProcessSourceField(root, "hyperbricks"), "development")
		collectReferences(developmentProcessSourceField(developmentProcessSourceField(root, "hyperbricks"), "hooks"))
		collectReferences(developmentProcessSourceField(development, "hooks"))
		collectReferences(developmentProcessSourceField(development, "services"))
	}
	for _, diagnostic := range failures {
		for _, path := range paths {
			if diagnostic.Path == path || strings.HasPrefix(diagnostic.Path, path+".") || strings.HasPrefix(diagnostic.Path, path+"[") {
				return &developmentProcessesSourceError{fmt.Errorf("load development process configuration: %s at %s: %s", diagnostic.Code, diagnostic.Path, diagnostic.Message)}
			}
		}
	}
	return nil
}

func developmentProcessSourceField(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if strings.TrimSpace(node.Content[i].Value) == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func validateDevelopmentProcessSourceTypes(source string) error {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(source), &root); err != nil {
		return err
	}
	if len(root.Content) == 0 {
		return nil
	}
	lookup := developmentProcessSourceField
	development := lookup(lookup(root.Content[0], "hyperbricks"), "development")
	checkString := func(node *yaml.Node, path string) error {
		if node != nil && node.Kind == yaml.ScalarNode && node.Tag != "!!str" {
			return &developmentProcessesSourceError{fmt.Errorf("%s must be a string (quote numeric or boolean values)", path)}
		}
		return nil
	}
	checkEntries := func(list *yaml.Node, path string) error {
		if list == nil || list.Kind != yaml.SequenceNode {
			return nil // Structure and resolved types are checked by the decoder.
		}
		for i, entry := range list.Content {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			for _, field := range []string{"name", "cwd", "timeout", "stop_timeout"} {
				if err := checkString(lookup(entry, field), itemPath+"."+field); err != nil {
					return err
				}
			}
			if command := lookup(entry, "command"); command != nil && command.Kind == yaml.SequenceNode {
				for j, arg := range command.Content {
					if err := checkString(arg, fmt.Sprintf("%s.command[%d]", itemPath, j)); err != nil {
						return err
					}
				}
			}
			if env := lookup(entry, "env"); env != nil && env.Kind == yaml.MappingNode {
				for j := 0; j < len(env.Content); j += 2 {
					if err := checkString(env.Content[j], itemPath+".env key"); err != nil {
						return err
					}
					if err := checkString(env.Content[j+1], itemPath+".env."+env.Content[j].Value); err != nil {
						return err
					}
				}
			}
			ready := lookup(entry, "ready")
			for _, field := range []string{"http", "timeout"} {
				if err := checkString(lookup(ready, field), itemPath+".ready."+field); err != nil {
					return err
				}
			}
		}
		return nil
	}
	hooks := lookup(development, "hooks")
	for _, phase := range []string{"before_start", "after_start"} {
		if err := checkEntries(lookup(hooks, phase), developmentProcessesPath+".hooks."+phase); err != nil {
			return err
		}
	}
	for _, phase := range []string{"before_start", "after_start", "before_static", "after_static", "finish"} {
		if err := checkEntries(lookup(lookup(lookup(root.Content[0], "hyperbricks"), "hooks"), phase), "hyperbricks.hooks."+phase); err != nil {
			return err
		}
	}
	return checkEntries(lookup(development, "services"), developmentProcessesPath+".services")
}
