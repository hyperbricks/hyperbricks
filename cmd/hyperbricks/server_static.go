package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/otiai10/copy"
)

func initStaticFileServer() {
	hbConfig := getHyperBricksConfiguration()
	staticPath := hbConfig.Directories["static"]

	// Create http.FileSystems for both embedded and static directories
	staticFS := http.Dir(staticPath)

	// Use a single handler for the defined directories
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/static/"):
			// Serve files from the static directory
			http.StripPrefix("/static/", http.FileServer(staticFS)).ServeHTTP(w, r)
		default:
			// Your custom logic for other paths
			handler(w, r)
		}
	})
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
		err := os.RemoveAll(renderDir)
		if err != nil {
			logger.Errorw("Error removing destination directory", "directory", renderDir, "error", err)
		}

		err = os.MkdirAll(renderDir, 0755)
		if err != nil {
			logger.Errorw("Error creating destination directory", "directory", renderDir, "error", err)
		}

		err = makeStatic(tempConfigs, renderDir)
		if err != nil {
			logger.Errorw("Error creating static files", "error", err)
		}

		err = copy.Copy(staticDir, filepath.Join(renderDir, "static"))
		if err != nil {
			logger.Errorw("Error copying directory", "source", staticDir, "destination", filepath.Join(renderDir, "static"), "error", err)
		} else {
			logger.Infow("Directory copied successfully", "source", staticDir, "destination", filepath.Join(renderDir, "static"))
		}

	}

}
