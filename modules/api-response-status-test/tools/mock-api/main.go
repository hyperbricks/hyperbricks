package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

// This deterministic loopback service supplies actual HTTP responses to the
// runnable module. It deliberately requires no credentials or external service.
func main() {
	port := flag.Int("port", 8114, "loopback API port")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/catalog", func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		message := "Catalog item"
		switch r.URL.Query().Get("id") {
		case "missing":
			status, message = http.StatusNotFound, "Item not found"
		case "unavailable":
			status, message = http.StatusServiceUnavailable, "Service unavailable"
		case "conflict":
			status, message = http.StatusConflict, "Expected conflict"
		case "forbidden":
			status, message = http.StatusForbidden, "Unhandled upstream status"
		case "invalid-json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, "invalid JSON")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
	})
	address := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("response-status fixture API listening at http://%s", address)
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
