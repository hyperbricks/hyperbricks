package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/core"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/otiai10/copy"
	"golang.org/x/time/rate"
)

// rateLimitMiddleware wraps a handler with rate limiting.
func rateLimitMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// initStaticFileServer sets up the static file server and wraps the default handler with rate limiting.
func initStaticFileServer_pre_2025(limiter *rate.Limiter) {
	hbConfig := getHyperBricksConfiguration()
	staticPath := hbConfig.Directories["static"]

	// Create an http.FileSystem for the static directory.
	staticFS := http.Dir(staticPath)

	// Base handler: serves static files if the URL starts with "/static/",
	// otherwise falls back to your custom handler.
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/static/"):
			// Serve files from the static directory.
			http.StripPrefix("/static/", http.FileServer(staticFS)).ServeHTTP(w, r)
		default:
			// Your custom logic for other paths.
			handler(w, r)
		}
	})

	// Wrap the base handler with the rate limiting middleware.
	rateLimitedHandler := rateLimitMiddleware(limiter)(baseHandler)

	// Register the wrapped handler with the default mux.
	http.Handle("/", rateLimitedHandler)
}
func initStaticFileServer_v1(limiter *rate.Limiter) {
	tbConfig := getHyperBricksConfiguration()
	// Define the directory where static files are located
	outDir := "./frontend/assets/"
	staticPath := tbConfig.Directories["static"]
	// Define multiple root directories
	directories := map[string]string{
		"/static/": staticPath,
		"/out/":    outDir,
	}

	// Use a single file handler for the defined directories
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/out/") {
			FileHandler(directories)(w, r)
		} else {
			// Your custom logic for other paths
			handler(w, r)
		}
	})

	// Wrap the base handler with the rate limiting middleware.
	rateLimitedHandler := rateLimitMiddleware(limiter)(baseHandler)

	// Register the wrapped handler with the default mux.
	http.Handle("/", rateLimitedHandler)
}

//go:embed frontend/*
var frontendFiles embed.FS

func initStaticFileServer(limiter *rate.Limiter) {
	http.Handle("/", buildRuntimeHandler(limiter))
}

func buildRuntimeHandler(limiter *rate.Limiter) http.Handler {
	tbConfig := getHyperBricksConfiguration()
	staticPath := tbConfig.Directories["static"]

	// Create http.FileSystems for both embedded and static directories
	staticFS := http.Dir(staticPath)
	frontendFS := http.FS(frontendFiles)

	// Use a single handler for the defined directories
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleRuntimeGateway(w, r) {
			return
		}
		if handleFrontendEditor(w, r) {
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/static/"):
			// Serve files from the static directory
			http.StripPrefix("/static/", http.FileServer(staticFS)).ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/out/"):
			// Serve embedded files
			http.StripPrefix("/out/", http.FileServer(frontendFS)).ServeHTTP(w, r)
		default:
			// Your custom logic for other paths
			handler(w, r)
		}
	})
	if limiter == nil {
		return assetRequestMiddleware(baseHandler)
	}
	// Wrap the base handler with the rate limiting middleware.
	return rateLimitMiddleware(limiter)(assetRequestMiddleware(baseHandler))
}

// FileHandler routes requests to the appropriate directory based on the URL path
func FileHandler(dirs map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Iterate over the defined directories
		for prefix, dir := range dirs {
			if strings.HasPrefix(r.URL.Path, prefix) {
				// Trim the prefix and serve the file from the corresponding directory
				http.StripPrefix(prefix, http.FileServer(http.Dir(dir))).ServeHTTP(w, r)
				return
			}
		}

		// If no match, return a 404
		http.NotFound(w, r)
	}
}

type staticExportPlan struct{ RenderDir, StaticDir string }
type staticExportResult struct{ RenderDir, ZipPath string }

var errStaticDeclined = errors.New("static export declined")

func planStaticExport(config *shared.Config) (staticExportPlan, error) {
	var plan staticExportPlan
	var err error
	if config.Directories["render"] == "" || config.Directories["static"] == "" {
		return plan, fmt.Errorf("static export requires render and static directories")
	}
	plan.RenderDir, err = filepath.Abs(config.Directories["render"])
	if err != nil {
		return plan, err
	}
	plan.StaticDir, err = filepath.Abs(config.Directories["static"])
	if err != nil {
		return plan, err
	}
	if err = validatePath(plan.RenderDir); err != nil {
		return plan, err
	}
	if _, err = parseExcludeList(commands.ExportExclude); err != nil {
		return plan, err
	}
	if !commands.ForceStatic && !commands.StaticWizard && !confirmDeletion(plan.RenderDir) {
		return plan, errStaticDeclined
	}
	return plan, nil
}

// PrepareForStaticRendering is retained for direct export callers. The CLI
// plans before preparation and invokes executeStaticExport after initialization.
func PrepareForStaticRendering(tempConfigs map[string]map[string]interface{}) error {
	plan, err := planStaticExport(shared.GetHyperBricksConfiguration())
	if err != nil {
		return err
	}
	_, err = executeStaticExport(tempConfigs, plan)
	return err
}

func executeStaticExport(tempConfigs map[string]map[string]interface{}, plan staticExportPlan) (staticExportResult, error) {
	return executeStaticExportContext(context.Background(), tempConfigs, plan)
}

func executeStaticExportContext(ctx context.Context, tempConfigs map[string]map[string]interface{}, plan staticExportPlan) (staticExportResult, error) {
	result := staticExportResult{RenderDir: plan.RenderDir}
	renderDir, staticDir := plan.RenderDir, plan.StaticDir
	logger := logging.GetLogger().Named("static")
	if err := os.RemoveAll(renderDir); err != nil {
		return result, fmt.Errorf("remove render directory: %w", err)
	}
	logger.Infow("Copying static directory", "source", staticDir, "destination", renderDir)

	err := os.MkdirAll(renderDir, 0755)
	if err != nil {
		return result, fmt.Errorf("create render directory %q: %w", renderDir, err)
	}

	err = snapshotStaticRoutesContext(ctx, tempConfigs, renderDir)
	if err != nil {
		if renderer := runtimeEsbuildRenderer(); renderer != nil {
			if cleanup := renderer.AssetFailure(); cleanup != nil {
				return result, phaseError("asset_cleanup", errors.Join(err, cleanup))
			}
		}
		return result, err
	}

	if renderer := runtimeEsbuildRenderer(); renderer != nil {
		// Snapshot files are now the response owners; protect their references
		// while pruning before assets are copied into the export.
		var pages strings.Builder
		if err := filepath.WalkDir(renderDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			pages.Write(raw)
			pages.WriteByte('\n')
			return nil
		}); err != nil {
			return result, phaseError("asset_cleanup", err)
		}
		rendered := pages.String()
		renderer.SetAssetProtection(func(output string) bool {
			relative, err := filepath.Rel(staticDir, output)
			if err != nil {
				return true
			}
			public := (&url.URL{Path: "/static/" + filepath.ToSlash(relative)}).EscapedPath()
			return strings.Contains(rendered, public)
		})
		defer renderer.SetAssetProtection(protectRuntimeAsset)
		if err := renderer.ReconcileAssets(nil); err != nil {
			return result, phaseError("asset_cleanup", err)
		}
	}
	err = copy.Copy(staticDir, filepath.Join(renderDir, "static"))
	if err != nil {
		return result, phaseError("copy_assets", fmt.Errorf("copy static directory %q to %q: %w", staticDir, filepath.Join(renderDir, "static"), err))
	} else {
		logger.Infow("Copied static file directory successfully", "source", staticDir, "destination", filepath.Join(renderDir, "static"))
	}

	if commands.ExportZip {
		exportPath, err := exportStaticZip(renderDir, commands.StartModule, commands.ExportOutDir, commands.ExportExclude)
		if err != nil {
			return result, phaseError("package", err)
		} else {
			result.ZipPath, err = filepath.Abs(exportPath)
			if err != nil {
				return result, err
			}
			logger.Infow("Created static export", "path", exportPath)
		}
	}

	logger.Infow("Rendering complete", "directory", renderDir)
	return result, nil

}

func serveStatic() error { return serveStaticContext(context.Background()) }

func serveStaticContext(operation context.Context) error {
	hbConfig := shared.GetHyperBricksConfiguration()

	// Get host IPv4 addresses
	ips, err := getHostIPv4s()
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("no IPv4 addresses found: %w", err)
	}

	ip := ips[0]
	port := hbConfig.Server.Port
	if commands.StaticServePort > 0 {
		port = commands.StaticServePort
	}
	portStr := strconv.Itoa(port)
	addr := ip + ":" + portStr

	renderDir := hbConfig.Directories["render"]
	if strings.TrimSpace(renderDir) == "" {
		renderDir = core.ModuleDirectories.RenderedDir
	}
	routing := normalizeRoutingConfig(hbConfig.Server.Routing)
	fileServer := http.FileServer(http.Dir(renderDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := r.URL.Path
		if requestPath == "" {
			requestPath = "/"
		}

		if strings.HasSuffix(requestPath, "/") {
			for _, indexFile := range routing.IndexFiles {
				candidate := requestPath + indexFile
				if filePath, ok := staticFilePath(renderDir, candidate); ok {
					serveStaticFile(w, r, filePath)
					return
				}
			}
		}

		if routing.CleanURLs {
			base := path.Base(requestPath)
			if !strings.Contains(base, ".") {
				for _, ext := range routing.Extensions {
					if ext == "" {
						continue
					}
					candidate := requestPath + "." + ext
					if staticFileExists(renderDir, candidate) {
						r.URL.Path = candidate
						fileServer.ServeHTTP(w, r)
						return
					}
				}
			}
		}

		fileServer.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: addr, Handler: handler}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer server.Close()

	served := make(chan error, 1)
	go func() {
		logging.GetLogger().Named("static").Infow("Listening", "url", "http://"+addr)
		served <- server.Serve(listener)
	}()
	var keys <-chan keyboard.KeyEvent
	if os.Getenv("HB_NO_KEYBOARD") == "" && logging.IsTerminal(os.Stdin) {
		events, err := keyboard.GetKeys(10)
		if err != nil {
			return err
		}
		keys = events
		defer keyboard.Close()
		logging.GetLogger().Info("Press q, Esc or Ctrl+C to stop")
	}
	var result error
wait:
	for {
		select {
		case <-operation.Done():
			result = context.Cause(operation)
			break wait
		case err := <-served:
			if err != http.ErrServerClosed {
				result = err
			}
			break wait
		case event, ok := <-keys:
			if !ok {
				keys = nil
				continue
			}
			if event.Err != nil {
				result = event.Err
				break wait
			}
			if event.Key == keyboard.KeyCtrlC {
				result = errRuntimeInterrupt
				break wait
			}
			if event.Rune == 'q' || event.Key == keyboard.KeyEsc {
				break wait
			}
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		result = errors.Join(result, phaseError("cleanup", err))
	}
	logging.GetLogger().Named("static").Info("Stopped")
	return result
}

func waitForServerSignal(server *http.Server) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logging.GetLogger().Named("static").Errorw("Shutdown failed", "error", err)
		return err
	}

	logging.GetLogger().Named("static").Info("Stopped")
	return nil
}

func staticFileExists(rootDir, urlPath string) bool {
	_, ok := staticFilePath(rootDir, urlPath)
	return ok
}

func staticFilePath(rootDir, urlPath string) (string, bool) {
	cleanPath := path.Clean("/" + urlPath)
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if cleanPath == "" {
		return "", false
	}
	fullPath := filepath.Join(rootDir, filepath.FromSlash(cleanPath))
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return "", false
	}
	return fullPath, true
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}
