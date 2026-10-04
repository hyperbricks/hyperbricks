package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDevelopmentProcessesDefaultsResolversAndLiteralArguments(t *testing.T) {
	t.Setenv("HB_PROCESS_TEST_GREETING", "hello from resolver")
	module := t.TempDir()
	config, err := ValidatePackageConfigBytes([]byte(`
hyperbricks:
  mode: live
  development:
    hooks:
      before_start:
        - name: prepare
          command: [definitely-not-installed, '$PORT', '~', '&&', '']
          cwd: {path: {base: module, path: directory-not-yet-present}}
          env:
            GREETING: {env: HB_PROCESS_TEST_GREETING}
            NUMBER: '123'
            BOOLEAN: 'true'
      after_start:
        - name: verify
          command: [sh, verify.sh]
          timeout: 2m
    services:
      - name: api
        command: [missing-api-executable]
        ready: {http: 'http://127.0.0.1:4319/health'}
      - name: second-api
        command: [missing-second-api]
        ready: {http: 'http://[::1]:4321/health', timeout: 3s}
        stop_timeout: 7s
`), module)
	if err != nil {
		t.Fatal(err)
	}
	if !config.HasDevelopmentProcesses() {
		t.Fatal("configured tasks/services not detected")
	}
	task := config.Development.Hooks.BeforeStart[0]
	if task.Cwd != filepath.Join(module, "directory-not-yet-present") {
		t.Fatalf("resolved cwd = %q", task.Cwd)
	}
	if task.Timeout != 30*time.Second || config.Development.Hooks.AfterStart[0].Timeout != 2*time.Minute {
		t.Fatal("hook timeout defaults/overrides were not retained")
	}
	if !reflect.DeepEqual(task.Command, []string{"definitely-not-installed", "$PORT", "~", "&&", ""}) {
		t.Fatalf("arguments changed: %#v", task.Command)
	}
	if !reflect.DeepEqual(task.Env, map[string]string{"GREETING": "hello from resolver", "NUMBER": "123", "BOOLEAN": "true"}) {
		t.Fatalf("env = %#v", task.Env)
	}
	first, second := config.Development.Services[0], config.Development.Services[1]
	if first.Ready.Timeout != 10*time.Second || first.StopTimeout != 5*time.Second || second.Ready.Timeout != 3*time.Second || second.StopTimeout != 7*time.Second {
		t.Fatalf("service defaults/overrides not retained: %#v", config.Development.Services)
	}
	if err := config.ValidateDevelopmentProcesses(); err != nil {
		t.Fatal(err)
	}
}

func TestDevelopmentProcessesInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, development, want string
	}{
		{"hooks scalar", "hooks: true", ".hooks must be a mapping"},
		{"hooks null", "hooks: null", ".hooks must be a mapping"},
		{"hooks unknown", "hooks: {pre: []}", ".hooks.pre is an unknown field"},
		{"task list", "hooks: {before_start: {name: prepare}}", ".hooks.before_start must be a list"},
		{"task list null", "hooks: {before_start: null}", ".hooks.before_start must be a list"},
		{"task entry null", "hooks: {before_start: [null]}", ".hooks.before_start[0] must be a mapping"},
		{"task scalar", "hooks: {before_start: [script.sh]}", ".hooks.before_start[0] must be a mapping"},
		{"task unknown", "hooks: {before_start: [{name: prepare, command: [sh], timout: 1s}]}", ".before_start[0].timout is an unknown field"},
		{"missing name", "hooks: {before_start: [{command: [sh]}]}", ".before_start[0].name"},
		{"blank name", "hooks: {before_start: [{name: ' ', command: [sh]}]}", ".before_start[0].name"},
		{"string command", "hooks: {before_start: [{name: prepare, command: 'sh prepare.sh'}]}", ".before_start[0].command"},
		{"empty command", "hooks: {before_start: [{name: prepare, command: []}]}", ".before_start[0].command"},
		{"empty executable", "hooks: {before_start: [{name: prepare, command: ['']}]}", ".before_start[0].command"},
		{"bool argument", "hooks: {before_start: [{name: prepare, command: [sh, false]}]}", ".before_start[0].command[1] must be a string"},
		{"numeric argument", "hooks: {after_start: [{name: verify, command: [sh, 123]}]}", ".after_start[0].command[1] must be a string"},
		{"null argument", "hooks: {before_start: [{name: prepare, command: [sh, null]}]}", ".before_start[0].command[1] must be a string"},
		{"mapping argument", "hooks: {before_start: [{name: prepare, command: [sh, {nested: value}]}]}", ".before_start[0].command[1] must be a string"},
		{"numeric name", "hooks: {before_start: [{name: 23, command: [sh]}]}", ".before_start[0].name must be a string"},
		{"numeric cwd", "hooks: {before_start: [{name: prepare, command: [sh], cwd: 4}]}", ".before_start[0].cwd must be a string"},
		{"numeric duration", "hooks: {before_start: [{name: prepare, command: [sh], timeout: 4}]}", ".before_start[0].timeout must be a string"},
		{"zero duration", "hooks: {before_start: [{name: prepare, command: [sh], timeout: 0s}]}", ".before_start[0].timeout must be a positive"},
		{"negative duration", "hooks: {before_start: [{name: prepare, command: [sh], timeout: -1s}]}", ".before_start[0].timeout must be a positive"},
		{"invalid duration", "hooks: {before_start: [{name: prepare, command: [sh], timeout: soon}]}", ".before_start[0].timeout must be a positive"},
		{"null duration", "hooks: {before_start: [{name: prepare, command: [sh], timeout: null}]}", ".before_start[0].timeout must be a string"},
		{"env scalar", "hooks: {before_start: [{name: prepare, command: [sh], env: x}]}", ".before_start[0].env must be a string mapping"},
		{"env null", "hooks: {before_start: [{name: prepare, command: [sh], env: null}]}", ".before_start[0].env must be a string mapping"},
		{"numeric env", "hooks: {before_start: [{name: prepare, command: [sh], env: {COUNT: 23}}]}", ".before_start[0].env.COUNT must be a string"},
		{"boolean env", "hooks: {before_start: [{name: prepare, command: [sh], env: {FLAG: true}}]}", ".before_start[0].env.FLAG must be a string"},
		{"null env", "hooks: {before_start: [{name: prepare, command: [sh], env: {EMPTY: null}}]}", ".before_start[0].env.EMPTY must be a string"},
		{"numeric env key", "hooks: {before_start: [{name: prepare, command: [sh], env: {23: value}}]}", ".before_start[0].env key must be a string"},
		{"invalid env key", "hooks: {before_start: [{name: prepare, command: [sh], env: {'A=B': value}}]}", ".before_start[0].env has an invalid"},
		{"reserved PATH", "hooks: {before_start: [{name: prepare, command: [sh], env: {PATH: /bin}}]}", ".before_start[0].env.PATH is reserved"},
		{"reserved executable", "hooks: {before_start: [{name: prepare, command: [sh], env: {HB_EXECUTABLE: /bin/sh}}]}", ".before_start[0].env.HB_EXECUTABLE is reserved"},
		{"reserved module", "hooks: {before_start: [{name: prepare, command: [sh], env: {HB_MODULE_ROOT: /tmp}}]}", ".before_start[0].env.HB_MODULE_ROOT is reserved"},
		{"reserved port", "hooks: {before_start: [{name: prepare, command: [sh], env: {HB_SERVER_PORT: '1234'}}]}", ".before_start[0].env.HB_SERVER_PORT is reserved"},
		{"duplicate phases", "hooks: {before_start: [{name: same, command: [sh]}], after_start: [{name: same, command: [sh]}]}", ".after_start[0].name \"same\" duplicates hyperbricks.development.hooks.before_start[0].name"},
		{"services scalar", "services: api", ".services must be a list"},
		{"services null", "services: null", ".services must be a list"},
		{"service entry null", "services: [null]", ".services[0] must be a mapping"},
		{"services unknown", "services: [{name: api, command: [api], restart: true}]", ".services[0].restart is an unknown field"},
		{"services ready missing", "services: [{name: api, command: [api]}]", ".services[0].ready must be a mapping"},
		{"services ready null", "services: [{name: api, command: [api], ready: null}]", ".services[0].ready must be a mapping"},
		{"services ready unknown", "services: [{name: api, command: [api], ready: {url: 'http://127.0.0.1'}}]", ".services[0].ready.url is an unknown field"},
		{"services http missing", "services: [{name: api, command: [api], ready: {}}]", ".services[0].ready.http"},
		{"services timeout zero", "services: [{name: api, command: [api], ready: {http: 'http://127.0.0.1', timeout: 0s}}]", ".services[0].ready.timeout must be a positive"},
		{"services stop zero", "services: [{name: api, command: [api], ready: {http: 'http://127.0.0.1'}, stop_timeout: 0s}]", ".services[0].stop_timeout must be a positive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidatePackageConfigBytes([]byte("hyperbricks: {development: {"+tc.development+"}}"), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), "hyperbricks.development") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want full process path and %q", err, tc.want)
			}
		})
	}
}

func TestDevelopmentProcessesDuplicateAcrossHooksAndServices(t *testing.T) {
	_, err := ValidatePackageConfigBytes([]byte(`
hyperbricks:
  development:
    hooks:
      before_start: [{name: same, command: [sh]}]
    services:
      - name: same
        command: [python3]
        ready: {http: 'http://127.0.0.1/health'}
`), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "services[0].name") || !strings.Contains(err.Error(), "before_start[0].name") {
		t.Fatalf("duplicate names error = %v", err)
	}
}

func TestDevelopmentServiceReadyURL(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1/health", "http://127.12.3.4:65535/health?ready=true", "http://[::1]:4319/health"} {
		t.Run(address, func(t *testing.T) {
			if err := validateDevelopmentReadyURL(address); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, address := range []string{"", "/health", "https://127.0.0.1/health", "http://localhost/health", "http://example.com/health", "http://192.168.0.1/health", "http://0.0.0.0/health", "http://[::]/health", "http://[::ffff:127.0.0.1]/health", "http://[::1%25lo0]/health", "http://user:pass@127.0.0.1/health", "http://@127.0.0.1/health", "http://127.0.0.1/health#fragment", "http://127.0.0.1/health#", "http://127.0.0.1:0/health", "http://127.0.0.1:65536/health", "http://127.0.0.1:/health", "http://127.0.0.1:invalid/health"} {
		t.Run(address, func(t *testing.T) {
			_, err := ValidatePackageConfigBytes([]byte(fmt.Sprintf(`hyperbricks: {development: {services: [{name: api, command: [python3], ready: {http: %q}}]}}`, address)), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), "hyperbricks.development.services[0].ready.http") {
				t.Fatalf("invalid ready URL error = %v", err)
			}
		})
	}
}

func TestDevelopmentProcessesSequentialDecodeAndValidationDoNotMutate(t *testing.T) {
	input := map[string]interface{}{"development": map[string]interface{}{"hooks": map[string]interface{}{"before_start": []interface{}{map[string]interface{}{"name": "prepare", "command": []interface{}{"sh"}}}}}}
	var config Config
	if err := decodeConfig(input, &config); err != nil {
		t.Fatal(err)
	}
	if !config.HasDevelopmentProcesses() || config.Development.Hooks.BeforeStart[0].Timeout != 30*time.Second {
		t.Fatal("expected initial task and default timeout")
	}
	// Defaulting belongs to decoding and must not modify raw resolver output.
	entry := input["development"].(map[string]interface{})["hooks"].(map[string]interface{})["before_start"].([]interface{})[0].(map[string]interface{})
	if _, exists := entry["timeout"]; exists {
		t.Fatal("decoder mutated input to add a default")
	}
	config.Development.Hooks.BeforeStart[0].Timeout = 0
	if err := config.ValidateDevelopmentProcesses(); err == nil || config.Development.Hooks.BeforeStart[0].Timeout != 0 {
		t.Fatal("validation must reject a zero duration without silently defaulting it")
	}
	if err := decodeConfig(map[string]interface{}{}, &config); err != nil {
		t.Fatal(err)
	}
	if config.HasDevelopmentProcesses() {
		t.Fatal("subsequent decode retained previous commands")
	}
}

func TestDevelopmentProcessesRuntimeLoaderPreservesErrors(t *testing.T) {
	for _, field := range []string{"env: {NUMBER: 123}", "command: sh", "timeout: 0s"} {
		t.Run(field, func(t *testing.T) {
			resetConfigurationForTest(t)
			module := t.TempDir()
			command := "command: [sh], "
			if strings.HasPrefix(field, "command:") {
				command = ""
			}
			path := writeStrictPackageConfig(t, module, "hyperbricks: {development: {hooks: {before_start: [{name: prepare, "+command+field+"}]}}}")
			Module = path
			SetRuntimeOptions(RuntimeOptions{ModuleRoot: module})
			config := GetHyperBricksConfiguration()
			if err := config.ValidateDevelopmentProcesses(); err == nil || !strings.Contains(err.Error(), "hyperbricks.development.hooks.before_start[0]") {
				t.Fatalf("runtime loader swallowed process error: %v", err)
			}
		})
	}
}

func TestDevelopmentProcessesEmptyConfiguration(t *testing.T) {
	for _, source := range []string{"hyperbricks: {}", "hyperbricks: {development: {hooks: {before_start: [], after_start: []}, services: []}}"} {
		config, err := ValidatePackageConfigBytes([]byte(source), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if config.HasDevelopmentProcesses() {
			t.Fatal("empty lists detected as commands")
		}
	}
}

func TestDevelopmentProcessesRuntimeRetainsRequiredResolverErrors(t *testing.T) {
	for _, tc := range []struct {
		name, variables, development, want string
	}{
		{"task environment", "", "hooks: {before_start: [{name: prepare, command: [sh], env: {REQUIRED_VALUE: {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}}}]}", "hyperbricks.development.hooks.before_start[0].env.REQUIRED_VALUE"},
		{"task argument", "", "hooks: {before_start: [{name: prepare, command: [sh, {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}]}]}", "hyperbricks.development.hooks.before_start[0].command[1]"},
		{"task cwd", "", "hooks: {before_start: [{name: prepare, command: [sh], cwd: {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}}]}", "hyperbricks.development.hooks.before_start[0].cwd"},
		{"after start", "", "hooks: {after_start: [{name: verify, command: [sh], env: {REQUIRED_VALUE: {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}}}]}", "hyperbricks.development.hooks.after_start[0].env.REQUIRED_VALUE"},
		{"service environment", "", "services: [{name: api, command: [python3], env: {REQUIRED_VALUE: {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}}, ready: {http: 'http://127.0.0.1'}}]", "hyperbricks.development.services[0].env.REQUIRED_VALUE"},
		{"variable dependency", "vars: {required_value: {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}}\n", "hooks: {before_start: [{name: prepare, command: [sh], env: {REQUIRED_VALUE: {var: required_value}}}]}", "vars.required_value"},
		{"nested variable dependency", "vars: {first: {var: options.value}, options: {value: {env: {name: HB_PROCESS_REVIEW_MISSING, required: true}}}}\n", "hooks: {before_start: [{name: prepare, command: [sh], env: {REQUIRED_VALUE: {var: {name: first}}}}]}", "vars.options.value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetConfigurationForTest(t)
			// Explicitly remove this test-only variable while preserving caller state.
			value, present := os.LookupEnv("HB_PROCESS_REVIEW_MISSING")
			os.Unsetenv("HB_PROCESS_REVIEW_MISSING")
			t.Cleanup(func() {
				if present {
					os.Setenv("HB_PROCESS_REVIEW_MISSING", value)
				}
			})
			module := t.TempDir()
			body := tc.variables + "hyperbricks: {mode: development, development: {" + tc.development + "}}"
			Module = writeStrictPackageConfig(t, module, body)
			SetRuntimeOptions(RuntimeOptions{ModuleRoot: module})
			config := GetHyperBricksConfiguration()
			err := config.ValidateDevelopmentProcesses()
			if err == nil || !strings.Contains(err.Error(), "env_missing at "+tc.want+":") {
				t.Fatalf("required process input lost its diagnostic/source path: %v", err)
			}
		})
	}
}

func TestDevelopmentProcessesRuntimeResolverChecksStayScoped(t *testing.T) {
	resetConfigurationForTest(t)
	module := t.TempDir()
	Module = writeStrictPackageConfig(t, module, `
vars:
  unrelated: {env: {name: HB_PROCESS_SCOPED_MISSING, required: true}}
hyperbricks:
  development:
    hooks:
      before_start: [{name: prepare, command: [sh]}]
  unrelated: {var: unrelated}
`)
	SetRuntimeOptions(RuntimeOptions{ModuleRoot: module})
	config := GetHyperBricksConfiguration()
	if err := config.ValidateDevelopmentProcesses(); err != nil || !config.HasDevelopmentProcesses() {
		t.Fatalf("unrelated package resolver changed process validation: %v", err)
	}
}
