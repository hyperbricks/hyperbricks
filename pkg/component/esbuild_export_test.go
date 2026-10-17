package component

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func TestEsbuildExportReadsGeneratedEntryAndTransitiveImports(t *testing.T) {
	for _, direct := range []bool{true, false} {
		t.Run(fmt.Sprintf("direct=%t", direct), func(t *testing.T) {
			source, cfg := esbuildFixture(t)
			cfg.Enclose = ""
			esbuildWrite(t, cfg.Entry, `globalThis.generated = "development";`)
			esbuildRender(t, source.Prepare(cfg))
			original := esbuildRead(t, cfg.Outfile)
			esbuildWrite(t, cfg.Entry, `globalThis.generated = "export";`)
			export := NewEsbuildRenderer(source.store.staticDir)
			export.store.cacheDir = source.store.cacheDir
			export.store.ownershipDir = source.store.ownershipDir
			if err := export.SetOutputRoot(filepath.Join(t.TempDir(), "static")); err != nil {
				t.Fatal(err)
			}
			producer := export.Prepare(cfg)
			esbuildRender(t, producer)
			consumer := cfg
			consumer.Entry, consumer.Outfile = cfg.Outfile, filepath.Join(source.store.staticDir, "js", "consumer.js")
			if !direct {
				consumer.Entry = filepath.Join(filepath.Dir(cfg.Entry), "consumer.ts")
				rel, err := filepath.Rel(filepath.Dir(consumer.Entry), cfg.Outfile)
				if err != nil {
					t.Fatal(err)
				}
				esbuildWrite(t, consumer.Entry, fmt.Sprintf("import %q; const answer: number = 42; globalThis.answer = answer;", filepath.ToSlash(rel)))
			}
			p := export.Prepare(consumer)
			url := esbuildRender(t, p)
			vm := goja.New()
			if _, err := vm.RunString(esbuildRead(t, esbuildURLPath(t, export, url))); err != nil {
				t.Fatal(err)
			}
			if got := vm.Get("generated").String(); got != "export" {
				t.Fatal("consumer read stale development bytes", got)
			}
			if !export.store.results[p.key].inputs[producer.spec.Outfile].Exists {
				t.Fatal("actual exported input not recorded")
			}
			if export.store.results[p.key].inputs[cfg.Outfile].Exists {
				t.Fatal("development output incorrectly recorded as build input")
			}
			if esbuildRead(t, cfg.Outfile) != original {
				t.Fatal("export changed development output")
			}
		})
	}
}

func TestEsbuildExportPreservesAuthoredResolutionAndLoaders(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose = ""
	output := filepath.Join(t.TempDir(), "static")
	if err := r.SetOutputRoot(output); err != nil {
		t.Fatal(err)
	}
	output = r.store.staticDir
	sourceRoot, err := filepath.Abs(r.store.sourceStaticDir)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(sourceRoot, "js", "generated.js")
	actual := filepath.Join(output, "js", "generated.js")
	authored := filepath.Join(filepath.Dir(cfg.Entry), "authored.json")
	esbuildWrite(t, authored, `{"label":"authored"}`)
	rel, err := filepath.Rel(filepath.Dir(entry), authored)
	if err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, entry, `globalThis.result = "stale";`)
	esbuildWrite(t, actual, fmt.Sprintf("import data from %q; import { label } from './child.ts'; import text from './value.dat'; globalThis.result = data.label + ':' + label + ':' + text;", filepath.ToSlash(rel)))
	esbuildWrite(t, filepath.Join(output, "js", "child.ts"), `export const label: string = "export";`)
	esbuildWrite(t, filepath.Join(output, "js", "value.dat"), "custom loader")
	cfg.Entry, cfg.Outfile = entry, filepath.Join(sourceRoot, "js", "result.js")
	cfg.Loader = map[string]string{".dat": "text"}
	url := esbuildRender(t, r.Prepare(cfg))
	vm := goja.New()
	if _, err := vm.RunString(esbuildRead(t, esbuildURLPath(t, r, url))); err != nil {
		t.Fatal(err)
	}
	if got := vm.Get("result").String(); got != "authored:export:custom loader" {
		t.Fatal("incorrect resolution/loader", got)
	}
	// The original target need not exist after its producer has generated the
	// export copy; entry resolution must still find the selected output root.
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	r.Invalidate()
	esbuildRender(t, r.Prepare(cfg))
}

func TestEsbuildExportCSSAndFileLoaderUseExportBytes(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose = ""
	sourceRoot := r.store.staticDir
	if err := r.SetOutputRoot(filepath.Join(t.TempDir(), "static")); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{sourceRoot, r.store.staticDir} {
		color, font := "red", "development font"
		if root == r.store.staticDir {
			color, font = "blue", "export font"
		}
		esbuildWrite(t, filepath.Join(root, "css", "generated.css"), "body {color: "+color+"}; @font-face {font-family: Demo; src: url('./font.woff2')}")
		esbuildWrite(t, filepath.Join(root, "css", "font.woff2"), font)
	}
	cfg.Entry = filepath.Join(filepath.Dir(cfg.Entry), "entry.css")
	cfg.Outfile = filepath.Join(sourceRoot, "css", "final.css")
	cfg.Loader = map[string]string{".woff2": "file"}
	rel, err := filepath.Rel(filepath.Dir(cfg.Entry), filepath.Join(sourceRoot, "css", "generated.css"))
	if err != nil {
		t.Fatal(err)
	}
	esbuildWrite(t, cfg.Entry, fmt.Sprintf("@import %q;", filepath.ToSlash(rel)))
	p := r.Prepare(cfg)
	url := esbuildRender(t, p)
	css := esbuildRead(t, esbuildURLPath(t, r, url))
	if !strings.Contains(css, "blue") || strings.Contains(css, "red") {
		t.Fatal("CSS used development generation", css)
	}
	fonts, err := filepath.Glob(filepath.Join(filepath.Dir(p.spec.Outfile), "font-*.woff2"))
	if err != nil || len(fonts) != 1 {
		t.Fatal("missing file-loader output", fonts, err)
	}
	if got := esbuildRead(t, fonts[0]); got != "export font" {
		t.Fatal("file-loader used development bytes", got)
	}
}

func TestEsbuildCanonicalHistoryAcceptsExistingAliasPaths(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacyManifest=%t", legacy), func(t *testing.T) {
			r, cfg := esbuildFixture(t)
			cfg.Enclose = ""
			p := r.Prepare(cfg)
			esbuildRender(t, p)
			alias := filepath.Join(t.TempDir(), "static-alias")
			if err := os.Symlink(r.store.staticDir, alias); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			history, err := r.store.historyPath()
			if err != nil {
				t.Fatal(err)
			}
			toAlias := func(path string) string {
				rel, err := filepath.Rel(r.store.staticDir, path)
				if err != nil {
					t.Fatal(err)
				}
				return filepath.Join(alias, rel)
			}
			if legacy {
				manifestPath, err := r.store.manifestPath(p)
				if err != nil {
					t.Fatal(err)
				}
				var manifest esbuildManifest
				if err := json.Unmarshal([]byte(esbuildRead(t, manifestPath)), &manifest); err != nil {
					t.Fatal(err)
				}
				manifest.Spec.StaticDir, manifest.Spec.Outfile, manifest.Entry = alias, toAlias(manifest.Spec.Outfile), toAlias(manifest.Entry)
				outputs := map[string]esbuildFingerprint{}
				for path, stamp := range manifest.Outputs {
					outputs[toAlias(path)] = stamp
				}
				manifest.Outputs = outputs
				data, _ := json.Marshal(manifest)
				esbuildWrite(t, manifestPath, string(data))
				if err := os.Remove(history); err != nil {
					t.Fatal(err)
				}
			} else {
				h, err := r.store.loadHistory()
				if err != nil {
					t.Fatal(err)
				}
				owner := h.Owners[p.spec.Outfile]
				for i, generation := range owner.Generations {
					outputs := map[string]esbuildFingerprint{}
					for path, stamp := range generation.Outputs {
						outputs[toAlias(path)] = stamp
					}
					owner.Generations[i] = esbuildGeneration{Entry: toAlias(generation.Entry), Outputs: outputs}
				}
				h.Root, h.Owners = alias, map[string]*esbuildOwnerHistory{toAlias(p.spec.Outfile): owner}
				data, _ := json.Marshal(h)
				esbuildWrite(t, history, string(data))
			}
			next := NewEsbuildRenderer(alias)
			next.store.cacheDir = r.store.cacheDir
			next.store.ownershipDir = r.store.ownershipDir
			h, err := next.store.loadHistory()
			if err != nil {
				t.Fatal(err)
			}
			if h.Root != r.store.staticDir || h.Owners[p.spec.Outfile] == nil {
				t.Fatalf("ownership aliases were lost: %#v", h)
			}
			cfg.Outfile = toAlias(cfg.Outfile)
			prepared := next.Prepare(cfg)
			if prepared.spec.Outfile != p.spec.Outfile || prepared.spec.SourceStaticDir != "" {
				t.Fatal("alias changed effective output identity")
			}
			if err := next.BeginAssetSession(); err != nil {
				t.Fatal(err)
			}
			defer next.CloseAssets()
			if got, errs := prepared.Render(context.Background()); got != "/static/js/app.js" || len(errs) != 0 {
				t.Fatal("alias render failed", got, errs)
			}
			if !next.OwnsOutput(cfg.Outfile) {
				t.Fatal("watcher alias did not identify generated output")
			}
			if err := os.Remove(cfg.Outfile); err != nil {
				t.Fatal(err)
			}
			if !next.OwnsOutput(cfg.Outfile) {
				t.Fatal("watcher alias lost removed generated output")
			}
		})
	}
}

func TestEsbuildHistoryAliasNeverClaimsSymlinkTarget(t *testing.T) {
	r, cfg := esbuildFixture(t)
	cfg.Enclose = ""
	p := r.Prepare(cfg)
	esbuildRender(t, p)
	original := esbuildRead(t, cfg.Outfile)
	handwritten := filepath.Join(r.store.staticDir, "handwritten.js")
	esbuildWrite(t, handwritten, original)
	if err := os.Remove(cfg.Outfile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(handwritten, cfg.Outfile); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	h, err := r.store.loadHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed := h.Owners[cfg.Outfile].Generations[0].Outputs[handwritten]; claimed {
		t.Fatal("ownership followed a replacement symlink")
	}
	if err := r.ReconcileAssets(map[string]int{}); err == nil {
		t.Fatal("cleanup accepted a replaced owned symlink")
	}
	if got := esbuildRead(t, handwritten); got != original {
		t.Fatal("handwritten target changed")
	}
	if info, err := os.Lstat(cfg.Outfile); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("cleanup removed the replacement symlink", err)
	}
	// A current explicit build replaces the configured link, never its target.
	next := esbuildRestart(r)
	prepared := next.Prepare(cfg)
	if prepared.spec.Outfile != cfg.Outfile {
		t.Fatal("preparation redirected publication to the symlink target")
	}
	esbuildWrite(t, cfg.Entry, `globalThis.changed = true;`)
	esbuildRender(t, prepared)
	if got := esbuildRead(t, handwritten); got != original {
		t.Fatal("publication overwrote handwritten target")
	}
	if info, err := os.Lstat(cfg.Outfile); err != nil || !info.Mode().IsRegular() {
		t.Fatal("publication did not replace the configured link", err)
	}
}
