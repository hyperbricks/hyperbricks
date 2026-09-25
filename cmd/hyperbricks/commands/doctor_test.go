package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type doctorFileSnapshot struct {
	Mode    fs.FileMode
	ModTime time.Time
	Content []byte
	Dir     bool
}

type doctorCountingTransport struct {
	requests atomic.Int32
}

func (transport *doctorCountingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.requests.Add(1)
	return nil, fmt.Errorf("doctor attempted an HTTP request")
}

func writeDoctorFixture(t *testing.T, projectRoot, moduleName, runtimeVersion, extraPackage, source string) string {
	t.Helper()
	moduleRoot := filepath.Join(projectRoot, "modules", moduleName)
	for _, directory := range []string{"hyperbricks", "templates", "resources", "static"} {
		if err := os.MkdirAll(filepath.Join(moduleRoot, directory), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
	}
	packageYAML := fmt.Sprintf(`hyperbricks:
  mode: development
  metadata:
    module: %s
    moduleversion: "1.0.0"
    hyperbricks: %s
  development:
    dashboard:
      enabled: false
      credentials:
        user: doctor-user-secret
        password: doctor-password-secret
    frontend_editing:
      enabled: false
%s`, moduleName, runtimeVersion, extraPackage)
	if err := os.WriteFile(filepath.Join(moduleRoot, PackageConfigFileName), []byte(packageYAML), 0o640); err != nil {
		t.Fatalf("write package fixture: %v", err)
	}
	if source == "" {
		source = `page:
  - type: hypermedia
  - route: index
  - title: Doctor fixture
  - content:
      - type: text
      - value: Healthy
`
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "hyperbricks", "app.hyperbricks.yaml"), []byte(source), 0o640); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	return moduleRoot
}

func snapshotDoctorTree(t *testing.T, root string) map[string]doctorFileSnapshot {
	t.Helper()
	snapshot := map[string]doctorFileSnapshot{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		item := doctorFileSnapshot{Mode: info.Mode(), ModTime: info.ModTime(), Dir: entry.IsDir()}
		if !entry.IsDir() {
			item.Content, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		snapshot[filepath.ToSlash(relative)] = item
		return nil
	}); err != nil {
		t.Fatalf("snapshot fixture: %v", err)
	}
	return snapshot
}

func executeDoctorJSON(t *testing.T, projectRoot string, args ...string) (doctorReport, string, int) {
	t.Helper()
	t.Chdir(projectRoot)
	previousExit, previousExitCode := Exit, ExitCode
	previousRuntimeOptions := shared.GetRuntimeOptions()
	Exit, ExitCode = false, 0
	shared.SetRuntimeOptions(shared.RuntimeOptions{})
	t.Cleanup(func() {
		Exit, ExitCode = previousExit, previousExitCode
		shared.SetRuntimeOptions(previousRuntimeOptions)
	})

	command := NewDoctorCommand()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(append(args, "--json"))
	if err := command.Execute(); err != nil {
		t.Fatalf("doctor command: %v; stderr=%s", err, stderr.String())
	}
	var report doctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode doctor JSON: %v\n%s", err, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected doctor stderr: %s", stderr.String())
	}
	return report, stdout.String(), ExitCode
}

func doctorCheckByID(t *testing.T, report doctorReport, id string) doctorCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("doctor report has no check %q", id)
	return doctorCheck{}
}

func TestDoctorHealthyJSONIsCompleteReadOnlyAndRedacted(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	before := snapshotDoctorTree(t, moduleRoot)

	report, output, code := executeDoctorJSON(t, projectRoot, "--module", "demo")
	if code != 0 || report.Status != "healthy" {
		t.Fatalf("doctor result code=%d status=%s checks=%+v", code, report.Status, report.Checks)
	}
	if report.SchemaVersion != doctorSchemaVersion {
		t.Fatalf("doctor schema version=%d, want %d", report.SchemaVersion, doctorSchemaVersion)
	}
	if len(report.Checks) != len(doctorCheckDefinitions) || report.Summary.Passed != len(doctorCheckDefinitions) {
		t.Fatalf("doctor summary=%+v checks=%d, want %d passing checks", report.Summary, len(report.Checks), len(doctorCheckDefinitions))
	}
	for index, definition := range doctorCheckDefinitions {
		if report.Checks[index].ID != definition.ID {
			t.Fatalf("check[%d]=%q, want %q", index, report.Checks[index].ID, definition.ID)
		}
	}
	for _, secret := range []string{"doctor-user-secret", "doctor-password-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("doctor output leaked secret %q: %s", secret, output)
		}
	}
	if strings.Contains(output, "\x1b") {
		t.Fatalf("JSON contains ANSI output: %q", output)
	}
	after := snapshotDoctorTree(t, moduleRoot)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("doctor changed the module tree\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDoctorWarningsAreNonFatalUnlessStrict(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", "v0.0.0", "", "")

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 0 || report.Status != "warning" || report.Summary.Warnings == 0 {
		t.Fatalf("normal warning result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "metadata.runtime_version"); check.Status != doctorWarn || check.Hint == "" {
		t.Fatalf("runtime metadata check=%+v", check)
	}

	strictReport, _, strictCode := executeDoctorJSON(t, projectRoot, "-m", "demo", "--strict")
	if strictCode != 1 || strictReport.Status != "warning" || !strictReport.Strict {
		t.Fatalf("strict warning result code=%d report=%+v", strictCode, strictReport)
	}
}

func TestDoctorFailureStillEmitsEveryStableCheck(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	packagePath := filepath.Join(moduleRoot, PackageConfigFileName)
	content, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.Replace(content, []byte(`moduleversion: "1.0.0"`), []byte(`moduleversion: invalid`), 1)
	if err := os.WriteFile(packagePath, content, 0o640); err != nil {
		t.Fatal(err)
	}

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" || report.Summary.Failed == 0 {
		t.Fatalf("failure result code=%d report=%+v", code, report)
	}
	if len(report.Checks) != len(doctorCheckDefinitions) {
		t.Fatalf("failure report has %d checks, want %d", len(report.Checks), len(doctorCheckDefinitions))
	}
	if check := doctorCheckByID(t, report, "metadata.module_version"); check.Status != doctorFail {
		t.Fatalf("module version check=%+v", check)
	}
}

func TestDoctorRequiresSourceModuleVersion(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	packagePath := filepath.Join(moduleRoot, PackageConfigFileName)
	content, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.Replace(content, []byte("    moduleversion: \"1.0.0\"\n"), nil, 1)
	if err := os.WriteFile(packagePath, content, 0o640); err != nil {
		t.Fatal(err)
	}

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("missing module version result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "metadata.module_version")
	if check.Status != doctorFail || !strings.Contains(check.Message, "missing") || check.Hint == "" {
		t.Fatalf("module version check=%+v", check)
	}
}

func TestDoctorRejectsRuntimeUnsafePackageConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "  system:\n    metrics_watch_interval: eventually\n", "")

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("invalid duration result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "package.configuration")
	if check.Status != doctorFail || !strings.Contains(check.Message, "invalid duration") {
		t.Fatalf("package check=%+v", check)
	}
}

func TestDoctorAcceptsRelativeAndAbsoluteModuleDirectories(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	for _, selection := range []string{"./modules/demo", moduleRoot} {
		report, _, code := executeDoctorJSON(t, projectRoot, "-m", selection)
		if code != 0 || report.Status != "healthy" || report.Module.Name != "demo" {
			t.Fatalf("selection %q result code=%d report=%+v", selection, code, report)
		}
	}
}

func TestDoctorDefaultsToDefaultModule(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "default", strings.TrimSpace(assets.VersionMD), "", "")

	report, _, code := executeDoctorJSON(t, projectRoot)
	if code != 0 || report.Status != "healthy" {
		t.Fatalf("default selection result code=%d report=%+v", code, report)
	}
	if report.Module.Input != "default" || report.Module.Name != "default" || report.Module.Root != "modules/default" {
		t.Fatalf("default module selection=%+v", report.Module)
	}
}

func TestDoctorAcceptsNestedAlternateConfig(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	content, err := os.ReadFile(filepath.Join(moduleRoot, PackageConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(moduleRoot, "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const profileConfig = "profiles/live.hyperbricks.yaml"
	if err := os.WriteFile(filepath.Join(moduleRoot, filepath.FromSlash(profileConfig)), content, 0o640); err != nil {
		t.Fatal(err)
	}

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo", "--config", profileConfig)
	if code != 0 || report.Status != "healthy" || report.Module.Config != profileConfig {
		t.Fatalf("alternate config result code=%d report=%+v", code, report)
	}
}

func TestDoctorConfinesAlternateConfig(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo", "--config", "../outside.hyperbricks.yaml")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("escape result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "module.selection"); check.Status != doctorFail || !strings.Contains(check.Message, "inside the selected module") {
		t.Fatalf("selection check=%+v", check)
	}
	if len(report.Checks) != len(doctorCheckDefinitions) {
		t.Fatalf("escape report has %d checks, want %d", len(report.Checks), len(doctorCheckDefinitions))
	}
}

func TestDoctorRejectsConfigSymlinkBeforeStrictLoading(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	outsidePath := filepath.Join(projectRoot, "outside.hyperbricks.yaml")
	if err := os.WriteFile(outsidePath, []byte("hyperbricks: {metadata: {module: outside-secret, moduleversion: 1.0.0}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(moduleRoot, PackageConfigFileName)
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, configPath); err != nil {
		t.Skipf("cannot create symlink fixture: %v", err)
	}

	report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("symlink result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "package.configuration")
	if check.Status != doctorFail || !strings.Contains(check.Message, "symlink") {
		t.Fatalf("package check=%+v", check)
	}
	if strings.Contains(output, "outside-secret") {
		t.Fatalf("doctor exposed content from outside config: %s", output)
	}
}

func TestDoctorDoesNotContactConfiguredAPI(t *testing.T) {
	transport := &doctorCountingTransport{}
	previousDefaultTransport := http.DefaultTransport
	previousDefaultClient := http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport = previousDefaultTransport
		http.DefaultClient = previousDefaultClient
	})
	projectRoot := t.TempDir()
	source := fmt.Sprintf(`page:
  - type: hypermedia
  - route: index
  - title: Offline doctor
  - content:
      - type: api_render
      - endpoint: %s
      - method: GET
      - inline: '{{ .Data }}'
`, "https://doctor.invalid/data")
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", source)

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 0 || report.Status != "healthy" {
		t.Fatalf("offline API result code=%d report=%+v", code, report)
	}
	if got := transport.requests.Load(); got != 0 {
		t.Fatalf("doctor contacted configured API %d time(s)", got)
	}
}

func TestDoctorValidatesAPISecurityWithoutContactingEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		authYAML string
		want     string
	}{
		{name: "userinfo", endpoint: "https://user:password@example.test/data", want: "must not contain user information"},
		{name: "invalid port", endpoint: "https://example.test:70000/data", want: "endpoint URL contains an invalid port"},
		{name: "mixed authentication", endpoint: "https://example.test/data", authYAML: "  - username: api-user-secret\n  - password: api-password-secret\n  - jwtsecret: api-jwt-secret\n", want: "exactly one upstream authentication source"},
		{name: "partial basic authentication", endpoint: "https://example.test/data", authYAML: "  - username: api-user-secret\n", want: "requires both username and password"},
		{name: "credentials over HTTP", endpoint: "http://example.test/data", authYAML: "  - username: api-user-secret\n  - password: api-password-secret\n", want: "credentials require HTTPS"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			source := fmt.Sprintf(`upstream:
  - type: api_render
  - endpoint: %s
  - method: GET
  - inline: '{{.Data}}'
%s`, test.endpoint, test.authYAML)
			writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", source)

			report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
			if code != 1 || report.Status != "unhealthy" {
				t.Fatalf("API validation result code=%d report=%+v", code, report)
			}
			check := doctorCheckByID(t, report, "components.native_schema")
			if check.Status != doctorFail || !strings.Contains(check.Message, test.want) {
				t.Fatalf("native component check=%+v, want %q", check, test.want)
			}
			for _, secret := range []string{"api-user-secret", "api-password-secret", "api-jwt-secret"} {
				if strings.Contains(output, secret) {
					t.Fatalf("API diagnostic leaked %q: %s", secret, output)
				}
			}
		})
	}
}

func TestDoctorTreatsErrorLevelSourceDiagnosticsAsFailures(t *testing.T) {
	const environmentName = "HB_DOCTOR_REQUIRED_ENV_MUST_NOT_EXIST"
	previous, existed := os.LookupEnv(environmentName)
	if err := os.Unsetenv(environmentName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(environmentName, previous)
		} else {
			_ = os.Unsetenv(environmentName)
		}
	})
	projectRoot := t.TempDir()
	source := fmt.Sprintf(`page:
  - type: hypermedia
  - route: index
  - title:
      env:
        name: %s
        required: true
  - content:
      - type: text
      - value: Required environment fixture
`, environmentName)
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", source)

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("required environment result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "sources.graph")
	if check.Status != doctorFail || !strings.Contains(check.Message, "source error diagnostic") {
		t.Fatalf("source graph check=%+v", check)
	}
}

func TestDoctorSourceWarningsFollowStrictAcceptancePolicy(t *testing.T) {
	const environmentName = "HB_DOCTOR_OPTIONAL_ENV_MUST_NOT_EXIST"
	previous, existed := os.LookupEnv(environmentName)
	if err := os.Unsetenv(environmentName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(environmentName, previous)
		} else {
			_ = os.Unsetenv(environmentName)
		}
	})
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", fmt.Sprintf(`page:
  - type: hypermedia
  - route: index
  - title: {env: %s}
`, environmentName))

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 0 || report.Status != "warning" {
		t.Fatalf("source warning result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "sources.graph"); check.Status != doctorWarn || !strings.Contains(check.Message, "source diagnostic") {
		t.Fatalf("source graph check=%+v", check)
	}
	strictReport, _, strictCode := executeDoctorJSON(t, projectRoot, "-m", "demo", "--strict")
	if strictCode != 1 || strictReport.Status != "warning" || !strictReport.Strict {
		t.Fatalf("strict source warning result code=%d report=%+v", strictCode, strictReport)
	}
}

func TestDoctorReportsMissingConfiguredSourceDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", "")
	if err := os.RemoveAll(filepath.Join(moduleRoot, "hyperbricks")); err != nil {
		t.Fatal(err)
	}

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("missing directory result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "directories.paths"); check.Status != doctorFail || !strings.Contains(check.Message, "hyperbricks") {
		t.Fatalf("directory check=%+v", check)
	}
}

func TestDoctorRejectsDuplicateRoutes(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", `first:
  - type: hypermedia
  - route: shared
second:
  - type: hypermedia
  - route: shared
`)

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("duplicate route result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "routes.unique"); check.Status != doctorFail || !strings.Contains(check.Message, "same path") {
		t.Fatalf("route check=%+v", check)
	}
}

func TestDoctorValidatesRoutedEditableSpaceSource(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", `page:
  - type: hypermedia
  - route: index
  - title: Routed Space source
  - content:
      - type: template
      - inline: '<h1>{{.heading}}</h1>'
      - values:
          heading: Welcome
      - editable:
          heading: {type: text, label: Heading}
`)

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 0 || report.Status != "healthy" {
		t.Fatalf("routed Space source result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "spaces.contract")
	if check.Status != doctorPass || check.Message != "1 eligible editable Space sources validated" {
		t.Fatalf("Spaces check=%+v", check)
	}
}

func TestDoctorRejectsInvalidRoutedEditableSpaceSource(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", `page:
  - type: hypermedia
  - route: index
  - title: Invalid routed Space source
  - content:
      - type: template
      - inline: '<h1>{{.heading}}</h1>'
      - values:
          heading: Welcome
      - editable:
          heading: {type: unsupported}
`)

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("invalid routed Space source result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "spaces.contract")
	if check.Status != doctorFail || !strings.Contains(check.Message, "source page") || !strings.Contains(check.Message, "unsupported field type") {
		t.Fatalf("Spaces check=%+v", check)
	}
}

func TestDoctorDoesNotMutateRuntimeTemplateStore(t *testing.T) {
	previousTemplates := parser.GetTemplateStore()
	parser.ClearTemplateStore()
	parser.AddTemplate("existing-runtime-template", "unchanged")
	t.Cleanup(func() {
		parser.ClearTemplateStore()
		for name, content := range previousTemplates {
			parser.AddTemplate(name, content)
		}
	})
	before := parser.GetTemplateStore()

	projectRoot := t.TempDir()
	moduleRoot := writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", `snippet:
  - type: template
  - template:
      file: doctor-template-registry-probe.html
`)
	if err := os.WriteFile(filepath.Join(moduleRoot, "templates", "doctor-template-registry-probe.html"), []byte("<p>{{.value}}</p>\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 0 || report.Status != "healthy" {
		t.Fatalf("template fixture result code=%d report=%+v", code, report)
	}
	if after := parser.GetTemplateStore(); !reflect.DeepEqual(before, after) {
		t.Fatalf("doctor changed runtime template store\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDoctorResourceFailuresDoNotLeakAbsoluteModulePaths(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", `picture:
  - type: image
  - src:
      path: {base: resources, path: images/missing.png}
  - alt: Missing fixture
`)

	report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("missing resource result code=%d report=%+v", code, report)
	}
	check := doctorCheckByID(t, report, "resources.local")
	if check.Status != doctorFail || !strings.Contains(check.Message, "missing.png") {
		t.Fatalf("resource check=%+v", check)
	}
	if strings.Contains(output, projectRoot) {
		t.Fatalf("JSON leaked absolute project path: %s", output)
	}

	command := NewDoctorCommand()
	var human bytes.Buffer
	command.SetOut(&human)
	command.SetArgs([]string{"-m", "demo"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(human.String(), projectRoot) {
		t.Fatalf("human output leaked absolute project path: %s", human.String())
	}
}

func TestDoctorRedactsCredentialsResolvedIntoSourceDiagnostics(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", `source:
  - type: hypermedia
  - title: Credential redaction fixture
  - content:
      - type: template
      - inline: '{{.secret}}'
      - values:
          secret: visible content
      - editable:
          secret:
            type: {config: hyperbricks.development.dashboard.credentials.password}
`)

	report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("credential resolver result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "spaces.contract"); check.Status != doctorFail {
		t.Fatalf("Spaces check=%+v", check)
	}
	for _, secret := range []string{"doctor-user-secret", "doctor-password-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("doctor output leaked resolved credential %q: %s", secret, output)
		}
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("doctor did not mark the credential redaction: %s", output)
	}
}

func TestDoctorRedactsEnvironmentSecretsResolvedIntoSourceDiagnostics(t *testing.T) {
	const environmentName = "HB_DOCTOR_SOURCE_SECRET"
	const environmentSecret = "doctor-environment-secret"
	t.Setenv(environmentName, environmentSecret)
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), "", fmt.Sprintf(`source:
  - type: hypermedia
  - title: Environment redaction fixture
  - content:
      - type: template
      - inline: '{{.secret}}'
      - values:
          secret: visible content
      - editable:
          secret:
            type: {env: %s}
`, environmentName))

	report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("environment resolver result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "spaces.contract"); check.Status != doctorFail {
		t.Fatalf("Spaces check=%+v", check)
	}
	if strings.Contains(output, environmentSecret) || !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("doctor did not redact resolved environment secret: %s", output)
	}
}

func TestDoctorRedactsGenericConfigResolverValuesFromDiagnostics(t *testing.T) {
	const configSecret = "generic-config-api-token-secret"
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), `myconf:
  api_token: `+configSecret+`
`, `source:
  - type: hypermedia
  - title: Config resolver redaction fixture
  - content:
      - type: template
      - inline: '{{.secret}}'
      - values:
          secret: visible content
      - editable:
          secret:
            type: {config: myconf.api_token}
`)

	report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 1 || report.Status != "unhealthy" {
		t.Fatalf("config resolver result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "spaces.contract"); check.Status != doctorFail {
		t.Fatalf("Spaces check=%+v", check)
	}
	if strings.Contains(output, configSecret) || !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("doctor did not redact resolved config value: %s", output)
	}
}

func TestDoctorReportsUnknownTypesWithoutLoadingPluginArtifact(t *testing.T) {
	projectRoot := t.TempDir()
	pluginDir := filepath.Join(projectRoot, "bin", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "AcmePlugin@1.0.0.wasm"), []byte("not executable and never loaded"), 0o600); err != nil {
		t.Fatal(err)
	}
	extraPackage := `  plugins:
    enabled: [AcmePlugin@1.0.0]
  directories:
    plugins: ./bin/plugins
`
	source := `custom:
  - type: acme_widget
  - secret: plugin-source-secret
`
	writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), extraPackage, source)

	report, output, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
	if code != 0 || report.Status != "warning" {
		t.Fatalf("plugin-owned result code=%d report=%+v", code, report)
	}
	if check := doctorCheckByID(t, report, "components.plugin_owned"); check.Status != doctorWarn || !strings.Contains(check.Message, "may be plugin-owned") || !strings.Contains(check.Message, "acme_widget") {
		t.Fatalf("plugin-owned check=%+v", check)
	}
	if check := doctorCheckByID(t, report, "plugins.artifacts"); check.Status != doctorPass {
		t.Fatalf("plugin artifact check=%+v", check)
	}
	if strings.Contains(output, "plugin-source-secret") {
		t.Fatalf("doctor leaked plugin source content: %s", output)
	}
}

func TestDoctorRejectsMissingAndAmbiguousPluginArtifacts(t *testing.T) {
	for _, test := range []struct {
		name      string
		artifacts []string
		want      string
	}{
		{name: "missing", want: "not found"},
		{name: "ambiguous", artifacts: []string{"AcmePlugin@1.0.0.so", "AcmePlugin@1.0.0.wasm"}, want: "both native and wasm"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			pluginDir := filepath.Join(projectRoot, "bin", "plugins")
			if err := os.MkdirAll(pluginDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, artifact := range test.artifacts {
				if err := os.WriteFile(filepath.Join(pluginDir, artifact), []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			writeDoctorFixture(t, projectRoot, "demo", strings.TrimSpace(assets.VersionMD), `  plugins:
    enabled: [AcmePlugin@1.0.0]
  directories:
    plugins: ./bin/plugins
`, "")

			report, _, code := executeDoctorJSON(t, projectRoot, "-m", "demo")
			if code != 1 || report.Status != "unhealthy" {
				t.Fatalf("plugin artifact result code=%d report=%+v", code, report)
			}
			if check := doctorCheckByID(t, report, "plugins.artifacts"); check.Status != doctorFail || !strings.Contains(check.Message, test.want) {
				t.Fatalf("plugin check=%+v, want %q", check, test.want)
			}
		})
	}
}

func TestDoctorHumanReportIsCompactAndActionable(t *testing.T) {
	projectRoot := t.TempDir()
	writeDoctorFixture(t, projectRoot, "demo", "v0.0.0", "", "")
	t.Chdir(projectRoot)
	previousExit, previousExitCode := Exit, ExitCode
	Exit, ExitCode = false, 0
	t.Cleanup(func() { Exit, ExitCode = previousExit, previousExitCode })

	command := NewDoctorCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"-m", "demo"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, value := range []string{"HyperBricks Doctor · demo", "DIAGNOSIS · HEALTHY WITH WARNINGS", "PRESCRIPTION", "hyperbricks init -m demo --update-metadata"} {
		if !strings.Contains(output, value) {
			t.Fatalf("human output missing %q:\n%s", value, output)
		}
	}
	if strings.Contains(output, "doctor-password-secret") || strings.Contains(output, "\x1b") {
		t.Fatalf("human output leaked secret or ANSI: %q", output)
	}
	if strings.Contains(output, "module identity is") || strings.Contains(output, "source metadata contains stable fields") {
		t.Fatalf("human output did not collapse passing metadata checks: %q", output)
	}
}

func TestDoctorHumanReportEscapesTerminalControlCharacters(t *testing.T) {
	report := doctorReport{
		Status: "unhealthy",
		Module: doctorModule{
			Name:   "demo\x1b]0;owned\x07",
			Root:   "modules/demo\nspoofed",
			Config: "package\r.hyperbricks.yaml",
		},
		Summary: doctorSummary{Failed: 1},
		Checks: []doctorCheck{{
			ID:      "metadata.identity",
			Group:   "metadata",
			Status:  doctorFail,
			Message: "bad metadata\x1b[2J",
			File:    "source\nforged.hyperbricks.yaml",
			Hint:    "repair\x1b]8;;https://example.invalid\x07link",
		}},
	}
	command := NewDoctorCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	if err := writeDoctorReport(command, report); err != nil {
		t.Fatal(err)
	}

	text := output.String()
	for _, control := range []string{"\x1b", "\x07", "\r", "\x00"} {
		if strings.Contains(text, control) {
			t.Fatalf("human report contains terminal control %q: %q", control, text)
		}
	}
	for _, escaped := range []string{`\u001b`, `\u0007`, `\nspoofed`, `\r.hyperbricks.yaml`, `source\nforged`} {
		if !strings.Contains(text, escaped) {
			t.Fatalf("human report missing escaped value %q: %q", escaped, text)
		}
	}
}

func TestDoctorCredentialsFollowEnabledDeveloperSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name, mode                                         string
		dashboard, frontendEditing, spaces, externalEditor bool
		want                                               doctorCheckStatus
	}{
		{"Spaces disabled", shared.DEVELOPMENT_MODE, false, true, false, false, doctorPass},
		{"Spaces enabled", shared.DEVELOPMENT_MODE, false, true, true, false, doctorWarn},
		{"external editor", shared.DEVELOPMENT_MODE, false, true, false, true, doctorWarn},
		{"parent disabled", shared.DEVELOPMENT_MODE, false, false, true, true, doctorPass},
		{"Dashboard enabled", shared.DEVELOPMENT_MODE, true, true, false, false, doctorWarn},
		{"live mode", shared.LIVE_MODE, true, true, true, true, doctorPass},
		{"debug Dashboard", shared.DEBUG_MODE, true, false, false, false, doctorWarn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &shared.Config{Mode: tc.mode}
			config.Development.Dashboard.Enabled = tc.dashboard
			config.Development.FrontendEditing.Enabled = tc.frontendEditing
			config.Development.FrontendEditing.Spaces.Enabled = tc.spaces
			if tc.externalEditor {
				config.Development.FrontendEditing.Editors = map[string]shared.FrontendEditorConfig{"other": {Plugin: "Other@1", Route: "/__hyperbricks/other"}}
			}
			collector := newDoctorCollector()
			checkDoctorCredentials(collector, config)
			if got := collector.checks["security.developer_credentials"].Status; got != tc.want {
				t.Fatalf("credentials status = %s, want %s", got, tc.want)
			}
		})
	}
}
