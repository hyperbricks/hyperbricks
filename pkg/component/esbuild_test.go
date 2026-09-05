package component

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func esbuildFixture(t *testing.T) (*EsbuildRenderer, EsbuildConfig) {
	t.Helper()
	root := t.TempDir()
	entry := filepath.Join(root, "custom resources", "main.js")
	esbuildWrite(t, entry, `import {message} from "./message.js"; document.body.dataset.message = message;`)
	esbuildWrite(t, filepath.Join(filepath.Dir(entry), "message.js"), `export const message = "first";`)
	static := filepath.Join(root, "custom public")
	r := NewEsbuildRenderer(static)
	r.store.cacheDir = t.TempDir()
	return r, EsbuildConfig{Entry: entry, Outfile: filepath.Join(static, "js", "app.js"), Cache: true,
		Component: shared.Component{Enclose: `<script src="|" defer></script>`}}
}

func esbuildWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func esbuildRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func esbuildRender(t *testing.T, p *PreparedEsbuild) string {
	t.Helper()
	output, errs := p.Render(context.Background())
	if len(errs) != 0 {
		t.Fatalf("render: %v", errs)
	}
	return output
}

func TestEsbuildLazyCacheConcurrentAndInvalidation(t *testing.T) {
	r, cfg := esbuildFixture(t)
	p := r.Prepare(cfg)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.Outfile); !os.IsNotExist(err) {
		t.Fatal("preparation compiled the asset")
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if output := esbuildRender(t, p); output != `<script src="/static/js/app.js" defer></script>` {
				t.Errorf("unexpected output: %s", output)
			}
		}()
	}
	wg.Wait()
	if r.store.buildCount != 1 {
		t.Fatalf("concurrent misses built %d times", r.store.buildCount)
	}
	dep := filepath.Join(filepath.Dir(cfg.Entry), "message.js")
	info, err := os.Stat(dep)
	if err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, dep, `export const message = "other";`)
	if err := os.Chtimes(dep, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	esbuildRender(t, p)
	if r.store.buildCount != 2 || !strings.Contains(esbuildRead(t, cfg.Outfile), "other") {
		t.Fatal("same-size/same-mtime dependency change remained cached")
	}
	if err := os.Remove(cfg.Outfile); err != nil {
		t.Fatal(err)
	}
	esbuildRender(t, p)
	if r.store.buildCount != 3 {
		t.Fatal("missing output remained cached")
	}
	r.Invalidate()
	esbuildRender(t, p)
	if r.store.buildCount != 4 {
		t.Fatal("reload did not invalidate cache")
	}
	if !r.OwnsOutput(cfg.Outfile) {
		t.Fatal("generated output not registered for watcher exclusion")
	}
}

func TestEsbuildUncachedAlwaysBuildsAndWrappersAreIndependent(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Cache = false
	p := r.Prepare(cfg)
	for i := 0; i < 3; i++ {
		esbuildRender(t, p)
	}
	if r.store.buildCount != 3 {
		t.Fatalf("cache false built %d times", r.store.buildCount)
	}
	cfg.Cache = true
	p = r.Prepare(cfg)
	esbuildRender(t, p)
	cfg.Enclose = `<a href="|">source</a>`
	wrapped := r.Prepare(cfg)
	if got := esbuildRender(t, wrapped); got != `<a href="/static/js/app.js">source</a>` || r.store.buildCount != 4 {
		t.Fatalf("wrapper changed build identity: %s (%d builds)", got, r.store.buildCount)
	}
	cfg.Enclose = ""
	if got := esbuildRender(t, r.Prepare(cfg)); got != "/static/js/app.js" {
		t.Fatal(got)
	}
}

func TestEsbuildFailedRebuildPreservesFilesAndRetries(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Sourcemap = true
	p := r.Prepare(cfg)
	esbuildRender(t, p)
	previous, previousMap := esbuildRead(t, cfg.Outfile), esbuildRead(t, cfg.Outfile+".map")
	esbuildWrite(t, cfg.Entry, `function broken(`)
	if output, errs := p.Render(nil); output != "" || len(errs) == 0 {
		t.Fatalf("bad source reported success: %q %v", output, errs)
	}
	if esbuildRead(t, cfg.Outfile) != previous || esbuildRead(t, cfg.Outfile+".map") != previousMap {
		t.Fatal("failed build replaced valid assets")
	}
	esbuildWrite(t, cfg.Entry, `console.log("fixed");`)
	esbuildRender(t, p)
	if !strings.Contains(esbuildRead(t, cfg.Outfile), "fixed") {
		t.Fatal("failed build poisoned future retries")
	}
	if files, _ := filepath.Glob(filepath.Join(filepath.Dir(cfg.Outfile), ".hb-esbuild-*")); len(files) != 0 {
		t.Fatalf("temporary files leaked: %v", files)
	}
}

func TestEsbuildConcurrentUncachedAndCancelledRequest(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Cache = false
	p := r.Prepare(cfg)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, errs := p.Render(context.Background()); len(errs) != 0 {
				t.Errorf("uncached render: %v", errs)
			}
		}()
	}
	wg.Wait()
	if r.store.buildCount != 8 {
		t.Fatalf("uncached requests shared a build: %d", r.store.buildCount)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, errs := p.Render(ctx); len(errs) == 0 || r.store.buildCount != 8 {
		t.Fatal("already cancelled request compiled assets")
	}
	if !strings.Contains(esbuildRead(t, cfg.Outfile), "first") {
		t.Fatal("concurrent publication corrupted the output")
	}
}

func TestEsbuildCSSImportsAssetsTargetsAndSourcemaps(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Entry = filepath.Join(filepath.Dir(cfg.Entry), "site.css")
	cfg.Outfile = filepath.Join(r.store.staticDir, "css", "site.css")
	cfg.Enclose = `<link rel="stylesheet" href="|">`
	cfg.Minify = true
	cfg.Sourcemap = true
	cfg.Target = []string{"chrome110", "safari16"}
	cfg.Loader = map[string]string{".woff2": "file", ".png": "dataurl"}
	cfg.External = []string{"/static/vendor/*"}
	esbuildWrite(t, cfg.Entry, `@import "./tokens.css"; @font-face { font-family: Demo; src: url("./sample.woff2") } body { background-image: url("./sample.png"), url("/static/vendor/texture.png"); }`)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "tokens.css"), `body { color: rgb(255, 0, 0); }`)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "sample.woff2"), "fixture font bytes")
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "sample.png"), "fixture image bytes")
	p := r.Prepare(cfg)
	if got := esbuildRender(t, p); got != `<link rel="stylesheet" href="/static/css/site.css">` {
		t.Fatal(got)
	}
	css := esbuildRead(t, cfg.Outfile)
	for _, want := range []string{"red", "data:image/png", "/static/vendor/texture.png", "sourceMappingURL=site.css.map"} {
		if !strings.Contains(css, want) {
			t.Errorf("missing %q in %s", want, css)
		}
	}
	fonts, err := filepath.Glob(filepath.Join(filepath.Dir(cfg.Outfile), "*.woff2"))
	if err != nil || len(fonts) != 1 || esbuildRead(t, fonts[0]) != "fixture font bytes" {
		t.Fatalf("font output = %v %v", fonts, err)
	}
	if err := os.Remove(fonts[0]); err != nil {
		t.Fatal(err)
	}
	esbuildRender(t, p)
	if r.store.buildCount != 2 {
		t.Fatal("missing auxiliary asset remained cached")
	}
}

func TestEsbuildOptionsValidationAndModuleIsolation(t *testing.T) {
	r, cfg := esbuildFixture(t)
	if (&EsbuildRenderer{}).Prepare(cfg).Err() == nil {
		t.Fatal("uninitialized renderer did not report a missing build store")
	}
	for _, tc := range []struct {
		name   string
		change func(*EsbuildConfig)
	}{
		{"missing entry", func(c *EsbuildConfig) { c.Entry = "" }},
		{"outside static", func(c *EsbuildConfig) { c.Outfile = filepath.Join(t.TempDir(), "bad.js") }},
		{"target", func(c *EsbuildConfig) { c.Target = []string{"unknown42"} }},
		{"duplicate target", func(c *EsbuildConfig) { c.Target = []string{"chrome110", "chrome120"} }},
		{"loader", func(c *EsbuildConfig) { c.Loader = map[string]string{".png": "not-a-loader"} }},
		{"external", func(c *EsbuildConfig) { c.External = []string{"a*b*c"} }},
		{"missing binary", func(c *EsbuildConfig) { c.Binary = filepath.Join(t.TempDir(), "missing-esbuild") }},
		{"CSS mangle", func(c *EsbuildConfig) {
			c.Entry = filepath.Join(filepath.Dir(c.Entry), "site.css")
			c.Outfile = filepath.Join(r.store.staticDir, "site.css")
			c.Mangle = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := cfg
			tc.change(&copy)
			if r.Prepare(copy).Err() == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	p := r.Prepare(cfg)
	esbuildRender(t, p)
	other, otherConfig := esbuildFixture(t)
	esbuildWrite(t, otherConfig.Entry, `console.log("second module");`)
	esbuildRender(t, other.Prepare(otherConfig))
	if strings.Contains(esbuildRead(t, cfg.Outfile), "second module") || !strings.Contains(esbuildRead(t, otherConfig.Outfile), "second module") {
		t.Fatal("cross-module result reuse")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(r.store.staticDir, "escape")); err != nil {
		t.Fatal(err)
	}
	cfg.Outfile = filepath.Join(r.store.staticDir, "escape", "bad.js")
	if r.Prepare(cfg).Err() == nil {
		t.Fatal("output symlink escaped static")
	}
	if output, errs := (*PreparedEsbuild)(nil).Render(nil); output != "" || len(errs) == 0 {
		t.Fatal("missing preparation not rejected")
	}
}

func TestEsbuildEmbeddedAndExternalFlagMapping(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Minify, cfg.MinifyIdentifiers, cfg.Mangle, cfg.Sourcemap = true, false, true, true
	cfg.Target = []string{"es2020", "chrome110"}
	cfg.Loader = map[string]string{".png": "file"}
	cfg.External = []string{"/static/vendor/*"}
	p := r.Prepare(cfg)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	opts, err := p.spec.options()
	if err != nil {
		t.Fatal(err)
	}
	if !opts.MinifyWhitespace || !opts.MinifySyntax || opts.MinifyIdentifiers || opts.MangleProps != ".*" || opts.Sourcemap != api.SourceMapLinked || opts.Target != api.ES2020 || opts.Loader[".png"] != api.LoaderFile {
		t.Fatalf("wrong Go API options: %+v", opts)
	}
	args := strings.Join(p.spec.externalArgs("output.js", "metadata.json"), " ")
	for _, want := range []string{"--minify-whitespace", "--minify-syntax", "--mangle-props=.*", "--sourcemap=linked", "--target=es2020,chrome110", "--loader:.png=file", "--external:/static/vendor/*"} {
		if !strings.Contains(args, want) {
			t.Errorf("missing flag %s in %s", want, args)
		}
	}
	if strings.Contains(args, "--minify ") || strings.Contains(args, "--minify-identifiers") {
		t.Fatal("CLI minify enabled identifiers implicitly")
	}
	cfg.Loader[".png"] = "dataurl"
	if reflect.DeepEqual(cfg.Loader, p.spec.Loader) {
		t.Fatal("prepared options retain mutable source map")
	}
}

func TestEsbuildRealExternalParity(t *testing.T) {
	binary := os.Getenv("ESBUILD_TEST_BINARY")
	if binary == "" {
		t.Skip("set ESBUILD_TEST_BINARY to test a real external esbuild executable")
	}
	r, cfg := esbuildFixture(t)
	cfg.Minify, cfg.MinifyIdentifiers, cfg.Sourcemap = true, true, true
	esbuildRender(t, r.Prepare(cfg))
	apiOutput := esbuildRead(t, cfg.Outfile)
	apiMap := esbuildRead(t, cfg.Outfile+".map")
	cfg.Binary = binary
	esbuildRender(t, r.Prepare(cfg))
	if cliOutput := esbuildRead(t, cfg.Outfile); !bytes.Equal([]byte(apiOutput), []byte(cliOutput)) {
		t.Fatalf("embedded/external output differs:\n%s\n%s", apiOutput, cliOutput)
	}
	var embeddedMap, externalMap interface{}
	if err := json.Unmarshal([]byte(apiMap), &embeddedMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(esbuildRead(t, cfg.Outfile+".map")), &externalMap); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(embeddedMap, externalMap) {
		t.Fatalf("source maps differ: API=%#v CLI=%#v", embeddedMap, externalMap)
	}
	cfg.Entry = filepath.Join(filepath.Dir(cfg.Entry), "site.css")
	cfg.Outfile = filepath.Join(r.store.staticDir, "css", "site.css")
	cfg.Loader = map[string]string{".woff2": "file"}
	cfg.Binary = ""
	esbuildWrite(t, cfg.Entry, `@import "./color.css"; @font-face {font-family: Demo; src: url("./font.woff2")}`)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "color.css"), `body { color: red; }`)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "font.woff2"), "font fixture")
	esbuildRender(t, r.Prepare(cfg))
	apiCSS := esbuildRead(t, cfg.Outfile)
	cfg.Binary = binary
	esbuildRender(t, r.Prepare(cfg))
	if cliCSS := esbuildRead(t, cfg.Outfile); cliCSS != apiCSS {
		t.Fatalf("CSS differs: API=%s CLI=%s", apiCSS, cliCSS)
	}
	esbuildWrite(t, cfg.Entry, `@import "./missing-stylesheet.css";`)
	if _, errs := r.Prepare(cfg).Render(nil); len(errs) == 0 {
		t.Fatal("external compile failure not reported")
	}
	if esbuildRead(t, cfg.Outfile) != apiCSS {
		t.Fatal("external compile failure replaced valid output")
	}
}
