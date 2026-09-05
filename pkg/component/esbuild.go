package component

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const EsbuildPreparedKey = "@esbuild_prepared"

type EsbuildConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string            `mapstructure:"@doc" description:"Build JavaScript, TypeScript, or CSS with embedded esbuild on first use, or on every render when cache is false."`
	Entry              string            `mapstructure:"entry" validate:"required" description:"Source filename. Use path with an explicit resources base."`
	Outfile            string            `mapstructure:"outfile" validate:"required" description:"Output filename inside the configured static directory. Use an explicit static path base."`
	Binary             string            `mapstructure:"binary" description:"Optional external esbuild executable; empty uses the embedded Go API."`
	Minify             bool              `mapstructure:"minify" description:"Minify whitespace and syntax. Default false."`
	MinifyIdentifiers  bool              `mapstructure:"minify_identifiers" description:"Minify identifiers independently of whitespace/syntax. Default false. YAML also accepts the legacy minifyident alias."`
	Mangle             bool              `mapstructure:"mangle" description:"Advanced: mangle JavaScript properties using .*; may break external property contracts. Default false; not allowed for CSS-only entries."`
	Sourcemap          bool              `mapstructure:"sourcemap" description:"Emit a linked source map. Default false."`
	Debug              bool              `mapstructure:"debug" description:"Log effective build options, engine, and cache diagnostics."`
	Cache              bool              `mapstructure:"cache" description:"True reuses valid builds; false rebuilds on every component render. Default false. Independent of page caching."`
	Fingerprint        bool              `mapstructure:"fingerprint" description:"Emit content-versioned JS/CSS filenames in the configured output directory. Default false. Old assets are retained."`
	Target             []string          `mapstructure:"target" description:"Optional browser/language targets, e.g. chrome110, safari16, es2020."`
	Loader             map[string]string `mapstructure:"loader" description:"Extension loader overrides, e.g. .woff2: file or .png: dataurl."`
	External           []string          `mapstructure:"external" description:"Import or asset URL patterns to leave unbundled, e.g. /static/vendor/*."`
	Prepared           interface{}       `mapstructure:"@esbuild_prepared" json:"-" exclude:"true"`
}

type EsbuildRenderer struct{ store *esbuildStore }

var _ shared.ComponentRenderer = (*EsbuildRenderer)(nil)

func EsbuildConfigGetName() string         { return "<ESBUILD>" }
func (r *EsbuildRenderer) Types() []string { return []string{EsbuildConfigGetName()} }

func NewEsbuildRenderer(staticDir string) *EsbuildRenderer {
	cacheDir, err := os.UserCacheDir()
	if err == nil {
		cacheDir = filepath.Join(cacheDir, "hyperbricks", "esbuild")
	}
	return &EsbuildRenderer{store: &esbuildStore{staticDir: staticDir, cacheDir: cacheDir, cacheDirErr: err, reuseDisk: true, results: make(map[string]*esbuildResult), generated: make(map[string]bool), owners: make(map[string]string)}}
}

// Invalidate preserves the publication lock across reloads and in-flight requests.
func (r *EsbuildRenderer) Invalidate() {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	// The loader also calls Invalidate before its first preparation. Only a real
	// reload must bypass disk reuse, including untracked resolution changes.
	if r.store.prepared {
		r.store.reuseDisk = false
	}
	r.store.results = make(map[string]*esbuildResult)
	r.store.owners = make(map[string]string)
}

type PreparedEsbuild struct {
	component shared.Component
	store     *esbuildStore
	spec      esbuildSpec
	key       string
	cache     bool
	debug     bool
	err       error
}

type esbuildSpec struct {
	Entry, Outfile, Binary, WorkingDir, StaticDir string
	Minify, MinifyIdentifiers, Mangle, Sourcemap  bool
	Fingerprint                                   bool
	Target                                        []string
	Loader                                        map[string]string
	External                                      []string
}

func (r *EsbuildRenderer) Prepare(config EsbuildConfig) *PreparedEsbuild {
	if r.store != nil {
		r.store.mu.Lock()
		r.store.prepared = true
		r.store.mu.Unlock()
	}
	p := &PreparedEsbuild{component: config.Component, store: r.store, cache: config.Cache, debug: config.Debug}
	p.err = p.prepare(config)
	return p
}

func (p *PreparedEsbuild) prepare(config EsbuildConfig) error {
	if p.store == nil {
		return fmt.Errorf("esbuild renderer has no module build store")
	}
	if strings.TrimSpace(config.Entry) == "" || strings.TrimSpace(config.Outfile) == "" {
		return fmt.Errorf("esbuild requires entry and outfile filenames (use path.base, not file.base)")
	}
	var err error
	s := esbuildSpec{Minify: config.Minify, MinifyIdentifiers: config.MinifyIdentifiers, Mangle: config.Mangle, Sourcemap: config.Sourcemap, Fingerprint: config.Fingerprint}
	if s.WorkingDir, err = os.Getwd(); err != nil {
		return err
	}
	if s.Entry, err = filepath.Abs(config.Entry); err != nil {
		return err
	}
	if s.Outfile, err = filepath.Abs(config.Outfile); err != nil {
		return err
	}
	if strings.TrimSpace(p.store.staticDir) == "" {
		return fmt.Errorf("esbuild requires a configured static directory")
	}
	if s.StaticDir, err = filepath.Abs(p.store.staticDir); err != nil {
		return err
	}
	if err := esbuildOutputPath(s.StaticDir, s.Outfile); err != nil {
		return err
	}
	if s.Entry == s.Outfile {
		return fmt.Errorf("esbuild entry and outfile must differ")
	}
	if filepath.Ext(s.Outfile) != ".js" && filepath.Ext(s.Outfile) != ".css" {
		return fmt.Errorf("esbuild outfile must end in .js or .css")
	}
	if filepath.Ext(s.Entry) == ".css" && (s.Mangle || filepath.Ext(s.Outfile) != ".css") {
		return fmt.Errorf("CSS entries require a .css outfile and mangle: false")
	}
	if config.Binary != "" {
		if s.Binary, err = exec.LookPath(config.Binary); err != nil {
			return fmt.Errorf("esbuild binary: %w", err)
		}
		if s.Binary, err = filepath.Abs(s.Binary); err != nil {
			return err
		}
	}
	s.Target = append([]string(nil), config.Target...)
	s.External = append([]string(nil), config.External...)
	s.Loader = make(map[string]string, len(config.Loader))
	for ext, loader := range config.Loader {
		s.Loader[ext] = loader
	}
	if _, err := s.options(); err != nil {
		return err
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		return err
	}
	p.key = fmt.Sprintf("%x", sha256.Sum256(encoded))
	p.spec = s
	return nil
}

func (p *PreparedEsbuild) Err() error {
	if p == nil {
		return fmt.Errorf("esbuild component was not prepared during module loading")
	}
	return p.err
}

// BuildIdentity excludes presentation and caching policy, so inherited wrappers
// may share output without changing where or how the HTML is rendered.
func (p *PreparedEsbuild) BuildIdentity() (string, string) { return p.spec.Outfile, p.key }

// RejectOutputConflict is called by the loader before publishing the snapshot.
func (p *PreparedEsbuild) RejectOutputConflict() {
	p.err = fmt.Errorf("conflicting esbuild configurations target %s; give each build a separate outfile", p.spec.Outfile)
}

func (r *EsbuildRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	config, ok := instance.(EsbuildConfig)
	if !ok {
		return "", []error{fmt.Errorf("invalid esbuild config: %T", instance)}
	}
	p, _ := config.Prepared.(*PreparedEsbuild)
	return p.Render(ctx)
}

func (p *PreparedEsbuild) Render(ctx context.Context) (string, []error) {
	if err := p.Err(); err != nil {
		return "", []error{p.componentError(err)}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	output, err := p.store.build(ctx, p)
	if err != nil {
		return "", []error{p.componentError(err)}
	}
	rel, _ := filepath.Rel(p.spec.StaticDir, output)
	publicURL := (&url.URL{Path: "/static/" + filepath.ToSlash(rel)}).EscapedPath()
	if p.component.Enclose == "" {
		return publicURL, nil
	}
	return shared.EncloseContent(p.component.Enclose, html.EscapeString(publicURL)), nil
}

func (p *PreparedEsbuild) componentError(err error) error {
	var config shared.Component
	if p != nil {
		config = p.component
	}
	return shared.ComponentError{Hash: shared.GenerateHash(), Type: EsbuildConfigGetName(), Rejected: true, Err: err.Error(), Path: config.HyperBricksPath, Key: config.HyperBricksKey}
}

var esbuildTargetPattern = regexp.MustCompile(`^(chrome|edge|firefox|safari|ios|node|opera|ie)([0-9]+(?:\.[0-9]+){0,2})$`)

func (s esbuildSpec) options() (api.BuildOptions, error) {
	o := api.BuildOptions{EntryPoints: []string{s.Entry}, Outfile: s.Outfile, AbsWorkingDir: s.WorkingDir, Bundle: true, Write: false, Metafile: true, LogLevel: api.LogLevelSilent,
		MinifyWhitespace: s.Minify, MinifySyntax: s.Minify, MinifyIdentifiers: s.MinifyIdentifiers}
	if s.Fingerprint {
		o.EntryPoints = nil
		o.EntryPointsAdvanced = []api.EntryPoint{{InputPath: s.Entry, OutputPath: s.outputStem()}}
		o.Outfile = ""
		o.Outdir = filepath.Dir(s.Outfile)
		o.EntryNames = "[name].[hash]"
	}
	if s.Mangle {
		o.MangleProps = ".*"
	}
	if s.Sourcemap {
		o.Sourcemap = api.SourceMapLinked
	}
	targets := map[string]api.Target{"esnext": api.ESNext, "es5": api.ES5, "es6": api.ES2015, "es2015": api.ES2015, "es2016": api.ES2016, "es2017": api.ES2017, "es2018": api.ES2018, "es2019": api.ES2019, "es2020": api.ES2020, "es2021": api.ES2021, "es2022": api.ES2022, "es2023": api.ES2023, "es2024": api.ES2024}
	engines := map[string]api.EngineName{"chrome": api.EngineChrome, "edge": api.EngineEdge, "firefox": api.EngineFirefox, "safari": api.EngineSafari, "ios": api.EngineIOS, "node": api.EngineNode, "opera": api.EngineOpera, "ie": api.EngineIE}
	seen := make(map[string]bool)
	for _, target := range s.Target {
		if value, ok := targets[target]; ok {
			if seen["language"] {
				return o, fmt.Errorf("esbuild target contains multiple language targets")
			}
			seen["language"] = true
			o.Target = value
			continue
		}
		parts := esbuildTargetPattern.FindStringSubmatch(target)
		if parts == nil || seen[parts[1]] {
			return o, fmt.Errorf("invalid or duplicate esbuild target %q", target)
		}
		seen[parts[1]] = true
		o.Engines = append(o.Engines, api.Engine{Name: engines[parts[1]], Version: parts[2]})
	}
	loaders := map[string]api.Loader{"file": api.LoaderFile, "dataurl": api.LoaderDataURL, "text": api.LoaderText, "binary": api.LoaderBinary, "base64": api.LoaderBase64, "json": api.LoaderJSON, "css": api.LoaderCSS, "js": api.LoaderJS, "jsx": api.LoaderJSX, "ts": api.LoaderTS, "tsx": api.LoaderTSX}
	o.Loader = make(map[string]api.Loader, len(s.Loader))
	for ext, name := range s.Loader {
		loader, ok := loaders[name]
		if !ok || !strings.HasPrefix(ext, ".") || len(ext) < 2 || strings.ContainsAny(ext, `/\`) {
			return o, fmt.Errorf("invalid esbuild loader %q: %q", ext, name)
		}
		o.Loader[ext] = loader
	}
	for _, pattern := range s.External {
		if strings.TrimSpace(pattern) == "" || strings.Count(pattern, "*") > 1 {
			return o, fmt.Errorf("invalid esbuild external pattern %q", pattern)
		}
	}
	o.External = append([]string(nil), s.External...)
	return o, nil
}

type esbuildStore struct {
	mu          sync.Mutex
	staticDir   string
	cacheDir    string
	cacheDirErr error
	reuseDisk   bool
	prepared    bool
	results     map[string]*esbuildResult
	generated   map[string]bool
	owners      map[string]string
	buildCount  uint64
}

func esbuildOutputPath(root, output string) error {
	rel, err := filepath.Rel(root, output)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("esbuild output %q must be inside configured static directory %q", output, root)
	}
	// Resolve existing ancestors too: a symlink inside static must not redirect writes outside it.
	resolvedRoot, err := esbuildExistingPath(root)
	if err != nil {
		return err
	}
	resolvedOutput, err := esbuildExistingPath(output)
	if err != nil {
		return err
	}
	rel, err = filepath.Rel(resolvedRoot, resolvedOutput)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("esbuild output follows a symlink outside static: %s", output)
	}
	return nil
}

func esbuildExistingPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = esbuildExistingPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}
