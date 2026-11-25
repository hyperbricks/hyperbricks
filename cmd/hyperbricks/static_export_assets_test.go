package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func configureStaticExportTest(t *testing.T) {
	t.Helper()
	force, render, processes, serve, zip := commands.ForceStatic, commands.RenderStatic, commands.StaticWithProcesses, commands.ServeStatic, commands.ExportZip
	commands.ForceStatic, commands.RenderStatic = true, true
	commands.StaticWithProcesses, commands.ServeStatic, commands.ExportZip = false, false, false
	t.Cleanup(func() {
		commands.ForceStatic, commands.RenderStatic = force, render
		commands.StaticWithProcesses, commands.ServeStatic, commands.ExportZip = processes, serve, zip
	})
}

func TestStaticExportUsesOwnAssetsBesideDevelopmentSession(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, false, true)
	development := runtimeEsbuildRenderer()
	if err := development.BeginAssetSession(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(development.CloseAssets)
	response := f.request()
	if response.Code != http.StatusOK {
		t.Fatalf("initial page: %d %s", response.Code, response.Body.String())
	}
	developmentOutput := filepath.Join(f.static, "js", "app.js")
	before, err := os.ReadFile(developmentOutput)
	if err != nil || !bytes.Contains(before, []byte("initial")) {
		t.Fatalf("initial development output: %q %v", before, err)
	}
	f.write(filepath.Join(f.static, "manual.txt"), "preserved")
	f.setLabel("export-generation")
	configureStaticExportTest(t)
	if err := runStaticOperation(f.config); err != nil {
		t.Fatal(err)
	}
	if activeStaticExport != nil {
		t.Fatal("completed export left active operation state")
	}
	if f.config.Directories["static"] != f.static {
		t.Fatal("export changed source directory configuration")
	}
	after, err := os.ReadFile(developmentOutput)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("export changed development output: %q %v", after, err)
	}
	output := filepath.Join(f.config.Directories["render"], "static", "js", "app.js")
	data, err := os.ReadFile(output)
	if err != nil || !bytes.Contains(data, []byte("export-generation")) {
		t.Fatalf("export used stale development asset: %q %v", data, err)
	}
	manual, err := os.ReadFile(filepath.Join(f.config.Directories["render"], "static", "manual.txt"))
	if err != nil || string(manual) != "preserved" {
		t.Fatalf("manual asset lost: %q %v", manual, err)
	}
	page, err := os.ReadFile(filepath.Join(f.config.Directories["render"], "index.html"))
	if err != nil || !bytes.Contains(page, []byte(`/static/js/app.js`)) {
		t.Fatalf("exported page URL: %q %v", page, err)
	}
}

func TestStaticExportReservationPrecedesHooksAndDeletion(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, true)
	configureStaticExportTest(t)
	commands.StaticWithProcesses = true
	render := f.config.Directories["render"]
	sentinel := filepath.Join(render, "keep")
	f.write(sentinel, "keep")
	marker := filepath.Join(commands.GetModuleRoot(), "hook-ran")
	f.config.Hooks.BeforeStatic = []shared.DevelopmentTaskConfig{{Name: "unexpected", Command: []string{"sh", "-c", "touch hook-ran"}, Timeout: time.Second}}
	unlock, err := component.LockAssetSession(render, "export")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err := runStaticOperation(f.config); err == nil {
		t.Fatal("second export acquired an occupied destination")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatalf("occupied destination was modified: %q %v", data, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hook ran before destination reservation: %v", err)
	}
}

func TestStaticExportHTTPUsesOutputWithoutRebindingSources(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, true)
	plan := staticExportPlan{RenderDir: f.config.Directories["render"], StaticDir: f.static}
	f.write(filepath.Join(f.static, "identity.txt"), "source")
	if err := os.MkdirAll(filepath.Join(plan.RenderDir, "static"), 0755); err != nil {
		t.Fatal(err)
	}
	f.write(filepath.Join(plan.RenderDir, "static", "identity.txt"), "export")
	previous := activeStaticExport
	activeStaticExport = &staticExportAssets{plan: plan}
	t.Cleanup(func() { activeStaticExport = previous })
	response := httptest.NewRecorder()
	buildRuntimeHandler(nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/static/identity.txt", nil))
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != "export" {
		t.Fatalf("HTTP selected source static: %d %q", response.Code, response.Body.String())
	}
	if f.config.Directories["static"] != f.static {
		t.Fatal("source configuration was rebound")
	}
}

func TestStaticExportCancelledBeforeReservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := withStaticExportAssets(ctx, staticExportPlan{}, func(*staticExportAssets) error {
		t.Fatal("cancelled operation acquired assets")
		return nil
	})
	if err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestStaticExportImagesUseOwnOutputRoot(t *testing.T) {
	f, src := imageFlow(t, shared.DEVELOPMENT_MODE, true, true)
	developmentImages := runtimeImageProcessor()
	if err := developmentImages.BeginAssetSession(filepath.Join(f.static, "images")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(developmentImages.CloseAssets)
	oldURL, _ := imageFlowAsset(t, f)
	oldPath := filepath.Join(f.static, strings.TrimPrefix(oldURL, "/static/"))
	before, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFlowImage(t, src, 30)
	configureStaticExportTest(t)
	if err := runStaticOperation(f.config); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(f.config.Directories["render"], "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	match := imageFlowURL.FindStringSubmatch(string(page))
	if len(match) != 2 || match[1] == oldURL {
		t.Fatalf("export did not produce changed image: %s", page)
	}
	newPath := filepath.Join(f.config.Directories["render"], strings.TrimPrefix(match[1], "/"))
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("export image missing: %v", err)
	}
	after, err := os.ReadFile(oldPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("development image changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.static, strings.TrimPrefix(match[1], "/static/"))); !os.IsNotExist(err) {
		t.Fatalf("export image was published into source static: %v", err)
	}
}

func TestStaticExportFreshHistoryDoesNotAdoptCopiedFiles(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, true)
	configureStaticExportTest(t)
	if err := runStaticOperation(f.config); err != nil {
		t.Fatal(err)
	}
	render := f.config.Directories["render"]
	page, err := os.ReadFile(filepath.Join(render, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	url := esbuildScriptURL(t, string(page), "/static/js/app.")
	rel := strings.TrimPrefix(url, "/static/")
	data, err := os.ReadFile(filepath.Join(render, "static", rel))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(f.static, rel)), 0755); err != nil {
		t.Fatal(err)
	}
	f.write(filepath.Join(f.static, rel), string(data))
	f.write(f.source, "page:\n  - type: hypermedia\n  - route: index\n  - body:\n      - type: html\n      - value: '<p>no asset owner</p>'\n")
	if err := runStaticOperation(f.config); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(render, "static", rel))
	if err != nil || !bytes.Equal(copied, data) {
		t.Fatalf("previous export history deleted a newly copied file: %v", err)
	}
}

func TestStaticExportDestinationHeldThroughFinish(t *testing.T) {
	if !developmentProcessesSupported() {
		t.Skip("processes unavailable")
	}
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, true)
	configureStaticExportTest(t)
	commands.StaticWithProcesses = true
	t.Setenv("HB_TEST_STATIC_FINISH_LOCK", "1")
	f.config.Hooks.Finish = []shared.DevelopmentTaskConfig{{Name: "check-lock", Command: []string{os.Args[0], "-test.run=^TestStaticExportFinishLockChild$"}, Timeout: 10 * time.Second}}
	if err := runStaticOperation(f.config); err != nil {
		t.Fatal(err)
	}
	unlock, err := component.LockAssetSession(f.config.Directories["render"], "export")
	if err != nil {
		t.Fatalf("finished export retained destination: %v", err)
	}
	unlock()
}

func TestStaticExportFinishLockChild(t *testing.T) {
	if os.Getenv("HB_TEST_STATIC_FINISH_LOCK") != "1" || os.Getenv("HB_HOOK_PHASE") != "finish" {
		t.Skip("finish subprocess only")
	}
	root := os.Getenv("HB_RENDER_DIR")
	for _, lock := range []struct{ root, kind string }{
		{root, "export"}, {filepath.Join(root, "static"), "esbuild"}, {filepath.Join(root, "static", "images"), "image"},
	} {
		unlock, err := component.LockAssetSession(lock.root, lock.kind)
		if err == nil {
			unlock()
			t.Fatalf("finish hook ran after %s destination was unlocked", lock.kind)
		}
	}
}

func TestStaticExportRejectsSnapshotAssetCollisions(t *testing.T) {
	for _, scenario := range []string{"copied asset", "same render", "asset before snapshot", "snapshot before asset"} {
		t.Run(scenario, func(t *testing.T) {
			f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, false, true)
			configureStaticExportTest(t)
			commands.ExportZip = true
			previousOut := commands.ExportOutDir
			commands.ExportOutDir = t.TempDir()
			t.Cleanup(func() { commands.ExportOutDir = previousOut })
			const bundle = `bundle:
  - type: esbuild
  - entry: {path: {base: resources, path: main.js}}
  - outfile: {path: {base: static, path: js/app.js}}
  - fingerprint: false
  - cache: true
`
			const page = `page:
  - type: hypermedia
  - route: index
  - static: static/js/app.js
  - body:
      - type: html
      - value: '<p>snapshot</p>'
`
			const assetHead = `  - head:
      - type: head
      - script:
          - inherit: bundle
`
			source := bundle + page
			want := "conflicts with an existing asset"
			sourceAsset := filepath.Join(f.static, "js", "app.js")
			switch scenario {
			case "copied asset":
				writeTestFile(t, sourceAsset, "authored asset")
			case "same render":
				source += assetHead
			case "asset before snapshot", "snapshot before asset":
				output := "a.html"
				if scenario == "snapshot before asset" {
					output, want = "z.html", "changed by asset publication"
				}
				source += fmt.Sprintf("producer:\n  - type: hypermedia\n  - route: producer\n  - static: %s\n", output) + assetHead
			}
			f.write(f.source, source)
			if err := runStaticOperation(f.config); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected collision %q, got %v", want, err)
			}
			entries, err := os.ReadDir(commands.ExportOutDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed export produced an archive: %v %v", entries, err)
			}
			if scenario == "copied asset" {
				for _, filename := range []string{sourceAsset, filepath.Join(f.config.Directories["render"], "static", "js", "app.js")} {
					data, err := os.ReadFile(filename)
					if err != nil || string(data) != "authored asset" {
						t.Fatalf("collision overwrote copied asset: %q %v", data, err)
					}
				}
			} else if _, err := os.Stat(sourceAsset); !os.IsNotExist(err) {
				t.Fatalf("failed export changed source static: %v", err)
			}
		})
	}
}

func TestStaticExportRejectsRemovedEsbuildBinary(t *testing.T) {
	for _, value := range []string{`""`, "/usr/local/bin/esbuild"} {
		t.Run(value, func(t *testing.T) {
			f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, false, true)
			source, err := os.ReadFile(f.source)
			if err != nil {
				t.Fatal(err)
			}
			f.write(f.source, strings.Replace(string(source), "  - type: esbuild\n", "  - type: esbuild\n  - binary: "+value+"\n", 1))
			configureStaticExportTest(t)
			if err := runStaticOperation(f.config); err == nil || !strings.Contains(err.Error(), "esbuild binary is no longer supported") {
				t.Fatalf("removed compiler option was silently accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(f.config.Directories["render"], "index.html")); !os.IsNotExist(err) {
				t.Fatalf("invalid configuration produced a snapshot: %v", err)
			}
		})
	}
}

func TestStaticExportDirectHelperRebindsAssetsAndTemplates(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, false, true)
	configureStaticExportTest(t)
	previousManager := rm
	previousEsbuild := runtimeEsbuildRenderer()
	custom := &staticExportCustomRenderer{}
	rm.RegisterComponent(component.TextConfigGetName(), custom, reflect.TypeOf(component.TextConfig{}))
	plugin := &staticContextTestPlugin{}
	rm.SetPlugin("direct-export", plugin)
	parser.AddTemplate("export-template", "<p>{{.message}}</p>")
	configMutex.RLock()
	routes := make(map[string]map[string]interface{}, len(configs))
	for route, config := range configs {
		routes[route] = shared.CloneMapDeep(config)
		routes[route]["body"] = map[string]interface{}{
			"@type": "<TEMPLATE>", "template": "export-template",
			"values": map[string]interface{}{"message": "retained template provider"},
		}
	}
	configMutex.RUnlock()
	routes["custom"] = map[string]interface{}{"@type": component.TextConfigGetName(), "route": "custom"}
	routes["plugin"] = map[string]interface{}{"@type": component.PluginRenderGetName(), "route": "plugin", "plugin": "direct-export"}
	if err := PrepareForStaticRendering(routes); err != nil {
		t.Fatal(err)
	}
	if rm != previousManager || activeStaticExport != nil {
		t.Fatal("direct export did not restore runtime ownership")
	}
	if rm.GetRenderComponent(component.TextConfigGetName()) != custom || runtimeEsbuildRenderer() != previousEsbuild {
		t.Fatal("direct export replaced caller renderers")
	}
	if retained, ok := rm.GetPlugin("direct-export"); !ok || retained != plugin {
		t.Fatal("direct export replaced caller plugin instance")
	}
	for name, want := range map[string]string{"custom.html": "custom renderer retained", "plugin.html": "static plugin output"} {
		body, err := os.ReadFile(filepath.Join(f.config.Directories["render"], name))
		if err != nil || !bytes.Contains(body, []byte(want)) {
			t.Fatalf("direct export lost caller behavior %s: %q %v", name, body, err)
		}
	}
	page, err := os.ReadFile(filepath.Join(f.config.Directories["render"], "index.html"))
	if err != nil || !bytes.Contains(page, []byte("retained template provider")) || !bytes.Contains(page, []byte("/static/js/app.js")) {
		t.Fatalf("direct export lost template or asset: %q %v", page, err)
	}
	if _, err := os.Stat(filepath.Join(f.static, "js", "app.js")); !os.IsNotExist(err) {
		t.Fatalf("direct export wrote to source static: %v", err)
	}
}

type staticExportCustomRenderer struct{}

func (*staticExportCustomRenderer) Types() []string { return []string{component.TextConfigGetName()} }
func (*staticExportCustomRenderer) Render(interface{}, context.Context) (string, []error) {
	return "custom renderer retained", nil
}

func TestStaticExportProtectsAssetReferencesInSnapshotsUnderStatic(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, true)
	configureStaticExportTest(t)
	rm.RegisterComponent(component.TextConfigGetName(), &staticExportChangingAssetRenderer{entry: f.entry, outfile: filepath.Join(f.static, "js", "vary.js")}, reflect.TypeOf(component.TextConfig{}))
	routes := map[string]map[string]interface{}{}
	for _, route := range []string{"first", "second"} {
		routes[route] = map[string]interface{}{"@type": component.TextConfigGetName(), "route": route, "static": "static/" + route + ".html"}
	}
	if err := PrepareForStaticRendering(routes); err != nil {
		t.Fatal(err)
	}
	for route := range routes {
		page, err := os.ReadFile(filepath.Join(f.config.Directories["render"], "static", route+".html"))
		if err != nil {
			t.Fatal(err)
		}
		url := esbuildScriptURL(t, string(page), "/static/js/vary.")
		asset, err := os.ReadFile(filepath.Join(f.config.Directories["render"], filepath.FromSlash(strings.TrimPrefix(url, "/"))))
		if err != nil || !bytes.Contains(asset, []byte(route)) {
			t.Fatalf("snapshot %s lost its referenced generation %s: %q %v", route, url, asset, err)
		}
	}
}

type staticExportChangingAssetRenderer struct{ entry, outfile string }

func (*staticExportChangingAssetRenderer) Types() []string {
	return []string{component.TextConfigGetName()}
}
func (r *staticExportChangingAssetRenderer) Render(_ interface{}, ctx context.Context) (string, []error) {
	request := ctx.Value(shared.Request).(*http.Request)
	if err := os.WriteFile(r.entry, []byte(fmt.Sprintf("console.log(%q);", request.URL.Path)), 0644); err != nil {
		return "", []error{err}
	}
	config := component.EsbuildConfig{
		Entry: r.entry, Outfile: r.outfile, Fingerprint: true,
	}
	config.Enclose = `<script src="|"></script>`
	return runtimeEsbuildRenderer().Prepare(config).Render(ctx)
}

func TestStaticExportBesideActualDevelopmentProcess(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, false, true)
	encoded, err := json.Marshal(f.config.Directories)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStaticExportDevelopmentProcessChild$")
	cmd.Env = append(os.Environ(), "HB_TEST_STATIC_DEVELOPMENT=1", "HB_TEST_STATIC_MODULE="+commands.GetModuleRoot(), "HB_TEST_STATIC_CONFIG="+string(encoded))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if url, ok := strings.CutPrefix(scanner.Text(), "STATIC-DEVELOPMENT-READY "); ok {
				ready <- url
				_, _ = io.Copy(io.Discard, stdout)
				return
			}
			fmt.Fprintln(os.Stderr, scanner.Text())
		}
		ready <- ""
	}()
	var base string
	select {
	case base = <-ready:
		if base == "" {
			t.Fatal("development subprocess exited before readiness")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("development subprocess did not become ready")
	}
	get := func(path string) []byte {
		t.Helper()
		response, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("development response %s: %d %s %v", path, response.StatusCode, body, err)
		}
		return body
	}
	get("/")
	before := get("/static/js/app.js")
	if !bytes.Contains(before, []byte("initial")) {
		t.Fatalf("development initial asset: %s", before)
	}
	f.setLabel("from-export-process")
	configureStaticExportTest(t)
	if err := runStaticOperation(f.config); err != nil {
		t.Fatal(err)
	}
	if after := get("/static/js/app.js"); !bytes.Equal(before, after) {
		t.Fatal("export changed the running development process asset")
	}
	output, err := os.ReadFile(filepath.Join(f.config.Directories["render"], "static", "js", "app.js"))
	if err != nil || !bytes.Contains(output, []byte("from-export-process")) {
		t.Fatalf("isolated export asset: %q %v", output, err)
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestStaticExportDevelopmentProcessChild(t *testing.T) {
	if os.Getenv("HB_TEST_STATIC_DEVELOPMENT") != "1" {
		t.Skip("development subprocess only")
	}
	setupDevelopmentModeServeContentTest(t, false)
	configuration := getHyperBricksConfiguration()
	if err := json.Unmarshal([]byte(os.Getenv("HB_TEST_STATIC_CONFIG")), &configuration.Directories); err != nil {
		t.Fatal(err)
	}
	configuration.RateLimit.Enabled = false
	configuration.Server.Beautify = false
	commands.ModuleRoot = os.Getenv("HB_TEST_STATIC_MODULE")
	commands.RenderStatic = false
	parser.HbConfig = map[string]interface{}{}
	if err := basic_initialisation(); err != nil {
		t.Fatal(err)
	}
	defer closeRuntimeAssets()
	server := httptest.NewServer(buildRuntimeHandler(nil))
	defer server.Close()
	fmt.Println("STATIC-DEVELOPMENT-READY " + server.URL)
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
