package component

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func esbuildRestart(r *EsbuildRenderer) *EsbuildRenderer {
	next := NewEsbuildRenderer(r.store.staticDir)
	next.store.cacheDir = r.store.cacheDir
	next.Invalidate() // The real module loader does this before its first Prepare.
	return next
}

func esbuildURLPath(t *testing.T, r *EsbuildRenderer, publicURL string) string {
	t.Helper()
	u, err := url.Parse(publicURL)
	if err != nil || !strings.HasPrefix(u.Path, "/static/") {
		t.Fatalf("invalid asset URL: %q (%v)", publicURL, err)
	}
	path := filepath.Join(r.store.staticDir, filepath.FromSlash(strings.TrimPrefix(u.Path, "/static/")))
	if err := esbuildOutputPath(r.store.staticDir, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEsbuildEmbeddedVersion(t *testing.T) {
	data := esbuildRead(t, filepath.Join("..", "..", "go.mod"))
	if !strings.Contains(data, "github.com/evanw/esbuild "+esbuildEmbeddedVersion+"\n") {
		t.Fatal("embedded esbuild test fallback must match the pinned engine")
	}
}

func TestEsbuildFingerprintFreshnessAndRestart(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Fingerprint, cfg.Sourcemap, cfg.Enclose = true, true, ""
	p := r.Prepare(cfg)
	first := esbuildRender(t, p)
	firstPath := esbuildURLPath(t, r, first)
	if firstPath == cfg.Outfile || !strings.HasPrefix(filepath.Base(firstPath), "app.") || filepath.Ext(firstPath) != ".js" {
		t.Fatalf("unversioned or misnamed output: %s", firstPath)
	}
	firstBytes, firstMap := esbuildRead(t, firstPath), esbuildRead(t, firstPath+".map")
	if !strings.Contains(firstBytes, "sourceMappingURL="+filepath.Base(firstPath)+".map") {
		t.Fatal("source map link does not use the versioned filename")
	}
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "unrelated.txt"), "unrelated resource")
	if got := esbuildRender(t, p); got != first || r.store.buildCount != 1 {
		t.Fatal("unrelated resource invalidated the build")
	}
	next := esbuildRestart(r)
	if got := esbuildRender(t, next.Prepare(cfg)); got != first || next.store.buildCount != 0 {
		t.Fatalf("restart rebuilt unchanged assets: %s, builds=%d", got, next.store.buildCount)
	}
	if !next.OwnsOutput(firstPath) || !next.OwnsOutput(firstPath+".map") {
		t.Fatal("disk cache outputs were not restored for watcher exclusion")
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
	next = esbuildRestart(next)
	second := esbuildRender(t, next.Prepare(cfg))
	if second == first || next.store.buildCount != 1 || !strings.Contains(esbuildRead(t, esbuildURLPath(t, next, second)), "other") {
		t.Fatal("same-size/same-time dependency change did not change the URL")
	}
	if esbuildRead(t, firstPath) != firstBytes || esbuildRead(t, firstPath+".map") != firstMap {
		t.Fatal("new generation replaced an older page's assets")
	}
	cfg.Minify = true
	third := esbuildRender(t, next.Prepare(cfg))
	if third == second || next.store.buildCount != 2 {
		t.Fatal("changed compiler options reused the previous build")
	}
}

func TestEsbuildPersistentFixedOutputAndReload(t *testing.T) {
	r, cfg := esbuildFixture(t)
	p := r.Prepare(cfg)
	first := esbuildRender(t, p)
	next := esbuildRestart(r)
	p = next.Prepare(cfg)
	if got := esbuildRender(t, p); got != first || next.store.buildCount != 0 {
		t.Fatal("fixed output failed persistent reuse")
	}
	next.Invalidate()
	esbuildRender(t, p)
	if next.store.buildCount != 1 {
		t.Fatal("explicit reload was bypassed by the disk manifest")
	}
	esbuildWrite(t, cfg.Outfile, "corrupted asset")
	next = esbuildRestart(next)
	esbuildRender(t, next.Prepare(cfg))
	if next.store.buildCount != 1 || !strings.Contains(esbuildRead(t, cfg.Outfile), "first") {
		t.Fatal("corrupted asset remained cached after restart")
	}
}

func TestEsbuildFingerprintCSSAndAuxiliaryInvalidation(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Entry = filepath.Join(filepath.Dir(cfg.Entry), "site.css")
	cfg.Outfile = filepath.Join(r.store.staticDir, "css", "site.css")
	cfg.Enclose, cfg.Fingerprint, cfg.Sourcemap = "", true, true
	cfg.Loader = map[string]string{".woff2": "file"}
	esbuildWrite(t, cfg.Entry, `@import "./tokens.css"; @font-face {font-family: Demo; src: url("./font.woff2")}`)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "tokens.css"), `body { color: red; }`)
	fontSource := filepath.Join(filepath.Dir(cfg.Entry), "font.woff2")
	esbuildWrite(t, fontSource, "original font")
	p := r.Prepare(cfg)
	first := esbuildRender(t, p)
	firstPath := esbuildURLPath(t, r, first)
	css := esbuildRead(t, firstPath)
	fonts, _ := filepath.Glob(filepath.Join(filepath.Dir(cfg.Outfile), "*.woff2"))
	if len(fonts) != 1 || !strings.Contains(css, filepath.Base(fonts[0])) || !strings.Contains(css, "red") {
		t.Fatalf("incomplete CSS asset generation: %v %s", fonts, css)
	}
	for _, missing := range []string{firstPath + ".map", fonts[0], firstPath} {
		if err := os.Remove(missing); err != nil {
			t.Fatal(err)
		}
		next := esbuildRestart(r)
		if got := esbuildRender(t, next.Prepare(cfg)); got != first || next.store.buildCount != 1 {
			t.Fatalf("missing output not rebuilt: %s %s", missing, got)
		}
		if _, err := os.Stat(missing); err != nil {
			t.Fatal(err)
		}
	}
	esbuildWrite(t, fontSource, "updated font")
	second := esbuildRender(t, p)
	if second == first || esbuildRead(t, firstPath) != css || esbuildRead(t, fonts[0]) != "original font" {
		t.Fatal("font edit did not create a new, independently usable CSS generation")
	}
}

func TestEsbuildFingerprintUncachedConcurrentAndFailure(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose, cfg.Fingerprint = "", true
	p := r.Prepare(cfg)
	urls := make(chan string, 24)
	var wg sync.WaitGroup
	for i := 0; i < cap(urls); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			urls <- esbuildRender(t, p)
		}()
	}
	wg.Wait()
	close(urls)
	first := ""
	for got := range urls {
		if first != "" && got != first {
			t.Fatal("concurrent requests returned different build URLs")
		}
		first = got
	}
	if r.store.buildCount != 1 {
		t.Fatalf("cached misses compiled %d times", r.store.buildCount)
	}
	cfg.Cache = false
	p = r.Prepare(cfg)
	for i := 0; i < 3; i++ {
		if esbuildRender(t, p) != first {
			t.Fatal("identical uncached builds changed URL")
		}
	}
	if r.store.buildCount != 4 {
		t.Fatal("fingerprinting overrode cache: false")
	}
	before := esbuildRead(t, esbuildURLPath(t, r, first))
	esbuildWrite(t, cfg.Entry, `function broken(`)
	if output, errs := p.Render(context.Background()); output != "" || len(errs) == 0 {
		t.Fatalf("failed build returned stale success: %s %v", output, errs)
	}
	if esbuildRead(t, esbuildURLPath(t, r, first)) != before {
		t.Fatal("failed rebuild destroyed previous output")
	}
	esbuildWrite(t, cfg.Entry, `console.log("fixed");`)
	if esbuildRender(t, p) == first {
		t.Fatal("failed rebuild prevented recovery")
	}
}

func TestEsbuildPersistentManifestValidation(t *testing.T) {
	for _, name := range []string{"malformed", "version", "engine", "key", "options", "empty inputs", "empty outputs", "outside output", "missing source", "new config"} {
		t.Run(name, func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			p := r.Prepare(cfg)
			esbuildRender(t, p)
			path, err := r.store.manifestPath(p)
			if err != nil {
				t.Fatal(err)
			}
			var manifest esbuildManifest
			if err := json.Unmarshal([]byte(esbuildRead(t, path)), &manifest); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "version":
				manifest.Version++
			case "engine":
				manifest.Engine = "different engine"
			case "key":
				manifest.Key = "different build"
			case "options":
				manifest.Spec.Minify = true
			case "empty inputs":
				manifest.Inputs = nil
			case "empty outputs":
				manifest.Outputs = nil
			case "outside output":
				manifest.Outputs[filepath.Join(t.TempDir(), "outside.js")] = esbuildFingerprint{Exists: true}
			case "missing source":
				if err := os.Remove(cfg.Entry); err != nil {
					t.Fatal(err)
				}
			case "new config":
				esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "tsconfig.json"), `{"compilerOptions":{"target":"ES2020"}}`)
			}
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if name == "malformed" {
				data = []byte("not json")
			}
			esbuildWrite(t, path, string(data))
			next := esbuildRestart(r)
			output, errs := next.Prepare(cfg).Render(nil)
			if name == "missing source" {
				if output != "" || len(errs) == 0 {
					t.Fatal("source-free SSR unexpectedly reused cached assets")
				}
			} else if len(errs) != 0 || output == "" {
				t.Fatalf("manifest rejection did not recover: %v", errs)
			}
			if next.store.buildCount != 1 {
				t.Fatal("invalid manifest was reused")
			}
		})
	}
}

func TestEsbuildPrivatePersistenceAndUnavailableCache(t *testing.T) {
	for _, name := range []string{"private", "unavailable", "inside static", "relative inside static"} {
		t.Run(name, func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			if name == "unavailable" {
				esbuildWrite(t, r.store.cacheDir+"/file", "not a directory")
				r.store.cacheDir = filepath.Join(r.store.cacheDir, "file", "cache")
			} else if name == "inside static" {
				r.store.cacheDir = filepath.Join(r.store.staticDir, "cache")
			} else if name == "relative inside static" {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				r.store.cacheDir, err = filepath.Rel(cwd, filepath.Join(r.store.staticDir, "cache"))
				if err != nil {
					t.Fatal(err)
				}
			}
			p := r.Prepare(cfg)
			esbuildRender(t, p)
			esbuildRender(t, p)
			if r.store.buildCount != 1 {
				t.Fatal("unavailable persistence broke in-memory caching")
			}
			if name == "private" {
				path, err := r.store.manifestPath(p)
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm()&0077 != 0 {
					t.Fatalf("manifest is not private: %v %v", info, err)
				}
			}
			if err := filepath.WalkDir(r.store.staticDir, func(path string, d os.DirEntry, err error) error {
				if err == nil && (strings.HasSuffix(path, ".json") || strings.HasPrefix(d.Name(), ".hb-esbuild-")) {
					t.Errorf("private metadata leaked into static: %s", path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEsbuildFingerprintExternalParityAndRestart(t *testing.T) {
	binary := os.Getenv("ESBUILD_TEST_BINARY")
	if binary == "" {
		t.Skip("set ESBUILD_TEST_BINARY to verify the external engine")
	}
	r, cfg := esbuildFixture(t)
	cfg.Enclose, cfg.Fingerprint, cfg.Sourcemap = "", true, true
	esbuildWrite(t, cfg.Entry, `import "./site.css"; console.log("JS with CSS");`)
	esbuildWrite(t, filepath.Join(filepath.Dir(cfg.Entry), "site.css"), `body { color: red; }`)
	first := esbuildRender(t, r.Prepare(cfg))
	mainPath := esbuildURLPath(t, r, first)
	mainBytes := esbuildRead(t, mainPath)
	cfg.Binary = binary
	second := esbuildRender(t, r.Prepare(cfg))
	secondPath := esbuildURLPath(t, r, second)
	// The external engine hashes its source map before we rebase source labels
	// from the staging directory. Hash spellings may differ between engines.
	normalize := func(data, path string) string {
		return strings.ReplaceAll(data, filepath.Base(path)+".map", "entry.js.map")
	}
	if normalize(esbuildRead(t, secondPath), secondPath) != normalize(mainBytes, mainPath) {
		t.Fatal("engines produce different JavaScript")
	}
	var embeddedMap, externalMap interface{}
	if err := json.Unmarshal([]byte(esbuildRead(t, mainPath+".map")), &embeddedMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(esbuildRead(t, secondPath+".map")), &externalMap); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(embeddedMap, externalMap) {
		t.Fatal("engines produce different rebased source maps")
	}
	p := r.Prepare(cfg)
	result := r.store.results[p.key]
	cssCount := 0
	for path := range result.outputs {
		if filepath.Ext(path) == ".css" {
			cssCount++
			if !strings.Contains(esbuildRead(t, path), "red") || !strings.Contains(esbuildRead(t, path), "sourceMappingURL="+filepath.Base(path)+".map") {
				t.Fatal("broken sibling CSS output")
			}
		}
	}
	if cssCount != 1 {
		t.Fatalf("expected one generated CSS sibling, got %d", cssCount)
	}
	next := esbuildRestart(r)
	if got := esbuildRender(t, next.Prepare(cfg)); got != second || next.store.buildCount != 0 {
		t.Fatal("external build did not survive restart")
	}
	cfg.Cache = false
	for i := 0; i < 3; i++ {
		if got := esbuildRender(t, next.Prepare(cfg)); got != second {
			t.Fatalf("external URL depends on its temporary directory: %s, expected %s", got, second)
		}
	}
}
