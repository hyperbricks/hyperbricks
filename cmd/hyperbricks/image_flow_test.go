package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func writeFlowImage(t *testing.T, path string, red uint8) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 12, 6))
	for y := range 6 {
		for x := range 12 {
			img.SetRGBA(x, y, color.RGBA{R: red, A: 255})
		}
	}
	err = png.Encode(f, img)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("write image %v %v", err, closeErr)
	}
}
func imageFlow(t *testing.T, mode string, cached, nocache bool) (*esbuildFlowFixture, string) {
	t.Helper()
	f := newEsbuildFlowFixture(t, mode, true, true, nocache)
	src := filepath.Join(f.config.Directories["resources"], "photo.png")
	writeFlowImage(t, src, 200)
	f.write(f.source, fmt.Sprintf(`page:
  - type: hypermedia
  - route: index
  - nocache: %t
  - body:
      - type: image
      - src: %q
      - width: 4
      - cache: %t
      - cache_grace: 0s
`, nocache, src, cached))
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatal(err)
	}
	return f, src
}

var imageFlowURL = regexp.MustCompile(`<img src="([^"]+)"`)

func imageFlowAsset(t *testing.T, f *esbuildFlowFixture) (string, os.FileInfo) {
	t.Helper()
	response := f.request()
	match := imageFlowURL.FindStringSubmatch(response.Body.String())
	if response.Code != 200 || len(match) != 2 {
		t.Fatalf("image page: %d %s", response.Code, response.Body.String())
	}
	asset := httptest.NewRecorder()
	f.handler.ServeHTTP(asset, httptest.NewRequest("GET", match[1], nil))
	if asset.Code != 200 {
		t.Fatalf("image HTTP %d", asset.Code)
	}
	if _, _, err := image.Decode(asset.Body); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(f.static, strings.TrimPrefix(match[1], "/static/")))
	if err != nil {
		t.Fatal(err)
	}
	return match[1], info
}
func TestImageFlowModesPageCacheAndReload(t *testing.T) {
	for _, mode := range []string{shared.DEVELOPMENT_MODE, shared.LIVE_MODE} {
		for _, cached := range []bool{false, true} {
			for _, nocache := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/cache=%t/nocache=%t", mode, cached, nocache), func(t *testing.T) {
					f, src := imageFlow(t, mode, cached, nocache)
					entries, _ := os.ReadDir(f.static)
					if len(entries) != 0 {
						t.Fatal("init generated images")
					}
					firstURL, first := imageFlowAsset(t, f)
					_, again := imageFlowAsset(t, f)
					fresh := mode == shared.DEVELOPMENT_MODE || nocache
					if os.SameFile(first, again) != (!fresh || cached) {
						t.Fatal("unchanged image cache policy")
					}
					writeFlowImage(t, src, 40)
					if !fresh {
						url, same := imageFlowAsset(t, f)
						if url != firstURL || !os.SameFile(again, same) {
							t.Fatal("live page cache ran image generation")
						}
						f.expirePage()
					}
					changedURL, changed := imageFlowAsset(t, f)
					if changedURL == firstURL {
						t.Fatal("source change stale URL")
					}
					_, next := imageFlowAsset(t, f)
					if os.SameFile(changed, next) != (!fresh || cached) {
						t.Fatal("next request cache policy")
					}
					if err := PreProcessAndPopulateConfigs(); err != nil {
						t.Fatal(err)
					}
					// Reload discards page cache but retains valid image output on disk.
					url, reloaded := imageFlowAsset(t, f)
					if url != changedURL || cached && !os.SameFile(next, reloaded) {
						t.Fatal("reload did not reuse valid image")
					}
				})
			}
		}
	}
}
func TestImageFlowCleanupWaitsForRequestsCachedHTMLAndExport(t *testing.T) {
	f, src := imageFlow(t, shared.DEVELOPMENT_MODE, true, true)
	oldURL, _ := imageFlowAsset(t, f)
	oldPath := filepath.Join(f.static, strings.TrimPrefix(oldURL, "/static/"))
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	go func() {
		defer close(done)
		assetRequestMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	writeFlowImage(t, src, 40)
	imageFlowAsset(t, f)
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("request lost image")
	}
	htmlCacheMutex.Lock()
	htmlCache["image-protected"] = CacheEntry{Content: `<img src="` + oldURL + `">`}
	htmlCacheMutex.Unlock()
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request blocked")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("cached HTML lost image")
	}
	htmlCacheMutex.Lock()
	delete(htmlCache, "image-protected")
	htmlCacheMutex.Unlock()
	previous := commands.RenderStatic
	commands.RenderStatic = true
	t.Cleanup(func() { commands.RenderStatic = previous })
	if err := runtimeImageProcessor().ReconcileAssets(filepath.Join(f.static, "images"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("export lost image")
	}
	commands.RenderStatic = false
	assetRequestMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("unprotected image not removed", err)
	}
}
func TestImageFlowWatchExcludesGeneratedFiles(t *testing.T) {
	f, src := imageFlow(t, shared.DEVELOPMENT_MODE, true, true)
	imageFlowAsset(t, f)
	events := make(chan error, 8)
	reloads := newRuntimeReload(func() { events <- PreProcessAndPopulateConfigs() })
	reloads.Enable()
	ctx, cancel := context.WithCancel(context.Background())
	done, err := startRuntimeWatcher(ctx, []string{f.config.Directories["resources"], f.static}, reloads.Request)
	if err != nil {
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
	writeFlowImage(t, src, 40)
	select {
	case err := <-events:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not reload")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		reloads.mu.Lock()
		running := reloads.running
		reloads.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reload blocked")
		}
		time.Sleep(time.Millisecond)
	}
	imageFlowAsset(t, f)
	select {
	case err := <-events:
		t.Fatal("generated image triggered reload", err)
	case <-time.After(650 * time.Millisecond):
	}
}

func TestImageFlowInvalidDiscoveryDoesNotOrphanAssets(t *testing.T) {
	f, _ := imageFlow(t, shared.DEVELOPMENT_MODE, true, true)
	url, _ := imageFlowAsset(t, f)
	path := filepath.Join(f.static, strings.TrimPrefix(url, "/static/"))
	f.write(f.source, `page:
  - type: hypermedia
  - route: index
  - body:
      - type: image
      - src: missing.png
      - cache_keep: -1
`)
	// Invalid owner discovery records diagnostics and must not authorize cleanup.
	PreProcessAndPopulateConfigs()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("invalid config removed output", err)
	}
	f.write(f.source, `page:
  - type: hypermedia
  - route: index
  - body:
      - type: text
      - value: No images
`)
	if err := PreProcessAndPopulateConfigs(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("valid owner removal did not prune output", err)
	}
}

func TestImageFlowReloadDoesNotBlockNestedAssetRequest(t *testing.T) {
	f, _ := imageFlow(t, shared.DEVELOPMENT_MODE, true, true)
	url, _ := imageFlowAsset(t, f)
	started, release, outerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	go func() {
		defer close(outerDone)
		assetRequestMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(started); <-release })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-started
	configMutex.Lock()
	previous := routeGeneration
	configMutex.Unlock()
	reloaded := make(chan error, 1)
	go func() { reloaded <- PreProcessAndPopulateConfigs() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		configMutex.Lock()
		published := routeGeneration > previous
		configMutex.Unlock()
		if published {
			break
		}
		if time.Now().After(deadline) {
			unblock()
			t.Fatal("reload did not publish")
		}
		time.Sleep(time.Millisecond)
	}
	// Let maintenance reach its write-gate decision before the nested request.
	time.Sleep(20 * time.Millisecond)
	nested := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		nested <- w.Code
	}()
	select {
	case code := <-nested:
		if code != 200 {
			t.Errorf("nested asset HTTP %d", code)
		}
	case <-time.After(time.Second):
		t.Error("reload blocks nested request required by an active API render")
	}
	unblock()
	select {
	case <-outerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("outer request did not stop")
	}
	select {
	case err := <-reloaded:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reload did not stop")
	}
}

func TestImageFlowReloadRetiresOwnersAfterRequests(t *testing.T) {
	f, _ := imageFlow(t, shared.DEVELOPMENT_MODE, true, true)
	url, _ := imageFlowAsset(t, f)
	path := filepath.Join(f.static, strings.TrimPrefix(url, "/static/"))
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	go func() {
		defer close(done)
		assetRequestMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(started); <-release })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-started
	f.write(f.source, `page:
  - type: hypermedia
  - route: index
  - body:
      - type: text
      - value: No images
`)
	reloaded := make(chan error, 1)
	go func() { reloaded <- PreProcessAndPopulateConfigs() }()
	select {
	case err := <-reloaded:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		unblock()
		t.Fatal("reload waited for active request")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("reload removed active request's image", err)
	}
	htmlCacheMutex.Lock()
	htmlCache["image-reload-protected"] = CacheEntry{Content: `<img src="` + url + `">`}
	htmlCacheMutex.Unlock()
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not stop")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("deferred cleanup removed cached image", err)
	}
	htmlCacheMutex.Lock()
	delete(htmlCache, "image-reload-protected")
	htmlCacheMutex.Unlock()
	assetRequestMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("deferred orphan cleanup not completed", err)
	}
}
