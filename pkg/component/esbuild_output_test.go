package component

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEsbuildLowercaseEntryHashes(t *testing.T) {
	engines := []struct{ name, binary string }{{name: "embedded"}}
	if binary := os.Getenv("ESBUILD_TEST_BINARY"); binary != "" {
		engines = append(engines, struct{ name, binary string }{"external", binary})
	}
	for _, engine := range engines {
		for _, kind := range []string{"js", "css"} {
			t.Run(engine.name+"/"+kind, func(t *testing.T) {
				r, cfg := esbuildFixture(t)
				cfg.Fingerprint, cfg.Sourcemap, cfg.Enclose = true, true, ""
				cfg.Binary = engine.binary
				cfg.Outfile = filepath.Join(r.store.staticDir, "Public Assets", "App Bundle."+kind)
				css := filepath.Join(filepath.Dir(cfg.Entry), "style.css")
				esbuildWrite(t, css, `body { color: red; }`)
				if kind == "js" {
					esbuildWrite(t, cfg.Entry, `import "./style.css"; console.log("Keep THIS Capitalized");`)
				} else {
					cfg.Entry = css
				}
				p := r.Prepare(cfg)
				publicURL := esbuildRender(t, p)
				entry := esbuildURLPath(t, r, publicURL)
				result := r.store.results[p.key]
				entries := 0
				for path := range result.outputs {
					if filepath.Ext(path) != ".js" && filepath.Ext(path) != ".css" {
						continue
					}
					entries++
					hash := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "App Bundle."), filepath.Ext(path))
					if hash == "" || hash != strings.ToLower(hash) || !strings.HasPrefix(filepath.Base(path), "App Bundle.") || filepath.Base(filepath.Dir(path)) != "Public Assets" {
						t.Fatalf("hash not lowercase or authored casing changed: %s", path)
					}
					mapURL := (&url.URL{Path: filepath.Base(path) + ".map"}).EscapedPath()
					if !strings.Contains(esbuildRead(t, path), "sourceMappingURL="+mapURL) {
						t.Fatal("source-map link has the wrong casing or escaping")
					}
					esbuildRead(t, path+".map")
					// Check actual directory entries too; reads alone miss casing bugs
					// on a case-insensitive developer filesystem.
					files, err := os.ReadDir(filepath.Dir(path))
					if err != nil {
						t.Fatal(err)
					}
					found, foundMap := false, false
					for _, file := range files {
						found = found || file.Name() == filepath.Base(path)
						foundMap = foundMap || file.Name() == filepath.Base(path)+".map"
					}
					if !found || !foundMap {
						t.Fatalf("actual filenames do not match the returned URL: %s", path)
					}
				}
				want := 1
				if kind == "js" {
					want = 2 // A JS entry's CSS sibling must be normalized as well.
					if !strings.Contains(esbuildRead(t, entry), "Keep THIS Capitalized") {
						t.Fatal("application content was lowercased")
					}
				}
				if entries != want {
					t.Fatalf("entry count = %d, want %d", entries, want)
				}
				next := esbuildRestart(r)
				if got := esbuildRender(t, next.Prepare(cfg)); got != publicURL || next.store.buildCount != 0 {
					t.Fatal("lowercase build did not survive restart")
				}
			})
		}
	}
}

func TestEsbuildLowercasePreservesContent(t *testing.T) {
	s := esbuildSpec{Outfile: "/static/JS/App.js", Fingerprint: true}
	old := "/static/JS/App.ABCDEFGH.js"
	body := []byte("console.log('App.ABCDEFGH.js.map');\n//# sourceMappingURL=App.ABCDEFGH.js.map\n")
	sourceMap := []byte(`{"version":3,"file":"App.ABCDEFGH.js","sourcesContent":["App.ABCDEFGH.js.map"]}`)
	asset := []byte("App.ABCDEFGH.js.map")
	outputs := map[string][]byte{old: body, old + ".map": sourceMap, "/static/JS/icon-ABCDEFGH.png": asset}
	meta := `{"outputs":{"/static/JS/App.ABCDEFGH.js":{}}}`
	got, entry, err := s.lowercaseEntryHashes(outputs, meta, old)
	if err != nil || entry != "/static/JS/App.abcdefgh.js" {
		t.Fatalf("entry=%s err=%v", entry, err)
	}
	if string(got[entry]) != "console.log('App.ABCDEFGH.js.map');\n//# sourceMappingURL=App.abcdefgh.js.map\n" || !bytes.Equal(got["/static/JS/icon-ABCDEFGH.png"], asset) {
		t.Fatal("application strings or file-loader assets were modified")
	}
	var decoded struct {
		File           string   `json:"file"`
		SourcesContent []string `json:"sourcesContent"`
	}
	if err := json.Unmarshal(got[entry+".map"], &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.File != "App.abcdefgh.js" || len(decoded.SourcesContent) != 1 || decoded.SourcesContent[0] != "App.ABCDEFGH.js.map" || !bytes.Equal(outputs[old], body) {
		t.Fatal("source content or original compiler output was modified")
	}
	outputs[old] = []byte("console.log('no linked footer');")
	if _, _, err := s.lowercaseEntryHashes(outputs, meta, old); err == nil {
		t.Fatal("missing source-map footer was silently accepted")
	}
	s.Fingerprint = false
	if _, entry, err := s.lowercaseEntryHashes(outputs, "", old); err != nil || entry != old {
		t.Fatal("fixed filenames were changed")
	}
}

func TestEsbuildLowercaseRejectsPreviousManifest(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose, cfg.Fingerprint, cfg.Sourcemap = "", true, true
	p := r.Prepare(cfg)
	outputs, meta, err := p.spec.compile(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	oldEntry, err := p.spec.entryOutput(meta, outputs)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := p.spec.dependencies(meta)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := esbuildFingerprints(paths)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := p.spec.engineIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manifest := esbuildManifest{Version: esbuildManifestVersion - 1, Key: p.key, Engine: engine, Spec: p.spec, Entry: oldEntry, Inputs: inputs, Outputs: make(map[string]esbuildFingerprint)}
	for path, data := range outputs {
		esbuildWrite(t, path, string(data))
		manifest.Outputs[path] = esbuildFingerprint{Exists: true, Hash: sha256.Sum256(data)}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path, err := r.store.manifestPath(p)
	if err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, path, string(data))
	next := esbuildRestart(r)
	got := esbuildRender(t, next.Prepare(cfg))
	if filepath.Base(got) != strings.ToLower(filepath.Base(oldEntry)) || next.store.buildCount != 1 {
		t.Fatalf("old manifest survived normalization: %s, builds=%d", got, next.store.buildCount)
	}
	if !next.OwnsOutput(esbuildURLPath(t, next, got)) {
		t.Fatal("lowercase output not registered for watcher exclusion")
	}
	files, err := os.ReadDir(filepath.Dir(oldEntry))
	if err != nil {
		t.Fatal(err)
	}
	want, found, foundMap := strings.ToLower(filepath.Base(oldEntry)), false, false
	for _, file := range files {
		found = found || file.Name() == want
		foundMap = foundMap || file.Name() == want+".map"
	}
	if !found || !foundMap {
		t.Fatal("cache migration changed URLs but not the on-disk filenames")
	}
}

func TestEsbuildLowercaseRepairsCaseMismatchedCache(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose, cfg.Fingerprint, cfg.Sourcemap = "", true, true
	first := esbuildRender(t, r.Prepare(cfg))
	entry := esbuildURLPath(t, r, first)
	wrong := filepath.Join(filepath.Dir(entry), strings.ToUpper(filepath.Base(entry)))
	stage := entry + ".case-stage"
	if err := os.Rename(entry, stage); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stage, wrong); err != nil {
		t.Fatal(err)
	}
	if esbuildHasExactFilename(entry) {
		t.Fatal("fixture did not create a case-mismatched directory entry")
	}
	next := esbuildRestart(r)
	if got := esbuildRender(t, next.Prepare(cfg)); got != first || next.store.buildCount != 1 {
		t.Fatalf("case-mismatched cache was reused: %s, builds=%d", got, next.store.buildCount)
	}
	if !esbuildHasExactFilename(entry) {
		t.Fatal("rebuilt asset did not restore exact lowercase filename casing")
	}
}
