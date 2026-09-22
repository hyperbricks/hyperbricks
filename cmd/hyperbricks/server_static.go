package main

import (
	"context"
	"embed"
	"fmt"
	"net"
	"net/http"
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
		return baseHandler
	}
	// Wrap the base handler with the rate limiting middleware.
	return rateLimitMiddleware(limiter)(baseHandler)
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

func PrepareForStaticRendering(tempConfigs map[string]map[string]interface{}) error {

	hbConfig := shared.GetHyperBricksConfiguration()
	logger := logging.GetLogger().Named("static")
	logger.Info("Rendering routes")

	renderDir := ""
	if tbrender, ok := hbConfig.Directories["render"]; ok {
		renderDir = tbrender
	}

	staticDir := ""
	if tbstatic, ok := hbConfig.Directories["static"]; ok {
		staticDir = tbstatic
	}

	if renderDir == "" || staticDir == "" {
		return nil
	}

	// Validate renderDir is inside ./modules
	if err := validatePath(renderDir); err != nil {
		return fmt.Errorf("validate render directory %q: %w", renderDir, err)
	}

	shouldDelete := commands.ForceStatic
	if !commands.StaticWizard {
		shouldDelete = commands.ForceStatic || confirmDeletion(renderDir)
	}
	if !shouldDelete {
		logger.Infow("User skipped deletion", "directory", renderDir)
	} else {
		logger.Infow("Deleting all files in ", "directory", renderDir)
		err := os.RemoveAll(renderDir)
		if err != nil {
			return fmt.Errorf("remove render directory %q: %w", renderDir, err)
		}
	}

	logger.Infow("Copying static directory", "source", staticDir, "destination", renderDir)

	err := os.MkdirAll(renderDir, 0755)
	if err != nil {
		return fmt.Errorf("create render directory %q: %w", renderDir, err)
	}

	err = snapshotStaticRoutes(tempConfigs, renderDir)
	if err != nil {
		return err
	}

	err = copy.Copy(staticDir, filepath.Join(renderDir, "static"))
	if err != nil {
		return fmt.Errorf("copy static directory %q to %q: %w", staticDir, filepath.Join(renderDir, "static"), err)
	} else {
		logger.Infow("Copied static file directory successfully", "source", staticDir, "destination", filepath.Join(renderDir, "static"))
	}

	if commands.ExportZip {
		if commands.StaticWizard && strings.TrimSpace(commands.ExportExclude) == "" {
			excludeCSV, err := commands.RunStaticExcludePicker(renderDir)
			if err != nil {
				logger.Errorw("Error selecting zip excludes", "error", err)
			} else {
				commands.ExportExclude = excludeCSV
			}
		}
		exportPath, err := exportStaticZip(renderDir, commands.StartModule, commands.ExportOutDir, commands.ExportExclude)
		if err != nil {
			return fmt.Errorf("export static zip: %w", err)
		} else {
			logger.Infow("Created static export", "path", exportPath)
		}
	}

	logger.Infow("Rendering complete", "directory", renderDir)
	return nil

}

func serveStatic() error {
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

	// Run server in background goroutine
	go func() {
		logging.GetLogger().Named("static").Infow("Listening", "url", "http://"+addr)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logging.GetLogger().Named("static").Fatalw("Server failed", "error", err)
		}
	}()

	if os.Getenv("HB_NO_KEYBOARD") != "" || !logging.IsTerminal(os.Stdin) {
		return waitForServerSignal(server)
	}

	// Open keyboard for input
	if err := keyboard.Open(); err != nil {
		logging.GetLogger().Warnw("Failed to open keyboard", "error", err)
		return err
	}
	defer keyboard.Close()

	logging.GetLogger().Info("Press q, Esc or Ctrl+C to stop")

	// Wait for q, ESC, or Ctrl+C
	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			logging.GetLogger().Warnw("Keyboard input unavailable", "error", err)
			break
		}
		if char == 'q' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			logging.GetLogger().Named("static").Info("Stopping")
			break
		}
	}

	// Gracefully shutdown server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logging.GetLogger().Named("static").Errorw("Shutdown failed", "error", err)
		return err
	}

	logging.GetLogger().Named("static").Info("Stopped")
	return nil
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
