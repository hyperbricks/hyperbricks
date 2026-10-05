package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/packagemetadata"
	"github.com/hyperbricks/hyperbricks/pkg/pluginruntime"
	"github.com/hyperbricks/hyperbricks/pkg/schema"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
	"github.com/hyperbricks/hyperbricks/pkg/spaces"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

const doctorSchemaVersion = 1

type doctorCheckStatus string

const (
	doctorPass doctorCheckStatus = "pass"
	doctorWarn doctorCheckStatus = "warn"
	doctorFail doctorCheckStatus = "fail"
	doctorSkip doctorCheckStatus = "skip"
)

type doctorCheck struct {
	ID      string            `json:"id"`
	Group   string            `json:"group"`
	Status  doctorCheckStatus `json:"status"`
	Message string            `json:"message"`
	File    string            `json:"file,omitempty"`
	Line    int               `json:"line,omitempty"`
	Column  int               `json:"column,omitempty"`
	Path    string            `json:"path,omitempty"`
	Hint    string            `json:"hint,omitempty"`
}

type doctorModule struct {
	Input  string `json:"input"`
	Name   string `json:"name,omitempty"`
	Root   string `json:"root,omitempty"`
	Config string `json:"config"`
}

type doctorSummary struct {
	Passed     int   `json:"passed"`
	Warnings   int   `json:"warnings"`
	Failed     int   `json:"failed"`
	Skipped    int   `json:"skipped"`
	DurationMS int64 `json:"duration_ms"`
}

type doctorReport struct {
	SchemaVersion int           `json:"schema_version"`
	Status        string        `json:"status"`
	Strict        bool          `json:"strict"`
	Module        doctorModule  `json:"module"`
	Summary       doctorSummary `json:"summary"`
	Checks        []doctorCheck `json:"checks"`
}

type doctorOptions struct {
	Module  string
	Config  string
	JSON    bool
	Verbose bool
	Strict  bool
}

type doctorCheckDefinition struct {
	ID    string
	Group string
}

var doctorCheckDefinitions = []doctorCheckDefinition{
	{ID: "module.selection", Group: "module"},
	{ID: "package.configuration", Group: "package"},
	{ID: "metadata.identity", Group: "metadata"},
	{ID: "metadata.module_version", Group: "metadata"},
	{ID: "metadata.runtime_version", Group: "metadata"},
	{ID: "metadata.source_fields", Group: "metadata"},
	{ID: "directories.paths", Group: "directories"},
	{ID: "sources.graph", Group: "sources"},
	{ID: "components.native_schema", Group: "components"},
	{ID: "components.plugin_owned", Group: "components"},
	{ID: "routes.unique", Group: "routes"},
	{ID: "resources.local", Group: "resources"},
	{ID: "spaces.contract", Group: "spaces"},
	{ID: "plugins.artifacts", Group: "plugins"},
	{ID: "security.developer_credentials", Group: "security"},
	{ID: "build.provenance", Group: "build"},
}

type doctorCollector struct {
	checks map[string]doctorCheck
}

func newDoctorCollector() *doctorCollector {
	return &doctorCollector{checks: make(map[string]doctorCheck, len(doctorCheckDefinitions))}
}

func (c *doctorCollector) set(check doctorCheck) {
	c.checks[check.ID] = check
}

func (c *doctorCollector) simple(id string, status doctorCheckStatus, message string) {
	for _, definition := range doctorCheckDefinitions {
		if definition.ID == id {
			c.set(doctorCheck{ID: id, Group: definition.Group, Status: status, Message: message})
			return
		}
	}
}

func (c *doctorCollector) ordered() []doctorCheck {
	checks := make([]doctorCheck, 0, len(doctorCheckDefinitions))
	for _, definition := range doctorCheckDefinitions {
		check, ok := c.checks[definition.ID]
		if !ok {
			check = doctorCheck{
				ID:      definition.ID,
				Group:   definition.Group,
				Status:  doctorSkip,
				Message: "not checked because a prerequisite failed",
			}
		}
		checks = append(checks, check)
	}
	return checks
}

// NewDoctorCommand creates the read-only module health command.
func NewDoctorCommand() *cobra.Command {
	opts := doctorOptions{Module: "default", Config: PackageConfigFileName}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose whether a source module is healthy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report := runDoctor(opts)
			Exit, ExitCode = true, 0
			if report.Summary.Failed > 0 || (opts.Strict && report.Summary.Warnings > 0) {
				ExitCode = 1
			}
			if opts.JSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetEscapeHTML(false)
				if err := encoder.Encode(report); err != nil {
					ExitCode = 1
					return err
				}
				return nil
			}
			return writeDoctorReport(cmd, report, opts.Verbose)
		},
	}
	cmd.Flags().StringVarP(&opts.Module, "module", "m", "default", "module name or directory path")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleSelection)
	cmd.Flags().StringVar(&opts.Config, "config", PackageConfigFileName, "package config path relative to the selected module")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "machine-readable output")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "show every check in categorized human-readable output")
	cmd.Flags().BoolVar(&opts.Strict, "strict", false, "treat warnings as an unhealthy result")
	cmd.MarkFlagsMutuallyExclusive("json", "verbose")
	return cmd
}

func runDoctor(opts doctorOptions) doctorReport {
	started := time.Now()
	collector := newDoctorCollector()
	report := doctorReport{
		SchemaVersion: doctorSchemaVersion,
		Strict:        opts.Strict,
		Module: doctorModule{
			Input:  opts.Module,
			Config: filepath.ToSlash(filepath.Clean(opts.Config)),
		},
	}

	cwd, err := os.Getwd()
	if err != nil {
		collector.simple("module.selection", doctorFail, "cannot resolve the current working directory")
		return finishDoctorReport(report, collector, started)
	}
	selection, err := resolveModuleSelection(opts.Module, cwd)
	if err != nil {
		collector.simple("module.selection", doctorFail, doctorCleanMessage(err.Error(), "", cwd))
		return finishDoctorReport(report, collector, started)
	}
	root, err := filepath.Abs(selection.Root)
	if err != nil {
		collector.simple("module.selection", doctorFail, "cannot resolve the selected module directory")
		return finishDoctorReport(report, collector, started)
	}
	root = filepath.Clean(root)
	report.Module.Name = selection.Name
	report.Module.Root = doctorDisplayPath(root, cwd)

	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		message := fmt.Sprintf("module directory is not readable: %s", report.Module.Root)
		collector.simple("module.selection", doctorFail, message)
		return finishDoctorReport(report, collector, started)
	}
	configPath, err := resolveModuleConfigPath(root, opts.Config)
	if err != nil {
		collector.simple("module.selection", doctorFail, doctorCleanMessage(err.Error(), root, cwd))
		return finishDoctorReport(report, collector, started)
	}
	configInfo, err := os.Stat(configPath)
	if err != nil || !configInfo.Mode().IsRegular() {
		collector.simple("module.selection", doctorFail, fmt.Sprintf("package configuration is not a readable regular file: %s", report.Module.Config))
		return finishDoctorReport(report, collector, started)
	}
	collector.simple("module.selection", doctorPass, fmt.Sprintf("selected %s", report.Module.Root))

	module, moduleErr := loadAuthoringModule(opts.Module, opts.Config)
	var config *shared.Config
	var configErr error
	if moduleErr == nil {
		// Use the authoring loader's symlink-safe paths so an alternate config is
		// proven to be module-local before the strict loader reads it.
		config, configErr = shared.LoadPackageConfigStrict(module.ConfigPath, module.Root)
	}
	if moduleErr != nil {
		collector.set(doctorCheck{ID: "package.configuration", Group: "package", Status: doctorFail, Message: doctorCleanMessage(moduleErr.Error(), root, cwd), File: report.Module.Config})
	} else if configErr != nil {
		collector.set(doctorCheck{ID: "package.configuration", Group: "package", Status: doctorFail, Message: doctorCleanMessage(configErr.Error(), root, cwd), File: report.Module.Config})
	} else {
		collector.simple("package.configuration", doctorPass, "configuration loaded and validated")
	}

	if module != nil {
		checkDoctorMetadata(collector, module.PackageBytes, selection.Name, strings.TrimSpace(assets.VersionMD), report.Module.Config)
	} else {
		for _, id := range []string{"metadata.identity", "metadata.module_version", "metadata.runtime_version", "metadata.source_fields", "build.provenance"} {
			collector.simple(id, doctorSkip, "package metadata is unavailable")
		}
	}

	if module != nil {
		checkDoctorDirectories(collector, module)
	} else {
		collector.simple("directories.paths", doctorSkip, "package directories are unavailable")
	}

	var graph doctorGraph
	if module != nil {
		graph, err = loadDoctorGraph(module)
	}
	if module == nil {
		collector.simple("sources.graph", doctorSkip, "source graph is unavailable")
	} else if err != nil {
		collector.set(doctorCheck{ID: "sources.graph", Group: "sources", Status: doctorFail, Message: doctorCleanMessage(err.Error(), root, cwd)})
	} else if len(graph.Diagnostics) > 0 {
		diagnostic := graph.Diagnostics[0]
		status := doctorWarn
		errorCount := 0
		for _, candidate := range graph.Diagnostics {
			if strings.EqualFold(candidate.Level, "error") {
				errorCount++
				if status != doctorFail {
					diagnostic = candidate
					status = doctorFail
				}
			}
		}
		diagnosticMessage := doctorCleanMessage(diagnostic.Message, root, cwd)
		message := fmt.Sprintf("%d source diagnostic(s); first: %s", len(graph.Diagnostics), diagnosticMessage)
		if errorCount > 0 {
			message = fmt.Sprintf("%d source error diagnostic(s); first: %s", errorCount, diagnosticMessage)
		}
		collector.set(doctorCheck{
			ID: "sources.graph", Group: "sources", Status: status, Message: message,
			File: doctorRelativePath(root, diagnostic.Source), Line: diagnostic.Line, Column: diagnostic.Column, Path: diagnostic.Path,
		})
	} else {
		collector.simple("sources.graph", doctorPass, fmt.Sprintf("%d files · %d roots · imports resolved", graph.FileCount, len(graph.Values)))
	}

	if module != nil && err == nil {
		plan, planErr := newScaffoldPlan(module)
		if planErr != nil {
			collector.simple("components.native_schema", doctorFail, doctorCleanMessage(planErr.Error(), root, cwd))
			collector.simple("resources.local", doctorSkip, "component source plan is unavailable")
		} else {
			checkDoctorComponents(collector, plan, graph, config)
			checkDoctorRoutes(collector, graph)
			checkDoctorSpaces(collector, graph)
		}
	} else {
		collector.simple("components.native_schema", doctorSkip, "source graph is unavailable")
		collector.simple("components.plugin_owned", doctorSkip, "source graph is unavailable")
		collector.simple("routes.unique", doctorSkip, "source graph is unavailable")
		collector.simple("resources.local", doctorSkip, "source graph is unavailable")
		collector.simple("spaces.contract", doctorSkip, "source graph is unavailable")
	}

	if config != nil {
		checkDoctorPlugins(collector, config, cwd, root)
		checkDoctorCredentials(collector, config)
	} else {
		collector.simple("plugins.artifacts", doctorSkip, "typed package configuration is unavailable")
		collector.simple("security.developer_credentials", doctorSkip, "typed package configuration is unavailable")
	}

	report = finishDoctorReport(report, collector, started)
	sensitiveValues := doctorCredentialValues(module)
	if module != nil {
		sensitiveValues = append(sensitiveValues, doctorEnvironmentValues(module.PackageBytes)...)
	}
	sensitiveValues = append(sensitiveValues, graph.SensitiveValues...)
	return redactDoctorReport(report, sensitiveValues)
}

func finishDoctorReport(report doctorReport, collector *doctorCollector, started time.Time) doctorReport {
	report.Checks = collector.ordered()
	for _, check := range report.Checks {
		switch check.Status {
		case doctorPass:
			report.Summary.Passed++
		case doctorWarn:
			report.Summary.Warnings++
		case doctorFail:
			report.Summary.Failed++
		case doctorSkip:
			report.Summary.Skipped++
		}
	}
	report.Summary.DurationMS = time.Since(started).Milliseconds()
	switch {
	case report.Summary.Failed > 0:
		report.Status = "unhealthy"
	case report.Summary.Warnings > 0:
		report.Status = "warning"
	default:
		report.Status = "healthy"
	}
	return report
}

func checkDoctorMetadata(collector *doctorCollector, content []byte, moduleName, runtimeVersion, configFile string) {
	result, err := packagemetadata.ReconcileSource(content, packagemetadata.ReconcileOptions{
		Module:                    moduleName,
		HyperBricks:               runtimeVersion,
		CanonicalizeModuleVersion: true,
	})
	if err != nil {
		message := doctorCleanMessage(err.Error(), "", "")
		if strings.Contains(message, "moduleversion") {
			collector.simple("metadata.identity", doctorSkip, "metadata could not be fully inspected")
			collector.set(doctorCheck{ID: "metadata.module_version", Group: "metadata", Status: doctorFail, Message: message, File: configFile, Path: "hyperbricks.metadata.moduleversion", Hint: fmt.Sprintf("Run hyperbricks init -m %s --update-metadata", moduleName)})
		} else {
			collector.set(doctorCheck{ID: "metadata.identity", Group: "metadata", Status: doctorFail, Message: message, File: configFile, Path: "hyperbricks.metadata"})
			collector.simple("metadata.module_version", doctorSkip, "metadata could not be fully inspected")
		}
		collector.simple("metadata.runtime_version", doctorSkip, "metadata could not be fully inspected")
		collector.simple("metadata.source_fields", doctorSkip, "metadata could not be fully inspected")
		collector.simple("build.provenance", doctorFail, "source metadata cannot receive build provenance")
		return
	}

	if result.Before.Module == moduleName {
		collector.simple("metadata.identity", doctorPass, fmt.Sprintf("module identity is %s", moduleName))
	} else {
		value := result.Before.Module
		if value == "" {
			value = "missing"
		}
		collector.set(doctorCheck{ID: "metadata.identity", Group: "metadata", Status: doctorFail, Message: fmt.Sprintf("metadata module is %s; selected directory is %s", value, moduleName), File: configFile, Path: "hyperbricks.metadata.module", Hint: fmt.Sprintf("Run hyperbricks init -m %s --update-metadata", moduleName)})
	}
	if strings.TrimSpace(result.Before.ModuleVersion) == "" {
		collector.set(doctorCheck{ID: "metadata.module_version", Group: "metadata", Status: doctorFail, Message: "metadata moduleversion is missing", File: configFile, Path: "hyperbricks.metadata.moduleversion", Hint: fmt.Sprintf("Run hyperbricks init -m %s --update-metadata", moduleName)})
	} else {
		collector.simple("metadata.module_version", doctorPass, fmt.Sprintf("module version %s is valid SemVer", result.Metadata.ModuleVersion))
	}

	recordedVersion := strings.TrimSpace(result.Before.HyperBricks)
	switch {
	case recordedVersion == runtimeVersion:
		collector.simple("metadata.runtime_version", doctorPass, fmt.Sprintf("records HyperBricks %s", runtimeVersion))
	case recordedVersion == "":
		collector.set(doctorCheck{ID: "metadata.runtime_version", Group: "metadata", Status: doctorWarn, Message: fmt.Sprintf("package does not record the running HyperBricks version %s", runtimeVersion), File: configFile, Path: "hyperbricks.metadata.hyperbricks", Hint: fmt.Sprintf("Run hyperbricks init -m %s --update-metadata", moduleName)})
	case doctorRecordedRuntimeIsOlder(recordedVersion, runtimeVersion):
		collector.simple("metadata.runtime_version", doctorPass, fmt.Sprintf("last updated with HyperBricks %s; running newer %s", recordedVersion, runtimeVersion))
	default:
		collector.set(doctorCheck{ID: "metadata.runtime_version", Group: "metadata", Status: doctorWarn, Message: fmt.Sprintf("package was last updated with HyperBricks %s; running %s", recordedVersion, runtimeVersion), File: configFile, Path: "hyperbricks.metadata.hyperbricks", Hint: fmt.Sprintf("Run hyperbricks init -m %s --update-metadata", moduleName)})
	}

	var stale []string
	for _, change := range result.Changes {
		if change.Removed || change.Field == "moduleversion" {
			stale = append(stale, change.Field)
		}
	}
	if len(stale) == 0 {
		collector.simple("metadata.source_fields", doctorPass, "source metadata contains stable fields only")
	} else {
		collector.set(doctorCheck{ID: "metadata.source_fields", Group: "metadata", Status: doctorWarn, Message: fmt.Sprintf("source metadata needs normalization: %s", strings.Join(stale, ", ")), File: configFile, Path: "hyperbricks.metadata", Hint: fmt.Sprintf("Run hyperbricks init -m %s --update-metadata", moduleName)})
	}

	_, err = packagemetadata.RenderArtifact(content, packagemetadata.ArtifactOptions{
		Module: moduleName, Format: "hra", FormatVersion: "1", Commit: packagemetadata.UnknownCommit,
		BuiltAt: "1970-01-01T00:00:00Z", HyperBricks: runtimeVersion,
	})
	if err != nil {
		collector.set(doctorCheck{ID: "build.provenance", Group: "build", Status: doctorFail, Message: err.Error(), File: configFile, Path: "hyperbricks.metadata"})
	} else {
		collector.simple("build.provenance", doctorPass, "archive provenance can be applied in memory")
	}
}

func doctorRecordedRuntimeIsOlder(recordedVersion, runtimeVersion string) bool {
	recorded, recordedErr := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(recordedVersion), "v"))
	runtime, runtimeErr := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(runtimeVersion), "v"))
	return recordedErr == nil && runtimeErr == nil && recorded.LessThan(runtime)
}

func checkDoctorDirectories(collector *doctorCollector, module *authoringModule) {
	missing := make([]string, 0)
	invalid := make([]string, 0)
	existing := 0
	for _, name := range []string{"hyperbricks", "templates", "resources", "static"} {
		path := module.Directories[name]
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			if name == "hyperbricks" {
				missing = append(missing, name)
			}
			continue
		}
		if err != nil || !info.IsDir() {
			invalid = append(invalid, name)
			continue
		}
		existing++
	}
	if len(missing) > 0 || len(invalid) > 0 {
		parts := make([]string, 0, 2)
		if len(missing) > 0 {
			parts = append(parts, "missing required directories: "+strings.Join(missing, ", "))
		}
		if len(invalid) > 0 {
			parts = append(parts, "unusable configured directories: "+strings.Join(invalid, ", "))
		}
		collector.simple("directories.paths", doctorFail, strings.Join(parts, "; "))
		return
	}
	collector.simple("directories.paths", doctorPass, fmt.Sprintf("%d configured source directories resolved safely", existing))
}

type doctorGraph struct {
	Values          map[string]map[string]interface{}
	Owners          map[string]string
	Diagnostics     []yamlparser.Diagnostic
	SensitiveValues []string
	FileCount       int
}

func loadDoctorGraph(module *authoringModule) (doctorGraph, error) {
	graph := doctorGraph{Values: map[string]map[string]interface{}{}, Owners: map[string]string{}}
	tops, err := filepath.Glob(filepath.Join(module.Directories["hyperbricks"], "*.hyperbricks.yaml"))
	if err != nil {
		return graph, err
	}
	sort.Strings(tops)
	if len(tops) == 0 {
		return graph, fmt.Errorf("no .hyperbricks.yaml files found in %s", module.Directories["hyperbricks"])
	}
	seen := map[string]bool{}
	opts := module.options()
	opts.AllowUnknownTypes = true
	// Keep graph materialization free of runtime registry mutations. Template
	// files are read and parsed explicitly by the resources check below.
	opts.TemplateDir = ""
	reader := func(path string) ([]byte, error) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(module.Directories["hyperbricks"], absolute)
		if err != nil {
			return nil, err
		}
		if _, err := authoringPath(module.Directories["hyperbricks"], filepath.ToSlash(relative)); err != nil {
			return nil, err
		}
		content, err := os.ReadFile(absolute)
		if err != nil {
			return nil, err
		}
		graph.SensitiveValues = append(graph.SensitiveValues, doctorEnvironmentValues(content)...)
		graph.SensitiveValues = append(graph.SensitiveValues, doctorConfigResolverValues(content, module.Config)...)
		document, err := yamlparser.ParseBytesWithOptions(content, yamlparser.ParseOptions{AllowUnknownTypes: true})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", absolute, err)
		}
		for _, root := range document.Roots {
			if previous := graph.Owners[root.Name]; previous != "" && previous != absolute {
				return nil, fmt.Errorf("duplicate root %q in %s and %s", root.Name, previous, absolute)
			}
			graph.Owners[root.Name] = absolute
		}
		seen[absolute] = true
		return content, nil
	}
	for _, top := range tops {
		document, err := yamlparser.LoadFileWithReader(top, opts, reader)
		if err != nil {
			return graph, err
		}
		values, diagnostics, err := document.MaterializeWithOptions(opts)
		if err != nil {
			return graph, err
		}
		graph.Diagnostics = append(graph.Diagnostics, document.Diagnostics...)
		graph.Diagnostics = append(graph.Diagnostics, diagnostics...)
		for name, value := range values {
			if object, ok := value.(map[string]interface{}); ok {
				graph.Values[name] = object
			}
		}
	}
	graph.FileCount = len(seen)
	return graph, nil
}

func checkDoctorComponents(collector *doctorCollector, plan *scaffoldPlan, graph doctorGraph, config *shared.Config) {
	unknown := map[string]bool{}
	nativeCount := 0
	mode := shared.LIVE_MODE
	if config != nil {
		mode = config.Mode
	}
	names := sortedDoctorRootNames(graph.Values)
	for _, name := range names {
		count, err := validateDoctorNativeObject(graph.Values[name], unknown, mode)
		nativeCount += count
		if err != nil {
			collector.set(doctorCheck{ID: "components.native_schema", Group: "components", Status: doctorFail, Message: fmt.Sprintf("%s: %v", name, err), File: doctorRelativePath(plan.module.Root, graph.Owners[name])})
			break
		}
	}
	if _, exists := collector.checks["components.native_schema"]; !exists {
		collector.simple("components.native_schema", doctorPass, fmt.Sprintf("%d native components validated", nativeCount))
	}

	unknownNames := make([]string, 0, len(unknown))
	for name := range unknown {
		unknownNames = append(unknownNames, name)
	}
	sort.Strings(unknownNames)
	if len(unknownNames) == 0 {
		collector.simple("components.plugin_owned", doctorPass, "no unverified unknown component types")
	} else if config != nil && len(config.Plugins.Enabled) > 0 {
		collector.simple("components.plugin_owned", doctorWarn, fmt.Sprintf("unknown component types may be plugin-owned and cannot be verified offline: %s", strings.Join(unknownNames, ", ")))
	} else {
		collector.simple("components.plugin_owned", doctorFail, fmt.Sprintf("unknown component types have no enabled plugin owner: %s", strings.Join(unknownNames, ", ")))
	}

	resourceCount := 0
	for _, name := range names {
		count, err := validateDoctorResources(plan, graph.Values[name], unknown)
		resourceCount += count
		if err != nil {
			message := doctorCleanMessage(err.Error(), plan.module.Root, "")
			collector.set(doctorCheck{ID: "resources.local", Group: "resources", Status: doctorFail, Message: fmt.Sprintf("%s: %s", name, message), File: doctorRelativePath(plan.module.Root, graph.Owners[name])})
			return
		}
	}
	collector.simple("resources.local", doctorPass, fmt.Sprintf("%d local resource references resolved", resourceCount))
}

func validateDoctorNativeObject(object map[string]interface{}, unknown map[string]bool, mode string) (int, error) {
	count := 0
	if token, ok := object["@type"].(string); ok {
		definition, err := scaffoldDefinition(token)
		if err != nil {
			unknown[strings.ToLower(strings.Trim(token, "<>"))] = true
			return 0, nil
		}
		count++
		kind := scaffoldTypeName(definition)
		factory := typefactory.NewTypeFactory()
		factory.RegisterType(token, definition.ConfigType)
		response, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: token, Data: object})
		if err != nil {
			return count, fmt.Errorf("%s: %w", kind, err)
		}
		if err := validateDoctorAPIInstance(response.Instance, mode); err != nil {
			return count, fmt.Errorf("%s: %w", kind, err)
		}
		for _, field := range schema.ExtractDefinition(definition).Fields {
			if !field.Required || strings.Contains(field.Path, ".") {
				continue
			}
			value, exists := object[field.Path]
			if !exists || value == nil || reflect.ValueOf(value).IsZero() {
				return count, fmt.Errorf("%s requires %s", kind, field.Path)
			}
		}
		if endpoint, ok := object["endpoint"].(string); ok {
			parsed, err := url.Parse(endpoint)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return count, fmt.Errorf("endpoint must be an explicit HTTP(S) URL")
			}
		}
	}
	for _, key := range sortedDoctorObjectKeys(object) {
		if key == "editable" || key == "data" {
			continue
		}
		value := object[key]
		if child, ok := value.(map[string]interface{}); ok {
			childCount, err := validateDoctorNativeObject(child, unknown, mode)
			count += childCount
			if err != nil {
				return count, err
			}
		}
	}
	return count, nil
}

func validateDoctorAPIInstance(instance interface{}, mode string) error {
	var endpoint string
	var values map[string]interface{}
	var settings apiutil.AuthSettings
	switch config := instance.(type) {
	case component.APIConfig:
		endpoint, values = config.Endpoint, config.Values
		settings = apiutil.AuthSettings{
			ForwardToken: config.ForwardToken, Headers: config.Headers,
			Username: config.Username, Password: config.Password,
			JwtSecret: config.JwtSecret, JwtClaims: config.JwtClaims,
			HasBody: config.Body != "",
		}
	case composite.ApiFragmentRenderConfig:
		endpoint, values = config.Endpoint, config.Values
		settings = apiutil.AuthSettings{
			ForwardToken: config.ForwardToken, Headers: config.Headers,
			Username: config.Username, Password: config.Password,
			JwtSecret: config.JwtSecret, JwtClaims: config.JwtClaims,
			HasBody: config.Body != "",
		}
	default:
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid API endpoint URL")
	}
	if err := apiutil.ValidateAuthSettings(parsed, mode, settings); err != nil {
		return err
	}
	for key := range values {
		if key == "Data" || key == "Status" {
			return fmt.Errorf("API values cannot override reserved Data or Status")
		}
	}
	return nil
}

func validateDoctorResources(plan *scaffoldPlan, object map[string]interface{}, unknown map[string]bool) (int, error) {
	count := 0
	if token, ok := object["@type"].(string); ok {
		definition, err := scaffoldDefinition(token)
		if err != nil {
			unknown[strings.ToLower(strings.Trim(token, "<>"))] = true
			return 0, nil
		}
		kind := scaffoldTypeName(definition)
		field := map[string]string{"json_render": "file", "styles": "file", "css": "file", "js": "file", "image": "src", "images": "directory", "esbuild": "entry"}[kind]
		if field != "" {
			if value, _ := object[field].(string); value != "" {
				count++
			}
		}
		if err := validateScaffoldFiles(plan, kind, object); err != nil {
			return count, err
		}
		if kind == "markdown" {
			if file, ok := object["file"].(string); ok && file != "" {
				count++
				if !strings.HasSuffix(file, ".md") && !strings.HasSuffix(file, ".markdown") {
					return count, fmt.Errorf("markdown file must end in .md or .markdown")
				}
				path, err := authoringPath(plan.module.Directories["resources"], file)
				if err != nil {
					return count, err
				}
				if _, err := plan.overlay(path); err != nil {
					return count, fmt.Errorf("markdown file: %w", err)
				}
			}
		}
		templateBody, _ := object["inline"].(string)
		if templateName, ok := object["template"].(string); ok && templateName != "" {
			count++
			path, err := authoringPath(plan.module.Directories["templates"], templateName)
			if err != nil {
				return count, err
			}
			body, err := plan.overlay(path)
			if err != nil {
				return count, fmt.Errorf("template file: %w", err)
			}
			if templateBody == "" {
				templateBody = string(body)
			}
		}
		switch kind {
		case "template", "api_render", "api_fragment_render", "json_render", "goja_render":
			if templateBody == "" {
				return count, fmt.Errorf("%s requires inline or template content", kind)
			}
			if _, err := shared.GenericTemplate().Parse(templateBody); err != nil {
				return count, fmt.Errorf("%s template: %w", kind, err)
			}
		}
	}
	for _, key := range sortedDoctorObjectKeys(object) {
		if key == "editable" || key == "data" {
			continue
		}
		value := object[key]
		if child, ok := value.(map[string]interface{}); ok {
			childCount, err := validateDoctorResources(plan, child, unknown)
			count += childCount
			if err != nil {
				return count, err
			}
		}
	}
	return count, nil
}

func checkDoctorRoutes(collector *doctorCollector, graph doctorGraph) {
	owners := map[string]string{}
	routeCount := 0
	hasIndex := false
	for _, name := range sortedDoctorRootNames(graph.Values) {
		object := graph.Values[name]
		token, _ := object["@type"].(string)
		definition, err := scaffoldDefinition(token)
		if err != nil || !scaffoldRouteOwner(scaffoldTypeName(definition)) {
			continue
		}
		raw, exists := object["route"]
		if !exists || raw == nil || raw == "" {
			continue
		}
		route, ok := raw.(string)
		if !ok {
			collector.simple("routes.unique", doctorFail, fmt.Sprintf("route on %s must be a string", name))
			return
		}
		if err := validateScaffoldRoute(route); err != nil {
			collector.simple("routes.unique", doctorFail, fmt.Sprintf("%s: %v", name, err))
			return
		}
		normalized := normalizeScaffoldRoute(route)
		if previous := owners[normalized]; previous != "" {
			collector.simple("routes.unique", doctorFail, fmt.Sprintf("the routes on %s and %s resolve to the same path", previous, name))
			return
		}
		owners[normalized] = name
		routeCount++
		if normalized == "" {
			hasIndex = true
		}
	}
	if routeCount > 0 && !hasIndex {
		collector.simple("routes.unique", doctorWarn, fmt.Sprintf("%d unique routes; no index route is configured", routeCount))
		return
	}
	collector.simple("routes.unique", doctorPass, fmt.Sprintf("%d unique routes", routeCount))
}

func checkDoctorSpaces(collector *doctorCollector, graph doctorGraph) {
	count := 0
	for _, name := range sortedDoctorRootNames(graph.Values) {
		object := graph.Values[name]
		if object["@type"] != "<HYPERMEDIA>" {
			continue
		}
		fields, err := spaces.SourceFields(object)
		if err != nil {
			collector.simple("spaces.contract", doctorFail, fmt.Sprintf("source %s: %v", name, err))
			return
		}
		if len(fields) > 0 {
			count++
		}
	}
	collector.simple("spaces.contract", doctorPass, fmt.Sprintf("%d eligible editable Space sources validated", count))
}

func checkDoctorPlugins(collector *doctorCollector, config *shared.Config, cwd, moduleRoot string) {
	if len(config.Plugins.Enabled) == 0 {
		collector.simple("plugins.artifacts", doctorPass, "no external plugins enabled")
		return
	}
	directory := strings.TrimSpace(config.Directories["plugins"])
	if directory == "" {
		directory = filepath.Join("bin", "plugins")
	}
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(cwd, directory)
	}
	for _, name := range config.Plugins.Enabled {
		if _, err := pluginruntime.ResolveArtifact(directory, name); err != nil {
			collector.simple("plugins.artifacts", doctorFail, doctorCleanMessage(err.Error(), moduleRoot, cwd))
			return
		}
	}
	collector.simple("plugins.artifacts", doctorPass, fmt.Sprintf("%d enabled plugin artifacts found", len(config.Plugins.Enabled)))
}

func checkDoctorCredentials(collector *doctorCollector, config *shared.Config) {
	credentials := config.Development.Dashboard.Credentials
	dashboardEnabled := (config.Mode == shared.DEVELOPMENT_MODE || config.Mode == shared.DEBUG_MODE) && config.Development.Dashboard.Enabled
	spacesEnabled := shared.SpacesAvailable(config, shared.RuntimeOptions{})
	frontendEditorsEnabled := config.Mode == shared.DEVELOPMENT_MODE && config.Development.FrontendEditing.Enabled &&
		len(config.Development.FrontendEditing.Editors) > 0
	surfaceEnabled := dashboardEnabled || spacesEnabled || frontendEditorsEnabled
	switch {
	case credentials.Complete():
		collector.simple("security.developer_credentials", doctorPass, "developer-interface credentials are configured")
	case credentials.Empty() && surfaceEnabled:
		messages := []string{}
		if dashboardEnabled {
			messages = append(messages, "Dashboard, Errors and diagnostics are accessible without login to anyone who can reach the server")
		}
		if spacesEnabled {
			messages = append(messages, fmt.Sprintf("Spaces and contextual editing are accessible without login through allowed hosts (write=%t)", config.Development.FrontendEditing.Spaces.Write))
		}
		if frontendEditorsEnabled {
			messages = append(messages, "frontend editor plugins remain locked because credentials are not configured")
		}
		collector.set(doctorCheck{ID: "security.developer_credentials", Group: "security", Status: doctorWarn, Message: strings.Join(messages, "; "), Path: "hyperbricks.development.dashboard.credentials", Hint: "Configure both development.dashboard.credentials.user and development.dashboard.credentials.password to require login"})
	case credentials.Empty():
		collector.simple("security.developer_credentials", doctorPass, "developer interfaces are disabled and credentials are not configured")
	default:
		collector.set(doctorCheck{ID: "security.developer_credentials", Group: "security", Status: doctorFail, Message: "developer-interface credentials require both user and password", Path: "hyperbricks.development.dashboard.credentials", Hint: "Configure both development.dashboard.credentials.user and development.dashboard.credentials.password"})
	}
}

func sortedDoctorRootNames(values map[string]map[string]interface{}) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedDoctorObjectKeys(object map[string]interface{}) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func doctorCredentialValues(module *authoringModule) []string {
	if module == nil {
		return nil
	}
	current := interface{}(module.Config)
	for _, key := range []string{"hyperbricks", "development", "dashboard", "credentials"} {
		mapping, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = mapping[key]
	}
	credentials, ok := current.(map[string]interface{})
	if !ok {
		return nil
	}
	values := make([]string, 0, 2)
	for _, key := range []string{"user", "password"} {
		if value, ok := credentials[key].(string); ok && value != "" {
			values = append(values, value)
		}
	}
	return values
}

func doctorConfigResolverValues(content []byte, config map[string]interface{}) []string {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil
	}
	values := make([]string, 0)
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		if node == nil {
			return
		}
		if node.Kind == yaml.MappingNode {
			for index := 0; index+1 < len(node.Content); index += 2 {
				key, value := node.Content[index], node.Content[index+1]
				if key.Value == "config" {
					path := ""
					switch value.Kind {
					case yaml.ScalarNode:
						path = strings.TrimSpace(value.Value)
					case yaml.MappingNode:
						for field := 0; field+1 < len(value.Content); field += 2 {
							if value.Content[field].Value == "path" && value.Content[field+1].Kind == yaml.ScalarNode {
								path = strings.TrimSpace(value.Content[field+1].Value)
								break
							}
						}
					}
					if resolved, ok := doctorLookupConfig(config, path); ok {
						values = appendDoctorStringValues(values, resolved)
					}
				}
				visit(value)
			}
			return
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	visit(&document)
	return values
}

func doctorLookupConfig(config map[string]interface{}, path string) (interface{}, bool) {
	if config == nil || strings.TrimSpace(path) == "" {
		return nil, false
	}
	current := interface{}(config)
	for _, part := range strings.Split(path, ".") {
		mapping, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = mapping[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func appendDoctorStringValues(values []string, value interface{}) []string {
	switch typed := value.(type) {
	case string:
		if typed != "" {
			values = append(values, typed)
		}
	case []interface{}:
		for _, item := range typed {
			values = appendDoctorStringValues(values, item)
		}
	case map[string]interface{}:
		for _, key := range sortedDoctorObjectKeys(typed) {
			values = appendDoctorStringValues(values, typed[key])
		}
	}
	return values
}

func doctorEnvironmentValues(content []byte) []string {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil
	}
	values := make([]string, 0)
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		if node == nil {
			return
		}
		if node.Kind == yaml.MappingNode {
			for index := 0; index+1 < len(node.Content); index += 2 {
				key, value := node.Content[index], node.Content[index+1]
				if key.Value == "env" {
					name := ""
					switch value.Kind {
					case yaml.ScalarNode:
						name = strings.TrimSpace(value.Value)
					case yaml.MappingNode:
						for field := 0; field+1 < len(value.Content); field += 2 {
							if value.Content[field].Value == "name" && value.Content[field+1].Kind == yaml.ScalarNode {
								name = strings.TrimSpace(value.Content[field+1].Value)
								break
							}
						}
					}
					if resolved, ok := os.LookupEnv(name); ok && resolved != "" {
						values = append(values, resolved)
					}
				}
				visit(value)
			}
			return
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	visit(&document)
	return values
}

func redactDoctorReport(report doctorReport, sensitiveValues []string) doctorReport {
	sensitiveValues = append([]string(nil), sensitiveValues...)
	sort.SliceStable(sensitiveValues, func(left, right int) bool {
		return len(sensitiveValues[left]) > len(sensitiveValues[right])
	})
	redact := func(value string) string {
		for _, sensitive := range sensitiveValues {
			if sensitive == "" {
				continue
			}
			value = strings.ReplaceAll(value, fmt.Sprintf("%q", sensitive), `"[REDACTED]"`)
			value = strings.ReplaceAll(value, "'"+sensitive+"'", "'[REDACTED]'")
			if len(sensitive) >= 4 {
				value = strings.ReplaceAll(value, sensitive, "[REDACTED]")
			} else {
				boundary := regexp.MustCompile(`(^|[^[:alnum:]_])` + regexp.QuoteMeta(sensitive) + `([^[:alnum:]_]|$)`)
				value = boundary.ReplaceAllString(value, `${1}[REDACTED]${2}`)
			}
		}
		return value
	}
	for index := range report.Checks {
		report.Checks[index].Message = redact(report.Checks[index].Message)
		report.Checks[index].Hint = redact(report.Checks[index].Hint)
		report.Checks[index].File = redact(report.Checks[index].File)
		report.Checks[index].Path = redact(report.Checks[index].Path)
	}
	return report
}

func doctorRelativePath(root, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}
	relative, err := filepath.Rel(root, absolute)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func doctorDisplayPath(path, cwd string) string {
	relative, err := filepath.Rel(cwd, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func doctorCleanMessage(message, root, cwd string) string {
	for _, prefix := range []string{root, cwd} {
		if strings.TrimSpace(prefix) == "" {
			continue
		}
		replacement := "."
		if prefix == root && root != "" {
			replacement = filepath.Base(root)
		}
		message = strings.ReplaceAll(message, filepath.Clean(prefix), replacement)
	}
	return filepath.ToSlash(message)
}

func writeDoctorReport(cmd *cobra.Command, report doctorReport, verbose bool) error {
	output := cmd.OutOrStdout()
	name := report.Module.Name
	if name == "" {
		name = report.Module.Input
	}
	fmt.Fprintf(output, "HyperBricks Doctor · %s\n", doctorTerminalText(name))
	if report.Module.Root != "" {
		fmt.Fprintf(output, "%s/%s\n", doctorTerminalText(strings.TrimSuffix(report.Module.Root, "/")), doctorTerminalText(report.Module.Config))
	}
	fmt.Fprintln(output)
	for start := 0; start < len(report.Checks); {
		end := start + 1
		for end < len(report.Checks) && report.Checks[end].Group == report.Checks[start].Group {
			end++
		}
		groupChecks := report.Checks[start:end]
		if verbose {
			writeDoctorVerboseGroup(output, groupChecks)
			start = end
			continue
		}
		nonPassing := make([]doctorCheck, 0, len(groupChecks))
		for _, check := range groupChecks {
			if check.Status != doctorPass {
				nonPassing = append(nonPassing, check)
			}
		}
		if len(nonPassing) == 0 {
			check := groupChecks[0]
			check.Message = doctorPassingGroupMessage(groupChecks)
			writeDoctorCheckLine(output, check)
		} else {
			for _, check := range nonPassing {
				writeDoctorCheckLine(output, check)
			}
		}
		start = end
	}
	fmt.Fprintln(output)
	diagnosis := strings.ToUpper(report.Status)
	if report.Status == "warning" {
		diagnosis = "HEALTHY WITH WARNINGS"
	}
	fmt.Fprintf(output, "DIAGNOSIS · %s\n", diagnosis)
	fmt.Fprintf(output, "%d passed · %d warning(s) · %d failed · %d skipped · %d ms\n",
		report.Summary.Passed, report.Summary.Warnings, report.Summary.Failed, report.Summary.Skipped, report.Summary.DurationMS)

	hints := make([]string, 0)
	seen := map[string]bool{}
	for _, check := range report.Checks {
		if check.Hint != "" && !seen[check.Hint] {
			seen[check.Hint] = true
			hints = append(hints, check.Hint)
		}
	}
	if len(hints) > 0 {
		fmt.Fprintln(output, "\nPRESCRIPTION")
		for _, hint := range hints {
			fmt.Fprintln(output, doctorTerminalText(hint))
		}
	}
	return nil
}

func writeDoctorVerboseGroup(output io.Writer, checks []doctorCheck) {
	if len(checks) == 0 {
		return
	}
	fmt.Fprintf(output, "%s %s\n", doctorStatusSymbol(doctorGroupStatus(checks)), doctorGroupTitle(checks[0].Group))
	for _, check := range checks {
		fmt.Fprintf(output, "  %s %-18s %s\n", doctorStatusSymbol(check.Status), doctorCheckTitle(check.ID), doctorTerminalText(check.Message))
		if check.File != "" {
			location := check.File
			if check.Line > 0 {
				location += fmt.Sprintf(":%d", check.Line)
				if check.Column > 0 {
					location += fmt.Sprintf(":%d", check.Column)
				}
			}
			fmt.Fprintf(output, "      File: %s\n", doctorTerminalText(location))
		}
		if check.Path != "" {
			fmt.Fprintf(output, "      Path: %s\n", doctorTerminalText(check.Path))
		}
	}
}

func doctorGroupStatus(checks []doctorCheck) doctorCheckStatus {
	status := doctorPass
	priority := map[doctorCheckStatus]int{doctorPass: 0, doctorSkip: 1, doctorWarn: 2, doctorFail: 3}
	for _, check := range checks {
		if priority[check.Status] > priority[status] {
			status = check.Status
		}
	}
	return status
}

func doctorCheckTitle(id string) string {
	name := id
	if separator := strings.IndexByte(name, '.'); separator >= 0 {
		name = name[separator+1:]
	}
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return "Check"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func doctorPassingGroupMessage(checks []doctorCheck) string {
	if len(checks) == 0 {
		return "checks passed"
	}
	switch checks[0].Group {
	case "metadata":
		return "identity, version, runtime, and source metadata validated"
	case "components":
		return checks[0].Message + " · no unverified unknown types"
	default:
		return checks[0].Message
	}
}

func writeDoctorCheckLine(output io.Writer, check doctorCheck) {
	fmt.Fprintf(output, "%s %-12s %s\n", doctorStatusSymbol(check.Status), doctorGroupTitle(check.Group), doctorTerminalText(check.Message))
	if check.File == "" || check.Status == doctorPass {
		return
	}
	location := check.File
	if check.Line > 0 {
		location += fmt.Sprintf(":%d", check.Line)
		if check.Column > 0 {
			location += fmt.Sprintf(":%d", check.Column)
		}
	}
	fmt.Fprintf(output, "  %s\n", doctorTerminalText(location))
}

func doctorStatusSymbol(status doctorCheckStatus) string {
	return map[doctorCheckStatus]string{doctorPass: "✓", doctorWarn: "!", doctorFail: "✗", doctorSkip: "–"}[status]
}

func doctorTerminalText(value string) string {
	var output strings.Builder
	for _, character := range value {
		switch character {
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
				fmt.Fprintf(&output, `\u%04x`, character)
				continue
			}
			output.WriteRune(character)
		}
	}
	return output.String()
}

func doctorGroupTitle(group string) string {
	if group == "" {
		return "Check"
	}
	return strings.ToUpper(group[:1]) + group[1:]
}
