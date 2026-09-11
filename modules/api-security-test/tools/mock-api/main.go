package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type response struct {
	Source     string `json:"source,omitempty"`
	AuthKind   string `json:"auth_kind,omitempty"`
	Result     string `json:"result,omitempty"`
	Message    string `json:"message"`
	UserToken  string `json:"user_token,omitempty"`
	AdminToken string `json:"admin_token,omitempty"`
}

type barrierGeneration struct {
	arrived int
	release chan struct{}
}

type pairBarrier struct {
	mu         sync.Mutex
	generation *barrierGeneration
}

func newPairBarrier() *pairBarrier {
	return &pairBarrier{generation: &barrierGeneration{release: make(chan struct{})}}
}

func (b *pairBarrier) wait(ctx context.Context) bool {
	b.mu.Lock()
	generation := b.generation
	generation.arrived++
	if generation.arrived == 2 {
		close(generation.release)
		b.generation = &barrierGeneration{release: make(chan struct{})}
	}
	b.mu.Unlock()

	select {
	case <-generation.release:
		return true
	case <-ctx.Done():
		b.mu.Lock()
		if b.generation == generation {
			generation.arrived--
		}
		b.mu.Unlock()
		return false
	}
}

func main() {
	port := flag.Int("port", 8098, "primary fixture API port")
	redirectPort := flag.Int("redirect-port", 8099, "cross-origin redirect target port")
	flag.Parse()

	redirectAddress := fmt.Sprintf("127.0.0.1:%d", *redirectPort)
	redirectURL := "http://" + redirectAddress
	redirectMux := http.NewServeMux()
	redirectMux.HandleFunc("/redirect-target/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		writeJSON(w, http.StatusOK, apiResponse(r, "redirect-target"))
	})
	go func() {
		log.Printf("redirect target listening on %s", redirectURL)
		if err := http.ListenAndServe(redirectAddress, redirectMux); err != nil {
			log.Fatal(err)
		}
	}()

	primaryAddress := fmt.Sprintf("127.0.0.1:%d", *port)
	primaryURL := "http://" + primaryAddress
	primaryMux := http.NewServeMux()
	barrier := newPairBarrier()
	primaryMux.HandleFunc("/issue/valid", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		writeJSON(w, http.StatusOK, response{
			Result:     "issued",
			Message:    "Issued two fixture credentials.",
			UserToken:  "fixture-user-token",
			AdminToken: "fixture-admin-token",
		})
	})
	primaryMux.HandleFunc("/issue/missing", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		writeJSON(w, http.StatusOK, response{
			Result:    "incomplete",
			Message:   "The admin token is deliberately absent.",
			UserToken: "fixture-user-token",
		})
	})
	primaryMux.HandleFunc("/issue/empty", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		writeJSON(w, http.StatusOK, response{
			Result:     "incomplete",
			Message:    "The user token is deliberately empty.",
			AdminToken: "fixture-admin-token",
		})
	})
	primaryMux.HandleFunc("/issue/invalid", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		writeJSON(w, http.StatusOK, response{
			Result:     "invalid",
			Message:    "The user token deliberately contains cookie syntax.",
			UserToken:  "invalid; Domain=example.test",
			AdminToken: "fixture-admin-token",
		})
	})
	primaryMux.HandleFunc("/issue/trailing-json", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"result":"issued","user_token":"fixture-user-token"} trailing`)
	})
	primaryMux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		w.WriteHeader(http.StatusNoContent)
	})
	primaryMux.HandleFunc("/redirect/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		http.Redirect(w, r, redirectURL+"/redirect-target/"+r.URL.Path[len("/redirect/"):], http.StatusFound)
	})
	primaryMux.HandleFunc("/barrier/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		if barrier.wait(r.Context()) {
			writeJSON(w, http.StatusOK, apiResponse(r, r.URL.Path[1:]))
		}
	})
	primaryMux.HandleFunc("/cancel/", func(_ http.ResponseWriter, r *http.Request) {
		logRequest(r)
		<-r.Context().Done()
	})
	primaryMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		writeJSON(w, http.StatusOK, apiResponse(r, r.URL.Path[1:]))
	})

	log.Printf("primary fixture API listening on %s", primaryURL)
	log.Printf("set HYPERBRICKS_API_SECURITY_UPSTREAM=%s", primaryURL)
	log.Fatal(http.ListenAndServe(primaryAddress, primaryMux))
}

func apiResponse(r *http.Request, source string) response {
	return response{
		Source:   source,
		AuthKind: classifyAuthorization(r.Header.Get("Authorization")),
		Message:  "Fixture request completed.",
	}
}

func classifyAuthorization(value string) string {
	switch value {
	case "":
		return "none"
	case "Bearer fixture-user-token":
		return "user-session"
	case "Bearer fixture-admin-token":
		return "admin-session"
	case "Bearer fixture-service-token":
		return "service"
	default:
		return "unexpected"
	}
}

func logRequest(r *http.Request) {
	log.Printf("%s %s auth=%s", r.Method, r.URL.Path, classifyAuthorization(r.Header.Get("Authorization")))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
