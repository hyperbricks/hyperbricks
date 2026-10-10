// Package staticserver serves existing static output without loading a module.
package staticserver

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Handler confines file access to its open directory, including through symlinks.
// Call Close after the HTTP server has finished serving requests.
type Handler struct {
	root    *os.Root
	routing shared.RoutingConfig
}

// New opens an existing directory and copies its routing configuration. It does
// not read package configuration, render pages, or create files.
func New(directory string, routing shared.RoutingConfig) (*Handler, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	return &Handler{root: root, routing: NormalizeRouting(routing)}, nil
}

func (h *Handler) Close() error { return h.root.Close() }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requestPath := r.URL.Path
	if requestPath == "" {
		requestPath = "/"
	}
	if !validRequestPath(requestPath) {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(requestPath, "/")
	if strings.HasSuffix(requestPath, "/") {
		if h.serveIndex(w, r, name) {
			return
		}
		http.NotFound(w, r)
		return
	}
	// Preserve the export preview's extension-before-exact-file precedence.
	if h.routing.CleanURLs && !strings.Contains(path.Base(name), ".") {
		for _, extension := range h.routing.Extensions {
			if h.serveFile(w, r, name+"."+extension) {
				return
			}
		}
	}
	file, info, err := h.open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	if info.Mode().IsRegular() {
		http.ServeContent(w, r, path.Base(name), info.ModTime(), file)
		return
	}
	if info.IsDir() && h.hasIndex(name+"/") {
		location := (&url.URL{Path: requestPath + "/", RawQuery: r.URL.RawQuery, ForceQuery: r.URL.ForceQuery}).String()
		http.Redirect(w, r, location, http.StatusMovedPermanently)
		return
	}
	http.NotFound(w, r)
}

func validRequestPath(name string) bool {
	if !strings.HasPrefix(name, "/") || strings.Contains(name, "//") || strings.ContainsAny(name, "\\\x00") {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func (h *Handler) open(name string) (*os.File, os.FileInfo, error) {
	file, err := openFile(h.root, name)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	file, info, err := h.open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	if !info.Mode().IsRegular() {
		return false
	}
	http.ServeContent(w, r, path.Base(name), info.ModTime(), file)
	return true
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request, directory string) bool {
	for _, index := range h.routing.IndexFiles {
		if h.serveFile(w, r, directory+index) {
			return true
		}
	}
	return false
}

func (h *Handler) hasIndex(directory string) bool {
	for _, index := range h.routing.IndexFiles {
		file, info, err := h.open(directory + index)
		if err == nil {
			file.Close()
			if info.Mode().IsRegular() {
				return true
			}
		}
	}
	return false
}

// NormalizeRouting preserves runtime/export defaults and never changes the
// caller's slices or the explicit CleanURLs setting.
func NormalizeRouting(routing shared.RoutingConfig) shared.RoutingConfig {
	if len(routing.IndexFiles) == 0 {
		routing.IndexFiles = []string{"index.html", "index.htm"}
	}
	if len(routing.Extensions) == 0 {
		routing.Extensions = []string{"html", "htm"}
	}
	indexFiles := make([]string, 0, len(routing.IndexFiles))
	for _, name := range routing.IndexFiles {
		name = strings.TrimSpace(name)
		name = strings.TrimPrefix(name, "/")
		if name != "" {
			indexFiles = append(indexFiles, name)
		}
	}
	if len(indexFiles) == 0 {
		indexFiles = []string{"index.html"}
	}
	routing.IndexFiles = indexFiles
	extensions := make([]string, 0, len(routing.Extensions))
	for _, extension := range routing.Extensions {
		extension = strings.TrimSpace(extension)
		extension = strings.TrimPrefix(extension, ".")
		if extension != "" {
			extensions = append(extensions, extension)
		}
	}
	if len(extensions) == 0 {
		extensions = []string{"html"}
	}
	routing.Extensions = extensions
	return routing
}
