package spaces

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/ui"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

//go:embed web/*
var webFiles embed.FS

type editorOptions = shared.SpacesConfig
type service struct {
	dashboard                   bool
	module, route, publicOrigin string
	dirs                        map[string]string
	write, watch                bool
	allowedHosts                []string
	sharingImage                *UploadPolicy
	config                      map[string]interface{}
}

// Handler serializes editorial operations for the served module. Its zero value is ready to use.
type Handler struct{ mu sync.Mutex }

func newService(module, route string, dirs map[string]string, options editorOptions, watch bool) (*service, error) {
	module, err := filepath.Abs(module)
	if err != nil {
		return nil, err
	}
	module, err = filepath.EvalSymlinks(module)
	if err != nil {
		return nil, err
	}
	s := &service{module: module, route: route, dirs: map[string]string{}, write: options.Write, watch: watch, allowedHosts: options.AllowedHosts, sharingImage: options.SharingImage}
	for _, base := range []string{"hyperbricks", "resources", "templates", "static"} {
		dir := dirs[base]
		if dir == "" {
			dir = filepath.Join(module, base)
		}
		absolute, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		// Module configuration may use the OS alias of a temporary directory.
		if resolved, e := filepath.EvalSymlinks(absolute); e == nil && strings.HasPrefix(resolved, module+string(filepath.Separator)) {
			absolute = resolved
		}
		rel, err := filepath.Rel(module, absolute)
		if err != nil || rel == "." {
			return nil, fmt.Errorf("%s must be a directory inside the module", base)
		}
		s.dirs[base], err = contained(module, rel)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", base, err)
		}
	}
	s.publicOrigin, err = publicOrigin(options.PublicOrigin)
	if err != nil {
		return nil, err
	}
	if s.sharingImage != nil {
		if err := validateUploadPolicy(s.sharingImage); err != nil {
			return nil, err
		}
		if s.sharingImage.Directory.Base != "static" {
			return nil, fmt.Errorf("sharing_image must use static storage")
		}
		for _, ext := range s.sharingImage.Accept {
			if ext == ".md" || ext == ".markdown" {
				return nil, fmt.Errorf("sharing_image accepts images only")
			}
		}
	}
	return s, nil
}
func (s *service) parserOptions() yamlparser.Options {
	return yamlparser.Options{Config: s.config, TemplateDir: s.dirs["templates"], Paths: yamlparser.PathMarkers{ModuleRoot: s.module, Module: s.module, Resources: s.dirs["resources"], Templates: s.dirs["templates"], Static: s.dirs["static"], HyperBricks: s.dirs["hyperbricks"]}}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cfg := shared.GetHyperBricksConfiguration()
	runtime := shared.GetRuntimeOptions()
	if !shared.SpacesAvailable(cfg, runtime) {
		http.NotFound(w, r)
		return
	}
	options := cfg.Development.FrontendEditing.Spaces
	if r.URL.Path != options.Route && !strings.HasPrefix(r.URL.Path, options.Route+"/") {
		http.NotFound(w, r)
		return
	}
	if !shared.RequireSpacesAuth(w, r, cfg.Development.Dashboard.Credentials) {
		return
	}
	s, err := newService(runtime.ModuleRoot, options.Route, cfg.Directories, options, cfg.Mode == shared.DEVELOPMENT_MODE && cfg.Development.Watch)
	if err != nil {
		logging.GetLogger().Errorw("Spaces initialization failed", "error", err)
		http.Error(w, "Spaces initialization failed; see server diagnostics", http.StatusInternalServerError)
		return
	}
	s.config = parser.HbConfig
	s.dashboard = cfg.Development.Dashboard.Enabled
	h.mu.Lock()
	defer h.mu.Unlock()
	s.ServeHTTP(w, r)
}

func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' https: http: blob:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	if !s.allowedHost(r.Host) {
		replyError(w, &statusError{403, "Host is not allowed; add the server hostname or IP without scheme or port to development.frontend_editing.spaces.allowed_hosts"})
		return
	}
	sub := strings.TrimPrefix(r.URL.Path, s.route)
	if sub == r.URL.Path || (sub != "" && !strings.HasPrefix(sub, "/")) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, HEAD, POST")
			replyError(w, &statusError{405, "method not allowed"})
			return
		}
		if !s.write {
			replyError(w, &statusError{403, "Spaces is read-only"})
			return
		}
		if err := checkOrigin(r); err != nil {
			replyError(w, err)
			return
		}
	}
	if sub == "" || sub == "/" || strings.HasPrefix(sub, "/web/") {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			replyError(w, &statusError{405, "method not allowed"})
			return
		}
		file := "web/index.html"
		if strings.HasPrefix(sub, "/web/") {
			file = strings.TrimPrefix(sub, "/")
		}
		if file == "web/hyperbricks.css" {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			if r.Method != http.MethodHead {
				_, _ = w.Write(ui.Stylesheet)
			}
			return
		}
		if file == "web/theme.js" {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			if r.Method != http.MethodHead {
				_, _ = w.Write(ui.ThemeScript)
			}
			return
		}
		if file == "web/lucide.js" {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			if r.Method != http.MethodHead {
				_, _ = w.Write(ui.IconsScript)
			}
			return
		}
		if file == "web/brandmark.svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			if r.Method != http.MethodHead {
				_, _ = w.Write(ui.LogoMark)
			}
			return
		}
		if file == "web/favicon.svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			if r.Method != http.MethodHead {
				_, _ = w.Write(ui.Favicon)
			}
			return
		}
		if file != "web/index.html" && file != "web/app.js" && file != "web/recovery.mjs" && file != "web/http.mjs" && file != "web/navigation.mjs" && file != "web/contextual.js" && file != "web/contextual.css" && file != "web/document.css" && file != "web/style.css" && file != "web/lucide.js" {
			http.NotFound(w, r)
			return
		}
		asset := file
		b, err := webFiles.ReadFile(asset)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if file == "web/index.html" {
			b = []byte(strings.ReplaceAll(string(b), "__SPACES_BASE__", s.route))
			navigation := ""
			errorsNavigation := ""
			if s.dashboard {
				navigation = `<a href="/__hyperbricks/dashboard"><i data-lucide="layout-dashboard"></i>Overview</a>`
				errorsNavigation = `<a href="/__hyperbricks/errors"><i data-lucide="circle-alert"></i>Errors</a>`
			}
			b = []byte(strings.ReplaceAll(string(b), "__DASHBOARD_NAV__", navigation))
			b = []byte(strings.ReplaceAll(string(b), "__ERRORS_NAV__", errorsNavigation))
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(file)))
		if r.Method != http.MethodHead {
			w.Write(b)
		}
		return
	}
	c, err := s.catalog()
	if err != nil {
		replyError(w, err)
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		switch sub {
		case "/api":
			snap, err := s.snapshot(c)
			if err != nil {
				replyError(w, err)
				return
			}
			replyJSON(w, 200, snap)
		case "/api/asset":
			field, err := s.assetField(c, r.URL.Query().Get("name"), r.URL.Query().Get("field"))
			if err != nil {
				replyError(w, err)
				return
			}
			reference := r.URL.Query().Get("reference")
			ext := strings.ToLower(filepath.Ext(reference))
			if fieldDirectory(field).Base != "resources" || !supportedExtension(ext) || ext == ".md" || ext == ".markdown" {
				replyError(w, fmt.Errorf("preview requires an image in the field's resources directory"))
				return
			}
			filename, data, err := s.assetData(field, reference)
			if err != nil {
				replyError(w, err)
				return
			}
			w.Header().Set("Content-Type", mime.TypeByExtension(ext))
			http.ServeContent(w, r, filepath.Base(filename), time.Time{}, bytes.NewReader(data))
		case "/api/assets":
			assets, err := s.assetList(c, r.URL.Query().Get("name"), r.URL.Query().Get("field"))
			if err != nil {
				replyError(w, err)
				return
			}
			replyJSON(w, 200, assets)
		case "/api/document":
			doc, err := s.document(c, r.URL.Query().Get("name"), r.URL.Query().Get("field"))
			if err != nil {
				replyError(w, err)
				return
			}
			replyJSON(w, 200, doc)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if sub == "/api/document" {
		media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if media != "application/json" {
			replyError(w, &statusError{415, "expected application/json"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 7<<20)
		var m DocumentMutation
		if err := decodeJSON(r.Body, &m); err != nil {
			replyError(w, err)
			return
		}
		doc, preview, err := s.mutateDocument(c, m)
		if err != nil {
			replyError(w, err)
			return
		}
		if m.Action == "preview" {
			replyJSON(w, 200, map[string]string{"html": preview})
		} else {
			replyJSON(w, 200, doc)
		}
		return
	}
	var mutation Mutation
	var upload *pendingUpload
	switch sub {
	case "/api":
		media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if media != "application/json" {
			replyError(w, &statusError{415, "expected application/json"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := decodeJSON(r.Body, &mutation); err != nil {
			replyError(w, err)
			return
		}
	case "/api/upload":
		r.Body = http.MaxBytesReader(w, r.Body, 22<<20)
		if err := r.ParseMultipartForm(22 << 20); err != nil {
			replyError(w, &statusError{413, "invalid or oversized upload"})
			return
		}
		defer r.MultipartForm.RemoveAll()
		if err := decodeJSON(strings.NewReader(r.FormValue("mutation")), &mutation); err != nil {
			replyError(w, err)
			return
		}
		if mutation.Revision != c.revision {
			replyError(w, conflict("Source files changed; reload before uploading"))
			return
		}
		if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
			replyError(w, fmt.Errorf("upload exactly one file"))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			replyError(w, err)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 20<<20+1))
		if err != nil {
			replyError(w, err)
			return
		}
		upload, err = s.prepareUpload(c, mutation.Name, r.FormValue("field"), header.Filename, data)
		if err != nil {
			replyError(w, err)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err := s.mutate(c, mutation, upload); err != nil {
		replyError(w, err)
		return
	}
	// Save acknowledgement concerns persistence, not asynchronous runtime refresh.
	replyJSON(w, 200, map[string]interface{}{"saved": true, "name": mutation.Name})
}

func decodeJSON(r io.Reader, v interface{}) error {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			return &statusError{413, "request is too large"}
		}
		return fmt.Errorf("invalid request: %w", err)
	}
	var extra interface{}
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected a single JSON object")
	}
	return nil
}
func checkOrigin(r *http.Request) error {
	if r.Header.Get("X-Spaces-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return &statusError{403, "same-origin editor request required"}
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if err != nil || u.Scheme != scheme || !strings.EqualFold(u.Host, r.Host) || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return &statusError{403, "cross-origin writes are not allowed"}
		}
	}
	return nil
}
func (s *service) allowedHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	for _, allowed := range s.allowedHosts {
		if strings.EqualFold(allowed, host) {
			return true
		}
	}
	return false
}
func replyJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func replyError(w http.ResponseWriter, err error) {
	status := 400
	var e *statusError
	if errors.As(err, &e) {
		status = e.status
	}
	replyJSON(w, status, map[string]string{"error": err.Error()})
}
