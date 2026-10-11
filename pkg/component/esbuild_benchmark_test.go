package component

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// BenchmarkEsbuildLifecycle measures PreparedEsbuild.Render with the embedded
// engine. Fixture setup, source edits and output assertions are outside timing.
// Cold means empty HyperBricks build caches, not a cold OS cache or new process.
func BenchmarkEsbuildLifecycle(b *testing.B) {
	for _, fingerprint := range []bool{false, true} {
		for _, phase := range []string{"cold", "warm", "changed"} {
			b.Run(fmt.Sprintf("fingerprint=%t/%s", fingerprint, phase), func(b *testing.B) {
				b.StopTimer()
				root := b.TempDir()
				source := filepath.Join(root, "resources")
				static := filepath.Join(root, "static")
				private := filepath.Join(root, "private")
				write := func(path, data string) {
					b.Helper()
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						b.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(data), 0600); err != nil {
						b.Fatal(err)
					}
				}
				var entry strings.Builder
				entry.WriteString("import './styles.css';\nimport {revision} from './revision.js';\n")
				expected := int64(0)
				for m := range 24 {
					fmt.Fprintf(&entry, "import {total as total%d} from './part%d.js';\n", m, m)
					var module strings.Builder
					module.WriteString("const products = [\n")
					for n := range 32 {
						value := m*32 + n + 1
						expected += int64(value)
						fmt.Fprintf(&module, "{name:'Product %04d',price:%d,category:'category-%02d'},\n", value, value, m)
					}
					module.WriteString("];\nexport function total(){return products.reduce((sum,p)=>sum+p.price,0);}\n")
					write(filepath.Join(source, fmt.Sprintf("part%d.js", m)), module.String())
				}
				entry.WriteString("globalThis.hbBenchmark={revision,total:")
				for m := range 24 {
					if m > 0 {
						entry.WriteString("+")
					}
					fmt.Fprintf(&entry, "total%d()", m)
				}
				entry.WriteString("};\n")
				write(filepath.Join(source, "main.js"), entry.String())
				write(filepath.Join(source, "styles.css"), "@import './tokens.css'; @font-face{font-family:Bench;src:url('./font.woff2')} .catalog-bench{color:var(--ink);font-family:Bench}")
				write(filepath.Join(source, "tokens.css"), ":root{--ink:#123456}")
				font := "benchmark-font-fixture"
				write(filepath.Join(source, "font.woff2"), font)
				dependency := filepath.Join(source, "revision.js")
				revision := "00000000"
				write(dependency, "export const revision='"+revision+"';")
				stamp, err := os.Stat(dependency)
				if err != nil {
					b.Fatal(err)
				}
				cfg := EsbuildConfig{Entry: filepath.Join(source, "main.js"), Outfile: filepath.Join(static, "js", "app.js"), Cache: true, Fingerprint: fingerprint, Minify: true, MinifyIdentifiers: true, Sourcemap: true, Loader: map[string]string{".woff2": "file"}}
				var r *EsbuildRenderer
				var prepared *PreparedEsbuild
				reset := func() {
					b.Helper()
					if r != nil {
						r.CloseAssets()
					}
					for _, p := range []string{static, private} {
						if err := os.RemoveAll(p); err != nil {
							b.Fatal(err)
						}
					}
					r = NewEsbuildRenderer(static)
					r.store.cacheDir = private
					if err := r.BeginAssetSession(); err != nil {
						b.Fatal(err)
					}
					prepared = r.Prepare(cfg)
					if err := prepared.Err(); err != nil {
						b.Fatal(err)
					}
					if r.store.buildCount != 0 {
						b.Fatal("preparation unexpectedly compiled")
					}
				}
				b.Cleanup(func() {
					if r != nil {
						r.CloseAssets()
					}
				})
				render := func() string {
					b.Helper()
					out, errs := prepared.Render(context.Background())
					if len(errs) != 0 {
						b.Fatal(errs)
					}
					return out
				}
				validate := func(public string) [32]byte {
					b.Helper()
					result := r.store.results[prepared.key]
					if result == nil {
						b.Fatal("missing successful build")
					}
					rel, err := filepath.Rel(static, result.entry)
					if err != nil {
						b.Fatal(err)
					}
					if public != "/static/"+filepath.ToSlash(rel) {
						b.Fatalf("wrong asset URL: %s", public)
					}
					js, err := os.ReadFile(result.entry)
					if err != nil {
						b.Fatal(err)
					}
					vm := goja.New()
					if _, err = vm.RunString(string(js)); err != nil {
						b.Fatal("generated JS cannot execute", err)
					}
					obj := vm.Get("hbBenchmark").ToObject(vm)
					if obj.Get("revision").String() != revision || obj.Get("total").ToInteger() != expected {
						b.Fatal("stale or incorrect JavaScript result")
					}
					if !strings.Contains(string(js), "sourceMappingURL="+filepath.Base(result.entry)+".map") {
						b.Fatal("missing linked JS source map")
					}
					css, fonts, maps := 0, 0, 0
					for path, record := range result.outputs {
						data, err := os.ReadFile(path)
						if err != nil {
							b.Fatal(err)
						}
						if !record.Exists || len(data) == 0 || sha256.Sum256(data) != record.Hash {
							b.Fatal("invalid generated file", path)
						}
						switch filepath.Ext(path) {
						case ".css":
							css++
							if !strings.Contains(string(data), ".catalog-bench") || !strings.Contains(string(data), "#123456") {
								b.Fatal("CSS imports missing")
							}
						case ".woff2":
							fonts++
							if string(data) != font {
								b.Fatal("file-loader bytes changed")
							}
						case ".map":
							maps++
							var m struct {
								Version  int      `json:"version"`
								Sources  []string `json:"sources"`
								Mappings string   `json:"mappings"`
							}
							if err := json.Unmarshal(data, &m); err != nil || m.Version != 3 || len(m.Sources) == 0 || m.Mappings == "" {
								b.Fatal("invalid source map", path, err)
							}
						}
					}
					if css != 1 || fonts != 1 || maps != 2 {
						b.Fatalf("incomplete output: css=%d fonts=%d maps=%d", css, fonts, maps)
					}
					return sha256.Sum256(js)
				}
				reset()
				var previousURL string
				var previousHash [32]byte
				var historyInfo os.FileInfo
				if phase != "cold" {
					previousURL = render()
					previousHash = validate(previousURL)
					if r.store.buildCount != 1 {
						b.Fatal("warm-up did not compile once")
					}
					path, err := r.store.historyPath()
					if err != nil {
						b.Fatal(err)
					}
					historyInfo, err = os.Stat(path)
					if err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if phase == "cold" {
						reset()
					}
					if phase == "changed" {
						revision = fmt.Sprintf("%08d", i+1)
						write(dependency, "export const revision='"+revision+"';")
						if err := os.Chtimes(dependency, stamp.ModTime(), stamp.ModTime()); err != nil {
							b.Fatal(err)
						}
					}
					before := r.store.buildCount
					b.StartTimer()
					public, errs := prepared.Render(context.Background())
					b.StopTimer()
					if len(errs) != 0 {
						b.Fatal(errs)
					}
					hash := validate(public)
					builds := r.store.buildCount - before
					if phase == "warm" {
						if builds != 0 || public != previousURL || hash != previousHash {
							b.Fatal("warm hit rebuilt or changed output")
						}
						path, err := r.store.historyPath()
						if err != nil {
							b.Fatal(err)
						}
						info, err := os.Stat(path)
						if err != nil {
							b.Fatal(err)
						}
						if !os.SameFile(info, historyInfo) || !info.ModTime().Equal(historyInfo.ModTime()) {
							b.Fatal("cache hit rewrote history")
						}
					} else {
						if builds != 1 {
							b.Fatalf("expected exactly one compilation; got %d", builds)
						}
						if phase == "changed" {
							if hash == previousHash || (fingerprint && public == previousURL) || (!fingerprint && public != previousURL) {
								b.Fatal("changed source did not update expected output/URL")
							}
							if again := render(); again != public || r.store.buildCount != before+1 {
								b.Fatal("rebuilt result was not reusable")
							}
						}
					}
					previousURL, previousHash = public, hash
				}
			})
		}
	}
}
