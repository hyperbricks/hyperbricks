package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestMarkdownFlowFreshnessAndPageCache(t *testing.T) {
	for _, mode := range []string{shared.DEVELOPMENT_MODE, shared.LIVE_MODE} {
		for _, cached := range []bool{false, true} {
			for _, nocache := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/cache=%t/nocache=%t", mode, cached, nocache), func(t *testing.T) {
					f := newEsbuildFlowFixture(t, mode, true, true, nocache)
					file := filepath.Join(f.config.Directories["resources"], "document.md")
					f.write(file, "# Original")
					source := func(body string) {
						f.write(f.source, fmt.Sprintf("page:\n  - type: hypermedia\n  - route: index\n  - nocache: %t\n  - body:\n      - type: markdown\n      - cache: %t\n%s", nocache, cached, body))
					}
					source("      - file: document.md\n")
					if err := PreProcessAndPopulateConfigs(); err != nil {
						t.Fatal(err)
					}
					check := func(want string) {
						t.Helper()
						r := f.request()
						if r.Code != 200 || !strings.Contains(r.Body.String(), "<h1>"+want+"</h1>") {
							t.Fatalf("%d %s", r.Code, r.Body.String())
						}
					}
					check("Original")
					check("Original")
					f.write(file, "# Modified")
					if mode == shared.LIVE_MODE && !nocache {
						check("Original")
						f.expirePage()
					}
					check("Modified")
					check("Modified")
					source("      - content: '# Inline'\n")
					if err := PreProcessAndPopulateConfigs(); err != nil {
						t.Fatal(err)
					}
					check("Inline")
					source("      - content: '# Changed'\n")
					if err := PreProcessAndPopulateConfigs(); err != nil {
						t.Fatal(err)
					}
					check("Changed")
				})
			}
		}
	}
}

// The load-time resolver requires a real watch reload, unlike direct file reads.
func TestMarkdownFlowWatchRefreshesResolvedContent(t *testing.T) {
	f := newEsbuildFlowFixture(t, shared.DEVELOPMENT_MODE, true, true, true)
	file := filepath.Join(f.config.Directories["resources"], "document.md")
	f.write(file, "# Before")
	f.write(f.source, `page:
  - type: hypermedia
  - route: index
  - body:
      - type: markdown
      - cache: true
      - content: {file: {base: resources, path: document.md}}
`)
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatal(err)
	}
	if out := f.request().Body.String(); !strings.Contains(out, "<h1>Before</h1>") {
		t.Fatal(out)
	}
	events := make(chan error, 8)
	reloads := newRuntimeReload(func() { events <- PreProcessAndPopulateConfigs() })
	reloads.Enable()
	ctx, cancel := context.WithCancel(context.Background())
	done, err := startRuntimeWatcher(ctx, []string{f.config.Directories["resources"]}, reloads.Request)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		<-done
		stopCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := reloads.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	})
	f.write(file, "# After")
	select {
	case err := <-events:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not reload Markdown")
	}
	stopCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := reloads.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if out := f.request().Body.String(); !strings.Contains(out, "<h1>After</h1>") {
		t.Fatal(out)
	}
}
