package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
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
func initStaticFileServer(limiter *rate.Limiter) {
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

func PrepareForStaticRendering(tempConfigs map[string]map[string]interface{}) {

	hbConfig := shared.GetHyperBricksConfiguration()
	logger := logging.GetLogger()

	if commands.RenderStatic {

		fmt.Println("\n\n\n\nSTATIC RENDERING...")
		renderDir := ""
		if tbrender, ok := hbConfig.Directories["render"]; ok {
			renderDir = tbrender
		}

		staticDir := ""
		if tbstatic, ok := hbConfig.Directories["static"]; ok {
			staticDir = tbstatic
		}

		if renderDir == "" || staticDir == "" {
			return
		}

		logger.Infow("Copying static directory", "source", staticDir, "destination", renderDir)

		if err := validatePath(renderDir); err != nil {
			logger.Errorw("Path validation failed", "path", renderDir, "error", err)
		}

		// Prompt for confirmation before deleting
		absPath, _ := filepath.Abs(renderDir)
		if !confirmAction(fmt.Sprintf("\n\nDo you want to remove all files in %s before rendering the new files?", absPath)) {
			fmt.Println("Leave render dir as it is... Continue rendering.")
		} else {
			err := os.RemoveAll(renderDir)
			if err != nil {
				logger.Errorw("Error removing destination directory", "directory", renderDir, "error", err)
			}

			err = os.MkdirAll(renderDir, 0755)
			if err != nil {
				logger.Errorw("Error creating destination directory", "directory", renderDir, "error", err)
			}
		}

		err := makeStatic(tempConfigs, renderDir)
		if err != nil {
			logger.Errorw("Error creating static files", "error", err)
		}

		err = copy.Copy(staticDir, filepath.Join(renderDir, "static"))
		if err != nil {
			logger.Errorw("Error copying directory", "source", staticDir, "destination", filepath.Join(renderDir, "static"), "error", err)
		} else {
			logger.Infow("Copied static file directory successfully", "source", staticDir, "destination", filepath.Join(renderDir, "static"))
		}
	}

}

// confirmAction prompts the user for confirmation before proceeding.
func confirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (yes/no): ", prompt)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes"
}
