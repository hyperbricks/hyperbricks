// This local teaching API keeps one project in memory. Restarting resets it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type project struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type fixture struct {
	mu      sync.Mutex
	project project
}

func newFixture() *fixture {
	return &fixture{project: project{Name: "Community garden", Version: 1}}
}

func (f *fixture) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ready": true})
	})
	mux.HandleFunc("GET /api/project", f.readProject)
	mux.HandleFunc("POST /api/project", f.saveProject)
	mux.HandleFunc("POST /auth/demo-login", demoLogin)
	mux.HandleFunc("POST /auth/demo-logout", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"message": "Demo session cleared."})
	})
	mux.HandleFunc("GET /auth/owner", func(w http.ResponseWriter, r *http.Request) {
		if requireOwner(w, r) {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		// The service validates access too, even if a caller bypasses the page guard.
		if requireOwner(w, r) {
			writeJSON(w, http.StatusOK, map[string]any{
				"message":    "You can read the owner-only project settings.",
				"visibility": "Private teaching project",
			})
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func (f *fixture) readProject(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	p := f.project
	f.mu.Unlock()
	writeJSON(w, http.StatusOK, p)
}

func (f *fixture) saveProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if !decodeBody(w, r, &input) {
		return
	}
	name := strings.TrimSpace(input.Name)
	version, err := strconv.Atoi(input.Version)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"message": "Enter a project name between 1 and 80 characters.",
		})
		return
	}
	if err != nil || version < 1 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"message": "Reload the project before saving: its version is missing or invalid.",
		})
		return
	}

	// Compare and update together: two editors cannot both save the same version.
	f.mu.Lock()
	defer f.mu.Unlock()
	if version != f.project.Version {
		writeJSON(w, http.StatusConflict, map[string]any{
			"message": "This project changed in another tab. Reload it before saving again.",
			"version": f.project.Version,
		})
		return
	}
	f.project.Name = name
	f.project.Version++
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Project name saved in the local demo API.",
		"name":    f.project.Name, "version": f.project.Version,
	})
}

func demoLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Role string `json:"role"`
	}
	if !decodeBody(w, r, &input) {
		return
	}
	if input.Role != "owner" && input.Role != "viewer" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "Choose owner or viewer."})
		return
	}
	// These public, fixed tokens demonstrate responses. They are not authentication.
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "You selected the demo " + input.Role + " role.",
		"token":   "demo-" + input.Role,
	})
}

func requireOwner(w http.ResponseWriter, r *http.Request) bool {
	switch r.Header.Get("Authorization") {
	case "Bearer demo-owner":
		return true
	case "Bearer demo-viewer":
		writeJSON(w, http.StatusForbidden, map[string]any{"message": "This demo role cannot read owner settings."})
	default:
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "Choose a demo role first."})
	}
	return false
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Send a valid JSON object with the expected fields."})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	port := flag.Int("port", 8099, "local fixture API port")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		log.Fatal("port must be between 1 and 65535")
	}
	address := fmt.Sprintf("127.0.0.1:%d", *port)
	server := &http.Server{
		Addr: address, Handler: newFixture().handler(),
		ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second,
	}
	log.Printf("Project Desk teaching API: http://%s (memory only; restart resets data)", address)
	log.Fatal(server.ListenAndServe())
}
